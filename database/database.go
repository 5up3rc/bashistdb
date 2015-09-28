// Copyright (c) 2015, Marios Andreopoulos.
//
// This file is part of bashistdb.
//
// 	Bashistdb is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// 	Bashistdb is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// 	You should have received a copy of the GNU General Public License
// along with bashistdb.  If not, see <http://www.gnu.org/licenses/>.

/*
Package database handles a SQLite3 database and access methods for the
specific needs of bashistdb.
*/
package database

import (
	"bufio"
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mattn/go-sqlite3"
	conf "projects.30ohm.com/mrsaccess/bashistdb/configuration"
	"projects.30ohm.com/mrsaccess/bashistdb/llog"
)

// Golang's RFC3339 does not comply with all RFC3339 representations
const RFC3339alt = "2006-01-02T15:04:05-0700"

const VERSION = "2"

type Database struct {
	*sql.DB
	statements
}

type statements struct {
	insert *sql.Stmt
}

var log *llog.Logger

func init() {
	log = conf.Log
}

func New() (Database, error) {
	// If database file does not exist, set a flag to create file and table.
	init := false
	if _, err := os.Stat(conf.DbFile); os.IsNotExist(err) {
		log.Info.Println("Database file not found. Creating new.")
		init = true
	} else {
		log.Info.Println("Database file found.")
	}
	// Open database. SQLite3 provides concurrency in the library level, thus
	// we don't need to implement locking.
	db, err := sql.Open("sqlite3", conf.DbFile)
	if err != nil {
		return Database{}, err
	}
	// If database is new, initialize it with our tables.
	if init {
		if err = initDB(db); err != nil {
			_ = db.Close()
			return Database{}, err
		}
	} else {
		err := upgradeIfNeed(db)
		if err != nil {
			return Database{}, err
		}
	}
	// Prepare various statements
	errs := make([]error, 5)
	var insert *sql.Stmt
	insert, errs[0] = db.Prepare("INSERT INTO history(user, host, command, datetime) VALUES(?, ?, ?, ?)")
	for _, e := range errs {
		if e != nil {
			_ = db.Close()
			return Database{}, e
		}
	}
	stmts := statements{insert}
	return Database{db, stmts}, nil
}

func initDB(db *sql.DB) error {
	stmt := `CREATE TABLE history (
                        user     TEXT,
                        host     TEXT,
                        command  TEXT,
                        datetime DATETIME,
                        PRIMARY KEY (user, command, datetime)
                     );
                    CREATE TABLE admin (
                        key   TEXT PRIMARY KEY,
                        value TEXT
                     );
                    CREATE TABLE connlog (
                        datetime TEXT PRIMARY KEY,
                        remote   TEXT
                     );
	            CREATE TABLE rlookup (
                        ip      TEXT PRIMARY KEY,
                        reverse TEXT
                     );
                    CREATE VIEW connections AS
                         SELECT datetime, remote, reverse
                           FROM connlog AS c
                             JOIN rlookup AS r
                               ON c.remote=d.ip;`

	if _, err := db.Exec(stmt); err != nil {
		return err
	}

	stmt = `INSERT INTO admin VALUES ("version", ?)`

	if _, err := db.Exec(stmt, VERSION); err != nil {
		return err
	}
	return nil
}

// AddRecord tries to insert a new record in the database,
// if the record already exists, it updates the count
func (d Database) AddRecord(user, host, command string, time time.Time) error {
	// Try to insert row
	_, err := d.insert.Exec(user, host, command, time)
	if err != nil {
		// If failed due to duplicate primary key, then ignore error
		// We expect for ease of use, the user to resubmit the whole
		// history from time to time.
		if driverErr, ok := err.(sqlite3.Error); ok {
			if driverErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
				log.Debug.Println("Duplicate entry. Ignoring.", user, host, command, time)
			} else {
				return err
			}
		} else { // Normally we can never reach this. Should we omit it?
			return err
		}
	}
	return nil
}

func (d Database) AddFromBuffer(r *bufio.Reader, user, host string) error {
	//                                  LINENUM        DATETIME         CM
	parseLine := regexp.MustCompile(`^ *[0-9]+\*? *([0-9T:+-]{24,24}) *(.*)`)
	tx, _ := d.Begin()
	stmt := tx.Stmt(d.insert)
	total, failed := 0, 0
	for {
		historyLine, err := r.ReadString('\n')
		total++
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return err
			}
		}
		// if historyLine == conf.TRANSMISSION_END {
		// 	break
		// }
		args := parseLine.FindStringSubmatch(historyLine)
		if len(args) != 3 {
			log.Info.Println("Could't decode line. Skipping:", historyLine)
			failed++
			continue
		}
		time, err := time.Parse(RFC3339alt, args[1])
		if err != nil {
			tx.Rollback()
			return err
		}

		_, err = stmt.Exec(user, host, strings.TrimSuffix(args[2], "\n"), time)
		if err != nil {
			// If failed due to duplicate primary key, then ignore error
			// We expect for ease of use, the user to resubmit the whole
			// history from time to time.
			if driverErr, ok := err.(sqlite3.Error); ok {
				if driverErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
					log.Debug.Println("Duplicate entry. Ignoring.", user, host, strings.TrimSuffix(args[2], "\n"), time)
					failed++
				} else {
					tx.Rollback()
					return err
				}
			} else { // Normally we can never reach this. Should we omit it?
				return err
			}
		}
	}
	tx.Commit()
	total--
	log.Info.Printf("Processed %d entries, successful %d, failed %d.\n", total, total-failed, failed)
	return nil
}

func (d Database) Top20() (result string, e error) {
	result = fmt.Sprintln("Top-20 commands:")
	rows, e := d.Query("SELECT command, count(*) as count FROM history GROUP BY command ORDER BY count DESC LIMIT 20")
	if e != nil {
		return result, e
	}
	defer rows.Close()
	for rows.Next() {
		var command string
		var count int
		rows.Scan(&command, &count)
		result += fmt.Sprintf("%d: %s\n", count, command)
	}
	return result, e
}

func (d Database) Last20() (result string, e error) {
	result = fmt.Sprintln("Last 10 commands:")
	rows, e := d.Query("SELECT  datetime, command FROM history ORDER BY datetime DESC LIMIT 10")
	if e != nil {
		return result, nil
	}
	defer rows.Close()
	for rows.Next() {
		var command string
		var time time.Time
		rows.Scan(&time, &command)
		result += fmt.Sprintf("%s %s\n", time, command)
	}
	return result, e
}

func (d Database) LogConn(remote net.Addr) (err error) {
	t := time.Now()
	_, err = d.Exec(`INSERT INTO connlog VALUES (?, ?, ?);`, t, remote.String(), nil)
	// Perform a reverse lookup if needed.
	go func() {
		if ip, _, err := net.SplitHostPort(remote.String()); err == nil {
			var rip string
			err = d.QueryRow("SELECT ip FROM rlookup WHERE ip = ?", ip).Scan(&rip)
			if err == sql.ErrNoRows {
				if addr, err := net.LookupAddr(ip); err == nil {
					_, _ = d.Exec(`INSERT INTO rlookup(ip, reverse)
                                                           VALUES(? ,?)`,
						remote.String(), strings.Join(addr, ","))
				}
			}
		}
	}()
	return
}

func upgradeIfNeed(d *sql.DB) error {
	var version string
	err := d.QueryRow(`SELECT value FROM admin WHERE key LIKE "version"`).Scan(&version)
	if err != nil {
		return err
	}

	switch version {
	case "1":
		tx, err := d.Begin()
		if err != nil {
			return err
		}
		stmt := `CREATE TABLE connlog_new(
                             datetime TEXT PRIMARY KEY,
                             remote   TEXT);
                         INSERT INTO connlog_new
                           SELECT datetime, remote FROM connlog;
                         DROP TABLE connlog;
                         ALTER TABLE connlog_new RENAME TO 'connlog';
                         CREATE TABLE rlookup (
                             ip      TEXT PRIMARY KEY,
                             reverse TEXT
                         );
                         CREATE VIEW connections AS
                             SELECT datetime, remote, reverse
                               FROM connlog AS c
                                 LEFT JOIN rlookup AS r
                                   ON c.remote = r.ip;`
		if _, err = tx.Exec(stmt); err != nil {
			return err
		}
		if _, err = tx.Exec(`UPDATE admin SET value=? WHERE key LIKE 'version'`, VERSION); err != nil {
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		log.Info.Println("Database upgraded to latest version.")
		return nil
	case "2":
		log.Debug.Println("Database on latest version.")
	}

	if version != VERSION {
		return errors.New("Database version different than code version but couldn't fix it.")
	}

	return nil
}

// Restore returns history within the search criteria in timestamped bash_history format
func (d Database) Restore(user, hostname string) (string, error) {
	rows, err := d.Query(`SELECT datetime, command FROM history WHERE user LIKE ? AND host LIKE ?`,
		"%"+user+"%", "%"+hostname+"%")
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var result bytes.Buffer
	for rows.Next() {
		var command string
		var t time.Time
		rows.Scan(&t, &command)
		result.WriteString(fmt.Sprintf("#%d\n%s\n", t.Unix(), command))
	}
	return result.String(), nil
}

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
	"sync"
	"time"

	conf "github.com/andmarios/bashistdb/configuration"
	"github.com/andmarios/bashistdb/llog"
	"github.com/andmarios/bashistdb/result"
	"github.com/mattn/go-sqlite3"
)

// Golang's RFC3339 does not comply with all RFC3339 representations
const RFC3339alt = "2006-01-02T15:04:05-0700"

// VERSION is the database's schema supported version.
// If your database is older it will be automatically migrated.
// If it is newer you have to update your bashistdb copy.
const VERSION = "2"

// A Database holds a bashistdb database.
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

// New returns a new Database instance. It gets the filename for the
// database from the configuration package. If the file does not exist,
// it creates a new database. If it exists, it migrates it if it has an
// older schema version than current.
func New() (Database, error) {
	// If database file does not exist, set a flag to create file and table.
	init := false
	if _, err := os.Stat(conf.Database); os.IsNotExist(err) {
		log.Info.Println("Database file not found. Creating new.")
		init = true
	} else {
		log.Info.Println("Database file found.")
	}
	// Open database. SQLite3 provides concurrency in the library level, thus
	// we don't need to implement locking.
	db, err := sql.Open("sqlite3", conf.Database)
	if err != nil {
		return Database{}, err
	}
	// If database is new, initialize it with our tables.
	// Else migrate it if needed.
	if init {
		if err = initDB(db); err != nil {
			_ = db.Close()
			return Database{}, err
		}
	} else {
		err := migrate(db)
		if err != nil {
			return Database{}, err
		}
	}
	// Prepare various statements that may be used frequently.
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
                             LEFT JOIN rlookup AS r
                               ON c.remote=r.ip;`

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
// Note: function isn't used anywhere, may need testing if used.
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

// A parseline parses history output lines of the following format:
//     LINENUM RFC3339_DATETIME COMMAND
var parseLine = regexp.MustCompile(`^ *[0-9]+\*? *([0-9T:+-]{24,24}) *(.*)`)

// A parseExportLine parses export formatted output from bashistdb:
//     USER HOSTNAME RFC3339_DATETIME COMMAND
var parseExportLine = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_-]*) ([a-zA-Z0-9][a-zA-Z0-9_.]*) *([0-9T:+-]{24,24}) *(.*)`)

// AddFromBuffer reads from a buffered Reader and scans for lines that match
// history command's structure:
//     LINENUM RFC3339_DATETIME COMMAND
// Upon succesful encounter it tries to store it to the database. It counts
// total lines read and lines failed to insert into the database —usually
// because they already exist. It reports the results in a sentence (stats
// string) because we don't anything fancier currently.
func (d Database) AddFromBuffer(r *bufio.Reader, user, host string) (stats string, e error) {
	//                                  LINENUM        DATETIME         CM
	tx, _ := d.Begin()
	stmt := tx.Stmt(d.insert)
	total, failed := 0, 0
	lineFormat := 1 // 1 means default history format, 3 is for export format
	var once sync.Once
	for {
		historyLine, err := r.ReadString('\n')
		total++
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return "", errors.New("Error while reading stdin: " + err.Error())
			}
		}

		args := parseLine.FindStringSubmatch(historyLine)
		if len(args) != 3 {
			args = parseExportLine.FindStringSubmatch(historyLine)
			if len(args) != 5 {
				log.Info.Println("Could't decode line, unknown format. Skipping:", historyLine)
				failed++
				continue
			}
			once.Do(func() { log.Info.Println("Bashistdb export format detected.") })
			lineFormat = 3
		}

		time, err := time.Parse(RFC3339alt, args[lineFormat])
		if err != nil {
			tx.Rollback()
			return "", err
		}

		switch lineFormat {
		case 1:
			_, err = stmt.Exec(user, host, strings.TrimSuffix(args[2], "\n"), time)
		case 3:
			_, err = stmt.Exec(args[1], args[2], strings.TrimSuffix(args[4], "\n"), time)
		}
		if err != nil {
			// If failed due to duplicate primary key, then ignore error
			// We expect for ease of use, the user to resubmit the whole
			// history from time to time.
			if driverErr, ok := err.(sqlite3.Error); ok {
				if driverErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
					switch lineFormat {
					case 1:
						log.Debug.Println("Duplicate entry. Ignoring.", user, host, strings.TrimSuffix(args[2], "\n"), time)
					case 3:
						log.Debug.Println("Duplicate entry. Ignoring.", args[1], args[2], strings.TrimSuffix(args[4], "\n"), time)
					}
					failed++
				} else {
					tx.Rollback()
					return "", err
				}
			} else { // Normally we can never reach this. Should we omit it?
				return "", err
			}
		}
	}
	tx.Commit()
	total--
	stats = fmt.Sprintf("Processed %d entries, successful %d, failed %d.", total, total-failed, failed)
	return stats, nil
}

// TopK returns the k most frequent command lines in history
func (d Database) TopK(qp conf.QueryParams) (res []byte, e error) {
	var result bytes.Buffer
	result.WriteString(fmt.Sprintf("Top-%d commands:", qp.Kappa))
	rows, e := d.Query(`SELECT command, count(*) as count FROM history
                               WHERE user LIKE ? AND host LIKE ? AND command LIKE ? ESCAPE '\'
                               GROUP BY command ORDER BY count DESC LIMIT ?`,
		qp.User, qp.Host, qp.Command, qp.Kappa)
	if e != nil {
		return result.Bytes(), e
	}
	defer rows.Close()
	for rows.Next() {
		var command string
		var count int
		rows.Scan(&command, &count)
		result.WriteString(fmt.Sprintf("\n%d: %s", count, command))
	}
	return result.Bytes(), e
}

// LastK returns the k most recent command lines in history
func (d Database) LastK(qp conf.QueryParams) (res []byte, e error) {
	var result bytes.Buffer
	result.WriteString(fmt.Sprintf("%d most recent commands:", qp.Kappa))
	var rows *sql.Rows
	switch qp.Unique {
	case true:
		rows, e = d.Query(`SELECT * FROM
                                      (SELECT datetime, command FROM history
                                         WHERE user LIKE ? AND host LIKE ? AND command LIKE ? ESCAPE '\'
                                         GROUP BY command
                                         ORDER BY datetime DESC LIMIT ?)
                                      ORDER BY datetime ASC`,
			qp.User, qp.Host, qp.Command, qp.Kappa)
	default:
		rows, e = d.Query(`SELECT * FROM
                                      (SELECT datetime, command FROM history
                                         WHERE user LIKE ? AND host LIKE ? AND command LIKE ? ESCAPE '\'
                                         ORDER BY datetime DESC LIMIT ?)
                                   ORDER BY datetime ASC`,
			qp.User, qp.Host, qp.Command, qp.Kappa)
	}
	if e != nil {
		return result.Bytes(), e
	}
	defer rows.Close()

	for rows.Next() {
		var command string
		var time time.Time
		rows.Scan(&time, &command)
		result.WriteString(fmt.Sprintf("\n%s %s", time.Format(RFC3339alt), command))
	}
	return result.Bytes(), e
}

// LogConn logs the remote's IP address and connection time into connlog table.
// Also if it can't find a reverse lookup for the IP address inside table rlookup,
// it performs it asynchronously. Reverse lookup may fail, but we don't care.
func (d Database) LogConn(remote net.Addr) (err error) {
	t := time.Now()
	// Find IP
	if ip, _, err := net.SplitHostPort(remote.String()); err == nil {
		// Store IP and datetime
		_, err = d.Exec(`INSERT INTO connlog VALUES (?, ?);`, t, ip)
		if err == nil {
			// Perform a reverse lookup if needed.
			go func() {
				var rip string
				err = d.QueryRow("SELECT ip FROM rlookup WHERE ip LIKE ?", ip).Scan(&rip)
				if err == sql.ErrNoRows {
					if addr, err := net.LookupAddr(ip); err == nil {
						_, err = d.Exec(`INSERT INTO rlookup(ip, reverse)
                                                           VALUES(? ,?)`,
							ip, strings.Join(addr, ","))
					}
				}
				if err != nil {
					log.Info.Println(err)
				}
			}()
		}
	}
	return
}

// migrate is a unexported function that handles database migrations.
// It is safe to run on databases that already are on latest version.
func migrate(d *sql.DB) error {
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

// DefaultQuery returns history within the search criteria in the format requested
func (d Database) DefaultQuery(qp conf.QueryParams) ([]byte, error) {
	var rows *sql.Rows
	var err error
	switch qp.Unique {
	case true:
		rows, err = d.Query(`SELECT rowid, datetime, user, host, command FROM history
                                        WHERE user LIKE ? AND host LIKE ? AND command LIKE ? ESCAPE '\'
                                        GROUP BY command ORDER BY DATETIME ASC`,
			qp.User, qp.Host, qp.Command)
	default:
		rows, err = d.Query(`SELECT rowid, datetime, user, host, command FROM history
                                         WHERE user LIKE ? AND host LIKE ? AND command LIKE ? ESCAPE '\'`,
			qp.User, qp.Host, qp.Command)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := result.New(qp.Format)
	for rows.Next() {
		var user, host, command string
		var t time.Time
		var row int
		rows.Scan(&row, &t, &user, &host, &command)
		res.AddRow(row, t, user, host, command)
	}
	// Return the result without the newline at the end.
	return res.Formatted(), nil
}

// RunQuery is a wrapper around various queries.
func (d Database) RunQuery(p conf.QueryParams) ([]byte, error) {
	switch p.Type {
	case conf.QUERY:
		return d.DefaultQuery(p)
	case conf.QUERY_LASTK:
		return d.LastK(p)
	case conf.QUERY_TOPK:
		return d.TopK(p)
	case conf.QUERY_USERS:
		return d.Users(p)
	}

	return []byte{}, errors.New("Unknown query type.")
}

func (d Database) Users(qp conf.QueryParams) (res []byte, e error) {
	var result bytes.Buffer
	result.WriteString(fmt.Sprintf("Unique user-hosts pairs:"))
	rows, e := d.Query(`SELECT distinct(user), host FROM history
                               WHERE user LIKE ? AND host LIKE ? AND command LIKE ? ESCAPE '\'`,
		qp.User, qp.Host, qp.Command)
	if e != nil {
		return result.Bytes(), e
	}
	defer rows.Close()

	for rows.Next() {
		var user string
		var host string
		rows.Scan(&user, &host)
		result.WriteString(fmt.Sprintf("\n%s@%s", user, host))
	}
	return result.Bytes(), e
}

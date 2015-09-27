// Copyright (c) 2015, Marios Andreopoulos.
//
// This file is part of bashistdb.
//
// 	Foobar is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// 	Foobar is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// 	You should have received a copy of the GNU General Public License
// along with Foobar.  If not, see <http://www.gnu.org/licenses/>.

/*
Package database handles a SQLite3 database and access methods for the
specific needs of bashistdb.
*/
package database

import (
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mattn/go-sqlite3"
	"projects.30ohm.com/mrsaccess/bashistdb/code"
	"projects.30ohm.com/mrsaccess/bashistdb/llog"
)

// Golang's RFC3339 does not comply with all RFC3339 representations
const RFC3339alt = "2006-01-02T15:04:05-0700"

type Database struct {
	db *sql.DB
	l  *llog.Logger
	statements
}

type statements struct {
	insert *sql.Stmt
}

func New(file string, l *llog.Logger) (Database, error) {
	// If database file does not exist, set a flag to create file and table.
	init := false
	if _, err := os.Stat(file); os.IsNotExist(err) {
		l.Info.Println("Database file not found. Creating new.")
		init = true
	} else {
		l.Info.Println("Database file found.")
	}
	// Open database. SQLite3 provides concurrency in the library level, thus
	// we don't need to implement locking.
	db, err := sql.Open("sqlite3", file)
	if err != nil {
		return Database{nil, l, statements{nil}}, err
	}
	// If database is new, initialize it with our tables.
	if init {
		if err = initDB(db); err != nil {
			_ = db.Close()
			return Database{nil, l, statements{nil}}, err
		}
	}
	// Prepare various statements
	errs := make([]error, 5)
	var insert *sql.Stmt
	insert, errs[0] = db.Prepare("INSERT INTO history(user, host, command, datetime) VALUES(?, ?, ?, ?)")
	for _, e := range errs {
		if e != nil {
			_ = db.Close()
			return Database{nil, l, statements{nil}}, e
		}
	}
	stmts := statements{insert}
	return Database{db, l, stmts}, nil
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
                     );`

	if _, err := db.Exec(stmt); err != nil {
		return err
	}

	stmt = `INSERT INTO admin VALUES ("version","1")`

	if _, err := db.Exec(stmt); err != nil {
		return err
	}
	return nil
}

func (d Database) Close() error {
	return d.db.Close()
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
				d.l.Debug.Println("Duplicate entry. Ignoring.", user, host, command, time)
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
	tx, _ := d.db.Begin()
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
		if historyLine == code.TRANSMISSION_END+"\n" {
			total--
			break
		}
		args := parseLine.FindStringSubmatch(historyLine)
		if len(args) != 3 {
			d.l.Info.Println("Could't decode line. Skipping:", historyLine)
			failed++
			continue
		}
		time, err := time.Parse(RFC3339alt, args[1])
		if err != nil {
			tx.Rollback()
			return err
		}

		_, err = stmt.Exec(user, host, args[2], time)
		if err != nil {
			// If failed due to duplicate primary key, then ignore error
			// We expect for ease of use, the user to resubmit the whole
			// history from time to time.
			if driverErr, ok := err.(sqlite3.Error); ok {
				if driverErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
					d.l.Debug.Println("Duplicate entry. Ignoring.", user, host, strings.TrimSuffix(args[2], "\n"), time)
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
	d.l.Info.Printf("Processed %d entries, successful %d, failed %d.\n", total, total-failed, failed)
	return nil
}

func (d Database) Top20() (result string, e error) {
	result = fmt.Sprintln("Top-20 commands:")
	rows, e := d.db.Query("SELECT command, count(*) as count FROM history GROUP BY command ORDER BY count DESC LIMIT 20")
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
	rows, e := d.db.Query("SELECT  datetime, command FROM history ORDER BY datetime DESC LIMIT 10")
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

// 	// Create a database tx
// 	tx, err := db.Begin()
// 	// Commit on exit. This is ok (and wanted) for our case since we do buffered commits
// 	// and we have only one table and no consistency issues.
// 	defer tx.Commit()
// 	if err != nil {
// 		log.Fatalln(err)
// 	}

// 	// Commit tx every five seconds
// 	go func() {
// 		for _ = range time.Tick(5 * time.Second) {
// 			tx.Commit()
// 		}
// 	}()

// 	// Prepare statement for inserting into database new entries
// 	insertStmt, err = tx.Prepare("INSERT INTO history(user, host, command, datetime) values(?, ?, ?, ?)")
// 	if err != nil {
// 		log.Fatalln(err)
// 	}
// 	defer insertStmt.Close()

// 	stdinReader := bufio.NewReader(os.Stdin)
// 	stats, _ := os.Stdin.Stat()
// 	if (stats.Mode() & os.ModeCharDevice) != os.ModeCharDevice {
// 		err = readFromStdin(stdinReader)
// 		if err != nil {
// 			log.Fatalln("Error while processing stdin:", err)
// 		}
// 		info.Printf("Processed %d entries, successful %d, failed %d.\n", total, total-failed, failed)
// 	} else if *queryString == "" { // Print some stats
// 		tx.Commit()
// 		fmt.Println("Top-20 commands:")
// 		rows, err := db.Query("SELECT command, count(*) as count FROM history GROUP BY command ORDER BY count DESC LIMIT 20")
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		defer rows.Close()
// 		for rows.Next() {
// 			var command string
// 			var count int
// 			rows.Scan(&command, &count)
// 			fmt.Printf("%d: %s\n", count, command)
// 		}
// 		fmt.Println("=================")
// 		fmt.Println("Last 10 commands:")
// 		rows, err = db.Query("SELECT  datetime, command FROM history ORDER BY datetime DESC LIMIT 10")
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		defer rows.Close()
// 		for rows.Next() {
// 			var command string
// 			var time time.Time
// 			rows.Scan(&time, &command)
// 			fmt.Printf("%s %s\n", time, command)
// 		}
// 	} else {
// 		tx.Commit()
// 		fmt.Println("Results:")
// 		rows, err := db.Query("SELECT command FROM history " + *queryString)
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		defer rows.Close()
// 		for rows.Next() {
// 			var command string
// 			rows.Scan(&command)
// 			fmt.Printf("%s\n", command)
// 		}
// 	}
// }

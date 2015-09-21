// Copyright (c) 2015, Marios Andreopoulos. All rights reserved.
// Use of this source code is governed by a BSD-style license that
// can be found in the LICENSE file that should come with this code.

package main

import (
	"bufio"
	"database/sql"
	"flag"
	"github.com/mattn/go-sqlite3"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

var (
	dbFile = flag.String("db", "database.sqlite", "path to database file (will be created if not exists)")
)

var (
	db                     *sql.DB
	insertStmt, updateStmt *sql.Stmt
)

// submitRecord tries to insert a new record in the database,
// if the record already exists, it updates the count
func submitRecord(user, host, command string) error {
	// Try to insert row
	_, err := insertStmt.Exec(user, host, command, 1)
	if err != nil {
		// If failed due to duplicate primary key, then increase count
		if driverErr, ok := err.(sqlite3.Error); ok {
			if driverErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
				_, err = updateStmt.Exec(user, host, command)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}
	return nil
}

func main() {
	flag.Parse()

	// if *printVersion {
	// 	fmt.Println("Quickcert v" + version)
	// 	fmt.Println("https://github.com/andmarios/quickcert")
	// 	os.Exit(0)
	// }

	// If database file does not exist, set a flag to create file and table.
	initDB := false
	if _, err := os.Stat(*dbFile); os.IsNotExist(err) {
		log.Println("Database file not found. Creating new.")
		initDB = true
	} else {
		log.Println("Database file found.")
	}
	// Open database
	var err error // If we do not do this and use := below, db becomes local variable in main()
	db, err = sql.Open("sqlite3", *dbFile)
	if err != nil {
		log.Fatalf("Could not open database file: %s\n", err)
	}
	defer db.Close()
	// Create table if new database
	if initDB {
		sqlStmt := `CREATE TABLE history (
                                user TEXT,
                                host TEXT,
                                command TEXT,
                                count INT,
                                PRIMARY KEY (user, host, command)
                             );`
		_, err := db.Exec(sqlStmt)
		if err != nil {
			log.Fatalf("Error creating table. %q: %s\n", err, sqlStmt)
		}
	}

	// Create a database tx
	tx, err := db.Begin()
	// Commit on exit. This is ok (and wanted) for our case since we do buffered commits
	// and we have only one table and no consistency issues.
	defer tx.Commit()
	if err != nil {
		log.Fatalln(err)
	}

	// Commit tx every five seconds
	go func() {
		for _ = range time.Tick(5 * time.Second) {
			tx.Commit()
		}
	}()
	// Prepare statement for inserting into database new entries
	insertStmt, err = tx.Prepare("INSERT INTO history(user, host, command, count) values(?, ?, ?, ?)")
	if err != nil {
		log.Fatalln(err)
	}
	defer insertStmt.Close()
	// Prepare statement for updating count in existing entries
	updateStmt, err = tx.Prepare(`UPDATE history SET count = count + 1
                                      WHERE user=? AND host=? AND command=?`)
	if err != nil {
		log.Fatalln(err)
	}
	defer updateStmt.Close()

	stdinReader := bufio.NewReader(os.Stdin)

	for {
		historyLine, err := stdinReader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				log.Println("Exiting. Bye")
				break
			} else {
				log.Fatalf("Error reading from stdin: %s\n", err)
			}
		}
		err = submitRecord("mrsaccess", "miles-kitt", strings.TrimSuffix(historyLine, "\n"))
		if err != nil {
			log.Fatalln("Error executing database statement:", err)
		}
	}

	tx.Commit()
	rows, err := db.Query("SELECT command, count FROM history ORDER BY count DESC LIMIT 30")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var command string
		var count int
		rows.Scan(&command, &count)
		log.Print(count, " ", command)
	}
}

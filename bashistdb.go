// Copyright (c) 2015, Marios Andreopoulos. All rights reserved.
// Use of this source code is governed by a BSD-style license that
// can be found in the LICENSE file that should come with this code.

package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"github.com/mattn/go-sqlite3"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

// Golang's RFC3339 does not comply with all RFC3339 representations
const RFC3339alt = "2006-01-02T15:04:05-0700"

var (
	dbDefault    = os.Getenv("HOME") + "/.bashistdb.sqlite3"
	dbFile       = flag.String("db", dbDefault, "path to database file (will be created if not exists)")
	printVersion = flag.Bool("V", false, "print version and exit")
	quietFlag    = flag.Bool("q", true, "quiet, do not log info to stderr")
	debugFlag    = flag.Bool("v", false, "very verbose output")
	user         = flag.String("user", "", "optional user name to use instead of reading $USER variable")
	hostname     = flag.String("hostname", "", "optional hostname to use instead of reading $HOSTNAME variable")
)

var (
	db                     *sql.DB
	insertStmt, updateStmt *sql.Stmt
)

var (
	info  *log.Logger
	debug *log.Logger
)

var (
	total  = 0
	failed = 0
)

// submitRecord tries to insert a new record in the database,
// if the record already exists, it updates the count
func submitRecord(user, host, command string, time time.Time) error {
	// Try to insert row
	_, err := insertStmt.Exec(user, host, command, time)
	if err != nil {
		// If failed due to duplicate primary key, then ignore error
		// We expect for ease of use, the user to resubmit the whole
		// history from time to time.
		if driverErr, ok := err.(sqlite3.Error); ok {
			if driverErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
				debug.Println("Duplicate entry. Ignoring.", user, host, command, time)
				failed++
			} else {
				return err
			}
		} else { // Normally we can never reach this. Should we omit it?
			return err
		}
	}
	total++
	return nil
}

func init() {
	// Read flags and set user and hostname if not provided.
	flag.Parse()

	if *user == "" {
		*user = os.Getenv("USER")
	}
	if *user == "" {
		log.Fatalln("Couldn't read username from $USER system variable and none was provided by -user flag.")
	}

	var err error
	if *hostname == "" {
		*hostname, err = os.Hostname()
		if err != nil {
			log.Fatalln("Couldn't read hostname from $HOSTNAME system variable and none was provided by -hostname flag:", err)
		}
	}

	// Set loggers
	if *debugFlag {
		*quietFlag = false
	}
	if *debugFlag {
		debug = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile)
		debug.Println("Debug enabled.")
	} else {
		debug = log.New(ioutil.Discard, "", 0)
	}
	if !*quietFlag {
		info = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile)
	} else {
		info = log.New(ioutil.Discard, "", 0)
	}

	info.Println("Welcome " + *user + "@" + *hostname + ".")
}

func main() {
	if *printVersion {
		fmt.Println("bashistdb v" + version)
		fmt.Println("https://github.com/andmarios/bashistdb")
		os.Exit(0)
	}

	// If database file does not exist, set a flag to create file and table.
	initDB := false
	if _, err := os.Stat(*dbFile); os.IsNotExist(err) {
		info.Println("Database file not found. Creating new.")
		initDB = true
	} else {
		info.Println("Database file found.")
	}
	// Open database
	// SQLite3 provides concurrency in the library level, thus we don't need to implement locking.
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
                                datetime DATETIME,
                                PRIMARY KEY (command, datetime)
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
	insertStmt, err = tx.Prepare("INSERT INTO history(user, host, command, datetime) values(?, ?, ?, ?)")
	if err != nil {
		log.Fatalln(err)
	}
	defer insertStmt.Close()

	stdinReader := bufio.NewReader(os.Stdin)
	stats, _ := os.Stdin.Stat()
	if (stats.Mode() & os.ModeCharDevice) != os.ModeCharDevice {
		//                                  LINENUM        DATETIME         CM
		parseLine := regexp.MustCompile(`^ *[0-9]+\*? *([0-9T:+-]{24,24}) *(.*)`)
		for {
			historyLine, err := stdinReader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					info.Println("Exiting. Bye")
					break
				} else {
					log.Fatalf("Error reading from stdin: %s\n", err)
				}
			}
			args := parseLine.FindStringSubmatch(historyLine)
			if len(args) != 3 {
				info.Println("Could't decode line. Skipping:", historyLine)
				continue
			}
			time, err := time.Parse(RFC3339alt, args[1])
			if err != nil {
				log.Fatalln(err)
			}
			err = submitRecord(*user, *hostname, strings.TrimSuffix(args[2], "\n"), time)
			if err != nil {
				log.Fatalln("Error executing database statement:", err)
			}
		}
		info.Printf("Processed %d entries, successful %d, failed %d.\n", total, total-failed, failed)
	} else { // Print some stats
		tx.Commit()
		fmt.Println("Top-20 commands:")
		rows, err := db.Query("SELECT command, count(*) as count FROM history GROUP BY command ORDER BY count DESC LIMIT 20")
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var command string
			var count int
			rows.Scan(&command, &count)
			fmt.Printf("%d: %s\n", count, command)
		}
		fmt.Println("=================")
		fmt.Println("Last 10 commands:")
		rows, err = db.Query("SELECT  datetime, command FROM history ORDER BY datetime DESC LIMIT 10")
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var command string
			var time time.Time
			rows.Scan(&time, &command)
			fmt.Printf("%s %s\n", time, command)
		}
	}
}

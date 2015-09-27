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
Command bashistdb stores and retrieves bash history into/from a sqlite3
database.
*/
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"projects.30ohm.com/mrsaccess/bashistdb/code"
	"projects.30ohm.com/mrsaccess/bashistdb/database"
	"projects.30ohm.com/mrsaccess/bashistdb/llog"
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
	queryString  = flag.String("query", "", "SQL query to run")
	serverMode   = flag.String("s", "", "server mode")
	clientMode   = flag.String("r", "", "remote client mode")
)

var (
	db database.Database
)

var (
	log *llog.Logger
)

var (
	total  = 0
	failed = 0
)

func init() {
	// Read flags and set user and hostname if not provided.
	flag.Parse()
	log = llog.New(*quietFlag, *debugFlag)

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

	log.Info.Println("Welcome " + *user + "@" + *hostname + ".")

	if *printVersion {
		fmt.Println("bashistdb v" + version)
		fmt.Println("https://github.com/andmarios/bashistdb")
		os.Exit(0)
	}
}

func main() {
	var err error
	db, err = database.New(*dbFile, log)
	if err != nil {
		log.Fatalln("Failed to load database:", err)
	}
	defer db.Close()

	stdinReader := bufio.NewReader(os.Stdin)
	stats, _ := os.Stdin.Stat()

	if *serverMode != "" {
		psock, err := net.Listen("tcp", ":5000")
		if err != nil {
			log.Fatalln(err)
		}
		for {
			conn, err := psock.Accept()
			if err != nil {
				log.Fatalln(err)
			}
			log.Info.Printf("Connection from %s.\n", conn.RemoteAddr())
			go remoteClient(conn)

		}
	} else if *clientMode != "" {
		if (stats.Mode() & os.ModeCharDevice) != os.ModeCharDevice {
			conn, err := net.Dial("tcp", *clientMode)

			history, err := ioutil.ReadAll(stdinReader)
			if err != nil {
				log.Fatalln(err)
			}
			fmt.Fprintf(conn, string(history))
			fmt.Fprintf(conn, code.TRANSMISSION_END+"\n")

			reply, _ := bufio.NewReader(conn).ReadString('\n')
			fmt.Println(reply)
			conn.Close()
		}
	} else if (stats.Mode() & os.ModeCharDevice) != os.ModeCharDevice {
		err = db.AddFromBuffer(stdinReader, *user, *hostname)
		if err != nil {
			log.Fatalln("Error while processing stdin:", err)
		}
	} else if *queryString == "" { // Print some stats
		res, err := db.Top20()
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println(res)
		res, err = db.Last20()
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println(res)
	}
}

func remoteClient(conn net.Conn) {
	db.AddFromBuffer(bufio.NewReader(conn), *user, *hostname)
	fmt.Fprint(conn, "Everything ok.\n")
	conn.Close()
}

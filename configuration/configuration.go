// Copyright (c) 2015, Marios Andreopoulos.
//
// This file is part of bashistdb.
//
//      Bashistdb is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
//      Bashistdb is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
//      You should have received a copy of the GNU General Public License
// along with bashistdb.  If not, see <http://www.gnu.org/licenses/>.

// Package configuration handles the configuration of bashistdb.
package configuration

import (
	"flag"
	"fmt"
	"log"
	"os"

	"projects.30ohm.com/mrsaccess/bashistdb/llog"
)

var (
	dbDefault    = os.Getenv("HOME") + "/.bashistdb.sqlite3"
	dbFile       = flag.String("db", dbDefault, "path to database file (will be created if not exists)")
	printVersion = flag.Bool("V", false, "print version and exit")
	verbosity    = flag.Int("v", 0, "Verbosity: 0 for silent, 1 for info, 2 for debug. Server mode will set it to 1 if left 0.")
	user         = flag.String("user", "", "optional user name to use instead of reading $USER variable")
	hostname     = flag.String("hostname", "", "optional hostname to use instead of reading $HOSTNAME variable")
	queryString  = flag.String("query", "", "SQL query to run")
	serverMode   = flag.Bool("s", false, "run in (network) server mode")
	clientMode   = flag.String("c", "", "run in (network) client mode, connect to address")
	port         = flag.String("port", "35628", "server port to listen on/connect to")
	passphrase   = flag.String("p", "", "passphrase to encrypt data with")
	restore      = flag.Bool("restore", false, "restores history data (prints to stdout, you may redirect it to your bash_history file), user and hostname act as wildcard surrounded search variables (% means all)")
)

var (
	Mode     int // mode of operation (local, server, client, etc)
	Function int // function (read, restore, et)
	Log      *llog.Logger
	Address  string
	User     string
	Hostname string
	DbFile   string
	Key      []byte
)

// Modes
const (
	_ = iota
	SERVER
	CLIENT
	LOCAL
	PRINT_VERSION // version flag overrides anything else
)

// Functions
const (
	_       = iota
	DEFAULT // Default tries to read from stdin or print some stats if it can't
	RESTORE
)

// Currently TRANSMISSION_END is unused.
// You should end the string below with \n
const TRANSMISSION_END = "END_OF_TRANSMISSION…»»»…\n"

func init() {
	// Read flags and set user and hostname if not provided.
	flag.Parse()

	// Determine mode of operation
	switch {
	case *printVersion:
		Mode = PRINT_VERSION
	case *serverMode:
		Mode = SERVER
		Address = ":" + *port
		if *clientMode != "" {
			fmt.Println("Incompatible options: server and client.")
			flag.PrintDefaults()
			os.Exit(1)
		}
	case *clientMode != "":
		Mode = CLIENT
		Address = *clientMode + ":" + *port
	default:
		Mode = LOCAL
	}

	switch {
	case *restore:
		Function = RESTORE
	default:
		Function = DEFAULT
	}

	if Mode == SERVER && Function != DEFAULT {
		fmt.Println("Incompatible options: asked for server mode and other functions.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if *serverMode && *verbosity == 0 {
		*verbosity = 1
	}
	Log = llog.New(*verbosity)

	if *user == "" {
		*user = os.Getenv("USER")
		if *user == "" {
			Log.Fatalln("Couldn't read username from $USER system variable and none was provided by -user flag.")
		}
	}
	User = *user

	var err error
	if *hostname == "" {
		*hostname, err = os.Hostname()
		if err != nil {
			Log.Fatalln("Couldn't read hostname from $HOSTNAME system variable and none was provided by -hostname flag:", err)
		}
	}
	Hostname = *hostname

	Log.Info.Println("Welcome " + *user + "@" + *hostname + ".")

	DbFile = *dbFile

	if Mode == SERVER || Mode == CLIENT {
		if *passphrase == "" {
			*passphrase = os.Getenv("BASHISTDB_KEY")
			if *passphrase == "" {
				log.Println("Using empty passphrase.")
			}
		}
		Key = []byte(*passphrase)
	}

}

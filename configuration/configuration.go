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
	dbDefault      = os.Getenv("HOME") + "/.bashistdb.sqlite3"
	userDefault    = os.Getenv("USER")
	hostDefault, _ = os.Hostname()
	dbFile         = flag.String("db", dbDefault,
		"Path to database file. It will be created if it doesn't exist.\n"+
			"       ")
	printVersion = flag.Bool("V", false,
		"Print version and exit.")
	verbosity = flag.Int("v", 0,
		"Verbosity level: 0 for silent, 1 for info, 2 for debug.\n"+
			"        Server mode will set it to 1 if left 0.")
	user = flag.String("user", userDefault,
		"Optional user name to use instead of reading $USER variable.\n"+
			"        In query operations, it doubles as search term for username. Think of\n"+
			"        it as wildcard surrounded: *<USER>*. Use percent (%) to match all.\n"+
			"       ")
	hostname = flag.String("hostname", hostDefault,
		"Optional hostname to use instead of reading it from the system.\n"+
			"        In query operations, it doubles as search term for hostname. Think of\n"+
			"        it as wildcard surrounded: *<HOST>*. Use percent (%) to match all.\n"+
			"       ")
	//	queryString  = flag.String("query", "", "SQL query to run")
	serverMode = flag.Bool("s", false,
		"Run in (network) server mode. Bashistdb currently binds to 0.0.0.0.")
	clientMode = flag.String("c", "",
		"Run in (network) client mode, connect to server address.")
	port = flag.String("port", "35628",
		"Server port to listen on/connect to.\n"+
			"       ")
	passphrase = flag.String("p", "",
		"Passphrase to encrypt network communications.")
	restore = flag.Bool("restore", false,
		"Restores history data (prints to stdout, you may redirect it to your bash_history file),\n"+
			"        user and hostname act as wildcard surrounded search variables (% means all)")
)

var (
	Mode      int // mode of operation (local, server, client, etc)
	Operation int // function (read, restore, et)
	Log       *llog.Logger
	Address   string
	User      string
	Hostname  string
	DbFile    string
	Key       []byte
)

// Output Formats
const (
	FORMAT_BASH_HISTORY = "restore"
	FORMAT_ALL          = "all"
	FORMAT_COMMAND      = "command"
	FORMAT_TIMESTAMP    = "timestamp"
	FORMAT_LOG          = "log"
	FORMAT_OP_DEFAULT   = FORMAT_COMMAND
)

// Format Strings
const (
	FORMAT_BASH_HISTORY_S = "#%d\n%s\n"
	FORMAT_ALL_S          = "%05d | %s | % 10s | % 10s | %s\n"
	FORMAT_COMMAND_S      = "%s\n"
	FORMAT_TIMESTAMP_S    = "%s: %s\n"
	FORMAT_LOG_S          = "%s, %s@%s, %s\n"
)

// Modes
const (
	_ = iota
	MODE_SERVER
	MODE_CLIENT
	MODE_LOCAL
	MODE_PRINT_VERSION // version flag overrides anything else
)

// Operations
const (
	_          = iota
	OP_DEFAULT // Default tries to read from stdin or print some stats if it can't
	OP_QUERY
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
		Mode = MODE_PRINT_VERSION
	case *serverMode:
		Mode = MODE_SERVER
		Address = ":" + *port
		if *clientMode != "" {
			fmt.Println("Incompatible options: server and client.")
			flag.PrintDefaults()
			os.Exit(1)
		}
	case *clientMode != "":
		Mode = MODE_CLIENT
		Address = *clientMode + ":" + *port
	default:
		Mode = MODE_LOCAL
	}

	switch {
	case *restore:
		Operation = OP_QUERY
	default:
		Operation = OP_DEFAULT
	}

	if Mode == MODE_SERVER && Operation != OP_DEFAULT {
		fmt.Println("Incompatible options: asked for server mode and other functions.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if *serverMode && *verbosity == 0 {
		*verbosity = 1
	}
	Log = llog.New(*verbosity)

	if *user == "" {
		Log.Fatalln("Couldn't read username from $USER system variable and none was provided by -user flag.")
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

	Log.Info.Println("Welcome " + User + "@" + Hostname + ".")

	DbFile = *dbFile

	if Mode == MODE_SERVER || Mode == MODE_CLIENT {
		if *passphrase == "" {
			*passphrase = os.Getenv("BASHISTDB_KEY")
			if *passphrase == "" {
				log.Println("Using empty passphrase.")
			}
		}
		Key = []byte(*passphrase)
	}

}

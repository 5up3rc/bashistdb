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
	"strings"

	"projects.30ohm.com/mrsaccess/bashistdb/llog"
)

// Exported fields are global settings.
var (
	Mode      int          // Mode of operation (local, server, client, etc)
	Operation int          // function (read, restore, et)
	Log       *llog.Logger // Log is the mail logger to log to
	Address   string       // Address is the remote server address
	User      string       // User is the username or username search term
	Hostname  string       // Hostname is the hostname or hostname search term
	Query     string       // Query is the command line search term
	DbFile    string       // DbFile is the filename of the sqlite database
	Key       []byte       // Key it the user passphrase to generate keys for net comms
	Format    string       // Format is the output format for the query results
)

// flagVars
var (
	dbFile       string
	printVersion bool
	verbosity    int
	user         string
	hostname     string
	query        string
	serverMode   bool
	clientMode   string
	port         string
	passphrase   string
	restore      bool
	format       string
	help         bool
)

// Output Formats
const (
	FORMAT_BASH_HISTORY = "restore"
	FORMAT_ALL          = "all"
	FORMAT_COMMAND_LINE = "command_line"
	FORMAT_TIMESTAMP    = "timestamp"
	FORMAT_LOG          = "log"
	FORMAT_JSON         = "json"
	FORMAT_OP_DEFAULT   = FORMAT_COMMAND_LINE
)

// Run Modes
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

// Configuration seems a bit messy but that's the way it is.
func init() {
	// Get some defaults from environment except encryption passphrase
	dbDefault := os.Getenv("HOME") + "/.bashistdb.sqlite3"
	userDefault := os.Getenv("USER")
	hostDefault, _ := os.Hostname()
	serverAddress := os.Getenv("BASHISTDB_SERVER")

	// flagVars are documentation!
	flag.StringVar(&dbFile, "db", dbDefault,
		"Path to database file. It will be created if it doesn't exist.\n"+
			"       ")
	flag.BoolVar(&printVersion, "V", false,
		"Print version and exit.")
	flag.IntVar(&verbosity, "v", 0, "Shorthand for -verbose")
	flag.IntVar(&verbosity, "verbose", 0,
		"Verbosity level: 0 for silent, 1 for info, 2 for debug.\n"+
			"        Server mode will set it to 1 if left 0.")
	flag.StringVar(&user, "u", userDefault, "Shorthand for -user")
	flag.StringVar(&user, "user", userDefault,
		"Optional user name to use instead of reading $USER variable.\n"+
			"        In query operations, it doubles as search term for the username. It accepts\n"+
			"        SQLite wildcard operators: percent (%) for asterisk (*) and underscore (_)\n"+
			`        for question mark (?). You may use backslash (\) to escape.`)
	flag.StringVar(&hostname, "H", hostDefault, "Shorthand for -host")
	flag.StringVar(&hostname, "host", hostDefault,
		"Optional hostname to use instead of reading it from the system.\n"+
			"        In query operations, it doubles as search term for the hostname. It accepts\n"+
			"        SQLite wildcard operators: percent (%) for asterisk (*) and underscore (_)\n"+
			`        for question mark (?). You may use backslash (\) to escape.`)
	flag.BoolVar(&serverMode, "s", false, "Shorthand for -server")
	flag.BoolVar(&serverMode, "server", false,
		"Run in (network) server mode. Bashistdb currently binds to 0.0.0.0.")
	flag.StringVar(&clientMode, "c", serverAddress, "Shorthand for -client")
	flag.StringVar(&clientMode, "client", serverAddress,
		"Run in (network) client mode, connect to server address.")
	flag.StringVar(&port, "p", "35628", "Shorthand for -port")
	flag.StringVar(&port, "port", "35628",
		"Server port to listen on/connect to.")
	flag.StringVar(&passphrase, "k", "", "Shorthand for -key")
	flag.StringVar(&passphrase, "key", "",
		"Passphrase to use for creating keys for network communication encryption.")
	flag.BoolVar(&restore, "restore", false,
		"Restores history data (prints to stdout, you may redirect it to your bash_history file),\n"+
			"        user and hostname act as wildcard surrounded search variables (% means all)")
	flag.StringVar(&format, "f", FORMAT_OP_DEFAULT, "Shorthand for -format")
	flag.StringVar(&format, "format", FORMAT_OP_DEFAULT,
		"How to format query output. Available types are:\n        "+
			FORMAT_ALL+", "+
			FORMAT_BASH_HISTORY+", "+
			FORMAT_COMMAND_LINE+", "+
			FORMAT_JSON+", "+
			FORMAT_LOG+", "+
			FORMAT_TIMESTAMP+"\n"+
			"        Format '"+FORMAT_BASH_HISTORY+"' can be used to restore your history file.")
	flag.BoolVar(&help, "h", false, "Shorthand for -help")
	flag.BoolVar(&help, "help", false, "This text")

	// Read flags and set user and hostname if not provided.
	flag.Parse()

	// Custom help text in order to document non-flag argument.
	if help {
		fmt.Println(`Usage of bashistdb; query or run in server mode:
  bashistdb [OPTIONS] [QUERY]
Import history:
  history | bashistdb [OPTIONS]

The query is run against the command lines only. Special flags exist for user and
hostname search. SQLite wildcard operators are percent (%) instead of asterisk (*)
and undercore (_) instead of question mark (?). You may use backslash (\) to escape.

Available options:`)
		flag.PrintDefaults()
		os.Exit(0)
	}

	// Determine run mode
	switch {
	case printVersion:
		Mode = MODE_PRINT_VERSION
	case serverMode:
		Mode = MODE_SERVER
		Address = ":" + port
		if clientMode != "" {
			fmt.Println("Incompatible options: server and client.")
			flag.PrintDefaults()
			os.Exit(1)
		}
	case clientMode != "":
		Mode = MODE_CLIENT
		Address = clientMode + ":" + port
	default:
		Mode = MODE_LOCAL
	}

	// Determine operation (used in local and client mode)
	switch {
	case restore:
		fallthrough
	case len(flag.Args()) > 0:
		Operation = OP_QUERY
	default:
		Operation = OP_DEFAULT
	}

	// Check mode-operation incompatibility
	if Mode == MODE_SERVER && Operation != OP_DEFAULT {
		fmt.Println("Incompatible options: asked for server mode and other functions.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Server mode has minimum verbosity of 1 (INFO)
	if serverMode && verbosity == 0 {
		verbosity = 1
	}
	// Create global logger
	Log = llog.New(verbosity)

	// Warn about username issues.
	if user == "" {
		Log.Info.Println("Couldn't read username from $USER system variable and none was provided by -user flag.")
	}
	User = user

	var err error
	if hostname == "" {
		Log.Info.Println("Couldn't read hostname from $HOSTNAME system variable and none was provided by -hostname flag:", err)
	}
	Hostname = hostname

	// Welcome message
	Log.Info.Println("Welcome " + User + "@" + Hostname + ".")

	Query = strings.Join(flag.Args(), " ")

	DbFile = dbFile

	// Passphrase may come for environment or flag
	if Mode == MODE_SERVER || Mode == MODE_CLIENT {
		if passphrase == "" {
			passphrase = os.Getenv("BASHISTDB_KEY")
			if passphrase == "" {
				log.Println("Using empty passphrase.")
			}
		}
		Key = []byte(passphrase)
	}
}

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
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
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
	Address   string       // Address is the remote server's address for client mode or server's address for server mode
	User      string       // User is the username or username search term
	Hostname  string       // Hostname is the hostname or hostname search term
	Query     string       // Query is the command line search term
	Database  string       // Database is the filename of the sqlite database
	Key       []byte       // Key it the user passphrase to generate keys for net comms
	Format    string       // Format is the output format for the query results
)

// Output Formats
const (
	FORMAT_BASH_HISTORY = "restore"
	FORMAT_ALL          = "all"
	FORMAT_COMMAND_LINE = "command_line"
	FORMAT_TIMESTAMP    = "timestamp"
	FORMAT_LOG          = "log"
	FORMAT_JSON         = "json"
	FORMAT_DEFAULT      = FORMAT_COMMAND_LINE
)

var availableFormats = map[string]bool{
	FORMAT_BASH_HISTORY: true,
	FORMAT_ALL:          true,
	FORMAT_COMMAND_LINE: true,
	FORMAT_TIMESTAMP:    true,
	FORMAT_LOG:          true,
	FORMAT_JSON:         true,
}

// Run Modes, you may only add entries at the end.
const (
	_ = iota
	MODE_SERVER
	MODE_CLIENT
	MODE_LOCAL
	MODE_PRINT_VERSION // version flag overrides anything else
)

// Operations, you may only add entries at the end.
const (
	_          = iota
	OP_DEFAULT // Default tries to read from stdin or print some stats if it can't
	OP_QUERY
)

const (
	bashistPort = "25625"
)

// exportFields is a struct used to export some
// configuration variables to JSON and then to a
// file
type exportFields struct {
	Database string
	Remote   string
	Port     string
	Key      string
}

// flagVars
var (
	database   string
	version    bool
	verbosity  int
	user       string
	hostname   string
	query      string
	server     bool
	remote     string
	port       string
	passphrase string
	format     string
	help       bool
	global     bool
	writeconf  bool
)

// Load some defaults from environment
var (
	databaseD   = os.Getenv("HOME") + "/.bashistdb.sqlite3"
	userD       = os.Getenv("USER")
	hostD, _    = os.Hostname()
	remoteD     = os.Getenv("BASHISTDB_REMOTE")
	portD       = os.Getenv("BASHISTDB_PORT")
	confFile    = os.Getenv("HOME") + "/.bashistdb.conf"
	passphraseD = os.Getenv("BASHISTDB_KEY")
)

// Configuration seems a bit messy but that's the way it is.
func init() {
	// Try to load settings from configuration file (if exists)
	loadedConf := false
	if c, err := ioutil.ReadFile(confFile); err == nil {
		e := &exportFields{}
		if err = json.Unmarshal(c, e); err == nil {
			if e.Database != "" {
				databaseD = e.Database
			}
			if e.Remote != "" {
				remoteD = e.Remote
			}
			if e.Port != "" {
				portD = e.Port
			}
			if e.Key != "" {
				passphraseD = e.Key
			}
			loadedConf = true
		}
	}

	// If port isn't set yet, set default port.
	if portD == "" {
		portD = bashistPort
	}

	// flagVars, we keep actual documentation separated
	flag.StringVar(&database, "db", databaseD, "Database file")
	flag.BoolVar(&version, "V", false, "Show version.")
	flag.IntVar(&verbosity, "v", 0, "verbosity level")
	flag.IntVar(&verbosity, "verbose", 0, "verbosity level")
	flag.StringVar(&user, "u", userD, "custom username")
	flag.StringVar(&user, "user", userD, "custom username")
	flag.StringVar(&hostname, "H", hostD, "custom hostname")
	flag.StringVar(&hostname, "host", hostD, " custom hostname")
	flag.BoolVar(&server, "s", false, "run as server")
	flag.BoolVar(&server, "server", false, "run as server")
	flag.StringVar(&remote, "r", remoteD, "run as client, connect to SERVER")
	flag.StringVar(&remote, "remote", remoteD, "run as client, connect to SERVER")
	flag.StringVar(&port, "p", portD, "port")
	flag.StringVar(&port, "port", portD, "port")
	flag.StringVar(&passphrase, "k", "", "passphrase")
	flag.StringVar(&passphrase, "key", "", "passphrase")
	flag.StringVar(&format, "f", FORMAT_DEFAULT, "query output format")
	flag.StringVar(&format, "format", FORMAT_DEFAULT, "query output format")
	flag.BoolVar(&help, "h", false, "help")
	flag.BoolVar(&help, "help", false, "help")
	flag.BoolVar(&global, "g", false, "global: '-user % -host %'")
	flag.BoolVar(&writeconf, "save", false, "write ~/.bashistdb.conf")

	flag.Parse()

	if help {
		printHelp()
		os.Exit(0)
	}

	// Determine run mode
	switch {
	case version:
		Mode = MODE_PRINT_VERSION
	case server:
		Mode = MODE_SERVER
		Address = ":" + port
	case remote != "":
		Mode = MODE_CLIENT
		Address = remote + ":" + port
	default:
		Mode = MODE_LOCAL
	}

	// Server mode incompatible with client mode.
	if Mode == MODE_SERVER && remote != "" {
		if remote != remoteD { // User may just set his BASHISTDB_REMOTE env var
			fmt.Println("Incompatible options: server and client.\n")
			printHelp()
			os.Exit(1)
		}
	}

	// Determine operation (used in local and client mode)
	switch {
	case len(flag.Args()) > 0: // We have non-flag arguments -> it is a query
		Operation = OP_QUERY
		if availableFormats[format] { // Query uses output format
			Format = format
		} else {
			Log.Info.Println("The specified format doesn't exist. Reverting to default:", FORMAT_DEFAULT)
			Format = FORMAT_DEFAULT
		}
	default: // Try to read from stdin or print some stats if you can't
		Operation = OP_DEFAULT
	}

	// Check mode-operation incompatibility
	if Mode == MODE_SERVER && Operation != OP_DEFAULT {
		fmt.Println("Incompatible options: asked for server mode and other functions.\n")
		printHelp()
		os.Exit(1)
	}

	// Server mode sets minimum verbosity of 1 (INFO)
	if Mode == MODE_SERVER && verbosity == 0 {
		verbosity = 1
	}
	// Verbosity reaches up to 2 (DEBUG)
	if verbosity > 2 {
		verbosity = 2
	}
	// Create global logger
	Log = llog.New(verbosity)

	// Protest about username issues.
	if user == "" {
		Log.Info.Println("Couldn't read username from $USER system variable and none was provided by -user flag.\n")
		printHelp()
		os.Exit(1)
	}
	User = user

	// Protest about hostname issues.
	if hostname == "" {
		Log.Info.Println("Couldn't get hostname from system and none was provided by -host flag.\n")
		printHelp()
		os.Exit(1)
	}
	Hostname = hostname

	// Check for global (search) flag
	if Operation == OP_QUERY && global {
		User, Hostname = "%", "%"
	}

	// Welcome message
	m := ""
	switch Mode {
	case MODE_SERVER:
		m = "server"
	case MODE_CLIENT:
		m = "client"
	case MODE_LOCAL:
		m = "local"
	}
	Log.Info.Println("Welcome " + User + "@" + Hostname + ". Bashistdb is in " + m + " mode.")
	Log.Debug.Println("Loaded some settings from environment. Configuration file and flags can override them.")
	if loadedConf {
		Log.Info.Println("Loaded some settings from ~/.bashistdbconf. Command line flags can override them.")
	}

	// Prepare query term. Join non-flag args and prefix-suffix with wildcard
	Query = strings.Join(flag.Args(), " ")
	Query = "%" + Query + "%" // Grep like behaviour

	// Set database filename
	Database = database

	// Passphrase may come from environment or flag
	if Mode == MODE_SERVER || Mode == MODE_CLIENT || writeconf {
		if passphrase == "" {
			passphrase = passphraseD
			if passphrase == "" {
				log.Println("Using empty passphrase.")
			}
		}
		Key = []byte(passphrase)
	}

	if writeconf {
		// Pretty print JSON instead of just Marshal
		conf := fmt.Sprintf(`{
"database": %#v,
"remote"  : %#v,
"port"    : %#v,
"key"     : %#v
}
`, Database, remote, port, string(Key))
		err := ioutil.WriteFile(confFile, []byte(conf), 0600)
		if err != nil {
			Log.Println(err)
		} else {
			Log.Info.Println("Wrote settings to ", confFile)
		}
	}

	Log.Debug.Printf("Database: %s, mode: %d, operation: %d, address: %s\n", Database, Mode, Operation, Address)
}

// printHelp prints custom help in order to be able to
// document non-flag argument and keep output nice
func printHelp() {
	fmt.Println("" + `Usage of bashistdb.
Query or run in server mode:
  bashistdb [OPTIONS] [QUERY]
Import history:
  history | bashistdb [OPTIONS]

The query is run against the command lines only. Special flags exist for user
and hostname search. SQLite wildcard operators are percent (%) instead of
asterisk (*) and undercore (_) instead of question mark (?). You may use
backslash (\) to escape. The query term always run with both a wildcard prefix
and suffix. Think of it as grep.

Available options:
    -db FILE
       Path to database file. It will be created if it doesn't exist.
       Current: ` + databaseD + `
    -V     Print version info and exit.
    -v , -verbose LEVEL
       Verbosity level: 0 for silent, 1 for info, 2 for debug.
       In server mode it is set to 1 if left 0.
    -u, -user USER
       Optional user name to use instead of reading $USER variable. In query
       operations it doubles as search term for the username. Wildcard
       operators (%, _) work but unlike query we search for the exact term.
       Current: ` + userD + `
    -H, -host HOST
       Optional hostname to use instead of reading it from the system. In query
       operations, it doubles as search term for the hostname. Wildcard
       operators (%, _) work but unlike query we search for the exact term.
       Current: ` + hostD + `
    -g     Sets user and host to % for query operation. (equiv: -user % -host %)
    -s, -server    Run in server mode. Bashistdb currently binds to 0.0.0.0.
    -r, -remote SERVER_ADDRESS
       Run in network client mode, connect to server address. You may also set
       this with the BASHISTDB_REMOTE env variable. Current: ` + remoteD + `
    -p, -port PORT
	Server port to listen on/connect to. You may also set this with the
        BASHISTDB_PORT env variable. Current: ` + portD + `
    -k, -key PASSPHRASE
	Passphrase to use for creating keys to encrypt network communications.
        You may also set it via the BASHISTDB_KEY env variable.
    -f, --format FORMAT
 	How to format query output. Available types are:
	` + FORMAT_ALL + ", " + FORMAT_BASH_HISTORY + ", " +
		FORMAT_COMMAND_LINE + ", " + FORMAT_JSON + ", " +
		FORMAT_LOG + ", " + FORMAT_TIMESTAMP + `
        Format '` + FORMAT_BASH_HISTORY + `' can be used to restore your history file.
        Default: ` + FORMAT_DEFAULT + `
    -save    Write some settings (database, remote, port, key) to configuration
        file: ` + confFile + `. These settings override environment variables.
    -h, --help    This text.`)
}

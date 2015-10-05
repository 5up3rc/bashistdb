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

	"github.com/andmarios/bashistdb/llog"
)

// Exported fields are global settings.
var (
	Mode      int          // Mode of operation (local, server, client, etc)
	Operation int          // function (read, restore, et)
	Log       *llog.Logger // Log is the mail logger to log to
	Address   string       // Address is the remote server's address for client mode or server's address for server mode
	Database  string       // Database is the filename of the sqlite database
	Key       []byte       // Key it the user passphrase to generate keys for net comms
	User      string       // User is the username detected or explicitly set
	Hostname  string       // Hostname is the hostname detected or explicitly set
	QParams   QueryParams  // Parameters to query
)

// Output Formats
const (
	FORMAT_BASH_HISTORY = "restore"
	FORMAT_ALL          = "all"
	FORMAT_COMMAND_LINE = "command_line"
	FORMAT_TIMESTAMP    = "timestamp"
	FORMAT_LOG          = "log"
	FORMAT_JSON         = "json"
	FORMAT_EXPORT       = "export"
	FORMAT_DEFAULT      = FORMAT_COMMAND_LINE
)

var availableFormats = map[string]bool{
	FORMAT_BASH_HISTORY: true,
	FORMAT_ALL:          true,
	FORMAT_COMMAND_LINE: true,
	FORMAT_TIMESTAMP:    true,
	FORMAT_LOG:          true,
	FORMAT_JSON:         true,
	FORMAT_EXPORT:       true,
}

// Run Modes, you may only add entries at the end.
// If many are set, precedence should be PRINT_VERSION > INIT > SERVER > CLIENT > LOCAL
// It is ok that we use ints because these are not communicated between client and server.
const (
	_ = iota
	MODE_SERVER
	MODE_CLIENT
	MODE_LOCAL
	MODE_PRINT_VERSION // version flag overrides anything else
	MODE_INIT
)

// Operations, you may only add entries at the end.
const (
	_         = iota
	OP_IMPORT // Import history from stdin
	OP_QUERY  // Run a query
	OP_STATS  // Run some default stat queries (runs when no args are given)
	OP_DELETE // Delete lines from history (to be implemented)
)

// A QueryParams contains parameters that are used to run a query.
// Depending on query type, some fields may not be used.
type QueryParams struct {
	Type    string // Query type
	Kappa   int    // If topk or lastk, we store k here
	User    string // Search User
	Host    string // Search Host
	Format  string // Return format
	Command string // Search Term for command line field
	Unique  bool   // Return unique command lines
}

// Available query types
const (
	QUERY         = "query"   // A normal search (grep)
	QUERY_LASTK   = "lastk"   // K most recent commands
	QUERY_TOPK    = "topk"    // K most used commands
	QUERY_USERS   = "users"   // users@host in database
	QUERY_CLIENTS = "clients" // unique clients connected
	QUERY_DEMO    = "demo"    // Run some demo queries
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

// Load some defaults from environment and some basic settings
var (
	databaseEnv   = os.Getenv("HOME") + "/.bashistdb.sqlite3"
	userEnv       = os.Getenv("USER")
	hostEnv, _    = os.Hostname()
	remoteEnv     = os.Getenv("BASHISTDB_REMOTE")
	portEnv       = os.Getenv("BASHISTDB_PORT")
	passphraseEnv = os.Getenv("BASHISTDB_KEY")
	// Vars below can not be overriden by user
	confFile      = os.Getenv("HOME") + "/.bashistdb.conf"
	foundConfFile = false
)

// flag variables
// Vars ending in Set are bool
// For non-bool vars we may create a bool counterpart
// using flag.Visit
var (
	// These are used as actual flagvars
	database     string
	versionSet   bool
	verbosity    int
	user         string
	host         string
	serverSet    bool
	remote       string
	port         string
	passphrase   string
	format       string
	helpSet      bool
	globalSet    bool
	writeconfSet bool
	setupSet     bool
	topk         int
	lastk        int
	localSet     bool
	uniqueSet    bool
	usersSet     bool
	// Here we will store the non flag arguments
	query string
	// These are not parsed from flags but we set them with flag.Visit
	userSet       = false
	hostSet       = false
	remoteSet     = false
	portSet       = false
	passphraseSet = false
	formatSet     = false
	topkSet       = false
	lastkSet      = false
	// These are set with manual searches
	querySet = false
	stdinSet = false
)

// Set visited flags so we may have boolean expression criteria
func setVisitedFlags(f *flag.Flag) {
	switch f.Name {
	case "u", "user":
		userSet = true
	case "H", "host":
		hostSet = true
	case "r", "remote":
		remoteSet = true
	case "p", "port":
		portSet = true
	case "k", "key":
		passphraseSet = true
	case "f", "format":
		formatSet = true
	case "topk":
		topkSet = true
	case "lastk":
		lastkSet = true
	}
}

// Read configuration file, overrides environment variables.
func readConfFile() {
	// Try to load settings from configuration file (if exists)
	if c, err := ioutil.ReadFile(confFile); err == nil {
		e := &exportFields{}
		if err = json.Unmarshal(c, e); err == nil {
			if e.Database != "" {
				databaseEnv = e.Database
			}
			if e.Remote != "" {
				remoteEnv = e.Remote
			}
			if e.Port != "" {
				portEnv = e.Port
			}
			if e.Key != "" {
				passphraseEnv = e.Key
			}
			foundConfFile = true
		} else {
			log.Fatalln("Could not parse configuration file:",
				err.Error())
		}
	}
}

// Configuration seems a bit messy but that's the way it is.
func init() {
	readConfFile()

	// If port isn't set yet, set default port.
	if portEnv == "" {
		portEnv = bashistPort
	}

	// flagVars, we keep actual documentation separated
	flag.StringVar(&database, "db", databaseEnv, "Database file")
	flag.BoolVar(&versionSet, "V", false, "Show version.")
	flag.IntVar(&verbosity, "v", 0, "verbosity level")
	flag.IntVar(&verbosity, "verbose", 0, "verbosity level")
	flag.StringVar(&user, "u", userEnv, "custom username")
	flag.StringVar(&user, "user", userEnv, "custom username")
	flag.StringVar(&host, "H", hostEnv, "custom hostname")
	flag.StringVar(&host, "host", hostEnv, " custom hostname")
	flag.BoolVar(&serverSet, "s", false, "run as server")
	flag.BoolVar(&serverSet, "server", false, "run as server")
	flag.StringVar(&remote, "r", remoteEnv, "run as client, connect to SERVER")
	flag.StringVar(&remote, "remote", remoteEnv, "run as client, connect to SERVER")
	flag.StringVar(&port, "p", portEnv, "port")
	flag.StringVar(&port, "port", portEnv, "port")
	flag.StringVar(&passphrase, "k", "", "passphrase")
	flag.StringVar(&passphrase, "key", "", "passphrase")
	flag.StringVar(&format, "f", FORMAT_DEFAULT, "query output format")
	flag.StringVar(&format, "format", FORMAT_DEFAULT, "query output format")
	flag.BoolVar(&helpSet, "h", false, "help")
	flag.BoolVar(&helpSet, "help", false, "help")
	flag.BoolVar(&globalSet, "g", false, "global: '-user % -host %'")
	flag.BoolVar(&writeconfSet, "save", false, "write ~/.bashistdb.conf")
	flag.BoolVar(&setupSet, "init", false, "set-up system to use bashistdb")
	flag.BoolVar(&uniqueSet, "unique", false, "show unique (distinct) command lines")
	flag.IntVar(&topk, "topk", 20, "return K most used command lines")
	flag.IntVar(&lastk, "lastk", 20, "return K most recent command lines")
	flag.BoolVar(&usersSet, "users", false, "show users in database")
	flag.BoolVar(&localSet, "local", false, "force local mode")

	flag.Parse()

	flag.Visit(setVisitedFlags)

	if helpSet {
		printHelp()
		os.Exit(0)
	}

	// Determine run mode. A run mode is expected to run and then bashistdb toexit.
	switch { // Cases are in precedence order
	case setupSet:
		Mode = MODE_INIT
	case versionSet:
		Mode = MODE_PRINT_VERSION
	case serverSet:
		Mode = MODE_SERVER
		Address = ":" + port
	case remote != "" && !localSet:
		Mode = MODE_CLIENT
		Address = remote + ":" + port
	default:
		Mode = MODE_LOCAL
	}

	// Server mode incompatible with client mode.
	if Mode == MODE_SERVER && remoteSet { // User may just set his BASHISTDB_REMOTE env var
		fmt.Printf("Incompatible options: server and client.\n\n")
		printHelp()
		os.Exit(1)
	}

	// Detect if there is a QUERY in the command line (that is non-flag arguments)
	if len(flag.Args()) > 0 {
		querySet = true
	}

	// Detect if there are data coming from stdin:
	stats, _ := os.Stdin.Stat()
	if (stats.Mode() & os.ModeCharDevice) != os.ModeCharDevice {
		stdinSet = true
	}

	// Determine operation (used in local and client mode)
	switch {
	case topkSet:
		Operation = OP_QUERY
		QParams.Type = QUERY_TOPK
		QParams.Kappa = topk
	case lastkSet:
		Operation = OP_QUERY
		QParams.Type = QUERY_LASTK
		QParams.Kappa = lastk
	case usersSet:
		Operation = OP_QUERY
		QParams.Type = QUERY_USERS
	case querySet: // We have non-flag arguments -> it is a query
		Operation = OP_QUERY
		QParams.Type = QUERY
	case topkSet, lastkSet:
		Operation = OP_QUERY
	case stdinSet:
		Operation = OP_IMPORT
	default: // Demo mode
		Operation = OP_QUERY
		QParams.Type = QUERY_DEMO
	}

	if Operation == OP_QUERY {
		if availableFormats[format] { // Query uses output format
			QParams.Format = format
		} else {
			Log.Info.Println("The specified format doesn't exist. Reverting to default:", FORMAT_DEFAULT)
			QParams.Format = FORMAT_DEFAULT
		}
		switch uniqueSet {
		case true:
			QParams.Unique = true
		default:
			QParams.Unique = false
		}
	}

	// Check mode-operation incompatibility
	if Mode == MODE_SERVER && QParams.Type != QUERY_DEMO {
		fmt.Printf("Incompatible options: asked for server mode and other functions.\n\n")
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
		Log.Info.Printf("Couldn't read username from $USER system variable and none was provided by -user flag.\n\n")
		printHelp()
		os.Exit(1)
	}
	User = user // TODO: remove
	QParams.User = user
	if usersSet && !userSet { // simple users query should be global
		QParams.User = "%"
	}

	// Protest about hostname issues.
	if host == "" {
		Log.Info.Printf("Couldn't get hostname from system and none was provided by -host flag.\n\n")
		printHelp()
		os.Exit(1)
	}
	Hostname = host // TODO: remove
	QParams.Host = host

	// Check for global (search) flag
	if Operation == OP_QUERY && globalSet {
		// User, Hostname = "%", "%" // TODO: remove
		QParams.User, QParams.Host = "%", "%"
	}
	if usersSet && !hostSet { // simple users query should be global
		QParams.Host = "%"
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
	if foundConfFile {
		Log.Info.Println("Loaded some settings from ~/.bashistdbconf. Command line flags can override them.")
	}

	// Prepare query term. Join non-flag args and prefix-suffix with wildcard
	QParams.Command = "%" + strings.Join(flag.Args(), " ") + "%" // Grep like behaviour

	if Operation == OP_QUERY {
		Log.Info.Printf("Your query parameters are user: %s, host: %s, command line: %s.\n", QParams.User, QParams.Host, QParams.Command)
	}

	// Set database filename
	Database = database

	// When we setup the system, we should also save settings
	if setupSet {
		writeconfSet = true
	}

	// Passphrase may come from environment or flag
	if Mode == MODE_SERVER || Mode == MODE_CLIENT || writeconfSet {
		if passphrase == "" {
			passphrase = passphraseEnv
			if passphrase == "" {
				log.Println("Using empty passphrase.")
			}
		}
		Key = []byte(passphrase)
	}

	if writeconfSet {
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
       Current: ` + databaseEnv + `
    -V     Print version info and exit.
    -v , -verbose LEVEL
       Verbosity level: 0 for silent, 1 for info, 2 for debug.
       In server mode it is set to 1 if left 0.
    -u, -user USER
       Optional user name to use instead of reading $USER variable. In query
       operations it doubles as search term for the username. Wildcard
       operators (%, _) work but unlike query we search for the exact term.
       Current: ` + userEnv + `
    -H, -host HOST
       Optional hostname to use instead of reading it from the system. In query
       operations, it doubles as search term for the hostname. Wildcard
       operators (%, _) work but unlike query we search for the exact term.
       Current: ` + hostEnv + `
    -g     Sets user and host to % for query operation. (equiv: -user % -host %)
    -unique    If the query type permits, return unique results for the command
       line field (returns the most recent execution of each command).
    -lastk K
       Return the K most recent commands for the set user and host. If you add
       a query term it will return the K most recent commands that include it.
    -topk K
       Return the K most frequent commands for the set user and host. If you add
       a query term it will return the K most frequent commands that include it.
    -users    Return the users in the database. You may use search criteria, eg
      to find users who run a certain commands. By default this option searches
      across all users and host unless you explicitly set them via flags.
    -s, -server    Run in server mode. Bashistdb currently binds to 0.0.0.0.
    -r, -remote SERVER_ADDRESS
       Run in network client mode, connect to server address. You may also set
       this with the BASHISTDB_REMOTE env variable. Current: ` + remoteEnv + `
    -p, -port PORT
       Server port to listen on/connect to. You may also set this with the
       BASHISTDB_PORT env variable. Current: ` + portEnv + `
    -k, -key PASSPHRASE
       Passphrase to use for creating keys to encrypt network communications.
       You may also set it via the BASHISTDB_KEY env variable.
    -f, --format FORMAT
       How to format query output. Available types are:
      ` + FORMAT_ALL + ", " + FORMAT_BASH_HISTORY + ", " +
		FORMAT_COMMAND_LINE + ", " + FORMAT_JSON + ", " +
		FORMAT_LOG + ", " + FORMAT_TIMESTAMP + ", " + FORMAT_EXPORT + `
       Format '` + FORMAT_BASH_HISTORY + `' can be used to restore your history file.
       Format '` + FORMAT_EXPORT + `' can be used to pipe your history to another
       instance of bashistdb, while retaining user and host of each command.
       Default: ` + FORMAT_DEFAULT + `
    -save    Write some settings (database, remote, port, key) to configuration
       file: ` + confFile + `. These settings override environment variables.
    -h, --help    This text.
    -init    Setup system for bashistdb: (1) Save settings to file. (2) Add to
       bashrc functions to timestamp history and sent each command to bashistdb
       (remote or local, taken from settings), (3) add a unique serial timestamp
       to any untimestamped line in your bash_history.
    -local   Force local [db] mode, despite remote mode being set by env or conf.`)
}

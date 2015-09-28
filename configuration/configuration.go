// Package configuration handles the configuration of bashistdb.
package configuration

import (
	"flag"
	"fmt"
	"os"

	"projects.30ohm.com/mrsaccess/bashistdb/llog"
)

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
	Mode     int
	Log      *llog.Logger
	Address  string
	User     string
	Hostname string
	DbFile   string
)

const (
	SERVER = iota
	CLIENT
	QUERY
)

func init() {
	// Read flags and set user and hostname if not provided.
	flag.Parse()
	Log = llog.New(*quietFlag, *debugFlag)

	if *user == "" {
		*user = os.Getenv("USER")
	}
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

	Log.Info.Println("Welcome " + *user + "@" + *hostname + ".")

	if *printVersion {
		//	fmt.Println("bashistdb v" + version)
		fmt.Println("https://github.com/andmarios/bashistdb")
		os.Exit(0)
	}

	DbFile = *dbFile

	switch {
	case *serverMode != "":
		Mode = SERVER
		Address = *serverMode
	case *clientMode != "":
		Mode = CLIENT
		Address = *clientMode
	}
}

// 	if *serverMode != "" {
// 		var err error
// 		db, err = database.New(*dbFile, log)
// 		if err != nil {
// 			log.Fatalln("Failed to load database:", err)
// 		}
// 		defer db.Close()

// 		err = network.ServerMode(*serverMode, db, log)
// 		if err != nil {
// 			log.Fatalln(err)
// 		}
// 		os.Exit(0)
// 	} else if *clientMode != "" {
// 		err := network.ClientMode(*clientMode, log)
// 		if err != nil {
// 			log.Fatalln(err)
// 		}
// 		os.Exit(0)
// 	} else {
// 		var err error
// 		db, err = database.New(*dbFile, log)
// 		if err != nil {
// 			log.Fatalln("Failed to load database:", err)
// 		}
// 		defer db.Close()

// 		stdinReader := bufio.NewReader(os.Stdin)
// 		stats, _ := os.Stdin.Stat()

// 		if (stats.Mode() & os.ModeCharDevice) != os.ModeCharDevice {
// 			err = db.AddFromBuffer(stdinReader, *user, *hostname)
// 			if err != nil {
// 				log.Fatalln("Error while processing stdin:", err)
// 			}
// 		} else if *queryString == "" { // Print some stats
// 			res, err := db.Top20()
// 			if err != nil {
// 				log.Fatalln(err)
// 			}
// 			fmt.Println(res)
// 			res, err = db.Last20()
// 			if err != nil {
// 				log.Fatalln(err)
// 			}
// 			fmt.Println(res)
// 		}
// 	}
// }

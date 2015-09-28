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
	"fmt"
	"os"

	conf "projects.30ohm.com/mrsaccess/bashistdb/configuration"
	"projects.30ohm.com/mrsaccess/bashistdb/database"
	"projects.30ohm.com/mrsaccess/bashistdb/llog"
	"projects.30ohm.com/mrsaccess/bashistdb/network"
)

// Golang's RFC3339 does not comply with all RFC3339 representations
const RFC3339alt = "2006-01-02T15:04:05-0700"

var log *llog.Logger

func main() {
	log = conf.Log

	switch conf.Mode {
	case conf.SERVER:
		err := network.ServerMode()
		if err != nil {
			log.Fatalln(err)
		}
		os.Exit(0)
	case conf.CLIENT:
		err := network.ClientMode()
		if err != nil {
			log.Fatalln(err)
		}
		os.Exit(0)
	default:
		db, err := database.New()
		if err != nil {
			log.Fatalln("Failed to load database:", err)
		}
		defer db.Close()

		stdinReader := bufio.NewReader(os.Stdin)
		stats, _ := os.Stdin.Stat()

		if (stats.Mode() & os.ModeCharDevice) != os.ModeCharDevice {
			err = db.AddFromBuffer(stdinReader, conf.User, conf.Hostname)
			if err != nil {
				log.Fatalln("Error while processing stdin:", err)
			}
		} else {
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
}

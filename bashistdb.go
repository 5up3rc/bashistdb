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

package main

import (
	"fmt"
	"os"

	conf "github.com/andmarios/bashistdb/configuration"
	"github.com/andmarios/bashistdb/llog"
	"github.com/andmarios/bashistdb/local"
	"github.com/andmarios/bashistdb/network"
	"github.com/andmarios/bashistdb/setup"
	"github.com/andmarios/bashistdb/version"
)

var log *llog.Logger
var Version = version.Version // a debug build will append pprof to this

func main() {
	log = conf.Log

	switch conf.Mode {
	case conf.MODE_PRINT_VERSION:
		fmt.Println("bashistdb v" + Version)
		fmt.Println("https://github.com/andmarios/bashistdb")
	case conf.MODE_SERVER:
		if err := network.ServerMode(); err != nil {
			log.Fatalln(err)
		}
	case conf.MODE_CLIENT:
		if err := network.ClientMode(); err != nil {
			log.Fatalln(err)
		}
	case conf.MODE_LOCAL:
		if err := local.Run(); err != nil {
			log.Fatalln(err)
		}
	case conf.MODE_INIT:
		if err := setup.Apply(true); err != nil {
			log.Fatalln(err)
		}
	case conf.MODE_ERROR:
		fmt.Printf("%s\n\n", conf.Error)
		conf.PrintHelp()
		os.Exit(1)
	case conf.MODE_HELP:
		conf.PrintHelp()
	}
}

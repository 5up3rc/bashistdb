// Copyright (c) 2015, Marios Andreopoulos.
//
// This file is part of bashistdb.
//
// 	Foobar is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// 	Foobar is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// 	You should have received a copy of the GNU General Public License
// along with Foobar.  If not, see <http://www.gnu.org/licenses/>.

/*
Command bashistdb stores and retrieves bash history into/from a sqlite3
database.
*/
package llog

import (
	"io/ioutil"
	log "log"
	"os"
)

type Logger struct {
	*log.Logger
	Info  *log.Logger
	Debug *log.Logger
}

func New(quiet, debug bool) *Logger {
	var deb, inf *log.Logger
	if debug {
		quiet = false
	}
	if debug {
		deb = log.New(os.Stderr, "", log.Ldate|log.Ltime|
			log.Lshortfile)
		deb.Println("Debug enabled.")
	} else {
		deb = log.New(ioutil.Discard, "", 0)
	}
	if !quiet {
		inf = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile)
	} else {
		inf = log.New(ioutil.Discard, "", 0)
	}

	std := log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile)
	return &Logger{std, inf, deb}
}

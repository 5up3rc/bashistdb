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
	"io"
	"io/ioutil"
	log "log"
	"os"
)

const (
	SILENT = iota
	INFO
	DEBUG
)

type Logger struct {
	*log.Logger
	Info  *log.Logger
	Debug *log.Logger
}

func New(verbosity int) *Logger {
	var deb, inf *log.Logger
	var debOut, infOut io.Writer
	debMod := log.Ldate | log.Ltime
	infMod := log.Ldate | log.Ltime

	switch verbosity {
	case SILENT:
		infOut = ioutil.Discard
		debOut = ioutil.Discard
	case INFO:
		infOut = os.Stderr
		debOut = ioutil.Discard
	case DEBUG:
		infOut = os.Stderr
		debOut = os.Stderr
		debMod = log.Ldate | log.Ltime | log.Lshortfile
		infMod = log.Ldate | log.Ltime | log.Lshortfile
	default:
		infOut = os.Stderr
		debOut = ioutil.Discard
	}

	// std is used for logging fatal errors
	std := log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile)

	// inf is used for logging info messages
	inf = log.New(infOut, "", infMod)

	// std is used for logging debug messages
	deb = log.New(debOut, "", debMod)
	deb.Println("Debug enabled.")

	return &Logger{std, inf, deb}
}

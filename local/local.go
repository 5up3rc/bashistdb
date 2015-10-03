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

// Package local manages the local operation of bashistdb.
package local

import (
	"bufio"
	"errors"
	"fmt"
	"os"

	conf "github.com/andmarios/bashistdb/configuration"
	"github.com/andmarios/bashistdb/database"
	"github.com/andmarios/bashistdb/llog"
)

var log *llog.Logger

func Run() error {
	db, err := database.New()
	if err != nil {
		return errors.New("Failed to load database: " + err.Error())
	}
	defer db.Close()

	log = conf.Log

	switch conf.Operation {
	case conf.OP_IMPORT:
		r := bufio.NewReader(os.Stdin)
		stats, err := db.AddFromBuffer(r, conf.User, conf.Hostname)
		if err != nil {
			return errors.New("Error while processing stdin: " +
				err.Error())
		}
		// We print to log because we usually want this to be quiet
		// as we may run it every time we hit ENTER in a bash prompt.
		log.Info.Println(stats)
	case conf.OP_STATS:
		res, err := db.TopK(conf.QueryParams{User: "%", Host: "%", Command: "%", Kappa: 20})
		if err != nil {
			return err
		}
		fmt.Println(string(res))
		res, err = db.LastK(conf.QueryParams{User: "%", Host: "%", Command: "%", Kappa: 10})
		if err != nil {
			return err
		}
		fmt.Println(string(res))
	case conf.OP_QUERY:
		res, err := db.RunQuery(conf.QParams)
		if err != nil {
			return err
		}
		fmt.Println(string(res))
	}
	return nil
}

// DEPRECATED
// GetStdin checks if stdin is a unix character device,
// that is if data is piped in to us. If yes it returns
// a reader for stdin, else it returns an error.
// func GetStdin() (r *bufio.Reader, e error) {
// 	r = bufio.NewReader(os.Stdin)
// 	stats, _ := os.Stdin.Stat()
// 	if (stats.Mode() & os.ModeCharDevice) != os.ModeCharDevice {
// 		return r, nil
// 	}
// 	return r, errors.New("Stdin is not character device.")
// }

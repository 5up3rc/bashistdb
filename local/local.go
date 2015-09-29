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

	conf "projects.30ohm.com/mrsaccess/bashistdb/configuration"
	"projects.30ohm.com/mrsaccess/bashistdb/database"
)

func Run() error {
	db, err := database.New()
	if err != nil {
		return errors.New("Failed to load database: " + err.Error())
	}
	defer db.Close()

	switch conf.Operation {
	case conf.OP_DEFAULT:
		stdinReader := bufio.NewReader(os.Stdin)
		stats, _ := os.Stdin.Stat()
		if (stats.Mode() & os.ModeCharDevice) != os.ModeCharDevice {
			err = db.AddFromBuffer(stdinReader, conf.User, conf.Hostname)
			if err != nil {
				return errors.New("Error while processing stdin: " + err.Error())
			}
		} else {
			res, err := db.Top20()
			if err != nil {
				return err
			}
			fmt.Println(res)
			res, err = db.Last20()
			if err != nil {
				return err
			}
			fmt.Println(res)
		}
	case conf.OP_QUERY:
		res, err := db.Restore(conf.User, conf.Hostname)
		if err != nil {
			return err
		}
		fmt.Println(res)
	}
	return nil
}

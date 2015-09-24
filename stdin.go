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
// along with Foobar.  If not, see <http://www.gnu.org/licenses/>.package main

package main

import (
	"bufio"
	"io"
	"regexp"
	"strings"
	"time"
)

func readFromStdin(r *bufio.Reader) error {
	//                                  LINENUM        DATETIME         CM
	parseLine := regexp.MustCompile(`^ *[0-9]+\*? *([0-9T:+-]{24,24}) *(.*)`)
	for {
		historyLine, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			} else {
				return err
			}
		}
		args := parseLine.FindStringSubmatch(historyLine)
		if len(args) != 3 {
			info.Println("Could't decode line. Skipping:", historyLine)
			continue
		}
		time, err := time.Parse(RFC3339alt, args[1])
		if err != nil {
			return err
		}
		err = submitRecord(*user, *hostname, strings.TrimSuffix(args[2], "\n"), time)
		if err != nil {
			return err
		}
	}
}

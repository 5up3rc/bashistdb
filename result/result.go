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

// Package result handles the output formatting for bashistdb.
package result

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	conf "projects.30ohm.com/mrsaccess/bashistdb/configuration"
)

// Format Strings
const (
	FORMAT_BASH_HISTORY_S = "#%d\n%s"
	FORMAT_ALL_S          = "%05d | %s | % 10s | % 10s | %s"
	FORMAT_COMMAND_LINE_S = "%s"
	FORMAT_TIMESTAMP_S    = "%s: %s"
	FORMAT_LOG_S          = "%s, %s@%s, %s"
	FORMAT_JSON_S         = "" // We use encoding/json for JSON
)

// A Result is used to store the formatted output of a query.
// Result's methods are responsible for formatting according to
// requested output format.
type Result struct {
	out     *bytes.Buffer
	written bool // we use this to work around json not accepting a trailing comma
}

// New returns a new Result
func New() *Result {
	var out bytes.Buffer
	if conf.Format == conf.FORMAT_JSON {
		out.WriteString("[\n")
	}
	return &Result{&out, false}
}

// A rowJson is an internal struct to use with json.Marshal
type rowJson struct {
	Row                           int
	Datetime, User, Host, Command string
}

// AddRow adds a query row to a Result struct. This function is not thread safe!
func (r Result) AddRow(row int, datetime time.Time, user, host string, command string) {
	var f string

	switch conf.Format {
	case conf.FORMAT_ALL:
		f = fmt.Sprintf(FORMAT_ALL_S, row, datetime, user, host, command)
	case conf.FORMAT_BASH_HISTORY:
		f = fmt.Sprintf(FORMAT_BASH_HISTORY_S, datetime.Unix(), command)
	case conf.FORMAT_TIMESTAMP:
		f = fmt.Sprintf(FORMAT_TIMESTAMP_S, datetime, command)
	case conf.FORMAT_LOG:
		f = fmt.Sprintf(FORMAT_LOG_S, datetime, user, host, command)
	case conf.FORMAT_JSON:
		if r.written {
			_ = r.out.WriteByte(',')
		}
		r.written = true
		b, _ := json.Marshal(rowJson{row, datetime.Format("RFC3339"), user, host, command})
		_, _ = r.out.Write(b)
		f = ""
	case conf.FORMAT_COMMAND_LINE:
		fallthrough
	default:
		f = fmt.Sprintf(FORMAT_COMMAND_LINE_S, command)

	}
	r.out.WriteString(f + "\n")
}

// Formatted returns the result in the desired format after performing any necessary adjustment.
func (r Result) Formatted() []byte {
	if conf.Format == conf.FORMAT_JSON {
		r.out.WriteByte(']')
	}
	return r.out.Bytes()
}

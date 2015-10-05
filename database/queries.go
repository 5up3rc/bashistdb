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
Package database handles a SQLite3 database and access methods for the
specific needs of bashistdb.
*/
package database

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"time"

	conf "github.com/andmarios/bashistdb/configuration"
	"github.com/andmarios/bashistdb/result"
)

// TopK returns the k most frequent command lines in history
func (d Database) TopK(qp conf.QueryParams) ([]byte, error) {
	rows, err := d.Query(`SELECT command, count(*) as count FROM history
                               WHERE user LIKE ? AND host LIKE ? AND command LIKE ? ESCAPE '\'
                               GROUP BY command ORDER BY count DESC LIMIT ?`,
		qp.User, qp.Host, qp.Command, qp.Kappa)
	if err != nil {
		return []byte{}, err
	}
	defer rows.Close()

	res := result.New("")
	for rows.Next() {
		var command string
		var count int
		rows.Scan(&command, &count)
		res.AddCountRow(count, command)
	}
	return res.Formatted(), err
}

// LastK returns the k most recent command lines in history
func (d Database) LastK(qp conf.QueryParams) ([]byte, error) {
	var rows *sql.Rows
	var err error
	switch qp.Unique {
	case true:
		rows, err = d.Query(`SELECT * FROM
                                      (SELECT rowid, * FROM history
                                         WHERE user LIKE ? AND host LIKE ? AND command LIKE ? ESCAPE '\'
                                         GROUP BY command
                                         ORDER BY datetime DESC LIMIT ?)
                                      ORDER BY datetime ASC`,
			qp.User, qp.Host, qp.Command, qp.Kappa)
	default:
		rows, err = d.Query(`SELECT * FROM
                                      (SELECT rowid, * FROM history
                                         WHERE user LIKE ? AND host LIKE ? AND command LIKE ? ESCAPE '\'
                                         ORDER BY datetime DESC LIMIT ?)
                                   ORDER BY datetime ASC`,
			qp.User, qp.Host, qp.Command, qp.Kappa)
	}
	if err != nil {
		return []byte{}, err
	}
	defer rows.Close()

	res := result.New(qp.Format)
	for rows.Next() {
		var user, host, command string
		var t time.Time
		var row int
		rows.Scan(&row, &user, &host, &command, &t)
		res.AddRow(row, user, host, command, t)
	}
	return res.Formatted(), nil
}

// DefaultQuery returns history within the search criteria in the format requested
func (d Database) DefaultQuery(qp conf.QueryParams) ([]byte, error) {
	var rows *sql.Rows
	var err error
	switch qp.Unique {
	case true:
		rows, err = d.Query(`SELECT rowid, datetime, user, host, command FROM history
                                        WHERE user LIKE ? AND host LIKE ? AND command LIKE ? ESCAPE '\'
                                        GROUP BY command ORDER BY DATETIME ASC`,
			qp.User, qp.Host, qp.Command)
	default:
		rows, err = d.Query(`SELECT rowid, * FROM history
                                         WHERE user LIKE ? AND host LIKE ? AND command LIKE ? ESCAPE '\'`,
			qp.User, qp.Host, qp.Command)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := result.New(qp.Format)
	for rows.Next() {
		var user, host, command string
		var t time.Time
		var row int
		rows.Scan(&row, &user, &host, &command, &t)
		res.AddRow(row, user, host, command, t)
	}
	// Return the result without the newline at the end.
	return res.Formatted(), nil
}

// RunQuery is a wrapper around various queries.
func (d Database) RunQuery(p conf.QueryParams) ([]byte, error) {
	switch p.Type {
	case conf.QUERY:
		return d.DefaultQuery(p)
	case conf.QUERY_LASTK:
		return d.LastK(p)
	case conf.QUERY_TOPK:
		return d.TopK(p)
	case conf.QUERY_USERS:
		return d.Users(p)
	case conf.QUERY_DEMO:
		return d.Demo(p)
	case conf.QUERY_ROW:
		return d.ReturnRow(p)
	}

	return []byte{}, errors.New("Unknown query type.")
}

// Users returns unique user@host pairs from the database.
func (d Database) Users(qp conf.QueryParams) (res []byte, e error) {
	var result bytes.Buffer
	result.WriteString(fmt.Sprintf("Unique user-hosts pairs:"))
	rows, e := d.Query(`SELECT distinct(user), host FROM history
                               WHERE user LIKE ? AND host LIKE ? AND command LIKE ? ESCAPE '\'`,
		qp.User, qp.Host, qp.Command)
	if e != nil {
		return result.Bytes(), e
	}
	defer rows.Close()

	for rows.Next() {
		var user string
		var host string
		rows.Scan(&user, &host)
		result.WriteString(fmt.Sprintf("\n%s@%s", user, host))
	}
	return result.Bytes(), e
}

// Demo returns some stats from the database to showcase bashistdb.
func (d Database) Demo(qp conf.QueryParams) (res []byte, e error) {
	var result bytes.Buffer

	var numUsers int
	err := d.QueryRow("SELECT count(*) FROM (SELECT distinct(user), host FROM history)").Scan(&numUsers)
	if err != nil {
		return result.Bytes(), err
	}

	var numHosts int
	err = d.QueryRow("SELECT count(distinct(host)) FROM history").Scan(&numHosts)
	if err != nil {
		return result.Bytes(), err
	}

	var numLines int
	err = d.QueryRow("SELECT count(command) FROM history").Scan(&numLines)
	if err != nil {
		return result.Bytes(), err
	}

	var numUniqueLines int
	err = d.QueryRow("SELECT count(distinct(command)) FROM history").Scan(&numUniqueLines)
	if err != nil {
		return result.Bytes(), err
	}

	qp.Kappa = 15
	restop, err := d.TopK(qp)
	if err != nil {
		return result.Bytes(), err
	}

	qp.Kappa = 10
	reslast, err := d.LastK(qp)
	if err != nil {
		return result.Bytes(), err
	}

	result.WriteString(fmt.Sprintf("There are %d command lines (%d unique) in your database from %d users across %d hosts.\n\n", numLines, numUniqueLines, numUsers, numHosts))

	result.WriteString(fmt.Sprintf("Top-15 commands for user %s@%s:\n", qp.User, qp.Host))
	result.Write(restop)

	result.WriteString(fmt.Sprintf("\n\nLast 10 commands user %s@%s ran:\n", qp.User, qp.Host))
	result.Write(reslast)

	return result.Bytes(), nil
}

// Return row returns a single row with no other data.
// It is useful to pipe to bash.
func (d Database) ReturnRow(qp conf.QueryParams) ([]byte, error) {
	var command string
	err := d.QueryRow("SELECT command FROM history WHERE rowid = ?", qp.Kappa).Scan(&command)
	if err != nil {
		return []byte{}, err
	}
	return []byte(command), nil
}

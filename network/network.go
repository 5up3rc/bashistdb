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

// Package network provides network functions for bashistdb.
package network

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"net"

	"github.com/andmarios/crypto/nacl/saltsecret"

	conf "projects.30ohm.com/mrsaccess/bashistdb/configuration"
	"projects.30ohm.com/mrsaccess/bashistdb/database"
	"projects.30ohm.com/mrsaccess/bashistdb/llog"
	"projects.30ohm.com/mrsaccess/bashistdb/local"
)

// Message Types
const (
	_ = iota
	RESULT
	HISTORY
	DEFAULT
	QUERY
)

type Message struct {
	Type     int
	Payload  []byte
	User     string
	Hostname string
	Query    string
	Format   string
}

var log *llog.Logger
var db database.Database

func init() {
	log = conf.Log
}

func ServerMode() error {
	var err error
	db, err = database.New()
	if err != nil {
		return err
	}
	defer db.Close()

	s, err := net.Listen("tcp", conf.Address)
	if err != nil {
		return err
	}
	log.Info.Println("Started listening on:", conf.Address)
	for {
		conn, err := s.Accept()
		if err != nil {
			log.Fatalln(err)
		}
		log.Info.Printf("Connection from %s.\n", conn.RemoteAddr())
		err = db.LogConn(conn.RemoteAddr())
		if err != nil {
			log.Fatalln(err)
		}
		go handleConn(conn)
	}
	return nil
}

func ClientMode() error {
	conn, err := net.Dial("tcp", conf.Address)
	if err != nil {
		return err
	}
	defer conn.Close()

	// If Operation == OP_DEFAULT, attempt to read from Stdin
	if conf.Operation == conf.OP_DEFAULT {
		r, err := local.GetStdin()
		if err == nil {
			history, err := ioutil.ReadAll(r)
			if err != nil {
				return err
			}

			msg := Message{Type: HISTORY, Payload: history, User: conf.User, Hostname: conf.Hostname}

			if err := encryptDispatch(conn, msg); err != nil {
				return err
			}

			log.Info.Println("Sent history.")

			reply, err := receiveDecrypt(conn)
			if err != nil {
				return err
			}

			switch reply.Type {
			case RESULT:
				log.Info.Println("Received:", string(reply.Payload))
			}
			return nil
		}
	}

	// Stdin not available? Switch operation.
	var msg Message
	switch conf.Operation {
	case conf.OP_DEFAULT:
		msg = Message{Type: DEFAULT, User: conf.User, Hostname: conf.Hostname}
	case conf.OP_QUERY:
		msg = Message{Type: QUERY, User: conf.User, Hostname: conf.Hostname, Query: conf.Query, Format: conf.Format}
	default:
		return errors.New("unknown function")
	}
	if err := encryptDispatch(conn, msg); err != nil {
		return err
	}
	log.Info.Println("Sent request.")

	reply, err := receiveDecrypt(conn)
	if err != nil {
		return err
	}

	switch reply.Type {
	case RESULT:
		fmt.Println(string(reply.Payload))
	}
	return nil
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	msg, err := receiveDecrypt(conn)
	if err != nil {
		log.Info.Println(err, "["+conn.RemoteAddr().String()+"]")
		return
	}

	var result []byte
	switch msg.Type {
	case HISTORY:
		r := bufio.NewReader(bytes.NewReader(msg.Payload))
		res, err := db.AddFromBuffer(r, msg.User, msg.Hostname)
		if err != nil {
			result = []byte(err.Error())
		} else {
			result = []byte(res)
		}
	case DEFAULT:
		res1, err := db.TopK(20)
		if err != nil {
			log.Fatalln(err)
		}
		res2, err := db.LastK(10)
		if err != nil {
			log.Fatalln(err)
		}
		result = []byte(res1 + res2)
	case QUERY:
		conf.Format = msg.Format
		result, err = db.RunQuery(msg.User, msg.Hostname, msg.Query)
		if err != nil {
			log.Fatalln(err)
		}
	}

	reply := Message{Type: RESULT, Payload: result}
	if err := encryptDispatch(conn, reply); err != nil {
		log.Println(err)
	}
}

func encryptDispatch(conn net.Conn, m Message) error {
	// We want to sent encrypted data.
	// In order to encrypt, we need to first serialize the message.
	// In order to sent/receive hassle free, we need to serialize the encrypted message
	// So: msg -> [GOB] -> [ENCRYPT] -> [GOB] -> (dispatch)

	// Create encrypter
	var encMsg bytes.Buffer
	encrypter, err := saltsecret.NewWriter(&encMsg, conf.Key, saltsecret.ENCRYPT, true)
	if err != nil {
		return err
	}

	// Serialize message
	enc := gob.NewEncoder(encrypter)
	if err = enc.Encode(m); err != nil {
		return err
	}

	// Flush encrypter to actuall encrypt the message
	if err = encrypter.Flush(); err != nil {
		return err
	}

	// Serialize encrypted message and dispatch it
	dispatch := gob.NewEncoder(conn)
	if err = dispatch.Encode(encMsg.Bytes()); err != nil {
		return err
	}

	return nil
}

func receiveDecrypt(conn net.Conn) (Message, error) {
	// Our work is:
	// (receive) -> [de-GOB] -> [DECRYPT] -> [de-GOB] -> msg

	// Receive data and de-serialize to get the encrypted message
	encMsg := &[]byte{}
	receive := gob.NewDecoder(conn)
	if err := receive.Decode(encMsg); err != nil {
		return Message{}, err
	}

	// Create decrypter and pass it the encrypted message
	r := bytes.NewReader(*encMsg)
	decrypter, err := saltsecret.NewReader(r, conf.Key, saltsecret.DECRYPT, false)
	if err != nil {
		return Message{}, err
	}

	// Read unencrypted serialized message and de-serialize it
	msg := &Message{}
	dec := gob.NewDecoder(decrypter)
	if err = dec.Decode(msg); err != nil {
		return Message{}, err
	}

	return *msg, nil
}

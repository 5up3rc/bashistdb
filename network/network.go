// Package network provides network functions for bashistdb.
package network

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"github.com/andmarios/crypto/nacl/saltsecret"

	conf "projects.30ohm.com/mrsaccess/bashistdb/configuration"
	"projects.30ohm.com/mrsaccess/bashistdb/database"
	"projects.30ohm.com/mrsaccess/bashistdb/llog"
)

const (
	PRINT = iota
	HISTORY
	QUERY
)

type Message struct {
	Type     int
	Payload  []byte
	User     string
	Hostname string
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
	stdinReader := bufio.NewReader(os.Stdin)
	stats, _ := os.Stdin.Stat()
	if (stats.Mode() & os.ModeCharDevice) != os.ModeCharDevice {
		conn, err := net.Dial("tcp", conf.Address)
		if err != nil {
			return err
		}
		defer conn.Close()

		// c := saltsecret.New([]byte("password"), true)

		history, err := ioutil.ReadAll(stdinReader)
		if err != nil {
			return err
		}
		msg := Message{HISTORY, history, conf.User, conf.Hostname}

		enc := gob.NewEncoder(conn)
		encmsg, err := encrypt(msg)
		if err != nil {
			return err
		}
		if err = enc.Encode(encmsg); err != nil {
			return err
		}
		log.Info.Println("Sent history.")

		reply, _ := bufio.NewReader(conn).ReadString('\n')
		fmt.Println(reply)
		conn.Close()
	}
	return nil
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	dec := gob.NewDecoder(conn)
	encMsg := &[]byte{}
	if err := dec.Decode(encMsg); err != nil {
		log.Info.Println(err)
		return
	}

	msg, err := decrypt(*encMsg)
	if err != nil {
		log.Info.Println(err, "["+conn.RemoteAddr().String()+"]")
		return
	}

	switch msg.Type {
	case HISTORY:
		r := bufio.NewReader(bytes.NewReader(msg.Payload))
		db.AddFromBuffer(r, msg.User, msg.Hostname)
	}
	fmt.Fprint(conn, "Everything ok.\n")
}

func encrypt(m Message) ([]byte, error) {
	var encMsg bytes.Buffer
	encrypter, err := saltsecret.NewWriter(&encMsg, conf.Key, saltsecret.ENCRYPT, true)
	if err != nil {
		return nil, err
	}

	enc := gob.NewEncoder(encrypter)
	if err = enc.Encode(m); err != nil {
		return nil, err
	}

	if err = encrypter.Flush(); err != nil {
		return nil, err
	}

	return encMsg.Bytes(), nil
}

func decrypt(encmsg []byte) (Message, error) {
	r := bytes.NewReader(encmsg)
	decrypter, err := saltsecret.NewReader(r, conf.Key, saltsecret.DECRYPT, false)
	if err != nil {
		return Message{}, err
	}

	msg := &Message{}
	dec := gob.NewDecoder(decrypter)
	if err = dec.Decode(msg); err != nil {
		return Message{}, err
	}

	return *msg, nil
}

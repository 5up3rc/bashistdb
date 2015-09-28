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

func init() {
	log = conf.Log
}

func ServerMode(db database.Database) error {
	s, err := net.Listen("tcp", conf.Address)
	if err != nil {
		return err
	}
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
		go handleConn(conn, db)
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
		enc.Encode(encmsg)
		log.Info.Println("Sent history.")

		reply, _ := bufio.NewReader(conn).ReadString('\n')
		fmt.Println(reply)
		conn.Close()
	}
	return nil
}

func handleConn(conn net.Conn, db database.Database) {
	defer conn.Close()

	dec := gob.NewDecoder(conn)
	encMsg := &[]byte{}
	dec.Decode(encMsg)

	msg, err := decrypt(*encMsg)
	if err != nil {
		log.Info.Println(err)
		return
	}

	switch msg.Type {
	case HISTORY:
		db.AddFromBuffer(bufio.NewReader(bytes.NewReader(msg.Payload)), msg.User, msg.Hostname)
	}
	fmt.Fprint(conn, "Everything ok.\n")
}

func encrypt(m Message) ([]byte, error) {
	var encMsg bytes.Buffer
	encrypter, err := saltsecret.NewWriter(&encMsg, []byte("password"), saltsecret.ENCRYPT, true)
	if err != nil {
		return nil, err
	}

	enc := gob.NewEncoder(encrypter)
	enc.Encode(m)

	err = encrypter.Flush()
	if err != nil {
		return nil, err
	}

	return encMsg.Bytes(), nil
}

func decrypt(encmsg []byte) (Message, error) {
	r := bytes.NewReader(encmsg)
	decrypter, err := saltsecret.NewReader(r, []byte("password"), saltsecret.DECRYPT, false)
	if err != nil {
		return Message{}, err
	}

	msg := &Message{}
	dec := gob.NewDecoder(decrypter)
	dec.Decode(msg)

	return *msg, nil
}

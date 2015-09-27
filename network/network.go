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

func ServerMode(address string, db database.Database, log *llog.Logger) error {
	s, err := net.Listen("tcp", ":5000")
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
		go handleConn(conn, db, log)
	}
	return nil
}

func ClientMode(address string, log *llog.Logger) error {
	stdinReader := bufio.NewReader(os.Stdin)
	stats, _ := os.Stdin.Stat()
	if (stats.Mode() & os.ModeCharDevice) != os.ModeCharDevice {
		conn, err := net.Dial("tcp", address)
		if err != nil {
			return err
		}

		// c := saltsecret.New([]byte("password"), true)

		history, err := ioutil.ReadAll(stdinReader)
		if err != nil {
			return err
		}
		msg := Message{HISTORY, history, "mrs", "mrs"}

		connEncoder := gob.NewEncoder(conn)
		var encMsg bytes.Buffer
		encrypter, err := saltsecret.NewWriter(&encMsg, []byte("password"), saltsecret.ENCRYPT, true)
		if err != nil {
			return err
		}
		msgEncoder := gob.NewEncoder(encrypter)
		msgEncoder.Encode(msg)
		err = encrypter.Flush()
		//history, err = c.Encrypt(history)
		if err != nil {
			return err
		}
		connEncoder.Encode(encMsg.Bytes())
		fmt.Println(len(encMsg.Bytes()))
		fmt.Printf("Sent history.\n")
		// fmt.Fprintf(conn, code.TRANSMISSION_END)

		reply, _ := bufio.NewReader(conn).ReadString('\n')
		fmt.Println(reply)
		conn.Close()
	}
	return nil
}

func handleConn(conn net.Conn, db database.Database, log *llog.Logger) {
	// r, err := saltsecret.NewReader(conn, []byte("password"), saltsecret.DECRYPT, false)
	// history, _ := ioutil.ReadAll(r)
	connDecoder := gob.NewDecoder(conn)
	encMsg := &[]byte{}
	connDecoder.Decode(encMsg)

	decrypter, err := saltsecret.NewReader(bytes.NewReader(*encMsg), []byte("password"), saltsecret.DECRYPT, false)
	if err != nil {
		log.Info.Println(err)
	}

	msg := &Message{}
	msgDecoder := gob.NewDecoder(decrypter)
	msgDecoder.Decode(msg)

	switch msg.Type {
	case HISTORY:
		db.AddFromBuffer(bufio.NewReader(bytes.NewReader(msg.Payload)), msg.User, msg.Hostname)
	}
	fmt.Fprint(conn, "Everything ok.\n")
	conn.Close()
}

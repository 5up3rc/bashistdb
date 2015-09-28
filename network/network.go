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
	_ = iota
	RESULT
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
			fmt.Println(string(reply.Payload))
		}
		conn.Close()
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

	switch msg.Type {
	case HISTORY:
		r := bufio.NewReader(bytes.NewReader(msg.Payload))
		db.AddFromBuffer(r, msg.User, msg.Hostname)
	}

	reply := Message{RESULT, []byte("Everything ok.\n"), "", ""}
	if err := encryptDispatch(conn, reply); err != nil {
		log.Println(err)
	}
}

func encryptDispatch(conn net.Conn, m Message) error {
	// We want to sent encrypted data.
	// In order to encrypt, we need to first serialize the message.
	// In order to sent/receive hassle free, we need to serialize the encrypted message
	// So: msg -> GOB -> ENCRYPT -> GOB -> (dispatch)

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
	// (incoming data) -> de-GOB -> DECRYPT -> de-GOB -> msg

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

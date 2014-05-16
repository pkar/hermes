package hermes

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"log"
	"net"
	"time"
)

// This is a simple stand-in for the Apple feedback service that
// can be used for testing purposes.
func mockFeedbackServer(cert, key string) {
	crt, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		log.Fatal(err)
	}
	config := tls.Config{Certificates: []tls.Certificate{crt}, ClientAuth: tls.RequireAnyClientCert}
	log.Print("- starting Mock Apple Feedback TCP server at 0.0.0.0:5555")

	srv, _ := tls.Listen("tcp", "0.0.0.0:5555", &config)
	for {
		conn, err := srv.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go loop(conn)
	}
}

// Writes binary data to the client in the same
// manner as the Apple service would.
//
// [4 bytes, 2 bytes, 32 bytes] = 38 bytes total
func loopFeedback(conn net.Conn) {
	defer conn.Close()
	for {
		timeT := uint32(1368809290) // 2013-05-17 12:48:10 -0400
		token := "abcd1234efab5678abcd1234efab5678"

		buf := new(bytes.Buffer)
		binary.Write(buf, binary.BigEndian, timeT)
		binary.Write(buf, binary.BigEndian, uint16(len(token)))
		binary.Write(buf, binary.BigEndian, []byte(token))
		conn.Write(buf.Bytes())

		dur, _ := time.ParseDuration("1s")
		time.Sleep(dur)
	}
}

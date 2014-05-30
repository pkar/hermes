package hermes

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"test"

	log "github.com/golang/glog"
)

// This is a simple stand-in for the Apple feedback service that
// can be used for testing purposes.
func mockServer(cert, key string) {
	crt, err := tls.X509KeyPair([]byte(cert), []byte(key))
	if err != nil {
		log.Fatal(err)
	}
	config := tls.Config{Certificates: []tls.Certificate{crt}, ClientAuth: tls.RequireAnyClientCert}
	log.Info("- starting Mock Apple TCP server at " + test.APNSGateway)

	srv, err := tls.Listen("tcp", test.APNSGateway, &config)
	if err != nil {
		log.Fatal(err)
	}
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
// [1 byte, 1 byte, 4 bytes] = 6 bytes total
func loop(conn net.Conn) {
	defer conn.Close()
	for {
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.BigEndian, uint8(8))
		binary.Write(buf, binary.BigEndian, uint8(0))
		binary.Write(buf, binary.BigEndian, uint32(0))
		conn.Write(buf.Bytes())
		time.Sleep(150 * time.Millisecond)
	}
}

func init() {
	go func() {
		mockServer(test.APNSCertMock, test.APNSKeyMock)
	}()
}

func TestNewAPNSClient(t *testing.T) {
	c, err := NewAPNSClient(test.APNSGateway, test.APNSCertMock, test.APNSKeyMock)
	if err != nil {
		t.Fatal(err)
	}
	if c.Gateway != test.APNSGateway {
		t.Fatal("gateway not set")
	}
}

func TestAPNSSend(t *testing.T) {
	c, _ := NewAPNSClient(test.APNSGateway, test.APNSCertMock, test.APNSKeyMock)
	ap := &APNSMessage{}
	apn, _ := NewAPNSPushNotification("E70331D08A2DA3BD02415DB2CAA4D7EEEC77FA2E5513B16F4F9E79C0BF89AED4", ap, 0)
	resp, err := c.Send(apn)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatal(resp.Error)
	}
	if resp.Status != 0 {
		t.Fatalf("%+v", resp)
	}
}

func TestAPNSPushNotificationSetGet(t *testing.T) {
	ap := &APNSMessage{}
	apn, _ := NewAPNSPushNotification("E70331D08A2DA3BD02415DB2CAA4D7EEEC77FA2E5513B16F4F9E79C0BF89AED4", ap, 0)
	apn.Set("test", []interface{}{0, "1234"})

	x := apn.Get("test")
	v, _ := x.([]interface{})
	if v[1] != "1234" {
		t.Fatal(`didn't get back "1234"`)
	}
}

func TestAPNSConnClose(t *testing.T) {
	c, _ := NewAPNSClient(test.APNSGateway, test.APNSCertMock, test.APNSKeyMock)
	conn := c.Pool.Get()
	err := conn.Close()
	if err != nil {
		t.Fatal(err)
	}
}

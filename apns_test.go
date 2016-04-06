package hermes

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"log"
	"net"
	"testing"
	"time"
)

const (
	APNSGateway  = "localhost:5555"
	APNSCertMock = `
-----BEGIN CERTIFICATE-----
MIIC+zCCAeOgAwIBAgIJAIPzSpouPyKwMA0GCSqGSIb3DQEBBQUAMBQxEjAQBgNV
BAMMCWxvY2FsaG9zdDAeFw0xMjEyMjExMTQyNTZaFw0yMjEyMTkxMTQyNTZaMBQx
EjAQBgNVBAMMCWxvY2FsaG9zdDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC
ggEBALiENcpbbpS5f+y1oHOK3W6S2dXPAjec3MojINgdHB4c+CvQgt0HgDQ/3Zz7
fgzy+McGoyrz32Ul0Vm6IYn7EfCvzY//SpMYcogiitmTK21Q/53mmOak3jX2pelq
rDUkg8F9fAJg6RfodRHzslBnl5YQ3MQFeiEBYyDpvFb6VvOCkKnN2bh8NkBgD0up
EBBBcSYV43ggW/Jl1ywUxSijBXEifcA1vbXxCvyf34mYlL+x1KJze5Wv6gQlBBxo
AzLibkC9a+4YIVl9X/eHFc1qMQBsTM414HGOxqvjHvYn6fnFxfViKOrBVgGR/BJC
HUaVi7VJJ6KT6AaK5QoH2XoIpFsCAwEAAaNQME4wHQYDVR0OBBYEFNM89MQu55aV
4MXBn3B/r0E2qqXNMB8GA1UdIwQYMBaAFNM89MQu55aV4MXBn3B/r0E2qqXNMAwG
A1UdEwQFMAMBAf8wDQYJKoZIhvcNAQEFBQADggEBAE8mB+46i8J5V0SVHB/wkI4T
kWWz0SnDXIgT7do9Ex99xlGr/uJM4b7Fa/UbF7tI5WiFuImxtfvY3bMJM1hvVTVY
bi+Vv0BDOelC1NXf94DwATs/M2Scb981JtH8Pgaftnfo643DsMe12cPcQORSsWbW
SAin94jx5UODWW8o7aGG+Fo4Z1E/e9/Te3MtRSihWScf/WGhvwDdIPUWLwIYnWXa
b0nWeXuDdjYWlDJh8k4zOCtEVRzlE4nBv6yZYKJaZdMCKJMuIxVvEEbu/UN0qPGs
e9Im1Ay4CX6wzkPWicbl2xnh76Yevf9y0sWzD13GCA6YMeEK/uBLibg33dfPzLE=
-----END CERTIFICATE-----
  `
	APNSKeyMock = `
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAuIQ1yltulLl/7LWgc4rdbpLZ1c8CN5zcyiMg2B0cHhz4K9CC
3QeAND/dnPt+DPL4xwajKvPfZSXRWbohifsR8K/Nj/9KkxhyiCKK2ZMrbVD/neaY
5qTeNfal6WqsNSSDwX18AmDpF+h1EfOyUGeXlhDcxAV6IQFjIOm8VvpW84KQqc3Z
uHw2QGAPS6kQEEFxJhXjeCBb8mXXLBTFKKMFcSJ9wDW9tfEK/J/fiZiUv7HUonN7
la/qBCUEHGgDMuJuQL1r7hghWX1f94cVzWoxAGxMzjXgcY7Gq+Me9ifp+cXF9WIo
6sFWAZH8EkIdRpWLtUknopPoBorlCgfZegikWwIDAQABAoIBAEaPmJpn2KPbREZb
Np64zfEJC3CuFyT5QZ2zTU4X47bIUUdAF6s6wRY6Dh+INS3yhJxnt2InnJhrm+F6
QnUnpDaspCma8QPLZ5ET1JFbrFHDldzmYDZjee6dAdl/R5eS/SezOwcV1E2mQY65
6MjCtL9Yd3QmvAt/Ik9l0vZYCYRZFUVAU7ej1MbOUuF6uh0xnthKRzc8Fp/0v9VO
zR1w4T2aT/bSaix9oMpVFPRP6APniqY92QDsPLjbi4idSFdklQn8kG9WFZVDMOeg
FW+dMjFwtgWfjdKCjUQ1x27piyB/4BxZqXgvWaNAtDbU/60xYLQo3hiMHCMxwJ1l
u0vfloECgYEA5iOCIWivfL1aYdpSC/e8jIJmA2LU4EVddWFPlJjAlY2r+9+j1Ic2
F4OVBM+yVKsB4klKJUAJ9OAQbk0Asa40KNdnF/VYFlMIZSUoHDasalWsxqGWKCBs
eYpNTf51tpRAsASj24zN4LdNMdduLknLBwqgcuYgxF6Xzl01MYqLRWECgYEAzUBE
pDriekbJBqje77+8ZjQCQF8iMmU64q4fMnBnKNSTEmxyHNbN7/BfF4J3QR/oU3N9
ZChIUCmyHJ8pXe6zFopv0cwGaakbf2Ee//dDcnigMOYbLhygdGi1oN6uPLEENhLN
J/udwkg459DzXn0jSOUnDw9pQ9Gln1skkg9KBzsCgYAz5dADrrLcQ2stY+lar4xC
d2l/2/q7dIkF3mLu1J+hWihtjVpJpBArr02cnyXM+B9doz9oNQ/Ju/mYlh7Q8sLq
buDdw0MRDbp37LAl5KJu/FERHgFZnS45Hloee4KaIMaRqwo0iYUn5s4urjE3mQaC
2P+jyYecIOTE8bn8KQ0NIQKBgQC0TYe/CWdYaQRBEGm/DJzQ31E3ARtGT/z5kmIf
afSFPq/v2EoqIVyJMYwnV9mw4PmzDVoSePyFRwuK7xpkxMKXw4bVMrhTa1WXgVa9
HpYmYea+7fTkfgtKF42uQs+myw3a/oswW23LdKxgoAKad61eZMb6CNy80db/dQ5c
LIgobwKBgClD9b44UuICoC1mvYWhV0/wLjXvCyUGwXsiNxGwp8u9Zrraa9KJVXsi
7g0GeN7N3V1+lQbl3XXcPx6JZVtqmDViaod6q6Fjgr5BOJej+q6twWzu3dOQjgil
l2N13a1hUxAt4VvyG57AwxaKo4FDNi9QHMmhwXvGC4Qt/qEyU1EL
-----END RSA PRIVATE KEY-----
`
)

// This is a simple stand-in for the Apple feedback service that
// can be used for testing purposes.
func mockServer(cert, key string) {
	crt, err := tls.X509KeyPair([]byte(cert), []byte(key))
	if err != nil {
		log.Fatal(err)
	}
	config := tls.Config{Certificates: []tls.Certificate{crt}, ClientAuth: tls.RequireAnyClientCert}
	log.Info("- starting Mock Apple TCP server at " + APNSGateway)

	srv, err := tls.Listen("tcp", APNSGateway, &config)
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
		mockServer(APNSCertMock, APNSKeyMock)
	}()
}

func TestNewAPNSClient(t *testing.T) {
	c, err := NewAPNSClient(APNSGateway, APNSCertMock, APNSKeyMock)
	if err != nil {
		t.Fatal(err)
	}
	if c.Gateway != APNSGateway {
		t.Fatal("gateway not set")
	}
}

func TestAPNSSend(t *testing.T) {
	c, _ := NewAPNSClient(APNSGateway, APNSCertMock, APNSKeyMock)
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
	c, _ := NewAPNSClient(APNSGateway, APNSCertMock, APNSKeyMock)
	conn := c.Pool.Get()
	err := conn.Close()
	if err != nil {
		t.Fatal(err)
	}
}

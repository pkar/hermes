package hermes

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"time"

	log "github.com/golang/glog"
)

const (
	// maxPoolSize is the number of sockets to open per
	// app.
	maxPoolSize = 20
)

// APNSURLs map environment to gateway.
var APNSURLs = map[string]string{
	"testing":         "localhost:5555",
	"development":     "gateway.sandbox.push.apple.com:2195",
	"staging":         "gateway.sandbox.push.apple.com:2195",
	"staging_sandbox": "gateway.sandbox.push.apple.com:2195",
	"sandbox":         "gateway.sandbox.push.apple.com:2195",
	"production":      "gateway.push.apple.com:2195",
}

// apnsStatusCodes are codes to message from apns.
var apnsStatusCodes = map[uint8]string{
	0:   "No errors encountered",
	1:   "Processing error",
	2:   "Missing device token",
	3:   "Missing topic",
	4:   "Missing payload",
	5:   "Invalid token size",
	6:   "Invalid topic size",
	7:   "Invalid payload size",
	8:   "Invalid token",
	10:  "Shutdown",
	255: "None (unknown)",
}

// APNSMessage alert is an interface here because it supports either a string
// or a dictionary, represented within by an AlertDictionary struct.
type APNSMessage struct {
	Alert interface{} `json:"alert,omitempty"`
	Badge int         `json:"badge,omitempty"`
	Sound string      `json:"sound,omitempty"`
}

// Bytes implements interface Message.
func (a *APNSMessage) Bytes() ([]byte, error) {
	return json.Marshal(a)
}

// APNSPushNotification ...
type APNSPushNotification struct {
	Identifier  int32
	Expiry      uint32
	DeviceToken string
	Priority    uint8
	payload     map[string]interface{}
}

// Bytes implements interface Message.
func (a *APNSPushNotification) Bytes() ([]byte, error) {
	return json.Marshal(a)
}

// APNSResponse ...
type APNSResponse struct {
	Command    uint8 `json:"command"`
	Status     uint8 `json:"status"`
	Identifier int32 `json:"identifier"`
	err        error `json:"err"`
}

// Bytes implements interface Response.
func (a *APNSResponse) Bytes() ([]byte, error) {
	return json.Marshal(a)
}

// APNSAlertDictionary From the APN docs:
// "Use the ... alert dictionary in general only if you absolutely need to."
// The AlertDictionary is suitable for specific localization needs.
type APNSAlertDictionary struct {
	Body         string   `json:"body,omitempty"`
	ActionLocKey string   `json:"action-loc-key,omitempty"`
	LocKey       string   `json:"loc-key,omitempty"`
	LocArgs      []string `json:"loc-args,omitempty"`
	LaunchImage  string   `json:"launch-image,omitempty"`
}

// APNSClient ...
type APNSClient struct {
	Certificate string
	Key         string
	Gateway     string
	Pool        *APNSPool
}

// APNSPool ...
type APNSPool struct {
	pool     chan *APNSConn
	nClients int
}

// APNSConn ...
type APNSConn struct {
	gateway        string
	readTimeout    time.Duration
	tlsConn        *tls.Conn
	tlsCfg         tls.Config
	transactionID  uint32
	connected      bool
	maxPayloadSize int // default to 256 as per Apple specifications (June 9 2012)
}

// NewAPNSClient ...
func NewAPNSClient(gateway, cert, key string) (*APNSClient, error) {
	start := time.Now()
	defer func() { log.Info("Hermes.APNS.NewAPNSClient ", time.Since(start)) }()

	p, err := newAPNSPool(gateway, cert, key)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	client := &APNSClient{
		Gateway:     gateway,
		Certificate: cert,
		Key:         key,
		Pool:        p,
	}

	return client, err
}

// newAPNSConn is the actual connection to the remote server.
func newAPNSConn(gateway, cert, key string) (*APNSConn, error) {
	start := time.Now()
	defer func() { log.V(2).Info("Hermes.APNS.newAPNSConn ", time.Since(start)) }()

	conn := &APNSConn{}
	crt, err := tls.X509KeyPair([]byte(cert), []byte(key))
	if err != nil {
		log.Error(err)
		return nil, err
	}
	conn.tlsConn = nil
	conn.tlsCfg = tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{crt},
	}

	conn.readTimeout = 150 * time.Millisecond
	conn.maxPayloadSize = 256
	conn.connected = false
	conn.gateway = gateway

	return conn, nil
}

// newAPNSPool ...
func newAPNSPool(gateway, certificate, key string) (*APNSPool, error) {
	start := time.Now()
	defer func() { log.Info("Hermes.APNS.newAPNSPool ", time.Since(start)) }()

	pool := make(chan *APNSConn, maxPoolSize)
	n := 0
	for x := 0; x < maxPoolSize; x++ {
		c, err := newAPNSConn(gateway, certificate, key)
		if err != nil {
			// Possible errors are missing/invalid environment which would be caught earlier.
			// Most likely invalid cert.
			log.Error(err)
			return nil, err
		}
		pool <- c
		n++
	}
	return &APNSPool{pool, n}, nil
}

// NewAPNSPushNotification ...
func NewAPNSPushNotification(deviceToken string, ap *APNSMessage, expiry uint32) (*APNSPushNotification, error) {
	/*
		if expiry == 0 {
			unixNow := uint32(time.Now().Unix())
			expiry = unixNow + 60*60
		}
	*/

	apn := &APNSPushNotification{
		Identifier:  rand.New(rand.NewSource(time.Now().UnixNano())).Int31n(9999),
		Priority:    10,
		Expiry:      expiry,
		DeviceToken: deviceToken,
		payload:     make(map[string]interface{}),
	}
	apn.payload["aps"] = ap

	return apn, nil
}

// Get gets a custom field from the apns payload.
func (a *APNSPushNotification) Get(key string) interface{} {
	return a.payload[key]
}

// Set adds custom fields to the apns payload.
func (a *APNSPushNotification) Set(key string, value interface{}) {
	a.payload[key] = value
}

// Close ...
func (c *APNSConn) Close() error {
	var err error
	if c.tlsConn != nil {
		err = c.tlsConn.Close()
		c.connected = false
	}
	return err
}

// connect ...
func (c *APNSConn) connect() (err error) {
	start := time.Now()
	defer func() { log.Info("Hermes.APNS.connect ", time.Since(start)) }()

	if c.connected {
		return nil
	}

	if c.tlsConn != nil {
		c.Close()
	}

	conn, err := net.Dial("tcp", c.gateway)
	if err != nil {
		log.Error(err)
		return err
	}

	c.tlsConn = tls.Client(conn, &c.tlsCfg)
	err = c.tlsConn.Handshake()
	if err == nil {
		c.connected = true
	}

	return err
}

// Get ...
func (p *APNSPool) Get() *APNSConn {
	return <-p.pool
}

// Release ...
func (p *APNSPool) Release(conn *APNSConn) {
	p.pool <- conn
}

// Send ...
func (c *APNSClient) Send(apn *APNSPushNotification) (*APNSResponse, error) {
	log.V(2).Infof("%+v", apn)
	start := time.Now()
	defer func() { log.Info("Hermes.APNS.Send ", time.Since(start)) }()

	conn := c.Pool.Get()
	defer c.Pool.Release(conn)
	err := conn.connect()
	if err != nil {
		log.Error(err)
		return nil, err
	}

	token, err := hex.DecodeString(apn.DeviceToken)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	payload, err := json.Marshal(apn.payload)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if len(payload) > 256 {
		return nil, fmt.Errorf("payload larger than 256, got %d", len(payload))
	}
	buffer := bytes.NewBuffer([]byte{})
	binary.Write(buffer, binary.BigEndian, uint8(1))           // command
	binary.Write(buffer, binary.BigEndian, apn.Identifier)     // transaction id, optional
	binary.Write(buffer, binary.BigEndian, apn.Expiry)         // expiration time(sec)
	binary.Write(buffer, binary.BigEndian, uint16(len(token))) // push device token
	binary.Write(buffer, binary.BigEndian, token)
	binary.Write(buffer, binary.BigEndian, uint16(len(payload))) // payload
	binary.Write(buffer, binary.BigEndian, payload)

	apr := &APNSResponse{
		Identifier: apn.Identifier,
	}

	_, err = conn.tlsConn.Write(buffer.Bytes())
	if err != nil {
		log.Error(err)
		conn.connected = false
		return nil, err
	}
	conn.tlsConn.SetReadDeadline(time.Now().Add(conn.readTimeout))
	read := [6]byte{}
	n, err := conn.tlsConn.Read(read[:])
	if err != nil {
		if err2, ok := err.(net.Error); ok && err2.Timeout() {
			// Success, apns doesn't usually return a response if successful.
			// Only issue is, is timeout length long enough (150ms) for err response.
			return apr, nil
		}
		return nil, err
	}
	if n >= 0 {
		status := uint8(read[1])
		apr.Status = status
		switch status {
		case 0:
			return apr, nil
		case 1, 2, 3, 4, 5, 6, 7, 8:
			//1:   "Processing error"
			//2:   "Missing Device Token",
			//3:   "Missing Topic",
			//4:   "Missing Payload",
			//5:   "Invalid Token Size",
			//6:   "Invalid Topic Size",
			//7:   "Invalid Payload Size",
			//8:   "Invalid Token",
			err := fmt.Errorf("error code:%s %v", apnsStatusCodes[status], hex.EncodeToString(read[:n]))
			log.Error(err)
			apr.err = err
			return apr, nil
		case 255:
			err := fmt.Errorf("unknown error code %v", hex.EncodeToString(read[:n]))
			log.Error(err)
			apr.err = err
			return apr, err
		}
	}

	return apr, nil
}

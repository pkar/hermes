package hermes

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net"
	"time"

	log "github.com/golang/glog"
)

// Wait at most this many seconds for feedback data from Apple.
const feedbackTimeout = 5

// APNSFeedbackChannel will receive individual responses from Apple.
var APNSFeedbackChannel = make(chan *FeedbackResponse)

// If there's nothing to read, ShutdownChannel gets a true.
var APNSShutdownChannel = make(chan bool)

// FeedbackResponse
type FeedbackResponse struct {
	Timestamp   uint32
	TokenLength uint16
	DeviceToken string
}

// Feedback runs APNS feedback waiting for errors.
func (client *APNSClient) RunFeedback() error {
	go client.ListenForFeedback()
	go func() {
		for {
			select {
			case resp := <-APNSFeedbackChannel:
				log.Info("- recv'd:", resp.DeviceToken)
			case <-APNSShutdownChannel:
				log.Info("- nothing returned from the feedback service")
			}
		}
	}()
	return nil
}

// Connect to the Apple Feedback Service and check for feedback.
// Feedback consists of device identifiers that should
// not be sent to in the future; Apple does monitor that
// you respect this so you should be checking it ;)
func (c *APNSClient) ListenForFeedback() (err error) {
	cert, err := tls.LoadX509KeyPair(c.Certificate, c.Key)
	if err != nil {
		log.Error(err)
		return err
	}

	conf := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	conn, err := net.Dial("tcp", c.Gateway)
	if err != nil {
		log.Error(err)
		return err
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(feedbackTimeout * time.Second))

	tlsConn := tls.Client(conn, conf)
	err = tlsConn.Handshake()
	if err != nil {
		log.Error(err)
		return err
	}

	var tokenLength uint16
	buffer := make([]byte, 38, 38)
	deviceToken := make([]byte, 32, 32)

	for {
		_, err := tlsConn.Read(buffer)
		if err != nil {
			log.Error(err)
			APNSShutdownChannel <- true
			break
		}

		resp := &FeedbackResponse{}

		r := bytes.NewReader(buffer)
		binary.Read(r, binary.BigEndian, &resp.Timestamp)
		binary.Read(r, binary.BigEndian, &tokenLength)
		binary.Read(r, binary.BigEndian, &deviceToken)
		if tokenLength != 32 {
			err := errors.New("Token length should be equal to 32, but isn't.")
			log.Error(err)
			return err
		}
		resp.DeviceToken = hex.EncodeToString(deviceToken)
		resp.TokenLength = tokenLength

		APNSFeedbackChannel <- resp
	}

	return nil
}

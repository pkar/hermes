package hermes

// NOTE untested

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/golang/glog"
)

const (
	c2dmServiceURL string = "http://android.apis.google.com/c2dm/send"
)

// C2DMURLs map environment to c2dm url.
var C2DMURLs = map[string]string{
	"testing":         "localhost:5556",
	"development":     "http://android.apis.google.com/c2dm/send",
	"staging":         "http://android.apis.google.com/c2dm/send",
	"staging_sandbox": "http://android.apis.google.com/c2dm/send",
	"sandbox":         "http://android.apis.google.com/c2dm/send",
	"production":      "http://android.apis.google.com/c2dm/send",
}

// C2DMClient ...
type C2DMClient struct {
	key  string
	http *http.Client
	url  string
}

// C2DMMessage https://developers.google.com/android/c2dm/?csw=1#push
type C2DMMessage struct {
	RegistrationID string                 `json:"registration_ids"`
	CollapseKey    string                 `json:"collapse_key,omitempty"`
	Data           map[string]interface{} `json:"data"`
	DelayWhileIdle bool                   `json:"delay_while_idle,omitempty"`
}

// Bytes implements interface Message.
func (g *C2DMMessage) Bytes() ([]byte, error) {
	return json.Marshal(g)
}

// C2DMResponse http://developer.android.com/guide/google/gcm/gcm.html#send-msg
type C2DMResponse struct {
	RegistrationID string `json:"registration_id"`
	Error          error  `json:"error"`
	StatusCode     int    `json:"status_code"`
	RetryAfter     int    `json:"retry_after"`
}

// Bytes implements interface Response.
func (c *C2DMResponse) Bytes() ([]byte, error) {
	return json.Marshal(c)
}

// Retry implements interface Response.
func (c *C2DMResponse) Retry() int {
	return c.RetryAfter
}

// UpdateToken implements interface Response.
func (c *C2DMResponse) UpdateToken() bool {
	if c == nil {
		return false
	}
	if c.Error == ErrRemoveToken || c.Error == ErrUpdateToken {
		return true
	}
	return false
}

// NewC2DMClient ...
func NewC2DMClient(url, key string) (*C2DMClient, error) {
	if url == "" {
		return nil, fmt.Errorf("url not provided")
	}

	return &C2DMClient{
		key:  key,
		http: &http.Client{},
		url:  url,
	}, nil
}

// NewC2DMMessage ...
func NewC2DMMessage(id string) *C2DMMessage {
	return &C2DMMessage{
		DelayWhileIdle: true,
		RegistrationID: id,
		Data:           make(map[string]interface{}),
	}
}

// Send https://developers.google.com/android/c2dm/
func (c *C2DMClient) Send(m *C2DMMessage) (*C2DMResponse, error) {
	log.V(2).Infof("%+v", m)
	start := time.Now()
	defer func() { log.Info("Hermes.C2DM.Send ", time.Since(start)) }()

	if m.RegistrationID == "" {
		log.Errorf("no registration id %+v", m)
		return nil, fmt.Errorf("no registration id")
	}

	if len(m.Data) == 0 {
		log.Errorf("no payload Defined %+v", m)
		return nil, fmt.Errorf("no payload")
	}

	data := url.Values{}
	data.Set("registration_id", m.RegistrationID)
	if m.CollapseKey == "" {
		m.CollapseKey = "piglet"
	}
	data.Set("collapse_key", m.CollapseKey)

	for k, v := range m.Data {
		val, ok := v.(string)
		if ok {
			data.Set("data."+k, val)
		}
	}

	enc := data.Encode()
	if len(enc) >= 1024 {
		log.Errorf("Message Too Long (1024 max): %d", len(enc))
		return nil, fmt.Errorf("message too big")
	}

	request, err := http.NewRequest("POST", c.url, strings.NewReader(enc))
	if err != nil {
		log.Error(err)
		return nil, err
	}

	request.Header.Add("Authorization", fmt.Sprintf("GoogleLogin auth=%s", c.key))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(request)
	if err != nil {
		log.Errorf("%s %+v", err, resp)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	res := &C2DMResponse{RetryAfter: -1}
	switch resp.StatusCode {
	case 503, 500:
		after := resp.Header.Get("Retry-After")
		sleepFor, e := strconv.Atoi(after)
		if e != nil {
			log.Error(e)
		}
		res.RetryAfter = sleepFor
		res.Error = ErrRetry
		return res, res.Error
	case 401:
		return nil, fmt.Errorf("unauthorized %s %s", resp.Status, string(body))
	case 400:
		return nil, fmt.Errorf("malformed JSON %s %s", resp.Status, string(body))
	case 200:
	default:
		return nil, fmt.Errorf("unknown error %s %s", resp.Status, string(body))
	}

	//regexp.Compile(`id=(.*)`)
	re, err := regexp.Compile(`Error=(.*)`)
	if err != nil {
		log.Errorf("c2dm %v %+v", err, m)
		return nil, err
	}
	errs := re.FindStringSubmatch(string(body))

	if len(errs) >= 2 {
		switch errs[1] {
		case "QuotaExceeded":
			// Too many messages, retry after a while.
			log.Errorf("c2dm Quota Exceeded %+v %s", m, errs[1])

			after := resp.Header.Get("Retry-After")
			sleepFor, e := strconv.Atoi(after)
			if e != nil {
				log.Error(e)
			}
			res.RetryAfter = sleepFor
			res.Error = ErrRetry
		case "DeviceQuotaExceeded":
			//  Too many messages sent by the sender to a specific device. Retry after a while.
			log.Errorf("c2dm Device Quota Exceeded %+v %s", m, errs[1])

			after := resp.Header.Get("Retry-After")
			sleepFor, e := strconv.Atoi(after)
			if e != nil {
				log.Error(e)
			}
			res.RetryAfter = sleepFor
			res.Error = ErrRetry
		case "InvalidRegistration":
			log.Errorf("c2dm Invalid Registration %+v %s", m, errs[1])
			res.Error = ErrRemoveToken
		case "NotRegistered":
			log.Errorf("c2dm not registered %+v %s", m, errs[1])
			res.Error = ErrRemoveToken
		case "MessageTooBig":
			log.Errorf("c2dm Message Too Big %+v %s", m, errs[1])
			res.Error = fmt.Errorf(errs[1])
		case "MissingCollapseKey":
			log.Errorf("c2dm Missing Collapse Key %+v %s", m, errs[1])
			res.Error = fmt.Errorf(errs[1])
		default:
			log.Errorf("c2dm Unknown Error %+v %s", m, errs[1])
			after := resp.Header.Get("Retry-After")
			sleepFor, e := strconv.Atoi(after)
			if e != nil {
				log.Error(e)
			}
			res.RetryAfter = sleepFor
			res.Error = ErrRetry
		}
	}
	return res, res.Error
}

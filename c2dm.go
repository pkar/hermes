// NOTE untested
package hermes

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	log "github.com/golang/glog"
)

const (
	c2dmServiceURL string = "http://android.apis.google.com/c2dm/send"
)

var C2DMURLs = map[string]string{
	"testing":         "localhost:5556",
	"development":     "http://android.apis.google.com/c2dm/send",
	"staging":         "http://android.apis.google.com/c2dm/send",
	"staging_sandbox": "http://android.apis.google.com/c2dm/send",
	"sandbox":         "http://android.apis.google.com/c2dm/send",
	"production":      "http://android.apis.google.com/c2dm/send",
}

// C2DMClient
type C2DMClient struct {
	key  string
	http *http.Client
	url  string
}

// https://developers.google.com/android/c2dm/?csw=1#push
type C2DMMessage struct {
	RegistrationIDs []string               `json:"registration_ids"`
	CollapseKey     string                 `json:"collapse_key,omitempty"`
	Data            map[string]interface{} `json:"data"`
	DelayWhileIdle  bool                   `json:"delay_while_idle,omitempty"`
	TimeToLive      int                    `json:"time_to_live,omitempty"`
	DryRun          bool                   `json:"dry_run,omitempty"`
}

// Bytes implements interface Message.
func (g *C2DMMessage) Bytes() ([]byte, error) {
	return json.Marshal(g)
}

// C2DMResult ...
type C2DMResult struct {
	RegistrationID string `json:"registration_id"`
	Error          error  `json:"error"`
	RetryAfter     int    `json:"retry_after"`
	StatusCode     int    `json:"status_code,omitempty"`
}

// http://developer.android.com/guide/google/gcm/gcm.html#send-msg
type C2DMResponse struct {
	Success int           `json:"success"`
	Failure int           `json:"failure"`
	Results []*C2DMResult `json:"results"`
}

// Bytes implements interface Response.
func (a *C2DMResponse) Bytes() ([]byte, error) {
	return json.Marshal(a)
}

// NewC2DMClient
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

// Send
// https://developers.google.com/android/c2dm/
func (c *C2DMClient) Send(m *C2DMMessage) (*C2DMResponse, error) {
	log.V(2).Infof("%+v", m)

	if len(m.RegistrationIDs) == 0 {
		log.Error("no registration ids %+v", m)
		return nil, fmt.Errorf("no registration ids")
	}

	if len(m.Data) == 0 {
		log.Error("no payload Defined %+v", m)
		return nil, fmt.Errorf("no payload")
	}

	ret := &C2DMResponse{}
	for _, regID := range m.RegistrationIDs {
		res, err := c.send(regID, m)
		if err != nil {
			log.Error(err)
			ret.Results = append(ret.Results, &C2DMResult{Error: err})
			ret.Failure++
		} else {
			ret.Results = append(ret.Results, res)
			if res.Error == nil {
				ret.Success++
			}
		}
	}
	return ret, nil
}

// send does the individual sends.
func (c *C2DMClient) send(regID string, m *C2DMMessage) (*C2DMResult, error) {
	data := url.Values{}
	data.Set("registration_id", regID)
	data.Set("collapse_key", m.CollapseKey)

	for k, v := range m.Data {
		val, ok := v.(string)
		if ok {
			data.Set("data."+k, val)
		}
	}

	enc := data.Encode()
	if len(enc) >= 1024 {
		log.Error("Message Too Long (1024 max): %d", len(enc))
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
		log.Error("%s %+v", err, resp)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	res := &C2DMResult{StatusCode: resp.StatusCode}
	switch resp.StatusCode {
	case 503, 500:
		after := resp.Header.Get("Retry-After")
		sleepFor, e := strconv.Atoi(after)
		if e != nil {
			log.Error(e)
		}
		res.RetryAfter = sleepFor
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
		log.Error("c2dm %v %+v", err, m)
		return nil, err
	}
	errs := re.FindStringSubmatch(string(body))

	if len(errs) >= 2 {
		switch errs[1] {
		case "QuotaExceeded":
			// Too many messages, retry after a while.
			log.Error("c2dm Quota Exceeded %+v", m)

			after := resp.Header.Get("Retry-After")
			sleepFor, e := strconv.Atoi(after)
			if e != nil {
				log.Error(e)
			}
			res.RetryAfter = sleepFor
			return res, fmt.Errorf(errs[1])
		case "DeviceQuotaExceeded":
			//  Too many messages sent by the sender to a specific device. Retry after a while.
			log.Error("c2dm Device Quota Exceeded %+v", m)

			after := resp.Header.Get("Retry-After")
			sleepFor, e := strconv.Atoi(after)
			if e != nil {
				log.Error(e)
			}
			res.RetryAfter = sleepFor
			return res, fmt.Errorf(errs[1])
		case "InvalidRegistration":
			log.Error("c2dm Invalid Registration %+v", m)
			return res, fmt.Errorf(errs[1])
		case "NotRegistered":
			log.Error("c2dm Not Registered %+v", m)
			return res, fmt.Errorf(errs[1])
		case "MessageTooBig":
			log.Error("c2dm Message Too Big %+v", m)
			return res, fmt.Errorf(errs[1])
		case "MissingCollapseKey":
			log.Error("c2dm Missing Collapse Key %+v", m)
			return res, fmt.Errorf(errs[1])
		default:
			log.Error("c2dm Unknown Error %+v", m)
			after := resp.Header.Get("Retry-After")
			sleepFor, e := strconv.Atoi(after)
			if e != nil {
				log.Error(e)
			}
			res.RetryAfter = sleepFor
			return res, fmt.Errorf(errs[1])
		}
	}
	return res, nil
}

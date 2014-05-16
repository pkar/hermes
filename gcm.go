package hermes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	log "github.com/golang/glog"
)

var GCMURLs = map[string]string{
	"testing":         "localhost:5556",
	"development":     "https://android.googleapis.com/gcm/send",
	"staging":         "https://android.googleapis.com/gcm/send",
	"staging_sandbox": "https://android.googleapis.com/gcm/send",
	"sandbox":         "https://android.googleapis.com/gcm/send",
	"production":      "https://android.googleapis.com/gcm/send",
}

// http://developer.android.com/guide/google/gcm/gcm.html#send-msg
type GCMMessage struct {
	RegistrationIDs []string `json:"registration_ids"`
	//NotificationKey string                 `json:"notification_key"`
	CollapseKey    string                 `json:"collapse_key,omitempty"`
	Data           map[string]interface{} `json:"data,omitempty"`
	DelayWhileIdle bool                   `json:"delay_while_idle,omitempty"`
	TimeToLive     int                    `json:"time_to_live,omitempty"`
	DryRun         bool                   `json:"dry_run,omitempty"`
}

// Bytes implements interface Message.
func (g *GCMMessage) Bytes() ([]byte, error) {
	return json.Marshal(g)
}

// http://developer.android.com/guide/google/gcm/gcm.html#send-msg
type GCMResponse struct {
	MulticastID  int64 `json:"multicast_id"`
	Success      int   `json:"success"`
	Failure      int   `json:"failure"`
	CanonicalIDs int   `json:"canonical_ids"`
	Results      []struct {
		MessageID      string `json:"message_id"`
		RegistrationID string `json:"registration_id"`
		Error          string `json:"error"`
	} `json:"results"`
	StatusCode int `json:"status_code,omitempty"`
	RetryAfter int `json:"retry_after,omitempty"`
}

// Bytes implements interface Response.
func (a *GCMResponse) Bytes() ([]byte, error) {
	return json.Marshal(a)
}

// GCMClient
type GCMClient struct {
	key  string
	http *http.Client
	url  string
}

// NewGCMClient
func NewGCMClient(url, key string) (*GCMClient, error) {
	if url == "" {
		return nil, fmt.Errorf("url not provided")
	}

	return &GCMClient{
		key:  key,
		http: &http.Client{},
		url:  url,
	}, nil
}

// NewGCMMessage
func NewGCMMessage(ids ...string) *GCMMessage {
	return &GCMMessage{
		TimeToLive:      2419200,
		DelayWhileIdle:  true,
		RegistrationIDs: ids,
		Data:            make(map[string]interface{}),
	}
}

// AddRecipients
func (m *GCMMessage) AddRecipients(ids ...string) {
	m.RegistrationIDs = append(m.RegistrationIDs, ids...)
}

// SetPayload
func (m *GCMMessage) SetPayload(key string, value string) {
	if m.Data == nil {
		m.Data = make(map[string]interface{})
	}
	m.Data[key] = value
}

// Send
func (c *GCMClient) Send(m *GCMMessage) (*GCMResponse, error) {
	log.V(2).Infof("%+v", m)

	start := time.Now()
	defer func() { log.Info("Hermes.GCM.Send ", time.Since(start)) }()

	j, err := json.Marshal(m)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	request, err := http.NewRequest("POST", c.url, bytes.NewBuffer(j))
	if err != nil {
		log.Error(err)
		return nil, err
	}
	request.Header.Add("Authorization", fmt.Sprintf("key=%s", c.key))
	request.Header.Add("Content-Type", "application/json")

	resp, err := c.http.Do(request)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	ret := GCMResponse{StatusCode: resp.StatusCode}

	switch resp.StatusCode {
	case 503, 500:
		after := resp.Header.Get("Retry-After")
		sleepFor, e := strconv.Atoi(after)
		if e != nil {
			log.Error(e)
		}
		ret.RetryAfter = sleepFor
	case 401:
		return nil, fmt.Errorf("unauthorized %s %s", resp.Status, string(body))
	case 400:
		return nil, fmt.Errorf("malformed JSON %s %s", resp.Status, string(body))
	case 200:
		err = json.Unmarshal(body, &ret)
		if err != nil {
			log.Error(err, string(body))
		}
	default:
		return nil, fmt.Errorf("unknown Error %s %s", resp.Status, string(body))
	}

	return &ret, err
}

// Return the indexes of successfully sent registration ids
func (r *GCMResponse) SuccessIndexes() []int {
	ret := make([]int, 0, r.Success)
	for i, result := range r.Results {
		if result.Error == "" {
			ret = append(ret, i)
		}
	}
	return ret
}

// Return the indexes of failed sent registration ids
func (r *GCMResponse) ErrorIndexes() []int {
	ret := make([]int, 0, r.Failure)
	for i, result := range r.Results {
		if result.Error != "" {
			ret = append(ret, i)
		}
	}
	return ret
}

// Return the indexes of registration ids which need update
func (r *GCMResponse) RefreshIndexes() []int {
	ret := make([]int, 0, r.CanonicalIDs)
	for i, result := range r.Results {
		if result.RegistrationID != "" {
			ret = append(ret, i)
		}
	}
	return ret
}

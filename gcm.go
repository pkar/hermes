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

// GCMURLs map environment to gcm url.
var GCMURLs = map[string]string{
	"testing":         "http://localhost:5556",
	"development":     "https://android.googleapis.com/gcm/send",
	"staging":         "https://android.googleapis.com/gcm/send",
	"staging_sandbox": "https://android.googleapis.com/gcm/send",
	"sandbox":         "https://android.googleapis.com/gcm/send",
	"production":      "https://android.googleapis.com/gcm/send",
}

// GCMMessage http://developer.android.com/guide/google/gcm/gcm.html#send-msg
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

// GCMResult embedded response from gcm.
type GCMResult struct {
	MessageID      string `json:"message_id"`
	RegistrationID string `json:"registration_id"`
	Error          string `json:"error"`
}

// GCMResponse http://developer.android.com/guide/google/gcm/gcm.html#send-msg
type GCMResponse struct {
	MulticastID  int64        `json:"multicast_id"`
	Success      int          `json:"success"`
	Failure      int          `json:"failure"`
	CanonicalIDs int          `json:"canonical_ids"`
	Results      []*GCMResult `json:"results"`
	StatusCode   int          `json:"status_code"`
	// set to -1 initially, if >= 0 then retry.
	RetryAfter int   `json:"retry_after"`
	Error      error `json:"error"`
}

// Bytes implements interface Response.
func (g *GCMResponse) Bytes() ([]byte, error) {
	return json.Marshal(g)
}

// Retry implements interface Response.
func (g *GCMResponse) Retry() int {
	return g.RetryAfter
}

// UpdateToken implements interface Response.
func (g *GCMResponse) UpdateToken() bool {
	if g == nil {
		return false
	}
	if g.Error == ErrRemoveToken || g.Error == ErrUpdateToken {
		return true
	}
	return false
}

// GCMClient ...
type GCMClient struct {
	key  string
	http *http.Client
	url  string
}

// NewGCMClient ...
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

// NewGCMMessage ...
func NewGCMMessage(ids ...string) *GCMMessage {
	return &GCMMessage{
		TimeToLive:      2419200,
		DelayWhileIdle:  true,
		RegistrationIDs: ids,
		Data:            make(map[string]interface{}),
	}
}

// AddRecipients ...
func (g *GCMMessage) AddRecipients(ids ...string) {
	g.RegistrationIDs = append(g.RegistrationIDs, ids...)
}

// SetPayload ...
func (g *GCMMessage) SetPayload(key string, value string) {
	if g.Data == nil {
		g.Data = make(map[string]interface{})
	}
	g.Data[key] = value
}

// Send ...
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

	ret := GCMResponse{RetryAfter: -1}

	switch resp.StatusCode {
	case 503, 500:
		after := resp.Header.Get("Retry-After")
		sleepFor, e := strconv.Atoi(after)
		if e != nil {
			log.Error(e)
		}
		ret.RetryAfter = sleepFor
		ret.Error = ErrRetry
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
		after := resp.Header.Get("Retry-After")
		sleepFor, e := strconv.Atoi(after)
		if e != nil {
			log.Error(e)
		}
		ret.RetryAfter = sleepFor
		ret.Error = ErrRetry
		log.Errorf("unknown error %s %s", resp.Status, string(body))
		return &ret, ret.Error
	}

	refresh := ret.RefreshIndexes()
	if len(refresh) > 0 {
		ret.Error = ErrUpdateToken
	}

	errs := ret.ErrorIndexes()
	if len(errs) > 0 {
		ret.Error = ErrRemoveToken
	}

	ret.StatusCode = resp.StatusCode
	return &ret, ret.Error
}

// SuccessIndexes return the indexes of successfully sent registration ids.
func (g *GCMResponse) SuccessIndexes() []int {
	ret := make([]int, 0, g.Success)
	for i, result := range g.Results {
		if result.Error == "" {
			ret = append(ret, i)
		}
	}
	return ret
}

// ErrorIndexes return the indexes of failed sent registration ids.
func (g *GCMResponse) ErrorIndexes() []int {
	ret := make([]int, 0, g.Failure)
	for i, result := range g.Results {
		if result.Error != "" {
			ret = append(ret, i)
		}
	}
	return ret
}

// RefreshIndexes return the indexes of registration ids which need update.
func (g *GCMResponse) RefreshIndexes() []int {
	ret := make([]int, 0, g.CanonicalIDs)
	for i, result := range g.Results {
		if result.RegistrationID != "" {
			ret = append(ret, i)
		}
	}
	return ret
}

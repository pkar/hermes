package hermes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// GCMURLs map environment to gcm url.
var GCMURLs = map[string]string{
	"testing":     "http://localhost:5556",
	"development": "https://android.googleapis.com/gcm/send",
	"staging":     "https://android.googleapis.com/gcm/send",
	"production":  "https://android.googleapis.com/gcm/send",
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
	RetryAfter   int   `json:"retry_after"`
	Error        error `json:"error"`
	ResponseTime int64 `json:"response_time"` // time in milliseconds
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
func NewGCMClient(apiURL, key, proxy string) (*GCMClient, error) {
	if apiURL == "" {
		return nil, fmt.Errorf("url not provided")
	}
	tr := &http.Transport{
		Dial: func(netw, addr string) (net.Conn, error) {
			deadline := time.Now().Add(20 * time.Second)
			c, err := net.DialTimeout(netw, addr, time.Second*20)
			if err != nil {
				return nil, err
			}
			c.SetDeadline(deadline)
			return c, nil
		},
		DisableKeepAlives: true,
	}
	if proxy != "" {
		// http://proxyIp:proxyPort
		proxyUrl, err := url.Parse(proxy)
		if err != nil {
			return nil, err
		}
		tr.Proxy = http.ProxyURL(proxyUrl)
	}

	return &GCMClient{
		key:  key,
		http: &http.Client{Transport: tr},
		url:  apiURL,
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
	ret := GCMResponse{RetryAfter: -1}
	start := time.Now()
	defer func() { ret.ResponseTime = time.Since(start).Nanoseconds() / 1000000 }()
	j, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest("POST", c.url, bytes.NewBuffer(j))
	if err != nil {
		return nil, err
	}
	request.Header.Add("Authorization", fmt.Sprintf("key=%s", c.key))
	request.Header.Add("Content-Type", "application/json")

	resp, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	switch {
	case resp.StatusCode >= 500 && resp.StatusCode <= 599:
		// Errors in the 500-599 range (such as 500 or 503) indicate that
		// there # was an internal error in the GCM connection server
		// while trying to process the request, or that the
		// server is temporarily unavailable
		// (for example, because of timeouts). Sender must retry later,
		// honoring any Retry-After header included in the response.
		after := resp.Header.Get("Retry-After")
		sleepFor, _ := strconv.Atoi(after)
		ret.RetryAfter = sleepFor
		ret.Error = ErrRetry
	case resp.StatusCode == 401:
		return nil, fmt.Errorf("unauthorized %s %s", resp.Status, string(body))
	case resp.StatusCode == 400:
		// Indicates that the request could not be parsed as JSON,
		// or it contained invalid fields (for instance, passing a
		// string where a number was expected). The exact failure
		// reason is described in the response and the problem
		// should be addressed before the request can be retried.
		return nil, fmt.Errorf("malformed JSON %s %s", resp.Status, string(body))
	case resp.StatusCode == 200:
		err = json.Unmarshal(body, &ret)
		if err != nil {
			return nil, err
		}
	default:
		after := resp.Header.Get("Retry-After")
		sleepFor, _ := strconv.Atoi(after)
		ret.RetryAfter = sleepFor
		ret.Error = ErrRetry
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

package hermes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
)

var (
	// ADMPath requires the registration url to be injected
	ADMPath = "/messaging/registrations/%s/messages"
)

// ADMURLs map environment to adm url.
var ADMURLs = map[string]string{
	"testing":         "localhost:5556",
	"development":     "https://api.amazon.com",
	"staging":         "https://api.amazon.com",
	"staging_sandbox": "https://api.amazon.com",
	"sandbox":         "https://api.amazon.com",
	"production":      "https://api.amazon.com",
}

// ADMMessage https://developer.amazon.com/public/apis/engage/device-messaging/tech-docs/06-sending-a-message
type ADMMessage struct {
	Data             map[string]string `json:"data"`
	ConsolidationKey string            `json:"consolidationKey"`
	Expires          int               `json:"expiresAfter"`
	RegistrationID   string            `json:"-"`
}

// Bytes implements interface Message.
func (a *ADMMessage) Bytes() ([]byte, error) {
	return json.Marshal(a)
}

// NewADMMessage ...
func NewADMMessage(id string) *ADMMessage {
	return &ADMMessage{
		RegistrationID: id,
		Data:           make(map[string]string),
	}
}

// ADMResponse https://developer.amazon.com/public/apis/engage/device-messaging/tech-docs/06-sending-a-message
type ADMResponse struct {
	StatusCode int   `json:"statusCode"`
	Error      error `json:"error"`
	// The calculated base-64-encoded MD5 checksum of the data field.
	MD5 string `json:"md5"`
	// A value created by ADM that uniquely identifies the request.
	// In the unlikely event that you have problems with ADM,
	// Amazon can use this value to troubleshoot the problem.
	RequestID string `json:"requestID"`
	// This field is returned in the case of a 429, 500, or 503 error response.
	// Retry-After specifies how long the service is expected to be unavailable.
	// This value can be either an integer number of seconds (in decimal) after
	// the time of the response or an HTTP-format date. See the HTTP/1.1
	// specification, section 14.37, for possible formats for this value.
	RetryAfter int `json:"retryAfter"`
	// The current registration ID of the app instance.
	// If this value is different than the one passed in by
	// your server, your server must update its records to use this value.
	RegistrationID string `json:"registrationID"`
	// 400
	//   InvalidRegistrationId
	//   InvalidData
	//   InvalidConsolidationKey
	//   InvalidExpiration
	//   InvalidChecksum
	//   InvalidType
	//   Unregistered
	// 401
	//   AccessTokenExpired
	// 413
	//   MessageTooLarge
	// 429
	//   MaxRateExceeded
	// 500
	//   n/a
	Reason string `json:"reason"`
}

// Bytes implements interface Response.
func (a *ADMResponse) Bytes() ([]byte, error) {
	return json.Marshal(a)
}

// Retry implements interface Response.
func (a *ADMResponse) Retry() int {
	return a.RetryAfter
}

// UpdateToken implements interface Response.
func (a *ADMResponse) UpdateToken() bool {
	if a == nil {
		return false
	}
	if a.Error == ErrRemoveToken || a.Error == ErrUpdateToken {
		return true
	}
	return false
}

// ADMClient ...
type ADMClient struct {
	Key  string
	http *http.Client
	url  string
}

// NewADMClient ...
func NewADMClient(url, key string) (*ADMClient, error) {
	if url == "" {
		return nil, fmt.Errorf("url not provided")
	}
	return &ADMClient{
		Key:  key,
		http: &http.Client{},
		url:  url,
	}, nil
}

// Send ...
func (c *ADMClient) Send(m *ADMMessage) (*ADMResponse, error) {
	j, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest("POST", fmt.Sprintf(c.url+ADMPath, m.RegistrationID), bytes.NewBuffer(j))
	if err != nil {
		return nil, err
	}
	request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Key))
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Accept", "application/json")
	request.Header.Add("X-Amzn-Type-Version ", "com.amazon.device.messaging.ADMMessage@1.0")
	request.Header.Add("X-Amzn-Accept-Type", "com.amazon.device.messaging.ADMSendResult@1.0")

	resp, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	ret := &ADMResponse{StatusCode: resp.StatusCode}
	switch resp.StatusCode {
	case 503, 500:
		// n/a
		ret.RetryAfter = 0
		after := resp.Header.Get("Retry-After")
		i, err := strconv.Atoi(after)
		if err != nil {
			ret.RetryAfter = i
		}
		ret.Error = ErrRetry
	case 429:
		// MaxRateExceeded
		ret.RetryAfter = 0
		err = json.Unmarshal(body, &ret)
		if err != nil {
			return nil, err
		}

		after := resp.Header.Get("Retry-After")
		i, err := strconv.Atoi(after)
		if err != nil {
			ret.RetryAfter = i
		}
		ret.Error = ErrRetry
	case 413:
		// MessageTooLarge
		err = json.Unmarshal(body, &ret)
		if err != nil {
			return nil, err
		}
	case 401:
		// AccessTokenExpired
		err = json.Unmarshal(body, &ret)
		if err != nil {
			return nil, err
		}
		ret.Error = ErrTokenExpired
	case 400:
		err = json.Unmarshal(body, &ret)
		if err != nil {
			return nil, err
		}
		switch ret.Reason {
		case "InvalidRegistrationId":
			ret.Error = ErrRemoveToken
		}
	case 200:
		err = json.Unmarshal(body, &ret)
		if err != nil {
			return nil, err
		}
	default:
		after := resp.Header.Get("Retry-After")
		i, err := strconv.Atoi(after)
		if err != nil {
			ret.RetryAfter = i
		}
		ret.Error = ErrRetry
	}
	ret.RequestID = resp.Header.Get("X-Amzn-RequestId")
	ret.MD5 = resp.Header.Get("X-Amzn-Data-md5")
	ret.StatusCode = resp.StatusCode
	return ret, ret.Error
}

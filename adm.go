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

var (
	ADMURL = "https://api.amazon.com"
	// ADMPath requires the registration url to be injected
	ADMPath = "/messaging/registrations/%s/messages"
)

// ADMMessage
// https://developer.amazon.com/public/apis/engage/device-messaging/tech-docs/06-sending-a-message
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

// ADMResponse
// https://developer.amazon.com/public/apis/engage/device-messaging/tech-docs/06-sending-a-message
type ADMResponse struct {
	StatusCode int `json:"statusCode"`
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
	RetryAfter interface{} `json:"retryAfter"`
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

// ADMClient ...
type ADMClient struct {
	key  string
	http *http.Client
	url  string
}

// NewADMClient ...
func NewADMClient(url, key string) (*ADMClient, error) {
	if url == "" {
		return nil, fmt.Errorf("url not provided")
	}
	return &ADMClient{
		key:  key,
		http: &http.Client{},
		url:  url,
	}, nil
}

// Send ...
func (c *ADMClient) Send(m *ADMMessage) (*ADMResponse, error) {
	start := time.Now()
	defer func() { log.Info("Hermes.ADM.Send ", time.Since(start)) }()

	j, err := json.Marshal(m)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	request, err := http.NewRequest("POST", fmt.Sprintf(c.url+ADMPath, m.RegistrationID), bytes.NewBuffer(j))
	if err != nil {
		log.Error(err)
		return nil, err
	}
	request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.key))
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Accept", "application/json")
	request.Header.Add("X-Amzn-Type-Version ", "com.amazon.device.messaging.ADMMessage@1.0")
	request.Header.Add("X-Amzn-Accept-Type", "com.amazon.device.messaging.ADMSendResult@1.0")

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

	ret := &ADMResponse{}
	switch resp.StatusCode {
	case 503, 500:
		// n/a
		after := resp.Header.Get("Retry-After")
		ret.RetryAfter = after
		i, err := strconv.Atoi(after)
		if err != nil {
			ret.RetryAfter = i
		}
	case 429:
		// MaxRateExceeded
		after := resp.Header.Get("Retry-After")
		ret.RetryAfter = after
		i, err := strconv.Atoi(after)
		if err != nil {
			ret.RetryAfter = i
		}

		err = json.Unmarshal(body, &ret)
		if err != nil {
			log.Error(err, string(body))
			return nil, err
		}
	case 413:
		// MessageTooLarge
		err = json.Unmarshal(body, &ret)
		if err != nil {
			log.Error(err, string(body))
			return nil, err
		}
	case 401:
		// AccessTokenExpired
		err = json.Unmarshal(body, &ret)
		if err != nil {
			log.Error(err, string(body))
			return nil, err
		}
	case 400:
		err = json.Unmarshal(body, &ret)
		if err != nil {
			log.Error(err, string(body))
			return nil, err
		}
	case 200:
		err = json.Unmarshal(body, &ret)
		if err != nil {
			log.Error(err, string(body))
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown response: %d %s", resp.StatusCode, string(body))
	}
	ret.RequestID = resp.Header.Get("X-Amzn-RequestId")
	ret.MD5 = resp.Header.Get("X-Amzn-Data-md5")
	ret.StatusCode = resp.StatusCode
	return ret, nil
}

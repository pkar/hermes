# hermes

[![wercker status](https://app.wercker.com/status/68c5ce741bbee3ff8772758a0d7044d1/m "wercker status")](https://app.wercker.com/project/bykey/68c5ce741bbee3ff8772758a0d7044d1)

Send push notifications to apns, gcm, c2dm, or adm. queue not included....

apns
```go
c, _ := NewAPNSClient(APNSGateway, APNSCertMock, APNSKeyMock)

ap := &APNSMessage{
	Alert: "hello", // or dict
	Badge: 40,
	Sound: "bingbong.aiff",
}
apn, _ := NewAPNSPushNotification("E70331D08A2DA3BD02415DB2CAA4D7EEEC77FA2E5513B16F4F9E79C0BF89AED4", ap, 0)
apn.Set("custom_field", []interface{}{0, "1234"}) // set a custom field into the payload.
resp, err := c.Send(apn)
```

gcm
```go
c, _ := NewGCMClient(GCMServer.URL, "abc")
	
m := GCMMessage{Data: map[string]interface{}{"a": "b"}}
m.AddRecipients("1", "2", "3")
resp, _ := c.Send(&m)
	
```

c2dm
```go
c, _ := NewC2DMClient(GCMServer.URL, "abc")

m := C2DMMessage{
	RegistrationID: "abc",
	Data:           map[string]interface{}{"a": "b"},
}
resp, _ := c.Send(&m)
```

adm requires the server to update it's access token peridically. One might do this
using a ticker.
```go
var admToken string
var ClientID := "client id"
var ClientSecret := "client secret"

type ADMResponse struct {
	AccessToken string `json:"access_token"`
	Expires     int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

// run periodically updates the adm access token.
func run() {
	go updateToken()

	ticker := time.NewTicker(time.Second * 1800)
	for {
		select {
		case <-ticker.C:
			attempts := 0
			for {
				if attempts > 5 {
					// give up and try at next tick
					break
				}
				err := updateToken()
				if err != nil {
					log.Error(err)
					time.Sleep(time.Second)
					attempts++
					continue
				}
				break
			}
		}
	}
}


// updateToken updates the adm access token.
func (a *ADM) updateToken() error {
	log.Info("getting new access token")

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("scope", "messaging:push")
	data.Set("client_id", ClientID)
	data.Set("client_secret", ClientSecret)

	request, err := http.NewRequest("POST", "https://api.amazon.com/auth/O2/token", strings.NewReader(data.Encode()))
	if err != nil {
		log.Error(err)
		return err
	}
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		log.Errorf("%s %+v", err, resp)
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return err
	}

	r := ADMResponse{}
	err = json.Unmarshal(body, &r)
	if err != nil {
		log.Error(err, string(body))
		return err
	}
	log.Infof("adm %+v", r)
	if admToken != "" {
		admToken = r.AccessToken
	}
	return nil
}

c, _ := NewADMClient(ADMServer.URL, admToken)
m := ADMMessage{
	Data: map[string]string{
		"test":  "test",
		"test2": "test2",
	},
	ConsolidationKey: "testing",
	Expires:          86400,
	RegistrationID:   "amzn1.adm-registration.v1.123",
}
resp, err := c.Send(&m)
```

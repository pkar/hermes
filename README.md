# hermes

[![wercker status](https://app.wercker.com/status/68c5ce741bbee3ff8772758a0d7044d1/m "wercker status")](https://app.wercker.com/project/bykey/68c5ce741bbee3ff8772758a0d7044d1)

Send push notifications to apns, gcm, c2dm, or adm

apns
```go
c, _ := NewAPNSClient(test.APNSGateway, test.APNSCertMock, test.APNSKeyMock)

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
c2dm
adm

still a work in progress

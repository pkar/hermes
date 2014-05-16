/*
Package hermes implements APNS and GCM clients.

To send to APNS, first create a client with the cert and key for the app,
then create a notification with a payload to be sent. The alert can be
a string or dict.

	c, _ := NewAPNSClient(test.APNSGateway, test.APNSCertMock, test.APNSKeyMock)

	// dict := &APNSAlertDictionary {
	// 	Body : "Alice wants Bob to join in the fun!",
	// 	ActionLocKey : "Play a Game!",
	// 	LocKey : "localized key",
	// 	LocArgs : args,
	// 	LaunchImage : "image.jpg",
	// }
	ap := &APNSMessage{
		Alert: "hello", // or dict
		Badge: 40,
		Sound: "bingbong.aiff",
	}
	apn, _ := NewAPNSPushNotification("E70331D08A2DA3BD02415DB2CAA4D7EEEC77FA2E5513B16F4F9E79C0BF89AED4", ap, 0)
	apn.Set("custom_field", []interface{}{0, "1234"}) // set a custom field into the payload.
	resp, err := c.Send(apn)


*/
package hermes

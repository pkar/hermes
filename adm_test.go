package hermes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	log "github.com/golang/glog"
)

var ADMServer *httptest.Server

func init() {
	ADMServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := ADMResponse{
			RegistrationID: "amzn1.adm-registration.v1.Y29tLmFtYXpvbi5EZXZpY2VNZXNzYWdpbmcuYL3FOMUlCWEdpdm5TZ3RWbm9XUT0hN0lrSU1YUlNSVVBpT2pOd0lnWktvUT09",
			Reason:         "",
		}
		j, err := json.Marshal(resp)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Fprintln(w, string(j))
	}))
}

func TestNewADMClient(t *testing.T) {
	c, err := NewADMClient(ADMServer.URL, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if c.Key != "abc" {
		t.Fatal("client not initialized")
	}
}

func TestADMSend(t *testing.T) {
	c, err := NewADMClient(ADMServer.URL, "abc")
	if err != nil {
		t.Fatal(err)
	}
	m := ADMMessage{
		Data: map[string]string{
			"test":  "test",
			"test2": "test2",
		},
		ConsolidationKey: "testing",
		Expires:          86400,
		RegistrationID:   "amzn1.adm-registration.v1.Y29tLmFtYXpvbi5EZXZpY2VNZXNzYWdpbmcuYL3FOMUlCWEdpdm5TZ3RWbm9XUT0hN0lrSU1YUlNSVVBpT2pOd0lnWktvUT09",
	}
	r, err := c.Send(&m)
	if err != nil {
		t.Fatal(err)
	}
	if r.StatusCode != 200 {
		t.Fatalf("didn't get back 200 got: %+v", r)
	}
}

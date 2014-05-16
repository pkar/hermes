package hermes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	log "github.com/golang/glog"
)

var C2DMServer *httptest.Server

func init() {
	C2DMServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := C2DMResponse{}
		j, err := json.Marshal(resp)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Fprintln(w, string(j))
	}))
}

func TestNewC2DMClient(t *testing.T) {
	c, err := NewC2DMClient(C2DMServer.URL, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if c.key != "abc" {
		t.Fatal("client not initialized")
	}
}

func TestC2DMSend(t *testing.T) {
	c, err := NewC2DMClient(GCMServer.URL, "abc")
	if err != nil {
		t.Fatal(err)
	}

	m := C2DMMessage{
		RegistrationID: "abc",
		Data:           map[string]interface{}{"a": "b"},
	}
	r, err := c.Send(&m)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", r)
}

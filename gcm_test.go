package hermes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	log "github.com/golang/glog"
)

var GCMServer *httptest.Server

func init() {
	GCMServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := GCMResponse{
			StatusCode: 200,
		}
		j, err := json.Marshal(resp)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Fprintln(w, string(j))
	}))
}

func TestNewGCMClient(t *testing.T) {
	c, err := NewGCMClient(GCMServer.URL, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if c.key != "abc" {
		t.Fatal("client not initialized")
	}
}

func TestGCMSend(t *testing.T) {
	c, err := NewGCMClient(GCMServer.URL, "abc")
	if err != nil {
		t.Fatal(err)
	}

	m := GCMMessage{}
	r, err := c.Send(&m)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", r)
}

func TestGCMSetPayload(t *testing.T) {
	m := GCMMessage{}
	m.SetPayload("data", "3")
	if d, ok := m.Data["data"]; ok {
		if d != "3" {
			t.Fatal("message data not set")
		}
	} else {
		t.Fatal("message data not set")
	}
}

func TestGCMAddRecipients(t *testing.T) {
	m := GCMMessage{}
	m.AddRecipients("1", "2", "3")
	if len(m.RegistrationIDs) != 3 {
		t.Fatal("registration ids not set")
	}
}

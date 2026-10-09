package upstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDedicatedDoesNotRetainSessionAfterConnectFailure(t *testing.T) {
	var requests atomic.Int32
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, "unavailable", http.StatusBadGateway)
	}))
	defer peer.Close()
	d, err := NewDedicated(DedicatedOptions{Server: ServerOptions{ID: "wire", Endpoint: peer.URL, ConnectTimeout: time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	previous := int32(0)
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err := d.Discover(context.Background()); err == nil {
			t.Fatalf("Discover attempt %d unexpectedly succeeded", attempt)
		}
		if got := requests.Load(); got <= previous {
			t.Fatalf("requests after attempt %d = %d, want another Connect attempt after failure", attempt, got)
		} else {
			previous = got
		}
	}
}

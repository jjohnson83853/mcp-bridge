package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

func TestFailureOutcomeClassifiesExecutionErrors(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		code  string
		state string
	}{
		{"known before execution", &CallError{Code: "invalid_arguments", ExecutionState: core.ExecutionNotStarted}, "invalid_arguments", core.ExecutionNotStarted},
		{"known after admission", &CallError{Code: "upstream_protocol_error", ExecutionState: "completed"}, "upstream_protocol_error", core.ExecutionUnknown},
		{"busy", errBusy, "backend_busy", core.ExecutionNotStarted},
		{"unavailable", errUnavailable, "backend_unavailable", core.ExecutionNotStarted},
		{"deadline", context.DeadlineExceeded, "deadline_exceeded", core.ExecutionUnknown},
		{"canceled", context.Canceled, "internal_error", core.ExecutionUnknown},
		{"unknown", errors.New("transport failed"), "backend_unavailable", core.ExecutionNotStarted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := failureOutcome(tt.err)
			if got.Code != tt.code || got.ExecutionState != tt.state {
				t.Fatalf("failureOutcome(%v) = %+v", tt.err, got)
			}
		})
	}
}

func TestDedicatedAccessorsAndDiscoveryProtocolFailures(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		contentType string
		body        string
	}{
		{"non-200", http.StatusBadGateway, "application/json", "{}"},
		{"wrong content type", http.StatusOK, "text/plain", "{}"},
		{"malformed body", http.StatusOK, "application/json", "not-json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer peer.Close()
			options := ServerOptions{ID: "catalog", Endpoint: peer.URL}
			assertDedicatedRequiresLogger(t, options)
			d, err := NewDedicated(DedicatedOptions{Logger: observe.NewLogger(nil, nil), Server: options})
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			if d.ID() != "catalog" || d.Endpoint() != peer.URL || d.Client() == nil {
				t.Fatalf("unexpected runtime identity: %q %q", d.ID(), d.Endpoint())
			}
			defs, err := d.Discover(context.Background())
			if err == nil || defs != nil {
				t.Fatalf("Discover() = (%v, %v), want error and no partial catalog", defs, err)
			}
		})
	}
}

func TestDedicatedRejectsMismatchedIdentityBeforeConnect(t *testing.T) {
	options := ServerOptions{ID: "actual", Endpoint: "http://127.0.0.1:1"}
	assertDedicatedRequiresLogger(t, options)
	d, err := NewDedicated(DedicatedOptions{Logger: observe.NewLogger(nil, nil), Server: options})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	result, outcome := d.CallTool(context.Background(), "", "other", "tool", nil)
	if result != nil || outcome == nil || outcome.Failure == nil || outcome.Failure.Code != "unknown_tool" || outcome.Failure.ExecutionState != core.ExecutionNotStarted {
		t.Fatalf("CallTool() = (%s, %+v)", result, outcome)
	}
}
func assertDedicatedRequiresLogger(t *testing.T, options ServerOptions) {
	t.Helper()
	dedicated, err := NewDedicated(DedicatedOptions{Server: options})
	if err == nil || dedicated != nil {
		t.Fatalf("NewDedicated without logger = (%v, %v); want nil and an error", dedicated, err)
	}
}
func TestDedicatedHandshakeOmitsHostileProtocolVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	dedicated, err := NewDedicated(DedicatedOptions{
		Logger: observe.NewLogger(&stdout, &stderr),
		Server: ServerOptions{ID: "alpha", Endpoint: "http://127.0.0.1:1"},
	})
	if err != nil {
		t.Fatalf("NewDedicated: %v", err)
	}
	defer dedicated.Close()
	dedicated.logEphemeralHandshake(time.Now().Add(-time.Second), "9.9.9-evil")

	if stderr.Len() != 0 || bytes.Count(stdout.Bytes(), []byte{'\n'}) != 1 {
		t.Fatalf("handshake streams stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	var record map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &record); err != nil {
		t.Fatalf("decode handshake envelope: %v: %q", err, stdout.String())
	}
	wantKeys := map[string]struct{}{
		"schema_version": {}, "level": {}, "service": {}, "event": {}, "message": {},
		"operation": {}, "outcome": {}, "backend_id": {}, "duration_ms": {},
	}
	if len(record) != len(wantKeys) {
		t.Fatalf("handshake keys=%v; want exactly %v", record, wantKeys)
	}
	for key := range record {
		if _, ok := wantKeys[key]; !ok {
			t.Errorf("unexpected handshake field %q", key)
		}
	}
	for key, want := range map[string]string{
		"schema_version": "1", "level": `"info"`, "service": `"mcp-bridge"`,
		"event": `"upstream_ephemeral_handshake"`, "message": `"upstream protocol handshake observed"`,
		"operation": `"handshake"`, "outcome": `"success"`, "backend_id": `"alpha"`,
	} {
		if string(record[key]) != want {
			t.Errorf("handshake %s = %s; want %s", key, record[key], want)
		}
	}
	if _, present := record["protocol_version"]; present || bytes.Contains(stdout.Bytes(), []byte("9.9.9-evil")) {
		t.Errorf("hostile protocol_version was emitted: %s", stdout.String())
	}
}

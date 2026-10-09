package upstream

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
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
			d, err := NewDedicated(DedicatedOptions{Server: ServerOptions{ID: "catalog", Endpoint: peer.URL}})
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
	d, err := NewDedicated(DedicatedOptions{Server: ServerOptions{ID: "actual", Endpoint: "http://127.0.0.1:1"}})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	result, outcome := d.CallTool(context.Background(), "", "other", "tool", nil)
	if result != nil || outcome == nil || outcome.Failure == nil || outcome.Failure.Code != "unknown_tool" || outcome.Failure.ExecutionState != core.ExecutionNotStarted {
		t.Fatalf("CallTool() = (%s, %+v)", result, outcome)
	}
}

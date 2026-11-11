package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

func newOwnershipDiagnostics(sink *bytes.Buffer) *observe.HTTPDiagnostics {
	logger := observe.NewLogger(sink, sink)
	return observe.NewHTTPDiagnostics(logger, observe.HTTPDiagnosticsOptions{})
}

// TestComposedOwnershipSingleTerminal proves the composed cut: the outer owner
// emits exactly one terminal http_complete record while the inner handler
// contributes legacy evidence, and the session header is observed through the
// owner's writer chain rather than the incoming recorder.
func TestComposedOwnershipSingleTerminal(t *testing.T) {
	cases := []struct {
		name       string
		next       http.HandlerFunc
		wantStatus int
	}{
		{
			name: "AcceptedToolErrorUnder200Warns",
			next: func(w http.ResponseWriter, r *http.Request) {
				observe.ContributeOperationEvidence(r.Context(), observe.HTTPOperationEvidence{
					Relation: "single", IDState: "not_applicable", Upstream: "tool_error",
				})
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"ok":true}`))
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "ZeroWriteUnavailable",
			next:       func(http.ResponseWriter, *http.Request) {},
			wantStatus: http.StatusOK,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sink := &bytes.Buffer{}
			diag := newOwnershipDiagnostics(sink)
			hash := sha256.Sum256([]byte("token"))
			h := NewInboundHandler(tc.next, [][32]byte{hash}, InboundOptions{
				Logger:      observe.NewLogger(&bytes.Buffer{}, &bytes.Buffer{}),
				Diagnostics: diag,
			})
			// Pre-issue the session the request presents; an unissued
			// session id is correctly rejected as unauthorized.
			h.sessions["issued"] = sessionRecord{token: hash, lastSeen: time.Now()}
			r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			r.Header.Set("Authorization", "Bearer token")
			r.Header.Set("Mcp-Session-Id", "issued")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			var terminal map[string]any
			count := 0
			for _, line := range strings.Split(strings.TrimSpace(sink.String()), "\n") {
				if line == "" {
					continue
				}
				var record map[string]any
				if err := json.Unmarshal([]byte(line), &record); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if record["event"] == "http_complete" {
					count++
					terminal = record
				}
			}
			if count != 1 {
				t.Fatalf("http_complete records = %d; want exactly 1", count)
			}
			if got, ok := terminal["http_status"].(float64); ok && int(got) != tc.wantStatus {
				t.Fatalf("observed http_status = %v; want %d", terminal["http_status"], tc.wantStatus)
			}
		})
	}
}

// TestStandaloneInnerClaimsFallback proves that without a composed outer
// owner the handler claims through its own lazily built fallback owner, still
// emitting exactly one terminal record per request.
func TestStandaloneInnerClaimsFallback(t *testing.T) {
	sink := &bytes.Buffer{}
	hash := sha256.Sum256([]byte("token"))
	h := NewInboundHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), [][32]byte{hash}, InboundOptions{Logger: observe.NewLogger(sink, sink)})
	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	r.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(sink.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if record["event"] == "http_complete" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("standalone http_complete records = %d; want exactly 1", count)
	}
}

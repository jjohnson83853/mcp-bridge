package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

const (
	inboundFaultPublicID = "11223344556677889900aabbccddeeff"
	inboundFaultOwnedID  = "ffeeddccbbaa00998877665544332211"
	inboundFaultToken    = "fault-fixture-accepted-bearer"
	inboundFaultBadToken = "fault-fixture-rejected-bearer"
	inboundFaultProse    = "synthetic backend authentication failed bearer-secret"
)

// TestInboundLoggingFaults supplements, rather than replaces, the real-root
// wire tests. Its empty generator proves owner behavior given an unavailable
// result, NOT an OS entropy failure or recovery from crypto/rand.Read's fatal.
func TestInboundLoggingFaults(t *testing.T) {
	t.Run("UnavailableGeneratorResult", func(t *testing.T) {
		delegations := 0
		fixture := newInboundLoggingFaultFixture(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			delegations++
		}))
		fixture.handler.generator = func() string {
			fixture.generatorCalls++
			return ""
		}
		request := fixture.request(http.MethodPost)
		request.Header.Set("Authorization", "Bearer "+inboundFaultBadToken)
		response := httptest.NewRecorder()
		fixture.handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized || response.Body.String() != "unauthorized\n" {
			t.Error("unavailable generator must preserve the existing 401 response and body")
		}
		if values := response.Header().Values("WWW-Authenticate"); len(values) != 1 || values[0] != "Bearer" {
			t.Error("unavailable generator must preserve the single Bearer challenge")
		}
		assertInboundFaultPublicID(t, response.Header())
		if delegations != 0 {
			t.Error("rejected bearer must not execute the delegated handler")
		}
		record := fixture.terminal(t)
		assertInboundFaultFields(t, record, map[string]any{
			"correlation_state": "unavailable", "http_status": float64(401),
			"http_method": "POST", "http_outcome": "failure", "outcome": "failure",
		})
		if _, exists := record["request_id"]; exists {
			t.Error("unavailable internal request_id must be omitted, never reminted")
		}
		assertInboundFaultClass(t, record, "correlation_unavailable")
	})

	for _, tc := range []struct {
		name   string
		cause  error
		reason string
	}{
		{"PositiveWriteFailure", net.ErrClosed, "client_disconnected"},
		{"NonDisconnectWriteFailure", http.ErrContentLength, "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeErr := fmt.Errorf("%s: %w", inboundFaultProse, tc.cause)
			writer := &inboundFaultResponseWriter{header: make(http.Header), writeN: 17, writeErr: writeErr}
			delegations := 0
			fixture := newInboundLoggingFaultFixture(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				delegations++
				w.Header().Set("Content-Length", "999999")
				w.WriteHeader(http.StatusOK)
				n, err := w.Write(bytes.Repeat([]byte{'x'}, 32768))
				if n != writer.writeN || err != writeErr {
					t.Error("delegated handler must receive the exact original partial count and error object")
				}
			}))
			fixture.handler.ServeHTTP(writer, fixture.request(http.MethodGet))
			if delegations != 1 || writer.writes != 1 || writer.status != http.StatusOK {
				t.Error("controlled delivery must delegate once and preserve committed status 200")
			}
			assertInboundFaultPublicID(t, writer.Header())
			record := fixture.terminal(t)
			assertInboundFaultFields(t, record, map[string]any{
				"request_id": inboundFaultOwnedID, "correlation_state": "available",
				"http_status": float64(200), "http_outcome": "incomplete",
				"stage": "response_delivery", "http_bytes_bucket": "le_1k",
			})
			assertInboundFaultClass(t, record, tc.reason)
		})
	}

	t.Run("FlushFirst", func(t *testing.T) {
		writer := &inboundFaultResponseWriter{header: make(http.Header)}
		fixture := newInboundLoggingFaultFixture(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Error("HTTP wrapper must preserve Flusher")
				return
			}
			flusher.Flush()
		}))
		fixture.handler.ServeHTTP(writer, fixture.request(http.MethodGet))
		if writer.flushes != 1 || writer.status != http.StatusOK || writer.writes != 0 {
			t.Error("Flush-first must actually delegate Flush and commit only implicit 200")
		}
		assertInboundFaultFields(t, fixture.terminal(t), map[string]any{
			"http_status": float64(200), "http_outcome": "success",
			"stage": "none", "http_bytes_bucket": "empty",
		})
	})

	t.Run("NoResponseEvidence", func(t *testing.T) {
		writer := &inboundFaultResponseWriter{header: make(http.Header)}
		delegations := 0
		fixture := newInboundLoggingFaultFixture(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			delegations++
		}))
		fixture.handler.ServeHTTP(writer, fixture.request(http.MethodGet))
		if delegations != 1 || writer.status != 0 || writer.writes != 0 || writer.flushes != 0 {
			t.Error("no-response fixture must delegate once without observing any response call")
		}
		assertInboundFaultFields(t, fixture.terminal(t), map[string]any{
			"http_status": nil, "http_outcome": "unknown",
			"stage": "none", "http_bytes_bucket": "empty",
		})
	})

	t.Run("HTTPDurationIncludesGeneration", func(t *testing.T) {
		delegations := 0
		fixture := newInboundLoggingFaultFixture(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			delegations++
		}))
		now := time.Time{}
		const generationInterval = 37 * time.Millisecond
		fixture.handler.options.Now = func() time.Time { return now }
		fixture.handler.generator = func() string {
			fixture.generatorCalls++
			now = now.Add(generationInterval)
			return inboundFaultOwnedID
		}
		response := httptest.NewRecorder()
		// The unknown route exits before public ID/authentication/session work.
		// Only owned generation advances the existing deterministic clock.
		fixture.handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/unmatched", nil))
		if response.Code != http.StatusNotFound || delegations != 0 {
			t.Error("timer fixture must stop at unknown-route 404 without delegated work")
		}
		record := fixture.terminalForRoute(t, "not_found")
		assertInboundFaultFields(t, record, map[string]any{
			"http_status": float64(404), "http_outcome": "failure",
			"stage": "http_route",
			"duration_ms": float64(generationInterval.Milliseconds()),
		})
	})
}

type inboundLoggingFaultFixture struct {
	handler        *InboundHandler
	logs           bytes.Buffer
	generatorCalls int
}

func newInboundLoggingFaultFixture(next http.Handler) *inboundLoggingFaultFixture {
	fixture := &inboundLoggingFaultFixture{}
	fixture.handler = NewInboundHandler(next, [][32]byte{sha256.Sum256([]byte(inboundFaultToken))},
		InboundOptions{Logger: observe.NewLogger(&fixture.logs, &fixture.logs)})
	// Instance-only substitution is complete before ServeHTTP. No global random
	// source, constructor option, public setter, or runtime mutation is involved.
	fixture.handler.generator = func() string {
		fixture.generatorCalls++
		return inboundFaultOwnedID
	}
	return fixture
}

func (*inboundLoggingFaultFixture) request(method string) *http.Request {
	request := httptest.NewRequest(method, "/mcp", nil)
	request.Header.Set("Authorization", "Bearer "+inboundFaultToken)
	request.Header.Set("X-Request-ID", inboundFaultPublicID)
	return request
}

func (f *inboundLoggingFaultFixture) terminal(t *testing.T) map[string]any {
	t.Helper()
	return f.terminalForRoute(t, "mcp")
}

func (f *inboundLoggingFaultFixture) terminalForRoute(t *testing.T, route string) map[string]any {
	t.Helper()
	// Every caller synchronously returned from actual ServeHTTP: its deferred
	// terminal Log has completed. There are no client-EOF assumptions, pending
	// requests, background captures, listeners, or goroutines to clean up.
	if f.generatorCalls != 1 {
		t.Errorf("owned generator calls = %d; want exactly one", f.generatorCalls)
	}
	logs := f.logs.String()
	for _, forbidden := range []string{inboundFaultPublicID, inboundFaultToken, inboundFaultBadToken, inboundFaultProse} {
		if strings.Contains(logs, forbidden) {
			t.Error("complete JSON log must exclude public identity, bearer credentials, and synthetic error prose")
		}
	}
	var terminal map[string]any
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil || record == nil {
			t.Fatal("captured stdout must contain complete JSON object records")
		}
		switch record["event"] {
		case "http_complete":
			count++
			terminal = record
		case "mcp_complete", "backend_complete":
			t.Error("controlled HTTP boundary must not fabricate operation children or backend attempts")
		}
	}
	if count != 1 {
		t.Fatalf("HTTP terminal count = %d; want exactly one", count)
	}
	for _, key := range []string{"operation_id", "backend_call_id", "tool", "server", "tool_outcome", "backend_outcome"} {
		if _, exists := terminal[key]; exists {
			t.Errorf("HTTP terminal must omit unowned execution field %s", key)
		}
	}
	assertInboundFaultFields(t, terminal, map[string]any{"route": route, "timing_scope": "http"})
	return terminal
}

func assertInboundFaultFields(t *testing.T, record, expected map[string]any) {
	t.Helper()
	for key, want := range expected {
		got, present := record[key]
		if !present || got != want {
			t.Errorf("HTTP terminal field %s must equal the specified canonical value (including explicit null)", key)
		}
	}
}

func assertInboundFaultPublicID(t *testing.T, header http.Header) {
	t.Helper()
	if values := header.Values("X-Request-ID"); len(values) != 1 || values[0] != inboundFaultPublicID {
		t.Error("existing accepted public request ID must be echoed exactly once")
	}
}

func assertInboundFaultClass(t *testing.T, record map[string]any, class string) {
	t.Helper()
	detail, ok := record["error"].(map[string]any)
	if !ok || len(detail) != 4 || detail["code"] != class {
		t.Errorf("HTTP terminal error = %#v; want nested code=%s and the AD2 four-field shape", record["error"], class)
		return
	}
	if _, ok := detail["kind"].(string); !ok {
		t.Error("nested error.kind must be a string")
	}
	if _, ok := detail["retryable"].(bool); !ok {
		t.Error("nested error.retryable must be a boolean")
	}
	if remediation, ok := detail["remediation"].(string); !ok || remediation == "" {
		t.Error("nested error.remediation must be non-empty")
	}
	if _, exists := record["error_class"]; exists {
		t.Error("noncanonical top-level error_class must not replace nested error.code")
	}
}

type inboundFaultResponseWriter struct {
	header   http.Header
	status   int
	writes   int
	flushes  int
	writeN   int
	writeErr error
}

func (w *inboundFaultResponseWriter) Header() http.Header { return w.header }

func (w *inboundFaultResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *inboundFaultResponseWriter) Write([]byte) (int, error) {
	w.writes++
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.writeN, w.writeErr
}

func (w *inboundFaultResponseWriter) Flush() {
	w.flushes++
	if w.status == 0 {
		w.status = http.StatusOK
	}
}

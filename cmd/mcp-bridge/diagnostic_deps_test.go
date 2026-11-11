package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

// TestComposedClockControlsDuration proves that the trusted startup-context
// clock drives the whole composed path: the signed legacy duration_ms carries
// the expanded (negative, capped) outer delta while diag_duration_ms is
// clamped and flagged, all from one shared clock with no real sleeps.
func TestComposedClockControlsDuration(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	calls := 0
	clock := func() time.Time {
		calls++
		// The composed clock REGRESSES a fixed step per call so the signed
		// legacy duration delta is negative; the bounded semantic duration
		// clamps it to zero and flags the cap.
		return base.Add(-time.Duration(calls) * 500 * time.Millisecond)
	}
	sink := &bytes.Buffer{}
	logger := observe.NewLogger(sink, sink)
	diag := observe.NewHTTPDiagnostics(logger, observe.HTTPDiagnosticsOptions{
		Now:   clock,
		NewID: func() string { return "0123456789abcdef0123456789abcdef" },
	})
	handler := diag.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/mcp", nil))

	var terminal map[string]any
	for _, line := range strings.Split(strings.TrimSpace(sink.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if record["event"] == "http_complete" {
			terminal = record
		}
	}
	if terminal == nil {
		t.Fatal("no http_complete record")
	}
	duration, ok := terminal["duration_ms"].(float64)
	if !ok {
		t.Fatalf("duration_ms missing: %v", terminal["duration_ms"])
	}
	if duration >= 0 {
		t.Fatalf("duration_ms = %v; want signed negative expanded delta", duration)
	}
	if got := terminal["diag_duration_ms"]; got != float64(0) {
		t.Fatalf("diag_duration_ms = %v; want 0 after clamp", got)
	}
	if got := terminal["duration_capped"]; got != true {
		t.Fatalf("duration_capped = %v; want true", got)
	}
}

package observe

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

type fallbackFailWriter struct {
	writes int
}

func (w *fallbackFailWriter) Write([]byte) (int, error) {
	w.writes++
	return 0, errors.New("private sink failure")
}

func TestLoggerFallbackOnWriteFailure(t *testing.T) {
	for _, tc := range []struct {
		name       string
		level      string
		wantFailed string
	}{
		{name: "stderr failure falls back to stdout", level: "warn", wantFailed: "stderr"},
		{name: "stdout failure falls back to stderr", level: "info", wantFailed: "stdout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdoutFailure, stderrFailure := &fallbackFailWriter{}, &fallbackFailWriter{}
			var stdout, stderr bytes.Buffer
			stdoutWriter, stderrWriter := io.Writer(&stdout), io.Writer(&stderr)
			if tc.wantFailed == "stdout" {
				stdoutWriter = stdoutFailure
			} else {
				stderrWriter = stderrFailure
			}
			logger := NewLogger(stdoutWriter, stderrWriter)
			logger.Log(Entry{Level: tc.level, Event: "mcp_complete", Operation: "call_tool", Outcome: "success"})

			failedWrites := stdoutFailure.writes + stderrFailure.writes
			if failedWrites != 1 {
				t.Fatalf("failed-lane writes = %d; want one", failedWrites)
			}
			fallback := stderr.String()
			if tc.wantFailed == "stderr" {
				fallback = stdout.String()
			}
			assertFallbackLine(t, fallback)
		})
	}
}

func TestLoggerFallbackOnEncodeFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	logger := NewLogger(&stdout, &stderr)
	logger.Log(Entry{
		Level:     "debug",
		Event:     "mcp_complete",
		Operation: "call_tool",
		Outcome:   "success",
		Fields:    map[string]any{"count": make(chan struct{})},
	})
	if stdout.Len() != 0 {
		t.Fatalf("failed record reached stdout: %q", stdout.String())
	}
	assertFallbackLine(t, stderr.String())
}

func TestLoggerFallbackWriteIsNotRetried(t *testing.T) {
	stdout, stderr := &fallbackFailWriter{}, &fallbackFailWriter{}
	logger := NewLogger(stdout, stderr)
	logger.Log(Entry{Level: "info", Event: "mcp_complete", Operation: "call_tool", Outcome: "success"})
	if stdout.writes != 1 || stderr.writes != 1 {
		t.Fatalf("primary/fallback writes = %d/%d; want one attempt each", stdout.writes, stderr.writes)
	}
}

func assertFallbackLine(t *testing.T, fallback string) {
	t.Helper()
	if strings.Count(fallback, "\n") != 1 {
		t.Fatalf("fallback = %q; want one JSON line", fallback)
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(fallback), &record); err != nil {
		t.Fatalf("decode fallback: %v", err)
	}
	want := map[string]any{
		"schema_version": float64(SchemaVersion),
		"level":          "warn",
		"service":        "mcp-bridge",
		"event":          "logging_write_failed",
		"message":        "logging write failed",
		"operation":      "logging",
		"outcome":        "failure",
	}
	if !equalJSONObjects(record, want) {
		t.Fatalf("fallback record = %#v; want only %#v", record, want)
	}
}

func equalJSONObjects(got, want map[string]any) bool {
	if len(got) != len(want) {
		return false
	}
	for key, value := range want {
		if got[key] != value {
			return false
		}
	}
	return true
}

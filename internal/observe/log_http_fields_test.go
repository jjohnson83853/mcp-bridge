package observe_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

func TestHTTPObservationFieldsUseSharedEnvelope(t *testing.T) {
	const requestID = "0123456789abcdef0123456789abcdef"
	fields := map[string]any{
		"http_method": "POST", "route": "mcp", "http_status": 200,
		"http_outcome": "success", "stage": "none", "timing_scope": "http",
		"http_bytes_bucket": "empty", "correlation_state": "valid",
	}
	var stdout, stderr bytes.Buffer
	observe.NewLogger(&stdout, &stderr).Log(observe.Entry{
		Level: "info", Event: "http_complete", RequestID: requestID,
		Generation: 17, Operation: "request", Outcome: "success",
		Duration: 3 * time.Millisecond, Fields: fields,
	})
	if stdout.Len() == 0 || stderr.Len() != 0 || stdout.Bytes()[stdout.Len()-1] != '\n' || bytes.Count(stdout.Bytes(), []byte{'\n'}) != 1 {
		t.Fatalf("expected one stdout JSON line, stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	var record map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &record); err != nil {
		t.Fatalf("decode shared HTTP envelope: %v: %q", err, stdout.String())
	}
	keys := make([]string, 0, len(record))
	for key := range record {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	wantKeys := []string{
		"correlation_state", "duration_ms", "event", "generation", "http_bytes_bucket", "http_method",
		"http_outcome", "http_status", "level", "message", "operation", "outcome", "request_id",
		"route", "schema_version", "service", "stage", "timing_scope",
	}
	if !reflect.DeepEqual(keys, wantKeys) {
		t.Fatalf("shared HTTP envelope keys = %v; want %v", keys, wantKeys)
	}
	for key, want := range map[string]string{
		"schema_version": "1", "level": `"info"`, "service": `"mcp-bridge"`,
		"event": `"http_complete"`, "operation": `"request"`, "outcome": `"success"`,
		"request_id": `"` + requestID + `"`,
	} {
		if string(record[key]) != want {
			t.Errorf("%s = %s; want %s", key, record[key], want)
		}
	}
	if string(record["message"]) == `""` {
		t.Fatal("message is empty")
	}
	for _, removed := range []string{"time", "timestamp", "environment", "version", "subsystem", "reason", "error.class"} {
		if strings.Contains(stdout.String(), `"`+removed+`"`) {
			t.Errorf("removed field %q appeared: %s", removed, stdout.String())
		}
	}
}

func TestHTTPObservationRejectsRemovedAndUnknownFields(t *testing.T) {
	for key, value := range map[string]any{
		"time": "2026-01-01T00:00:00Z", "environment": "prod", "version": "1.0",
		"reason": "failure", "config_path": "/tmp/config.yaml", "error.class": "internal_error",
		"error": map[string]any{"code": "internal_error"}, "api_key": "synthetic-secret",
	} {
		t.Run(key, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			fields := map[string]any{
				"http_method": "POST", "route": "mcp", "http_status": 200,
				"http_outcome": "success", "stage": "none", "timing_scope": "http",
				"http_bytes_bucket": "empty", "correlation_state": "valid", key: value,
			}
			observe.NewLogger(&stdout, &stderr).Log(observe.Entry{
				Level: "info", Event: "http_complete", Operation: "request", Outcome: "success", Fields: fields,
			})
			if stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("invalid field %q was emitted: stdout=%q stderr=%q", key, stdout.String(), stderr.String())
			}
		})
	}
}

package observe_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

// Exercise the public logger and its encoded output, not its validation helpers.
func TestHTTPObservationFieldValidation(t *testing.T) {
	const secret = "synthetic-http-field-secret-7e91"
	const ownedID = "0123456789abcdef0123456789abcdef"
	idPattern := regexp.MustCompile(`^[0-9a-f]{32}$`)
	vocab := map[string][]string{
		"http_method": {"GET", "POST", "DELETE", "HEAD", "OPTIONS", "PUT", "PATCH", "OTHER"},
		"route": {"mcp", "not_found"},
		"http_outcome": {"success", "failure", "incomplete", "unknown"},
		"stage": {"none", "http_route", "host_admission", "origin_admission", "client_auth", "session_admission", "request_limits", "mcp_decode", "mcp_dispatch", "tool_admission", "backend_dispatch", "backend_response", "response_delivery", "unknown"},
		"timing_scope": {"http", "operation", "backend"},
		"http_bytes_bucket": {"empty", "le_1k", "le_16k", "le_256k", "le_1m", "gt_1m", "unavailable"},
		"correlation_state": {"available", "unavailable"},
	}
	capture := func(t *testing.T, fields map[string]any, requestID string) map[string]any {
		t.Helper()
		var output bytes.Buffer
		observe.NewLogger(&output).Log(observe.Entry{
			Level: "info", Event: "http_complete", RequestID: requestID,
			Generation: 17, Operation: "request", Outcome: "success", Reason: "none",
			Duration: 3 * time.Millisecond, Fields: fields,
		})
		if bytes.Contains(output.Bytes(), []byte(secret)) {
			t.Error("synthetic secret escaped into complete logger output")
		}
		if !json.Valid(bytes.TrimSpace(output.Bytes())) {
			t.Fatalf("logger did not emit one valid JSON object: %q", output.String())
		}
		var got map[string]any
		if err := json.Unmarshal(output.Bytes(), &got); err != nil || got == nil {
			t.Fatalf("decode logger object: %v", err)
		}
		for key, want := range map[string]any{"event": "http_complete", "generation": float64(17), "operation": "request", "outcome": "success", "duration_ms": float64(3)} {
			if got[key] != want {
				t.Errorf("valid metadata %s = %v; want %v", key, got[key], want)
			}
		}
		if id, exists := got["request_id"]; exists {
			text, ok := id.(string)
			if !ok || !idPattern.MatchString(text) {
				t.Errorf("invalid internal request_id: %v", id)
			}
		}
		return got
	}
	base := func() map[string]any {
		return map[string]any{"http_method": "POST", "route": "mcp", "http_status": 200, "http_outcome": "success", "stage": "none", "timing_scope": "http", "http_bytes_bucket": "empty", "correlation_state": "available"}
	}
	t.Run("base_metadata_is_fixed_and_not_overridable", func(t *testing.T) {
		var fixedMessage string
		for index, entry := range []observe.Entry{
			{Level: "info", Event: "http_complete", RequestID: ownedID, Generation: 17, Operation: "request", Outcome: "success", Reason: "none", Duration: time.Millisecond},
			{Level: secret, Event: "http_complete", RequestID: "abcdef0123456789abcdef0123456789", Generation: 23, Operation: secret, Outcome: secret, Reason: secret, Duration: 9 * time.Millisecond},
		} {
			entry.Fields = base()
			for _, key := range []string{"service", "environment", "version", "message", "time", "level", "event", "request_id", "generation", "operation", "duration_ms", "outcome", "reason"} {
				entry.Fields[key] = secret + fmt.Sprint(index)
			}
			var output bytes.Buffer
			observe.NewLogger(&output).Log(entry)
			if bytes.Contains(output.Bytes(), []byte(secret)) {
				t.Error("reserved metadata or entry input leaked into complete JSON output")
			}
			if !json.Valid(bytes.TrimSpace(output.Bytes())) {
				t.Fatalf("base metadata output is not one valid JSON object: %q", output.String())
			}
			var got map[string]any
			if err := json.Unmarshal(output.Bytes(), &got); err != nil || got == nil {
				t.Fatalf("decode base metadata: %v", err)
			}
			for key, want := range map[string]string{"service": "mcp-bridge", "environment": "unspecified", "version": "unspecified", "event": "http_complete"} {
				if got[key] != want {
					t.Errorf("fixed base %s = %v; want %q", key, got[key], want)
				}
			}
			stamp, ok := got["time"].(string)
			parsed, err := time.Parse(time.RFC3339Nano, stamp)
			if !ok || err != nil || !strings.HasSuffix(stamp, "Z") {
				t.Errorf("time = %v; want UTC RFC3339Nano", got["time"])
			} else if _, offset := parsed.Zone(); offset != 0 {
				t.Error("timestamp is not UTC")
			}
			switch got["level"] {
			case "debug", "info", "warn", "error":
			default:
				t.Errorf("level = %v; want finite severity", got["level"])
			}
			message, ok := got["message"].(string)
			if !ok || strings.TrimSpace(message) == "" {
				t.Errorf("message = %v; want nonempty fixed event-specific text", got["message"])
			}
			if index == 0 {
				fixedMessage = message
			} else if message != fixedMessage {
				t.Error("http_complete message varies with input metadata")
			}
		}
	})
	for key, allowed := range vocab {
		for _, value := range allowed {
			t.Run("valid/"+key+"/"+value, func(t *testing.T) {
				fields := base()
				fields[key] = value
				got := capture(t, fields, ownedID)
				if got[key] != value {
					t.Errorf("valid %s = %v; want %q", key, got[key], value)
				}
				if got["request_id"] != ownedID {
					t.Error("valid owned request_id was not preserved")
				}
			})
		}
		for name, value := range map[string]any{"secret": secret, "empty": "", "integer": 42, "float": 1.5, "bool": true, "null": nil, "object": map[string]any{"arbitrary": secret}, "array": []any{secret}, "nonfinite": math.Inf(1)} {
			t.Run("invalid/"+key+"/"+name, func(t *testing.T) {
				fields := base()
				fields[key] = value
				got := capture(t, fields, ownedID)
				if result, exists := got[key]; exists {
					valid := false
					for _, candidate := range allowed {
						if text, ok := result.(string); ok && text == candidate {
							valid = true
						}
					}
					if !valid {
						t.Errorf("invalid %s produced out-of-manifest value %v", key, result)
					}
				}
				if got["http_status"] != float64(200) {
					t.Error("hostile field discarded valid observed status")
				}
			})
		}
	}
	for _, status := range []int{100, 401, 599} {
		t.Run(fmt.Sprintf("status/valid/%d", status), func(t *testing.T) {
			fields := base()
			fields["http_status"] = status
			if got := capture(t, fields, ownedID); got["http_status"] != float64(status) {
				t.Errorf("observed status = %v; want %d", got["http_status"], status)
			}
		})
	}
	for name, status := range map[string]any{"zero": 0, "negative": -1, "too_large": 600, "huge": int64(math.MaxInt64), "huge_int": int(1 << 30), "float": 200.5, "integral_float": float64(200), "string": secret, "numeric_string": "200", "bool": true, "object": map[string]any{"status": secret}, "array": []any{secret}, "nan": math.NaN(), "positive_infinity": math.Inf(1), "negative_infinity": math.Inf(-1)} {
		t.Run("status/invalid/"+name, func(t *testing.T) {
			fields := base()
			fields["http_status"] = status
			got := capture(t, fields, ownedID)
			if value, exists := got["http_status"]; exists && value != nil {
				t.Errorf("invalid observed status emitted as %v; want omitted or unavailable null", value)
			}
			if got["http_method"] != "POST" || got["route"] != "mcp" {
				t.Error("invalid status discarded valid HTTP metadata")
			}
		})
	}
	t.Run("status/unavailable_is_explicit_null_not_zero", func(t *testing.T) {
		fields := base()
		fields["http_status"] = nil
		got := capture(t, fields, ownedID)
		if value, exists := got["http_status"]; !exists || value != nil {
			t.Errorf("unavailable observed status = %v (present %t); want explicit null", value, exists)
		}
	})
	t.Run("correlation/unavailable_uses_canonical_error_object", func(t *testing.T) {
		fields := base()
		fields["http_status"] = 401
		fields["http_outcome"] = "failure"
		fields["correlation_state"] = "unavailable"
		fields["error"] = map[string]any{
			"class": secret + "-nested-class",
			"message": secret + "-nested-message",
			"data": map[string]any{"arbitrary": secret + "-nested-data"},
		}
		fields["error_class"] = secret + "-flat-class"
		var output bytes.Buffer
		observe.NewLogger(&output).Log(observe.Entry{
			Level: "warn", Event: "http_complete", RequestID: "",
			Operation: "request", Outcome: "failure", Reason: "client_auth_failed",
			Fields: fields,
		})
		if bytes.Contains(output.Bytes(), []byte(secret)) {
			t.Error("hostile error override escaped into complete logger output")
		}
		if !json.Valid(bytes.TrimSpace(output.Bytes())) {
			t.Fatal("unavailable owned result did not emit one valid JSON object")
		}
		var got map[string]any
		if err := json.Unmarshal(output.Bytes(), &got); err != nil || got == nil {
			t.Fatal("cannot decode unavailable owned result as a JSON object")
		}
		for _, key := range []string{"request_id", "operation_id", "backend_call_id"} {
			if _, exists := got[key]; exists {
				t.Errorf("unavailable HTTP-only boundary emitted internal ID %s", key)
			}
		}
		for key, want := range map[string]any{
			"correlation_state": "unavailable", "reason": "client_auth_failed",
			"outcome": "failure", "http_status": float64(401), "http_outcome": "failure",
		} {
			if got[key] != want {
				t.Errorf("unavailable result did not preserve required %s", key)
			}
		}
		if _, exists := got["error_class"]; exists {
			t.Error("unapproved top-level error_class emitted instead of canonical nested error")
		}
		errorObject, ok := got["error"].(map[string]any)
		if !ok {
			t.Error("missing canonical structured error object")
			return
		}
		if errorObject["class"] != "correlation_unavailable" {
			t.Error("nested error.class must report correlation_unavailable")
		}
		for key, value := range errorObject {
			switch key {
			case "class":
			case "rpc_error_class":
				switch value {
				case "none", "parse_error", "invalid_request", "method_not_found", "invalid_params", "internal_error", "server_error", "other", "not_observed":
				default:
					t.Error("nested rpc_error_class is not a canonical finite scalar")
				}
			default:
				t.Error("structured error contains an unapproved key")
			}
		}
	})
	for name, id := range map[string]string{"empty": "", "hostile": secret, "uppercase": "ABCDEF0123456789ABCDEF0123456789", "short": "0123456789abcdef"} {
		t.Run("correlation/unavailable/"+name, func(t *testing.T) {
			fields := base()
			fields["correlation_state"] = "unavailable"
			got := capture(t, fields, id)
			if _, exists := got["request_id"]; exists {
				t.Error("unavailable correlation emitted an invented or invalid internal request_id")
			}
			if got["correlation_state"] != "unavailable" {
				t.Error("unavailable correlation was not reported honestly")
			}
		})
	}
}

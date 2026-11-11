package observe_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

func decodeOneLogLine(t *testing.T, output *bytes.Buffer) map[string]any {
	t.Helper()
	text := output.String()
	if strings.Count(text, "\n") != 1 {
		t.Fatalf("expected one JSON line, got %q", text)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		t.Fatalf("decode %q: %v", text, err)
	}
	return decoded
}

func assertFallbackLine(t *testing.T, output *bytes.Buffer) {
	t.Helper()
	record := decodeOneLogLine(t, output)
	want := map[string]any{
		"schema_version": float64(1),
		"level":          "warn",
		"service":        "mcp-bridge",
		"event":          "logging_write_failed",
		"message":        "logging write failed",
		"operation":      "logging",
		"outcome":        "failure",
	}
	if !reflect.DeepEqual(record, want) {
		t.Fatalf("fallback record = %#v; want exactly %#v", record, want)
	}
}

func TestConfigDiagnosticFieldManifest(t *testing.T) {
	t.Run("safe identity is re-keyed into the closed config object", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		observe.NewLogger(&stdout, &stderr).Log(observe.Entry{
			Level: "warn", Event: "reload", Operation: "reload", Outcome: "failure",
			Error: observe.RuntimeErrorInfo("invalid_config"), Server: "_bridge",
			Fields: map[string]any{
				"config.path":                   "/tmp/bridge/config.yaml",
				"config.property_path":          "servers[0].credentials.bearer",
				"config.line":                   12,
				"config.column":                 5,
				"config.server":                 "alpha",
				"config.pin_server":             "beta",
				"config.pin_tool":               "tool_name",
				"config.ref_kind":               "env",
				"config.ref_name":               "BRIDGE_CLIENT_TOKEN",
				"config.shape":                  "empty",
				"config.related.property_path":  "servers[1].credentials.bearer",
				"config.related.ref_kind":       "file",
				"config.related.ref_name":       "secrets/developer-api-key",
				"config.diagnostics_incomplete": true,
				"config.remaining_stages":       "not_checked",
			},
		})
		if stdout.Len() != 0 {
			t.Fatalf("warn record went to stdout: %s", stdout.String())
		}
		decoded := decodeOneLogLine(t, &stderr)
		keys := make([]string, 0, len(decoded))
		for key := range decoded {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		wantKeys := []string{"config", "error", "event", "level", "message", "operation", "outcome", "schema_version", "server", "service"}
		if !reflect.DeepEqual(keys, wantKeys) {
			t.Fatalf("envelope keys = %v; want %v", keys, wantKeys)
		}
		wantConfig := map[string]any{
			"path": "/tmp/bridge/config.yaml", "property_path": "servers[0].credentials.bearer",
			"line": float64(12), "column": float64(5), "server": "alpha", "pin_server": "beta",
			"pin_tool": "tool_name", "ref_kind": "env", "ref_name": "BRIDGE_CLIENT_TOKEN", "shape": "empty",
			"related": map[string]any{
				"property_path": "servers[1].credentials.bearer", "ref_kind": "file", "ref_name": "secrets/developer-api-key",
			},
			"diagnostics_incomplete": true, "remaining_stages": "not_checked",
		}
		if !reflect.DeepEqual(decoded["config"], wantConfig) {
			t.Errorf("config = %#v; want %#v", decoded["config"], wantConfig)
		}
		wantError := map[string]any{
			"code": "invalid_config", "kind": "configuration", "retryable": false,
			"remediation": "correct the configuration",
		}
		if !reflect.DeepEqual(decoded["error"], wantError) {
			t.Errorf("error = %#v; want %#v", decoded["error"], wantError)
		}
		for _, key := range []string{"schema_version", "level", "service", "event", "message", "operation", "outcome", "server"} {
			if _, present := decoded[key]; !present {
				t.Errorf("required envelope field %q missing", key)
			}
		}
		for _, key := range []string{"time", "timestamp", "environment", "version", "reason", "config_path", "location", "code"} {
			if _, present := decoded[key]; present {
				t.Errorf("removed flat field %q appeared", key)
			}
		}
	})

	t.Run("optional config fields remain omitted", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		observe.NewLogger(&stdout, &stderr).Log(observe.Entry{
			Level: "warn", Event: "reload", Operation: "reload", Outcome: "failure",
			Error:  observe.RuntimeErrorInfo("invalid_config"),
			Fields: map[string]any{"config.path": "/tmp/bridge/config.yaml", "config.property_path": "schema_version"},
		})
		decoded := decodeOneLogLine(t, &stderr)
		config, ok := decoded["config"].(map[string]any)
		if !ok {
			t.Fatalf("config = %#v; want object", decoded["config"])
		}
		for _, key := range []string{"line", "column", "pin_server", "related", "diagnostics_incomplete"} {
			if _, present := config[key]; present {
				t.Errorf("optional config field %q was defaulted", key)
			}
		}
	})

	t.Run("legacy flat manifest keys are rejected", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		observe.NewLogger(&stdout, &stderr).Log(observe.Entry{
			Level: "warn", Event: "reload", Operation: "reload", Outcome: "failure",
			Error:  observe.RuntimeErrorInfo("invalid_config"),
			Fields: map[string]any{"config_path": "/tmp/config.yaml", "reason": "invalid_config", "api_key": "synthetic-secret"},
		})
		if stderr.Len() != 0 {
			t.Fatalf("legacy metadata fallback went to stderr: %q", stderr.String())
		}
		assertFallbackLine(t, &stdout)
	})
}

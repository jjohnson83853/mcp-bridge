package observe_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

// manifestKeys is the exact 18-field diagnostic logger manifest (spec
// HMB-CD-v1 §4.5 item 4 / AC-38): the 17 diagnostic keys plus the existing
// top-level `server` aggregate label. Key spellings are pinned here literally;
// a rename must fail this test and force a deliberate contract revision.
var manifestKeys = []string{
	"config_path", "location", "line", "column", "code", "property", "shape", "ref_kind", "ref_name",
	"pin_server", "pin_tool", "related_location", "related_ref_kind", "related_ref_name",
	"diagnostic_index", "diagnostics_incomplete", "remaining_stages", "server",
}

// manifestStringKeys are the manifest fields that must decode as JSON strings.
var manifestStringKeys = []string{
	"config_path", "location", "code", "property", "shape", "ref_kind", "ref_name",
	"pin_server", "pin_tool", "related_location", "related_ref_kind", "related_ref_name",
	"remaining_stages", "server",
}

// forbiddenKeys are aliases of approved names and freeform metadata the
// manifest must never emit, whether dropped by the allowlist or redacted by
// the recursive redactor.
var forbiddenKeys = []string{
	"config_key", "secret_ref", "configpath", "refkind", "refname", "ref_kind_name",
	"location_key", "message", "error", "header", "url", "endpoint",
	"ref", "value", "fingerprint", "preview", "length",
}

// decodeOneLogLine decodes the single JSON line the logger wrote.
func decodeOneLogLine(t *testing.T, output *bytes.Buffer) map[string]any {
	t.Helper()
	text := output.String()
	if strings.Contains(strings.TrimSuffix(text, "\n"), "\n") {
		t.Fatalf("expected one JSON line, got %q", text)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSuffix(text, "\n")), &decoded); err != nil {
		t.Fatalf("decode %q: %v", text, err)
	}
	return decoded
}

// TestConfigDiagnosticFieldManifest decodes the real JSON the observe logger
// emits for typed configuration-diagnostic entries and asserts the exact key
// set, spelling, JSON types and original safe values of the 18-field manifest.
// It also pins that the optional fields are omitted rather than defaulted,
// that forbidden aliases and freeform metadata never appear, and that the
// unrelated credential redaction stays intact.
func TestConfigDiagnosticFieldManifest(t *testing.T) {
	t.Run("every manifest field survives with its exact spelling type and value", func(t *testing.T) {
		var output bytes.Buffer
		logger := observe.NewLogger(&output)
		logger.Log(observe.Entry{
			Level:   "warn",
			Event:   "reload",
			Outcome: "failure",
			Reason:  "invalid_config",
			Server:  "_bridge",
			Fields: map[string]any{
				"config_path":            "/tmp/bridge/config.yaml",
				"location":               "servers[0].credentials.bearer",
				"line":                   12,
				"column":                 5,
				"code":                   "secret_empty",
				"property":               "value",
				"shape":                  "empty",
				"ref_kind":               "env",
				"ref_name":               "BRIDGE_CLIENT_TOKEN",
				"pin_server":             "beta",
				"pin_tool":               "tool_name",
				"related_location":       "servers[1].credentials.bearer",
				"related_ref_kind":       "file",
				"related_ref_name":       "secrets/developer-api-key",
				"diagnostic_index":       2,
				"diagnostics_incomplete": true,
				"remaining_stages":       "not_checked",
			},
		})
		decoded := decodeOneLogLine(t, &output)
		want := map[string]any{
			"config_path":            "/tmp/bridge/config.yaml",
			"location":               "servers[0].credentials.bearer",
			"line":                   float64(12),
			"column":                 float64(5),
			"code":                   "secret_empty",
			"property":               "value",
			"shape":                  "empty",
			"ref_kind":               "env",
			"ref_name":               "BRIDGE_CLIENT_TOKEN",
			"pin_server":             "beta",
			"pin_tool":               "tool_name",
			"related_location":       "servers[1].credentials.bearer",
			"related_ref_kind":       "file",
			"related_ref_name":       "secrets/developer-api-key",
			"diagnostic_index":       float64(2),
			"diagnostics_incomplete": true,
			"remaining_stages":       "not_checked",
			"server":                 "_bridge",
		}
		for _, key := range manifestKeys {
			got, ok := decoded[key]
			if !ok {
				t.Errorf("manifest field %q missing", key)
				continue
			}
			if got != want[key] {
				t.Errorf("manifest field %q = %v (%T), want %v", key, got, got, want[key])
			}
		}
		for _, key := range manifestStringKeys {
			if _, ok := decoded[key]; ok {
				if _, isString := decoded[key].(string); !isString {
					t.Errorf("manifest field %q is not a JSON string: %T", key, decoded[key])
				}
			}
		}
		for _, key := range []string{"line", "column", "diagnostic_index"} {
			if _, ok := decoded[key].(float64); !ok {
				t.Errorf("manifest field %q is not a JSON number: %T", key, decoded[key])
			}
		}
		if _, ok := decoded["diagnostics_incomplete"].(bool); !ok {
			t.Errorf("manifest field diagnostics_incomplete is not a JSON boolean: %T", decoded["diagnostics_incomplete"])
		}
		for _, key := range forbiddenKeys {
			if _, ok := decoded[key]; ok {
				t.Errorf("forbidden key %q appeared in the manifest entry", key)
			}
		}
		if strings.Contains(output.String(), "[REDACTED]") {
			t.Errorf("an approved manifest field was redacted: %s", output.String())
		}
		// The standard envelope stays finite and its randomized values are not
		// frozen: time must parse, and the correlation id is generated.
		if _, err := time.Parse(time.RFC3339Nano, decoded["time"].(string)); err != nil {
			t.Errorf("time = %v is not RFC3339Nano", decoded["time"])
		}
		for _, envelope := range []struct{ key, value string }{
			{"level", "warn"}, {"event", "reload"}, {"outcome", "failure"}, {"reason", "invalid_config"},
		} {
			if decoded[envelope.key] != envelope.value {
				t.Errorf("%s = %v, want %v", envelope.key, decoded[envelope.key], envelope.value)
			}
		}
		if id, ok := decoded["request_id"].(string); !ok || len(id) != 32 {
			t.Errorf("request_id = %v, want a generated 32-hex correlation id", decoded["request_id"])
		}
	})

	t.Run("optional manifest fields are omitted not defaulted", func(t *testing.T) {
		var output bytes.Buffer
		logger := observe.NewLogger(&output)
		logger.Log(observe.Entry{
			Level:   "warn",
			Event:   "reload",
			Outcome: "failure",
			Reason:  "invalid_config",
			Server:  "_bridge",
			Fields: map[string]any{
				"config_path":      "/tmp/bridge/config.yaml",
				"location":         "schema_version",
				"code":             "schema_version",
				"property":         "schema_version",
				"shape":            "unsupported_integer",
				"diagnostic_index": 0,
				"remaining_stages": "not_checked",
			},
		})
		decoded := decodeOneLogLine(t, &output)
		for _, key := range []string{"line", "column", "ref_kind", "ref_name", "pin_server", "pin_tool", "related_location", "related_ref_kind", "related_ref_name", "diagnostics_incomplete"} {
			if _, ok := decoded[key]; ok {
				t.Errorf("optional manifest field %q was defaulted instead of omitted", key)
			}
		}
	})

	t.Run("forbidden metadata is dropped or redacted and never reflected", func(t *testing.T) {
		var output bytes.Buffer
		logger := observe.NewLogger(&output)
		logger.Log(observe.Entry{
			Level:   "warn",
			Event:   "reload",
			Outcome: "failure",
			Reason:  "invalid_config",
			Server:  "_bridge",
			Fields: map[string]any{
				"config_key": "servers[0].credentials.bearer",
				"secret_ref": "env:BRIDGE_CLIENT_TOKEN",
				"header":     "X-API-Key",
				"url":        "https://host.invalid/mcp",
				"endpoint":   "https://host.invalid/mcp",
				"error":      "opaque wrapped prose",
				"message":    "opaque wrapped prose",
				"api_key":    "credential-value-canary",
			},
		})
		text := output.String()
		decoded := decodeOneLogLine(t, &output)
		// Existing policy: a disallowed key never reaches the output at all —
		// the narrow allowlist drops it before the recursive redactor runs —
		// so neither the key nor any value is ever reflected (spec §4.5 item 5:
		// dropped/redacted according to existing policy, never reflected).
		for _, key := range []string{"config_key", "secret_ref", "header", "url", "endpoint", "error", "message", "api_key"} {
			if _, ok := decoded[key]; ok {
				t.Errorf("forbidden key %q was reflected", key)
			}
		}
		for _, fragment := range []string{"credential-value-canary", "env:BRIDGE_CLIENT_TOKEN", "servers[0].credentials.bearer", "opaque wrapped prose"} {
			if strings.Contains(text, fragment) {
				t.Fatalf("forbidden metadata leaked into the entry: %s", text)
			}
		}
	})
}

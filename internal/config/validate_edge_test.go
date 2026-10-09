package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const edgeValid = `schema_version: 1
servers:
  - id: alpha
    endpoint: https://alpha.invalid/mcp
pins: []
`

func TestParseValidationEdges(t *testing.T) {
	cases := []struct{ name, input, want string }{
		{"schema version", strings.Replace(edgeValid, "schema_version: 1", "schema_version: 2", 1), "schema_version"},
		{"unknown root key", edgeValid + "unexpected: true\n", "unknown key"},
		{"missing schema version", strings.Replace(edgeValid, "schema_version: 1\n", "", 1), "schema_version"},
		{"no servers", strings.Replace(edgeValid, "servers:\n  - id: alpha\n    endpoint: https://alpha.invalid/mcp\n", "servers: []\n", 1), "servers"},
		{"duplicate server", strings.Replace(edgeValid, "pins: []", "  - id: alpha\n    endpoint: https://other.invalid/mcp\npins: []", 1), "duplicate server id"},
		{"invalid endpoint", strings.Replace(edgeValid, "https://alpha.invalid/mcp", "file:///tmp/socket", 1), "endpoint"},
		{"invalid pin", strings.Replace(edgeValid, "pins: []", "pins:\n  - {server: absent, tool: x}", 1), "invalid pin"},
		{"duplicate pin", strings.Replace(edgeValid, "pins: []", "pins:\n  - {server: alpha, tool: x}\n  - {server: alpha, tool: x}", 1), "duplicate pin"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.input), filepath.Join(t.TempDir(), "config.yaml"))
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.want)) {
				t.Fatalf("Parse error = %v, want class/field containing %q", err, tc.want)
			}
		})
	}
}

func TestParseFileAndCredentialResolutionEdges(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, err := ParseFile(filepath.Join(t.TempDir(), "absent.yaml"))
		if err == nil {
			t.Fatal("ParseFile accepted a missing file")
		}
	})
	t.Run("unset env resolves to empty-secret rejection", func(t *testing.T) {
		t.Setenv("CONFIG_EDGE_MISSING", "present temporarily")
		if err := os.Unsetenv("CONFIG_EDGE_MISSING"); err != nil {
			t.Fatal(err)
		}
		input := strings.Replace(edgeValid, "endpoint: https://alpha.invalid/mcp", "endpoint: https://alpha.invalid/mcp\n    credentials: {bearer: {env: CONFIG_EDGE_MISSING}}", 1)
		if _, err := Parse([]byte(input), t.TempDir()); err == nil || !strings.Contains(err.Error(), "secret must be nonempty") {
			t.Fatalf("Parse error = %v, want empty-secret rejection", err)
		}
	})
	t.Run("file credential trims terminal newline", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "secret"), []byte("token\n"), 0600); err != nil {
			t.Fatal(err)
		}
		input := strings.Replace(edgeValid, "endpoint: https://alpha.invalid/mcp", "endpoint: https://alpha.invalid/mcp\n    credentials: {bearer: {file: secret}}", 1)
		cfg, err := Parse([]byte(input), filepath.Join(dir, "config.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		got, err := cfg.ServerCredentials("alpha")
		if err != nil || got.Bearer != "token" {
			t.Fatalf("ServerCredentials = %#v, %v", got, err)
		}
	})
}

func TestServerLimitsRejectInvalidValues(t *testing.T) {
	cases := []struct {
		name   string
		limits map[string]any
		want   string
	}{
		{"noninteger", map[string]any{"max_concurrent_calls": 1.5}, "max_concurrent_calls"},
		{"wrong duration type", map[string]any{"call_timeout": 1}, "call_timeout"},
		{"invalid duration", map[string]any{"call_timeout": "soon"}, "call_timeout"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{Servers: []Server{{ID: "alpha", Limits: tc.limits}}}
			_, err := cfg.ServerLimits("alpha")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ServerLimits error = %v, want field %q", err, tc.want)
			}
		})
	}
}

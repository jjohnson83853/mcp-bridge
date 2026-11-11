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

// TestParseValidationEdges migrates the C-1 semantic table to full-metadata
// oracles: key + owner + finite code/property/shape + static reason, with the
// stage barrier naming the skipped later stages (spec §5.2).
func TestParseValidationEdges(t *testing.T) {
	cases := []struct {
		name, input string
		fragments   []string
		forbidden   []string
	}{
		{"schema version", strings.Replace(edgeValid, "schema_version: 1", "schema_version: 2", 1),
			[]string{`key="schema_version"`, "code=schema_version", "property=schema_version", "shape=unsupported_integer", "schema_version must equal 1"},
			nil},
		{"unknown root key", edgeValid + "unexpected: true\n",
			[]string{`key="<document>"`, "code=yaml_key_unknown", "property=unknown_key", "unknown key"},
			[]string{"unexpected"}},
		{"no schema", strings.Replace(edgeValid, "schema_version: 1\n", "", 1),
			[]string{`key="schema_version"`, "code=schema_version", "property=schema_version", "shape=missing", "schema_version must equal 1"},
			nil},
		{"no servers", strings.Replace(edgeValid, "servers:\n  - id: alpha\n    endpoint: https://alpha.invalid/mcp\n", "servers: []\n", 1),
			[]string{`key="servers"`, "code=server_count", "property=server_count", "shape=empty", "servers must contain 1..1024 entries"},
			nil},
		{"duplicate server", strings.Replace(edgeValid, "  - id: alpha\n    endpoint: https://alpha.invalid/mcp", "  - id: alpha\n    endpoint: https://alpha.invalid/mcp\n  - id: alpha\n    endpoint: https://alpha.invalid/mcp", 1),
			[]string{`key="servers[1].id"`, `server="alpha"`, "code=server_id_duplicate", "shape=duplicate", `related_key="servers[0].id"`, "duplicate server id"},
			nil},
		{"invalid endpoint", strings.Replace(edgeValid, "https://alpha.invalid/mcp", "not-a-url", 1),
			[]string{`key="servers[0].endpoint"`, `server="alpha"`, "code=url_invalid", "property=absolute_http_url", "shape=invalid_url", "invalid absolute HTTP(S) URL"},
			[]string{"not-a-url"}},
		{"unknown pin server", strings.Replace(edgeValid, "pins: []", "pins:\n  - {server: absent, tool: x}", 1),
			[]string{`key="pins[0].server"`, `pin_server="absent"`, `pin_tool="x"`, "code=pin_server_unknown", "property=server", "shape=unknown_server", "pin references unknown server"},
			nil},
		{"duplicate pin", strings.Replace(edgeValid, "pins: []", "pins:\n  - {server: alpha, tool: x}\n  - {server: alpha, tool: x}", 1),
			[]string{`key="pins[1]"`, `pin_server="alpha"`, `pin_tool="x"`, `related_key="pins[0]"`, "code=pin_duplicate", "property=identity", "shape=duplicate", "duplicate pin"},
			nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.input), filepath.Join(t.TempDir(), "config.yaml"))
			diags := requireDiagnostics(t, err)
			record := diags.Records[0]
			switch tc.name {
			case "no schema":
				assertNoDiagnosticNodePosition(t, record)
			case "unknown root key":
				index := strings.Index(tc.input, "unexpected:")
				if index < 0 {
					t.Fatal("unknown-key fixture has no offending key")
				}
				line := strings.Count(tc.input[:index], "\n") + 1
				lineStart := strings.LastIndex(tc.input[:index], "\n") + 1
				assertConfigNodePosition(t, record, line, index-lineStart+1)
			default:
				assertDiagnosticAtYAMLPath(t, []byte(tc.input), record, record.Key)
			}
			if record.RelatedKey != "" {
				assertRelatedDiagnosticAtYAMLPath(t, []byte(tc.input), record, record.RelatedKey)
			}
			requireRendered(t, err, tc.fragments...)
			forbidRendered(t, err, tc.forbidden...)
			if diags.RemainingStages != stageNotChecked {
				t.Fatalf("expected the stage barrier to name skipped stages, got %q", diags.RemainingStages)
			}
		})
	}
}

// TestParseFileAndCredentialResolutionEdges migrates the C-1 file/env rows:
// a missing file is a config_read record for the requested path, and an unset
// environment reference is the unset class, never the old conflated
// empty-secret phrase (spec §5.2).
func TestParseFileAndCredentialResolutionEdges(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "absent.yaml")
		_, err := ParseFile(path)
		diags := requireDiagnostics(t, err)
		requireRendered(t, err,
			`config="`+path+`"`, `key="<document>"`,
			"code=config_read", "property=document", "shape=not_found",
		)
		if diags.Records[0].Path != path {
			t.Fatalf("record path = %q, want the requested path", diags.Records[0].Path)
		}
		forbidRendered(t, err, "no such file", "os.")
	})
	t.Run("unset env resolves to unset-secret rejection", func(t *testing.T) {
		t.Setenv("CONFIG_EDGE_MISSING", "present temporarily")
		if err := os.Unsetenv("CONFIG_EDGE_MISSING"); err != nil {
			t.Fatal(err)
		}
		input := strings.Replace(edgeValid, "endpoint: https://alpha.invalid/mcp", "endpoint: https://alpha.invalid/mcp\n    credentials: {bearer: {env: CONFIG_EDGE_MISSING}}", 1)
		path := filepath.Join(t.TempDir(), "config.yaml")
		_, err := Parse([]byte(input), path)
		diags := requireDiagnostics(t, err)
		// The record must attribute the failure to the document, the known
		// location, the owner and the named reference, with the unset class:
		// never the old conflated empty-secret phrase for a var that was never set.
		requireRendered(t, err,
			`config="`+path+`"`,
			`key="servers[0].credentials.bearer"`,
			`server="alpha"`,
			`ref="env:CONFIG_EDGE_MISSING"`,
			"code=secret_unset", "property=value", "shape=unset",
			"secret environment variable is unset",
			"; remaining_stages="+stageNotChecked,
		)
		forbidRendered(t, err, "secret must be nonempty")
		if diags.RemainingStages != stageNotChecked {
			t.Fatalf("expected the stage barrier, got %q", diags.RemainingStages)
		}
	})
	t.Run("file credential trims terminal newline", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "secret"), []byte("token\n"), 0600); err != nil {
			t.Fatal(err)
		}
		input := strings.Replace(edgeValid, "endpoint: https://alpha.invalid/mcp", "endpoint: https://alpha.invalid/mcp\n    credentials: {bearer: {file: secret}}", 1)
		path := filepath.Join(dir, "config.yaml")
		cfg, err := Parse([]byte(input), path)
		if err != nil {
			t.Fatalf("Parse rejected one-terminal-LF file: %v", err)
		}
		credentials, err := cfg.ServerCredentials("alpha")
		if err != nil {
			t.Fatalf("ServerCredentials rejected a valid credential: %v", err)
		}
		if credentials.Bearer != "token" {
			t.Fatalf("bearer token = %q, want exactly the file contents with one LF removed", credentials.Bearer)
		}
		if strings.Contains(cfg.String(), "token") {
			t.Fatal("Config.String leaked a resolved credential")
		}
	})
}

// TestServerLimitsRejectInvalidValues migrates the C-1 direct limits rows:
// config=<memory>, full key, owner, and finite setting_type / setting_duration
// classes with static reasons (spec §5.2).
func TestServerLimitsRejectInvalidValues(t *testing.T) {
	_, err := (Config{Servers: []Server{{ID: "alpha", Limits: map[string]any{"max_concurrent_calls": 1.5}}}}).ServerLimits("alpha")
	requireDiagnostics(t, err)
	requireRendered(t, err,
		`config="<memory>"`,
		`key="servers[0].limits.max_concurrent_calls"`,
		`server="alpha"`,
		"code=setting_type", "property=type", "shape=number", "must be an integer",
	)
	forbidRendered(t, err, "1.5")

	_, err = (Config{Servers: []Server{{ID: "alpha", Limits: map[string]any{"call_timeout": 1}}}}).ServerLimits("alpha")
	requireDiagnostics(t, err)
	requireRendered(t, err,
		`config="<memory>"`,
		`key="servers[0].limits.call_timeout"`,
		`server="alpha"`,
		"code=setting_duration", "property=type", "shape=wrong_type", "must be a duration string",
	)

	_, err = (Config{Servers: []Server{{ID: "alpha", Limits: map[string]any{"call_timeout": "soon"}}}}).ServerLimits("alpha")
	requireDiagnostics(t, err)
	requireRendered(t, err,
		`config="<memory>"`,
		`key="servers[0].limits.call_timeout"`,
		`server="alpha"`,
		"code=setting_duration", "property=type", "shape=invalid_duration", "must be a duration string",
	)
	forbidRendered(t, err, "soon")
}

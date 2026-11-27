package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestCeilingOrderRejected(t *testing.T) {
	for _, tc := range []struct {
		name, delivery, preview string
	}{
		{name: "equal", delivery: "8192", preview: "8192"},
		{name: "inverted", delivery: "8192", preview: "8193"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := retainedResultsConfig(fmt.Sprintf(
				"  delivery_ceiling_bytes: %s\n  preview_ceiling_bytes: %s\n",
				tc.delivery, tc.preview,
			))
			requireRetainedResultsDiagnostic(t, input, "ceiling_order", "retained_results.preview_ceiling_bytes")
		})
	}
}

func TestConfigRelationsEntryCap(t *testing.T) {
	input := retainedResultsConfig("  delivery_ceiling_bytes: 8192\n  max_retained_result_bytes: 8192\n")
	requireRetainedResultsDiagnostic(t, input, "entry_cap_order", "retained_results.max_retained_result_bytes")
}

func TestConfigRelationsNamespaceCap(t *testing.T) {
	input := retainedResultsConfig("  max_bytes_per_namespace: 8388607\n")
	requireRetainedResultsDiagnostic(t, input, "namespace_cap_order", "retained_results.max_bytes_per_namespace")
}

func TestConfigRelationsTtls(t *testing.T) {
	for _, tc := range []struct {
		name, key, value string
	}{
		{name: "handle zero", key: "handle_ttl", value: "0s"},
		{name: "handle negative", key: "handle_ttl", value: "-1s"},
		{name: "tombstone zero", key: "tombstone_ttl", value: "0s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := retainedResultsConfig("  " + tc.key + ": " + tc.value + "\n")
			requireRetainedResultsDiagnostic(t, input, "ttl_invalid", "retained_results."+tc.key)
		})
	}
}

func TestConfigRelationsEntries(t *testing.T) {
	input := retainedResultsConfig("  max_entries_per_namespace: 0\n")
	requireRetainedResultsDiagnostic(t, input, "bound_range", "retained_results.max_entries_per_namespace")
}

func TestConfigRelationsMarkers(t *testing.T) {
	for _, value := range []string{"0", "1.5"} {
		t.Run(value, func(t *testing.T) {
			input := retainedResultsConfig("  max_markers_per_namespace: " + value + "\n")
			requireRetainedResultsDiagnostic(t, input, "bound_range", "retained_results.max_markers_per_namespace")
		})
	}
}

func TestConfigRelationsPositiveKeys(t *testing.T) {
	for _, key := range []string{"delivery_ceiling_bytes", "preview_ceiling_bytes", "max_upstream_code_bytes"} {
		t.Run(key, func(t *testing.T) {
			input := retainedResultsConfig("  " + key + ": 0\n")
			requireRetainedResultsDiagnostic(t, input, "bound_range", "retained_results."+key)
		})
	}
	input := retainedResultsConfig("  max_upstream_code_bytes: 10\n")
	requireRetainedResultsDiagnostic(t, input, "upstream_code_marker_bound", "retained_results.max_upstream_code_bytes")
	if _, err := Parse([]byte(retainedResultsConfig("  max_upstream_code_bytes: 11\n")), "<memory>"); err != nil {
		t.Fatalf("marker floor of 11 rejected: %v", err)
	}
}

func TestConfigRelationsAggregateCap(t *testing.T) {
	input := retainedResultsConfig("  max_bytes_aggregate: 8388607\n")
	requireRetainedResultsDiagnostic(t, input, "aggregate_order", "retained_results.max_bytes_aggregate")
}

func TestConfigRelationsEntryCapBound(t *testing.T) {
	t.Run("only entry cap raised", func(t *testing.T) {
		input := retainedResultsConfig("  max_retained_result_bytes: 4294967296\n")
		requireRetainedResultsDiagnostic(t, input, "entry_cap_bound", "retained_results.max_retained_result_bytes")
	})
	t.Run("maximum accepted", func(t *testing.T) {
		input := retainedResultsConfig("  max_retained_result_bytes: 4294967295\n  max_bytes_per_namespace: 4294967295\n  max_bytes_aggregate: 4294967295\n")
		if _, err := Parse([]byte(input), "<memory>"); err != nil {
			t.Fatalf("maximum supported entry cap rejected: %v", err)
		}
	})
	t.Run("above maximum rejected", func(t *testing.T) {
		input := retainedResultsConfig("  max_retained_result_bytes: 4294967296\n  max_bytes_per_namespace: 4294967296\n  max_bytes_aggregate: 4294967296\n")
		requireRetainedResultsDiagnostic(t, input, "entry_cap_bound", "retained_results.max_retained_result_bytes")
	})
}

const (
	// R11(g) preview widths: zero-byte text; handle 43; total_bytes 20; upstream_code 512; paging_required true (4); is_error false (5).
	maximalPreviewEnvelopeBytes = 722

	// R11(g) page widths: zero-byte text; handle 43; page, total_pages and remaining_pages 10 each; total_bytes and remaining_bytes 20 each; upstream_code 512; is_error false (5).
	maximalPageEnvelopeBytes = 810

	// R11(g) terminal widths: zero-byte text; retention result_aggregate_capacity_exceeded (34); execution_state executed (8); total_bytes 20; upstream_code 512; is_error false (5).
	maximalTerminalEnvelopeBytes = 722
)

func TestConfigEnvelopeFeasibility(t *testing.T) {
	t.Run("preview", func(t *testing.T) {
		delivery := maxInt(maximalPageEnvelopeBytes+6, maxInt(maximalPreviewEnvelopeBytes+6, maximalTerminalEnvelopeBytes+6))
		input := retainedResultsConfig(fmt.Sprintf(
			"  delivery_ceiling_bytes: %d\n  preview_ceiling_bytes: %d\n",
			delivery, maximalPreviewEnvelopeBytes+5,
		))
		requireRetainedResultsDiagnostic(t, input, "envelope_feasibility", "retained_results.preview_ceiling_bytes")
	})
	t.Run("page", func(t *testing.T) {
		preview := maxInt(maximalPreviewEnvelopeBytes+6, maximalTerminalEnvelopeBytes+6)
		input := retainedResultsConfig(fmt.Sprintf(
			"  delivery_ceiling_bytes: %d\n  preview_ceiling_bytes: %d\n",
			maximalPageEnvelopeBytes+5, preview,
		))
		requireRetainedResultsDiagnostic(t, input, "envelope_feasibility", "retained_results.delivery_ceiling_bytes")
	})
	t.Run("terminal", func(t *testing.T) {
		if retainedResultEnvelopeFits(maximalTerminalMetadata(), 512, 727) {
			t.Fatal("terminal envelope unexpectedly fits with 727-byte ceiling")
		}
		if !retainedResultEnvelopeFits(maximalTerminalMetadata(), 512, 728) {
			t.Fatal("terminal envelope does not fit with 728-byte ceiling")
		}
		delivery := maxInt(maximalPageEnvelopeBytes+6, maximalTerminalEnvelopeBytes+6)
		input := retainedResultsConfig(fmt.Sprintf(
			"  delivery_ceiling_bytes: %d\n  preview_ceiling_bytes: %d\n",
			delivery, maximalTerminalEnvelopeBytes+5,
		))
		requireRetainedResultsDiagnostic(t, input, "envelope_feasibility", "retained_results.preview_ceiling_bytes")
	})
}

func TestEnvelopeHeadroomMaximalMetadata(t *testing.T) {
	preview := maxInt(maximalPreviewEnvelopeBytes+6, maximalTerminalEnvelopeBytes+6)
	delivery := maximalPageEnvelopeBytes + 6
	input := retainedResultsConfig(fmt.Sprintf(
		"  delivery_ceiling_bytes: %d\n  preview_ceiling_bytes: %d\n",
		delivery, preview,
	))
	if _, err := Parse([]byte(input), "<memory>"); err != nil {
		t.Fatalf("maximal metadata with exactly six bytes of headroom rejected: %v", err)
	}
}

func retainedResultsConfig(overrides string) string {
	return edgeValid + "retained_results:\n" + overrides
}

func requireRetainedResultsDiagnostic(t *testing.T, input, code, key string) {
	t.Helper()
	_, err := Parse([]byte(input), "<memory>")
	diags := requireDiagnostics(t, err)
	if len(diags.Records) != 1 {
		t.Fatalf("diagnostics = %#v, want one record", diags.Records)
	}
	record := diags.Records[0]
	if record.Code != code || record.Key != key {
		t.Fatalf("diagnostic = (%q, %q), want (%q, %q)", record.Code, record.Key, code, key)
	}
	if strings.Contains(err.Error(), "8192") || strings.Contains(err.Error(), "8388607") {
		t.Fatalf("diagnostic leaked an observed number: %v", err)
	}
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func TestConfigRelationsBoundsDefaultsAndOverrides(t *testing.T) {
	defaults, err := Parse([]byte(edgeValid), "<memory>")
	if err != nil {
		t.Fatalf("parse defaults: %v", err)
	}
	wantDefaults := RetainedResultBounds{
		DeliveryCeilingBytes:   8192,
		PreviewCeilingBytes:    1536,
		MaxRetainedResultBytes: 8388608,
		MaxBytesPerNamespace:   16777216,
		MaxEntriesPerNamespace: 16,
		MaxBytesAggregate:      67108864,
		HandleTTL:              time.Hour,
		TombstoneTTL:           15 * time.Minute,
		MaxUpstreamCodeBytes:   512,
		MaxMarkersPerNamespace: 16,
	}
	if defaults.RetainedResultBounds != wantDefaults {
		t.Fatalf("default retained result bounds = %#v, want %#v", defaults.RetainedResultBounds, wantDefaults)
	}
	if (Config{}).RetainedResultBounds != (RetainedResultBounds{}) {
		t.Fatal("a directly constructed Config acquired parsed retained-result defaults")
	}

	input := retainedResultsConfig("  delivery_ceiling_bytes: 9000\n  handle_ttl: 2h\n  max_entries_per_namespace: 5\n  max_upstream_code_bytes: 600\n")
	overridden, err := Parse([]byte(input), "<memory>")
	if err != nil {
		t.Fatalf("parse retained result overrides: %v", err)
	}
	wantOverrides := wantDefaults
	wantOverrides.DeliveryCeilingBytes = 9000
	wantOverrides.HandleTTL = 2 * time.Hour
	wantOverrides.MaxEntriesPerNamespace = 5
	wantOverrides.MaxUpstreamCodeBytes = 600
	if overridden.RetainedResultBounds != wantOverrides {
		t.Fatalf("overridden retained result bounds = %#v, want %#v", overridden.RetainedResultBounds, wantOverrides)
	}
	if overridden.RetainedResults["delivery_ceiling_bytes"] != 9000 {
		t.Fatalf("RetainedResults did not keep the parsed root mapping: %#v", overridden.RetainedResults)
	}
}

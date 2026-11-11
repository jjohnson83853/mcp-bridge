package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// pinsValid is the minimal valid schema-1 document: one known server and one
// pin, so every case below isolates a single pin-stage bound or naming rule
// (ticket 01: preloaded tool configuration bounds and wire naming).
const pinsValid = `schema_version: 1
servers:
  - id: alpha
    endpoint: https://alpha.invalid/mcp
pins:
  - {server: alpha, tool: search_docs}
`

// parsePinsDocument replaces the pins block in the valid document with the
// given pins text and parses it from a fresh temporary config path.
func parsePinsDocument(t *testing.T, pins string) (*Config, error) {
	t.Helper()
	input := strings.Replace(pinsValid,
		"pins:\n  - {server: alpha, tool: search_docs}\n", pins, 1)
	return Parse([]byte(input), filepath.Join(t.TempDir(), "config.yaml"))
}

// pinLines renders count distinct valid pins, one per line, so count-bounds
// cases never trip the duplicate-pin rule.
func pinLines(count int) string {
	var builder strings.Builder
	for index := range count {
		fmt.Fprintf(&builder, "  - {server: alpha, tool: t%02d}\n", index)
	}
	return builder.String()
}

// TestPinCountBounds verifies the exact upper bound of 64 configured pins:
// 64 parse successfully and 65 fail closed with the typed pin_ceiling
// diagnostic (max_preloaded_tools = 64).
func TestPinCountBounds(t *testing.T) {
	t.Run("64 pins parse", func(t *testing.T) {
		cfg, err := parsePinsDocument(t, "pins:\n"+pinLines(64))
		if err != nil {
			t.Fatalf("expected 64 pins to parse, got: %v", err)
		}
		if len(cfg.Pins) != 64 {
			t.Fatalf("expected 64 parsed pins, got %d", len(cfg.Pins))
		}
	})
	t.Run("65 pins attribute the first excess entry", func(t *testing.T) {
		_, err := parsePinsDocument(t, "pins:\n"+pinLines(65))
		diags := requireDiagnostics(t, err)
		if len(diags.Records) != 1 {
			t.Fatalf("expected exactly one pin_ceiling record, got %+v", diags)
		}
		record := diags.Records[0]
		if record.Code != "pin_ceiling" || record.Key != "pins[64]" {
			t.Fatalf("diagnostic identity = %s at %q, want pin_ceiling at pins[64]", record.Code, record.Key)
		}
		line := strings.Count(strings.SplitN(pinsValid, "pins:\n", 2)[0], "\n") + 66
		if record.Line != line || record.Column != 5 {
			t.Fatalf("position = %d:%d, want %d:5", record.Line, record.Column, line)
		}
		if record.ActualCount != 65 || record.MaxCount != 64 || record.FirstExcessIndex != 64 {
			t.Fatalf("ceiling counts = %d/%d/%d, want 65/64/64", record.ActualCount, record.MaxCount, record.FirstExcessIndex)
		}
		if record.PinServer != "alpha" || record.PinTool != "t64" {
			t.Fatalf("excess pin identity = %q/%q, want alpha/t64", record.PinServer, record.PinTool)
		}
	})
	t.Run("earlier malformed pin wins over ceiling", func(t *testing.T) {
		pins := strings.Replace(pinLines(65), "  - {server: alpha, tool: t00}\n", "  - {server: alpha}\n", 1)
		_, err := parsePinsDocument(t, "pins:\n"+pins)
		diags := requireDiagnostics(t, err)
		if len(diags.Records) != 1 || diags.Records[0].Code != "pin_fields" || diags.Records[0].Key != "pins[0]" {
			t.Fatalf("first error = %+v, want one pin_fields diagnostic at pins[0]", diags.Records)
		}
	})
}

// TestPinToolNameValidation verifies that upstream tool names adhere to
// ^[a-zA-Z0-9_-]+$ and that the total wire name length obeys
// len(server_id) + 2 + len(upstream_tool_name) <= 128, each failing closed
// with a typed diagnostic.
func TestPinToolNameValidation(t *testing.T) {
	cases := []struct {
		name      string
		tool      string
		fragments []string
	}{
		{"dot separator is rejected", "search.docs", []string{
			`key="pins[0].tool"`, `pin_server="alpha"`, `pin_tool="search.docs"`,
			"code=pin_tool_name", "property=tool", "shape=invalid_chars",
			"identifier characters"}},
		{"slash separator is rejected", "read/write", []string{
			`key="pins[0].tool"`, `pin_server="alpha"`, `pin_tool="read/write"`,
			"code=pin_tool_name", "property=tool", "shape=invalid_chars",
			"identifier characters"}},
		{"wire name over 128 characters is rejected", strings.Repeat("a", 122), []string{
			`key="pins[0].tool"`, `pin_server="alpha"`,
			"code=pin_tool_name", "property=tool", "shape=too_long",
			"wire name"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pins := fmt.Sprintf("pins:\n  - {server: alpha, tool: %q}\n", tc.tool)
			_, err := parsePinsDocument(t, pins)
			diags := requireDiagnostics(t, err)
			requireRendered(t, err, tc.fragments...)
			if len(diags.Records) != 1 {
				t.Fatalf("expected exactly one record, got %+v", diags)
			}
		})
	}
	t.Run("valid identifier characters parse", func(t *testing.T) {
		cfg, err := parsePinsDocument(t, "pins:\n  - {server: alpha, tool: Wiki_Search-Notes2}\n")
		if err != nil {
			t.Fatalf("expected a valid pin tool name to parse, got: %v", err)
		}
		if len(cfg.Pins) != 1 || cfg.Pins[0]["tool"] != "Wiki_Search-Notes2" {
			t.Fatalf("expected the configured pin, got %+v", cfg.Pins)
		}
	})
}

// TestWireToolNameHelpers verifies the canonical double-underscore wire
// naming function and its inverse mapping: <server_id>__<upstream_tool_name>.
func TestWireToolNameHelpers(t *testing.T) {
	t.Run("wire name joins server and tool", func(t *testing.T) {
		if got := WireToolName("rag", "search_docs"); got != "rag__search_docs" {
			t.Fatalf("WireToolName(rag, search_docs) = %q, want %q",
				got, "rag__search_docs")
		}
	})
	t.Run("parse splits a valid wire name", func(t *testing.T) {
		server, tool, ok := ParseWireToolName("rag__search_docs")
		if !ok || server != "rag" || tool != "search_docs" {
			t.Fatalf("ParseWireToolName(rag__search_docs) = (%q, %q, %v), want (rag, search_docs, true)",
				server, tool, ok)
		}
	})
	t.Run("parse rejects a name without a separator", func(t *testing.T) {
		server, tool, ok := ParseWireToolName("invalid_no_double_underscore")
		if ok || server != "" || tool != "" {
			t.Fatalf("ParseWireToolName(invalid_no_double_underscore) = (%q, %q, %v), want empty with ok=false",
				server, tool, ok)
		}
	})
	t.Run("parse rejects an empty server id", func(t *testing.T) {
		server, tool, ok := ParseWireToolName("__empty_server")
		if ok || server != "" || tool != "" {
			t.Fatalf("ParseWireToolName(__empty_server) = (%q, %q, %v), want empty with ok=false",
				server, tool, ok)
		}
	})
	t.Run("parse rejects an empty tool name", func(t *testing.T) {
		server, tool, ok := ParseWireToolName("alpha__")
		if ok || server != "" || tool != "" {
			t.Fatalf("ParseWireToolName(alpha__) = (%q, %q, %v), want empty with ok=false",
				server, tool, ok)
		}
	})
}

package main

import (
	"bytes"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
)

func TestConfigDiagnosticFieldManifest(t *testing.T) {
	record := config.Diagnostic{
		Path:           "/tmp/config.yaml",
		Key:            "servers[0].credentials.bearer",
		Line:           12,
		Column:         5,
		Server:         "alpha",
		PinServer:      "beta",
		PinTool:        "tool_name",
		RefKind:        "env",
		RefName:        "BRIDGE_CLIENT_TOKEN",
		RelatedKey:     "servers[1].credentials.bearer",
		RelatedRefKind: "file",
		RelatedRefName: "secrets/secret",
		Code:           "secret_empty",
		Property:       "value",
		Shape:          "empty",
		Reason:         "secret must be nonempty; provide a nonempty value for the named reference",
	}
	diagnostics := &config.Diagnostics{
		Records: []config.Diagnostic{
			record,
			{Code: "second_record", Reason: "must not be emitted"},
		},
		Incomplete:      true,
		RemainingStages: "not_checked",
	}
	var stdout, stderr bytes.Buffer
	operatorLog(&stderr)("configuration reload rejected: %v", diagnostics)
	if stdout.Len() != 0 {
		t.Fatalf("warn diagnostic went to stdout: %s", stdout.String())
	}
	if strings.Count(stderr.String(), "\n") != 1 {
		t.Fatalf("reload diagnostics produced %d lines, want exactly one: %q", strings.Count(stderr.String(), "\n"), stderr.String())
	}
	decoded := decodeEnvelopeLine(t, stderr.String())
	wantKeys := []string{"config", "error", "event", "level", "message", "operation", "outcome", "schema_version", "service"}
	keys := make([]string, 0, len(decoded))
	for key := range decoded {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, wantKeys) {
		t.Fatalf("diagnostic envelope keys = %v; want %v", keys, wantKeys)
	}
	for key, want := range map[string]any{
		"schema_version": float64(1), "level": "warn", "service": "mcp-bridge",
		"event": "config_reload_rejected", "operation": "reload", "outcome": "failure",
		"message": record.Reason,
	} {
		if decoded[key] != want {
			t.Errorf("%s = %v; want %v", key, decoded[key], want)
		}
	}
	wantConfig := map[string]any{
		"path": "/tmp/config.yaml", "property_path": "servers[0].credentials.bearer",
		"line": float64(12), "column": float64(5), "server": "alpha", "pin_server": "beta",
		"pin_tool": "tool_name", "ref_kind": "env", "ref_name": "BRIDGE_CLIENT_TOKEN", "shape": "empty",
		"related": map[string]any{
			"property_path": "servers[1].credentials.bearer", "ref_kind": "file", "ref_name": "secrets/secret",
		},
		"diagnostics_incomplete": true, "remaining_stages": "not_checked",
	}
	if !reflect.DeepEqual(decoded["config"], wantConfig) {
		t.Errorf("config = %#v; want %#v", decoded["config"], wantConfig)
	}
	wantError := map[string]any{
		"code": "secret_empty", "kind": "configuration", "retryable": false,
		"remediation": "correct the configuration and rerun validation",
	}
	if !reflect.DeepEqual(decoded["error"], wantError) {
		t.Errorf("error = %#v; want %#v", decoded["error"], wantError)
	}
	if strings.Contains(stderr.String(), "second_record") || strings.Contains(stderr.String(), "must not be emitted") {
		t.Fatalf("later diagnostic record escaped the single-envelope renderer: %q", stderr.String())
	}
}

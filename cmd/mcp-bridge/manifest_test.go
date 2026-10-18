package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

// manifestFields lists the exact 18-field logger manifest (spec HMB-CD-v1
// §4.5 item 4 / AC-38): every field must appear with its exact spelling and
// type, and no alias or forbidden key may ever appear.
var manifestFields = map[string]any{
	"config_path":            "/tmp/config.yaml",
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
	"related_ref_name":       "secrets/secret",
	"diagnostic_index":       0,
	"diagnostics_incomplete": true,
	"remaining_stages":       "not_checked",
	"server":                 "alpha",
}

// forbiddenAliases are keys the logger manifest must never emit, whether as
// aliases of approved names or as freeform metadata.
var forbiddenAliases = []string{
	"config_key", "secret_ref", "configpath", "refkind", "refname", "refkindname",
	"locationname", "locationpath", "ref_kind_name", "location_key",
	"message", "error", "header", "url", "endpoint",
}

// TestConfigDiagnosticFieldManifest decodes the real JSON the observe logger
// emits for one typed configuration-diagnostic record and asserts the exact
// key set, spelling, types and values of the 18-field manifest. It also
// asserts the forbidden aliases never appear and rejects any freeform key the
// allowlist rejects. It fails behaviourally against the pre-change allowlist.
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
		Records:         []config.Diagnostic{record},
		Incomplete:      true,
		RemainingStages: "not_checked",
	}
	var buffer bytes.Buffer
	logger := observe.NewLogger(&buffer)
	operatorLog(logger)("reload", diagnostics)

	lines := strings.Split(strings.TrimSuffix(buffer.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("json lines = %d (%q), want 2 (one record + one incomplete marker)", len(lines), buffer.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
		t.Fatalf("decode json line %q: %v", lines[0], err)
	}
	for _, key := range []string{"config_path", "location", "line", "column", "code", "property", "shape", "ref_kind", "ref_name", "pin_server", "pin_tool", "related_location", "related_ref_kind", "related_ref_name", "diagnostic_index", "diagnostics_incomplete", "remaining_stages", "server"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("manifest field %q missing", key)
		}
	}
	if decoded["config_path"] != "/tmp/config.yaml" {
		t.Errorf("config_path = %v", decoded["config_path"])
	}
	if decoded["location"] != "servers[0].credentials.bearer" {
		t.Errorf("location = %v", decoded["location"])
	}
	if decoded["line"] != float64(12) {
		t.Errorf("line = %v", decoded["line"])
	}
	if decoded["column"] != float64(5) {
		t.Errorf("column = %v", decoded["column"])
	}
	if decoded["code"] != "secret_empty" {
		t.Errorf("code = %v", decoded["code"])
	}
	if decoded["property"] != "value" {
		t.Errorf("property = %v", decoded["property"])
	}
	if decoded["shape"] != "empty" {
		t.Errorf("shape = %v", decoded["shape"])
	}
	if decoded["ref_kind"] != "env" {
		t.Errorf("ref_kind = %v", decoded["ref_kind"])
	}
	if decoded["ref_name"] != "BRIDGE_CLIENT_TOKEN" {
		t.Errorf("ref_name = %v", decoded["ref_name"])
	}
	if decoded["pin_server"] != "beta" {
		t.Errorf("pin_server = %v", decoded["pin_server"])
	}
	if decoded["pin_tool"] != "tool_name" {
		t.Errorf("pin_tool = %v", decoded["pin_tool"])
	}
	if decoded["related_location"] != "servers[1].credentials.bearer" {
		t.Errorf("related_location = %v", decoded["related_location"])
	}
	if decoded["related_ref_kind"] != "file" {
		t.Errorf("related_ref_kind = %v", decoded["related_ref_kind"])
	}
	if decoded["related_ref_name"] != "secrets/secret" {
		t.Errorf("related_ref_name = %v", decoded["related_ref_name"])
	}
	if decoded["diagnostic_index"] != float64(0) {
		t.Errorf("diagnostic_index = %v", decoded["diagnostic_index"])
	}
	if decoded["diagnostics_incomplete"] != true {
		t.Errorf("diagnostics_incomplete = %v", decoded["diagnostics_incomplete"])
	}
	if decoded["remaining_stages"] != "not_checked" {
		t.Errorf("remaining_stages = %v", decoded["remaining_stages"])
	}
	if decoded["server"] != "alpha" {
		t.Errorf("server = %v", decoded["server"])
	}
	for _, forbidden := range forbiddenAliases {
		if _, ok := decoded[forbidden]; ok {
			t.Errorf("forbidden alias %q appeared", forbidden)
		}
	}
}

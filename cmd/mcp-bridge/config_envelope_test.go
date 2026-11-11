package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

func pinConfig(count int, excessTool string) string {
	var document strings.Builder
	document.WriteString("schema_version: 1\nservers:\n  - id: alpha\n    endpoint: http://127.0.0.1:9\npins:\n")
	for index := range count {
		tool := fmt.Sprintf("tool-%02d", index)
		if index == 128 && excessTool != "" {
			tool = excessTool
		}
		fmt.Fprintf(&document, "  - {server: alpha, tool: %q}\n", tool)
	}
	return document.String()
}

func runCheckConfigText(t *testing.T, document string) (int, string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--check-config", "--config", path}, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func decodeEnvelopeLine(t *testing.T, text string) map[string]any {
	t.Helper()
	if strings.Count(text, "\n") != 1 || !strings.HasSuffix(text, "\n") {
		t.Fatalf("stderr must contain exactly one JSON line, got %q", text)
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	var record map[string]any
	if err := decoder.Decode(&record); err != nil {
		t.Fatalf("decode envelope: %v; stderr=%q", err, text)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("stderr contains more than one JSON object: extra=%v err=%v", extra, err)
	}
	return record
}

func TestPinCeilingConfigEnvelope(t *testing.T) {
	document := pinConfig(129, "")
	code, stdout, stderr := runCheckConfigText(t, document)
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%q", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	record := decodeEnvelopeLine(t, stderr)
	if record["event"] != "config_validation_failed" || record["operation"] != "load_config" {
		t.Fatalf("event/operation = %v/%v, want config_validation_failed/load_config", record["event"], record["operation"])
	}
	if record["schema_version"] != float64(1) || record["level"] != "error" || record["service"] != "mcp-bridge" {
		t.Fatalf("unexpected envelope identity: %#v", record)
	}
	errorRecord, ok := record["error"].(map[string]any)
	if !ok || errorRecord["remediation"] != "remove pins at index 128 or later" {
		t.Errorf("error remediation = %#v, want the 128-pin ceiling", record["error"])
	}
	configRecord, ok := record["config"].(map[string]any)
	if !ok {
		t.Fatalf("config field = %#v, want object", record["config"])
	}
	for key, want := range map[string]any{
		"property_path":          "pins[128]",
		"line":                   float64(134),
		"column":                 float64(5),
		"pin_server":             "alpha",
		"pin_tool":               "tool-128",
		"actual_count":           float64(129),
		"max_count":              float64(128),
		"first_excess_index":     float64(128),
		"diagnostics_incomplete": false,
		"remaining_stages":       "not_checked",
	} {
		if got := configRecord[key]; got != want {
			t.Errorf("config.%s = %#v, want %#v", key, got, want)
		}
	}
	encoded := stderr
	for _, forbidden := range []string{"\"time\"", "\"timestamp\"", "\"environment\"", "\"version\"", "\"subsystem\"", "mcp-bridge:"} {
		if strings.Contains(encoded, forbidden) {
			t.Errorf("stderr contains forbidden value %q: %s", forbidden, encoded)
		}
	}
	for _, earlier := range []string{"tool-00", "tool-01", "tool-126", "tool-127"} {
		if strings.Contains(encoded, earlier) {
			t.Errorf("stderr serialized non-excess pin identity %q: %s", earlier, encoded)
		}
	}
}

func TestPinCeilingOmitsMalformedExcessIdentity(t *testing.T) {
	const hostile = "mcp-bridge: attacker-controlled"
	code, stdout, stderr := runCheckConfigText(t, pinConfig(129, hostile))
	if code != 1 || stdout != "" {
		t.Fatalf("exit/stdout = %d/%q, want 1/empty; stderr=%q", code, stdout, stderr)
	}
	record := decodeEnvelopeLine(t, stderr)
	configRecord, ok := record["config"].(map[string]any)
	if !ok {
		t.Fatalf("config field = %#v, want object", record["config"])
	}
	if _, exists := configRecord["pin_server"]; exists {
		t.Errorf("malformed excess pin_server was emitted: %#v", configRecord["pin_server"])
	}
	if _, exists := configRecord["pin_tool"]; exists {
		t.Errorf("malformed excess pin_tool was emitted: %#v", configRecord["pin_tool"])
	}
	if strings.Contains(stderr, hostile) {
		t.Errorf("attacker-controlled identity was serialized: %q", stderr)
	}
}

func TestPinCeilingPrecedesOnlyLaterPinErrors(t *testing.T) {
	document := pinConfig(129, "")
	document = strings.Replace(document, "  - {server: alpha, tool: \"tool-00\"}\n", "  - {server: alpha}\n", 1)
	code, stdout, stderr := runCheckConfigText(t, document)
	if code != 1 || stdout != "" {
		t.Fatalf("exit/stdout = %d/%q, want 1/empty; stderr=%q", code, stdout, stderr)
	}
	record := decodeEnvelopeLine(t, stderr)
	if record["event"] != "config_validation_failed" {
		t.Fatalf("event = %v, want config_validation_failed", record["event"])
	}
	errorRecord, ok := record["error"].(map[string]any)
	if !ok || errorRecord["code"] != "pin_fields" {
		t.Fatalf("error = %#v, want pin_fields at the first malformed pin", record["error"])
	}
	configRecord, ok := record["config"].(map[string]any)
	if !ok || configRecord["property_path"] != "pins[0]" {
		t.Fatalf("config = %#v, want property_path pins[0]", record["config"])
	}
}

func TestPinCeilingBoundaryStillAccepts128Pins(t *testing.T) {
	code, _, stderr := runCheckConfigText(t, pinConfig(128, ""))
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no error record", stderr)
	}
}

func TestEnvelopeFieldNamesAreSortedAndUnique(t *testing.T) {
	names := observe.EnvelopeFieldNames()
	if len(names) != 129 {
		t.Fatalf("field count = %d, want 129", len(names))
	}
	if !sort.StringsAreSorted(names) {
		t.Fatal("field names are not sorted")
	}
	for index := 1; index < len(names); index++ {
		if names[index] == names[index-1] {
			t.Fatalf("duplicate field name %q", names[index])
		}
	}
	for _, forbidden := range []string{"time", "timestamp", "environment", "version", "subsystem", "reason", "diag_environment", "source_revision"} {
		for _, name := range names {
			if name == forbidden {
				t.Errorf("forbidden field %q appears in vocabulary", forbidden)
			}
		}
	}
	names[0] = "mutated"
	if observe.EnvelopeFieldNames()[0] == "mutated" {
		t.Fatal("field-name result aliases the package vocabulary")
	}
}

func assertConfigValidationSuccess(t *testing.T, output string) map[string]any {
	t.Helper()
	record := decodeEnvelopeLine(t, output)
	if record["event"] != "config_validation_succeeded" ||
		record["outcome"] != "success" ||
		record["operation"] != "check_config" ||
		record["level"] != "info" ||
		record["message"] != "configuration validated" {
		t.Fatalf("success record = %#v, want pinned config validation success fields", record)
	}
	if record["error"] != nil {
		t.Fatalf("success record error = %#v, want null/absent", record["error"])
	}
	configRecord, ok := record["config"].(map[string]any)
	if !ok {
		t.Fatalf("config attribution = %#v, want object", record["config"])
	}
	if configRecord["path"] == "" {
		t.Fatalf("config.path = %#v, want supplied path", configRecord["path"])
	}
	if configRecord["diagnostics_incomplete"] != false {
		t.Fatalf("config.diagnostics_incomplete = %#v, want false", configRecord["diagnostics_incomplete"])
	}
	return record
}

func TestCheckConfigSuccessEnvelope(t *testing.T) {
	code, stdout, stderr := runCheckConfigText(t, pinConfig(1, ""))
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	assertConfigValidationSuccess(t, stdout)
	for _, value := range []string{"alpha", "tool-00", "127.0.0.1:9"} {
		if strings.Contains(stdout, value) {
			t.Errorf("success record leaked configuration value %q: %s", value, stdout)
		}
	}
}

func TestCheckConfigFailureEnvelopeHasNoSuccessRecord(t *testing.T) {
	code, stdout, stderr := runCheckConfigText(t, "schema_version: [invalid\n")
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%q", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want no success record", stdout)
	}
	record := decodeEnvelopeLine(t, stderr)
	if record["event"] != "config_validation_failed" {
		t.Fatalf("event = %#v, want config_validation_failed", record["event"])
	}
}

func TestConfigValidationDiagnosticAttributionAndCompleteness(t *testing.T) {
	diagnostics := &config.Diagnostics{
		Records: []config.Diagnostic{{
			Path: "/tmp/config.yaml", Key: "credentials.bearer", Line: 4, Column: 15,
			Server: "alpha", PinServer: "alpha", PinTool: "lookup",
			RefKind: "env", RefName: "SAFE_REFERENCE",
			RelatedKey: "client_auth.bearer_tokens[0]", RelatedLine: 8, RelatedColumn: 7,
			RelatedRefKind: "file", RelatedRefName: "SAFE_FILE_REFERENCE",
			Code: "credential_overlap", Reason: "credential references overlap",
		}},
		Incomplete:      true,
		RemainingStages: "not_checked",
	}
	var output bytes.Buffer
	_ = observe.WriteJSONLine(&output, configValidationFailureEnvelope(diagnostics))
	record := decodeEnvelopeLine(t, output.String())
	configRecord, ok := record["config"].(map[string]any)
	if !ok {
		t.Fatalf("config attribution = %#v, want object", record["config"])
	}
	if configRecord["diagnostics_incomplete"] != true {
		t.Fatalf("config.diagnostics_incomplete = %#v, want true", configRecord["diagnostics_incomplete"])
	}
	for key, want := range map[string]any{
		"path": "/tmp/config.yaml", "property_path": "credentials.bearer",
		"line": float64(4), "column": float64(15), "server": "alpha",
		"pin_server": "alpha", "pin_tool": "lookup", "ref_kind": "env", "ref_name": "SAFE_REFERENCE",
	} {
		if configRecord[key] != want {
			t.Errorf("config.%s = %#v, want %#v", key, configRecord[key], want)
		}
	}
	related, ok := configRecord["related"].(map[string]any)
	if !ok {
		t.Fatalf("config.related = %#v, want object", configRecord["related"])
	}
	for key, want := range map[string]any{
		"property_path": "client_auth.bearer_tokens[0]", "line": float64(8), "column": float64(7),
		"ref_kind": "file", "ref_name": "SAFE_FILE_REFERENCE",
	} {
		if related[key] != want {
			t.Errorf("config.related.%s = %#v, want %#v", key, related[key], want)
		}
	}
	for _, secret := range []string{"private-upstream-value", "private-client-value"} {
		if strings.Contains(output.String(), secret) {
			t.Errorf("diagnostic record leaked secret value %q", secret)
		}
	}
}

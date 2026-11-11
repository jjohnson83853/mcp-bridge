package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
)

const (
	loggingContractPath = "../../docs/logging-contract.md"
	readmePath          = "../../README.md"
	changelogPath       = "../../CHANGELOG.md"
)

func readLoggingContract(t *testing.T) string {
	t.Helper()
	contents, err := os.ReadFile(loggingContractPath)
	if err != nil {
		t.Fatalf("read logging contract: %v", err)
	}
	return string(contents)
}

func contractMarkedList(t *testing.T, document, start, end string) []string {
	t.Helper()
	startIndex := strings.Index(document, start)
	endIndex := strings.Index(document, end)
	if startIndex < 0 || endIndex <= startIndex {
		t.Fatalf("logging contract is missing delimited list %q", start)
	}
	block := strings.TrimSpace(document[startIndex+len(start) : endIndex])
	block = strings.TrimPrefix(block, "```text\n")
	block = strings.TrimSuffix(block, "\n```")
	return strings.Fields(block)
}

func contractFieldList(t *testing.T, document string) []string {
	t.Helper()
	return contractMarkedList(t, document, "<!-- envelope-fields:start -->", "<!-- envelope-fields:end -->")
}

func decodeOneJSONLine(t *testing.T, line string) map[string]any {
	t.Helper()
	if strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") {
		t.Fatalf("expected exactly one newline-terminated record, got %q", line)
	}
	decoder := json.NewDecoder(strings.NewReader(line))
	var record map[string]any
	if err := decoder.Decode(&record); err != nil {
		t.Fatalf("decode JSON record: %v; line=%q", err, line)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("expected one JSON object only, extra=%v err=%v", extra, err)
	}
	return record
}

func checkClosedFieldPaths(t *testing.T, record map[string]any) {
	t.Helper()
	allowed := make(map[string]struct{}, len(observe.EnvelopeFieldNames()))
	for _, name := range observe.EnvelopeFieldNames() {
		allowed[name] = struct{}{}
	}
	var visit func(map[string]any, string)
	visit = func(object map[string]any, prefix string) {
		for key, value := range object {
			name := key
			if prefix != "" {
				name = prefix + "." + key
			}
			if _, ok := allowed[name]; !ok {
				t.Errorf("record field %q is outside EnvelopeFieldNames", name)
			}
			if nested, ok := value.(map[string]any); ok &&
				(name == "error" || name == "config" || name == "resource") {
				visit(nested, name)
			}
		}
	}
	visit(record, "")
}
func checkNoRemovedFields(t *testing.T, record map[string]any) {
	t.Helper()
	for _, key := range []string{
		"time", "timestamp", "environment", "version", "subsystem", "reason",
		"config_path", "location", "line", "column", "code", "property", "shape",
		"ref_kind", "ref_name", "pin_server", "pin_tool", "related_location",
		"diag_environment", "source_revision",
	} {
		if _, exists := record[key]; exists {
			t.Errorf("record contains removed or collector-owned key %q", key)
		}
	}
}

func TestLoggingContractDocumentsExactEnvelopeFieldUnion(t *testing.T) {
	document := readLoggingContract(t)
	if got, want := contractFieldList(t, document), observe.EnvelopeFieldNames(); !reflect.DeepEqual(got, want) {
		t.Fatalf("documented field union differs from EnvelopeFieldNames:\n got: %v\nwant: %v", got, want)
	}
	configCodes := contractMarkedList(t, document, "<!-- config-codes:start -->", "<!-- config-codes:end -->")
	if got, want := configCodes, config.DiagnosticCodes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("documented config code registry differs from DiagnosticCodes:\n got: %v\nwant: %v", got, want)
	}
	for _, required := range []string{
		"Removed, renamed, retyped, or semantically changed fields increment `schema_version`",
		"optional event-specific additions do not",
		"opaque per-process instance-correlation token",
		"never derived from host, container, environment, or version identifiers",
		"Collector-owned fields",
		"`timestamp`",
		"`environment`",
		"`version`",
	} {
		if !strings.Contains(document, required) {
			t.Errorf("logging contract is missing %q", required)
		}
	}
}

func TestLoggingContractQueriesUseOnlyDocumentedFields(t *testing.T) {
	document := readLoggingContract(t)
	fields := make(map[string]struct{}, len(observe.EnvelopeFieldNames()))
	for _, name := range observe.EnvelopeFieldNames() {
		fields[name] = struct{}{}
	}
	for _, key := range []string{"event", "error.code", "error.kind"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("query key %q is absent from the application field union", key)
		}
		if !strings.Contains(document, `"`+key+`"`) {
			t.Errorf("queries do not select documented field %q", key)
		}
	}
	queryStart := strings.Index(document, "## Loki query examples")
	if queryStart < 0 {
		t.Fatal("missing Loki query examples")
	}
	queryBlock := document[queryStart:]
	assignments := regexp.MustCompile(`([A-Za-z][A-Za-z0-9_.]*)=`)
	for _, line := range strings.Split(queryBlock, "\n") {
		if !strings.Contains(line, "count_over_time") {
			continue
		}
		for _, match := range assignments.FindAllStringSubmatch(line, -1) {
			name := match[1]
			source, alias := map[string]string{"error_code": "error.code", "error_kind": "error.kind"}[name]
			if alias {
				if !strings.Contains(line, name+`="`+source+`"`) {
					t.Errorf("query alias %q does not map to an application field", name)
				}
				continue
			}
			if _, ok := fields[name]; !ok {
				t.Errorf("query uses field %q outside EnvelopeFieldNames", name)
			}
		}
	}
	for _, severity := range []string{"debug", "info", "warn", "error"} {
		marker := "<!-- alert-" + severity + " -->"
		start := strings.Index(document, marker)
		if start < 0 {
			t.Errorf("missing alert-ready %s severity example", severity)
			continue
		}
		end := strings.Index(document[start+len(marker):], "\n\n")
		if end < 0 {
			end = len(document) - start - len(marker)
		}
		query := strings.TrimSpace(document[start+len(marker) : start+len(marker)+end])
		if !strings.Contains(query, `level="`+severity+`"`) {
			t.Errorf("%s alert query does not constrain level: %s", severity, query)
		}
	}
}

func TestLoggingContractDocumentsClassificationsAndSpecialCases(t *testing.T) {
	document := readLoggingContract(t)
	for _, required := range []string{
		"AD13 lifecycle classification",
		"`startup_snapshot_rejected`",
		"`snapshot_unavailable`",
		"`prompt_publication_failed`",
		"`reload_rejected`",
		"`reload_publication_failed`",
		"exactly one",
		"`config_validation_failed`",
		"`load_config`",
		"`config_validation_succeeded`",
		"`check_config`",
		"`upstream_ephemeral_handshake`",
		"`handshake`",
		"`error.kind`",
		"`error.retryable`",
		"`error.remediation`",
		"65-pin",
		"`pins[64]`",
		"one-based line and column",
		"nested `error` object",
		"process_id",
		"HTTP diagnostic lane",
		"60-slot",
		"Embedding environment references",
	} {
		if !strings.Contains(document, required) {
			t.Errorf("logging contract is missing %q", required)
		}
	}
}

func TestLoggingContractREADMEAndChangelogPointToCurrentContract(t *testing.T) {
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	changelog, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatal(err)
	}
	readmeText := string(readme)
	for _, required := range []string{"## Logging", "docs/logging-contract.md"} {
		if !strings.Contains(readmeText, required) {
			t.Errorf("README is missing %q", required)
		}
	}
	for _, stale := range []string{
		"`time` (UTC RFC3339Nano)",
		"`environment` (`unspecified`)",
		"`version` (`unspecified`)",
		"typed manifest",
		"18-field diagnostic logger manifest",
		"mcp-bridge: configuration:",
		"source_revision",
		"diag_environment",
		"error.class",
	} {
		if strings.Contains(readmeText, stale) {
			t.Errorf("README retains stale logging documentation %q", stale)
		}
	}
	if !strings.Contains(string(changelog), "Structured logging contract") {
		t.Error("CHANGELOG is missing the structured logging feature entry")
	}
}

func TestLoggingContractJSONLinesAndLevelStreams(t *testing.T) {
	var stdout, stderr bytes.Buffer
	logger := observe.NewLogger(&stdout, &stderr)
	levels := []string{"debug", "info", "warn", "error"}
	for _, level := range levels {
		logger.Log(observe.Entry{
			Level: level, Event: "config_validation_succeeded",
			Operation: "check_config", Outcome: "success",
		})
	}
	if got := strings.Count(stdout.String(), "\n"); got != 2 {
		t.Fatalf("stdout has %d records, want debug and info", got)
	}
	if got := strings.Count(stderr.String(), "\n"); got != 2 {
		t.Fatalf("stderr has %d records, want warn and error", got)
	}
	for index, line := range strings.Split(stdout.String(), "\n")[:2] {
		if line == "" {
			t.Fatal("stdout contains a blank record")
		}
		record := decodeOneJSONLine(t, line+"\n")
		if got, want := record["level"], levels[index]; got != want {
			t.Errorf("stdout record level = %v, want %q", got, want)
		}
		checkClosedFieldPaths(t, record)
		checkNoRemovedFields(t, record)
	}
	for index, line := range strings.Split(stderr.String(), "\n")[:2] {
		if line == "" {
			t.Fatal("stderr contains a blank record")
		}
		record := decodeOneJSONLine(t, line+"\n")
		if got, want := record["level"], levels[index+2]; got != want {
			t.Errorf("stderr record level = %v, want %q", got, want)
		}
		checkClosedFieldPaths(t, record)
		checkNoRemovedFields(t, record)
	}
	if stdout.String() == "" || stderr.String() == "" {
		t.Fatal("each stream must receive its assigned levels")
	}
}

func TestLoggingContractPinCeilingEndToEnd(t *testing.T) {
	document := pinConfig(65, "")
	code, stdout, stderr := runCheckConfigText(t, document)
	if code != 1 || stdout != "" {
		t.Fatalf("exit/stdout = %d/%q, want 1/empty; stderr=%q", code, stdout, stderr)
	}
	record := decodeOneJSONLine(t, stderr)
	if record["event"] != "config_validation_failed" || record["operation"] != "load_config" {
		t.Fatalf("event/operation = %v/%v, want config_validation_failed/load_config", record["event"], record["operation"])
	}
	configuration, ok := record["config"].(map[string]any)
	if !ok {
		t.Fatalf("config = %#v, want object", record["config"])
	}
	if configuration["property_path"] != "pins[64]" || configuration["line"] != float64(70) || configuration["column"] != float64(5) {
		t.Fatalf("pin attribution = %#v, want pins[64] at line 70 column 5", configuration)
	}
	checkClosedFieldPaths(t, record)
	checkNoRemovedFields(t, record)
}

func loggingContractRecords(t *testing.T, encoded string) []map[string]any {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(encoded))
	var records []map[string]any
	for {
		var record map[string]any
		err := decoder.Decode(&record)
		if err == io.EOF {
			return records
		}
		if err != nil {
			t.Fatalf("decode producer records: %v; output=%q", err, encoded)
		}
		checkClosedFieldPaths(t, record)
		checkNoRemovedFields(t, record)
		records = append(records, record)
	}
}

func loggingContractClassRow(document, event, code string) []string {
	prefix := "| `" + event + "` |"
	for _, line := range strings.Split(document, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		parts := strings.Split(line, "|")
		cells := make([]string, 0, len(parts)-2)
		for _, part := range parts[1 : len(parts)-1] {
			cells = append(cells, strings.TrimSpace(part))
		}
		if len(cells) > 4 && strings.Trim(cells[4], "`") == code {
			return cells
		}
	}
	return nil
}

func TestLoggingContractProducerOperationPairs(t *testing.T) {
	var records []map[string]any
	appendRecords := func(encoded string) {
		t.Helper()
		records = append(records, loggingContractRecords(t, encoded)...)
	}

	if code, stdout, stderr := runCheckConfigText(t, pinConfig(65, "")); code != 1 || stdout != "" {
		t.Fatalf("invalid config result = %d, stdout=%q, want exit 1 and empty stdout; stderr=%q", code, stdout, stderr)
	} else {
		appendRecords(stderr)
	}
	validConfig := "schema_version: 1\nservers:\n  - id: alpha\n    endpoint: http://127.0.0.1:9\npins: []\n"
	if code, stdout, stderr := runCheckConfigText(t, validConfig); code != 0 || stderr != "" {
		t.Fatalf("valid config result = %d, stderr=%q, want exit 0 and empty stderr", code, stderr)
	} else {
		appendRecords(stdout)
	}

	var cliStdout, cliStderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--unknown-flag"}, &cliStdout, &cliStderr); code != 1 || cliStdout.Len() != 0 {
		t.Fatalf("CLI error result = %d, stdout=%q, want exit 1 and empty stdout", code, cliStdout.String())
	}
	appendRecords(cliStderr.String())

	var startup bytes.Buffer
	writeStartupFailure(&startup, stateDirectoryFailure("/tmp/bridge-state"))
	writeStartupFailure(&startup, jwksCacheFailure())
	writeStartupFailure(&startup, embeddingProviderFailure())
	writeStartupFailure(&startup, toolSurfaceFailure())
	writeStartupFailure(&startup, catalogLoopFailure())
	writeStartupFailure(&startup, listenerBindFailure("inbound", "127.0.0.1:8000"))
	writeStartupFailure(&startup, fallbackStartupFailure())
	writeStartupFailure(&startup, startupPublicationFailure())
	writeStartupFailure(&startup, cliUsageFailure())
	appendRecords(startup.String())

	var lifecycle bytes.Buffer
	logf := operatorLog(&lifecycle)
	diagnostics := &config.Diagnostics{Records: []config.Diagnostic{{
		Path: "/tmp/bridge.yaml", Key: "servers[0].endpoint", Line: 2, Column: 15,
		Code: "url_invalid", Reason: "endpoint URL is invalid",
	}}}
	logf("configuration reload rejected: %v", diagnostics)
	logf("refresh publication failed after runtime acceptance: %v", service.ErrPublicationFailed)
	logf("startup_snapshot_rejected: %v", errors.New("private snapshot cause"))
	logf("prompt publication rejected: %s", "private prompt cause")
	logf("configuration reload rejected: %v", errors.New("private reload cause"))
	logf("unclassified producer: %v", errors.New("private fallback cause"))
	if strings.Contains(lifecycle.String(), "private ") {
		t.Fatalf("lifecycle record leaked wrapped error text: %s", lifecycle.String())
	}
	appendRecords(lifecycle.String())

	const id = "0123456789abcdef0123456789abcdef"
	var shared bytes.Buffer
	logger := observe.NewLogger(&shared, &shared)
	logger.Log(observe.Entry{
		Level: "info", Event: "upstream_ephemeral_handshake", Server: "alpha",
		Operation: "handshake", Outcome: "success", Duration: time.Millisecond,
		Fields: map[string]any{"backend_id": "alpha", "protocol_version": "2025-11-25"},
	})
	logger.Log(observe.Entry{
		Level: "info", Event: "mcp_complete", Operation: "call_tool", Outcome: "success",
		RequestID: id, Fields: map[string]any{
			"mcp_method": "tools_call", "mcp_message_kind": "request", "operation_id": id,
		},
	})
	logger.Log(observe.Entry{
		Level: "info", Event: "backend_complete", Operation: "call_tool", Outcome: "success",
		RequestID: id, Server: "alpha", Fields: map[string]any{
			"backend_call_id": id, "operation_id": id, "tool": "alpha__read",
		},
	})
	appendRecords(shared.String())

	var httpLogs bytes.Buffer
	httpDiagnostics := observe.NewHTTPDiagnostics(observe.NewLogger(&httpLogs, &httpLogs), observe.HTTPDiagnosticsOptions{
		Now:        time.Now,
		NewID:      func() string { return id },
		Process:    observe.HTTPProcessContext{State: "available", ID: id},
		Provenance: observe.HTTPProvenance{BuildState: "unavailable"},
	})
	httpDiagnostics.Wrap(http.NotFoundHandler()).ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "http://example.test/unmatched", nil),
	)
	httpRecords := loggingContractRecords(t, httpLogs.String())
	if len(httpRecords) != 1 || httpRecords[0]["event"] != "http_complete" ||
		httpRecords[0]["operation"] == "" {
		t.Fatalf("HTTP diagnostic record = %#v, want one http_complete with operation", httpRecords)
	}
	records = append(records, httpRecords...)

	expectedOperations := map[string]string{
		"config_validation_failed":     "load_config",
		"config_validation_succeeded":  "check_config",
		"startup_failed":               "startup",
		"startup_publication_failed":   "startup",
		"cli_usage_failed":             "cli",
		"config_reload_rejected":       "reload",
		"reload_publication_failed":    "publish",
		"startup_snapshot_rejected":    "startup",
		"prompt_publication_failed":    "publish",
		"reload_rejected":              "reload",
		"internal_warning":             "reload",
		"upstream_ephemeral_handshake": "handshake",
		"mcp_complete":                 "call_tool",
		"backend_complete":             "call_tool",
	}
	seen := make(map[string]bool, len(expectedOperations))
	for _, record := range records {
		event, _ := record["event"].(string)
		want, pinned := expectedOperations[event]
		if !pinned {
			continue
		}
		if got := record["operation"]; got != want {
			t.Errorf("%s operation = %v, want %q", event, got, want)
		}
		seen[event] = true
	}
	for event := range expectedOperations {
		if !seen[event] {
			t.Errorf("no producer record observed for pinned event %q", event)
		}
	}
	document := readLoggingContract(t)
	classifiedEvents := map[string]bool{
		"startup_failed": true, "startup_publication_failed": true,
		"cli_usage_failed": true, "startup_snapshot_rejected": true,
		"prompt_publication_failed": true, "reload_rejected": true,
		"reload_publication_failed": true, "internal_warning": true,
	}
	for _, record := range records {
		event, _ := record["event"].(string)
		if !classifiedEvents[event] {
			continue
		}
		errorInfo, ok := record["error"].(map[string]any)
		if !ok {
			t.Errorf("%s record has no nested error classification: %#v", event, record)
			continue
		}
		stringField := func(object map[string]any, key string) string {
			value, ok := object[key].(string)
			if !ok {
				t.Errorf("%s.%s = %#v, want a string", event, key, object[key])
			}
			return value
		}
		retryable, ok := errorInfo["retryable"].(bool)
		if !ok {
			t.Errorf("%s.error.retryable = %#v, want a boolean", event, errorInfo["retryable"])
		}
		code := stringField(errorInfo, "code")
		expected := []string{
			"`" + event + "`",
			"`" + stringField(record, "level") + "`",
			"`" + stringField(record, "operation") + "`",
			"`" + stringField(record, "outcome") + "`",
			"`" + code + "`",
			"`" + stringField(errorInfo, "kind") + "`",
			"`" + strconv.FormatBool(retryable) + "`",
			"`" + stringField(errorInfo, "remediation") + "`",
			"`" + stringField(record, "message") + "`",
		}
		if got := loggingContractClassRow(document, event, code); !reflect.DeepEqual(got, expected) {
			t.Errorf("%s/%s documentation row = %v, want %v", event, code, got, expected)
		}
	}
}

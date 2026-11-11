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
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
	"github.com/jjohnson83853/mcp-bridge/internal/upstream"
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
		"129-pin",
		"`pins[128]`",
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
	document := pinConfig(129, "")
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
	for key, want := range map[string]any{
		"property_path":      "pins[128]",
		"line":               float64(134),
		"column":             float64(5),
		"pin_server":         "alpha",
		"pin_tool":           "tool-128",
		"actual_count":       float64(129),
		"max_count":          float64(128),
		"first_excess_index": float64(128),
	} {
		if got := configuration[key]; got != want {
			t.Errorf("config.%s = %#v, want %#v", key, got, want)
		}
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
		if got := record["schema_version"]; got != float64(1) {
			t.Errorf("%v record schema_version = %v, want 1", record["event"], got)
		}
		records = append(records, record)
	}
}
func loggingContractR8OperationRecords(t *testing.T) []map[string]any {
	t.Helper()
	records := loggingContractRefreshPassRecords(t, &config.Config{}, "catalog_refresh_completed")
	pinCandidate := &config.Config{Pins: []map[string]string{{"server": "alpha", "tool": "read"}}}
	records = append(records, loggingContractRefreshPassRecords(t, pinCandidate, "catalog_refresh_failed")...)
	records = append(records, loggingContractDiscoveryRecords(t)...)
	records = append(records, loggingContractReloadAppliedRecords(t)...)
	records = append(records, loggingContractFallbackRecords(t)...)
	return records
}

func loggingContractRefreshPassRecords(t *testing.T, candidate *config.Config, wantEvent string) []map[string]any {
	t.Helper()
	var output bytes.Buffer
	reported := make(chan struct{}, 1)
	refresher := service.NewRefresher(service.NewRuntime(), func() *config.Config {
		return candidate
	}, service.RefreshDeps{
		ReloadDeps: service.ReloadDeps{
			Discover: func(config.Server) ([]core.ToolDefinition, error) {
				return nil, nil
			},
			NewClient: func(config.Server) (core.Upstream, error) {
				return nil, nil
			},
			Now:                 func() time.Time { return time.Unix(1, 0) },
			ModelIdentityDigest: strings.Repeat("a", 64),
		},
		OnRejected: func(error) { reported <- struct{}{} },
	}, service.RefresherOptions{
		Logger: observe.NewLogger(&output, &output),
		Published: func(*service.Assembly) error {
			reported <- struct{}{}
			return nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		refresher.Run(ctx)
		close(done)
	}()
	refresher.Trigger()
	select {
	case <-reported:
	case <-time.After(time.Second):
		cancel()
		<-done
		t.Fatalf("refresh pass did not report %s", wantEvent)
	}
	cancel()
	<-done
	records := loggingContractRecords(t, output.String())
	for _, record := range records {
		if record["event"] == wantEvent {
			return records
		}
	}
	t.Fatalf("refresh records do not contain %q: %#v", wantEvent, records)
	return nil
}

func loggingContractDiscoveryRecords(t *testing.T) []map[string]any {
	t.Helper()
	peer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(peer.Close)

	var output bytes.Buffer
	dedicated, err := upstream.NewDedicated(upstream.DedicatedOptions{
		Logger:        observe.NewLogger(&output, &output),
		RetryInterval: time.Minute,
		Server: upstream.ServerOptions{
			ID:                  "alpha",
			Endpoint:            peer.URL + "/mcp",
			ConnectTimeout:      time.Second,
			TransportMaxRetries: 1,
		},
	})
	if err != nil {
		t.Fatalf("create local discovery fixture: %v", err)
	}
	t.Cleanup(func() { _ = dedicated.Close() })
	if _, err := dedicated.Discover(context.Background()); err == nil {
		t.Fatal("discovery unexpectedly succeeded against an unauthorized fixture")
	}
	return loggingContractRecords(t, output.String())
}

func loggingContractReloadAppliedRecords(t *testing.T) []map[string]any {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("capture applied-reload output: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = originalStdout }()
	operatorLog(io.Discard)("configuration reload applied", map[string]string{
		"config_revision": strings.Repeat("b", 64),
		"trigger":         "signal",
	})
	os.Stdout = originalStdout
	if err := writer.Close(); err != nil {
		t.Fatalf("close applied-reload capture: %v", err)
	}
	encoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read applied-reload output: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close applied-reload reader: %v", err)
	}
	return loggingContractRecords(t, string(encoded))
}

type loggingContractFailWriter struct{}

func (loggingContractFailWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func loggingContractFallbackRecords(t *testing.T) []map[string]any {
	t.Helper()
	var fallback bytes.Buffer
	observe.NewLogger(loggingContractFailWriter{}, &fallback).Log(observe.Entry{
		Level: "info", Event: "backend_complete", Operation: "call_tool", Outcome: "success",
	})
	return loggingContractRecords(t, fallback.String())
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

	if code, stdout, stderr := runCheckConfigText(t, pinConfig(129, "")); code != 1 || stdout != "" {
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
func TestLoggingContractDocumentsR8EventsAndSchemaEvolution(t *testing.T) {
	document := readLoggingContract(t)
	if !strings.Contains(document, "## R8 event-specific additions") {
		t.Fatal("logging contract is missing the r8 event-specific additions")
	}
	for _, row := range []string{
		"| `catalog_refresh_started` | `refresh` | `refresh_id`, `config_revision`, `trigger`, `next_attempt_ms` | optional event-specific addition; no schema-version change (`schema_version` remains 1) |",
		"| `catalog_refresh_completed` | `refresh` | `refresh_id`, `config_revision`, `trigger`, `servers_discovered`, `tools_discovered`, `tools_published`, `pins_resolved`, `pins_total` | optional event-specific addition; no schema-version change (`schema_version` remains 1) |",
		"| `catalog_refresh_failed` | `refresh` | `refresh_id`, `config_revision`, `trigger`, optional `next_attempt_ms`, `catalog_state`, `retained_generation`, `catalog_tools`, `retained_age_ms`, `candidate_servers`, `candidate_tools`, `publication_impact`, `pins_resolved`, `pins_total`, `error` | optional event-specific addition; no schema-version change (`schema_version` remains 1) |",
		"| `upstream_discovery_failed` | `refresh` | `server`, `failure_stage`, `failure_reason`, applicable `http_status`, sanitized `endpoint`, `error`, scheduled `next_attempt_ms` | optional event-specific addition; no schema-version change (`schema_version` remains 1) |",
		"| `pin_blocker_detail` | `refresh` | per-pin: `refresh_id`, `server`, `config.pin_tool`, `config.property_path`, `blocker_class`, applicable `failure_reason`, `publication_impact`; tool-list chunk: `refresh_id`, `server`, `blocker_class`, `discovered_tools`, `chunk_index`, `chunk_count`, `publication_impact` | optional event-specific addition; no schema-version change (`schema_version` remains 1) |",
		"| `config_reload_applied` | `reload` | `config_revision`, `trigger` | optional event-specific addition; no schema-version change (`schema_version` remains 1) |",
		"| `logging_write_failed` | `logging` | none beyond the common envelope | optional event-specific addition; no schema-version change (`schema_version` remains 1) |",
	} {
		if !strings.Contains(document, row) {
			t.Errorf("logging contract is missing r8 event row %q", row)
		}
	}
}

func TestLoggingContractR8EventMessagesOperationsAndSchemaVersion(t *testing.T) {
	eventMessages := map[string]string{
		"http_complete":                "http request completed",
		"mcp_complete":                 "MCP operation completed",
		"backend_complete":             "upstream tool call completed",
		"upstream_ephemeral_handshake": "upstream protocol handshake observed",
		"startup_failed":               "bridge startup failed",
		"cli_usage_failed":             "invalid command-line usage",
		"config_validation_failed":     "configuration validation failed",
		"config_validation_succeeded":  "configuration valid",
		"startup_publication_failed":   "startup publication failed",
		"startup_snapshot_rejected":    "startup snapshot unavailable",
		"prompt_publication_failed":    "prompt publication failed",
		"config_reload_applied":        "configuration reload applied",
		"config_reload_rejected":       "configuration reload rejected",
		"reload_rejected":              "configuration reload rejected",
		"reload_publication_failed":    "configuration reload publication failed",
		"catalog_refresh_started":      "catalog refresh started",
		"catalog_refresh_completed":    "catalog refresh completed",
		"catalog_refresh_failed":       "catalog refresh failed",
		"upstream_discovery_failed":    "upstream discovery failed",
		"pin_blocker_detail":           "mandatory pin blocks catalog refresh",
		"logging_write_failed":         "logging write failed",
		"internal_warning":             "bridge encountered an internal warning",
		"reload":                       "configuration reload diagnostic",
	}
	eventOperations := map[string]string{
		"http_complete":                "request",
		"mcp_complete":                 "call_tool",
		"backend_complete":             "call_tool",
		"upstream_ephemeral_handshake": "handshake",
		"startup_failed":               "startup",
		"cli_usage_failed":             "cli",
		"config_validation_failed":     "load_config",
		"config_validation_succeeded":  "check_config",
		"startup_publication_failed":   "startup",
		"startup_snapshot_rejected":    "startup",
		"prompt_publication_failed":    "publish",
		"config_reload_applied":        "reload",
		"config_reload_rejected":       "reload",
		"reload_rejected":              "reload",
		"reload_publication_failed":    "publish",
		"catalog_refresh_started":      "refresh",
		"catalog_refresh_completed":    "refresh",
		"catalog_refresh_failed":       "refresh",
		"upstream_discovery_failed":    "refresh",
		"pin_blocker_detail":           "refresh",
		"logging_write_failed":         "logging",
		"internal_warning":             "reload",
		"reload":                       "reload",
	}
	if len(eventMessages) != 23 || len(eventOperations) != len(eventMessages) {
		t.Fatalf("event contract has %d messages and %d operation pins; want 23 each", len(eventMessages), len(eventOperations))
	}
	var output bytes.Buffer
	logger := observe.NewLogger(&output, &output)
	for event, message := range eventMessages {
		logger.Log(observe.Entry{
			Level: "info", Event: event, Operation: eventOperations[event], Outcome: "unknown",
		})
		if message == "" {
			t.Errorf("%s has an empty expected static message", event)
		}
	}
	records := loggingContractRecords(t, output.String())
	if len(records) != len(eventMessages) {
		t.Fatalf("emitted %d named event records; want %d", len(records), len(eventMessages))
	}
	seen := make(map[string]bool, len(records))
	for _, record := range records {
		event, ok := record["event"].(string)
		if !ok {
			t.Errorf("event = %#v, want string", record["event"])
			continue
		}
		if seen[event] {
			t.Errorf("event %q emitted more than once", event)
		}
		seen[event] = true
		if got, want := record["message"], eventMessages[event]; got != want {
			t.Errorf("%s message = %v, want %q", event, got, want)
		}
		if got, want := record["operation"], eventOperations[event]; got != want {
			t.Errorf("%s operation = %v, want %q", event, got, want)
		}
	}
	for event := range eventMessages {
		if !seen[event] {
			t.Errorf("no record emitted for named event %q", event)
		}
	}
	actualR8 := loggingContractR8OperationRecords(t)
	expectedR8 := []string{
		"catalog_refresh_started",
		"catalog_refresh_completed",
		"catalog_refresh_failed",
		"upstream_discovery_failed",
		"pin_blocker_detail",
		"config_reload_applied",
		"logging_write_failed",
	}
	seenR8 := make(map[string]bool, len(expectedR8))
	for _, record := range actualR8 {
		event, ok := record["event"].(string)
		if !ok {
			t.Errorf("r8 producer event = %#v, want string", record["event"])
			continue
		}
		want, ok := eventOperations[event]
		if !ok {
			t.Errorf("unexpected r8 producer event %q", event)
			continue
		}
		if got := record["operation"]; got != want {
			t.Errorf("%s producer operation = %v, want %q", event, got, want)
		}
		if got := record["schema_version"]; got != float64(1) {
			t.Errorf("%s producer schema_version = %v, want 1", event, got)
		}
		seenR8[event] = true
	}
	for _, event := range expectedR8 {
		if !seenR8[event] {
			t.Errorf("no r8 producer record observed for %q", event)
		}
	}
}

func TestLoggingContractQueriesFindRefreshAndPinFailures(t *testing.T) {
	document := readLoggingContract(t)
	for _, scenario := range []struct {
		marker string
		fields []string
	}{
		{
			marker: "<!-- query-refresh-rejections -->",
			fields: []string{`event="catalog_refresh_failed"`, `error_code="error.code"`, `error_kind="error.kind"`},
		},
		{
			marker: "<!-- query-unresolved-pins -->",
			fields: []string{`event="pin_blocker_detail"`, `blocker_class="blocker_class"`},
		},
	} {
		start := strings.Index(document, scenario.marker)
		if start < 0 {
			t.Errorf("logging contract is missing query marker %q", scenario.marker)
			continue
		}
		block := document[start+len(scenario.marker):]
		opening := strings.Index(block, "```")
		if opening < 0 {
			t.Errorf("query %q is missing its code fence", scenario.marker)
			continue
		}
		languageEnd := strings.Index(block[opening:], "\n")
		if languageEnd < 0 {
			t.Errorf("query %q has an incomplete code fence", scenario.marker)
			continue
		}
		queryStart := opening + languageEnd + 1
		closing := strings.Index(block[queryStart:], "```")
		if closing < 0 {
			t.Errorf("query %q is missing its closing code fence", scenario.marker)
			continue
		}
		query := block[queryStart : queryStart+closing]
		if strings.Contains(query, "message") {
			t.Errorf("query %q uses message text: %s", scenario.marker, query)
		}
		for _, field := range scenario.fields {
			if !strings.Contains(query, field) {
				t.Errorf("query %q is missing %q", scenario.marker, field)
			}
		}
	}
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

type snapshotRecoveryHarness struct {
	configPath string
	stateDir   string
	httpAddr   string
}

type snapshotRecoveryRun struct {
	cancel   context.CancelFunc
	done     chan int
	stdout   *syncWriter
	stderr   *syncWriter
	finished bool
	exitCode int
}

func newSnapshotRecoveryHarness(t *testing.T) snapshotRecoveryHarness {
	t.Helper()
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)
	embeddings := fakeEmbeddings(t)
	upstream := fakeUpstream(t, "read", `{"type":"object","properties":{},"additionalProperties":true}`)
	root := t.TempDir()
	harness := snapshotRecoveryHarness{
		configPath: filepath.Join(root, "bridge.yaml"),
		stateDir:   filepath.Join(root, "state"),
		httpAddr:   freeAddress(t),
	}
	writeBridgeConfig(t, harness.configPath, harness.httpAddr, freeAddress(t), harness.stateDir, embeddings.URL, []bridgeServerSpec{{id: "alpha", endpoint: upstream.URL}})
	return harness
}

func beginSnapshotRecoveryRun(t *testing.T, harness snapshotRecoveryHarness) *snapshotRecoveryRun {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	run := &snapshotRecoveryRun{
		cancel: cancel,
		done:   make(chan int, 1),
		stdout: &syncWriter{},
		stderr: &syncWriter{},
	}
	go func() {
		run.done <- runContext(ctx, []string{"--config", harness.configPath}, run.stdout, run.stderr)
	}()
	t.Cleanup(func() {
		run.cancel()
		if !run.finished {
			select {
			case <-run.done:
			case <-time.After(6 * time.Second):
			}
		}
	})

	deadline := time.Now().Add(5 * time.Second)
	client := http.Client{Timeout: 100 * time.Millisecond}
	for time.Now().Before(deadline) {
		select {
		case code := <-run.done:
			run.finished = true
			run.exitCode = code
			return run
		default:
		}
		response, err := client.Get("http://" + harness.httpAddr + "/healthz")
		if err == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return run
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	run.cancel()
	t.Fatalf("serve did not become live within 5s; stderr=%q", run.stderr.String())
	return nil
}

func stopSnapshotRecoveryRun(t *testing.T, run *snapshotRecoveryRun) {
	t.Helper()
	if !run.finished {
		run.cancel()
		select {
		case run.exitCode = <-run.done:
			run.finished = true
		case <-time.After(5 * time.Second):
			t.Fatalf("serve did not stop after cancellation; stderr=%q", run.stderr.String())
		}
	}
	if run.exitCode != 0 {
		t.Fatalf("serve exit=%d; stderr=%q", run.exitCode, run.stderr.String())
	}
}

func runSnapshotRecoveryToExit(t *testing.T, harness snapshotRecoveryHarness) (int, string) {
	t.Helper()
	stdout, stderr := &syncWriter{}, &syncWriter{}
	code := runContext(context.Background(), []string{"--config", harness.configPath}, stdout, stderr)
	return code, stderr.String()
}

func seedRecoveryPath(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func waitForFreshRecoveryState(t *testing.T, harness snapshotRecoveryHarness) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if validRegularSnapshot(harness.stateDir) && validRegularPrompts(harness.stateDir) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("fresh valid snapshots were not published within 5s: snapshot=%t prompts=%t", validRegularSnapshot(harness.stateDir), validRegularPrompts(harness.stateDir))
}

func validRegularSnapshot(directory string) bool {
	path := filepath.Join(directory, storage.SnapshotFile)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var snapshot storage.Snapshot
	return json.Unmarshal(data, &snapshot) == nil && snapshot.Validate() == nil
}

func validRegularPrompts(directory string) bool {
	path := filepath.Join(directory, storage.PromptSnapshotFile)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var prompts storage.PromptSnapshot
	return json.Unmarshal(data, &prompts) == nil && prompts.Validate() == nil
}

func assertDirectPromptSaveAndLoads(t *testing.T, harness snapshotRecoveryHarness) {
	t.Helper()
	store, err := storage.Open(harness.stateDir)
	if err != nil {
		t.Fatalf("open recovered store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("close recovered store: %v", err)
		}
	}()
	fresh := &storage.PromptSnapshot{SchemaVersion: storage.PromptSchemaVersion, Prompts: []storage.Prompt{{ServerID: "portainer", RawName: "greet", PublicName: "p_greet"}}}
	if err := store.SavePrompts(fresh); err != nil {
		t.Fatalf("direct SavePrompts after recovery: %v", err)
	}
	if _, err := store.Load(); err != nil {
		t.Fatalf("load recovered snapshot: %v", err)
	}
	if _, err := store.LoadPrompts(); err != nil {
		t.Fatalf("load refreshed prompt snapshot: %v", err)
	}
}

func restartSnapshotRecoveryNoop(t *testing.T, harness snapshotRecoveryHarness) {
	t.Helper()
	before := recoveryQuarantineCounts(t, harness.stateDir)
	run := beginSnapshotRecoveryRun(t, harness)
	if run.finished {
		t.Fatalf("restart exited before serving with code %d: %q", run.exitCode, run.stderr.String())
	}
	stopSnapshotRecoveryRun(t, run)
	for _, record := range recoveryLogRecords(t, run.stderr.String()) {
		if record["event"] == "startup_snapshot_rejected" || record["event"] == "snapshot_retention_failed" {
			t.Fatalf("restart emitted recovery record: %#v", record)
		}
	}
	if after := recoveryQuarantineCounts(t, harness.stateDir); !reflect.DeepEqual(after, before) {
		t.Fatalf("restart changed quarantine counts: before=%v after=%v", before, after)
	}
}

func recoveryLogRecords(t *testing.T, text string) []map[string]any {
	t.Helper()
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	records := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("decode log line %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

func recoveryEvents(records []map[string]any, event string) []map[string]any {
	var found []map[string]any
	for _, record := range records {
		if record["event"] == event {
			found = append(found, record)
		}
	}
	return found
}

func assertRecoveryWarning(t *testing.T, record map[string]any, stateDir, reason string) string {
	t.Helper()
	if record["level"] != "warn" || record["event"] != "startup_snapshot_rejected" ||
		record["operation"] != "startup" || record["outcome"] != "degraded" ||
		record["message"] != "startup snapshot unavailable" {
		t.Fatalf("recovery warning envelope=%#v", record)
	}
	if record["failure_reason"] != reason {
		t.Fatalf("failure_reason=%v; want %q", record["failure_reason"], reason)
	}
	if !reflect.DeepEqual(record["resource"], map[string]any{"path": stateDir}) {
		t.Fatalf("resource=%#v; want configured state path %q", record["resource"], stateDir)
	}
	quarantine, ok := record["quarantine_path"].(string)
	if !ok || !filepath.IsAbs(quarantine) {
		t.Fatalf("quarantine_path=%#v; want absolute path", record["quarantine_path"])
	}
	if !reflect.DeepEqual(record["error"], map[string]any{
		"code": "snapshot_unavailable", "kind": "resource", "retryable": true,
		"remediation": "verify the durable snapshot and storage permissions, then retry",
	}) {
		t.Fatalf("snapshot_unavailable error shape=%#v", record["error"])
	}
	return quarantine
}

func recoveryQuarantineCounts(t *testing.T, directory string) map[string]int {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{"snapshot": 0, "prompts": 0}
	for _, entry := range entries {
		name := entry.Name()
		for _, family := range []string{"snapshot", "prompts"} {
			if strings.HasPrefix(name, family+".corrupt-") && strings.HasSuffix(name, ".json") {
				counts[family]++
			}
		}
	}
	return counts
}

func seedRecoveryQuarantine(t *testing.T, directory, family string, at time.Time, content []byte) string {
	t.Helper()
	path := filepath.Join(directory, fmt.Sprintf("%s.corrupt-%s.json", family, at.UTC().Format(time.RFC3339Nano)))
	seedRecoveryPath(t, path, content)
	return path
}

func TestEmitRecoveryRecords(t *testing.T) {
	var stdout, stderr bytes.Buffer
	logger := observe.NewLogger(&stdout, &stderr)
	reasons := []storage.Reason{
		storage.ReasonCorruptJSON,
		storage.ReasonChecksumMismatch,
		storage.ReasonUnsupportedSchema,
		storage.ReasonValidationFailure,
		storage.ReasonReadError,
		storage.ReasonOrphanedLink,
	}
	recoveries := make([]storage.Recovery, 0, len(reasons))
	for i, reason := range reasons {
		recoveries = append(recoveries, storage.Recovery{
			Family: storage.FamilySnapshot, Reason: reason,
			OriginalPath: filepath.Join(t.TempDir(), fmt.Sprintf("original-%d", i)),
			QuarantinePath: filepath.Join(t.TempDir(), fmt.Sprintf("snapshot-%d.corrupt.json", i)),
		})
	}
	result := &storage.RecoveryResult{
		Recoveries: recoveries,
		RetentionFailures: []storage.RetentionFailure{
			{Family: storage.FamilySnapshot, QuarantinePath: "/state/snapshot.corrupt-old.json"},
			{Family: storage.FamilyPrompts, QuarantinePath: "/state/prompts.corrupt-old.json"},
		},
	}
	emitRecoveryRecords(logger, "/state", result)
	records := recoveryLogRecords(t, stderr.String())
	if len(records) != len(reasons)+2 {
		t.Fatalf("emitted %d records, want %d: %q", len(records), len(reasons)+2, stderr.String())
	}
	for i, reason := range reasons {
		assertRecoveryWarning(t, records[i], "/state", string(reason))
		if records[i]["quarantine_path"] != recoveries[i].QuarantinePath {
			t.Errorf("record %d quarantine_path=%v; want %q", i, records[i]["quarantine_path"], recoveries[i].QuarantinePath)
		}
	}
	for i, failure := range result.RetentionFailures {
		record := records[len(reasons)+i]
		if record["level"] != "warn" || record["event"] != "snapshot_retention_failed" ||
			record["operation"] != "startup" || record["outcome"] != "failure" ||
			record["message"] != "quarantine retention failed" || record["quarantine_path"] != failure.QuarantinePath ||
			!reflect.DeepEqual(record["resource"], map[string]any{"path": "/state"}) {
			t.Errorf("retention record=%#v", record)
		}
		if !reflect.DeepEqual(record["error"], map[string]any{
			"code": "state_directory", "kind": "resource", "retryable": false,
			"remediation": "create the configured state directory or fix its permissions",
		}) {
			t.Errorf("retention error shape=%#v", record["error"])
		}
	}
}

func TestSnapshotRecoveryStartup(t *testing.T) {
	t.Setenv("MCP_SNAPSHOT_QUARANTINE", "")
	harness := newSnapshotRecoveryHarness(t)
	original := []byte("{")
	seedRecoveryPath(t, filepath.Join(harness.stateDir, storage.SnapshotFile), original)
	run := beginSnapshotRecoveryRun(t, harness)
	if run.finished {
		t.Fatalf("recovered startup exited %d: %q", run.exitCode, run.stderr.String())
	}
	waitForFreshRecoveryState(t, harness)
	stopSnapshotRecoveryRun(t, run)
	assertDirectPromptSaveAndLoads(t, harness)

	records := recoveryLogRecords(t, run.stderr.String())
	warnings := recoveryEvents(records, "startup_snapshot_rejected")
	if len(warnings) != 1 || len(recoveryEvents(records, "startup_failed")) != 0 {
		t.Fatalf("startup recovery records=%#v", records)
	}
	quarantine := assertRecoveryWarning(t, warnings[0], harness.stateDir, string(storage.ReasonCorruptJSON))
	if got, err := os.ReadFile(quarantine); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("quarantined snapshot=%q err=%v", got, err)
	}
	if counts := recoveryQuarantineCounts(t, harness.stateDir); counts["snapshot"] != 1 || counts["prompts"] != 0 {
		t.Fatalf("quarantine counts=%v; want one snapshot and no prompt quarantine", counts)
	}
}

func TestSnapshotRecoveryBothCorrupt(t *testing.T) {
	t.Setenv("MCP_SNAPSHOT_QUARANTINE", "on")
	harness := newSnapshotRecoveryHarness(t)
	snapshotBytes, promptBytes := []byte("snapshot-bad"), []byte("prompts-bad")
	seedRecoveryPath(t, filepath.Join(harness.stateDir, storage.SnapshotFile), snapshotBytes)
	seedRecoveryPath(t, filepath.Join(harness.stateDir, storage.PromptSnapshotFile), promptBytes)
	run := beginSnapshotRecoveryRun(t, harness)
	if run.finished {
		t.Fatalf("recovered startup exited %d: %q", run.exitCode, run.stderr.String())
	}
	waitForFreshRecoveryState(t, harness)
	stopSnapshotRecoveryRun(t, run)
	assertDirectPromptSaveAndLoads(t, harness)

	records := recoveryLogRecords(t, run.stderr.String())
	warnings := recoveryEvents(records, "startup_snapshot_rejected")
	if len(warnings) != 2 || len(recoveryEvents(records, "startup_failed")) != 0 {
		t.Fatalf("both-family recovery records=%#v", records)
	}
	paths := map[string]bool{}
	for _, warning := range warnings {
		paths[assertRecoveryWarning(t, warning, harness.stateDir, string(storage.ReasonCorruptJSON))] = true
	}
	if len(paths) != 2 {
		t.Fatalf("quarantine paths are not distinct: %v", paths)
	}
	for path, want := range map[string][]byte{
		filepath.Join(harness.stateDir, storage.SnapshotFile): snapshotBytes,
		filepath.Join(harness.stateDir, storage.PromptSnapshotFile): promptBytes,
	} {
		family := strings.TrimSuffix(filepath.Base(path), ".json")
		var found bool
		for quarantine := range paths {
			if strings.HasPrefix(filepath.Base(quarantine), family+".corrupt-") {
				found = true
				if got, err := os.ReadFile(quarantine); err != nil || !bytes.Equal(got, want) {
					t.Errorf("%s quarantine=%q err=%v", family, got, err)
				}
			}
		}
		if !found {
			t.Errorf("no %s quarantine in %v", family, paths)
		}
	}
}

func TestSnapshotRecoverySecondRun(t *testing.T) {
	clearSnapshotQuarantineEnv(t)
	harness := newSnapshotRecoveryHarness(t)
	first := beginSnapshotRecoveryRun(t, harness)
	if first.finished {
		t.Fatalf("initial run exited %d: %q", first.exitCode, first.stderr.String())
	}
	waitForFreshRecoveryState(t, harness)
	stopSnapshotRecoveryRun(t, first)
	restartSnapshotRecoveryNoop(t, harness)
}

func clearSnapshotQuarantineEnv(t *testing.T) {
	t.Helper()
	t.Setenv("MCP_SNAPSHOT_QUARANTINE", "")
	if err := os.Unsetenv("MCP_SNAPSHOT_QUARANTINE"); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotQuarantineOptOutAndDefaults(t *testing.T) {
	for _, spelling := range []string{"off", "Off", "OFF"} {
		t.Run("disabled-"+spelling, func(t *testing.T) {
			t.Setenv("MCP_SNAPSHOT_QUARANTINE", spelling)
			harness := newSnapshotRecoveryHarness(t)
			original := []byte("{")
			path := filepath.Join(harness.stateDir, storage.SnapshotFile)
			seedRecoveryPath(t, path, original)
			code, stderr := runSnapshotRecoveryToExit(t, harness)
			if code != 1 {
				t.Fatalf("disabled corrupt startup exit=%d; want 1; stderr=%q", code, stderr)
			}
			if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, original) {
				t.Fatalf("disabled recovery changed snapshot=%q err=%v", got, err)
			}
			if counts := recoveryQuarantineCounts(t, harness.stateDir); counts["snapshot"] != 0 || counts["prompts"] != 0 {
				t.Fatalf("disabled recovery created quarantine: %v", counts)
			}
			records := recoveryLogRecords(t, stderr)
			warnings := recoveryEvents(records, "startup_snapshot_rejected")
			failures := recoveryEvents(records, "startup_failed")
			if len(warnings) != 1 || len(failures) == 0 {
				t.Fatalf("disabled recovery records=%#v", records)
			}
			if _, ok := warnings[0]["failure_reason"]; ok {
				t.Fatalf("disabled Durable record was enriched: %#v", warnings[0])
			}
			if _, ok := warnings[0]["quarantine_path"]; ok {
				t.Fatalf("disabled Durable record has quarantine path: %#v", warnings[0])
			}
			failureInfo := failures[0]["error"].(map[string]any)
			if failureInfo["code"] != "internal_error" || len(recoveryEvents(records, "snapshot_retention_failed")) != 0 {
				t.Fatalf("disabled failure records=%#v", records)
			}
		})
	}

	for _, spelling := range []struct {
		name  string
		value string
		unset bool
	}{{name: "unset", unset: true}, {name: "empty", value: ""}, {name: "one", value: "1"}} {
		t.Run("enabled-"+spelling.name, func(t *testing.T) {
			if spelling.unset {
				clearSnapshotQuarantineEnv(t)
			} else {
				t.Setenv("MCP_SNAPSHOT_QUARANTINE", spelling.value)
			}
			harness := newSnapshotRecoveryHarness(t)
			seedRecoveryPath(t, filepath.Join(harness.stateDir, storage.SnapshotFile), []byte("{"))
			run := beginSnapshotRecoveryRun(t, harness)
			if run.finished {
				t.Fatalf("enabled corrupt startup exited %d: %q", run.exitCode, run.stderr.String())
			}
			waitForFreshRecoveryState(t, harness)
			stopSnapshotRecoveryRun(t, run)
			warnings := recoveryEvents(recoveryLogRecords(t, run.stderr.String()), "startup_snapshot_rejected")
			if len(warnings) != 1 {
				t.Fatalf("enabled recovery records=%#v", recoveryLogRecords(t, run.stderr.String()))
			}
			quarantine := assertRecoveryWarning(t, warnings[0], harness.stateDir, string(storage.ReasonCorruptJSON))
			if got, err := os.ReadFile(quarantine); err != nil || string(got) != "{" {
				t.Fatalf("enabled quarantine=%q err=%v", got, err)
			}
		})
	}
}

func TestSnapshotRecoveryMixedFatal(t *testing.T) {
	t.Setenv("MCP_SNAPSHOT_QUARANTINE", "on")
	harness := newSnapshotRecoveryHarness(t)
	original := []byte("{")
	seedRecoveryPath(t, filepath.Join(harness.stateDir, storage.SnapshotFile), original)
	if err := os.Mkdir(filepath.Join(harness.stateDir, storage.PromptSnapshotFile), 0o700); err != nil {
		t.Fatal(err)
	}
	code, stderr := runSnapshotRecoveryToExit(t, harness)
	if code != 1 {
		t.Fatalf("mixed recoverable/fatal startup exit=%d; want 1; stderr=%q", code, stderr)
	}
	records := recoveryLogRecords(t, stderr)
	warnings := recoveryEvents(records, "startup_snapshot_rejected")
	failures := recoveryEvents(records, "startup_failed")
	if len(warnings) != 1 || len(failures) != 1 {
		t.Fatalf("mixed fatal records=%#v", records)
	}
	quarantine := assertRecoveryWarning(t, warnings[0], harness.stateDir, string(storage.ReasonCorruptJSON))
	if got, err := os.ReadFile(quarantine); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("mixed fatal snapshot quarantine=%q err=%v", got, err)
	}
	if strings.Index(stderr, "startup_snapshot_rejected") > strings.Index(stderr, "startup_failed") {
		t.Fatalf("fatal record preceded completed recovery warning: %q", stderr)
	}
}

func TestSnapshotRecoveryFatalFirst(t *testing.T) {
	t.Setenv("MCP_SNAPSHOT_QUARANTINE", "on")
	harness := newSnapshotRecoveryHarness(t)
	fifo := filepath.Join(harness.stateDir, storage.SnapshotFile)
	if err := os.MkdirAll(harness.stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	promptBytes := []byte("prompts-bad")
	seedRecoveryPath(t, filepath.Join(harness.stateDir, storage.PromptSnapshotFile), promptBytes)
	stdout, stderr := &syncWriter{}, &syncWriter{}
	done := make(chan int, 1)
	go func() {
		done <- runContext(context.Background(), []string{"--config", harness.configPath}, stdout, stderr)
	}()
	select {
	case code := <-done:
		if code != 1 {
			t.Fatalf("fatal-first exit=%d; want 1", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fatal FIFO recovery did not complete within 2s")
	}
	records := recoveryLogRecords(t, stderr.String())
	if len(recoveryEvents(records, "startup_failed")) != 1 || len(recoveryEvents(records, "startup_snapshot_rejected")) != 0 {
		t.Fatalf("fatal-first records=%#v", records)
	}
	if got, err := os.ReadFile(filepath.Join(harness.stateDir, storage.PromptSnapshotFile)); err != nil || !bytes.Equal(got, promptBytes) {
		t.Fatalf("fatal-first prompts changed=%q err=%v", got, err)
	}
	if counts := recoveryQuarantineCounts(t, harness.stateDir); counts["snapshot"] != 0 || counts["prompts"] != 0 {
		t.Fatalf("fatal-first mutated quarantine families: %v", counts)
	}
	if _, err := os.Lstat(fifo); err != nil {
		t.Fatalf("fatal-first FIFO moved: %v", err)
	}
}

func TestSnapshotRecoverySymlinkLoop(t *testing.T) {
	t.Setenv("MCP_SNAPSHOT_QUARANTINE", "on")
	harness := newSnapshotRecoveryHarness(t)
	link := filepath.Join(harness.stateDir, storage.SnapshotFile)
	if err := os.MkdirAll(harness.stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(storage.SnapshotFile, link); err != nil {
		t.Fatal(err)
	}
	run := beginSnapshotRecoveryRun(t, harness)
	if run.finished {
		t.Fatalf("symlink-loop recovery exited %d: %q", run.exitCode, run.stderr.String())
	}
	waitForFreshRecoveryState(t, harness)
	stopSnapshotRecoveryRun(t, run)
	records := recoveryLogRecords(t, run.stderr.String())
	warnings := recoveryEvents(records, "startup_snapshot_rejected")
	if len(warnings) != 1 || len(recoveryEvents(records, "startup_failed")) != 0 {
		t.Fatalf("symlink-loop records=%#v", records)
	}
	quarantine := assertRecoveryWarning(t, warnings[0], harness.stateDir, string(storage.ReasonReadError))
	if target, err := os.Readlink(quarantine); err != nil || target != storage.SnapshotFile {
		t.Fatalf("quarantined loop target=%q err=%v", target, err)
	}
	assertDirectPromptSaveAndLoads(t, harness)
}

func TestSnapshotRecoveryPromptsOnly(t *testing.T) {
	t.Setenv("MCP_SNAPSHOT_QUARANTINE", "on")
	harness := newSnapshotRecoveryHarness(t)
	initial := beginSnapshotRecoveryRun(t, harness)
	if initial.finished {
		t.Fatalf("initial healthy startup exited %d: %q", initial.exitCode, initial.stderr.String())
	}
	waitForFreshRecoveryState(t, harness)
	stopSnapshotRecoveryRun(t, initial)

	promptBytes := []byte("broken prompts")
	seedRecoveryPath(t, filepath.Join(harness.stateDir, storage.PromptSnapshotFile), promptBytes)
	run := beginSnapshotRecoveryRun(t, harness)
	if run.finished {
		t.Fatalf("prompts recovery startup exited %d: %q", run.exitCode, run.stderr.String())
	}
	waitForFreshRecoveryState(t, harness)
	stopSnapshotRecoveryRun(t, run)
	assertDirectPromptSaveAndLoads(t, harness)
	warnings := recoveryEvents(recoveryLogRecords(t, run.stderr.String()), "startup_snapshot_rejected")
	if len(warnings) != 1 {
		t.Fatalf("prompts-only recovery records=%#v", recoveryLogRecords(t, run.stderr.String()))
	}
	quarantine := assertRecoveryWarning(t, warnings[0], harness.stateDir, string(storage.ReasonCorruptJSON))
	if !strings.HasPrefix(filepath.Base(quarantine), "prompts.corrupt-") {
		t.Fatalf("prompts-only quarantine=%q", quarantine)
	}
	if got, err := os.ReadFile(quarantine); err != nil || !bytes.Equal(got, promptBytes) {
		t.Fatalf("prompts quarantine=%q err=%v", got, err)
	}
	if counts := recoveryQuarantineCounts(t, harness.stateDir); counts["snapshot"] != 0 || counts["prompts"] != 1 {
		t.Fatalf("prompts-only quarantine counts=%v", counts)
	}
	restartSnapshotRecoveryNoop(t, harness)
}

func TestSnapshotRecoveryOrphanedLink(t *testing.T) {
	t.Setenv("MCP_SNAPSHOT_QUARANTINE", "on")
	harness := newSnapshotRecoveryHarness(t)
	original := []byte("{")
	seedRecoveryPath(t, filepath.Join(harness.stateDir, storage.SnapshotFile), original)
	if err := os.Symlink(storage.SnapshotFile, filepath.Join(harness.stateDir, storage.PromptSnapshotFile)); err != nil {
		t.Fatal(err)
	}
	run := beginSnapshotRecoveryRun(t, harness)
	if run.finished {
		t.Fatalf("orphaned-link startup exited %d: %q", run.exitCode, run.stderr.String())
	}
	waitForFreshRecoveryState(t, harness)
	stopSnapshotRecoveryRun(t, run)
	assertDirectPromptSaveAndLoads(t, harness)
	assertOrphanedLinkRecords(t, harness, run, original)
	restartSnapshotRecoveryNoop(t, harness)
}

func TestSnapshotRecoveryMutualLink(t *testing.T) {
	t.Setenv("MCP_SNAPSHOT_QUARANTINE", "on")
	harness := newSnapshotRecoveryHarness(t)
	if err := os.MkdirAll(harness.stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	var oldestSeed string
	for age := 1; age <= storage.QuarantineKeep; age++ {
		seed := seedRecoveryQuarantine(t, harness.stateDir, "prompts", time.Now().UTC().Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("prompt-%d", age)))
		if age == storage.QuarantineKeep {
			oldestSeed = seed
		}
	}
	if err := os.Symlink(storage.PromptSnapshotFile, filepath.Join(harness.stateDir, storage.SnapshotFile)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(storage.SnapshotFile, filepath.Join(harness.stateDir, storage.PromptSnapshotFile)); err != nil {
		t.Fatal(err)
	}
	run := beginSnapshotRecoveryRun(t, harness)
	if run.finished {
		t.Fatalf("mutual-link startup exited %d: %q", run.exitCode, run.stderr.String())
	}
	waitForFreshRecoveryState(t, harness)
	stopSnapshotRecoveryRun(t, run)
	assertDirectPromptSaveAndLoads(t, harness)
	records := recoveryLogRecords(t, run.stderr.String())
	warnings := recoveryEvents(records, "startup_snapshot_rejected")
	if len(warnings) != 2 || len(recoveryEvents(records, "startup_failed")) != 0 {
		t.Fatalf("mutual-link records=%#v", records)
	}
	first := assertRecoveryWarning(t, warnings[0], harness.stateDir, string(storage.ReasonReadError))
	second := assertRecoveryWarning(t, warnings[1], harness.stateDir, string(storage.ReasonOrphanedLink))
	if !strings.HasPrefix(filepath.Base(first), "snapshot.corrupt-") || !strings.HasPrefix(filepath.Base(second), "prompts.corrupt-") {
		t.Fatalf("mutual-link record order paths=%q, %q", first, second)
	}
	if target, err := os.Readlink(second); err != nil || target != storage.SnapshotFile {
		t.Fatalf("reconciled mutual prompts target=%q err=%v", target, err)
	}
	if got := recoveryQuarantineCounts(t, harness.stateDir)["prompts"]; got != storage.QuarantineKeep {
		t.Fatalf("mutual reconciliation prompts quarantine count=%d; want %d", got, storage.QuarantineKeep)
	}
	if _, err := os.Stat(oldestSeed); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("oldest prompts retention seed %q remains: %v", oldestSeed, err)
	}
	restartSnapshotRecoveryNoop(t, harness)
}

func TestSnapshotRecoveryMutualBothMoved(t *testing.T) {
	t.Run("both moved", func(t *testing.T) {
		t.Setenv("MCP_SNAPSHOT_QUARANTINE", "on")
		harness := newSnapshotRecoveryHarness(t)
		if err := os.MkdirAll(harness.stateDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(storage.PromptSnapshotFile, filepath.Join(harness.stateDir, storage.SnapshotFile)); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(storage.PromptSnapshotFile, filepath.Join(harness.stateDir, storage.PromptSnapshotFile)); err != nil {
			t.Fatal(err)
		}
		run := beginSnapshotRecoveryRun(t, harness)
		if run.finished {
			t.Fatalf("both-moved startup exited %d: %q", run.exitCode, run.stderr.String())
		}
		waitForFreshRecoveryState(t, harness)
		stopSnapshotRecoveryRun(t, run)
		assertDirectPromptSaveAndLoads(t, harness)
		records := recoveryLogRecords(t, run.stderr.String())
		warnings := recoveryEvents(records, "startup_snapshot_rejected")
		if len(warnings) != 2 || len(recoveryEvents(records, "startup_failed")) != 0 {
			t.Fatalf("both-moved records=%#v", records)
		}
		assertRecoveryWarning(t, warnings[0], harness.stateDir, string(storage.ReasonReadError))
		assertRecoveryWarning(t, warnings[1], harness.stateDir, string(storage.ReasonReadError))
		if len(recoveryEvents(records, "startup_snapshot_rejected")) != 2 {
			t.Fatalf("both-moved recovery count=%d; want exactly two", len(warnings))
		}
		restartSnapshotRecoveryNoop(t, harness)
	})

	t.Run("self moved", func(t *testing.T) {
		t.Setenv("MCP_SNAPSHOT_QUARANTINE", "on")
		harness := newSnapshotRecoveryHarness(t)
		if err := os.MkdirAll(harness.stateDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(storage.SnapshotFile, filepath.Join(harness.stateDir, storage.SnapshotFile)); err != nil {
			t.Fatal(err)
		}
		run := beginSnapshotRecoveryRun(t, harness)
		if run.finished {
			t.Fatalf("self-moved startup exited %d: %q", run.exitCode, run.stderr.String())
		}
		waitForFreshRecoveryState(t, harness)
		stopSnapshotRecoveryRun(t, run)
		assertDirectPromptSaveAndLoads(t, harness)
		records := recoveryLogRecords(t, run.stderr.String())
		warnings := recoveryEvents(records, "startup_snapshot_rejected")
		if len(warnings) != 1 || len(recoveryEvents(records, "startup_failed")) != 0 {
			t.Fatalf("self-moved records=%#v", records)
		}
		assertRecoveryWarning(t, warnings[0], harness.stateDir, string(storage.ReasonReadError))
		if len(recoveryEvents(records, "startup_snapshot_rejected")) != 1 {
			t.Fatalf("self-moved recovery count=%d; want exactly one", len(warnings))
		}
		restartSnapshotRecoveryNoop(t, harness)
	})
}

func TestSnapshotRecoveryMutualHardlinked(t *testing.T) {
	t.Setenv("MCP_SNAPSHOT_QUARANTINE", "on")
	harness := newSnapshotRecoveryHarness(t)
	if err := os.MkdirAll(harness.stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	snapshotPath := filepath.Join(harness.stateDir, storage.SnapshotFile)
	promptsPath := filepath.Join(harness.stateDir, storage.PromptSnapshotFile)
	if err := os.Symlink(storage.SnapshotFile, snapshotPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(snapshotPath, promptsPath); err != nil {
		t.Fatal(err)
	}
	snapshotInfo, err := os.Lstat(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	promptsInfo, err := os.Lstat(promptsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(snapshotInfo, promptsInfo) {
		t.Fatal("fixture is not a hard-linked symlink pair")
	}
	run := beginSnapshotRecoveryRun(t, harness)
	if run.finished {
		t.Fatalf("hardlinked mutual startup exited %d: %q", run.exitCode, run.stderr.String())
	}
	waitForFreshRecoveryState(t, harness)
	stopSnapshotRecoveryRun(t, run)
	assertDirectPromptSaveAndLoads(t, harness)
	records := recoveryLogRecords(t, run.stderr.String())
	warnings := recoveryEvents(records, "startup_snapshot_rejected")
	if len(warnings) != 2 || len(recoveryEvents(records, "startup_failed")) != 0 {
		t.Fatalf("hardlinked mutual records=%#v", records)
	}
	first := assertRecoveryWarning(t, warnings[0], harness.stateDir, string(storage.ReasonReadError))
	second := assertRecoveryWarning(t, warnings[1], harness.stateDir, string(storage.ReasonOrphanedLink))
	if !strings.HasPrefix(filepath.Base(first), "snapshot.corrupt-") || !strings.HasPrefix(filepath.Base(second), "prompts.corrupt-") {
		t.Fatalf("hardlinked mutual paths=%q, %q", first, second)
	}
	if target, err := os.Readlink(second); err != nil || target != storage.SnapshotFile {
		t.Fatalf("hardlinked reconciled link target=%q err=%v", target, err)
	}
	restartSnapshotRecoveryNoop(t, harness)
}

func TestSnapshotRecoveryNonMutualDangling(t *testing.T) {
	t.Setenv("MCP_SNAPSHOT_QUARANTINE", "on")
	harness := newSnapshotRecoveryHarness(t)
	original := []byte("{")
	seedRecoveryPath(t, filepath.Join(harness.stateDir, storage.SnapshotFile), original)
	promptsPath := filepath.Join(harness.stateDir, storage.PromptSnapshotFile)
	if err := os.Symlink("gone.json", promptsPath); err != nil {
		t.Fatal(err)
	}
	run := beginSnapshotRecoveryRun(t, harness)
	if run.finished {
		t.Fatalf("non-mutual dangling startup exited %d: %q", run.exitCode, run.stderr.String())
	}
	waitForFreshRecoveryState(t, harness)
	stopSnapshotRecoveryRun(t, run)
	assertDirectPromptSaveAndLoads(t, harness)
	records := recoveryLogRecords(t, run.stderr.String())
	warnings := recoveryEvents(records, "startup_snapshot_rejected")
	if len(warnings) != 1 || len(recoveryEvents(records, "startup_failed")) != 0 {
		t.Fatalf("non-mutual dangling records=%#v", records)
	}
	quarantine := assertRecoveryWarning(t, warnings[0], harness.stateDir, string(storage.ReasonCorruptJSON))
	if got, err := os.ReadFile(quarantine); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("non-mutual snapshot quarantine=%q err=%v", got, err)
	}
	if counts := recoveryQuarantineCounts(t, harness.stateDir); counts["snapshot"] != 1 || counts["prompts"] != 0 {
		t.Fatalf("non-mutual dangling limitation violated: %v", counts)
	}
	info, err := os.Lstat(promptsPath)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("dangling link was not replaced by regular prompts: info=%v err=%v", info, err)
	}
	restartSnapshotRecoveryNoop(t, harness)
}

func assertOrphanedLinkRecords(t *testing.T, harness snapshotRecoveryHarness, run *snapshotRecoveryRun, original []byte) {
	t.Helper()
	records := recoveryLogRecords(t, run.stderr.String())
	warnings := recoveryEvents(records, "startup_snapshot_rejected")
	if len(warnings) != 2 || len(recoveryEvents(records, "startup_failed")) != 0 {
		t.Fatalf("orphaned-link records=%#v", records)
	}
	snapshotQuarantine := assertRecoveryWarning(t, warnings[0], harness.stateDir, string(storage.ReasonCorruptJSON))
	promptsQuarantine := assertRecoveryWarning(t, warnings[1], harness.stateDir, string(storage.ReasonOrphanedLink))
	if !strings.HasPrefix(filepath.Base(snapshotQuarantine), "snapshot.corrupt-") || !strings.HasPrefix(filepath.Base(promptsQuarantine), "prompts.corrupt-") {
		t.Fatalf("orphaned-link records not family-ordered: %q, %q", snapshotQuarantine, promptsQuarantine)
	}
	if got, err := os.ReadFile(snapshotQuarantine); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("orphaned snapshot bytes=%q err=%v", got, err)
	}
	if target, err := os.Readlink(promptsQuarantine); err != nil || target != storage.SnapshotFile {
		t.Fatalf("orphaned prompt link target=%q err=%v", target, err)
	}
}

func TestSnapshotRecoveryFatalDirectoryPaths(t *testing.T) {
	for _, setup := range []struct {
		name  string
		seed  func(*testing.T, string)
		check func(*testing.T, string)
	}{
		{
			name: "directory",
			seed: func(t *testing.T, path string) {
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, path string) {
				info, err := os.Stat(path)
				if err != nil || !info.IsDir() {
					t.Fatalf("directory changed: info=%v err=%v", info, err)
				}
			},
		},
		{
			name: "symlink to directory",
			seed: func(t *testing.T, path string) {
				if err := os.Mkdir(filepath.Join(filepath.Dir(path), "target"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("target", path); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, path string) {
				if target, err := os.Readlink(path); err != nil || target != "target" {
					t.Fatalf("directory link changed: target=%q err=%v", target, err)
				}
			},
		},
	} {
		t.Run(setup.name, func(t *testing.T) {
			t.Setenv("MCP_SNAPSHOT_QUARANTINE", "on")
			harness := newSnapshotRecoveryHarness(t)
			path := filepath.Join(harness.stateDir, storage.SnapshotFile)
			if err := os.MkdirAll(harness.stateDir, 0o700); err != nil {
				t.Fatal(err)
			}
			setup.seed(t, path)
			code, stderr := runSnapshotRecoveryToExit(t, harness)
			if code != 1 {
				t.Fatalf("directory fatal exit=%d; want 1", code)
			}
			setup.check(t, path)
			records := recoveryLogRecords(t, stderr)
			failures := recoveryEvents(records, "startup_failed")
			if len(failures) != 1 || len(recoveryEvents(records, "startup_snapshot_rejected")) != 0 {
				t.Fatalf("directory fatal records=%#v", records)
			}
			resource := failures[0]["resource"].(map[string]any)
			if resource["path"] != harness.stateDir || failures[0]["error"].(map[string]any)["code"] != "state_directory" {
				t.Fatalf("directory fatal record=%#v", failures[0])
			}
			if counts := recoveryQuarantineCounts(t, harness.stateDir); counts["snapshot"] != 0 || counts["prompts"] != 0 {
				t.Fatalf("directory fatal created quarantine: %v", counts)
			}
		})
	}
}

func TestSnapshotRecoverySelfMovedExactlyOnce(t *testing.T) {
	t.Setenv("MCP_SNAPSHOT_QUARANTINE", "on")
	harness := newSnapshotRecoveryHarness(t)
	if err := os.MkdirAll(harness.stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(storage.SnapshotFile, filepath.Join(harness.stateDir, storage.SnapshotFile)); err != nil {
		t.Fatal(err)
	}
	run := beginSnapshotRecoveryRun(t, harness)
	if run.finished {
		t.Fatalf("self-loop startup exited %d: %q", run.exitCode, run.stderr.String())
	}
	waitForFreshRecoveryState(t, harness)
	stopSnapshotRecoveryRun(t, run)
	records := recoveryLogRecords(t, run.stderr.String())
	if got := len(recoveryEvents(records, "startup_snapshot_rejected")); got != 1 {
		t.Fatalf("self-loop emitted %d records; want exactly one: %#v", got, records)
	}
	for _, record := range records {
		if record["failure_reason"] == storage.ReasonOrphanedLink {
			t.Fatalf("self-loop was falsely reconciled: %#v", record)
		}
	}
	assertDirectPromptSaveAndLoads(t, harness)
	restartSnapshotRecoveryNoop(t, harness)
}

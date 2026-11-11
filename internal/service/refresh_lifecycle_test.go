package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
)

func TestRefreshLifecycleRecords(t *testing.T) {
	t.Run("rejected candidate retains catalog and repeats diagnostic", func(t *testing.T) {
		h := newReloadHarness(t)
		h.setDefs("alpha", core.ToolDefinition{RawName: "read", InputSchema: []byte(`{"type":"object"}`)})
		initialConfig := refreshConfig(t, reloadAlphaOnly)
		if _, err := h.runtime.Reload(initialConfig, h.deps()); err != nil {
			t.Fatalf("initial reload: %v", err)
		}

		candidateText := strings.Replace(reloadAlphaOnly, "pins: []", "pins:\n  - {server: alpha, tool: missing}", 1)
		candidate := refreshConfig(t, candidateText)
		var stdout, stderr bytes.Buffer
		refresher := service.NewRefresher(h.runtime, func() *config.Config { return candidate }, refreshDeps(h), service.RefresherOptions{
			Interval: time.Minute,
			Logger:   observe.NewLogger(&stdout, &stderr),
		})
		for pass := int64(1); pass <= 2; pass++ {
			runRefreshLifecyclePass(t, refresher, pass)
		}

		records := decodeRefreshLifecycleRecords(t, stdout.String(), stderr.String())
		starts := refreshLifecycleEvents(records, "catalog_refresh_started")
		failures := refreshLifecycleEvents(records, "catalog_refresh_failed")
		if len(starts) != 2 || len(failures) != 2 {
			t.Fatalf("start/failure counts = %d/%d; want exactly 2/2: %s", len(starts), len(failures), stdout.String()+stderr.String())
		}
		var firstError, firstDetails map[string]any
		for i, failed := range failures {
			if failed["level"] != "warn" || failed["catalog_state"] != "retained_published" {
				t.Errorf("failure %d level/state = %v/%v; want warn/retained_published", i, failed["level"], failed["catalog_state"])
			}
			if failed["retained_generation"] != float64(1) || failed["catalog_tools"] != float64(1) {
				t.Errorf("failure %d retained generation/tools = %v/%v; want 1/1", i, failed["retained_generation"], failed["catalog_tools"])
			}
			if _, ok := failed["retained_age_ms"].(float64); !ok {
				t.Errorf("failure %d retained_age_ms = %#v; want a numeric age", i, failed["retained_age_ms"])
			}
			if failed["candidate_servers"] != float64(1) || failed["candidate_tools"] != float64(1) {
				t.Errorf("failure %d candidate server/tool counts = %v/%v; want 1/1", i, failed["candidate_servers"], failed["candidate_tools"])
			}
			if _, exists := failed["next_attempt_ms"]; exists {
				t.Errorf("non-retryable pin rejection carries next_attempt_ms: %#v", failed)
			}
			assertRefreshLifecyclePair(t, starts[i], failed, candidate.Fingerprint())
			gotError, ok := failed["error"].(map[string]any)
			if !ok || gotError["kind"] != "configuration" || gotError["retryable"] != false || gotError["code"] != "pin_rejected" {
				t.Errorf("failure %d error = %#v; want pin_rejected configuration error, retryable=false", i, failed["error"])
			}
			if i == 0 {
				firstError = gotError
				firstDetails = make(map[string]any, len(failed))
				for key, value := range failed {
					if key != "refresh_id" && key != "retained_age_ms" && key != "error" {
						firstDetails[key] = value
					}
				}
			} else {
				if !equalLifecycleMaps(firstError, gotError) {
					t.Errorf("repeat failure diagnostic = %#v; want complete first diagnostic %#v", gotError, firstError)
				}
				for key, value := range firstDetails {
					if failed[key] != value {
						t.Errorf("repeat failure %s = %v; want %v", key, failed[key], value)
					}
				}
			}
		}
		pinEvents := refreshLifecycleEvents(records, "pin_blocker_detail")
		if len(pinEvents) != 4 {
			t.Fatalf("pin blocker event count = %d; want one pin and one tool-list chunk per failed pass", len(pinEvents))
		}
		for _, failed := range failures {
			var passEvents []map[string]any
			for _, detail := range pinEvents {
				if detail["refresh_id"] == failed["refresh_id"] {
					passEvents = append(passEvents, detail)
				}
			}
			pinRecords := pinBlockerPinRecords(passEvents)
			chunks := pinBlockerToolChunks(passEvents)
			if len(pinRecords) != 1 || len(chunks) != 1 {
				t.Errorf("failed refresh %v pin/chunk records = %d/%d; want 1/1", failed["refresh_id"], len(pinRecords), len(chunks))
				continue
			}
			pin := pinRecords[0]
			configInfo := pin["config"].(map[string]any)
			if pin["level"] != "warn" || pin["server"] != "alpha" ||
				pin["blocker_class"] != "tool_absent" || pin["publication_impact"] != "candidate_rejected" ||
				configInfo["property_path"] != "pins[0]" || configInfo["pin_tool"] != "missing" {
				t.Errorf("pin blocker record = %#v; want warn alpha/missing at pins[0]", pin)
			}
			if got := decodedToolNames(t, chunks[0]["discovered_tools"]); !reflect.DeepEqual(got, []string{"read"}) {
				t.Errorf("discovered tools for failed refresh = %v; want [read]", got)
			}
			if chunks[0]["level"] != "warn" || chunks[0]["server"] != "alpha" ||
				chunks[0]["publication_impact"] != "candidate_rejected" {
				t.Errorf("pin tool chunk = %#v; want warn alpha/candidate_rejected", chunks[0])
			}
		}
		allOutput := stdout.String() + stderr.String()
		for _, forbidden := range []string{"core.AddPins", "not in catalog"} {
			if strings.Contains(allOutput, forbidden) {
				t.Errorf("refresh lifecycle output contains raw diagnostic text %q: %s", forbidden, allOutput)
			}
		}
		if got := refreshLifecycleEvents(records, "catalog_refresh_completed"); len(got) != 0 {
			t.Fatalf("rejected candidate emitted completion record: %#v", got)
		}
	})

	t.Run("failed pass with no usable catalog is error level", func(t *testing.T) {
		h := newReloadHarness(t)
		h.setDefs("alpha", core.ToolDefinition{RawName: "read", InputSchema: []byte(`{"type":"object"}`)})
		candidateText := strings.Replace(reloadAlphaOnly, "pins: []", "pins:\n  - {server: alpha, tool: missing}", 1)
		candidate := refreshConfig(t, candidateText)
		if _, err := h.runtime.AdmitStartup(candidate, h.deps(), nil); err != nil {
			t.Fatalf("initial startup admission: %v", err)
		}
		var stdout, stderr bytes.Buffer
		refresher := service.NewRefresher(h.runtime, func() *config.Config { return candidate }, refreshDeps(h), service.RefresherOptions{
			Interval: time.Minute,
			Logger:   observe.NewLogger(&stdout, &stderr),
		})
		runRefreshLifecyclePass(t, refresher, 1)

		records := decodeRefreshLifecycleRecords(t, stdout.String(), stderr.String())
		starts := refreshLifecycleEvents(records, "catalog_refresh_started")
		failures := refreshLifecycleEvents(records, "catalog_refresh_failed")
		if len(starts) != 1 || len(failures) != 1 {
			t.Fatalf("start/failure counts = %d/%d; want exactly 1/1", len(starts), len(failures))
		}
		failed := failures[0]
		if failed["level"] != "error" || failed["catalog_state"] != "none_available" {
			t.Errorf("failure level/state = %v/%v; want error/none_available", failed["level"], failed["catalog_state"])
		}
		if _, exists := failed["retained_generation"]; exists {
			t.Errorf("none_available failure has retained_generation: %#v", failed)
		}
		if failed["candidate_servers"] != float64(1) || failed["candidate_tools"] != float64(1) {
			t.Errorf("candidate server/tool counts = %v/%v; want 1/1", failed["candidate_servers"], failed["candidate_tools"])
		}
		if failed["pins_resolved"] != float64(0) || failed["pins_total"] != float64(1) {
			t.Errorf("failed pin counts = %v/%v; want 0/1", failed["pins_resolved"], failed["pins_total"])
		}
		errorInfo, ok := failed["error"].(map[string]any)
		if !ok || errorInfo["kind"] != "configuration" || errorInfo["retryable"] != false || errorInfo["code"] != "pin_rejected" {
			t.Errorf("failure error = %#v; want pin_rejected configuration error, retryable=false", failed["error"])
		}
		assertRefreshLifecyclePair(t, starts[0], failed, candidate.Fingerprint())
		if completed := refreshLifecycleEvents(records, "catalog_refresh_completed"); len(completed) != 0 {
			t.Errorf("failed pass emitted completion records: %#v", completed)
		}
	})
	t.Run("success reports counts without catalog names", func(t *testing.T) {
		h := newReloadHarness(t)
		h.setDefs("alpha", core.ToolDefinition{RawName: "read", InputSchema: []byte(`{"type":"object"}`)})
		candidateText := strings.Replace(reloadAlphaOnly, "pins: []", "pins:\n  - {server: alpha, tool: read}", 1)
		candidate := refreshConfig(t, candidateText)
		if _, err := h.runtime.AdmitStartup(candidate, h.deps(), nil); err != nil {
			t.Fatalf("initial startup admission: %v", err)
		}
		var stdout, stderr bytes.Buffer
		refresher := service.NewRefresher(h.runtime, func() *config.Config { return candidate }, refreshDeps(h), service.RefresherOptions{
			Interval: time.Minute,
			Logger:   observe.NewLogger(&stdout, &stderr),
		})
		runRefreshLifecyclePass(t, refresher, 1)

		records := decodeRefreshLifecycleRecords(t, stdout.String(), stderr.String())
		starts := refreshLifecycleEvents(records, "catalog_refresh_started")
		completed := refreshLifecycleEvents(records, "catalog_refresh_completed")
		if len(starts) != 1 || len(completed) != 1 {
			t.Fatalf("start/completion counts = %d/%d; want exactly 1/1", len(starts), len(completed))
		}
		if failures := refreshLifecycleEvents(records, "catalog_refresh_failed"); len(failures) != 0 {
			t.Errorf("successful pass emitted failure records: %#v", failures)
		}
		got := completed[0]
		for key, want := range map[string]float64{
			"servers_discovered": 1,
			"tools_discovered":   1,
			"tools_published":    1,
			"pins_resolved":      1,
			"pins_total":         1,
		} {
			if got[key] != want {
				t.Errorf("completion %s = %v; want %v", key, got[key], want)
			}
		}
		assertRefreshLifecyclePair(t, starts[0], got, candidate.Fingerprint())
		allOutput := stdout.String() + stderr.String()
		for _, forbidden := range []string{"alpha", "read"} {
			if strings.Contains(allOutput, forbidden) {
				t.Errorf("successful lifecycle output contains catalog name %q: %s", forbidden, allOutput)
			}
		}
		if got["level"] != "info" {
			t.Errorf("completion level = %v; want info", got["level"])
		}
	})
}

func runRefreshLifecyclePass(t *testing.T, refresher *service.Refresher, wantPasses int64) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		refresher.Run(ctx)
		close(done)
	}()
	refresher.Trigger()
	waitFor(t, "refresh lifecycle pass", func() bool { return refresher.Passes() == wantPasses })
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("refresh run loop did not stop after cancellation")
	}
}

func decodeRefreshLifecycleRecords(t *testing.T, outputs ...string) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, output := range outputs {
		for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
			if line == "" {
				continue
			}
			var record map[string]any
			if err := json.Unmarshal([]byte(line), &record); err != nil {
				t.Fatalf("decode refresh lifecycle record %q: %v", line, err)
			}
			records = append(records, record)
		}
	}
	return records
}

func refreshLifecycleEvents(records []map[string]any, event string) []map[string]any {
	var matches []map[string]any
	for _, record := range records {
		if record["event"] == event {
			matches = append(matches, record)
		}
	}
	return matches
}

func assertRefreshLifecyclePair(t *testing.T, start, outcome map[string]any, wantRevision string) {
	t.Helper()
	for _, record := range []map[string]any{start, outcome} {
		if record["refresh_id"] == "" || record["refresh_id"] != start["refresh_id"] {
			t.Errorf("lifecycle refresh_id = %v; want shared non-empty id %v", record["refresh_id"], start["refresh_id"])
		}
		if record["config_revision"] != wantRevision {
			t.Errorf("lifecycle config_revision = %v; want %s", record["config_revision"], wantRevision)
		}
		if record["trigger"] == "" || record["trigger"] != start["trigger"] {
			t.Errorf("lifecycle trigger = %v; want shared non-empty trigger %v", record["trigger"], start["trigger"])
		}
	}
	if _, ok := start["next_attempt_ms"].(float64); !ok {
		t.Errorf("start next_attempt_ms = %#v; want numeric delay", start["next_attempt_ms"])
	}
}

func equalLifecycleMaps(left, right map[string]any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

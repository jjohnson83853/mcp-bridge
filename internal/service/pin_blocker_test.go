package service_test

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
)

func TestRefreshReportsEveryPinBlockerWithConfigPath(t *testing.T) {
	h := newReloadHarness(t)
	h.setDefs("alpha", core.ToolDefinition{RawName: "read", InputSchema: []byte(`{"type":"object"}`)})
	if _, err := h.runtime.Reload(refreshConfig(t, reloadAlphaOnly), h.deps()); err != nil {
		t.Fatalf("initial reload: %v", err)
	}

	candidateText := strings.Replace(reloadAlphaOnly, "pins: []", "pins:\n  - {server: alpha, tool: missing_one}\n  - {server: alpha, tool: missing_two}", 1)
	candidate := refreshConfig(t, candidateText)
	records, output := runPinRefreshPass(t, h, candidate)
	details := refreshLifecycleEvents(records, "pin_blocker_detail")
	if len(details) != 3 {
		t.Fatalf("pin blocker detail count = %d; want two pin records and one tool-list chunk: %s", len(details), output)
	}
	failure := assertPinPassCorrelation(t, records, candidate)
	if failure["publication_impact"] != "candidate_rejected" {
		t.Errorf("failed pass publication_impact = %v; want candidate_rejected", failure["publication_impact"])
	}

	pinDetails := pinBlockerPinRecords(details)
	if len(pinDetails) != 2 {
		t.Fatalf("per-pin record count = %d; want 2", len(pinDetails))
	}
	wantTools := map[string]string{"pins[0]": "missing_one", "pins[1]": "missing_two"}
	for _, detail := range details {
		if detail["level"] != "warn" || detail["publication_impact"] != "candidate_rejected" {
			t.Errorf("pin blocker level/impact = %v/%v; want warn/candidate_rejected", detail["level"], detail["publication_impact"])
		}
	}
	for _, detail := range pinDetails {
		if detail["server"] != "alpha" || detail["blocker_class"] != "tool_absent" {
			t.Errorf("pin blocker identity/class = %v/%v; want alpha/tool_absent", detail["server"], detail["blocker_class"])
		}
		configInfo := detail["config"].(map[string]any)
		path, ok := configInfo["property_path"].(string)
		if !ok || wantTools[path] == "" {
			t.Errorf("pin property_path = %#v; want pins[0] or pins[1]", configInfo["property_path"])
			continue
		}
		if configInfo["pin_tool"] != wantTools[path] {
			t.Errorf("pin %s tool = %v; want %s", path, configInfo["pin_tool"], wantTools[path])
		}
		delete(wantTools, path)
		if _, hasCause := detail["failure_reason"]; hasCause {
			t.Errorf("tool-absent blocker unexpectedly carries discovery cause: %#v", detail)
		}
	}
	if len(wantTools) != 0 {
		t.Errorf("unreported pin paths: %v", wantTools)
	}
	chunks := pinBlockerToolChunks(details)
	if len(chunks) != 1 {
		t.Fatalf("discovered tool chunk count = %d; want 1", len(chunks))
	}
	if got := decodedToolNames(t, chunks[0]["discovered_tools"]); !reflect.DeepEqual(got, []string{"read"}) {
		t.Errorf("discovered tools = %v; want [read]", got)
	}
	if chunks[0]["chunk_index"] != float64(0) || chunks[0]["chunk_count"] != float64(1) {
		t.Errorf("small tool-list chunk index/count = %v/%v; want 0/1", chunks[0]["chunk_index"], chunks[0]["chunk_count"])
	}
	for _, forbidden := range []string{"core.AddPins", "not in catalog"} {
		if strings.Contains(output, forbidden) {
			t.Errorf("pin diagnostic contains raw error text %q: %s", forbidden, output)
		}
	}
}

func TestRefreshReportsUnverifiedPinWithDiscoveryCause(t *testing.T) {
	h := newReloadHarness(t)
	if _, err := h.runtime.Reload(refreshConfig(t, reloadAlphaOnly), h.deps()); err != nil {
		t.Fatalf("initial reload: %v", err)
	}
	h.failDiscovery("alpha", errors.New("private upstream failure detail"))
	candidateText := strings.Replace(reloadAlphaOnly, "pins: []", "pins:\n  - {server: alpha, tool: missing}", 1)
	candidate := refreshConfig(t, candidateText)
	records, output := runPinRefreshPass(t, h, candidate)
	details := refreshLifecycleEvents(records, "pin_blocker_detail")
	pinDetails := pinBlockerPinRecords(details)
	if len(pinDetails) != 1 {
		t.Fatalf("per-pin record count = %d; want 1: %s", len(pinDetails), output)
	}
	assertPinPassCorrelation(t, records, candidate)
	detail := pinDetails[0]
	if detail["level"] != "warn" || detail["server"] != "alpha" || detail["blocker_class"] != "server_unverified" {
		t.Errorf("unverified pin level/server/class = %v/%v/%v", detail["level"], detail["server"], detail["blocker_class"])
	}
	if detail["failure_reason"] != service.RefreshReasonUnavailable {
		t.Errorf("unverified pin failure_reason = %v; want %s", detail["failure_reason"], service.RefreshReasonUnavailable)
	}
	if len(pinBlockerToolChunks(details)) != 0 {
		t.Errorf("failed discovery emitted a tool listing: %#v", details)
	}
	if strings.Contains(output, "private upstream failure detail") {
		t.Errorf("refresh diagnostics serialized the discovery error: %s", output)
	}
}

func TestRefreshChunksMissingPinToolListingAndExcludesHealthyServers(t *testing.T) {
	h := newReloadHarness(t)
	wantNames := make([]string, 130)
	definitions := make([]core.ToolDefinition, 0, len(wantNames)+1)
	for index := range len(wantNames) {
		name := fmt.Sprintf("tool_%03d", index)
		wantNames[index] = name
		definitions = append(definitions, core.ToolDefinition{RawName: name})
	}
	definitions = append(definitions, core.ToolDefinition{RawName: "unsafe tool name"})
	h.setDefs("alpha", definitions...)
	if _, err := h.runtime.Reload(refreshConfig(t, reloadTwoServers), h.deps()); err != nil {
		t.Fatalf("initial reload: %v", err)
	}
	candidateText := strings.Replace(reloadTwoServers, "pins: []", "pins:\n  - {server: alpha, tool: missing}", 1)
	candidate := refreshConfig(t, candidateText)
	records, output := runPinRefreshPass(t, h, candidate)
	details := refreshLifecycleEvents(records, "pin_blocker_detail")
	failure := assertPinPassCorrelation(t, records, candidate)
	if failure["publication_impact"] != "candidate_rejected" {
		t.Errorf("failed pass publication_impact = %v; want candidate_rejected", failure["publication_impact"])
	}
	pinDetails := pinBlockerPinRecords(details)
	if len(pinDetails) != 1 || pinDetails[0]["blocker_class"] != "tool_absent" {
		t.Fatalf("missing-pin records = %#v; want one tool_absent pin", pinDetails)
	}

	chunks := pinBlockerToolChunks(details)
	if len(chunks) != 3 {
		t.Fatalf("tool chunk count = %d; want 3 for 130 names: %s", len(chunks), output)
	}
	var gotNames []string
	for index, chunk := range chunks {
		if chunk["level"] != "warn" || chunk["server"] != "alpha" || chunk["blocker_class"] != "tool_absent" || chunk["publication_impact"] != "candidate_rejected" {
			t.Errorf("chunk %d level/server/class/impact = %v/%v/%v/%v", index, chunk["level"], chunk["server"], chunk["blocker_class"], chunk["publication_impact"])
		}
		if chunk["refresh_id"] != failure["refresh_id"] {
			t.Errorf("chunk %d refresh_id = %v; want %v", index, chunk["refresh_id"], failure["refresh_id"])
		}
		if chunk["chunk_index"] != float64(index) || chunk["chunk_count"] != float64(3) {
			t.Errorf("chunk %d index/count = %v/%v; want %d/3", index, chunk["chunk_index"], chunk["chunk_count"], index)
		}
		names := decodedToolNames(t, chunk["discovered_tools"])
		if len(names) > 64 {
			t.Errorf("chunk %d contains %d names; maximum is 64", index, len(names))
		}
		gotNames = append(gotNames, names...)
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Errorf("chunked discovered tools = %d names, want all %d names in order", len(gotNames), len(wantNames))
	}
	for _, record := range details {
		if record["server"] == "beta" {
			t.Errorf("healthy server appeared in blocker details: %#v", record)
		}
	}
	if strings.Contains(output, "unsafe tool name") || strings.Contains(output, "write") {
		t.Errorf("diagnostics included an invalid or healthy tool name: %s", output)
	}
}

func runPinRefreshPass(t *testing.T, h *reloadHarness, candidate *config.Config) ([]map[string]any, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	refresher := service.NewRefresher(h.runtime, func() *config.Config { return candidate }, refreshDeps(h), service.RefresherOptions{
		Interval: time.Minute,
		Logger:   observe.NewLogger(&stdout, &stderr),
	})
	runRefreshLifecyclePass(t, refresher, 1)
	output := stdout.String() + stderr.String()
	return decodeRefreshLifecycleRecords(t, stdout.String(), stderr.String()), output
}

func assertPinPassCorrelation(t *testing.T, records []map[string]any, candidate *config.Config) map[string]any {
	t.Helper()
	starts := refreshLifecycleEvents(records, "catalog_refresh_started")
	failures := refreshLifecycleEvents(records, "catalog_refresh_failed")
	if len(starts) != 1 || len(failures) != 1 {
		t.Fatalf("refresh start/failure counts = %d/%d; want 1/1", len(starts), len(failures))
	}
	assertRefreshLifecyclePair(t, starts[0], failures[0], candidate.Fingerprint())
	for _, detail := range refreshLifecycleEvents(records, "pin_blocker_detail") {
		if detail["refresh_id"] != failures[0]["refresh_id"] {
			t.Errorf("pin detail refresh_id = %v; want failed pass %v", detail["refresh_id"], failures[0]["refresh_id"])
		}
		if detail["publication_impact"] != "candidate_rejected" {
			t.Errorf("pin detail publication_impact = %v; want candidate_rejected", detail["publication_impact"])
		}
	}
	return failures[0]
}

func pinBlockerPinRecords(records []map[string]any) []map[string]any {
	var pins []map[string]any
	for _, record := range records {
		configInfo, ok := record["config"].(map[string]any)
		if ok && configInfo["property_path"] != nil {
			pins = append(pins, record)
		}
	}
	return pins
}

func pinBlockerToolChunks(records []map[string]any) []map[string]any {
	var chunks []map[string]any
	for _, record := range records {
		if _, ok := record["discovered_tools"]; ok {
			chunks = append(chunks, record)
		}
	}
	return chunks
}

func decodedToolNames(t *testing.T, value any) []string {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("discovered_tools = %#v; want an array", value)
	}
	names := make([]string, len(items))
	for index, item := range items {
		name, ok := item.(string)
		if !ok {
			t.Fatalf("discovered_tools[%d] = %#v; want string", index, item)
		}
		names[index] = name
	}
	return names
}

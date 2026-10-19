package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

// TestServeLoggingShapes exercises the real composition root through a
// complete tools/call flow and asserts the upstream/final result-shape
// observability contract: the raw upstream result shape must be captured on
// backend_complete, and the client-facing final result shape (including the
// bridge's own structuredContent wrapper) must be captured on mcp_complete.
func TestServeLoggingShapes(t *testing.T) {
	t.Run("TextOnlyUpstreamProducesStructuredFinalShape", func(t *testing.T) {
		ctx, httpURL, stdout, stop := startHTTPLoggingBridge(t)

		client := &http.Client{Timeout: 5 * time.Second}
		send := func(body string, session string) (*http.Response, []byte) {
			t.Helper()
			request, err := http.NewRequestWithContext(ctx, http.MethodPost, httpURL+"/mcp", strings.NewReader(body))
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			request.Header.Set("Authorization", "Bearer "+serveClientToken)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "application/json, text/event-stream")
			if session != "" {
				request.Header.Set("Mcp-Session-Id", session)
				request.Header.Set("Mcp-Protocol-Version", "2025-11-25")
			}
			response, err := client.Do(request)
			if err != nil {
				t.Fatalf("send %s: %v", body, err)
			}
			content, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil {
				t.Fatalf("read response body: %v", err)
			}
			return response, content
		}

		// Phase 1: initialize to obtain the session identity.
		const initializeBody = `{"jsonrpc":"2.0","id":"shapes-init-id","method":"initialize",` +
			`"params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"shapes-client","version":"1"}}}`
		initResponse, initBody := send(initializeBody, "")
		if initResponse.StatusCode != http.StatusOK {
			t.Fatalf("initialize status = %d; want 200; body %s", initResponse.StatusCode, string(initBody))
		}
		session := initResponse.Header.Get("Mcp-Session-Id")
		if session == "" {
			t.Fatal("initialize omitted session identity")
		}

		// Phase 2: notifications/initialized handshake required before tools/call.
		send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`, session)

		// Phase 2b: search_tools to discover the published catalog revision.
		searchBody := `{"jsonrpc":"2.0","id":"shapes-search-id","method":"tools/call",` +
			`"params":{"name":"search_tools","arguments":{"query":"read"}}}`
		searchResp, searchRaw := send(searchBody, session)
		if searchResp.StatusCode != http.StatusOK {
			t.Fatalf("search_tools status = %d; body %s", searchResp.StatusCode, string(searchRaw))
		}
		searchEnvelope := extractJSONRPCPayload(t, string(searchRaw))
		searchResult, ok := searchEnvelope["result"].(map[string]any)
		if !ok {
			t.Fatalf("search_tools missing result object: %v", searchEnvelope)
		}
		searchStructured, ok := searchResult["structuredContent"].(map[string]any)
		if !ok {
			t.Fatalf("search_tools missing structuredContent: %v", searchResult)
		}
		catalogRev, _ := searchStructured["catalog_revision"].(string)
		if catalogRev == "" {
			t.Fatalf("search_tools missing catalog_revision in %v", searchStructured)
		}

		// Phase 3: tools/call whose upstream answer is a single text content
		// block without structuredContent; the admitted public tool name is
		// derived from the catalog exactly as the wire contract requires.
		argsJSON, _ := json.Marshal(map[string]any{
			"tool_name":        core.PublicName("alpha", "read"),
			"catalog_revision": catalogRev,
			"arguments": map[string]any{
				"query": "test",
			},
		})
		callBody := `{"jsonrpc":"2.0","id":"call-shape-id","method":"tools/call",` +
			`"params":{"name":"call_tool","arguments":` + string(argsJSON) + `}}`
		response, body := send(callBody, session)

		// Public assertions: transport and JSON-RPC behavior stay PASS.
		if response.StatusCode != http.StatusOK {
			t.Fatalf("public status = %d; want 200; body %s", response.StatusCode, string(body))
		}
		envelope := extractJSONRPCPayload(t, string(body))
		if envelope == nil {
			t.Fatalf("public response missing JSON-RPC payload: %q", string(body))
		}
		if envelope["jsonrpc"] != "2.0" {
			t.Fatalf("public response jsonrpc = %v; want 2.0", envelope["jsonrpc"])
		}
		_, hasResult := envelope["result"]
		_, hasError := envelope["error"]
		if !hasResult || hasError {
			t.Fatalf("public response must carry a JSON-RPC success result; got %v", envelope)
		}

		// Drain the runtime before inspecting captured terminal records.
		stop()
		records := decodeCapturedRecords(t, stdout.String())

		// Boundary A: the backend_complete record carries the raw upstream
		// result shape. The upstream is text-only, so structured_presence
		// must be absent.
		backendCompletions := filterRecords(records, "backend_complete")
		if len(backendCompletions) != 1 {
			t.Fatalf("backend_complete count = %d; want exactly 1 for the admitted dispatch", len(backendCompletions))
		}
		backend := backendCompletions[0]
		upstreamShape := shapeFields(t, backend, "upstream_shape", "upstream")
		wantUpstreamShape := map[string]any{
			"content_count":       float64(1),
			"text_presence":       "present",
			"structured_presence": "absent",
		}
		wantShapeFields(t, upstreamShape, wantUpstreamShape, "upstream", backend)

		// Boundary B: the mcp_complete record carries the client-facing
		// final result shape. The bridge attaches its own structured wrapper
		// to text-only upstreams, so structured_presence must be present.
		var callOps []map[string]any
		for _, rec := range filterRecords(records, "mcp_complete") {
			if rec["mcp_method"] == "tools_call" && rec["outcome"] == "success" {
				callOps = append(callOps, rec)
			}
		}
		if len(callOps) != 1 {
			t.Fatalf("mcp_complete count = %d; want exactly 1 for the successful decoded tools/call", len(callOps))
		}
		mcp := callOps[0]
		finalShape := shapeFields(t, mcp, "final_shape", "final")
		wantFinalShape := map[string]any{
			"content_count":       float64(1),
			"text_presence":       "present",
			"structured_presence": "present",
		}
		wantShapeFields(t, finalShape, wantFinalShape, "final", mcp)
	})
}

// shapeFields extracts a result-shape map from a log record, accepting either
// a nested shape object under nestedKey or flattened prefix_* fields.
func shapeFields(t *testing.T, record map[string]any, nestedKey, prefix string) map[string]any {
	t.Helper()
	if shape, ok := record[nestedKey].(map[string]any); ok {
		return shape
	}
	flat := make(map[string]any)
	for _, field := range []string{"content_count", "text_presence", "structured_presence"} {
		if value, ok := record[prefix+"_"+field]; ok {
			flat[field] = value
		}
	}
	return flat
}

// wantShapeFields asserts every expected shape field is present with the
// expected value, failing with a deterministic message on the first mismatch.
func wantShapeFields(t *testing.T, got, want map[string]any, label string, record map[string]any) {
	t.Helper()
	for _, field := range []string{"content_count", "text_presence", "structured_presence"} {
		value, ok := got[field]
		if !ok {
			t.Errorf("%s shape missing %q in record %v; want %v", label, field, record, want[field])
			continue
		}
		if value != want[field] {
			t.Errorf("%s shape %s = %v; want %v", label, field, value, want[field])
		}
	}
	if len(got) > len(want) {
		t.Errorf("%s shape has %d fields; want exactly %d: %v", label, len(got), len(want), got)
	}
}

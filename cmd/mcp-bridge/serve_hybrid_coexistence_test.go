package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// writeToolSchema is the exact upstream schema the unpinned write tool must
// expose to the hybrid catalog for long-tail hash naming.
const writeToolSchema = `{"properties":{"path":{"type":"string"},"data":{"type":"string"}},"required":["path","data"],"type":"object"}`

// fakeUpstreamReadAndWriteTools serves the alpha upstream providing the pinned
// read tool and the unpinned write tool for coexistence testing.
func fakeUpstreamReadAndWriteTools(t *testing.T) *httptest.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "alpha", Version: "1"}, nil)
	server.AddTool(&mcp.Tool{
		Name:        "read",
		Description: "Read a file from disk",
		InputSchema: json.RawMessage(readToolSchema),
	}, func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
		}, nil
	})
	server.AddTool(&mcp.Tool{
		Name:        "write",
		Description: "Write data to a file on disk",
		InputSchema: json.RawMessage(writeToolSchema),
	}, func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "written"}},
		}, nil
	})
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true},
	))
	t.Cleanup(httpServer.Close)
	return httpServer
}

// TestServeHybridCoexistence verifies that the hybrid bridge exposes pinned
// wire names and long-tail hash names side by side, and that the meta tools
// route wire-name lookups correctly.
//
// RED state: describe_tool and call_tool currently resolve only hash names,
// so wire-name lookups fail deterministically with unknown_tool.
func TestServeHybridCoexistence(t *testing.T) {
	upstream := fakeUpstreamReadAndWriteTools(t)
	ctx, httpURL, _, _ := startHybridBridgeWithUpstream(t, upstream)
	client := &http.Client{Timeout: 5 * time.Second}

	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"hybrid-coexistence-client","version":"1"}}}`
	response, body := mcpSend(t, ctx, client, httpURL, "", initialize)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d; want 200 (body=%s)", response.StatusCode, body)
	}
	session := response.Header.Get("Mcp-Session-Id")
	if session == "" {
		t.Fatal("initialize omitted Mcp-Session-Id")
	}

	response, body = mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("initialized notification status = %d; want 202 (body=%s)", response.StatusCode, body)
	}

	extractSSEData := func(t *testing.T, raw []byte) []byte {
		t.Helper()
		payload := raw
		if index := bytes.Index(payload, []byte("data: ")); index >= 0 {
			payload = payload[index+len("data: "):]
			if end := bytes.IndexByte(payload, '\n'); end >= 0 {
				payload = payload[:end]
			}
		}
		return payload
	}

	decodeEnvelope := func(t *testing.T, raw []byte, step string) (id int, resultIsError bool, text string, rpcError *string) {
		t.Helper()
		payload := extractSSEData(t, raw)
		var envelope struct {
			JSONRPC string `json:"jsonrpc"`
			ID      int    `json:"id"`
			Result  *struct {
				IsError bool `json:"isError"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(payload, &envelope); err != nil {
			t.Fatalf("%s response not JSON: %v (body=%s)", step, err, raw)
		}
		if envelope.Error != nil {
			message := envelope.Error.Message
			return envelope.ID, false, "", &message
		}
		if envelope.Result == nil {
			t.Fatalf("%s envelope missing result and error (payload=%s)", step, payload)
		}
		var builder strings.Builder
		for _, item := range envelope.Result.Content {
			builder.WriteString(item.Text)
		}
		return envelope.ID, envelope.Result.IsError, builder.String(), nil
	}

	var catalogRevision string

	t.Run("SearchToolsPinnedAndUnpinned", func(t *testing.T) {
		response, body := mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search_tools","arguments":{"query":"file"}}}`)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("search_tools status = %d; want 200 (body=%s)", response.StatusCode, body)
		}
		id, resultIsError, text, rpcError := decodeEnvelope(t, body, "SearchToolsPinnedAndUnpinned")
		if rpcError != nil {
			t.Fatalf("search_tools returned JSON-RPC error %q (body=%s)", *rpcError, body)
		}
		if id != 2 {
			t.Errorf("search_tools response id = %d; want 2 (body=%s)", id, body)
		}
		if resultIsError {
			t.Fatalf("search_tools result isError = true (text=%q, body=%s); want isError false", text, body)
		}
		var searchResponse struct {
			CatalogRevision string `json:"catalog_revision"`
			Results         []struct {
				ToolName     string `json:"tool_name"`
				Server       string `json:"server"`
				UpstreamName string `json:"upstream_name"`
				Pinned       bool   `json:"pinned"`
			} `json:"results"`
		}
		if err := json.Unmarshal([]byte(text), &searchResponse); err != nil {
			t.Fatalf("search_tools content not JSON: %v (text=%q)", err, text)
		}
		if searchResponse.CatalogRevision == "" {
			t.Errorf("search_tools content missing catalog_revision (text=%q)", text)
		}
		catalogRevision = searchResponse.CatalogRevision

		var foundRead, foundWrite bool
		for _, result := range searchResponse.Results {
			if result.Server == "alpha" && result.UpstreamName == "read" {
				foundRead = true
				if !result.Pinned {
					t.Errorf("alpha/read result pinned = false; want true")
				}
				if result.ToolName != "alpha__read" {
					t.Errorf("alpha/read result tool_name = %q; want %q", result.ToolName, "alpha__read")
				}
			}
			if result.Server == "alpha" && result.UpstreamName == "write" {
				foundWrite = true
				if result.Pinned {
					t.Errorf("alpha/write result pinned = true; want false")
				}
				if !regexp.MustCompile(`^t_[a-z2-7]{52}$`).MatchString(result.ToolName) {
					t.Errorf("alpha/write result tool_name = %q; want hash name matching ^t_[a-z2-7]{52}$", result.ToolName)
				}
			}
		}
		if !foundRead {
			t.Errorf("search_tools results missing alpha/read (text=%q)", text)
		}
		if !foundWrite {
			t.Errorf("search_tools results missing alpha/write (text=%q)", text)
		}
	})

	t.Run("DescribeToolByWireName", func(t *testing.T) {
		if catalogRevision == "" {
			t.Skip("search_tools did not yield a catalog revision; wire lookups cannot be exercised")
		}
		payload, err := json.Marshal(map[string]any{
			"tool_name":        "alpha__read",
			"catalog_revision": catalogRevision,
		})
		if err != nil {
			t.Fatalf("marshal describe_tool arguments: %v", err)
		}
		response, body := mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"describe_tool","arguments":`+string(payload)+`}}`)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("describe_tool status = %d; want 200 (body=%s)", response.StatusCode, body)
		}
		id, resultIsError, text, rpcError := decodeEnvelope(t, body, "DescribeToolByWireName")
		if rpcError != nil {
			t.Fatalf("describe_tool returned JSON-RPC error %q (body=%s)", *rpcError, body)
		}
		if id != 3 {
			t.Errorf("describe_tool response id = %d; want 3 (body=%s)", id, body)
		}
		if resultIsError {
			t.Errorf("describe_tool result isError = true (text=%q, body=%s); want isError false; unknown_tool rejection present: %v", text, body, strings.Contains(string(body), "unknown_tool"))
		}
		var describeResponse struct {
			UpstreamName string          `json:"upstream_name"`
			Pinned       bool            `json:"pinned"`
			InputSchema  json.RawMessage `json:"input_schema"`
		}
		if err := json.Unmarshal([]byte(text), &describeResponse); err != nil {
			t.Fatalf("describe_tool content not JSON: %v (text=%q)", err, text)
		}
		if describeResponse.UpstreamName != "read" {
			t.Errorf("describe_tool upstream_name = %q; want %q", describeResponse.UpstreamName, "read")
		}
		if !describeResponse.Pinned {
			t.Errorf("describe_tool pinned = false; want true")
		}
		var schema map[string]any
		if err := json.Unmarshal(describeResponse.InputSchema, &schema); err != nil {
			t.Errorf("describe_tool input_schema not JSON: %v (raw=%q)", err, describeResponse.InputSchema)
		} else {
			expectedSchema := map[string]any{
				"properties": map[string]any{
					"path": map[string]any{"type": "string"},
				},
				"required": []any{"path"},
				"type":     "object",
			}
			if !reflect.DeepEqual(schema, expectedSchema) {
				t.Errorf("describe_tool input_schema = %v; want %v", schema, expectedSchema)
			}
		}
	})

	t.Run("CallToolEnvelopeByWireName", func(t *testing.T) {
		if catalogRevision == "" {
			t.Skip("search_tools did not yield a catalog revision; wire lookups cannot be exercised")
		}
		payload, err := json.Marshal(map[string]any{
			"tool_name":        "alpha__read",
			"catalog_revision": catalogRevision,
			"arguments":        map[string]any{"path": "test.txt"},
		})
		if err != nil {
			t.Fatalf("marshal call_tool arguments: %v", err)
		}
		response, body := mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"call_tool","arguments":`+string(payload)+`}}`)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("call_tool status = %d; want 200 (body=%s)", response.StatusCode, body)
		}
		id, resultIsError, text, rpcError := decodeEnvelope(t, body, "CallToolEnvelopeByWireName")
		if rpcError != nil {
			t.Fatalf("call_tool returned JSON-RPC error %q (body=%s)", *rpcError, body)
		}
		if id != 4 {
			t.Errorf("call_tool response id = %d; want 4 (body=%s)", id, body)
		}
		if resultIsError {
			t.Errorf("call_tool result isError = true (text=%q, body=%s); want isError false; unknown_tool rejection present: %v", text, body, strings.Contains(string(body), "unknown_tool"))
		}
		if !strings.Contains(text, "ok") {
			t.Errorf("call_tool content %q does not contain %q; unknown_tool rejection present: %v", text, "ok", strings.Contains(string(body), "unknown_tool"))
		}
	})
}

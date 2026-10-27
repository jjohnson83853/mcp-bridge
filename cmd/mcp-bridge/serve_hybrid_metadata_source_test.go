package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// metadataUpstreamServer serves the alpha read tool with the full supported
// upstream metadata set: title, annotations with explicit false pointer hints,
// icons and supported _meta keys.
func metadataUpstreamServer(t *testing.T) *httptest.Server {
	t.Helper()
	destructiveFalse := false
	server := mcp.NewServer(&mcp.Implementation{Name: "alpha", Version: "1"}, nil)
	server.AddTool(&mcp.Tool{
		Name:        "read",
		Description: "Read a file from disk",
		Title:       "Read a file",
		InputSchema: json.RawMessage(readToolSchema),
		Annotations: &mcp.ToolAnnotations{
			Title:           "Read",
			ReadOnlyHint:    true,
			DestructiveHint: &destructiveFalse,
			IdempotentHint:  true,
		},
		Icons: []mcp.Icon{{Source: "https://example.test/read.svg", MIMEType: "image/svg+xml"}},
		Meta:  mcp.Meta{"audience": []any{"user"}},
	}, func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
		}, nil
	})
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true},
	))
	t.Cleanup(httpServer.Close)
	return httpServer
}

// listedTool is the wire shape of one tools/list entry the source roundtrip
// asserts on.
type listedTool struct {
	Name        string          `json:"name"`
	Title       string          `json:"title"`
	Annotations json.RawMessage `json:"annotations"`
	Icons       json.RawMessage `json:"icons"`
}

// TestServeHybridMetadataSourceRoundTrip verifies on the real /mcp seam that
// the pinned alpha__read tool reaches clients with its full SDK-normalized
// source metadata: tools/list carries annotations, title and icons, and
// describe_tool reports the upstream metadata verbatim.
func TestServeHybridMetadataSourceRoundTrip(t *testing.T) {
	ctx, httpURL, _, _ := startHybridBridgeWithUpstream(t, metadataUpstreamServer(t))
	client := &http.Client{Timeout: 5 * time.Second}

	response, body := mcpSend(t, ctx, client, httpURL, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"metadata-source-client","version":"1"}}}`)
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

	response, body = mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("tools/list status = %d; want 200 (body=%s)", response.StatusCode, body)
	}
	payload := ssePayload(body)
	var envelope struct {
		Result *struct {
			Tools []listedTool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("decode tools/list response: %v (%s)", err, payload)
	}
	var preloaded *listedTool
	for index := range envelope.Result.Tools {
		if envelope.Result.Tools[index].Name == "alpha__read" {
			preloaded = &envelope.Result.Tools[index]
			break
		}
	}
	if preloaded == nil {
		t.Fatalf("tools/list omitted the pinned alpha__read tool: %s", payload)
	}
	if preloaded.Title != "Read a file" {
		t.Fatalf("tools/list title for alpha__read = %q; want %q", preloaded.Title, "Read a file")
	}
	if len(preloaded.Annotations) == 0 {
		t.Fatal("tools/list omitted annotations for alpha__read")
	}
	var annotations struct {
		Title       string `json:"title"`
		ReadOnly    *bool  `json:"readOnlyHint"`
		Destructive *bool  `json:"destructiveHint"`
		Idempotent  *bool  `json:"idempotentHint"`
	}
	if err := json.Unmarshal(preloaded.Annotations, &annotations); err != nil {
		t.Fatalf("decode alpha__read annotations: %v (%s)", err, preloaded.Annotations)
	}
	if annotations.Title != "Read" || annotations.ReadOnly == nil || !*annotations.ReadOnly || annotations.Idempotent == nil || !*annotations.Idempotent {
		t.Fatalf("alpha__read annotations lost the source hints: %s", preloaded.Annotations)
	}
	if annotations.Destructive == nil || *annotations.Destructive {
		t.Fatalf("alpha__read annotations lost the explicit destructiveHint false: %s", preloaded.Annotations)
	}
	if len(preloaded.Icons) == 0 {
		t.Fatal("tools/list omitted icons for alpha__read")
	}

	// Describe must answer with the same source metadata.
	response, body = mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search_tools","arguments":{"query":"read"}}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("search_tools status = %d; want 200 (body=%s)", response.StatusCode, body)
	}
	var search struct {
		Result *struct {
			StructuredContent json.RawMessage `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(ssePayload(body), &search); err != nil {
		t.Fatalf("decode search_tools response: %v (%s)", err, body)
	}
	var searchPayload struct {
		Results []struct {
			ToolName        string `json:"tool_name"`
			CatalogRevision string `json:"catalog_revision"`
		} `json:"results"`
	}
	if err := json.Unmarshal(search.Result.StructuredContent, &searchPayload); err != nil {
		t.Fatalf("decode search payload: %v (%s)", err, search.Result.StructuredContent)
	}
	if len(searchPayload.Results) == 0 {
		t.Fatalf("search_tools returned no results: %s", search.Result.StructuredContent)
	}
	revision := ""
	toolName := ""
	for _, result := range searchPayload.Results {
		if result.ToolName == "alpha__read" {
			revision, toolName = result.CatalogRevision, result.ToolName
			break
		}
	}
	if toolName == "" || len(revision) != 64 {
		t.Fatalf("search_tools did not return alpha__read with a revision: %s", search.Result.StructuredContent)
	}

	describe := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"describe_tool","arguments":{"tool_name":"` + toolName + `","catalog_revision":"` + revision + `"}}}`
	response, body = mcpSend(t, ctx, client, httpURL, session, describe)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("describe_tool status = %d; want 200 (body=%s)", response.StatusCode, body)
	}
	var described struct {
		Result *struct {
			StructuredContent json.RawMessage `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(ssePayload(body), &described); err != nil {
		t.Fatalf("decode describe response: %v (%s)", err, body)
	}
	var describePayload struct {
		UpstreamMetadata *struct {
			Title       string          `json:"title"`
			Annotations json.RawMessage `json:"annotations"`
			Icons       json.RawMessage `json:"icons"`
			Meta        json.RawMessage `json:"_meta"`
		} `json:"upstream_metadata"`
	}
	if err := json.Unmarshal(described.Result.StructuredContent, &describePayload); err != nil {
		t.Fatalf("decode describe payload: %v (%s)", err, described.Result.StructuredContent)
	}
	if describePayload.UpstreamMetadata == nil {
		t.Fatalf("describe omitted upstream_metadata: %s", described.Result.StructuredContent)
	}
	if describePayload.UpstreamMetadata.Title != "Read a file" {
		t.Fatalf("describe upstream_metadata title = %q; want %q", describePayload.UpstreamMetadata.Title, "Read a file")
	}
	if len(describePayload.UpstreamMetadata.Annotations) == 0 || len(describePayload.UpstreamMetadata.Icons) == 0 || len(describePayload.UpstreamMetadata.Meta) == 0 {
		t.Fatalf("describe upstream_metadata lost members: %+v", describePayload.UpstreamMetadata)
	}
}

// ssePayload extracts the first SSE data line of a response body, or returns
// the body unchanged when it is plain JSON.
func ssePayload(body []byte) []byte {
	payload := body
	if index := bytes.Index(payload, []byte("data: ")); index >= 0 {
		payload = payload[index+len("data: "):]
		if end := bytes.IndexByte(payload, '\n'); end >= 0 {
			payload = payload[:end]
		}
	}
	return payload
}

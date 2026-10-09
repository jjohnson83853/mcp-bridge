package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var legacyVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

func TestLatestInitializeOfferNegotiatesDownOnWire(t *testing.T) {
	server := service.NewMCPServer()
	httpServer := httptest.NewServer(service.NewStreamableHandler(server))
	defer httpServer.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "wire-client", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: httpServer.URL, MaxRetries: -1}, &mcp.ClientSessionOptions{ProtocolVersion: "2026-07-28"})
	if err != nil {
		t.Fatalf("connect with latest offer: %v", err)
	}
	defer session.Close()
	if got := session.InitializeResult().ProtocolVersion; got != "2025-11-25" {
		t.Fatalf("negotiated version=%q, want 2025-11-25", got)
	}
}

func TestLatestDiscoverProbeReturnsExactlyNarrowedVersions(t *testing.T) {
	httpServer := httptest.NewServer(service.NewStreamableHandler(service.NewMCPServer()))
	defer httpServer.Close()
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"wire-client","version":"1"},"io.modelcontextprotocol/clientCapabilities":{"sampling":{}}}}}`)
	req, err := http.NewRequest(http.MethodPost, httpServer.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", "2026-07-28")
	req.Header.Set("Mcp-Method", "server/discover")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, responseBody)
	}
	payload := responseBody
	if index := bytes.Index(payload, []byte("data: ")); index >= 0 {
		payload = payload[index+len("data: "):]
		if end := bytes.IndexByte(payload, '\n'); end >= 0 {
			payload = payload[:end]
		}
	}
	var result struct {
		Result mcp.DiscoverResult `json:"result"`
	}
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("decode response %q: %v", responseBody, err)
	}
	if len(result.Result.SupportedVersions) != len(legacyVersions) {
		t.Fatalf("supported versions=%v, want %v", result.Result.SupportedVersions, legacyVersions)
	}
	for i, version := range legacyVersions {
		if result.Result.SupportedVersions[i] != version {
			t.Fatalf("supported versions=%v, want %v", result.Result.SupportedVersions, legacyVersions)
		}
	}
}

func TestLatestToolsCallRejectedWithNarrowedVersionsAndNoExecution(t *testing.T) {
	server := service.NewMCPServer()
	calls := 0
	server.AddTool(&mcp.Tool{Name: "count", InputSchema: json.RawMessage(`{"type":"object"}`)}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		calls++
		return &mcp.CallToolResult{}, nil
	})
	httpServer := httptest.NewServer(service.NewStreamableHandler(server))
	defer httpServer.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "wire-client", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: httpServer.URL, MaxRetries: -1}, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	body := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"count","arguments":{},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`)
	req, err := http.NewRequest(http.MethodPost, httpServer.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", "2026-07-28")
	req.Header.Set("Mcp-Session-Id", session.ID())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400", resp.StatusCode, responseBody)
	}
	var rpcResponse struct {
		Error *struct {
			Code int `json:"code"`
			Data struct {
				Supported []string `json:"supported"`
			} `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(responseBody, &rpcResponse); err != nil {
		t.Fatalf("decode response %q: %v", responseBody, err)
	}
	if rpcResponse.Error == nil || rpcResponse.Error.Code != mcp.CodeUnsupportedProtocolVersion {
		t.Fatalf("error=%+v, want unsupported protocol version", rpcResponse.Error)
	}
	if len(rpcResponse.Error.Data.Supported) != len(legacyVersions) {
		t.Fatalf("advertised versions=%v, want %v", rpcResponse.Error.Data.Supported, legacyVersions)
	}
	for i, version := range legacyVersions {
		if rpcResponse.Error.Data.Supported[i] != version {
			t.Fatalf("advertised versions=%v, want %v", rpcResponse.Error.Data.Supported, legacyVersions)
		}
	}
	if calls != 0 {
		t.Fatalf("tool executed %d times, want 0", calls)
	}
}

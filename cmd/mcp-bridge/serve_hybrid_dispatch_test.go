package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestServeHybridDirectPreloadedToolExecution verifies direct preloaded tool
// execution dispatch against the pinned alpha__read tool on the hybrid bridge.
//
// RED state: internal/service/tools.go pendingDispatch currently rejects every
// tools/call with "tool call dispatch pending", so the Success subtest fails
// deterministically until the dispatcher is implemented.
func TestServeHybridDirectPreloadedToolExecution(t *testing.T) {
	upstream := fakeUpstreamReadTool(t)
	ctx, httpURL, _, _ := startHybridBridgeWithUpstream(t, upstream)
	client := &http.Client{Timeout: 5 * time.Second}

	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"hybrid-dispatch-client","version":"1"}}}`
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

	t.Run("Success", func(t *testing.T) {
		response, body := mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"alpha__read","arguments":{"path":"hello.txt"}}}`)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("tools/call status = %d; want 200 (body=%s)", response.StatusCode, body)
		}
		id, resultIsError, text, rpcError := decodeEnvelope(t, body, "Success tools/call")
		if rpcError != nil {
			t.Fatalf("tools/call returned JSON-RPC error %q (body=%s); want success result", *rpcError, body)
		}
		if id != 2 {
			t.Errorf("tools/call response id = %d; want 2 (body=%s)", id, body)
		}
		if resultIsError {
			t.Errorf("tools/call result isError = true (text=%q, body=%s); want isError false; pending dispatch error: %v", text, body, strings.Contains(string(body), "tool call dispatch pending"))
		}
		if !strings.Contains(text, "ok") {
			t.Errorf("tools/call content %q does not contain %q; pending dispatch error present: %v", text, "ok", strings.Contains(string(body), "tool call dispatch pending"))
		}
	})

	t.Run("InvalidArgument", func(t *testing.T) {
		response, body := mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"alpha__read","arguments":{"invalid_field":123}}}`)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("tools/call status = %d; want 200 (body=%s)", response.StatusCode, body)
		}
		id, resultIsError, text, rpcError := decodeEnvelope(t, body, "InvalidArgument tools/call")
		if !resultIsError {
			t.Errorf("tools/call with invalid arguments returned success result (id=%d, text=%q, body=%s); want isError true", id, text, body)
		}
		if rpcError != nil {
			t.Errorf("tools/call with invalid arguments returned JSON-RPC error %q (body=%s); want isError true result", *rpcError, body)
		}
	})

	t.Run("UnknownTool", func(t *testing.T) {
		response, body := mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"alpha__nonexistent","arguments":{}}}`)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("tools/call status = %d; want 200 (body=%s)", response.StatusCode, body)
		}
		id, resultIsError, text, rpcError := decodeEnvelope(t, body, "UnknownTool tools/call")
		if !resultIsError && rpcError == nil {
			t.Errorf("tools/call for unknown tool returned success result (id=%d, text=%q, body=%s); want error result or JSON-RPC error", id, text, body)
		}
	})
}

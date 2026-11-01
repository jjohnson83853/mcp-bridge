package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestServeHybridProgressiveRevealSessionAdmission(t *testing.T) {
	upstream := fakeUpstreamReadTool(t)
	ctx, httpURL, _, _ := startHybridBridgeWithUpstream(t, upstream)
	client := &http.Client{Timeout: 5 * time.Second}

	// An authenticated unknown ID is rehydrated, and forged exposure headers
	// are replaced before the request reaches the shared MCP server.
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, httpURL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":0,"method":"tools/list","params":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+serveClientToken)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Mcp-Session-Id", "not-yet-registered")
	request.Header["X-Mcp-Bridge-Exposure-Epoch"] = []string{"999", "1000"}
	request.Header["x-mcp-bridge-exposure-epoch"] = []string{"0"}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		t.Fatalf("read rehydrated tools/list response: %v", readErr)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("rehydrated-session tools/list status = %d, want 200 (body=%s)", response.StatusCode, body)
	}
	payload := body
	if dataStart := strings.Index(string(body), "data: "); dataStart >= 0 {
		payload = body[dataStart+len("data: "):]
		if end := strings.IndexByte(string(payload), '\n'); end >= 0 {
			payload = payload[:end]
		}
	}
	var rehydratedEnvelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(payload, &rehydratedEnvelope); err != nil {
		t.Fatalf("decode rehydrated tools/list response: %v (body=%s)", err, body)
	}
	if rehydratedEnvelope.JSONRPC != "2.0" || rehydratedEnvelope.ID != 0 ||
		len(rehydratedEnvelope.Result) == 0 ||
		(len(rehydratedEnvelope.Error) != 0 && string(rehydratedEnvelope.Error) != "null") {
		t.Fatalf("rehydrated tools/list envelope = %s, want client request ID 0 result", payload)
	}
	if response.Header.Get("Mcp-Session-Id") != "" {
		t.Fatalf("tools/list response replaced the client session ID with %q", response.Header.Get("Mcp-Session-Id"))
	}

	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"progressive-session-client","version":"1"}}}`
	sessions := make([]string, 0, 2)
	for index := range 2 {
		response, body := mcpSend(t, ctx, client, httpURL, "", initialize)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("session %d initialize status = %d, want 200 (body=%s)", index, response.StatusCode, body)
		}
		session := response.Header.Get("Mcp-Session-Id")
		if session == "" {
			t.Fatalf("session %d initialize omitted Mcp-Session-Id", index)
		}
		if len(sessions) != 0 && session == sessions[0] {
			t.Fatalf("two initialize POSTs reused SDK session ID %q", session)
		}
		sessions = append(sessions, session)

		response, body = mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
		if response.StatusCode != http.StatusAccepted {
			t.Fatalf("session %d initialized notification status = %d, want 202 (body=%s)", index, response.StatusCode, body)
		}

		// Each independent POST must pass receiving middleware using its own
		// admitted epoch and the SDK's actual session identity. The client also
		// supplies forged duplicate carrier values; inbound must overwrite them.
		listRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, httpURL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`))
		if err != nil {
			t.Fatal(err)
		}
		listRequest.Header.Set("Authorization", "Bearer "+serveClientToken)
		listRequest.Header.Set("Content-Type", "application/json")
		listRequest.Header.Set("Accept", "application/json, text/event-stream")
		listRequest.Header.Set("Mcp-Session-Id", session)
		listRequest.Header.Set("Mcp-Protocol-Version", "2025-11-25")
		listRequest.Header["X-Mcp-Bridge-Exposure-Epoch"] = []string{"999", "1000"}
		listRequest.Header["x-mcp-bridge-exposure-epoch"] = []string{"0"}
		listResponse, err := client.Do(listRequest)
		if err != nil {
			t.Fatal(err)
		}
		listBody, readErr := io.ReadAll(listResponse.Body)
		_ = listResponse.Body.Close()
		if readErr != nil {
			t.Fatalf("read session %d tools/list response: %v", index, readErr)
		}
		if listResponse.StatusCode != http.StatusOK {
			t.Fatalf("session %d tools/list status = %d, want 200 (body=%s)", index, listResponse.StatusCode, listBody)
		}
		listPayload := listBody
		if dataStart := strings.Index(string(listBody), "data: "); dataStart >= 0 {
			listPayload = listBody[dataStart+len("data: "):]
			if end := strings.IndexByte(string(listPayload), '\n'); end >= 0 {
				listPayload = listPayload[:end]
			}
		}
		var envelope struct {
			JSONRPC string          `json:"jsonrpc"`
			Result  json.RawMessage `json:"result"`
			Error   json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(listPayload, &envelope); err != nil {
			t.Fatalf("decode session %d tools/list response: %v (body=%s)", index, err, listBody)
		}
		if envelope.JSONRPC != "2.0" || len(envelope.Result) == 0 || (len(envelope.Error) != 0 && string(envelope.Error) != "null") {
			t.Fatalf("session %d tools/list envelope = %s, want a successful JSON-RPC result", index, listPayload)
		}
	}

	if sessions[0] == sessions[1] {
		t.Fatal("separate initialize POSTs did not create separate SDK sessions")
	}
}

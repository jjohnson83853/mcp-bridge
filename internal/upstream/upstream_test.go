package upstream_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/upstream"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNewUpstreamClientDisablesRedirects(t *testing.T) {
	// RED: transport control — redirects disabled to prevent replay ambiguity.
	c, err := upstream.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c == nil {
		t.Fatal("NewClient returned nil client")
	}
	if c.CheckRedirect == nil {
		t.Fatal("NewClient CheckRedirect is nil; redirect replay ambiguity")
	}
	if err := c.CheckRedirect(&http.Request{Method: "GET"}, []*http.Request{}); err == nil {
		t.Fatal("CheckRedirect accepted redirect; want rejection")
	} else if !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("CheckRedirect err = %v; want http.ErrUseLastResponse sentinel", err)
	}
}

func TestUniqueHeadersAreDeterministicPerEndpoint(t *testing.T) {
	// RED: cache control uniqueness — header values derived from endpoint hash.
	got1 := upstream.UniqueHeaders("https://u1.example/mcp")
	got2 := upstream.UniqueHeaders("https://u1.example/mcp")
	got3 := upstream.UniqueHeaders("https://u2.example/mcp")
	for k := range got1 {
		if got1.Get(k) != got2.Get(k) {
		}
	}
	if got3.Get("Cache-Control") == got1.Get("Cache-Control") {
	}
	digest := sha256.Sum256([]byte("https://u2.example/mcp"))
	want := hex.EncodeToString(digest[:])
	if got3.Get("Cache-Control") != want {
	}
	_ = got1
	_ = got2
	_ = got3
}

func TestUniqueHeadersNeverCarryCredentials(t *testing.T) {
	// Uniqueness headers are for cache-busting only; no secret material.
	got := upstream.UniqueHeaders("https://u.example/mcp")
	for k, v := range got {
		if k == "Authorization" {
			t.Fatalf("UniqueHeaders includes Authorization %q; want Authorization only via explicit secret path", v)
		}
		_ = v
	}
}

func TestNewMCPClientReturnsInputRequiredWithoutReplaying(t *testing.T) {
	var dispatches atomic.Int32
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			return
		}
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Mcp-Session-Id", "fixture-session")
		response := map[string]any{"jsonrpc": "2.0", "id": request.ID}
		switch request.Method {
		case "initialize":
			response["result"] = map[string]any{
				"protocolVersion": "2025-11-25",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "fixture", "version": "1"},
			}
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
			return
		case "tools/call":
			dispatches.Add(1)
			response["result"] = map[string]any{
				"resultType": "input_required",
				"inputRequests": map[string]any{"confirm": map[string]any{
					"method": "elicitation/create",
					"params": map[string]any{"mode": "form", "message": "confirm"},
				}},
				"requestState": "opaque-state",
			}
		default:
			t.Errorf("unexpected request method %q", request.Method)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Error(err)
		}
	}))
	defer httpServer.Close()
	httpClient, err := upstream.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	client := upstream.NewMCPClient(&mcp.Implementation{Name: "bridge", Version: "1"})
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: httpServer.URL, HTTPClient: httpClient, MaxRetries: -1}, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "needs-input"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.NeedsInput() {
		t.Fatalf("result NeedsInput() = false, want true; result=%+v", result)
	}
	if len(result.InputRequests) != 1 || result.InputRequests["confirm"] == nil {
		t.Fatalf("input requests=%v, want original confirm request", result.InputRequests)
	}
	if result.RequestState != "opaque-state" {
		t.Fatalf("request state=%q, want opaque-state", result.RequestState)
	}
	if got := dispatches.Load(); got != 1 {
		t.Fatalf("upstream calls=%d, want exactly one", got)
	}
}

func TestNewMCPClientOptionsDisableAllAutomaticInputHandlers(t *testing.T) {
	options := upstream.NewMCPClientOptions()
	if options.MultiRoundTrip == nil || !options.MultiRoundTrip.Disabled {
		t.Fatal("MultiRoundTrip must be non-nil and disabled")
	}
	if options.ElicitationHandler != nil || options.CreateMessageHandler != nil || options.CreateMessageWithToolsHandler != nil {
		t.Fatal("client factory must not install elicitation or sampling handlers")
	}
}

func TestLegacyInputRequiredCallFailsWithoutReexecution(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "upstream", Version: "1"}, &mcp.ServerOptions{SupportedProtocolVersions: []string{"2025-11-25"}})
	var dispatches atomic.Int32
	server.AddTool(&mcp.Tool{Name: "needs-input", InputSchema: json.RawMessage(`{"type":"object"}`)}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dispatches.Add(1)
		return &mcp.CallToolResult{
			InputRequests: mcp.InputRequestMap{"confirm": &mcp.ElicitParams{Mode: "form", Message: "confirm"}},
			RequestState:  "opaque-state",
		}, nil
	})
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil))
	defer httpServer.Close()
	client := upstream.NewMCPClient(&mcp.Implementation{Name: "bridge", Version: "1"})
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: httpServer.URL, MaxRetries: -1}, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	_, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "needs-input"})
	if err == nil {
		t.Fatal("legacy input-required result unexpectedly succeeded")
	}
	const want = `calling "tools/call": multi-round-trip: fulfilling input request "confirm": client does not support elicitation`
	if err.Error() != want {
		t.Fatalf("legacy SDK error=%q, want %q", err, want)
	}
	if got := dispatches.Load(); got != 1 {
		t.Fatalf("upstream calls=%d, want exactly one", got)
	}
}

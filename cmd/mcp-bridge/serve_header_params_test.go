package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	parameterHeaderProtocolVersion = "2026-07-28"
	parameterHeaderOwner           = "ticket-owner-42"
	parameterHeaderRepo            = "header-transit-repo"
	parameterHeaderTool            = "parameter_headers"
	parameterHeaderControlTool     = "body_control"
)

const parameterHeaderSchema = `{"type":"object","properties":{"owner":{"type":"string","x-mcp-header":"owner"},"repo":{"type":"string","x-mcp-header":"repo"}},"required":["owner","repo"]}`
const parameterHeaderControlSchema = `{"type":"object","properties":{"owner":{"type":"string"},"repo":{"type":"string"}},"required":["owner","repo"]}`

type parameterHeaderWireRequest struct {
	Method          string
	ProtocolVersion string
	SessionID       string
	Owner           string
	Repo            string
	Body            string
}

type parameterHeaderPeerRecorder struct {
	mu       sync.Mutex
	requests []parameterHeaderWireRequest
}

func (r *parameterHeaderPeerRecorder) handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body []byte
		if request.Body != nil {
			var err error
			body, err = io.ReadAll(request.Body)
			if err != nil {
				http.Error(writer, "read JSON-RPC request", http.StatusBadRequest)
				return
			}
		}
		var envelope struct {
			Method string `json:"method"`
		}
		if len(body) > 0 {
			if err := json.Unmarshal(body, &envelope); err != nil {
				http.Error(writer, "decode JSON-RPC request", http.StatusBadRequest)
				return
			}
		}

		r.mu.Lock()
		r.requests = append(r.requests, parameterHeaderWireRequest{
			Method:          envelope.Method,
			ProtocolVersion: request.Header.Get("Mcp-Protocol-Version"),
			SessionID:       request.Header.Get("Mcp-Session-Id"),
			Owner:           request.Header.Get("Mcp-Param-owner"),
			Repo:            request.Header.Get("Mcp-Param-repo"),
			Body:            string(body),
		})
		r.mu.Unlock()

		if request.Body != nil {
			request.Body = io.NopCloser(bytes.NewReader(body))
		}
		next.ServeHTTP(writer, request)
	})
}

func (r *parameterHeaderPeerRecorder) snapshot() []parameterHeaderWireRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]parameterHeaderWireRequest(nil), r.requests...)
}

func newParameterHeaderPeer(t *testing.T) (*httptest.Server, *parameterHeaderPeerRecorder) {
	return newParameterHeaderPeerWithVersions(t, []string{parameterHeaderProtocolVersion})
}

func newParameterHeaderPeerWithVersions(t *testing.T, versions []string) (*httptest.Server, *parameterHeaderPeerRecorder) {
	t.Helper()
	serverOptions := &mcp.ServerOptions{SupportedProtocolVersions: versions}
	serverOptions.SetCacheable = func(_ context.Context, request mcp.Request, cacheable *mcp.Cacheable) {
		if _, ok := request.GetParams().(*mcp.ListToolsParams); ok {
			cacheable.TTLMs = 60_000
		}
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "parameter-header-peer", Version: "1"}, serverOptions)
	echoArguments := func(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{
			&mcp.TextContent{Text: string(request.Params.Arguments)},
		}}, nil
	}
	server.AddTool(&mcp.Tool{
		Name:        parameterHeaderTool,
		Description: "Echo owner and repo through parameter header transit",
		InputSchema: json.RawMessage(parameterHeaderSchema),
	}, echoArguments)
	server.AddTool(&mcp.Tool{
		Name:        parameterHeaderControlTool,
		Description: "Echo owner and repo through a body-only control",
		InputSchema: json.RawMessage(parameterHeaderControlSchema),
	}, echoArguments)

	recorder := &parameterHeaderPeerRecorder{}
	inner := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true},
	)
	peer := httptest.NewServer(recorder.handler(inner))
	t.Cleanup(peer.Close)
	return peer, recorder
}

func callParameterHeaderTool(t *testing.T, ctx context.Context, session *mcp.ClientSession, rawName string, arguments map[string]any) *mcp.CallToolResult {
	t.Helper()
	return callTool(t, ctx, session, catalog.ToolCallTool, map[string]any{
		"tool_name": "alpha__" + rawName,
		"arguments": arguments,
	})
}

func parameterHeaderRequestForTool(t *testing.T, requests []parameterHeaderWireRequest, rawName string) parameterHeaderWireRequest {
	t.Helper()
	for _, request := range requests {
		if request.Method != "tools/call" {
			continue
		}
		var envelope struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		if err := json.Unmarshal([]byte(request.Body), &envelope); err != nil {
			t.Fatalf("decode %s request body %q: %v", rawName, request.Body, err)
		}
		if envelope.Params.Name == rawName {
			return request
		}
	}
	t.Fatalf("no tools/call request for %q in %+v", rawName, requests)
	return parameterHeaderWireRequest{}
}

func countParameterHeaderMethod(requests []parameterHeaderWireRequest, method string) int {
	count := 0
	for _, request := range requests {
		if request.Method == method {
			count++
		}
	}
	return count
}

func TestParameterHeaderTransitWireFacts(t *testing.T) {
	peer, recorder := newParameterHeaderPeer(t)
	ctx, session, _ := startUpstreamRestartBridge(t, peer.URL, false)

	headerResult := callParameterHeaderTool(t, ctx, session, parameterHeaderTool, map[string]any{
		"owner": parameterHeaderOwner,
		"repo":  parameterHeaderRepo,
	})
	controlResult := callParameterHeaderTool(t, ctx, session, parameterHeaderControlTool, map[string]any{
		"owner": parameterHeaderOwner,
		"repo":  parameterHeaderRepo,
	})

	requests := recorder.snapshot()
	headerRequest := parameterHeaderRequestForTool(t, requests, parameterHeaderTool)
	controlRequest := parameterHeaderRequestForTool(t, requests, parameterHeaderControlTool)
	t.Logf("mode=default-stateless method=%q protocol=%q owner=%q repo=%q body=%s control_is_error=%t",
		headerRequest.Method, headerRequest.ProtocolVersion, headerRequest.Owner, headerRequest.Repo, headerRequest.Body, controlResult.IsError)

	if headerRequest.ProtocolVersion != parameterHeaderProtocolVersion {
		t.Errorf("parameter-header call protocol version = %q; want %q", headerRequest.ProtocolVersion, parameterHeaderProtocolVersion)
	}
	if headerResult.IsError {
		t.Errorf("parameter-header call returned isError=true; missing parameter headers: owner=%q repo=%q body=%s", headerRequest.Owner, headerRequest.Repo, headerRequest.Body)
	}
	if headerRequest.Owner != parameterHeaderOwner {
		t.Errorf("Mcp-Param-owner = %q; want %q", headerRequest.Owner, parameterHeaderOwner)
	}
	if headerRequest.Repo != parameterHeaderRepo {
		t.Errorf("Mcp-Param-repo = %q; want %q", headerRequest.Repo, parameterHeaderRepo)
	}
	for _, argument := range []string{`"owner":"` + parameterHeaderOwner + `"`, `"repo":"` + parameterHeaderRepo + `"`} {
		if !strings.Contains(headerRequest.Body, argument) {
			t.Errorf("parameter-header call body %s does not contain %s", headerRequest.Body, argument)
		}
	}
	if controlResult.IsError {
		t.Errorf("body-only control call returned isError=true: %s", controlRequest.Body)
	}
}

func TestParameterHeaderStatefulControlIsAchievable(t *testing.T) {
	peer, recorder := newParameterHeaderPeer(t)
	ctx, session, _ := startUpstreamRestartBridge(t, peer.URL, true)

	result := callParameterHeaderTool(t, ctx, session, parameterHeaderTool, map[string]any{
		"owner": parameterHeaderOwner,
		"repo":  parameterHeaderRepo,
	})
	requests := recorder.snapshot()
	headerRequest := parameterHeaderRequestForTool(t, requests, parameterHeaderTool)
	discoverRequests := countParameterHeaderMethod(requests, "server/discover")
	initializeRequests := countParameterHeaderMethod(requests, "initialize")
	listRequests := countParameterHeaderMethod(requests, "tools/list")
	t.Logf("mode=stateful-control method=%q protocol=%q owner=%q repo=%q body=%s discover=%d initialize=%d tools_list=%d",
		headerRequest.Method, headerRequest.ProtocolVersion, headerRequest.Owner, headerRequest.Repo, headerRequest.Body,
		discoverRequests, initializeRequests, listRequests)

	if result.IsError {
		t.Errorf("stateful parameter-header call returned isError=true: %s", headerRequest.Body)
	}
	if headerRequest.ProtocolVersion != parameterHeaderProtocolVersion {
		t.Errorf("parameter-header call protocol version = %q; want %q", headerRequest.ProtocolVersion, parameterHeaderProtocolVersion)
	}
	if headerRequest.Owner != parameterHeaderOwner {
		t.Errorf("Mcp-Param-owner = %q; want %q", headerRequest.Owner, parameterHeaderOwner)
	}
	if headerRequest.Repo != parameterHeaderRepo {
		t.Errorf("Mcp-Param-repo = %q; want %q", headerRequest.Repo, parameterHeaderRepo)
	}
	for _, argument := range []string{`"owner":"` + parameterHeaderOwner + `"`, `"repo":"` + parameterHeaderRepo + `"`} {
		if !strings.Contains(headerRequest.Body, argument) {
			t.Errorf("parameter-header call body %s does not contain %s", headerRequest.Body, argument)
		}
	}
	if discoverRequests != 1 {
		t.Errorf("server/discover requests = %d; want 1 to reuse the discovery session", discoverRequests)
	}
	if initializeRequests != 0 {
		t.Errorf("initialize requests = %d; want 0 for the negotiated modern protocol", initializeRequests)
	}
	if listRequests != 1 {
		t.Errorf("tools/list requests = %d; want 1 from the reused discovery session", listRequests)
	}
}

func TestParameterHeaderRestartSurvivesInStatelessMode(t *testing.T) {
	recorder := &parameterHeaderPeerRecorder{}
	newGenerationHandler := func(generation string) http.Handler {
		server := mcp.NewServer(&mcp.Implementation{Name: "parameter-header-peer", Version: "1"}, &mcp.ServerOptions{
			SupportedProtocolVersions: []string{parameterHeaderProtocolVersion},
		})
		server.AddTool(&mcp.Tool{
			Name:        parameterHeaderTool,
			Description: "Echo owner and repo through parameter header transit",
			InputSchema: json.RawMessage(parameterHeaderSchema),
		}, func(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{StructuredContent: map[string]any{
				"generation": generation,
				"arguments":  string(request.Params.Arguments),
			}}, nil
		})
		inner := mcp.NewStreamableHTTPHandler(
			func(*http.Request) *mcp.Server { return server },
			&mcp.StreamableHTTPOptions{Stateless: true},
		)
		return recorder.handler(inner)
	}

	activeHandler := newGenerationHandler("before")
	var handlerMu sync.RWMutex
	peer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		handlerMu.RLock()
		handler := activeHandler
		handlerMu.RUnlock()
		handler.ServeHTTP(writer, request)
	}))
	t.Cleanup(peer.Close)

	ctx, session, _ := startUpstreamRestartBridge(t, peer.URL, false)
	afterHandler := newGenerationHandler("after")
	handlerMu.Lock()
	activeHandler = afterHandler
	handlerMu.Unlock()

	result := callParameterHeaderTool(t, ctx, session, parameterHeaderTool, map[string]any{
		"owner": parameterHeaderOwner,
		"repo":  parameterHeaderRepo,
	})
	if result.IsError || !hasGeneration(structuredContent(t, result), "after") {
		t.Fatalf("post-restart tools/call result = %v; want a successful new-generation echo", structuredContent(t, result))
	}
	requests := recorder.snapshot()
	if got := countParameterHeaderMethod(requests, "tools/call"); got != 1 {
		t.Fatalf("post-restart tools/call requests = %d; want exactly 1", got)
	}
	call := parameterHeaderRequestForTool(t, requests, parameterHeaderTool)
	if call.Owner != parameterHeaderOwner || call.Repo != parameterHeaderRepo {
		t.Errorf("post-restart parameter headers owner=%q repo=%q; want %q/%q", call.Owner, call.Repo, parameterHeaderOwner, parameterHeaderRepo)
	}
}

func startHeaderBridgeCaptured(t *testing.T, peerURL string) (context.Context, *mcp.ClientSession, *syncWriter) {
	t.Helper()
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)

	httpAddress := freeAddress(t)
	operationsAddress := freeAddress(t)
	embeddings := fakeEmbeddings(t)
	directory := t.TempDir()
	configPath := filepath.Join(directory, "bridge.yaml")
	writeBridgeConfig(t, configPath, httpAddress, operationsAddress, filepath.Join(directory, "state"), embeddings.URL,
		[]bridgeServerSpec{{id: "alpha", endpoint: peerURL}})

	ctx, cancel := context.WithCancel(context.Background())
	stdout, stderr, combined := &syncWriter{}, &syncWriter{}, &syncWriter{}
	done := make(chan int, 1)
	go func() {
		done <- runContext(ctx, []string{"--config", configPath},
			io.MultiWriter(stdout, combined), io.MultiWriter(stderr, combined))
	}()
	var session *mcp.ClientSession
	t.Cleanup(func() {
		if session != nil {
			if err := session.Close(); err != nil {
				t.Errorf("close bridge client: %v", err)
			}
		}
		cancel()
		select {
		case code := <-done:
			if code != 0 {
				t.Errorf("bridge exited with status %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		case <-time.After(10 * time.Second):
			t.Errorf("bridge did not stop after cancellation")
		}
	})

	operationsURL := "http://" + operationsAddress
	waitFor(t, "ready bridge", func() bool {
		status, body, ok := probeBody(t, operationsURL+"/readyz")
		return ok && status == http.StatusOK && strings.Contains(body, `"status":"ready"`) && strings.Contains(body, "alpha")
	})
	session = connectBridge(t, ctx, "http://"+httpAddress+"/mcp", serveClientToken)
	return ctx, session, combined
}

func TestLegacyParamHeaderCallFailsWithNamedCode(t *testing.T) {
	peer, recorder := newParameterHeaderPeerWithVersions(t, []string{"2025-11-25"})
	ctx, session, combinedOutput := startHeaderBridgeCaptured(t, peer.URL)

	afterDiscovery := recorder.snapshot()
	negotiatedVersion := ""
	for _, request := range afterDiscovery {
		if request.Method == "tools/list" {
			negotiatedVersion = request.ProtocolVersion
			break
		}
	}
	if negotiatedVersion != "2025-11-25" {
		t.Fatalf("observed negotiated protocol version = %q; want 2025-11-25", negotiatedVersion)
	}

	result := callParameterHeaderTool(t, ctx, session, parameterHeaderTool, map[string]any{
		"owner": parameterHeaderOwner,
		"repo":  parameterHeaderRepo,
	})
	if !result.IsError {
		t.Fatalf("legacy parameter-header call returned isError=false; want named protocol rejection: %v", structuredContent(t, result))
	}
	payload := structuredContent(t, result)
	if payload["code"] != "parameter_header_unsupported" || payload["execution_state"] != "not_started" {
		t.Fatalf("legacy failure payload = %v; want parameter_header_unsupported/not_started", payload)
	}
	if retryable, _ := payload["retryable"].(bool); retryable {
		t.Errorf("legacy failure retryable = true; want false")
	}
	message, _ := payload["message"].(string)
	for _, forbidden := range []string{parameterHeaderOwner, parameterHeaderRepo, parameterHeaderTool, peer.URL, serveClientToken, serveEmbeddingKey} {
		if strings.Contains(message, forbidden) {
			t.Errorf("legacy failure message contains forbidden value %q: %q", forbidden, message)
		}
	}

	requests := recorder.snapshot()
	if got := countParameterHeaderMethod(requests, "tools/list") - countParameterHeaderMethod(afterDiscovery, "tools/list"); got != 0 {
		t.Errorf("post-discovery tools/list delta = %d; want 0", got)
	}
	if got := countParameterHeaderMethod(requests, "tools/call") - countParameterHeaderMethod(afterDiscovery, "tools/call"); got != 0 {
		t.Errorf("post-discovery tools/call delta = %d; want 0", got)
	}

	backendErrorCode := ""
	for _, line := range strings.Split(combinedOutput.String(), "\n") {
		var record struct {
			Event string `json:"event"`
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &record); err == nil && record.Event == "backend_complete" {
			backendErrorCode = record.Error.Code
			break
		}
	}
	if backendErrorCode != "parameter_header_unsupported" {
		t.Fatalf("captured combined backend_complete error.code = %q; want parameter_header_unsupported (output=%q)", backendErrorCode, combinedOutput.String())
	}
}

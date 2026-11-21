package upstream_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/upstream"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const headerParamsProtocolVersion = "2026-07-28"

const headerParamsToolSchema = `{"type":"object","properties":{"owner":{"type":"string","x-mcp-header":"owner"},"repo":{"type":"string","x-mcp-header":"repo"}},"required":["owner","repo"]}`
const headerParamsControlSchema = `{"type":"object","properties":{"owner":{"type":"string"},"repo":{"type":"string"}},"required":["owner","repo"]}`

type headerParamsRequest struct {
	Method          string
	ProtocolVersion string
	Owner           string
	Repo            string
	Body            string
}

type headerParamsRecorder struct {
	mu       sync.Mutex
	requests []headerParamsRequest
	failList atomic.Bool
}

func (r *headerParamsRecorder) handler(next http.Handler) http.Handler {
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
		r.requests = append(r.requests, headerParamsRequest{
			Method:          envelope.Method,
			ProtocolVersion: request.Header.Get("Mcp-Protocol-Version"),
			Owner:           request.Header.Get("Mcp-Param-owner"),
			Repo:            request.Header.Get("Mcp-Param-repo"),
			Body:            string(body),
		})
		r.mu.Unlock()

		if request.Body != nil {
			request.Body = io.NopCloser(bytes.NewReader(body))
		}
		if envelope.Method == "tools/list" && r.failList.Load() {
			http.Error(writer, "tool list unavailable", http.StatusServiceUnavailable)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (r *headerParamsRecorder) snapshot() []headerParamsRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]headerParamsRequest(nil), r.requests...)
}

func newHeaderParamsPeer(t *testing.T) (*httptest.Server, *headerParamsRecorder) {
	return newHeaderParamsPeerWithTools(
		t,
		&mcp.ServerOptions{SupportedProtocolVersions: []string{headerParamsProtocolVersion}},
		&mcp.StreamableHTTPOptions{Stateless: true},
		&mcp.Tool{
			Name:        "parameter_headers",
			Description: "Echo owner and repo through parameter header transit",
			InputSchema: json.RawMessage(headerParamsToolSchema),
		},
		&mcp.Tool{
			Name:        "body_control",
			Description: "Echo owner and repo through a body-only control",
			InputSchema: json.RawMessage(headerParamsControlSchema),
		},
	)
}

func newHeaderParamsPeerWithTools(
	t *testing.T,
	serverOptions *mcp.ServerOptions,
	streamOptions *mcp.StreamableHTTPOptions,
	tools ...*mcp.Tool,
) (*httptest.Server, *headerParamsRecorder) {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "header-params-peer", Version: "1"}, serverOptions)
	echoArguments := func(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{
			&mcp.TextContent{Text: string(request.Params.Arguments)},
		}}, nil
	}
	for _, tool := range tools {
		server.AddTool(tool, echoArguments)
	}
	recorder := &headerParamsRecorder{}
	inner := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, streamOptions)
	peer := httptest.NewServer(recorder.handler(inner))
	t.Cleanup(peer.Close)
	return peer, recorder
}

func headerParamsServerOptionsWithListTTL(ttlMillis int) *mcp.ServerOptions {
	options := &mcp.ServerOptions{SupportedProtocolVersions: []string{headerParamsProtocolVersion}}
	options.SetCacheable = func(_ context.Context, request mcp.Request, cacheable *mcp.Cacheable) {
		if _, ok := request.GetParams().(*mcp.ListToolsParams); ok {
			cacheable.TTLMs = ttlMillis
		}
	}
	return options
}

func countHeaderParamsMethod(requests []headerParamsRequest, method string) int {
	count := 0
	for _, request := range requests {
		if request.Method == method {
			count++
		}
	}
	return count
}

func headerParamsCallForTool(t *testing.T, requests []headerParamsRequest, rawName string) headerParamsRequest {
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
			t.Fatalf("decode tools/call request body %q: %v", request.Body, err)
		}
		if envelope.Params.Name == rawName {
			return request
		}
	}
	t.Fatalf("no tools/call request for %q in %+v", rawName, requests)
	return headerParamsRequest{}
}

func TestDedicatedParamHeadersReachPeerInStatelessMode(t *testing.T) {
	for _, test := range []struct {
		name          string
		pageSize      int
		wantListDelta int
	}{
		{name: "single-page", wantListDelta: 1},
		{name: "PageSize:1", pageSize: 1, wantListDelta: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			peer, recorder := newHeaderParamsPeerWithTools(
				t,
				&mcp.ServerOptions{SupportedProtocolVersions: []string{headerParamsProtocolVersion}, PageSize: test.pageSize},
				&mcp.StreamableHTTPOptions{Stateless: true},
				&mcp.Tool{Name: "aaa_control_tool", InputSchema: json.RawMessage(headerParamsControlSchema)},
				&mcp.Tool{Name: "mmm_body_tool", InputSchema: json.RawMessage(headerParamsControlSchema)},
				&mcp.Tool{Name: "zzz_header_tool", InputSchema: json.RawMessage(headerParamsToolSchema)},
			)
			dedicated := newDedicatedWithStateless(t, peer.URL, true)
			if _, err := dedicated.Discover(context.Background()); err != nil {
				t.Fatalf("Discover: %v", err)
			}
			afterDiscovery := recorder.snapshot()

			args := json.RawMessage(`{"owner":"stateless-owner","repo":"stateless-repo"}`)
			resultBytes, outcome := dedicated.CallTool(context.Background(), "alpha__zzz_header_tool", "alpha", "zzz_header_tool", args)
			if outcome != nil {
				t.Fatalf("CallTool outcome = %+v; want success", outcome)
			}
			var result struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
				IsError bool `json:"isError"`
			}
			if err := json.Unmarshal(resultBytes, &result); err != nil {
				t.Fatalf("decode CallTool result: %v", err)
			}
			if result.IsError || len(result.Content) != 1 || result.Content[0].Text != string(args) {
				t.Fatalf("CallTool content = %+v; want exact echoed arguments %s", result, args)
			}

			requests := recorder.snapshot()
			if got := countHeaderParamsMethod(requests, "tools/call"); got != 1 {
				t.Fatalf("tools/call requests = %d; want exactly 1", got)
			}
			if got := countHeaderParamsMethod(requests, "tools/list") - countHeaderParamsMethod(afterDiscovery, "tools/list"); got != test.wantListDelta {
				t.Errorf("tools/list request delta = %d; want %d", got, test.wantListDelta)
			}
			call := headerParamsCallForTool(t, requests, "zzz_header_tool")
			if call.Owner != "stateless-owner" || call.Repo != "stateless-repo" {
				t.Errorf("parameter headers owner=%q repo=%q; want stateless-owner/stateless-repo", call.Owner, call.Repo)
			}
			if !strings.Contains(call.Body, string(args)) {
				t.Errorf("tools/call body %s does not preserve exact arguments %s", call.Body, args)
			}
		})
	}
}

func TestBodyOnlyControlAddsNoExtraListCall(t *testing.T) {
	peer, recorder := newHeaderParamsPeer(t)
	dedicated := newDedicatedWithStateless(t, peer.URL, true)
	if _, err := dedicated.Discover(context.Background()); err != nil {
		t.Fatalf("Discover: %v", err)
	}
	afterDiscovery := recorder.snapshot()
	args := json.RawMessage(`{"owner":"body-owner","repo":"body-repo"}`)

	resultBytes, outcome := dedicated.CallTool(context.Background(), "alpha__body_control", "alpha", "body_control", args)
	if outcome != nil {
		t.Fatalf("CallTool outcome = %+v; want success", outcome)
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("decode CallTool result: %v", err)
	}
	if result.IsError || len(result.Content) != 1 || result.Content[0].Text != string(args) {
		t.Fatalf("CallTool content = %+v; want exact echoed arguments %s", result, args)
	}

	requests := recorder.snapshot()
	if got := countHeaderParamsMethod(requests, "tools/list") - countHeaderParamsMethod(afterDiscovery, "tools/list"); got != 0 {
		t.Errorf("body-only call tools/list delta = %d; want 0", got)
	}
	call := headerParamsCallForTool(t, requests, "body_control")
	if call.Owner != "" || call.Repo != "" {
		t.Errorf("body-only call carried parameter headers owner=%q repo=%q; want none", call.Owner, call.Repo)
	}
	if !strings.Contains(call.Body, string(args)) {
		t.Errorf("tools/call body %s does not preserve exact arguments %s", call.Body, args)
	}
}

func TestStatefulParamHeadersReuseDiscoveryCache(t *testing.T) {
	t.Run("positive-ttl", func(t *testing.T) {
		const ttlMillis = 60_000
		started := time.Now()
		peer, recorder := newHeaderParamsPeerWithTools(
			t,
			headerParamsServerOptionsWithListTTL(ttlMillis),
			&mcp.StreamableHTTPOptions{Stateless: true},
			&mcp.Tool{Name: "parameter_headers", InputSchema: json.RawMessage(headerParamsToolSchema)},
		)
		dedicated := newDedicatedWithStateless(t, peer.URL, false)
		if _, err := dedicated.Discover(context.Background()); err != nil {
			t.Fatalf("Discover: %v", err)
		}
		afterDiscovery := recorder.snapshot()
		args := json.RawMessage(`{"owner":"cached-owner","repo":"cached-repo"}`)
		resultBytes, outcome := dedicated.CallTool(context.Background(), "alpha__parameter_headers", "alpha", "parameter_headers", args)
		if outcome != nil {
			t.Fatalf("CallTool outcome = %+v; want success", outcome)
		}
		var result struct {
			IsError bool `json:"isError"`
		}
		if err := json.Unmarshal(resultBytes, &result); err != nil {
			t.Fatalf("decode CallTool result: %v", err)
		}
		if result.IsError {
			t.Fatal("stateful parameter-header call returned isError=true")
		}
		requests := recorder.snapshot()
		if got := countHeaderParamsMethod(requests, "tools/list") - countHeaderParamsMethod(afterDiscovery, "tools/list"); got != 0 {
			t.Errorf("stateful tools/list delta = %d; want 0", got)
		}
		if got := countHeaderParamsMethod(requests, "tools/call"); got != 1 {
			t.Errorf("stateful tools/call requests = %d; want exactly 1", got)
		}
		if got := countHeaderParamsMethod(requests, "server/discover"); got != 1 {
			t.Errorf("stateful server/discover requests = %d; want exactly 1", got)
		}
		if got := countHeaderParamsMethod(requests, "initialize"); got != 0 {
			t.Errorf("stateful initialize requests = %d; want 0 for the modern protocol", got)
		}
		call := headerParamsCallForTool(t, requests, "parameter_headers")
		if call.Owner != "cached-owner" || call.Repo != "cached-repo" {
			t.Errorf("stateful parameter headers owner=%q repo=%q; want cached-owner/cached-repo", call.Owner, call.Repo)
		}
		if !strings.Contains(call.Body, string(args)) {
			t.Errorf("stateful tools/call body %s does not preserve exact arguments %s", call.Body, args)
		}
		if elapsed := time.Since(started); elapsed >= time.Duration(ttlMillis)*time.Millisecond {
			t.Fatalf("positive tools/list TTL %d ms did not exceed test runtime %s", ttlMillis, elapsed)
		}
	})

	t.Run("zero-ttl", func(t *testing.T) {
		peer, recorder := newHeaderParamsPeerWithTools(
			t,
			headerParamsServerOptionsWithListTTL(0),
			&mcp.StreamableHTTPOptions{Stateless: true},
			&mcp.Tool{Name: "parameter_headers", InputSchema: json.RawMessage(headerParamsToolSchema)},
		)
		dedicated := newDedicatedWithStateless(t, peer.URL, false)
		if _, err := dedicated.Discover(context.Background()); err != nil {
			t.Fatalf("Discover: %v", err)
		}
		afterDiscovery := recorder.snapshot()
		args := json.RawMessage(`{"owner":"fresh-owner","repo":"fresh-repo"}`)
		resultBytes, outcome := dedicated.CallTool(context.Background(), "alpha__parameter_headers", "alpha", "parameter_headers", args)
		if outcome != nil {
			t.Fatalf("CallTool outcome = %+v; want success", outcome)
		}
		var result struct {
			IsError bool `json:"isError"`
		}
		if err := json.Unmarshal(resultBytes, &result); err != nil {
			t.Fatalf("decode CallTool result: %v", err)
		}
		if result.IsError {
			t.Fatal("zero-ttl parameter-header call returned isError=true")
		}
		requests := recorder.snapshot()
		if got := countHeaderParamsMethod(requests, "server/discover"); got != 1 {
			t.Errorf("zero-ttl server/discover requests = %d; want exactly 1", got)
		}
		if got := countHeaderParamsMethod(requests, "initialize"); got != 0 {
			t.Errorf("zero-ttl initialize requests = %d; want 0 for the modern protocol", got)
		}
		if got := countHeaderParamsMethod(requests, "tools/list") - countHeaderParamsMethod(afterDiscovery, "tools/list"); got < 1 {
			t.Errorf("zero-ttl tools/list delta = %d; want refetch", got)
		}
		if got := countHeaderParamsMethod(requests, "tools/call"); got != 1 {
			t.Errorf("zero-ttl tools/call requests = %d; want exactly 1", got)
		}
		call := headerParamsCallForTool(t, requests, "parameter_headers")
		if call.Owner != "fresh-owner" || call.Repo != "fresh-repo" {
			t.Errorf("zero-ttl parameter headers owner=%q repo=%q; want fresh-owner/fresh-repo", call.Owner, call.Repo)
		}
		if !strings.Contains(call.Body, string(args)) {
			t.Errorf("zero-ttl tools/call body %s does not preserve exact arguments %s", call.Body, args)
		}
	})
}


func TestParamHeaderFailurePathsUseSafeCodes(t *testing.T) {
	t.Run("list failure", func(t *testing.T) {
		peer, recorder := newHeaderParamsPeer(t)
		dedicated, err := upstream.NewDedicated(upstream.DedicatedOptions{
			Stateless: true,
			Logger:    observe.NewLogger(io.Discard, io.Discard),
			Server: upstream.ServerOptions{
				ID:                  "alpha",
				Endpoint:            peer.URL,
				BreakerThreshold:    1,
				CallTimeout:         5 * time.Second,
				ConnectTimeout:      5 * time.Second,
			},
		})
		if err != nil {
			t.Fatalf("NewDedicated: %v", err)
		}
		t.Cleanup(func() { _ = dedicated.Close() })
		if _, err := dedicated.Discover(context.Background()); err != nil {
			t.Fatalf("Discover: %v", err)
		}
		afterDiscovery := recorder.snapshot()
		recorder.failList.Store(true)

		args := json.RawMessage(`{"owner":"private-owner","repo":"private-repo"}`)
		resultBytes, outcome := dedicated.CallTool(context.Background(), "alpha__parameter_headers", "alpha", "parameter_headers", args)
		if resultBytes != nil || outcome == nil || outcome.Failure == nil {
			t.Fatalf("CallTool = (%s, %+v); want a list failure outcome", resultBytes, outcome)
		}
		if outcome.Failure.Code != "backend_unavailable" || outcome.Failure.ExecutionState != "not_started" {
			t.Errorf("failure = %+v; want backend_unavailable/not_started", outcome.Failure)
		}
		requests := recorder.snapshot()
		if got := countHeaderParamsMethod(requests, "tools/call") - countHeaderParamsMethod(afterDiscovery, "tools/call"); got != 0 {
			t.Errorf("tools/call delta after list failure = %d; want 0", got)
		}
		if got := countHeaderParamsMethod(requests, "tools/list") - countHeaderParamsMethod(afterDiscovery, "tools/list"); got != 1 {
			t.Errorf("tools/list delta after list failure = %d; want exactly 1", got)
		}
		release, admitErr := dedicated.Client().AdmitCall(context.Background())
		if admitErr != nil {
			t.Errorf("tool admission after list preparation error = %v; want breaker still closed", admitErr)
		} else {
			release()
		}
		safeOutput, err := json.Marshal(outcome)
		if err != nil {
			t.Fatalf("marshal outcome: %v", err)
		}
		for _, forbidden := range []string{string(args), "private-owner", "private-repo", "parameter_headers", peer.URL, "private-token"} {
			if strings.Contains(string(safeOutput), forbidden) {
				t.Errorf("failure outcome contains forbidden value %q: %s", forbidden, safeOutput)
			}
		}
	})

	t.Run("unsupported version observability", func(t *testing.T) {
		info := observe.RuntimeErrorInfo("parameter_header_unsupported")
		if info == nil || info.Code != "parameter_header_unsupported" || info.Kind != "configuration" || info.Retryable || info.Remediation != "correct the server configuration or credentials" {
			t.Fatalf("RuntimeErrorInfo = %+v; want configuration, nonretryable, and fixed remediation", info)
		}
		for _, forbidden := range []string{"private-owner", "private-repo", "parameter_headers", "https://private.invalid", "private-token"} {
			if strings.Contains(info.Remediation, forbidden) {
				t.Errorf("runtime error remediation contains forbidden value %q: %q", forbidden, info.Remediation)
			}
		}
	})
}

func TestDedicatedReusesDiscoveredParameterHeaderMetadata(t *testing.T) {
	peer, recorder := newHeaderParamsPeer(t)
	dedicated, err := upstream.NewDedicated(upstream.DedicatedOptions{
		Server: upstream.ServerOptions{
			ID:             "alpha",
			Endpoint:       peer.URL,
			CallTimeout:    5 * time.Second,
			ConnectTimeout: 5 * time.Second,
		},
		Logger: observe.NewLogger(io.Discard, io.Discard),
	})
	if err != nil {
		t.Fatalf("NewDedicated: %v", err)
	}
	t.Cleanup(func() {
		if err := dedicated.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	definitions, err := dedicated.Discover(ctx)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	found := false
	for _, definition := range definitions {
		if definition.RawName == "parameter_headers" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Discover omitted parameter_headers")
	}

	arguments := json.RawMessage(`{"owner":"stateful-owner","repo":"stateful-repo"}`)
	resultBytes, outcome := dedicated.CallTool(ctx, "alpha__parameter_headers", "alpha", "parameter_headers", arguments)
	if outcome != nil {
		t.Fatalf("CallTool outcome = %+v; want success", outcome)
	}
	var result mcp.CallToolResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("decode CallTool result: %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool returned isError=true: %+v", result)
	}

	requests := recorder.snapshot()
	var call *headerParamsRequest
	for index := range requests {
		request := &requests[index]
		if request.Method != "tools/call" {
			continue
		}
		var envelope struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		if err := json.Unmarshal([]byte(request.Body), &envelope); err != nil {
			t.Fatalf("decode request body %q: %v", request.Body, err)
		}
		if envelope.Params.Name == "parameter_headers" {
			call = request
			break
		}
	}
	if call == nil {
		t.Fatalf("no parameter_headers tools/call request in %+v", requests)
	}
	if call.ProtocolVersion != headerParamsProtocolVersion {
		t.Errorf("Mcp-Protocol-Version = %q; want %q", call.ProtocolVersion, headerParamsProtocolVersion)
	}
	if call.Owner != "stateful-owner" {
		t.Errorf("Mcp-Param-owner = %q; want stateful-owner", call.Owner)
	}
	if call.Repo != "stateful-repo" {
		t.Errorf("Mcp-Param-repo = %q; want stateful-repo", call.Repo)
	}
	for _, argument := range []string{`"owner":"stateful-owner"`, `"repo":"stateful-repo"`} {
		if !strings.Contains(call.Body, argument) {
			t.Errorf("tools/call body %s does not contain %s", call.Body, argument)
		}
	}
}

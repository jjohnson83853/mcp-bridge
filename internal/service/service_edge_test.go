package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/search"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

func TestLoopValidatesRequiredCollaborators(t *testing.T) {
	loader := func() (*config.Config, error) { return nil, nil }
	deps := RefreshDeps{ReloadDeps: ReloadDeps{
		Discover:  func(config.Server) ([]core.ToolDefinition, error) { return nil, nil },
		NewClient: func(config.Server) (core.Upstream, error) { return nil, nil },
		Now:       time.Now,
	}}
	cases := []struct {
		name string
		rt   *Runtime
		opts LoopOptions
	}{
		{"nil runtime", nil, LoopOptions{Load: loader, Deps: deps}},
		{"missing loader", NewRuntime(), LoopOptions{Deps: deps}},
		{"missing discovery", NewRuntime(), LoopOptions{Load: loader, Deps: RefreshDeps{ReloadDeps: ReloadDeps{NewClient: deps.NewClient, Now: time.Now}}}},
		{"missing client", NewRuntime(), LoopOptions{Load: loader, Deps: RefreshDeps{ReloadDeps: ReloadDeps{Discover: deps.Discover, Now: time.Now}}}},
		{"missing clock", NewRuntime(), LoopOptions{Load: loader, Deps: RefreshDeps{ReloadDeps: ReloadDeps{Discover: deps.Discover, NewClient: deps.NewClient}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if loop, err := NewLoop(tc.rt, tc.opts); err == nil || loop != nil {
				t.Fatalf("NewLoop() = (%v, %v), want validation error", loop, err)
			}
		})
	}
	loop, err := NewLoop(NewRuntime(), LoopOptions{Load: loader, Deps: deps})
	if err != nil {
		t.Fatal(err)
	}
	if got := loop.Reloads(); got != 0 {
		t.Fatalf("initial Reloads() = %d", got)
	}
}

func TestNormalizedRefreshReasonUsesFiniteFallback(t *testing.T) {
	for name, classify := range map[string]func(config.Server, error) string{
		"missing classifier":   nil,
		"empty classification": func(config.Server, error) string { return "" },
	} {
		t.Run(name, func(t *testing.T) {
			if got := normalizedReason(classify)(config.Server{}, errors.New("discovery failed")); got != RefreshReasonDiscoveryFailed {
				t.Fatalf("normalized reason = %q", got)
			}
		})
	}
	if got := normalizedReason(func(config.Server, error) string { return RefreshReasonUnavailable })(config.Server{}, nil); got != RefreshReasonUnavailable {
		t.Fatalf("non-empty classification = %q", got)
	}
}

// TestInboundTerminalObservesFirstStatusAndImplicitOK replaces the obsolete
// direct statusWriter implementation test with a consumer-behavior matrix: the
// terminal record must observe the first named status and the implicit 200 of
// a first write, while the response carries no second superfluous header.
func TestInboundTerminalObservesFirstStatusAndImplicitOK(t *testing.T) {
	cases := []struct {
		name       string
		next       http.HandlerFunc
		wantStatus any // nil means the record's http_status key must be absent
		wantWire   int // the status actually forwarded to the writer
	}{
		{
			name: "FirstStatusWins",
			next: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusTeapot)
				w.WriteHeader(http.StatusCreated)
			},
			wantStatus: float64(http.StatusTeapot),
			wantWire:   http.StatusTeapot,
		},
		{
			name: "ImplicitOKOnFirstWrite",
			next: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("stream"))
			},
			wantStatus: float64(http.StatusOK),
			wantWire:   http.StatusOK,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			captured := &bytes.Buffer{}
			hash := sha256.Sum256([]byte("token"))
			h := NewInboundHandler(tc.next, [][32]byte{hash}, InboundOptions{Logger: observe.NewLogger(captured)})
			r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"x"}`))
			r.Header.Set("Authorization", "Bearer token")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			if rec.Code != tc.wantWire {
				t.Fatalf("forwarded status = %d; want %d", rec.Code, tc.wantWire)
			}
			var terminal map[string]any
			for _, line := range strings.Split(strings.TrimSpace(captured.String()), "\n") {
				var record map[string]any
				if err := json.Unmarshal([]byte(line), &record); err != nil {
					t.Fatalf("decode terminal line: %v", err)
				}
				if record["event"] == "http_complete" {
					if terminal != nil {
						t.Fatal("more than one terminal record for one request")
					}
					terminal = record
				}
			}
			if terminal == nil {
				t.Fatal("no terminal record emitted")
			}
			got, ok := terminal["http_status"]
			if tc.wantStatus == nil {
				if ok && got != nil {
					t.Fatalf("http_status = %v; want absent", got)
				}
				return
			}
			if got != tc.wantStatus {
				t.Fatalf("observed http_status = %v; want %v", got, tc.wantStatus)
			}
		})
	}
}

func TestInboundAdmissionRejectsHostSizeAndSessionCap(t *testing.T) {
	hash := sha256.Sum256([]byte("token"))
	accepted := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(sessionIDHeader, "issued")
		w.WriteHeader(http.StatusOK)
	})
	h := NewInboundHandler(accepted, [][32]byte{hash}, InboundOptions{MaxRequestBytes: 2, MaxSessions: 1, AllowedHosts: []string{"allowed.test"}})
	request := func(body string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
		r.Host = "allowed.test"
		r.Header.Set("Authorization", "Bearer token")
		return r
	}
	tooLarge := httptest.NewRecorder()
	h.ServeHTTP(tooLarge, request("long"))
	if tooLarge.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status = %d", tooLarge.Code)
	}

	wrongHostReq := request("")
	wrongHostReq.Host = "other.test"
	wrongHost := httptest.NewRecorder()
	h.ServeHTTP(wrongHost, wrongHostReq)
	if wrongHost.Code != http.StatusForbidden {
		t.Fatalf("host rejection status = %d", wrongHost.Code)
	}

	first := httptest.NewRecorder()
	h.ServeHTTP(first, request(""))
	if first.Code != http.StatusOK || h.liveSessions() != 1 {
		t.Fatalf("first response = %d, live sessions = %d", first.Code, h.liveSessions())
	}
	second := httptest.NewRecorder()
	h.ServeHTTP(second, request(""))
	if second.Code != http.StatusServiceUnavailable {
		t.Fatalf("session cap status = %d", second.Code)
	}
}

func TestReadinessLevelTransitionsAndOperationsRoutes(t *testing.T) {
	state := NewReadinessState(true, true, true, true)
	if got := state.Level(); got != 0 {
		t.Fatalf("empty readiness level = %d", got)
	}
	state.SetServer("alpha", true, true, "")
	if got := state.Level(); got != 2 {
		t.Fatalf("ready level = %d", got)
	}
	state.SetServer("beta", false, false, "offline")
	if got := state.Level(); got != 1 {
		t.Fatalf("degraded level = %d", got)
	}
	state.SetLocal(false, true, true, true)
	if got := state.Level(); got != 0 {
		t.Fatalf("invalid local readiness level = %d", got)
	}

	handler := NewOperationsHandler(state, nil)
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/livez", http.StatusOK},
		{http.MethodPost, "/livez", http.StatusMethodNotAllowed},
		{http.MethodPost, "/readyz", http.StatusMethodNotAllowed},
		{http.MethodGet, "/metrics", http.StatusNotFound},
		{http.MethodGet, "/missing", http.StatusNotFound},
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.want {
			t.Errorf("%s %s status = %d, want %d", tc.method, tc.path, w.Code, tc.want)
		}
	}
	ready := httptest.NewRecorder()
	handler.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if ready.Code != http.StatusServiceUnavailable || !strings.Contains(ready.Body.String(), "unavailable") {
		t.Fatalf("unready response = %d %q", ready.Code, ready.Body.String())
	}
}

func TestOperationsRateLimitRejectsExcessRequests(t *testing.T) {
	called := 0
	handler := RateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called++
		w.WriteHeader(http.StatusNoContent)
	}), 0.0001, 1)
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/livez", nil))
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/livez", nil))
	if first.Code != http.StatusNoContent || second.Code != http.StatusTooManyRequests || called != 1 {
		t.Fatalf("responses = %d/%d, downstream calls = %d", first.Code, second.Code, called)
	}
}

func TestRefresherDisabledIntervalWaitsForTrigger(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	refresher := NewRefresher(NewRuntime(), func() *config.Config { return nil }, RefreshDeps{}, RefresherOptions{Interval: -time.Second})
	if refresher.ticks() != nil {
		t.Fatal("disabled polling exposed a tick channel")
	}
	done := make(chan struct{})
	go func() { refresher.Run(ctx); close(done) }()
	refresher.Trigger()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after cancellation")
	}
}
func TestInboundUnknownSessionAndDeleteReleaseTracking(t *testing.T) {
	hash := sha256.Sum256([]byte("token"))
	issued := "session-1"
	h := NewInboundHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("X-Mcp-Bridge-Rehydration") {
		case "initialize":
			w.Header().Set(sessionIDHeader, issued)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"rehydrate","result":{"protocolVersion":"2025-06-18","capabilities":{},"serverInfo":{"name":"test","version":"1"}}}`))
			return
		case "initialized":
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if r.Method != http.MethodDelete && r.Header.Get(sessionIDHeader) == "" {
			w.Header().Set(sessionIDHeader, issued)
		}
	}), [][32]byte{hash}, InboundOptions{Bound: func() InboundBounds { return InboundBounds{} }, Tokens: func() [][32]byte { return [][32]byte{hash} }})
	request := func(method, session string) *http.Request {
		r := httptest.NewRequest(method, "/mcp", nil)
		r.Header.Set("Authorization", "Bearer token")
		if session != "" {
			r.Header.Set(sessionIDHeader, session)
		}
		return r
	}
	unknown := httptest.NewRecorder()
	h.ServeHTTP(unknown, request(http.MethodPost, "not-issued"))
	if unknown.Code != http.StatusOK || h.liveSessions() != 1 {
		t.Fatalf("unknown session status=%d tracked=%d, want 200 and one rehydrated session", unknown.Code, h.liveSessions())
	}
	created := httptest.NewRecorder()
	h.ServeHTTP(created, request(http.MethodPost, ""))
	if h.liveSessions() != 2 {
		t.Fatalf("tracked sessions after issue = %d, want 2", h.liveSessions())
	}
	closed := httptest.NewRecorder()
	h.ServeHTTP(closed, request(http.MethodDelete, issued))
	if h.liveSessions() != 1 {
		t.Fatalf("tracked sessions after deleting issued session = %d, want 1", h.liveSessions())
	}
	closed = httptest.NewRecorder()
	h.ServeHTTP(closed, request(http.MethodDelete, "not-issued"))
	if h.liveSessions() != 0 {
		t.Fatalf("tracked sessions after deleting rehydrated session = %d, want 0", h.liveSessions())
	}
	missingReq := httptest.NewRequest(http.MethodGet, "/missing", nil)
	missing := httptest.NewRecorder()
	h.ServeHTTP(missing, missingReq)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("wrong route status = %d", missing.Code)
	}
}

func TestRefreshRejectsMissingCandidateAndDependencies(t *testing.T) {
	runtime := NewRuntime()
	if _, err := runtime.Refresh(nil, RefreshDeps{}); err == nil {
		t.Fatal("Refresh accepted a nil candidate")
	}
	if _, err := runtime.Refresh(&config.Config{}, RefreshDeps{}); err == nil {
		t.Fatal("Refresh accepted missing discovery/clock")
	}
}

func TestOperationsRateLimitRejectsUnconfiguredBucket(t *testing.T) {
	for _, tc := range []struct {
		rate  float64
		burst int
	}{{0, 1}, {1, 0}} {
		h := RateLimit(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unconfigured limiter called downstream") }), tc.rate, tc.burst)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("limiter (%v,%d) status = %d", tc.rate, tc.burst, w.Code)
		}
	}
}

func TestRefreshReportDoesNotEmitWithoutPublishedAssembly(t *testing.T) {
	var reported int
	r := NewRefresher(NewRuntime(), nil, RefreshDeps{}, RefresherOptions{Metrics: func(string, string, string) { reported++ }})
	r.report(nil, errors.New("refresh rejected"))
	if reported != 0 {
		t.Fatalf("metrics emitted without a published assembly: %d", reported)
	}
}

func TestRefreshReportsFreshAndRetainedStaleCatalog(t *testing.T) {
	t.Setenv("EDGE_ALPHA_SECRET", "alpha-secret")
	t.Setenv("EDGE_CLIENT_TOKEN", "client-token")
	input := strings.Join([]string{
		"schema_version: 1", "servers:", "  - id: alpha", "    endpoint: https://alpha.invalid/mcp",
		"    credentials:", "      bearer: {env: EDGE_ALPHA_SECRET}", "client_auth:",
		"  bearer_tokens:", "    - {env: EDGE_CLIENT_TOKEN}", "pins: []",
	}, "\n")
	cfg, err := config.Parse([]byte(input), ".")
	if err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime()
	definitions := []core.ToolDefinition{{RawName: "read", Description: "read one row", InputSchema: []byte("{\"type\":\"object\"}")}}
	deps := RefreshDeps{ReloadDeps: ReloadDeps{
		Discover:            func(config.Server) ([]core.ToolDefinition, error) { return definitions, nil },
		NewClient:           func(config.Server) (core.Upstream, error) { return &core.UpstreamRecorder{}, nil },
		Now:                 func() time.Time { return time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC) },
		ModelIdentityDigest: strings.Repeat("a", 64),
	}, Reason: func(config.Server, error) string { return RefreshReasonUnavailable }}
	first, err := runtime.Reload(cfg, deps.ReloadDeps)
	if err != nil {
		t.Fatal(err)
	}
	var metrics [][3]string
	r := NewRefresher(runtime, func() *config.Config { return cfg }, deps, RefresherOptions{Metrics: func(id, status, reason string) { metrics = append(metrics, [3]string{id, status, reason}) }})
	r.report(first, nil)
	if len(metrics) != 1 || metrics[0] != [3]string{"alpha", "fresh", ""} {
		t.Fatalf("fresh metrics = %v", metrics)
	}
	definitions = append(definitions, core.ToolDefinition{RawName: "read2", Description: "another read", InputSchema: []byte("{\"type\":\"object\"}")})
	second, err := runtime.Refresh(cfg, deps)
	if err != nil {
		t.Fatal(err)
	}
	if second.CatalogRevision() == first.CatalogRevision() || len(second.Catalog().Bindings()) != 2 {
		t.Fatalf("successful refresh revision/catalog = %q/%d", second.CatalogRevision(), len(second.Catalog().Bindings()))
	}
	metrics = nil
	deps.Discover = func(config.Server) ([]core.ToolDefinition, error) { return nil, errors.New("offline") }
	third, err := runtime.Refresh(cfg, deps)
	if err != nil {
		t.Fatal(err)
	}
	if len(third.Catalog().Bindings()) != 2 {
		t.Fatalf("failed discovery lost retained bindings: %d", len(third.Catalog().Bindings()))
	}
	health, ok := third.Health("alpha")
	if !ok || health.Status != "stale" || health.Reason != RefreshReasonUnavailable {
		t.Fatalf("stale health = %+v, exists=%v", health, ok)
	}
	r.report(nil, errors.New("rejected refresh"))
	if len(metrics) != 1 || metrics[0] != [3]string{"alpha", "stale", RefreshReasonUnavailable} {
		t.Fatalf("stale metrics = %v", metrics)
	}
}

type toolEdgeClient struct {
	calls   int
	outcome *core.UpstreamOutcome
}

func (c *toolEdgeClient) CallTool(_ context.Context, _, _, _ string, _ json.RawMessage) (json.RawMessage, *core.UpstreamOutcome) {
	c.calls++
	return json.RawMessage(`{"content":[]}`), c.outcome
}

func newToolEdgeSurface(t *testing.T, client *toolEdgeClient) (*ToolSurface, *Assembly) {
	t.Helper()
	t.Setenv("EDGE_ALPHA_SECRET", "alpha-secret")
	t.Setenv("EDGE_CLIENT_TOKEN", "client-token")
	input := strings.Join([]string{
		"schema_version: 1", "servers:", "  - id: alpha", "    endpoint: https://alpha.invalid/mcp",
		"    credentials:", "      bearer: {env: EDGE_ALPHA_SECRET}", "client_auth:",
		"  bearer_tokens:", "    - {env: EDGE_CLIENT_TOKEN}", "pins:",
		"  - {server: alpha, tool: read}",
	}, "\n")
	cfg, err := config.Parse([]byte(input), ".")
	if err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime()
	defs := []core.ToolDefinition{
		{RawName: "read", Description: "read one row", InputSchema: []byte("{\"type\":\"object\",\"properties\":{\"limit\":{\"type\":\"integer\",\"minimum\":1}},\"required\":[\"limit\"],\"additionalProperties\":false}"), OutputSchema: []byte("{\"type\":\"object\"}")},
		{RawName: "broken", Description: "unsupported", InputSchema: []byte("{\"type\":\"object\",\"properties\":{\"x\":{\"$ref\":\"#/$defs/missing\"}}}")},
	}
	assembly, err := runtime.Reload(cfg, ReloadDeps{
		Discover:            func(config.Server) ([]core.ToolDefinition, error) { return defs, nil },
		NewClient:           func(config.Server) (core.Upstream, error) { return client, nil },
		Now:                 time.Now,
		ModelIdentityDigest: strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	surface, err := NewToolSurface(runtime, ToolSurfaceConfig{Model: search.ModelIdentity{Provider: "test", Model: "embed", Dimensions: 4, FormatVersion: "v1"}})
	if err != nil {
		t.Fatal(err)
	}
	return surface, assembly
}

func TestToolSurfaceReturnsSafeAdmissionAndDispatchErrors(t *testing.T) {
	client := &toolEdgeClient{}
	surface, assembly := newToolEdgeSurface(t, client)
	request := func(arguments string) *mcp.CallToolRequest {
		return &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: []byte(arguments)}}
	}
	call := func(args string) map[string]any {
		result, err := surface.callTool(context.Background(), request(args))
		if err != nil {
			t.Fatal(err)
		}
		payload, ok := result.StructuredContent.(map[string]any)
		if !ok {
			t.Fatalf("structured error payload has type %T", result.StructuredContent)
		}
		return payload
	}
	readName := core.PublicName("alpha", "read")
	brokenName := core.PublicName("alpha", "broken")
	valid := `{"tool_name":"` + readName + `","catalog_revision":"` + assembly.CatalogRevision() + `","arguments":{"limit":"secret-value"}}`
	badArgs := call(valid)
	if badArgs["code"] != "invalid_arguments" || badArgs["execution_state"] != core.ExecutionNotStarted || strings.Contains(badArgs["message"].(string), "secret-value") || client.calls != 0 {
		t.Fatalf("invalid argument response/calls = %v/%d", badArgs, client.calls)
	}
	unknown := call(`{"tool_name":"unknown","catalog_revision":"` + assembly.CatalogRevision() + `"}`)
	if unknown["code"] != codeUnknownTool {
		t.Fatalf("unknown tool response = %v", unknown)
	}
	stale := call(`{"tool_name":"` + readName + `","catalog_revision":"` + strings.Repeat("b", 64) + `","arguments":{"limit":1}}`)
	if stale["code"] != codeCatalogChanged {
		t.Fatalf("stale revision response = %v", stale)
	}
	unsupported := call(`{"tool_name":"` + brokenName + `","catalog_revision":"` + assembly.CatalogRevision() + `","arguments":{}}`)
	if unsupported["code"] != codeUnsupportedSchema {
		t.Fatalf("unsupported schema response = %v", unsupported)
	}
	client.outcome = &core.UpstreamOutcome{Failure: &core.UpstreamFailure{Code: "backend_busy", ExecutionState: core.ExecutionUnknown}}
	busy := call(`{"tool_name":"` + readName + `","catalog_revision":"` + assembly.CatalogRevision() + `","arguments":{"limit":1}}`)
	if busy["code"] != "backend_busy" || client.calls != 1 {
		t.Fatalf("dispatch failure/calls = %v/%d", busy, client.calls)
	}
}

func TestLoopStartRejectsLoadAndStartupAdmissionFailures(t *testing.T) {
	deps := RefreshDeps{ReloadDeps: ReloadDeps{
		Discover:  func(config.Server) ([]core.ToolDefinition, error) { return nil, nil },
		NewClient: func(config.Server) (core.Upstream, error) { return &core.UpstreamRecorder{}, nil },
		Now:       time.Now,
	}}
	loadFailure := errors.New("configuration unreadable")
	for _, tc := range []struct {
		name   string
		load   func() (*config.Config, error)
		reason string
	}{
		{"loader error", func() (*config.Config, error) { return nil, loadFailure }, "invalid_configuration"},
		{"nil startup candidate", func() (*config.Config, error) { return nil, nil }, "admission_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var reported [][2]string
			loop, err := NewLoop(NewRuntime(), LoopOptions{Load: tc.load, Deps: deps, ReloadMetrics: func(outcome, reason string) { reported = append(reported, [2]string{outcome, reason}) }})
			if err != nil {
				t.Fatal(err)
			}
			if err := loop.Start(context.Background()); err == nil {
				t.Fatal("Start accepted a rejected startup")
			}
			if len(reported) != 1 || reported[0] != [2]string{"rejected", tc.reason} {
				t.Fatalf("reported outcomes = %v", reported)
			}
		})
	}
}

func TestStartupAdmissionAndDrainHandleEmptyRuntime(t *testing.T) {
	runtime := NewRuntime()
	if _, err := runtime.AdmitStartup(nil, ReloadDeps{}, nil); err == nil {
		t.Fatal("AdmitStartup accepted nil candidate")
	}
	if _, err := runtime.AdmitStartup(&config.Config{}, ReloadDeps{}, nil); err == nil {
		t.Fatal("AdmitStartup accepted missing client/clock")
	}
	runtime.Drain(ReloadDeps{})
	if runtime.Current() != nil {
		t.Fatal("draining an empty runtime published an assembly")
	}
}

func TestRefreshRunPassSkipsMissingCurrentConfiguration(t *testing.T) {
	r := NewRefresher(NewRuntime(), func() *config.Config { return nil }, RefreshDeps{}, RefresherOptions{})
	r.runPass()
	if r.Passes() != 0 {
		t.Fatalf("passes without current configuration = %d", r.Passes())
	}
}

func TestBuildModeStringNamesAdmissionModes(t *testing.T) {
	for mode, want := range map[buildMode]string{buildReload: "reload", buildRefresh: "refresh", buildStartup: "startup"} {
		if got := mode.String(); got != want {
			t.Errorf("mode %d string = %q, want %q", mode, got, want)
		}
	}
}

func TestToolSurfaceValidationAndSafeErrorVocabulary(t *testing.T) {
	if _, err := NewToolSurface(nil, ToolSurfaceConfig{Model: search.ModelIdentity{Dimensions: 4}}); err == nil {
		t.Fatal("tool surface accepted nil runtime")
	}
	if _, err := NewToolSurface(NewRuntime(), ToolSurfaceConfig{}); err == nil {
		t.Fatal("tool surface accepted zero embedding dimensions")
	}
	surface := &ToolSurface{runtime: NewRuntime()}
	if err := surface.Register(nil); err == nil {
		t.Fatal("tool surface registered on nil MCP server")
	}
	request := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: []byte("{\"query\":\"read\"}")}}
	result, err := surface.searchTools(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.StructuredContent.(map[string]any)["code"]; got != "backend_unavailable" {
		t.Fatalf("search without catalog code = %v", got)
	}
	bad, err := surface.describeTool(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: []byte("{")}})
	if err != nil {
		t.Fatal(err)
	}
	if got := bad.StructuredContent.(map[string]any)["code"]; got != "invalid_arguments" {
		t.Fatalf("invalid describe code = %v", got)
	}
}

func TestDispatchMessagesRemainFiniteAndPayloadFree(t *testing.T) {
	for code, want := range map[string]string{
		codeCatalogChanged:       "the catalog revision changed",
		codeUnknownTool:          "no such tool in the published catalog",
		"unsupported_capability": "the upstream requested an unsupported interaction",
		"backend_auth_failed":    "the upstream call did not complete",
		"unrecognized":           "the upstream call did not complete",
	} {
		if got := dispatchMessage(code); got != want {
			t.Errorf("message for %q = %q, want %q", code, got, want)
		}
	}
	result := failureResult(errors.New("private upstream detail"))
	payload := result.StructuredContent.(map[string]any)
	if payload["code"] != codeInternalError || strings.Contains(payload["message"].(string), "private upstream detail") {
		t.Fatalf("untyped failure leaked detail: %v", payload)
	}
}

func TestPinnedNamesSkipsUnresolvedPinBindings(t *testing.T) {
	assembly := &Assembly{pins: []core.ResolvedPin{{}}}
	if got := assembly.PinnedNames(); len(got) != 0 {
		t.Fatalf("unresolved pins became public names: %v", got)
	}
}

func TestLoopWatchExitsWithoutSignalSource(t *testing.T) {
	loop := &Loop{}
	done := make(chan struct{})
	go func() {
		loop.watch(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watch did not exit when signal source is absent")
	}
}

func TestToolSurfaceRejectsCallsWithoutPublishedGeneration(t *testing.T) {
	surface, err := NewToolSurface(NewRuntime(), ToolSurfaceConfig{Model: search.ModelIdentity{Dimensions: 4}})
	if err != nil {
		t.Fatal(err)
	}
	request := func(arguments string) *mcp.CallToolRequest {
		return &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: []byte(arguments)}}
	}
	checks := []struct {
		name string
		call func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error)
		args string
	}{
		{name: "describe", call: surface.describeTool, args: "{\"tool_name\":\"alpha.read\",\"catalog_revision\":\"rev\"}"},
		{name: "call", call: surface.callTool, args: "{\"tool_name\":\"alpha.read\",\"catalog_revision\":\"rev\",\"arguments\":{}}"},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			result, err := check.call(context.Background(), request(check.args))
			if err != nil {
				t.Fatal(err)
			}
			payload, ok := result.StructuredContent.(map[string]any)
			if !ok || payload["code"] != codeBackendUnavailable {
				t.Fatalf("unpublished call response = %#v", result.StructuredContent)
			}
		})
	}
}
func TestToolPayloadDecodingBoundaries(t *testing.T) {
	if got := rawArguments(nil); got != nil {
		t.Fatalf("nil request arguments = %q", got)
	}
	if got := rawArguments(&mcp.CallToolRequest{}); got != nil {
		t.Fatalf("nil params arguments = %q", got)
	}
	if err := decodeStrict(nil, &struct{}{}); err == nil {
		t.Fatal("empty strict input was accepted")
	}
	var target struct{}
	if err := decodeStrict([]byte("{\"unknown\":true}"), &target); err == nil {
		t.Fatal("unknown strict field was accepted")
	}
	if got := rawValue(nil); got != nil {
		t.Fatalf("empty raw value = %#v", got)
	}
	if got := rawValue([]byte("not-json")); got != nil {
		t.Fatalf("malformed raw value = %#v", got)
	}
	if got := decodeUpstreamResult(nil); len(got.Content) != 0 || got.IsError {
		t.Fatalf("empty upstream result = %#v", got)
	}
	if got := decodeUpstreamResult([]byte("not-json")); len(got.Content) != 0 || got.IsError {
		t.Fatalf("malformed upstream result = %#v", got)
	}
	if got := timestamp(time.Time{}); got != nil {
		t.Fatalf("zero timestamp = %#v", got)
	}
	if got := timestamp(time.Date(2025, 3, 4, 5, 6, 7, 0, time.FixedZone("east", 2*60*60))); got != "2025-03-04T03:06:07Z" {
		t.Fatalf("UTC timestamp = %#v", got)
	}
}
func TestSearchToolsRejectsInvalidAndUnpublishedRequests(t *testing.T) {
	request := func(arguments string) *mcp.CallToolRequest {
		return &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: []byte(arguments)}}
	}
	empty, err := NewToolSurface(NewRuntime(), ToolSurfaceConfig{Model: search.ModelIdentity{Dimensions: 4}, Search: search.Config{MaxQueryLength: 20}})
	if err != nil {
		t.Fatal(err)
	}
	invalid, err := empty.searchTools(context.Background(), request("not-json"))
	if err != nil {
		t.Fatal(err)
	}
	if invalid.StructuredContent.(map[string]any)["code"] != search.CodeInvalidArguments {
		t.Fatalf("invalid search request = %#v", invalid.StructuredContent)
	}
	unpublished, err := empty.searchTools(context.Background(), request("{\"query\":\"term\"}"))
	if err != nil {
		t.Fatal(err)
	}
	if unpublished.StructuredContent.(map[string]any)["code"] != codeBackendUnavailable {
		t.Fatalf("unpublished search response = %#v", unpublished.StructuredContent)
	}

	client := &toolEdgeClient{}
	surface, _ := newToolEdgeSurface(t, client)
	surface.cfg.Search = search.Config{MaxQueryLength: 20, TopKMax: -1}
	failedIndex, err := surface.searchTools(context.Background(), request("{\"query\":\"term\"}"))
	if err != nil {
		t.Fatal(err)
	}
	if failedIndex.StructuredContent.(map[string]any)["code"] == "" {
		t.Fatalf("searcher failure was not represented: %#v", failedIndex.StructuredContent)
	}
}
func TestInboundUnauthorizedLogClassifiesAuthFailure(t *testing.T) {
	var logs bytes.Buffer
	handler := NewInboundHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unauthorized request reached handler") }), [][32]byte{sha256.Sum256([]byte("token"))}, InboundOptions{Logger: observe.NewLogger(&logs)})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.Contains(logs.String(), "client_auth_failed") {
		t.Fatalf("unauthorized audit event = %s", logs.String())
	}
	if strings.Contains(logs.String(), "backend_auth_failed") {
		t.Fatalf("unauthorized audit event falsely contained backend_auth_failed: %s", logs.String())
	}
}

func TestInboundSessionReaperThrottlesSmallSweeps(t *testing.T) {
	base := time.Date(2025, 4, 5, 6, 7, 8, 0, time.UTC)
	handler := NewInboundHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil, InboundOptions{SessionIdle: time.Second})
	handler.sessions["expired"] = sessionRecord{lastSeen: base.Add(-2 * time.Second)}
	handler.lastSweep = base.Add(-time.Millisecond)
	handler.mu.Lock()
	handler.reapExpiredLocked(base)
	_, stillTracked := handler.sessions["expired"]
	handler.mu.Unlock()
	if !stillTracked {
		t.Fatal("recent small sweep unexpectedly reaped session")
	}
	handler.mu.Lock()
	handler.reapExpiredLocked(base.Add(time.Second))
	_, stillTracked = handler.sessions["expired"]
	handler.mu.Unlock()
	if stillTracked {
		t.Fatal("expired session survived next sweep")
	}
}

func TestDescribeToolReturnsStoredOutputSchemaAndUnknownTool(t *testing.T) {
	upstream := &toolEdgeClient{}
	surface, assembly := newToolEdgeSurface(t, upstream)
	server := NewMCPServer()
	if err := surface.Register(server); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := surface.Published(assembly); err != nil {
		t.Fatalf("Published: %v", err)
	}
	const token = "client-token"
	inbound := NewInboundHandler(
		NewStreamableHandler(server),
		[][32]byte{sha256.Sum256([]byte(token))},
		InboundOptions{ToolSurface: surface},
	)
	httpServer := httptest.NewServer(inbound)
	t.Cleanup(httpServer.Close)
	session := connectProgressiveDiscoveryClient(t, httpServer, token)
	request := func(name string) *mcp.CallToolParams {
		return &mcp.CallToolParams{
			Name:      "describe_tool",
			Arguments: json.RawMessage(fmt.Sprintf("{\"tool_name\":%q,\"catalog_revision\":%q}", name, assembly.CatalogRevision())),
		}
	}
	read := core.PublicName("alpha", "read")
	described, err := session.CallTool(context.Background(), request(read))
	if err != nil {
		t.Fatal(err)
	}
	payload := described.StructuredContent.(map[string]any)
	if schema, ok := payload["output_schema"].(map[string]any); !ok || schema["type"] != "object" {
		t.Fatalf("output schema = %#v", payload["output_schema"])
	}
	unknown, err := session.CallTool(context.Background(), request("missing"))
	if err != nil {
		t.Fatal(err)
	}
	if got := unknown.StructuredContent.(map[string]any)["code"]; got != codeUnknownTool {
		t.Fatalf("unknown describe code = %#v", got)
	}
	if upstream.calls != 0 {
		t.Fatalf("upstream calls = %d, want 0", upstream.calls)
	}
}

func TestServerDefinitionsWithoutPreviousCatalogReturnsEmpty(t *testing.T) {
	if got := serverDefinitions(nil, "alpha"); len(got) != 0 {
		t.Fatalf("definitions without previous generation = %#v", got)
	}
}

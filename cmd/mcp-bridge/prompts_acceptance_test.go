package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

// methodLog records the MCP methods one upstream server was actually asked for.
// It sits in front of the streamable handler, so the count is evidence of what
// left the bridge, not of what the bridge intended.
type methodLog struct {
	mu      sync.Mutex
	methods []string
}

func (l *methodLog) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err == nil && len(body) > 0 {
				var header struct {
					Method string `json:"method"`
				}
				if json.Unmarshal(body, &header) == nil && header.Method != "" {
					l.record(header.Method)
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (l *methodLog) record(method string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.methods = append(l.methods, method)
}

func (l *methodLog) count(method string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	total := 0
	for _, seen := range l.methods {
		if seen == method {
			total++
		}
	}
	return total
}

type getCall struct {
	name      string
	arguments map[string]string
}

// promptUpstream is one scripted upstream server. It serves the methods the
// bridge must call (tools/list, prompts/list, prompts/get), records every call
// and can fail prompts/get on demand, so the acceptance test can prove
// exactly-once routing, capability gating and safe error translation.
type promptUpstream struct {
	httpServer *httptest.Server
	server     *mcp.Server
	log        *methodLog

	mu       sync.Mutex
	gets     []getCall
	failWith string
}

func newPromptUpstream(t *testing.T, toolName string, promptsCapability bool) *promptUpstream {
	t.Helper()
	var options *mcp.ServerOptions
	if promptsCapability {
		options = &mcp.ServerOptions{HasPrompts: true}
	}
	upstream := &promptUpstream{
		server: mcp.NewServer(&mcp.Implementation{Name: "prompt-upstream", Version: "1"}, options),
		log:    &methodLog{},
	}
	upstream.server.AddTool(
		&mcp.Tool{
			Name:        toolName,
			Description: toolName + " tool",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer"}},"additionalProperties":true}`),
		},
		func(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{
				Text: "upstream " + toolName + " saw " + string(request.Params.Arguments),
			}}}, nil
		})
	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return upstream.server },
		&mcp.StreamableHTTPOptions{Stateless: true},
	)
	upstream.httpServer = httptest.NewServer(upstream.log.wrap(handler))
	return upstream
}

func (up *promptUpstream) address() string { return up.httpServer.URL }

func (up *promptUpstream) close() { up.httpServer.Close() }

// addPrompt publishes one upstream prompt. Its rendered message carries the
// server marker, never the raw upstream name, so the assertions can tell which
// upstream answered without relying on a value the bridge must hide.
func (up *promptUpstream) addPrompt(rawName, description, marker string) {
	up.server.AddPrompt(
		&mcp.Prompt{
			Name:        rawName,
			Description: description,
			Arguments:   []*mcp.PromptArgument{{Name: "who", Description: "who is asking"}},
		},
		func(_ context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			up.mu.Lock()
			up.gets = append(up.gets, getCall{name: request.Params.Name, arguments: request.Params.Arguments})
			failWith := up.failWith
			up.mu.Unlock()
			if failWith != "" {
				return nil, errors.New(failWith)
			}
			return &mcp.GetPromptResult{
				Description: description,
				Messages: []*mcp.PromptMessage{{
					Role:    mcp.Role("user"),
					Content: &mcp.TextContent{Text: "rendered " + marker + " for " + request.Params.Arguments["who"]},
				}},
			}, nil
		})
}

func (up *promptUpstream) removePrompt(rawName string) { up.server.RemovePrompts(rawName) }

func (up *promptUpstream) getCalls() []getCall {
	up.mu.Lock()
	defer up.mu.Unlock()
	return append([]getCall(nil), up.gets...)
}

func (up *promptUpstream) setFailure(text string) {
	up.mu.Lock()
	defer up.mu.Unlock()
	up.failWith = text
}

// notificationLog counts the prompts/list_changed notifications one inbound
// client received.
type notificationLog struct {
	mu       sync.Mutex
	received int
}

func (l *notificationLog) record(context.Context, *mcp.PromptListChangedRequest) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.received++
}

func (l *notificationLog) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.received
}

func connectBridgePrompts(t *testing.T, ctx context.Context, endpoint string, changed *notificationLog) *mcp.ClientSession {
	t.Helper()
	options := &mcp.ClientOptions{}
	if changed != nil {
		options.PromptListChangedHandler = changed.record
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "prompt-acceptance-client", Version: "1"}, options)
	// The version is pinned to the newest accepted inbound version. A client that
	// offers the pre-release 2026-07-28 version is negotiated down to 2025-11-25
	// by the bridge, but go-sdk@v1.8.0 still classifies that server session as
	// SEP-2575 because notifySessions (mcp/server.go) keys off the version the
	// client offered in InitializeParams; its list-changed notifications would
	// then be routed to a subscriptions/listen stream the legacy handshake never
	// opened, and the prompt surface would be unobservable. The four legacy
	// versions are the bridge's accepted inbound call versions, so the prompt
	// surface is exercised as the legacy client it serves.
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   endpoint,
		HTTPClient: &http.Client{Transport: bearerTransport{token: serveClientToken, base: http.DefaultTransport}},
		MaxRetries: -1,
	}, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
	if err != nil {
		t.Fatalf("connect bridge: %v", err)
	}
	return session
}

// bridgeRun is one live bridge process under test.
type bridgeRun struct {
	cancel        context.CancelFunc
	done          chan int
	stdout        *syncWriter
	stderr        *syncWriter
	httpURL       string
	operationsURL string
}

func startBridgeProcess(t *testing.T, ctx context.Context, configPath, httpAddress, operationsAddress string) *bridgeRun {
	t.Helper()
	processCtx, cancel := context.WithCancel(ctx)
	stdout, stderr := &syncWriter{}, &syncWriter{}
	done := make(chan int, 1)
	go func() { done <- runContext(processCtx, []string{"--config", configPath}, stdout, stderr) }()
	run := &bridgeRun{
		cancel:        cancel,
		done:          done,
		stdout:        stdout,
		stderr:        stderr,
		httpURL:       "http://" + httpAddress,
		operationsURL: "http://" + operationsAddress,
	}
	waitFor(t, "the operations listener of a new bridge process", func() bool {
		status, _, ok := probeBody(t, run.operationsURL+"/livez")
		return ok && status == http.StatusOK
	})
	return run
}

func (r *bridgeRun) stop(t *testing.T) {
	t.Helper()
	r.cancel()
	select {
	case status := <-r.done:
		if status != 0 {
			t.Fatalf("bridge run status = %d; want 0 (stderr=%s)", status, r.stderr.String())
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the bridge did not shut down")
	}
}

func listPromptNames(t *testing.T, ctx context.Context, session *mcp.ClientSession) []string {
	t.Helper()
	page, err := session.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatalf("prompts/list: %v", err)
	}
	names := make([]string, 0, len(page.Prompts))
	for _, prompt := range page.Prompts {
		names = append(names, prompt.Name)
	}
	sort.Strings(names)
	return names
}

// TestPromptSurfaceEndToEnd is the process-level acceptance of the prompt
// increment (CT-07): capability negotiation, collision-safe publication with
// provenance, capability and opt-out isolation, bounded observability, tool
// revision isolation across prompt-only changes, exactly-once get routing with
// safe errors, and durable last-known-good prompts across a dark restart.
func TestPromptPassThroughCT07(t *testing.T) {
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)

	alpha := newPromptUpstream(t, "read", true)
	beta := newPromptUpstream(t, "list", true)
	// gamma advertises no prompts capability at all: its upstream must never be
	// asked for one.
	gamma := newPromptUpstream(t, "scan", false)
	delta := newPromptUpstream(t, "lookup", true)

	embeddings := fakeEmbeddings(t)
	httpAddress := freeAddress(t)
	operationsAddress := freeAddress(t)
	directory := t.TempDir()
	configPath := filepath.Join(directory, "bridge.yaml")
	storageDirectory := filepath.Join(directory, "state")
	// A short refresh interval makes the periodic re-discovery observable
	// without signalling the process.
	writeBridgeConfigInterval(t, configPath, httpAddress, operationsAddress, storageDirectory, embeddings.URL, "250ms", []bridgeServerSpec{
		{id: "alpha", endpoint: alpha.address()},
		{id: "beta", endpoint: beta.address()},
		{id: "gamma", endpoint: gamma.address()},
		{id: "delta", endpoint: delta.address(), promptsDisabled: true},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	run := startBridgeProcess(t, ctx, configPath, httpAddress, operationsAddress)

	changed := &notificationLog{}
	session := connectBridgePrompts(t, ctx, run.httpURL+"/mcp", changed)
	defer session.Close()

	// Phase 1: the bridge is a prompt provider with change notification, and a
	// cold catalog publishes no prompt yet.
	caps := session.InitializeResult().Capabilities
	if caps.Prompts == nil || !caps.Prompts.ListChanged {
		t.Fatalf("initialize capabilities = %+v; want prompts.listChanged", caps.Prompts)
	}
	if names := listPromptNames(t, ctx, session); len(names) != 0 {
		t.Fatalf("prompts before any upstream published one = %v", names)
	}

	// Phase 2: two upstreams with the SAME raw name must both be reachable and
	// indistinguishable-looking without leaking the raw name.
	const collidingRawName = "raw-collision-name"
	alpha.addPrompt(collidingRawName, "alpha prompt body", "alpha body")
	beta.addPrompt(collidingRawName, "beta prompt body", "beta body")
	alphaPublicName := core.PromptPublicName("alpha", collidingRawName)
	betaPublicName := core.PromptPublicName("beta", collidingRawName)
	if alphaPublicName == betaPublicName {
		t.Fatalf("colliding public names are not distinct: %q", alphaPublicName)
	}
	var page *mcp.ListPromptsResult
	waitFor(t, "both colliding prompts to be published", func() bool {
		var err error
		page, err = session.ListPrompts(ctx, nil)
		return err == nil && len(page.Prompts) == 2
	})
	seen := map[string]string{}
	for _, prompt := range page.Prompts {
		seen[prompt.Name] = prompt.Description
	}
	for _, want := range []string{alphaPublicName, betaPublicName} {
		if _, ok := seen[want]; !ok {
			t.Fatalf("published prompts = %v; want the server-derived name %q", seen, want)
		}
	}
	if !strings.Contains(seen[alphaPublicName], "via alpha") || !strings.Contains(seen[betaPublicName], "via beta") {
		t.Fatalf("mirrored descriptions = %v; want per-server provenance", seen)
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("marshal prompt page: %v", err)
	}
	if strings.Contains(string(encoded), collidingRawName) {
		t.Fatalf("the raw upstream name reached the client: %s", encoded)
	}

	// Phase 3: capability gating and operator opt-out keep the outbound promise.
	if gamma.log.count("prompts/list") != 0 {
		t.Fatalf("an upstream without the prompts capability was asked for prompts")
	}
	if delta.log.count("prompts/list") != 0 {
		t.Fatalf("a prompts-disabled upstream was asked for prompts")
	}
	if alpha.log.count("prompts/list") == 0 || beta.log.count("prompts/list") == 0 {
		t.Fatalf("prompt discovery never ran, so the negative assertions are vacuous")
	}
	if names := listPromptNames(t, ctx, session); len(names) != 2 {
		t.Fatalf("prompts = %v; want only the two capability-bearing servers", names)
	}

	// Phase 4: observability stays bounded to the finite vocabulary, one
	// unlabelled gauge, and no prompt data.
	waitFor(t, "bounded prompt metrics", func() bool {
		status, body, ok := probeBody(t, run.operationsURL+"/metrics")
		return ok && status == http.StatusOK &&
			strings.Contains(body, `bridge_requests_total{operation="prompts_list"`) &&
			strings.Contains(body, "bridge_catalog_prompts 2")
	})
	if _, body := getBody(t, run.operationsURL+"/metrics"); strings.Contains(body, collidingRawName) {
		t.Fatalf("a prompt value reached the metrics exposition: %s", body)
	}

	// Phase 5: a prompt-only change must not invalidate a tool catalog revision.
	payload := searchPayload(t, ctx, session, "read")
	toolRevision, readName := catalogRevisionAndTool(t, payload, "alpha")
	baselineNotifications := changed.count()
	alpha.addPrompt("added-later", "alpha second prompt", "alpha second body")
	waitFor(t, "the added prompt to be published", func() bool {
		return len(listPromptNames(t, ctx, session)) == 3
	})
	waitFor(t, "a prompts/list_changed notification", func() bool {
		return changed.count() > baselineNotifications
	})
	if !strings.Contains(alphaPublicName, "p_") {
		t.Fatalf("server-derived prompt name %q is not opaque", alphaPublicName)
	}
	// The tool catalog of this run is byte-identical, so the revision a client
	// already holds stays admissible.
	afterRevision, _ := catalogRevisionAndTool(t, searchPayload(t, ctx, session, "read"), "alpha")
	if afterRevision != toolRevision {
		t.Fatalf("prompt change moved the tool revision: %s -> %s", toolRevision, afterRevision)
	}
	stillAdmitted := callTool(t, ctx, session, catalog.ToolCallTool, map[string]any{
		"tool_name":        readName,
		"catalog_revision": toolRevision,
		"arguments":        map[string]any{"limit": 7},
	})
	if stillAdmitted.IsError {
		t.Fatalf("a prompt-only change invalidated a held tool revision: %#v", stillAdmitted)
	}
	alpha.removePrompt("added-later")
	waitFor(t, "the removed prompt to disappear", func() bool {
		return len(listPromptNames(t, ctx, session)) == 2
	})
	waitFor(t, "a removal notification", func() bool {
		return changed.count() > baselineNotifications+1
	})

	// Phase 6: get routing is exactly-once, forwards arguments, and reports a
	// safe error when the upstream fails.
	got, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      alphaPublicName,
		Arguments: map[string]string{"who": "alpha-caller"},
	})
	if err != nil {
		t.Fatalf("prompts/get: %v", err)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("prompts/get messages = %#v", got.Messages)
	}
	text, ok := got.Messages[0].Content.(*mcp.TextContent)
	if !ok || !strings.Contains(text.Text, "alpha body") || !strings.Contains(text.Text, "alpha-caller") {
		t.Fatalf("prompts/get content = %#v; want alpha's rendered prompt", got.Messages[0].Content)
	}
	alphaGets := alpha.getCalls()
	if len(alphaGets) != 1 {
		t.Fatalf("upstream alpha saw %d prompt gets; want exactly one", len(alphaGets))
	}
	if alphaGets[0].name != collidingRawName {
		t.Fatalf("upstream get name = %q; want the raw upstream name", alphaGets[0].name)
	}
	if len(alphaGets[0].arguments) != 1 || alphaGets[0].arguments["who"] != "alpha-caller" {
		t.Fatalf("upstream get arguments = %v; want the forwarded arguments", alphaGets[0].arguments)
	}
	if calls := beta.getCalls(); len(calls) != 0 {
		t.Fatalf("the collision-free get also reached beta: %#v", calls)
	}

	alpha.setFailure("raw-secret-transport-canary")
	if _, err := session.GetPrompt(ctx, &mcp.GetPromptParams{Name: alphaPublicName}); err == nil {
		t.Fatal("a failing upstream prompt returned success")
	} else if strings.Contains(err.Error(), "raw-secret-transport-canary") {
		t.Fatalf("the raw upstream error leaked to the client: %v", err)
	} else if !strings.Contains(err.Error(), "prompt unavailable") {
		t.Fatalf("prompt failure message = %v; want the generic prompt unavailable error", err)
	}
	alpha.setFailure("")

	unknown, err := session.GetPrompt(ctx, &mcp.GetPromptParams{Name: "p_unknown_prompt"})
	if err == nil {
		t.Fatalf("an unknown public prompt name returned success: %#v", unknown)
	}
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc.CodeInvalidParams {
		t.Fatalf("unknown prompt error = %v; want JSON-RPC invalid params", err)
	}

	// Phase 7: a restart with every upstream dark still serves the prompts and
	// tools the previous process persisted.
	run.stop(t)
	alpha.close()
	beta.close()
	gamma.close()
	delta.close()

	dark := startBridgeProcess(t, ctx, configPath, httpAddress, operationsAddress)
	darkSession := connectBridgePrompts(t, ctx, dark.httpURL+"/mcp", nil)
	defer darkSession.Close()
	var restored []string
	waitFor(t, "the persisted prompts of a dark restart", func() bool {
		restored = listPromptNames(t, ctx, darkSession)
		return len(restored) == 2
	})
	if restored[0] != alphaPublicName || restored[1] != betaPublicName {
		t.Fatalf("restored prompts = %v; want the persisted public names %q and %q", restored, alphaPublicName, betaPublicName)
	}
	darkPayload := searchPayload(t, ctx, darkSession, "read")
	darkRevision, _ := catalogRevisionAndTool(t, darkPayload, "alpha")
	if darkRevision == "" {
		t.Fatal("a dark restart lost the persisted tool catalog")
	}
	if _, err := darkSession.GetPrompt(ctx, &mcp.GetPromptParams{Name: alphaPublicName}); err == nil {
		t.Fatal("a dark upstream prompt returned success")
	} else if strings.Contains(err.Error(), "raw-secret-transport-canary") {
		t.Fatalf("a dark restart leaked an upstream error: %v", err)
	}
	dark.stop(t)

	// Phase 8: the new binary loads an old tool-only snapshot normally and
	// publishes no prompt for it.
	oldSnapshotDirectory := filepath.Join(directory, "old-state")
	if err := os.MkdirAll(oldSnapshotDirectory, 0o700); err != nil {
		t.Fatalf("create old snapshot directory: %v", err)
	}
	encoded, err = os.ReadFile(filepath.Join(storageDirectory, "snapshot.json"))
	if err != nil {
		t.Fatalf("read the run's tool snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldSnapshotDirectory, "snapshot.json"), encoded, 0o600); err != nil {
		t.Fatalf("write the old tool-only snapshot: %v", err)
	}
	oldConfigPath := filepath.Join(directory, "old-bridge.yaml")
	writeBridgeConfigInterval(t, oldConfigPath, httpAddress, operationsAddress, oldSnapshotDirectory, embeddings.URL, "250ms", []bridgeServerSpec{
		{id: "alpha", endpoint: alpha.address()},
	})
	old := startBridgeProcess(t, ctx, oldConfigPath, httpAddress, operationsAddress)
	oldSession := connectBridgePrompts(t, ctx, old.httpURL+"/mcp", nil)
	defer oldSession.Close()
	oldPayload := searchPayload(t, ctx, oldSession, "read")
	if revision, _ := catalogRevisionAndTool(t, oldPayload, "alpha"); revision == "" {
		t.Fatal("an old tool-only snapshot did not serve its cached tool")
	}
	if names := listPromptNames(t, ctx, oldSession); len(names) != 0 {
		t.Fatalf("an old tool-only snapshot published prompts: %v", names)
	}
	old.stop(t)
}

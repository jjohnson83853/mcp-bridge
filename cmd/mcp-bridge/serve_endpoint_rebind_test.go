package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

// endpointFingerprintOf reproduces the service's endpoint identity digest
// (internal/service/reload.go), so the test compares the durable binding
// against the exact configured URL.
func endpointFingerprintOf(endpoint string) string {
	sum := sha256.Sum256([]byte(endpoint))
	return hex.EncodeToString(sum[:])
}

// serverAdvertises reports whether one search payload row names serverID with
// the expected public tool name.
func serverAdvertises(payload map[string]any, serverID, want string) bool {
	results, ok := payload["results"].([]any)
	if !ok {
		return false
	}
	for _, entry := range results {
		row, ok := entry.(map[string]any)
		if !ok || row["server"] != serverID {
			continue
		}
		return row["tool_name"] == want
	}
	return false
}

// searchFindsTool searches once and reports whether serverID advertises want.
func searchFindsTool(t *testing.T, ctx context.Context, session *mcp.ClientSession, query, serverID, want string) bool {
	t.Helper()
	return serverAdvertises(searchPayload(t, ctx, session, query), serverID, want)
}

// searchResultNames lists every tool name a search payload returned.
func searchResultNames(t *testing.T, payload map[string]any) []string {
	t.Helper()
	results, ok := payload["results"].([]any)
	if !ok {
		t.Fatalf("search payload results = %T", payload["results"])
	}
	names := make([]string, 0, len(results))
	for _, entry := range results {
		row, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := row["tool_name"].(string); ok {
			names = append(names, name)
		}
	}
	return names
}

// revisionOf reads the catalog revision a search payload was answered from.
func revisionOf(t *testing.T, payload map[string]any) string {
	t.Helper()
	revision, ok := payload["catalog_revision"].(string)
	if !ok || revision == "" {
		t.Fatalf("search payload catalog_revision = %v", payload["catalog_revision"])
	}
	return revision
}

// callCurrentTool invokes one catalog tool through the call_tool envelope with
// the revision a fresh search reports, and asserts the upstream echoed the
// exact forwarded arguments fragment. A refusal carrying the typed
// catalog_changed code means a newer generation published between the search
// and the call, so the retry re-reads the current revision; any other failure
// is fatal.
func callCurrentTool(t *testing.T, ctx context.Context, session *mcp.ClientSession, query, publicName string, arguments map[string]any, expectFragment string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		payload := searchPayload(t, ctx, session, query)
		result := callTool(t, ctx, session, catalog.ToolCallTool, map[string]any{
			"tool_name":        publicName,
			"catalog_revision": revisionOf(t, payload),
			"arguments":        arguments,
		})
		if !result.IsError {
			if len(result.Content) == 0 {
				t.Fatalf("call of %s returned no content: %#v", publicName, result)
			}
			text, ok := result.Content[0].(*mcp.TextContent)
			if !ok || !strings.Contains(text.Text, expectFragment) {
				t.Fatalf("call of %s content = %#v; want upstream fragment %q", publicName, result.Content, expectFragment)
			}
			return
		}
		envelope := structuredContent(t, result)
		if envelope["code"] != "catalog_changed" || time.Now().After(deadline) {
			t.Fatalf("call of %s failed: %#v", publicName, envelope)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// listAdvertisedTools returns every tool name one session's tools/list shows.
func listAdvertisedTools(t *testing.T, ctx context.Context, session *mcp.ClientSession) []string {
	t.Helper()
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	names := make([]string, 0, len(listed.Tools))
	for _, tool := range listed.Tools {
		names = append(names, tool.Name)
	}
	return names
}

// readBinding decodes the durable snapshot and returns one server's binding.
func readBinding(t *testing.T, snapshotPath, serverID string) storage.Binding {
	t.Helper()
	encoded, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var durable storage.Snapshot
	if err := json.Unmarshal(encoded, &durable); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	for _, binding := range durable.Bindings {
		if binding.ServerID == serverID {
			return binding
		}
	}
	t.Fatalf("snapshot has no binding for %q: %s", serverID, string(encoded))
	return storage.Binding{}
}

// TestBridgeRebindsServerEndpointOnRestartAndReload is the process-level
// acceptance of endpoint reconfiguration: a restart whose server endpoint
// changed rebinds (purging that server's cache) instead of failing startup,
// the durable binding adopts the new fingerprint, and a SIGHUP reload
// rebinds a running bridge without dropping it.
func TestBridgeRebindsServerEndpointOnRestartAndReload(t *testing.T) {
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)

	// The test process must survive the SIGHUP it sends; the bridge under test
	// registers its own handler for the same signal.
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)

	readSchema := `{"type":"object","properties":{"limit":{"type":"integer"}},"additionalProperties":true}`
	querySchema := `{"type":"object","properties":{"prefix":{"type":"string"}},"additionalProperties":true}`
	alpha1 := fakeUpstream(t, "read_v1", readSchema)
	beta := fakeUpstream(t, "query", querySchema)
	embeddings := fakeEmbeddings(t)

	directory := t.TempDir()
	configPath := filepath.Join(directory, "bridge.yaml")
	storageDirectory := filepath.Join(directory, "state")
	snapshotPath := filepath.Join(storageDirectory, storage.SnapshotFile)

	// runBridge starts one bridge process against the shared config path.
	runBridge := func() (context.Context, context.CancelFunc, chan int, *syncWriter, *syncWriter) {
		ctx, cancel := context.WithCancel(context.Background())
		stdout, stderr := &syncWriter{}, &syncWriter{}
		done := make(chan int, 1)
		go func() { done <- runContext(ctx, []string{"--config", configPath}, stdout, stderr) }()
		return ctx, cancel, done, stdout, stderr
	}
	// stopBridge cancels and joins one bridge run, asserting exit status 0.
	stopBridge := func(cancel context.CancelFunc, done chan int, stderr *syncWriter) {
		t.Helper()
		cancel()
		select {
		case status := <-done:
			if status != 0 {
				t.Fatalf("run status = %d; want 0 (stderr=%s)", status, stderr.String())
			}
		case <-time.After(30 * time.Second):
			t.Fatal("the process did not shut down after cancellation")
		}
	}
	// awaitReady polls the operations edge until both servers are ready.
	awaitReady := func(operationsAddress string) {
		t.Helper()
		operationsURL := "http://" + operationsAddress
		waitFor(t, "live operations listener", func() bool {
			status, body, ok := probeBody(t, operationsURL+"/livez")
			return ok && status == http.StatusOK && strings.Contains(body, "live")
		})
		waitFor(t, "readiness of alpha and beta", func() bool {
			status, body, ok := probeBody(t, operationsURL+"/readyz")
			return ok && status == http.StatusOK &&
				strings.Contains(body, `"status":"ready"`) &&
				strings.Contains(body, "alpha") && strings.Contains(body, "beta")
		})
	}
	// awaitDiscovery polls one server's advertised tool through search.
	awaitDiscovery := func(ctx context.Context, session *mcp.ClientSession, query, serverID, publicName string) {
		t.Helper()
		waitFor(t, "discovery of "+publicName, func() bool {
			return searchFindsTool(t, ctx, session, query, serverID, publicName)
		})
	}

	// ---- Phase 1: the bridge serves alpha (read_v1) and beta (query). ----
	httpAddress1, operationsAddress1 := freeAddress(t), freeAddress(t)
	writeBridgeConfig(t, configPath, httpAddress1, operationsAddress1, storageDirectory, embeddings.URL, []bridgeServerSpec{
		{id: "alpha", endpoint: alpha1.URL},
		{id: "beta", endpoint: beta.URL},
	})

	ctx1, cancel1, done1, _, stderr1 := runBridge()
	awaitReady(operationsAddress1)
	session1 := connectBridge(t, ctx1, "http://"+httpAddress1+"/mcp", serveClientToken)

	readV1 := core.PublicName("alpha", "read_v1")
	awaitDiscovery(ctx1, session1, "read", "alpha", readV1)
	callCurrentTool(t, ctx1, session1, "read", readV1, map[string]any{"limit": 2}, `{"limit":2}`)

	queryTool := core.PublicName("beta", "query")
	awaitDiscovery(ctx1, session1, "query", "beta", queryTool)
	callCurrentTool(t, ctx1, session1, "query", queryTool, map[string]any{"prefix": "x"}, `{"prefix":"x"}`)

	waitFor(t, "the durable snapshot to exist", func() bool {
		_, err := os.Stat(snapshotPath)
		return err == nil
	})
	session1.Close()
	stopBridge(cancel1, done1, stderr1)

	// ---- Phase 2: restart with alpha rebound to a fresh endpoint. ----
	alpha2 := fakeUpstream(t, "read_v2", readSchema)
	httpAddress2, operationsAddress2 := freeAddress(t), freeAddress(t)
	writeBridgeConfig(t, configPath, httpAddress2, operationsAddress2, storageDirectory, embeddings.URL, []bridgeServerSpec{
		{id: "alpha", endpoint: alpha2.URL},
		{id: "beta", endpoint: beta.URL},
	})

	_, cancel2, done2, stdout2, stderr2 := runBridge()
	awaitReady(operationsAddress2)

	// The bridge neither exited fatally nor logged the endpoint-change error:
	// the listener still serves and the run channel is silent.
	select {
	case status := <-done2:
		t.Fatalf("bridge exited during restart with status %d (stdout=%s stderr=%s)", status, stdout2.String(), stderr2.String())
	default:
	}
	if strings.Contains(stderr2.String(), "endpoint identity changed") || strings.Contains(stdout2.String(), "endpoint identity changed") {
		t.Fatalf("restart logged an endpoint-identity failure: %s | %s", stdout2.String(), stderr2.String())
	}

	ctx2, cancel2Client := context.WithCancel(context.Background())
	defer cancel2Client()
	session2 := connectBridge(t, ctx2, "http://"+httpAddress2+"/mcp", serveClientToken)
	defer session2.Close()

	readV2 := core.PublicName("alpha", "read_v2")
	awaitDiscovery(ctx2, session2, "read", "alpha", readV2)
	callCurrentTool(t, ctx2, session2, "read", readV2, map[string]any{"limit": 3}, `{"limit":3}`)

	// read_v1 of the abandoned endpoint is gone from search and the catalog.
	waitFor(t, "read_v1 absence in search", func() bool {
		payload := searchPayload(t, ctx2, session2, "read")
		return !serverAdvertises(payload, "alpha", readV1)
	})
	payload := searchPayload(t, ctx2, session2, "read")
	if serverAdvertises(payload, "alpha", readV1) {
		t.Fatalf("search still advertises the abandoned read_v1: %#v", payload["results"])
	}
	if strings.Contains(strings.Join(searchResultNames(t, payload), ","), "read_v1") {
		t.Fatalf("search results still mention read_v1: %v", searchResultNames(t, payload))
	}
	for _, name := range listAdvertisedTools(t, ctx2, session2) {
		if strings.Contains(name, "read_v1") {
			t.Fatalf("tools/list still advertises %q after the rebind", name)
		}
	}

	// beta was never rebound, so it is discoverable and callable as before.
	awaitDiscovery(ctx2, session2, "query", "beta", queryTool)
	callCurrentTool(t, ctx2, session2, "query", queryTool, map[string]any{"prefix": "y"}, `{"prefix":"y"}`)

	// The durable binding for alpha now names the alpha2 endpoint identity,
	// while beta's binding kept its unchanged endpoint identity.
	binding := readBinding(t, snapshotPath, "alpha")
	if binding.EndpointFingerprint != endpointFingerprintOf(alpha2.URL) {
		t.Fatalf("alpha binding fingerprint = %q; want the alpha2 identity %q", binding.EndpointFingerprint, endpointFingerprintOf(alpha2.URL))
	}
	if binding.EndpointFingerprint == endpointFingerprintOf(alpha1.URL) {
		t.Fatalf("alpha binding still names the abandoned endpoint")
	}
	if binding.Retired {
		t.Fatalf("alpha binding retired=%v; want a live rebind", binding.Retired)
	}
	if got := readBinding(t, snapshotPath, "beta").EndpointFingerprint; got != endpointFingerprintOf(beta.URL) {
		t.Fatalf("beta binding fingerprint = %q; want %q", got, endpointFingerprintOf(beta.URL))
	}

	// ---- Phase 3: SIGHUP reload rebinds alpha without a restart. ----
	alpha3 := fakeUpstream(t, "read_v3", readSchema)
	writeBridgeConfig(t, configPath, httpAddress2, operationsAddress2, storageDirectory, embeddings.URL, []bridgeServerSpec{
		{id: "alpha", endpoint: alpha3.URL},
		{id: "beta", endpoint: beta.URL},
	})
	sendReload(t)

	readV3 := core.PublicName("alpha", "read_v3")
	awaitDiscovery(ctx2, session2, "read", "alpha", readV3)
	callCurrentTool(t, ctx2, session2, "read", readV3, map[string]any{"limit": 4}, `{"limit":4}`)

	// The reload purged the previous generation's read_v2 and left beta alone.
	payload = searchPayload(t, ctx2, session2, "read")
	if serverAdvertises(payload, "alpha", readV2) {
		t.Fatalf("search still advertises read_v2 after the reload: %#v", payload["results"])
	}
	if got := readBinding(t, snapshotPath, "alpha").EndpointFingerprint; got != endpointFingerprintOf(alpha3.URL) {
		t.Fatalf("alpha binding fingerprint after reload = %q; want %q", got, endpointFingerprintOf(alpha3.URL))
	}
	callCurrentTool(t, ctx2, session2, "query", queryTool, map[string]any{"prefix": "z"}, `{"prefix":"z"}`)

	// The same process served the whole reload: the client session never
	// reconnected, and the operations edge is still the phase-2 listener.
	select {
	case status := <-done2:
		t.Fatalf("bridge exited during reload with status %d (stdout=%s stderr=%s)", status, stdout2.String(), stderr2.String())
	default:
	}
	awaitReady(operationsAddress2)

	stopBridge(cancel2, done2, stderr2)
	cancel2Client()
}

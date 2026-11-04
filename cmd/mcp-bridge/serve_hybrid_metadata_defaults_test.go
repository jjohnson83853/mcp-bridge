package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// metadataDefaultsUpstreamServer serves the alpha read tool with NO upstream
// metadata at all, so every corrected field a client observes must come from
// the configured server defaults, never from the backend.
func metadataDefaultsUpstreamServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "alpha", Version: "1"}, nil)
	server.AddTool(&mcp.Tool{
		Name:        "read",
		Description: "Read a file from disk",
		InputSchema: json.RawMessage(readToolSchema),
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

// metadataDefaultsDefaults is the configured alpha defaults patch.
const metadataDefaultsDefaults = `    tool_metadata:
      defaults:
        title: Corrected read
        annotations:
          readOnlyHint: true
          destructiveHint: false
`

// startMetadataDefaultsBridge mirrors startHybridBridgeWithUpstream but
// injects the alpha tool_metadata defaults block into the fixture config and
// also rewrites the config path on demand, so reload phases can change the
// defaults in place.
func startMetadataDefaultsBridge(t *testing.T, upstream *httptest.Server, configPath string, withDefaults bool) (context.Context, string) {
	t.Helper()
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)
	stdout := &syncWriter{}
	embeddings := fakeEmbeddings(t)
	httpAddress, operationsAddress := freeAddress(t), freeAddress(t)
	for attempts := 0; operationsAddress == httpAddress && attempts < 8; attempts++ {
		operationsAddress = freeAddress(t)
	}
	directory := filepath.Dir(configPath)
	writeBridgeConfig(t, configPath, httpAddress, operationsAddress, filepath.Join(directory, "state"), embeddings.URL,
		[]bridgeServerSpec{{id: "alpha", endpoint: upstream.URL}})
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read fixture config: %v", err)
	}
	config = []byte(strings.Replace(string(config), "allowed_origins: []", `allowed_origins: ["https://allowed.example"]`, 1))
	config = []byte(strings.Replace(string(config), "pins: []", "pins:\n  - {server: alpha, tool: read}\n", 1))
	if withDefaults {
		config = []byte(strings.Replace(string(config), "    endpoint: \""+upstream.URL+"\"\n",
			"    endpoint: \""+upstream.URL+"\"\n"+metadataDefaultsDefaults, 1))
	}
	if err := os.WriteFile(configPath, config, 0600); err != nil {
		t.Fatalf("write fixture config: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	stderr := &syncWriter{}
	done := make(chan int, 1)
	go func() { done <- runContext(ctx, []string{"--config", configPath}, stdout, stderr) }()
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("bridge stdout: %s", stdout.String())
			t.Logf("bridge stderr: %s", stderr.String())
		}
	})
	stop := func() {
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
	t.Cleanup(stop)
	waitFor(t, "readiness of the configured server", func() bool {
		status, body, ok := probeBody(t, "http://"+operationsAddress+"/readyz")
		return ok && status == http.StatusOK && strings.Contains(body, `"status":"ready"`) && strings.Contains(body, "alpha")
	})
	return ctx, "http://" + httpAddress
}

// TestServeHybridMetadataServerDefaults verifies on the real /mcp seam that
// the alpha defaults patch reaches the pinned alpha__read tool in direct
// tools/list and describe_tool while the built-ins stay untouched, and that
// removing the defaults on reload reverts the same session to the source view.
func TestServeHybridMetadataServerDefaults(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "bridge.yaml")
	upstream := metadataDefaultsUpstreamServer(t)

	// The test process must survive the SIGHUP it sends; the bridge registers
	// its own handler for the same signal.
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)

	ctx, httpURL := startMetadataDefaultsBridge(t, upstream, configPath, true)
	client := &http.Client{Timeout: 5 * time.Second}

	response, body := mcpSend(t, ctx, client, httpURL, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"metadata-defaults-client","version":"1"}}}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d; want 200 (body=%s)", response.StatusCode, body)
	}
	session := response.Header.Get("Mcp-Session-Id")
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
	var pinned, builtin *listedTool
	for index := range envelope.Result.Tools {
		switch {
		case envelope.Result.Tools[index].Name == "alpha__read":
			pinned = &envelope.Result.Tools[index]
		case envelope.Result.Tools[index].Name == "search_tools":
			builtin = &envelope.Result.Tools[index]
		}
	}
	if pinned == nil {
		t.Fatalf("tools/list omitted the pinned alpha__read tool: %s", payload)
	}
	if pinned.Title != "Corrected read" {
		t.Fatalf("tools/list title for alpha__read = %q; want the configured default %q (RED)", pinned.Title, "Corrected read")
	}
	if len(pinned.Annotations) == 0 {
		t.Fatal("tools/list omitted the default annotations for alpha__read")
	}
	var annotations struct {
		ReadOnly    *bool `json:"readOnlyHint"`
		Destructive *bool `json:"destructiveHint"`
	}
	if err := json.Unmarshal(pinned.Annotations, &annotations); err != nil {
		t.Fatalf("decode alpha__read annotations: %v (%s)", err, pinned.Annotations)
	}
	if annotations.ReadOnly == nil || !*annotations.ReadOnly || annotations.Destructive == nil || *annotations.Destructive {
		t.Fatalf("alpha__read did not carry the default hints: %s", pinned.Annotations)
	}
	if builtin == nil {
		t.Fatal("tools/list omitted search_tools")
	}
	if !strings.Contains(string(builtin.Annotations), `"readOnlyHint":true`) {
		t.Fatalf("built-in search_tools lost its classification: %s", builtin.Annotations)
	}
	if strings.Contains(string(builtin.Annotations), "Corrected") {
		t.Fatalf("server defaults leaked onto a built-in tool: %s", builtin.Annotations)
	}

	// Describe must keep the upstream (source) view and add the effective view
	// with per-field server_default provenance.
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
		CatalogRevision string `json:"catalog_revision"`
		Results         []struct {
			ToolName string `json:"tool_name"`
		} `json:"results"`
	}
	if err := json.Unmarshal(search.Result.StructuredContent, &searchPayload); err != nil {
		t.Fatalf("decode search payload: %v (%s)", err, search.Result.StructuredContent)
	}
	revision := searchPayload.CatalogRevision
	foundRead := false
	for _, result := range searchPayload.Results {
		if result.ToolName == "alpha__read" {
			foundRead = true
			break
		}
	}
	if !foundRead || len(revision) != 64 {
		t.Fatalf("search_tools did not return alpha__read with a revision: %s", search.Result.StructuredContent)
	}
	describe := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"describe_tool","arguments":{"tool_name":"alpha__read","catalog_revision":"` + revision + `"}}}`
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
		UpstreamMetadata   map[string]json.RawMessage `json:"upstream_metadata"`
		Metadata           map[string]json.RawMessage `json:"metadata"`
		MetadataOverridden []struct {
			Path  string `json:"path"`
			Layer string `json:"layer"`
		} `json:"metadata_override_fields"`
	}
	if err := json.Unmarshal(described.Result.StructuredContent, &describePayload); err != nil {
		t.Fatalf("decode describe payload: %v (%s)", err, described.Result.StructuredContent)
	}
	if describePayload.UpstreamMetadata != nil {
		t.Fatalf("describe upstream_metadata must stay the source view, which carries no metadata: %s", described.Result.StructuredContent)
	}
	if describePayload.Metadata == nil {
		t.Fatalf("describe omitted the effective metadata view: %s", described.Result.StructuredContent)
	}
	var effectiveAnnotations struct {
		ReadOnly *bool `json:"readOnlyHint"`
	}
	if err := json.Unmarshal(describePayload.Metadata["annotations"], &effectiveAnnotations); err != nil {
		t.Fatalf("decode effective annotations: %v (%s)", err, describePayload.Metadata["annotations"])
	}
	if effectiveAnnotations.ReadOnly == nil || !*effectiveAnnotations.ReadOnly {
		t.Fatalf("describe effective annotations lost the default hint: %s", described.Result.StructuredContent)
	}
	winningPaths := map[string]string{}
	for _, field := range describePayload.MetadataOverridden {
		if field.Layer != "server_default" {
			t.Fatalf("provenance layer = %q; want server_default", field.Layer)
		}
		winningPaths[field.Path] = field.Layer
	}
	for _, want := range []string{"title", "annotations.readOnlyHint", "annotations.destructiveHint"} {
		if _, ok := winningPaths[want]; !ok {
			t.Fatalf("metadata_override_fields omitted %q: %s", want, described.Result.StructuredContent)
		}
	}

	// Reload without the defaults: the same session re-lists the source view.
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read fixture config for reload: %v", err)
	}
	config = []byte(strings.Replace(string(config), metadataDefaultsDefaults, "", 1))
	if err := os.WriteFile(configPath, config, 0o600); err != nil {
		t.Fatalf("write fixture config for reload: %v", err)
	}
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatalf("raise SIGHUP: %v", err)
	}
	select {
	case <-hup:
	case <-time.After(10 * time.Second):
		t.Fatal("SIGHUP was not delivered to the test process")
	}
	waitFor(t, "effective view reverting to the source", func() bool {
		response, body = mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","id":5,"method":"tools/list","params":{}}`)
		if response.StatusCode != http.StatusOK {
			t.Logf("tools/list status %d", response.StatusCode)
			return false
		}
		var reloaded struct {
			Result *struct {
				Tools []listedTool `json:"tools"`
			} `json:"result"`
		}
		if err := json.Unmarshal(ssePayload(body), &reloaded); err != nil {
			t.Logf("tools/list decode: %v", err)
			return false
		}
		for index := range reloaded.Result.Tools {
			if reloaded.Result.Tools[index].Name == "alpha__read" {
				return reloaded.Result.Tools[index].Title == "" && len(reloaded.Result.Tools[index].Annotations) == 0
			}
		}
		return false
	})
}

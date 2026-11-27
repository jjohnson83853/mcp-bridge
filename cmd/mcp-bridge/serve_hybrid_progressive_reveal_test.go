package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServeHybridProgressiveRevealDiscovery(t *testing.T) {
	upstream := fakeUpstreamReadAndWriteTools(t)
	ctx, httpURL := startHybridBridgeWithProgressiveMetadataOverrides(t, upstream)
	client := &http.Client{Timeout: 5 * time.Second}

	initialize := func(name string) string {
		t.Helper()
		response, body := mcpSend(t, ctx, client, httpURL, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"`+name+`","version":"1"}}}`)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s initialize status = %d; want 200 (body=%s)", name, response.StatusCode, body)
		}
		session := response.Header.Get("Mcp-Session-Id")
		if session == "" {
			t.Fatalf("%s initialize omitted Mcp-Session-Id", name)
		}
		response, body = mcpSend(t, ctx, client, httpURL, session, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
		if response.StatusCode != http.StatusAccepted {
			t.Fatalf("%s initialized notification status = %d; want 202 (body=%s)", name, response.StatusCode, body)
		}
		return session
	}

	type envelope struct {
		Result *struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			Tools []struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				Annotations map[string]any `json:"annotations"`
			} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	decode := func(raw []byte) envelope {
		t.Helper()
		payload := raw
		if index := bytes.Index(payload, []byte("data: ")); index >= 0 {
			payload = payload[index+len("data: "):]
			if end := bytes.IndexByte(payload, '\n'); end >= 0 {
				payload = payload[:end]
			}
		}
		var decoded envelope
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("response is not a JSON-RPC envelope: %v (body=%s)", err, raw)
		}
		return decoded
	}
	call := func(session, request string) (int, envelope) {
		t.Helper()
		response, body := mcpSend(t, ctx, client, httpURL, session, request)
		return response.StatusCode, decode(body)
	}

	sessionA := initialize("progressive-reveal-a")
	sessionB := initialize("progressive-reveal-b")
	status, searchResult := call(sessionA, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search_tools","arguments":{"query":"file"}}}`)
	if status != http.StatusOK || searchResult.Error != nil || searchResult.Result == nil || searchResult.Result.IsError {
		t.Fatalf("search_tools failed: status=%d envelope=%+v", status, searchResult)
	}
	if len(searchResult.Result.Content) == 0 {
		t.Fatal("search_tools returned no response content")
	}
	var found struct {
		CatalogRevision string `json:"catalog_revision"`
		Results         []struct {
			ToolName string `json:"tool_name"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(searchResult.Result.Content[0].Text), &found); err != nil {
		t.Fatalf("decode search_tools content: %v (text=%q)", err, searchResult.Result.Content[0].Text)
	}
	if found.CatalogRevision == "" {
		t.Fatal("search_tools omitted catalog_revision")
	}
	targets := make(map[string]string, len(found.Results))
	for _, result := range found.Results {
		switch result.ToolName {
		case "alpha__read":
			targets["read"] = result.ToolName
		case "alpha__write":
			targets["write"] = result.ToolName
		}
	}
	for _, target := range []struct {
		upstreamName string
		toolName     string
	}{
		{upstreamName: "read", toolName: "alpha__read"},
		{upstreamName: "write", toolName: "alpha__write"},
	} {
		if targets[target.upstreamName] != target.toolName {
			t.Fatalf("search_tools omitted alpha/%s identity: %+v", target.upstreamName, found.Results)
		}
	}

	// Search results are not declared automatically.
	status, listedAfterSearch := call(sessionA, `{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`)
	if status != http.StatusOK || listedAfterSearch.Error != nil || listedAfterSearch.Result == nil {
		t.Fatalf("tools/list after search failed: status=%d envelope=%+v", status, listedAfterSearch)
	}
	listedNames := make(map[string]bool, len(listedAfterSearch.Result.Tools))
	for _, tool := range listedAfterSearch.Result.Tools {
		listedNames[tool.Name] = true
	}
	if !listedNames[targets["read"]] {
		t.Errorf("configured pin %q disappeared after search", targets["read"])
	}
	if listedNames[targets["write"]] {
		t.Fatalf("search automatically exposed unpinned result %q", targets["write"])
	}
	status, directBeforeExpose := call(sessionA, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"`+targets["write"]+`","arguments":{"path":"file.txt","data":"before-expose"}}}`)
	if status == http.StatusOK && directBeforeExpose.Error == nil && directBeforeExpose.Result != nil && !directBeforeExpose.Result.IsError {
		t.Fatalf("direct call to unexposed search result %q unexpectedly succeeded", targets["write"])
	}
	status, exposedA := call(sessionA, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"expose_tools","arguments":{"tool_names":["`+targets["write"]+`"]}}}`)
	if status != http.StatusOK || exposedA.Error != nil || exposedA.Result == nil || exposedA.Result.IsError {
		t.Fatalf("expose_tools in session A failed: status=%d envelope=%+v", status, exposedA)
	}
	status, listed := call(sessionA, `{"jsonrpc":"2.0","id":6,"method":"tools/list","params":{}}`)
	if status != http.StatusOK || listed.Error != nil || listed.Result == nil {
		t.Fatalf("tools/list after expose_tools failed: status=%d envelope=%+v", status, listed)
	}
	listedNames = make(map[string]bool, len(listed.Result.Tools))
	for _, tool := range listed.Result.Tools {
		listedNames[tool.Name] = true
	}
	for _, name := range targets {
		if !listedNames[name] {
			t.Errorf("explicitly exposed result %q was absent from tools/list", name)
		}
	}

	directID := 7
	for rawName, name := range targets {
		arguments := `{"path":"file.txt"}`
		if rawName == "write" {
			arguments = `{"path":"file.txt","data":"discovery"}`
		}
		status, direct := call(sessionA, `{"jsonrpc":"2.0","id":`+strconv.Itoa(directID)+`,"method":"tools/call","params":{"name":"`+name+`","arguments":`+arguments+`}}`)
		directID++
		if status != http.StatusOK || direct.Error != nil || direct.Result == nil || direct.Result.IsError {
			var isError bool
			var content any
			if direct.Result != nil {
				isError = direct.Result.IsError
				content = direct.Result.Content
			}
			t.Errorf("direct call to explicitly exposed alpha/%s (%q) failed: status=%d error=%+v isError=%t content=%#v", rawName, name, status, direct.Error, isError, content)
		}
	}

	// Exposure in A must not enroll B through the shared SDK name union.
	status, guessed := call(sessionB, `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"`+targets["write"]+`","arguments":{"path":"file.txt","data":"guess"}}}`)
	if status == http.StatusOK && guessed.Error == nil && guessed.Result != nil && !guessed.Result.IsError {
		t.Fatalf("session B called session A's exposed target %q without exposure", targets["write"])
	}

	// B can inspect the same target without declaring it.
	status, described := call(sessionB, `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"describe_tool","arguments":{"tool_name":"`+targets["write"]+`","catalog_revision":"`+found.CatalogRevision+`"}}}`)
	if status != http.StatusOK || described.Error != nil || described.Result == nil || described.Result.IsError {
		t.Fatalf("describe_tool failed: status=%d envelope=%+v", status, described)
	}

	var projectedWriteDescription string
	var projectedWriteAnnotations map[string]any
	for _, tool := range listed.Result.Tools {
		if tool.Name == targets["write"] {
			projectedWriteDescription = tool.Description
			projectedWriteAnnotations = tool.Annotations
			break
		}
	}
	if projectedWriteDescription != "Configured write description" {
		t.Fatalf("tools/list description = %q; want configured override", projectedWriteDescription)
	}
	if len(described.Result.Content) == 0 {
		t.Fatal("describe_tool returned no response content")
	}
	var describedMetadata struct {
		Description            string         `json:"description"`
		Pinned                 bool           `json:"pinned"`
		Metadata               map[string]any `json:"metadata"`
		UpstreamMetadata       map[string]any `json:"upstream_metadata"`
		MetadataOverrideFields []struct {
			Path  string `json:"path"`
			Layer string `json:"layer"`
		} `json:"metadata_override_fields"`
	}
	if err := json.Unmarshal([]byte(described.Result.Content[0].Text), &describedMetadata); err != nil {
		t.Fatalf("decode describe_tool content: %v (text=%q)", err, described.Result.Content[0].Text)
	}
	if describedMetadata.Pinned {
		t.Fatal("describe_tool fixture unexpectedly resolved a pinned tool")
	}
	if describedMetadata.Description != projectedWriteDescription {
		t.Errorf("describe_tool description = %q; want projected effective description %q", describedMetadata.Description, projectedWriteDescription)
	}
	if len(describedMetadata.MetadataOverrideFields) == 0 {
		t.Error("describe_tool omitted metadata_override_fields for a tool with configured overrides")
	}
	overridePaths := make(map[string]struct{}, len(describedMetadata.MetadataOverrideFields))
	for _, field := range describedMetadata.MetadataOverrideFields {
		overridePaths[field.Path] = struct{}{}
	}
	for _, path := range []string{"description", "annotations.readOnlyHint"} {
		if _, found := overridePaths[path]; !found {
			t.Errorf("describe_tool metadata_override_fields omitted %q: %#v", path, describedMetadata.MetadataOverrideFields)
		}
	}
	if len(describedMetadata.Metadata) == 0 {
		t.Error("describe_tool omitted effective metadata for a tool with configured overrides")
	}
	effectiveAnnotations, hasAnnotations := describedMetadata.Metadata["annotations"].(map[string]any)
	if !hasAnnotations || effectiveAnnotations["readOnlyHint"] != true || projectedWriteAnnotations["readOnlyHint"] != true {
		t.Errorf("describe_tool effective readOnlyHint = %#v; want the projected configured value true", describedMetadata.Metadata["annotations"])
	}
	if describedMetadata.UpstreamMetadata != nil {
		t.Errorf("describe_tool upstream_metadata = %#v; want unmodified empty source metadata", describedMetadata.UpstreamMetadata)
	}
	if reflect.DeepEqual(describedMetadata.Metadata, describedMetadata.UpstreamMetadata) {
		t.Error("describe_tool metadata did not reflect configured overrides")
	}
	status, listedB := call(sessionB, `{"jsonrpc":"2.0","id":10,"method":"tools/list","params":{}}`)
	if status != http.StatusOK || listedB.Error != nil || listedB.Result == nil {
		t.Fatalf("tools/list after describe failed: status=%d envelope=%+v", status, listedB)
	}
	for _, tool := range listedB.Result.Tools {
		if tool.Name == targets["write"] {
			t.Fatalf("describe_tool automatically exposed %q", targets["write"])
		}
	}
	status, exposedB := call(sessionB, `{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"expose_tools","arguments":{"tool_names":["`+targets["write"]+`"]}}}`)
	if status != http.StatusOK || exposedB.Error != nil || exposedB.Result == nil || exposedB.Result.IsError {
		t.Fatalf("expose_tools in session B failed: status=%d envelope=%+v", status, exposedB)
	}
	status, listedBAfterExpose := call(sessionB, `{"jsonrpc":"2.0","id":12,"method":"tools/list","params":{}}`)
	if status != http.StatusOK || listedBAfterExpose.Error != nil || listedBAfterExpose.Result == nil {
		t.Fatalf("tools/list after session B expose_tools failed: status=%d envelope=%+v", status, listedBAfterExpose)
	}
	for _, tool := range listedBAfterExpose.Result.Tools {
		if tool.Name == targets["write"] {
			status, directB := call(sessionB, `{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"`+targets["write"]+`","arguments":{"path":"file.txt","data":"owner-b"}}}`)
			if status != http.StatusOK || directB.Error != nil || directB.Result == nil || directB.Result.IsError {
				t.Fatalf("direct call after session B exposure failed: status=%d envelope=%+v", status, directB)
			}
			return
		}
	}
	t.Fatalf("explicit expose_tools did not declare %q in session B; listed=%s", targets["write"], strings.TrimSpace(string(mustJSON(t, listedBAfterExpose.Result.Tools))))
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal test diagnostic: %v", err)
	}
	return encoded
}

func startHybridBridgeWithProgressiveMetadataOverrides(
	t *testing.T,
	upstream *httptest.Server,
) (context.Context, string) {
	return startHybridBridgeWithProgressiveMetadataOverridesPageSize(t, upstream, 0)
}

func startHybridBridgeWithProgressiveMetadataOverridesPageSize(
	t *testing.T,
	upstream *httptest.Server,
	pageSize int,
) (context.Context, string) {
	t.Helper()
	return startHybridBridgeWithProgressiveMetadataOverridesPageSizeAndIdle(t, upstream, pageSize, 10*time.Minute)
}

func startHybridBridgeWithProgressiveMetadataOverridesPageSizeAndIdle(
	t *testing.T,
	upstream *httptest.Server,
	pageSize int,
	sessionIdle time.Duration,
) (context.Context, string) {
	t.Helper()
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)

	stdout := &syncWriter{}
	embeddings := fakeEmbeddings(t)
	httpAddress, operationsAddress := freeAddress(t), freeAddress(t)
	for attempts := 0; operationsAddress == httpAddress && attempts < 8; attempts++ {
		operationsAddress = freeAddress(t)
	}
	if operationsAddress == httpAddress {
		t.Fatal("could not choose distinct public and operations addresses")
	}

	directory := t.TempDir()
	configPath := filepath.Join(directory, "bridge.yaml")
	writeBridgeConfig(
		t,
		configPath,
		httpAddress,
		operationsAddress,
		filepath.Join(directory, "state"),
		embeddings.URL,
		[]bridgeServerSpec{{id: "alpha", endpoint: upstream.URL}},
	)
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read fixture config: %v", err)
	}
	configText := strings.Replace(string(config), "allowed_origins: []", `allowed_origins: ["https://allowed.example"]`, 1)
	configText = strings.Replace(configText, `session_idle_timeout: "10m"`, `session_idle_timeout: "`+sessionIdle.String()+`"`, 1)
	configText = strings.Replace(configText, "pins: []", "pins:\n  - {server: alpha, tool: read}\n", 1)
	serverEndpoint := "    endpoint: \"" + upstream.URL + "\"\n"
	configText = strings.Replace(configText, serverEndpoint, serverEndpoint+`    tool_metadata:
      tools:
        write:
          description: "Configured write description"
          annotations:
            readOnlyHint: true
`, 1)
	if err := os.WriteFile(configPath, []byte(configText), 0o600); err != nil {
		t.Fatalf("configure progressive reveal metadata overrides: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	stderr := &syncWriter{}
	done := make(chan error, 1)
	go func() {
		done <- serveWithToolPageSize(ctx, configPath, stdout, stderr, pageSize)
	}()
	stopped := false
	t.Cleanup(func() {
		if stopped {
			return
		}
		cancel()
		select {
		case err := <-done:
			stopped = true
			if err != nil {
				t.Fatalf("run error: %v (stderr=%s)", err, stderr.String())
			}
		case <-time.After(30 * time.Second):
			t.Fatal("the process did not shut down after cancellation")
		}
	})
	waitFor(t, "readiness of the configured server", func() bool {
		status, body, ok := probeBody(t, "http://"+operationsAddress+"/readyz")
		return ok && status == http.StatusOK && strings.Contains(body, `"status":"ready"`) && strings.Contains(body, "alpha")
	})
	return ctx, "http://" + httpAddress
}
func TestServeHybridProgressiveRevealPages(t *testing.T) {
	upstream := fakeUpstreamReadAndWriteTools(t)
	ctx, httpURL := startHybridBridgeWithProgressiveMetadataOverridesPageSize(t, upstream, 2)
	client := mcp.NewClient(&mcp.Implementation{Name: "progressive-page-client", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: httpURL + "/mcp",
		HTTPClient: &http.Client{Transport: progressiveRevealTransport{
			token: serveClientToken,
			base:  http.DefaultTransport,
		}},
		MaxRetries: -1,
	}, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
	if err != nil {
		t.Fatalf("connect bridge: %v", err)
	}
	defer session.Close()

	expected := map[string]bool{
		catalog.ToolSearchTools:    false,
		catalog.ToolDescribeTool:   false,
		catalog.ToolCallTool:       false,
		catalog.ToolRetrieveResult: false,
		catalog.ToolExposeTools:    false,
		"alpha__read":              false,
	}
	seen := make(map[string]bool, len(expected))
	cursor := ""
	pages := 0
	for {
		var params *mcp.ListToolsParams
		if cursor != "" {
			params = &mcp.ListToolsParams{Cursor: cursor}
		}
		page, err := session.ListTools(ctx, params)
		if err != nil {
			t.Fatalf("tools/list page %d: %v", pages+1, err)
		}
		pages++
		if len(page.Tools) > 2 {
			t.Fatalf("page %d has %d tools; want at most 2", pages, len(page.Tools))
		}
		for _, tool := range page.Tools {
			if seen[tool.Name] {
				t.Errorf("tools/list duplicated %q across pages", tool.Name)
			}
			seen[tool.Name] = true
		}
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	if pages < 2 {
		t.Fatalf("tools/list used %d page(s); want multiple pages", pages)
	}
	for name := range expected {
		if !seen[name] {
			t.Errorf("tools/list traversal omitted %q; got %v", name, seen)
		}
	}
	for name := range seen {
		if _, known := expected[name]; !known {
			t.Errorf("tools/list traversal returned unexpected tool %q", name)
		}
	}

	firstPage, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list first page before search: %v", err)
	}
	if firstPage.NextCursor == "" {
		t.Fatal("first page omitted cursor before search")
	}
	discovered, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      catalog.ToolSearchTools,
		Arguments: json.RawMessage(`{"query":"file"}`),
	})
	if err != nil {
		t.Fatalf("search_tools: %v", err)
	}
	if discovered == nil || discovered.IsError {
		t.Fatalf("search_tools failed: %+v", discovered)
	}
	postSearchNames := make(map[string]bool, len(firstPage.Tools))
	for _, tool := range firstPage.Tools {
		postSearchNames[tool.Name] = true
	}
	cursor = firstPage.NextCursor
	for cursor != "" {
		page, err := session.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			t.Fatalf("resume pre-search cursor after search_tools: %v", err)
		}
		if len(page.Tools) > 2 {
			t.Fatalf("post-search page has %d tools; want at most 2", len(page.Tools))
		}
		for _, tool := range page.Tools {
			if postSearchNames[tool.Name] {
				t.Errorf("post-search traversal duplicated %q", tool.Name)
			}
			postSearchNames[tool.Name] = true
			if tool.Name == "alpha__write" {
				t.Error("search result alpha__write appeared in a post-search tools/list page without explicit exposure")
			}
		}
		cursor = page.NextCursor
	}
	if len(postSearchNames) != len(expected) {
		t.Errorf("post-search traversal returned %d tools; want %d: %v", len(postSearchNames), len(expected), postSearchNames)
	}
	for name := range expected {
		if !postSearchNames[name] {
			t.Errorf("post-search traversal omitted %q; got %v", name, postSearchNames)
		}
	}
	for name := range postSearchNames {
		if _, known := expected[name]; !known {
			t.Errorf("post-search traversal returned unexpected tool %q", name)
		}
	}
}

type progressiveRevealTransport struct {
	token string
	base  http.RoundTripper
}

func (transport progressiveRevealTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set("Authorization", "Bearer "+transport.token)
	clone.Header.Set("Origin", "https://allowed.example")
	return transport.base.RoundTrip(clone)
}

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

// A failed descriptor validation must reject the whole candidate generation
// without touching the previously published generation (METADATA-r7): removals
// and re-registrations may happen only after every descriptor in the candidate
// is built successfully.
func TestPublishedKeepsPreviousGenerationWhenDescriptorFails(t *testing.T) {
	server := NewMCPServer()
	surface := &ToolSurface{runtime: NewRuntime()}
	if err := surface.Register(server); err != nil {
		t.Fatal(err)
	}

	good := core.ToolDefinition{
		RawName:     "good",
		Description: "good",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}
	first := &Assembly{pins: []core.ResolvedPin{
		{PinnedTool: core.PinnedTool{ServerID: "alpha", RawName: "good"}, Binding: &core.Binding{Def: good}},
	}}
	surface.Published(first)

	broken := core.ToolDefinition{
		RawName:     "broken",
		Description: "broken",
		InputSchema: json.RawMessage(`{"type":"object"}`),
		// A JSON array of non-object entries passes Metadata.Validate but
		// fails SDK icon unmarshal inside sdkToolDescriptor.
		Metadata: core.ToolMetadata{Icons: json.RawMessage(`[123]`)},
	}
	rejected := &Assembly{pins: []core.ResolvedPin{
		{PinnedTool: core.PinnedTool{ServerID: "alpha", RawName: "broken"}, Binding: &core.Binding{Def: broken}},
	}}
	surface.Published(rejected)

	client := connectPromptClient(t, server, nil)
	page, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"search_tools": true, "describe_tool": true, "call_tool": true, "alpha__good": true}
	got := make([]string, 0, len(page.Tools))
	for _, tool := range page.Tools {
		got = append(got, tool.Name)
		delete(want, tool.Name)
	}
	if len(want) != 0 || len(got) != 4 {
		t.Fatalf("published tools after rejected candidate = %v; missing %v", got, want)
	}
}

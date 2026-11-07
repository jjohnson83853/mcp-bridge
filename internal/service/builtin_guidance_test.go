package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http/httptest"
	"testing"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

const (
	searchToolsBaselineDescription = "Search the bridge's indexed tools. Tools declared in your tools list MUST be invoked directly instead of searching. Unpinned tools remain discoverable through this tool."
	describeToolGuidanceDescription = "Describe one indexed tool with its exact stored schema. DO NOT use this tool to discover input schemas for tools that are already declared in your tools list; invoke those tools directly. Use this tool only to inspect tool metadata, provenance, or unpinned tools."
	callToolGuidanceDescription    = "Call one indexed upstream tool through the bridge. Directly declared tools in your active tools list MUST be invoked directly instead of calling this multiplexer. Use this tool only to invoke unpinned upstream tools discovered via search_tools."
)

func TestBuiltinGuidanceSnapshotCoherence(t *testing.T) {
	server := NewMCPServer()
	_, surface := registerGuidanceSurface(t, server, guidanceAssembly())

	registryDescriptions := listBuiltinGuidance(t, connectPromptClient(t, server, nil))
	ownerDescriptions := listBuiltinGuidance(t, connectBuiltinGuidanceOwner(t, server, surface))
	if !equalBuiltinDescriptions(registryDescriptions, ownerDescriptions) {
		t.Fatalf("middleware-free registry descriptions = %#v; owner tools/list descriptions = %#v", registryDescriptions, ownerDescriptions)
	}
	want := map[string]string{
		"search_tools":  "Search the bridge's indexed tools. The following tools are already directly declared in your tools list and MUST be invoked directly instead of searching: alpha__read, zeta__write. Unpinned tools remain discoverable through this tool.",
		"describe_tool": describeToolGuidanceDescription,
		"call_tool":     callToolGuidanceDescription,
	}
	if !equalBuiltinDescriptions(registryDescriptions, want) {
		t.Fatalf("published builtin descriptions = %#v; want %#v", registryDescriptions, want)
	}
}

func TestBuiltinGuidanceDynamicPinInventory(t *testing.T) {
	got := publishedBuiltinGuidance(t, guidanceAssembly())["search_tools"]
	want := "Search the bridge's indexed tools. The following tools are already directly declared in your tools list and MUST be invoked directly instead of searching: alpha__read, zeta__write. Unpinned tools remain discoverable through this tool."
	if got != want {
		t.Fatalf("search_tools description = %q; want %q", got, want)
	}
}

func TestBuiltinGuidanceZeroPinBaseline(t *testing.T) {
	got := publishedBuiltinGuidance(t, &Assembly{})["search_tools"]
	if got != searchToolsBaselineDescription {
		t.Fatalf("zero-pin search_tools description = %q; want %q", got, searchToolsBaselineDescription)
	}
}

func TestBuiltinGuidanceDescribeAndCallDirections(t *testing.T) {
	got := publishedBuiltinGuidance(t, guidanceAssembly())
	if got["describe_tool"] != describeToolGuidanceDescription {
		t.Errorf("describe_tool description = %q; want %q", got["describe_tool"], describeToolGuidanceDescription)
	}
	if got["call_tool"] != callToolGuidanceDescription {
		t.Errorf("call_tool description = %q; want %q", got["call_tool"], callToolGuidanceDescription)
	}
}

func TestBuiltinGuidanceRegistryParity(t *testing.T) {
	server := NewMCPServer()
	_, surface := registerGuidanceSurface(t, server, guidanceAssembly())

	registryDescriptions := listBuiltinGuidance(t, connectPromptClient(t, server, nil))
	ownerDescriptions := listBuiltinGuidance(t, connectBuiltinGuidanceOwner(t, server, surface))
	if !equalBuiltinDescriptions(registryDescriptions, ownerDescriptions) {
		t.Fatalf("middleware-free registry descriptions = %#v; owner tools/list descriptions = %#v", registryDescriptions, ownerDescriptions)
	}
}

func TestPublishedRetainsBuiltinGuidanceWhenDescriptorFails(t *testing.T) {
	registryServer := NewMCPServer()
	registryAssembly := guidanceAssembly()
	_, registrySurface := registerGuidanceSurface(t, registryServer, registryAssembly)
	registryClient := connectPromptClient(t, registryServer, nil)

	ownerServer := NewMCPServer()
	ownerAssembly := guidanceAssembly()
	_, ownerSurface := registerGuidanceSurface(t, ownerServer, ownerAssembly)
	ownerClient := connectBuiltinGuidanceOwner(t, ownerServer, ownerSurface)
	previousRegistry := listBuiltinGuidance(t, registryClient)
	previousOwner := listBuiltinGuidance(t, ownerClient)

	broken := core.ToolDefinition{
		RawName:     "broken",
		Description: "broken",
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Metadata:    core.ToolMetadata{Icons: json.RawMessage(`[123]`)},
	}
	rejected := &Assembly{
		generation: registryAssembly.Generation(),
		pins: []core.ResolvedPin{{
			PinnedTool: core.PinnedTool{ServerID: "alpha", RawName: "broken"},
			Binding:    &core.Binding{ServerID: "alpha", RawName: "broken", Def: broken},
		}},
	}
	if err := registrySurface.Published(rejected); err == nil {
		t.Fatal("registry surface accepted a candidate with an invalid SDK descriptor")
	}
	if err := ownerSurface.Published(rejected); err == nil {
		t.Fatal("owner surface accepted a candidate with an invalid SDK descriptor")
	}

	if got := listBuiltinGuidance(t, registryClient); !equalBuiltinDescriptions(got, previousRegistry) {
		t.Errorf("registry descriptions after rejected publication = %#v; want previous %#v", got, previousRegistry)
	}
	if got := listBuiltinGuidance(t, ownerClient); !equalBuiltinDescriptions(got, previousOwner) {
		t.Errorf("owner tools/list descriptions after rejected publication = %#v; want previous %#v", got, previousOwner)
	}
}

func guidanceAssembly() *Assembly {
	return &Assembly{pins: []core.ResolvedPin{
		{
			PinnedTool: core.PinnedTool{ServerID: "zeta", RawName: "write"},
			Binding: &core.Binding{ServerID: "zeta", RawName: "write", Def: core.ToolDefinition{
				RawName: "write", InputSchema: json.RawMessage(`{"type":"object"}`),
			}},
		},
		{
			PinnedTool: core.PinnedTool{ServerID: "alpha", RawName: "read"},
			Binding: &core.Binding{ServerID: "alpha", RawName: "read", Def: core.ToolDefinition{
				RawName: "read", InputSchema: json.RawMessage(`{"type":"object"}`),
			}},
		},
		{PinnedTool: core.PinnedTool{ServerID: "unresolved", RawName: "ignored"}},
	}}
}

func registerGuidanceSurface(t *testing.T, server *mcp.Server, assembly *Assembly) (*Runtime, *ToolSurface) {
	t.Helper()
	runtime := NewRuntime()
	if err := runtime.accept(assembly, nil); err != nil {
		t.Fatalf("accept guidance assembly: %v", err)
	}
	surface := &ToolSurface{runtime: runtime}
	if err := surface.Register(server); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := surface.Published(assembly); err != nil {
		t.Fatalf("Published: %v", err)
	}
	return runtime, surface
}

func publishedBuiltinGuidance(t *testing.T, assembly *Assembly) map[string]string {
	t.Helper()
	server := NewMCPServer()
	registerGuidanceSurface(t, server, assembly)
	return listBuiltinGuidance(t, connectPromptClient(t, server, nil))
}

func connectBuiltinGuidanceOwner(t *testing.T, server *mcp.Server, surface *ToolSurface) *mcp.ClientSession {
	t.Helper()
	const token = "builtin-guidance-test-token"
	inbound := NewInboundHandler(
		NewStreamableHandler(server),
		[][32]byte{sha256.Sum256([]byte(token))},
		InboundOptions{ToolSurface: surface},
	)
	httpServer := httptest.NewServer(inbound)
	t.Cleanup(httpServer.Close)
	return connectProgressiveDiscoveryClient(t, httpServer, token)
}

func listBuiltinGuidance(t *testing.T, client *mcp.ClientSession) map[string]string {
	t.Helper()
	page, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	return builtinGuidanceDescriptions(page.Tools)
}

func builtinGuidanceDescriptions(tools []*mcp.Tool) map[string]string {
	descriptions := make(map[string]string, 3)
	for _, tool := range tools {
		if tool.Name == "search_tools" || tool.Name == "describe_tool" || tool.Name == "call_tool" {
			descriptions[tool.Name] = tool.Description
		}
	}
	return descriptions
}

func equalBuiltinDescriptions(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for name, description := range left {
		if right[name] != description {
			return false
		}
	}
	return true
}

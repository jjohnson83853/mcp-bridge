package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/search"
)

const pinnedToolBenchmarkSchema = `{"type":"object","properties":{"query":{"type":"string","minLength":1},"limit":{"type":"integer","minimum":1,"maximum":50}},"required":["query"],"additionalProperties":false}`

var pinnedToolBenchmarkResult = json.RawMessage(`{"content":[{"type":"text","text":"ok"}],"isError":false}`)

type pinnedToolBenchmarkUpstream struct{}

func (pinnedToolBenchmarkUpstream) CallTool(context.Context, string, string, string, json.RawMessage) (json.RawMessage, *core.UpstreamOutcome) {
	return pinnedToolBenchmarkResult, nil
}

func BenchmarkPinnedToolCallDescriptorComparison(b *testing.B) {
	const ownerReferences = 8
	b.Setenv("PROGRESSIVE_ALPHA_SECRET", "benchmark-secret")
	b.Setenv("PROGRESSIVE_CLIENT_TOKEN", "benchmark-token")

	candidate, err := config.Parse([]byte(fmt.Sprintf(progressiveConfig, "shared")), "pinned-tool-benchmark.yaml")
	if err != nil {
		b.Fatal(err)
	}
	upstream := pinnedToolBenchmarkUpstream{}
	runtime := NewRuntime()
	assembly, err := runtime.Reload(candidate, ReloadDeps{
		Discover: func(config.Server) ([]core.ToolDefinition, error) {
			return []core.ToolDefinition{{
				RawName:     "shared",
				Description: "Search benchmark records by query and optional result limit.",
				InputSchema: json.RawMessage(pinnedToolBenchmarkSchema),
			}}, nil
		},
		NewClient:           func(config.Server) (core.Upstream, error) { return upstream, nil },
		ModelIdentityDigest: strings.Repeat("a", 64),
		Now:                 func() time.Time { return time.Unix(1, 0) },
	})
	if err != nil {
		b.Fatal(err)
	}

	surface, err := NewToolSurface(runtime, ToolSurfaceConfig{
		Model: search.ModelIdentity{Provider: "benchmark", Model: "projection", Dimensions: 1, FormatVersion: "v1"},
	})
	if err != nil {
		b.Fatal(err)
	}
	if err := surface.Register(NewMCPServer()); err != nil {
		b.Fatal(err)
	}
	if err := surface.Published(assembly); err != nil {
		b.Fatal(err)
	}
	for i := range ownerReferences {
		owner, err := surface.newLiveOwner(fmt.Sprintf("benchmark-session-%d", i))
		if err != nil {
			b.Fatal(err)
		}
		if err := surface.publishOwnerProjection(owner, assembly); err != nil {
			b.Fatal(err)
		}
	}

	const toolName = "alpha__shared"
	coordinator := surface.projectionCoordinator()
	coordinator.barrier.Lock()
	entry := coordinator.union[toolName]
	if entry == nil {
		coordinator.barrier.Unlock()
		b.Fatal("registered pinned tool is missing")
	}
	if got := len(entry.references); got != ownerReferences+1 {
		coordinator.barrier.Unlock()
		b.Fatalf("registered owner references = %d; want %d", got, ownerReferences+1)
	}
	descriptor := entry.descriptor
	coordinator.barrier.Unlock()

	request := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Name:      toolName,
		Arguments: json.RawMessage(`{"query":"invoice","limit":10}`),
	}}
	ctx := context.Background()
	b.Run("equal-descriptor", func(b *testing.B) {
		handler := surface.pinnedToolHandler(toolName, descriptor)
		if result, err := handler(ctx, request); err != nil || result == nil || result.IsError {
			b.Fatalf("matching pinned call = (%#v, %v)", result, err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			if _, err := handler(ctx, request); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("changed-descriptor", func(b *testing.B) {
		changed := *descriptor
		changed.Description += " changed"
		handler := surface.pinnedToolHandler(toolName, &changed)
		if result, err := handler(ctx, request); err != nil || result == nil || !result.IsError {
			b.Fatalf("changed pinned call = (%#v, %v); want a stale registration rejection", result, err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			if _, err := handler(ctx, request); err != nil {
				b.Fatal(err)
			}
		}
	})
}

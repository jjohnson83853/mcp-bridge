package upstream_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/upstream"
)

// TestDiscoverToolMetadata verifies that discovery converts the SDK-normalized
// upstream metadata (title, icons, annotations, _meta) onto each definition,
// preserves explicit false pointer hints, and leaves metadata-absent tools
// with zero-value metadata.
func TestDiscoverToolMetadata(t *testing.T) {
	destructiveFalse := false
	openWorldFalse := false
	server := mcp.NewServer(&mcp.Implementation{Name: "peer", Version: "1"}, nil)
	server.AddTool(&mcp.Tool{
		Name:        "annotated",
		Description: "annotated tool",
		Title:       "Annotated Tool",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		Annotations: &mcp.ToolAnnotations{
			Title:           "Annotated",
			ReadOnlyHint:    true,
			DestructiveHint: &destructiveFalse,
			IdempotentHint:  true,
			OpenWorldHint:   &openWorldFalse,
		},
		Icons: []mcp.Icon{{Source: "https://example.test/annotated.svg", MIMEType: "image/svg+xml", Sizes: []string{"any"}}},
		Meta:  mcp.Meta{"audience": []any{"user"}},
	}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
	})
	server.AddTool(&mcp.Tool{
		Name:        "plain",
		Description: "plain tool",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
	})
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true},
	))
	t.Cleanup(httpServer.Close)

	dedicated, err := upstream.NewDedicated(upstream.DedicatedOptions{
		Logger: observe.NewLogger(nil, nil),
		Server: upstream.ServerOptions{
			ID:             "alpha",
			Endpoint:       httpServer.URL,
			CallTimeout:    5 * time.Second,
			ConnectTimeout: 5 * time.Second,
		},
	})
	if err != nil {
		t.Fatalf("NewDedicated: %v", err)
	}
	t.Cleanup(func() { _ = dedicated.Close() })

	definitions, err := dedicated.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	byName := make(map[string]core.ToolDefinition, len(definitions))
	for _, def := range definitions {
		byName[def.RawName] = def
	}

	annotated, ok := byName["annotated"]
	if !ok {
		t.Fatalf("missing annotated tool in discovered definitions")
	}
	metadata := annotated.Metadata
	if metadata.Title != "Annotated Tool" {
		t.Fatalf("discovered metadata title = %q; want %q", metadata.Title, "Annotated Tool")
	}
	if len(metadata.Icons) == 0 {
		t.Fatal("discovered metadata lost the upstream icons")
	}
	var icons []mcp.Icon
	if err := json.Unmarshal(metadata.Icons, &icons); err != nil {
		t.Fatalf("discovered icons are not a JSON icon array: %v (%s)", err, metadata.Icons)
	}
	if len(icons) != 1 || icons[0].Source != "https://example.test/annotated.svg" {
		t.Fatalf("discovered icons = %s; want the upstream icon source", metadata.Icons)
	}
	var annotations struct {
		Title       string `json:"title"`
		ReadOnly    *bool  `json:"readOnlyHint"`
		Destructive *bool  `json:"destructiveHint"`
		Idempotent  *bool  `json:"idempotentHint"`
		OpenWorld   *bool  `json:"openWorldHint"`
	}
	if err := json.Unmarshal(metadata.Annotations, &annotations); err != nil {
		t.Fatalf("discovered annotations are not JSON: %v (%s)", err, metadata.Annotations)
	}
	if annotations.Title != "Annotated" {
		t.Fatalf("discovered annotation title = %q; want %q", annotations.Title, "Annotated")
	}
	if annotations.ReadOnly == nil || !*annotations.ReadOnly {
		t.Fatalf("discovered annotations lost readOnlyHint true: %s", metadata.Annotations)
	}
	// Explicit false pointer hints must survive as false, not be dropped.
	if annotations.Destructive == nil || *annotations.Destructive {
		t.Fatalf("discovered annotations lost explicit destructiveHint false: %s", metadata.Annotations)
	}
	if annotations.OpenWorld == nil || *annotations.OpenWorld {
		t.Fatalf("discovered annotations lost explicit openWorldHint false: %s", metadata.Annotations)
	}
	if annotations.Idempotent == nil || !*annotations.Idempotent {
		t.Fatalf("discovered annotations lost idempotentHint true: %s", metadata.Annotations)
	}
	if len(metadata.Meta) == 0 {
		t.Fatal("discovered metadata lost the supported _meta keys")
	}
	var meta map[string]any
	if err := json.Unmarshal(metadata.Meta, &meta); err != nil {
		t.Fatalf("discovered _meta is not a JSON object: %v (%s)", err, metadata.Meta)
	}
	if !reflect.DeepEqual(meta["audience"], []any{"user"}) {
		t.Fatalf("discovered _meta = %s; want the upstream audience entry", metadata.Meta)
	}

	plain, ok := byName["plain"]
	if !ok {
		t.Fatalf("missing plain tool in discovered definitions")
	}
	if !reflect.DeepEqual(plain.Metadata, core.ToolMetadata{}) {
		t.Fatalf("metadata-absent tool discovered with metadata %+v; want zero value", plain.Metadata)
	}
}
func TestDiscoverToolMetadataRequiresLogger(t *testing.T) {
	dedicated, err := upstream.NewDedicated(upstream.DedicatedOptions{
		Server: upstream.ServerOptions{ID: "alpha", Endpoint: "http://127.0.0.1:1"},
	})
	if err == nil || dedicated != nil {
		t.Fatalf("NewDedicated without logger = (%v, %v); want nil and an error", dedicated, err)
	}
}

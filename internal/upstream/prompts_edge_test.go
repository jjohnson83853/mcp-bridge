package upstream

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDiscoverPromptsCapabilityPaginationAndFailure(t *testing.T) {
	t.Run("non-capable upstream issues no prompt request", func(t *testing.T) {
		var promptRequests atomic.Int32
		server := mcp.NewServer(&mcp.Implementation{Name: "peer", Version: "1"}, nil)
		inner := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
		httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read request: %v", err)
			}
			if bytes.Contains(body, []byte("\"method\":\"prompts/list\"")) {
				promptRequests.Add(1)
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			inner.ServeHTTP(w, r)
		}))
		defer httpServer.Close()
		options := ServerOptions{ID: "test", Endpoint: httpServer.URL, ConnectTimeout: time.Second}
		assertDedicatedRequiresLogger(t, options)
		d, err := NewDedicated(DedicatedOptions{Logger: observe.NewLogger(nil, nil), Server: options})
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		got, err := d.DiscoverPrompts(context.Background())
		if err != nil {
			t.Fatalf("DiscoverPrompts error = %v, want success", err)
		}
		if len(got) != 0 {
			t.Fatalf("prompts = %v, want empty", got)
		}
		if got := promptRequests.Load(); got != 0 {
			t.Fatalf("prompt requests = %d, want 0", got)
		}
	})
	t.Run("all pages sorted by raw name", func(t *testing.T) {
		var cursors []string
		pages := map[string]*mcp.ListPromptsResult{
			"":     {Prompts: []*mcp.Prompt{{Name: "zeta"}}, NextCursor: "next"},
			"next": {Prompts: []*mcp.Prompt{{Name: "alpha"}}},
		}
		got, err := discoverPromptPages(context.Background(), "server", func(_ context.Context, cursor string) (*mcp.ListPromptsResult, error) {
			cursors = append(cursors, cursor)
			return pages[cursor], nil
		})
		if err != nil {
			t.Fatalf("discovery error = %v, want success", err)
		}
		if !reflect.DeepEqual(cursors, []string{"", "next"}) {
			t.Fatalf("cursors = %v, want [\"\" \"next\"]", cursors)
		}
		gotNames := []string{got[0].Name, got[1].Name}
		if !reflect.DeepEqual(gotNames, []string{"alpha", "zeta"}) {
			t.Fatalf("prompt names = %v, want [\"alpha\" \"zeta\"]", gotNames)
		}
	})
	t.Run("page error returns no partial set", func(t *testing.T) {
		calls := 0
		got, err := discoverPromptPages(context.Background(), "server", func(_ context.Context, cursor string) (*mcp.ListPromptsResult, error) {
			calls++
			if cursor == "next" {
				return nil, errors.New("page failed")
			}
			return &mcp.ListPromptsResult{Prompts: []*mcp.Prompt{{Name: "partial"}}, NextCursor: "next"}, nil
		})
		if err == nil {
			t.Fatalf("discovery error = %v, want page failure", err)
		}
		if got != nil {
			t.Fatalf("prompts = %v, want nil on page failure", got)
		}
		if calls != 2 {
			t.Fatalf("page calls = %d, want 2", calls)
		}
	})
	t.Run("nameless prompt fails without partial set", func(t *testing.T) {
		got, err := discoverPromptPages(context.Background(), "server", func(_ context.Context, _ string) (*mcp.ListPromptsResult, error) {
			return &mcp.ListPromptsResult{Prompts: []*mcp.Prompt{{Name: "valid"}, {}}}, nil
		})
		if err == nil {
			t.Fatalf("discovery error = %v, want nameless prompt failure", err)
		}
		if got != nil {
			t.Fatalf("prompts = %v, want nil on malformed page", got)
		}
	})
	t.Run("repeated cursor fails without partial set", func(t *testing.T) {
		calls := 0
		got, err := discoverPromptPages(context.Background(), "server", func(_ context.Context, _ string) (*mcp.ListPromptsResult, error) {
			calls++
			return &mcp.ListPromptsResult{Prompts: []*mcp.Prompt{{Name: "partial"}}, NextCursor: "repeat"}, nil
		})
		if err == nil {
			t.Fatalf("discovery error = %v, want non-advancing cursor failure", err)
		}
		if got != nil {
			t.Fatalf("prompts = %v, want nil on cursor failure", got)
		}
		if calls != 2 {
			t.Fatalf("page calls = %d, want 2", calls)
		}
	})
}

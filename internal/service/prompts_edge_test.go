package service

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

func TestPromptRefreshLKGAcrossRestartAndOutage(t *testing.T) {
	cfg, err := config.Parse([]byte("schema_version: 1\nservers:\n  - id: alpha\n    endpoint: https://alpha.invalid/mcp\npins: []\n"), filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	prompt := storage.Prompt{ServerID: "alpha", RawName: "hello", PublicName: "p_opaque", Description: "greeting"}
	deps := ReloadDeps{
		Discover:            func(config.Server) ([]core.ToolDefinition, error) { return nil, nil },
		DiscoverPrompts:     func(config.Server) ([]storage.Prompt, error) { return []storage.Prompt{prompt}, nil },
		NewClient:           func(config.Server) (core.Upstream, error) { return &core.UpstreamRecorder{}, nil },
		PersistPrompts:      store.SavePrompts,
		LoadPrompts:         store.LoadPrompts,
		Now:                 time.Now,
		ModelIdentityDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	first, err := NewRuntime().Reload(cfg, deps)
	if err != nil {
		t.Fatalf("initial reload error = %v", err)
	}
	if got := first.Prompts(); len(got) != 1 || got[0].RawName != "hello" {
		t.Fatalf("initial prompts = %#v, want mirrored prompt", got)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	deps.LoadPrompts = store.LoadPrompts
	deps.PersistPrompts = store.SavePrompts
	deps.DiscoverPrompts = func(config.Server) ([]storage.Prompt, error) { return nil, errors.New("upstream outage") }
	runtime := NewRuntime()
	restarted, err := runtime.AdmitStartup(cfg, deps, nil)
	if err != nil {
		t.Fatalf("startup admission error = %v", err)
	}
	if got := restarted.Prompts(); len(got) != 1 || got[0].RawName != "hello" {
		t.Fatalf("restored prompts = %#v, want persisted prompt", got)
	}
	refreshed, err := runtime.Refresh(cfg, RefreshDeps{ReloadDeps: deps, Reason: func(config.Server, error) string { return RefreshReasonUnavailable }})
	if err != nil {
		t.Fatalf("outage refresh error = %v", err)
	}
	if got := refreshed.Prompts(); len(got) != 1 || got[0].RawName != "hello" {
		t.Fatalf("prompts after outage = %#v, want last-known-good prompt", got)
	}
}
func TestPromptRefreshPublishesStableOrdering(t *testing.T) {
	cfg, err := config.Parse([]byte("schema_version: 1\nservers:\n  - id: zeta\n    endpoint: https://zeta.invalid/mcp\n  - id: alpha\n    endpoint: https://alpha.invalid/mcp\npins: []\n"), filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	prompts := map[string][]storage.Prompt{
		"zeta":  {{ServerID: "zeta", RawName: "b"}, {ServerID: "zeta", RawName: "a"}},
		"alpha": {{ServerID: "alpha", RawName: "b"}, {ServerID: "alpha", RawName: "a"}},
	}
	deps := ReloadDeps{
		Discover:            func(config.Server) ([]core.ToolDefinition, error) { return nil, nil },
		DiscoverPrompts:     func(server config.Server) ([]storage.Prompt, error) { return prompts[server.ID], nil },
		NewClient:           func(config.Server) (core.Upstream, error) { return &core.UpstreamRecorder{}, nil },
		Now:                 time.Now,
		ModelIdentityDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	want := []string{"alpha/a", "alpha/b", "zeta/a", "zeta/b"}
	runtime := NewRuntime()
	first, err := runtime.Reload(cfg, deps)
	if err != nil {
		t.Fatalf("initial reload error = %v", err)
	}
	if got := promptKeys(first.Prompts()); !reflect.DeepEqual(got, want) {
		t.Fatalf("prompt order = %v, want %v", got, want)
	}
	second, err := runtime.Refresh(cfg, RefreshDeps{ReloadDeps: deps, Reason: func(config.Server, error) string { return RefreshReasonUnavailable }})
	if err != nil {
		t.Fatalf("refresh error = %v", err)
	}
	if got := promptKeys(second.Prompts()); !reflect.DeepEqual(got, want) {
		t.Fatalf("refreshed prompt order = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(second.Prompts(), first.Prompts()) {
		t.Fatalf("unchanged refresh reordered prompts: got %#v, before %#v", second.Prompts(), first.Prompts())
	}
}

func promptKeys(prompts []storage.Prompt) []string {
	keys := make([]string, 0, len(prompts))
	for _, prompt := range prompts {
		keys = append(keys, prompt.ServerID+"/"+prompt.RawName)
	}
	return keys
}

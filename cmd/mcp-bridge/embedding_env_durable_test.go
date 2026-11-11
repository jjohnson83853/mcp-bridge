package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

func TestEmbeddingEnvDurableIdentity(t *testing.T) {
	for _, field := range []string{"model", "base_url"} {
		t.Run(field, func(t *testing.T) {
			t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
			t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)
			upstream := fakeUpstream(t, "read", `{"type":"object"}`)
			provider := fakeEmbeddings(t)
			other := fakeEmbeddings(t)
			path := filepath.Join(t.TempDir(), "bridge.yaml")
			writeBridgeConfig(t, path, freeAddress(t), freeAddress(t), t.TempDir(), provider.URL, []bridgeServerSpec{{id: "alpha", endpoint: upstream.URL}})
			literal, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			load := func(doc []byte) *config.Config {
				t.Helper()
				c, err := config.Parse(doc, path)
				if err != nil {
					t.Fatalf("restart native reference admission: %v", err)
				}
				return c
			}
			depsFor := func(c *config.Config) service.ReloadDeps {
				t.Helper()
				settings, err := c.Command()
				if err != nil {
					t.Fatal(err)
				}
				model, p, err := newEmbeddingProvider(c, settings.Embedding)
				if err != nil {
					t.Fatal(err)
				}
				holder := newConfigHolder()
				holder.stage(c, settings)
				registry := newUpstreamRegistry(holder, observe.NewLogger(nil, nil))
				return service.ReloadDeps{Discover: registry.discover, NewClient: registry.newClient, Close: registry.close, Now: time.Now, ModelIdentityDigest: model.Digest(), Embed: embedder(p, model, settings.Embedding), Persist: func(*storage.Snapshot) error { return nil }}
			}
			old := load(literal)
			oldDeps := depsFor(old)
			runtime := service.NewRuntime()
			assembly, err := runtime.Reload(old, oldDeps)
			if err != nil {
				t.Fatalf("literal snapshot setup: %v", err)
			}
			previous := assembly.Snapshot()
			if len(previous.Embeddings) == 0 {
				t.Fatal("literal snapshot has no vectors")
			}
			// Existing literal envelopes remain admissible and retain matching vectors.
			restored, err := service.NewRuntime().AdmitStartup(old, oldDeps, previous)
			if err != nil {
				t.Fatal(err)
			}
			if len(restored.Snapshot().Embeddings) != len(previous.Embeddings) {
				t.Fatal("old literal snapshot vectors lost")
			}
			oldText := `model: "test-model"`
			value := "test-model"
			if field == "base_url" {
				oldText = "base_url: \"" + provider.URL + "\""
				value = provider.URL
			}
			document := []byte(strings.Replace(string(literal), oldText, field+": {env: EMBEDDING_DURABLE_VALUE}", 1))
			if string(document) == string(literal) {
				t.Fatal("fixture replacement failed")
			}
			t.Setenv("EMBEDDING_DURABLE_VALUE", value)
			first := load(document)
			firstDeps := depsFor(first)
			matching, err := service.NewRuntime().AdmitStartup(first, firstDeps, previous)
			if err != nil {
				t.Fatal(err)
			}
			if len(matching.Snapshot().Embeddings) != len(previous.Embeddings) {
				t.Fatal("matching effective identity did not reuse vectors")
			}
			fingerprint := first.Fingerprint()
			rotated := "rotated-model"
			if field == "base_url" {
				rotated = other.URL
			}
			t.Setenv("EMBEDDING_DURABLE_VALUE", rotated)
			next := load(document)
			nextDeps := depsFor(next)
			if next.Fingerprint() != fingerprint {
				t.Fatal("raw fingerprint changed on value-only rotation")
			}
			if nextDeps.ModelIdentityDigest == firstDeps.ModelIdentityDigest {
				t.Fatal("effective identity digest unchanged")
			}
			cold := service.NewRuntime()
			startup, err := cold.AdmitStartup(next, nextDeps, matching.Snapshot())
			if err != nil {
				t.Fatal(err)
			}
			if len(startup.Snapshot().Embeddings) != 0 {
				t.Fatal("startup reused stale vectors")
			}
			rebuilt, err := cold.Reload(next, nextDeps)
			if err != nil {
				t.Fatal(err)
			}
			if len(rebuilt.Snapshot().Embeddings) == 0 {
				t.Fatal("fresh vectors not published")
			}
			for _, row := range rebuilt.Snapshot().Embeddings {
				if row.ModelIdentityDigest != nextDeps.ModelIdentityDigest {
					t.Fatal("published vector has stale digest")
				}
			}
		})
	}
}

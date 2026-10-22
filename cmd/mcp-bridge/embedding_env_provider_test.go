package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
)

func TestEmbeddingEnvProviderBytes(t *testing.T) {
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)
	modelValue := " model: 'quoted' # ${literal}\n"
	t.Setenv("EMBEDDING_PROVIDER_MODEL", modelValue)
	requests := make(chan map[string]any, 1)
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		body["request_path"] = r.URL.Path
		body["authorization"] = r.Header.Get("Authorization")
		requests <- body
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"index": 0, "embedding": []float64{1, 0, 0, 0, 0, 0, 0, 0}}}}); err != nil {
			t.Error(err)
		}
	}))
	defer local.Close()
	t.Setenv("EMBEDDING_PROVIDER_URL", local.URL+"/selected/v1")
	upstream := fakeUpstream(t, "read", `{"type":"object"}`)
	path := filepath.Join(t.TempDir(), "bridge.yaml")
	writeBridgeConfig(t, path, freeAddress(t), freeAddress(t), t.TempDir(), "https://literal.invalid", []bridgeServerSpec{{id: "alpha", endpoint: upstream.URL}})
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc := strings.Replace(string(raw), `model: "test-model"`, "model: {env: EMBEDDING_PROVIDER_MODEL}", 1)
	doc = strings.Replace(doc, `base_url: "https://literal.invalid"`, "base_url: {env: EMBEDDING_PROVIDER_URL}", 1)
	cfg, err := config.Parse([]byte(doc), path)
	if err != nil {
		t.Fatalf("provider native reference admission: %v", err)
	}
	t.Setenv("EMBEDDING_PROVIDER_MODEL", "mutated")
	t.Setenv("EMBEDDING_PROVIDER_URL", local.URL+"/wrong")
	settings, err := cfg.Command()
	if err != nil {
		t.Fatal(err)
	}
	model, provider, err := newEmbeddingProvider(cfg, settings.Embedding)
	if err != nil {
		t.Fatal(err)
	}
	if model.Model != modelValue || model.BaseURL != local.URL+"/selected/v1" {
		t.Fatal("startup identity did not use snapshotted bytes")
	}
	vectors, err := provider.Embed(context.Background(), []string{"document"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 1 || len(vectors[0]) != serveDimensions {
		t.Fatal("local fixed provider response not consumed")
	}
	select {
	case request := <-requests:
		if request["model"] != modelValue {
			t.Errorf("provider model bytes=%q want=%q", request["model"], modelValue)
		}
		if request["request_path"] != "/selected/v1/embeddings" {
			t.Errorf("provider effective URL path=%v", request["request_path"])
		}
		if request["authorization"] != "Bearer "+serveEmbeddingKey {
			t.Error("api_key compatibility changed")
		}
	default:
		t.Fatal("effective URL did not reach selected local provider")
	}
}

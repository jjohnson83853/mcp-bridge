package config_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
)

func TestEmbeddingEnvExistingGrammar(t *testing.T) {
	commandEnv(t)
	file := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(file, []byte("file-key\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, node, want string
		valid            bool
	}{
		{"api env", "{env: COMMAND_FIXTURE_EMBEDDING_KEY}", "embedding-secret-value", true},
		{"api file", fmt.Sprintf("{file: %q}", file), "file-key", true},
		{"api literal", `"literal-key"`, "", false},
		{"api extra", "{env: COMMAND_FIXTURE_EMBEDDING_KEY, extra: rejected}", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := strings.Replace(commandFixture, "api_key: {env: COMMAND_FIXTURE_EMBEDDING_KEY}", "api_key: "+tc.node, 1)
			cfg, err := config.Parse([]byte(doc), "/tmp/embedding-compat.yaml")
			if !tc.valid {
				if err == nil {
					t.Fatal("invalid api_key admitted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			key, err := cfg.EmbeddingCredential()
			if err != nil {
				t.Fatal(err)
			}
			if key != tc.want {
				t.Fatal("existing credential resolution changed")
			}
			settings, err := cfg.Command()
			if err != nil {
				t.Fatal(err)
			}
			if settings.Embedding.Model != "operator-model" || settings.Embedding.BaseURL != "https://embeddings.invalid/v1" {
				t.Fatal("literal typed settings changed")
			}
		})
	}
	for _, tc := range []struct{ name, old, replacement string }{
		{"provider", `provider: "openai-compatible"`, "provider: {env: COMMAND_FIXTURE_EMBEDDING_KEY}"},
		{"revision", `model_revision: "r7"`, "model_revision: {env: COMMAND_FIXTURE_EMBEDDING_KEY}"},
		{"endpoint", `endpoint: "https://alpha.invalid/mcp"`, "endpoint: {env: COMMAND_FIXTURE_EMBEDDING_KEY}"},
		{"unknown embedding", `dimensions: 512`, "dimensions: 512\n  unknown_embedding: {env: COMMAND_FIXTURE_EMBEDDING_KEY}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.Parse([]byte(strings.Replace(commandFixture, tc.old, tc.replacement, 1)), "/tmp/embedding-compat.yaml")
			if err == nil {
				t.Fatal("unrelated grammar broadened")
			}
		})
	}
}

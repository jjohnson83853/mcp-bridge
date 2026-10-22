package config_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
)

func embeddingEnvDocument(model, url string) string {
	s := strings.Replace(commandFixture, `model: "operator-model"`, "model: "+model, 1)
	return strings.Replace(s, `base_url: "https://embeddings.invalid/v1"`, "base_url: "+url, 1)
}

func TestEmbeddingEnvNativeSnapshot(t *testing.T) {
	for _, fields := range []string{"model", "base_url", "both"} {
		t.Run(fields, func(t *testing.T) {
			commandEnv(t)
			model, url := `"operator-model"`, `"https://embeddings.invalid/v1"`
			wantModel, wantURL := "operator-model", "https://embeddings.invalid/v1"
			if fields != "base_url" {
				model = "{env: EMBEDDING_ENV_MODEL}"
				wantModel = "  model: 'quoted' # $ ${unchanged} {braces}\n\t"
			}
			if fields != "model" {
				url = "{env: EMBEDDING_ENV_URL}"
				wantURL = "https://example.invalid/v1?x=%20"
			}
			t.Setenv("EMBEDDING_ENV_MODEL", wantModel)
			t.Setenv("EMBEDDING_ENV_URL", wantURL)
			doc := embeddingEnvDocument(model, url)
			cfg, err := config.Parse([]byte(doc), "/tmp/embedding-env.yaml")
			if err != nil {
				t.Fatalf("native reference admission: %v", err)
			}
			rendered := fmt.Sprintf("%v %+v %#v", cfg, cfg, cfg)
			if fields != "base_url" && strings.Contains(rendered, wantModel) {
				t.Error("Config formatting leaked effective model")
			}
			if fields != "model" && strings.Contains(rendered, wantURL) {
				t.Error("Config formatting leaked effective URL")
			}
			raw := map[string]any{}
			for k, v := range cfg.Embedding {
				raw[k] = v
			}
			fingerprint := cfg.Fingerprint()
			t.Setenv("EMBEDDING_ENV_MODEL", "rotated")
			t.Setenv("EMBEDDING_ENV_URL", "https://rotated.invalid/v1")
			settings, err := cfg.Command()
			if err != nil {
				t.Fatal(err)
			}
			if settings.Embedding.Model != wantModel || settings.Embedding.BaseURL != wantURL {
				t.Fatalf("load snapshot bytes changed: %#v", settings.Embedding)
			}
			if !reflect.DeepEqual(raw, cfg.Embedding) {
				t.Fatal("Command mutated raw embedding")
			}
			next, err := config.Parse([]byte(doc), "/tmp/embedding-env.yaml")
			if err != nil {
				t.Fatal(err)
			}
			if next.Fingerprint() != fingerprint {
				t.Fatal("value rotation changed raw fingerprint")
			}
			if fields != "base_url" && !reflect.DeepEqual(cfg.Embedding["model"], map[string]any{"env": "EMBEDDING_ENV_MODEL"}) {
				t.Fatal("raw model reference replaced")
			}
			if fields != "model" && !reflect.DeepEqual(cfg.Embedding["base_url"], map[string]any{"env": "EMBEDDING_ENV_URL"}) {
				t.Fatal("raw URL reference replaced")
			}
			nextSettings, err := next.Command()
			if err != nil {
				t.Fatal(err)
			}
			if fields != "base_url" && nextSettings.Embedding.Model != "rotated" {
				t.Fatal("new load did not resolve new model")
			}
			if fields != "model" && nextSettings.Embedding.BaseURL != "https://rotated.invalid/v1" {
				t.Fatal("new load did not resolve new URL")
			}
		})
	}
}

func TestEmbeddingEnvModelByteFidelity(t *testing.T) {
	for _, value := range []string{" ", "\t\n", "line\n", "a: b # 'quoted' ${X} $() {env: OTHER}", "模型"} {
		t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
			commandEnv(t)
			t.Setenv("EMBEDDING_ENV_MODEL", value)
			cfg, err := config.Parse([]byte(embeddingEnvDocument("{env: EMBEDDING_ENV_MODEL}", `"https://embeddings.invalid/v1"`)), "/tmp/embedding-env.yaml")
			if err != nil {
				t.Fatalf("nonempty model admission: %v", err)
			}
			got, err := cfg.Command()
			if err != nil {
				t.Fatal(err)
			}
			if got.Embedding.Model != value {
				t.Fatalf("model bytes=%q want=%q", got.Embedding.Model, value)
			}
		})
	}
}

func TestEmbeddingEnvDiagnostics(t *testing.T) {
	cases := []struct{ name, node, code, property, shape, reason, forbidden string }{
		{"extra key", "{env: EMBEDDING_ENV_MISSING, other: secret}", "reference_shape", "reference", "mapping", "embedding value must be a literal string or an env reference", "secret"},
		{"file", "{file: unsafe-file}", "reference_shape", "reference", "mapping", "embedding value must be a literal string or an env reference", "unsafe-file"},
		{"number", "{env: 123}", "reference_shape", "reference", "mapping", "embedding value must be a literal string or an env reference", "123"},
		{"nested", "{env: [unsafe]}", "reference_shape", "reference", "mapping", "embedding value must be a literal string or an env reference", "unsafe"},
		{"null", "{env: null}", "reference_shape", "reference", "mapping", "embedding value must be a literal string or an env reference", ""},
		{"boolean", "{env: true}", "reference_shape", "reference", "mapping", "embedding value must be a literal string or an env reference", ""},
		{"empty mapping", "{}", "reference_shape", "reference", "mapping", "embedding value must be a literal string or an env reference", ""},
		{"whitespace name", `{env: " \t"}`, "reference_env_name", "name", "invalid_identifier", "invalid environment variable name (name withheld)", ""},
		{"invalid start", `{env: "9UNSAFE"}`, "reference_env_name", "name", "invalid_identifier", "invalid environment variable name (name withheld)", "9UNSAFE"},
		{"empty name", `{env: ""}`, "reference_env_name", "name", "invalid_identifier", "invalid environment variable name (name withheld)", ""},
		{"space name", `{env: "BAD NAME"}`, "reference_env_name", "name", "invalid_identifier", "invalid environment variable name (name withheld)", "BAD NAME"},
		{"expression", `{env: "${UNSAFE_SENTINEL}"}`, "reference_literal_required", "reference", "literal", "environment references cannot contain interpolation or commands", "${UNSAFE_SENTINEL}"},
		{"command", `{env: "$(UNSAFE_SENTINEL)"}`, "reference_literal_required", "reference", "literal", "environment references cannot contain interpolation or commands", "$(UNSAFE_SENTINEL)"},
		{"missing", "{env: EMBEDDING_ENV_MISSING}", "reference_unresolved", "reference", "missing", "environment variable is not set", ""},
		{"empty", "{env: EMBEDDING_ENV_EMPTY}", "reference_unresolved", "reference", "empty", "environment variable is empty", ""},
	}
	for _, field := range []string{"model", "base_url"} {
		for _, tc := range cases {
			t.Run(field+"/"+tc.name, func(t *testing.T) {
				commandEnv(t)
				t.Setenv("EMBEDDING_ENV_EMPTY", "")
				t.Setenv("EMBEDDING_ENV_MISSING", "")
				if err := os.Unsetenv("EMBEDDING_ENV_MISSING"); err != nil {
					t.Fatal(err)
				}
				model, url := `"operator-model"`, `"https://embeddings.invalid/v1"`
				if field == "model" {
					model = tc.node
				} else {
					url = tc.node
				}
				cfg, err := config.Parse([]byte(embeddingEnvDocument(model, url)), "/tmp/embedding-env.yaml")
				if err == nil {
					t.Fatal("invalid reference admitted")
				}
				if cfg != nil {
					t.Error("partial Config returned")
				}
				var diagnostics *config.Diagnostics
				if !errors.As(err, &diagnostics) {
					t.Fatalf("not Diagnostics: %v", err)
				}
				found := false
				for _, d := range diagnostics.Records {
					if d.Key == "embedding."+field {
						found = true
						if d.Path != "/tmp/embedding-env.yaml" || d.Code != tc.code || d.Property != tc.property || d.Shape != tc.shape || d.Reason != tc.reason {
							t.Errorf("diagnostic=%+v want code=%s property=%s shape=%s reason=%s", d, tc.code, tc.property, tc.shape, tc.reason)
						}
					}
				}
				if !found {
					t.Errorf("missing field attribution: %v", err)
				}
				if tc.forbidden != "" && strings.Contains(err.Error(), tc.forbidden) {
					t.Error("unsafe reference text leaked")
				}
			})
		}
	}
}

func TestEmbeddingEnvEffectiveURLDiagnostics(t *testing.T) {
	for _, value := range []string{" https://example.invalid/v1", "https://example.invalid/v1 ", "https://example.invalid/\tpath", "https://example.invalid/v1\r\n", "ftp://example.invalid", "SENTINEL-URL: invalid"} {
		t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
			commandEnv(t)
			t.Setenv("EMBEDDING_ENV_URL", value)
			_, err := config.Parse([]byte(embeddingEnvDocument(`"operator-model"`, "{env: EMBEDDING_ENV_URL}")), "/tmp/embedding-env.yaml")
			var ds *config.Diagnostics
			if !errors.As(err, &ds) {
				t.Fatalf("want URL diagnostic, got %v", err)
			}
			found := false
			for _, d := range ds.Records {
				if d.Key == "embedding.base_url" && d.Code == "url_invalid" && d.Property == "absolute_http_url" && d.Shape == "invalid_url" && d.Reason == "invalid absolute HTTP(S) URL" {
					found = true
				}
			}
			if !found {
				t.Errorf("effective URL metadata: %v", err)
			}
			escaped, _ := json.Marshal(value)
			for _, text := range []string{value, string(escaped[1 : len(escaped)-1])} {
				if strings.Contains(err.Error(), text) {
					t.Error("URL value leaked")
				}
			}
		})
	}
}

func TestEmbeddingEnvLiteralAndDirectConfig(t *testing.T) {
	commandEnv(t)
	for _, present := range []bool{false, true} {
		t.Run(fmt.Sprint(present), func(t *testing.T) {
			t.Setenv("EMBEDDING_ENV_MODEL", "ignored")
			if !present {
				if err := os.Unsetenv("EMBEDDING_ENV_MODEL"); err != nil {
					t.Fatal(err)
				}
			}
			cfg, err := config.Parse([]byte(embeddingEnvDocument(`"{env: EMBEDDING_ENV_MODEL}"`, `https://embeddings.invalid/v1`)), "/tmp/embedding-env.yaml")
			if err != nil {
				t.Fatal(err)
			}
			settings, err := cfg.Command()
			if err != nil {
				t.Fatal(err)
			}
			if settings.Embedding.Model != "{env: EMBEDDING_ENV_MODEL}" {
				t.Fatal("quoted model was resolved")
			}
			direct := *cfg
			direct.Embedding = map[string]any{}
			for k, v := range cfg.Embedding {
				direct.Embedding[k] = v
			}
			direct.Embedding["model"] = "direct-literal"
			// Construct a fresh Config without any parsed private snapshot.
			fresh := config.Config{SchemaVersion: direct.SchemaVersion, HTTP: direct.HTTP, Operations: direct.Operations, Storage: direct.Storage, Catalog: direct.Catalog, Search: direct.Search, Embedding: direct.Embedding, UpstreamDefaults: direct.UpstreamDefaults}
			got, err := fresh.Command()
			if err != nil {
				t.Fatal(err)
			}
			if got.Embedding.Model != "direct-literal" {
				t.Fatal("direct literal changed")
			}
			fresh.Embedding["model"] = map[string]any{"env": "EMBEDDING_ENV_MODEL"}
			if _, err := fresh.Command(); err == nil {
				t.Fatal("direct unresolved map lazily resolved")
			}
			_, err = config.Parse([]byte(embeddingEnvDocument(`operator-model`, `"{env: EMBEDDING_ENV_MODEL}"`)), "/tmp/embedding-env.yaml")
			if err == nil || !strings.Contains(err.Error(), "url_invalid") {
				t.Fatalf("quoted URL must fail ordinary URL validation: %v", err)
			}
		})
	}
}

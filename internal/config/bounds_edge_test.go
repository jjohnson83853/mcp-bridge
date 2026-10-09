package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const boundsEdgeBase = `schema_version: 1
servers:
  - id: alpha
    endpoint: https://alpha.invalid/mcp
pins: []
`

func TestParseRejectsOutOfRangeFields(t *testing.T) {
	cases := []struct{ name, yaml, field string }{
		{"http max request bytes", "http:\n  max_request_bytes: 0\n", "max_request_bytes"},
		{"http max request bytes ceiling", "http:\n  max_request_bytes: 1048577\n", "max_request_bytes"},
		{"http max sessions", "http:\n  max_sessions: 0\n", "max_sessions"},
		{"storage snapshot bytes", "storage:\n  max_snapshot_bytes: 0\n", "max_snapshot_bytes"},
		{"catalog tools per server", "catalog:\n  max_tools_per_server: 0\n", "max_tools_per_server"},
		{"catalog tools total", "catalog:\n  max_tools_total: 0\n", "max_tools_total"},
		{"catalog pages", "catalog:\n  max_pages_per_server: 0\n", "max_pages_per_server"},
		{"catalog schema bytes", "catalog:\n  max_schema_bytes: 0\n", "max_schema_bytes"},
		{"search top-k", "search:\n  default_top_k: 0\n", "default_top_k"},
		{"search max top-k", "search:\n  max_top_k: 51\n", "max_top_k"},
		{"embedding dimensions", "embedding:\n  dimensions: 0\n", "dimensions"},
		{"embedding batch size", "embedding:\n  batch_size: 129\n", "batch_size"},
		{"embedding concurrency", "embedding:\n  max_concurrency: 0\n", "max_concurrency"},
		{"embedding response bytes", "embedding:\n  max_response_bytes: 0\n", "max_response_bytes"},
		{"upstream breaker open duration", "upstream_defaults:\n  breaker:\n    open_duration: 0s\n", "open_duration"},
		{"search invalid weight sum", "search:\n  keyword_weight: 0.4\n  embedding_weight: 0.5\n", "search weights"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := strings.Replace(boundsEdgeBase, "pins: []", tc.yaml+"pins: []", 1)
			_, err := Parse([]byte(input), filepath.Join(t.TempDir(), "config.yaml"))
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.field)) {
				t.Fatalf("Parse error = %v, want field/class %q", err, tc.field)
			}
		})
	}
}

func TestParseCredentialFileBoundaries(t *testing.T) {
	cases := []struct {
		name    string
		content []byte
		special string
		want    string
	}{
		{"missing", nil, "missing", "secret file"},
		{"directory", nil, "directory", "secret file"},
		{"empty", []byte{}, "file", "nonempty"},
		{"oversized", []byte(strings.Repeat("x", 8193)), "file", "8KiB"},
		{"carriage return", []byte("a\rb"), "file", "CR/LF/NUL"},
		{"line feed", []byte("a\nb"), "file", "CR/LF/NUL"},
		{"nul", []byte{'a', 0, 'b'}, "file", "CR/LF/NUL"},
		{"two final newlines", []byte("token\n\n"), "file", "CR/LF/NUL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			ref := "secret"
			switch tc.special {
			case "missing":
			case "directory":
				if err := os.Mkdir(filepath.Join(dir, ref), 0700); err != nil {
					t.Fatal(err)
				}
			default:
				if err := os.WriteFile(filepath.Join(dir, ref), tc.content, 0600); err != nil {
					t.Fatal(err)
				}
			}
			input := strings.Replace(boundsEdgeBase, "endpoint: https://alpha.invalid/mcp", fmt.Sprintf("endpoint: https://alpha.invalid/mcp\n    credentials: {bearer: {file: %s}}", ref), 1)
			_, err := Parse([]byte(input), filepath.Join(dir, "config.yaml"))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Parse error = %v, want %q", err, tc.want)
			}
		})
	}
	t.Run("symlink rejected", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		if err := os.WriteFile(target, []byte("token"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(dir, "secret")); err != nil {
			t.Fatal(err)
		}
		input := strings.Replace(boundsEdgeBase, "endpoint: https://alpha.invalid/mcp", "endpoint: https://alpha.invalid/mcp\n    credentials: {bearer: {file: secret}}", 1)
		_, err := Parse([]byte(input), filepath.Join(dir, "config.yaml"))
		if err == nil || !strings.Contains(err.Error(), "secret file") {
			t.Fatalf("Parse error = %v, want symlink rejection", err)
		}
	})
	t.Run("permissions rejected", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "secret")
		if err := os.WriteFile(path, []byte("token"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0644); err != nil {
			t.Fatal(err)
		}
		input := strings.Replace(boundsEdgeBase, "endpoint: https://alpha.invalid/mcp", "endpoint: https://alpha.invalid/mcp\n    credentials: {bearer: {file: secret}}", 1)
		_, err := Parse([]byte(input), filepath.Join(dir, "config.yaml"))
		if err == nil || !strings.Contains(err.Error(), "secret file") {
			t.Fatalf("Parse error = %v, want permission rejection", err)
		}
	})
}

func TestEndpointNormalizationContributesCanonicalFingerprint(t *testing.T) {
	parse := func(endpoint string) *Config {
		t.Helper()
		input := strings.Replace(boundsEdgeBase, "https://alpha.invalid/mcp", endpoint, 1)
		cfg, err := Parse([]byte(input), t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}
	first := parse("HTTPS://ALPHA.INVALID/mcp")
	second := parse("https://alpha.invalid/mcp")
	if first.Servers[0].Endpoint != "https://alpha.invalid/mcp" || first.Fingerprint() != second.Fingerprint() {
		t.Fatalf("normalized endpoint/fingerprint mismatch: %q", first.Servers[0].Endpoint)
	}
	if parse("https://alpha.invalid/mcp/").Fingerprint() == second.Fingerprint() {
		t.Fatal("trailing slash unexpectedly ignored by fingerprint")
	}
}

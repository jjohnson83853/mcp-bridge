package config

import (
	"strings"
	"testing"
)

func TestCredentialAccessorsRejectUnknownOrMalformedReferences(t *testing.T) {
	cases := []struct {
		name   string
		cfg    Config
		server string
		want   string
	}{
		{"unknown server", Config{}, "unknown", "unknown server"},
		{"headers not mapping", Config{Servers: []Server{{ID: "alpha", Credentials: map[string]any{"headers": "bad"}}}}, "alpha", "headers must be a mapping"},
		{"invalid header name", Config{Servers: []Server{{ID: "alpha", Credentials: map[string]any{"headers": map[string]any{"bad header": map[string]any{"env": "TOKEN"}}}}}}, "alpha", "header name is invalid"},
		{"missing resolved bearer", Config{Servers: []Server{{ID: "alpha", Credentials: map[string]any{"bearer": map[string]any{"env": "TOKEN"}}}}}, "alpha", "does not resolve"},
		{"non-reference bearer", Config{Servers: []Server{{ID: "alpha", Credentials: map[string]any{"bearer": "plain"}}}}, "alpha", "exactly one secret"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.cfg.ServerCredentials(tc.server)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ServerCredentials error = %v, want %q", err, tc.want)
			}
		})
	}
	if _, err := (Config{}).ServerLimits("missing"); err == nil || !strings.Contains(err.Error(), "unknown server") {
		t.Fatalf("ServerLimits unknown server error = %v", err)
	}
}

func TestEmbeddingCredentialResolutionFailures(t *testing.T) {
	for _, tc := range []struct {
		name      string
		embedding map[string]any
		resolved  map[string]string
		want      string
	}{
		{"multiple references", map[string]any{"api_key": map[string]any{"env": "A", "file": "b"}}, nil, "exactly one"},
		{"unresolved reference", map[string]any{"api_key": map[string]any{"env": "TOKEN"}}, nil, "does not resolve"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (Config{Embedding: tc.embedding, resolved: tc.resolved}).EmbeddingCredential()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("EmbeddingCredential error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestCredentialFingerprintFailureAndSuccess(t *testing.T) {
	cfg := Config{Servers: []Server{{ID: "alpha", Credentials: map[string]any{"bearer": map[string]any{"env": "TOKEN"}}}}, resolved: map[string]string{"env:TOKEN": "secret"}}
	if _, ok := cfg.ServerCredentialFingerprint("missing"); ok {
		t.Fatal("fingerprint exists for unknown server")
	}
	if _, ok := (Config{Servers: []Server{{ID: "alpha", Credentials: map[string]any{"bearer": "literal"}}}}).ServerCredentialFingerprint("alpha"); ok {
		t.Fatal("fingerprint exists for malformed credential reference")
	}
	if _, ok := (Config{Servers: []Server{{ID: "alpha", Credentials: map[string]any{"bearer": map[string]any{"env": "TOKEN"}}}}}).ServerCredentialFingerprint("alpha"); ok {
		t.Fatal("fingerprint exists for unresolved credential")
	}
	fingerprint, ok := cfg.ServerCredentialFingerprint("alpha")
	if !ok || len(fingerprint) != 64 {
		t.Fatalf("fingerprint = %q, ok=%t", fingerprint, ok)
	}
}

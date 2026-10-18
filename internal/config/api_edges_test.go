package config

import (
	"testing"
)

// TestCredentialAccessorsRejectUnknownOrMalformedReferences migrates the C-1
// direct credential guards: config=<memory>, full key, validated owner, finite
// code/property/shape and static reasons, with references read only from valid
// single-reference shapes (spec §5.2).
func TestCredentialAccessorsRejectUnknownOrMalformedReferences(t *testing.T) {
	cases := []struct {
		name      string
		cfg       Config
		server    string
		fragments []string
		forbidden []string
	}{
		{"unknown server", Config{}, "unknown",
			[]string{`config="<memory>"`, `key="servers"`, "code=server_unknown", "property=credentials", "shape=unknown_server", "credentials reference unknown server"},
			nil},
		{"headers not mapping", Config{Servers: []Server{{ID: "alpha", Credentials: map[string]any{"headers": "bad"}}}}, "alpha",
			[]string{`config="<memory>"`, `key="servers[0].credentials.headers"`, `server="alpha"`, "code=header_mapping", "property=type", "shape=string", "credentials.headers must be a mapping"},
			[]string{"bad"}},
		{"invalid header name", Config{Servers: []Server{{ID: "alpha", Credentials: map[string]any{"headers": map[string]any{"bad header": map[string]any{"env": "TOKEN"}}}}}}, "alpha",
			[]string{`config="<memory>"`, `key="servers[0].credentials.headers"`, `server="alpha"`, "code=header_name_invalid", "property=name", "shape=invalid_identifier", "invalid credential header name"},
			[]string{"bad header"}},
		{"missing resolved bearer", Config{Servers: []Server{{ID: "alpha", Credentials: map[string]any{"bearer": map[string]any{"env": "TOKEN"}}}}}, "alpha",
			[]string{`config="<memory>"`, `key="servers[0].credentials.bearer"`, `server="alpha"`, `ref="env:TOKEN"`, "code=reference_unresolved", "property=reference", "shape=unresolved", "secret reference does not resolve"},
			nil},
		{"non-reference bearer", Config{Servers: []Server{{ID: "alpha", Credentials: map[string]any{"bearer": "plain"}}}}, "alpha",
			[]string{`config="<memory>"`, `key="servers[0].credentials.bearer"`, `server="alpha"`, "code=reference_shape", "property=reference", "shape=string", "must reference exactly one secret"},
			[]string{"plain"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.cfg.ServerCredentials(tc.server)
			requireDiagnostics(t, err)
			requireRendered(t, err, tc.fragments...)
			forbidRendered(t, err, tc.forbidden...)
		})
	}
	_, err := (Config{}).ServerLimits("missing")
	requireDiagnostics(t, err)
	requireRendered(t, err,
		`config="<memory>"`, `key="servers"`, "code=server_unknown", "property=limits", "shape=unknown_server", "limits reference unknown server",
	)
}

// TestEmbeddingCredentialResolutionFailures migrates the C-1 embedding rows:
// reference_shape for a multi-reference mapping and reference_unresolved with
// the configured reference identity (spec §5.2).
func TestEmbeddingCredentialResolutionFailures(t *testing.T) {
	for _, tc := range []struct {
		name      string
		embedding map[string]any
		resolved  map[string]string
		fragments []string
	}{
		{"multiple references", map[string]any{"api_key": map[string]any{"env": "A", "file": "b"}}, nil,
			[]string{`config="<memory>"`, `key="embedding.api_key"`, "code=reference_shape", "property=reference", "shape=mapping", "must reference exactly one secret"}},
		{"unresolved reference", map[string]any{"api_key": map[string]any{"env": "TOKEN"}}, nil,
			[]string{`config="<memory>"`, `key="embedding.api_key"`, `ref="env:TOKEN"`, "code=reference_unresolved", "property=reference", "shape=unresolved", "secret reference does not resolve"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (Config{Embedding: tc.embedding, resolved: tc.resolved}).EmbeddingCredential()
			requireDiagnostics(t, err)
			requireRendered(t, err, tc.fragments...)
		})
	}
}

// TestCredentialFingerprintFailureAndSuccess stays a behavioral regression: the
// fingerprint surface neither gains nor loses cases, and no diagnostic may
// carry a fingerprint value (spec §5.2).
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

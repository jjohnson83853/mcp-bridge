package config_test

import (
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
)

// The embedding provider's own credential is resolved for the client that
// performs embedding requests and for nothing else: it never appears in the
// public fingerprint, and a document that configures no key is not an error
// because an OpenAI-compatible endpoint on a trusted network may need none
// (plan §4.2/§4.8).
func TestEmbeddingCredentialResolvesTheConfiguredReference(t *testing.T) {
	fingerprintEnv(t)
	cfg := mustParseConfig(t, fingerprintBase)
	credential, err := cfg.EmbeddingCredential()
	if err != nil {
		t.Fatalf("EmbeddingCredential: %v", err)
	}
	if credential != "value-of-FPRINT_EMBED_SECRET" {
		t.Fatalf("credential = %q", credential)
	}
	if strings.Contains(cfg.Fingerprint(), credential) {
		t.Fatal("the embedding credential leaked into the public fingerprint")
	}
}

func TestEmbeddingCredentialAbsentIsNotAnError(t *testing.T) {
	fingerprintEnv(t)
	document := strings.Replace(fingerprintBase, "  api_key: {env: FPRINT_EMBED_SECRET}\n", "", 1)
	cfg := mustParseConfig(t, document)
	credential, err := cfg.EmbeddingCredential()
	if err != nil {
		t.Fatalf("EmbeddingCredential: %v", err)
	}
	if credential != "" {
		t.Fatalf("credential = %q, want empty", credential)
	}
}

// A literal key is not a secret reference, so the document is rejected rather
// than run with a credential pasted into configuration.
func TestEmbeddingCredentialRejectsLiteralSecrets(t *testing.T) {
	fingerprintEnv(t)
	document := strings.Replace(fingerprintBase,
		"api_key: {env: FPRINT_EMBED_SECRET}", "api_key: literal-key", 1)
	if _, err := config.Parse([]byte(document), "."); err == nil {
		t.Fatal("expected a literal embedding key to be rejected")
	}
}

package config_test

import (
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
)

// secretSequenceConfig builds a minimal valid document whose only variable is
// the single element placed inside client_auth.bearer_tokens. Every element of
// that sequence must be a secret reference ({env: NAME} or {file: PATH}); a
// literal value silently vanishes from resolveSecrets/collectRefs, which would
// make Config.ClientTokenHashes() omit the token and 401 every client with no
// parse-time diagnostic.
func secretSequenceConfig(element string) string {
	return "schema_version: 1\n" +
		"servers:\n" +
		"  - id: alpha\n" +
		"    endpoint: https://alpha.invalid/mcp\n" +
		"client_auth:\n" +
		"  bearer_tokens:\n" +
		"    - " + element + "\n" +
		"pins: []\n"
}

// TestParseRejectsNonMappingBearerTokenElements covers the inverted predicate in
// validateNode's sequence branch: a scalar string child carries tag !!str, so it
// used to slip through while only non-string scalars (ints, bools) were caught.
// Every non-mapping element must be rejected with "secret must be a reference".
func TestParseRejectsNonMappingBearerTokenElements(t *testing.T) {
	t.Setenv("SEQ_UNUSED_SECRET", "unused")
	cases := []struct {
		name    string
		element string
	}{
		{"literal string", "plain-secret"},
		{"quoted literal string", "'plain-secret'"},
		{"integer", "12345"},
		{"boolean", "true"},
		{"null", "null"},
		{"nested sequence", "[plain-secret]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.Parse([]byte(secretSequenceConfig(tc.element)), t.TempDir())
			if err == nil {
				t.Fatalf("Parse accepted non-mapping bearer token %s; want rejection", tc.element)
			}
			if !strings.Contains(err.Error(), "secret must be a reference") {
				t.Fatalf("Parse error = %v, want %q", err, "secret must be a reference")
			}
		})
	}
}

// TestParseAcceptsEnvBearerTokenReferences guards the legal form: a sequence of
// {env: NAME} mappings must still parse and yield one client token hash per
// reference, so the rejection above cannot be implemented by dropping sequence
// support wholesale.
func TestParseAcceptsEnvBearerTokenReferences(t *testing.T) {
	t.Setenv("SEQ_CLIENT_TOKEN_A", "client-token-a")
	t.Setenv("SEQ_CLIENT_TOKEN_B", "client-token-b")
	input := "schema_version: 1\n" +
		"servers:\n" +
		"  - id: alpha\n" +
		"    endpoint: https://alpha.invalid/mcp\n" +
		"client_auth:\n" +
		"  bearer_tokens:\n" +
		"    - {env: SEQ_CLIENT_TOKEN_A}\n" +
		"    - {env: SEQ_CLIENT_TOKEN_B}\n" +
		"pins: []\n"
	cfg, err := config.Parse([]byte(input), t.TempDir())
	if err != nil {
		t.Fatalf("Parse({env: ...}) = %v, want nil", err)
	}
	if got := len(cfg.ClientTokenHashes()); got != 2 {
		t.Fatalf("ClientTokenHashes() length = %d, want 2", got)
	}
}

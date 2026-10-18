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
// the sequence-element branch: a scalar string child carries tag !!str, so it
// used to slip through while only non-string scalars (ints, bools) were caught.
// Every non-mapping element must be rejected with the reference_required class,
// the exact location, the actual finite element kind, and the static reason —
// never a plain-secret or scalar echo (spec §5.2).
func TestParseRejectsNonMappingBearerTokenElements(t *testing.T) {
	t.Setenv("SEQ_UNUSED_SECRET", "unused")
	cases := []struct {
		name, element, shape string
	}{
		{"literal string", "plain-secret", "string"},
		{"quoted literal string", "'plain-secret'", "string"},
		{"integer", "12345", "integer"},
		{"boolean", "true", "boolean"},
		{"null", "null", "null"},
		{"nested sequence", "[plain-secret]", "sequence"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := t.TempDir()
			_, err := config.Parse([]byte(secretSequenceConfig(tc.element)), path)
			if err == nil {
				t.Fatalf("Parse accepted non-mapping bearer token %s; want rejection", tc.element)
			}
			line := err.Error()
			if strings.Contains(line, "\n") {
				t.Fatalf("expected one physical line, got %q", line)
			}
			for _, want := range []string{
				`config="` + path + `"`,
				`key="client_auth.bearer_tokens[0]"`,
				"code=reference_required",
				"property=reference",
				"shape=" + tc.shape,
				"secret must be a reference",
				"; remaining_stages=not_checked",
			} {
				if !strings.Contains(line, want) {
					t.Fatalf("rendered line lacks %q: %s", want, line)
				}
			}
			if strings.Contains(line, "plain-secret") || strings.Contains(line, "12345") {
				t.Fatalf("rendered line echoes the rejected scalar: %s", line)
			}
		})
	}
}

// TestParseAcceptsEnvBearerTokenReferences guards the legal form: a sequence of
// {env: NAME} mappings must still parse and yield one client token hash per
// reference, so the rejection above cannot be implemented by dropping sequence
// support wholesale (spec §5.2 AC-32; unchanged).
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

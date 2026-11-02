package config_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
)

// fingerprintBase exercises every public root section so the fingerprint
// contract is asserted over the whole public configuration, not one corner.
const fingerprintBase = `schema_version: 1
http:
  listen: 127.0.0.1:8080
operations:
  listen: 127.0.0.1:8081
storage:
  directory: /var/lib/mcp-bridge
catalog:
  refresh_interval: 60s
search:
  default_top_k: 5
embedding:
  provider: openai-compatible
  model: embed-model
  model_revision: r1
  base_url: https://embed.invalid/v1
  dimensions: 8
  api_key: {env: FPRINT_EMBED_SECRET}
upstream_defaults:
  call_timeout: 30s
servers:
  - id: alpha
    endpoint: https://alpha.invalid/mcp
    credentials:
      bearer: {env: FPRINT_UPSTREAM_SECRET}
    limits:
      max_concurrent_calls: 2
      call_timeout: 10s
client_auth:
  bearer_tokens:
    - {env: FPRINT_CLIENT_SECRET}
pins:
  - {server: alpha, tool: read}
`

var hexDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

func fingerprintEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"FPRINT_EMBED_SECRET", "FPRINT_UPSTREAM_SECRET", "FPRINT_CLIENT_SECRET",
		"FPRINT_UPSTREAM_SECRET2", "FPRINT_CLIENT_SECRET2",
	} {
		t.Setenv(name, "value-of-"+name)
	}
}

func mustParseConfig(t *testing.T, input string) *config.Config {
	t.Helper()
	cfg, err := config.Parse([]byte(input), ".")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return cfg
}

// TestFingerprintIsDeterministicHexAndPresentationIndependent pins that the
// public fingerprint is a canonical identity: stable across repeated parses
// and independent of comments, quoting, key order, and list order that carry no
// semantic difference. See plan §4.2/§4.8/§4.9.
func TestFingerprintIsDeterministicHexAndPresentationIndependent(t *testing.T) {
	fingerprintEnv(t)
	base := mustParseConfig(t, fingerprintBase).Fingerprint()
	if !hexDigest.MatchString(base) {
		t.Fatalf("fingerprint %q is not a lowercase 64-hex digest", base)
	}
	if again := mustParseConfig(t, fingerprintBase).Fingerprint(); again != base {
		t.Fatalf("same configuration parsed twice: %q != %q", again, base)
	}

	reformatted := strings.Replace(fingerprintBase,
		"  - id: alpha\n    endpoint: https://alpha.invalid/mcp\n",
		"  # the only server\n  - endpoint: \"https://alpha.invalid/mcp\"\n    id: alpha\n", 1)
	reformatted = strings.Replace(reformatted, "schema_version: 1\n", "schema_version: 1  # fixed schema\n", 1)
	if reformatted == fingerprintBase {
		t.Fatal("reformat fixture did not apply")
	}
	if got := mustParseConfig(t, reformatted).Fingerprint(); got != base {
		t.Fatalf("presentation-only reformat changed fingerprint: %q != %q", got, base)
	}

	firstOrder := strings.Replace(fingerprintBase, "client_auth:",
		"  - id: beta\n    endpoint: https://beta.invalid/mcp\n    credentials:\n      bearer: {env: FPRINT_UPSTREAM_SECRET2}\nclient_auth:", 1)
	secondOrder := strings.Replace(fingerprintBase, "servers:\n  - id: alpha",
		"servers:\n  - id: beta\n    endpoint: https://beta.invalid/mcp\n    credentials:\n      bearer: {env: FPRINT_UPSTREAM_SECRET2}\n  - id: alpha", 1)
	if firstOrder == secondOrder {
		t.Fatal("reorder fixture did not apply")
	}
	// Entry order within `servers` is not semantic: the fingerprint sorts by
	// server id, so reordering the two entries must not change it.
	if first, second := mustParseConfig(t, firstOrder).Fingerprint(), mustParseConfig(t, secondOrder).Fingerprint(); first != second {
		t.Fatalf("server entry order changed the fingerprint: %q != %q", first, second)
	}
}

// TestFingerprintTracksPublicShapeChanges pins that every public, non-secret
// change produces a different fingerprint, so a reload candidate cannot be
// mistaken for the running configuration. Plan §4.2 `config_public_fingerprint`.
func TestFingerprintTracksPublicShapeChanges(t *testing.T) {
	fingerprintEnv(t)
	base := mustParseConfig(t, fingerprintBase).Fingerprint()

	cases := []struct {
		name  string
		old   string
		new   string
		extra string
	}{
		{name: "endpoint", old: "https://alpha.invalid/mcp", new: "https://alpha2.invalid/mcp"},
		{name: "server limits", old: "max_concurrent_calls: 2", new: "max_concurrent_calls: 3"},
		{name: "server limit timeout", old: "call_timeout: 10s", new: "call_timeout: 11s"},
		{name: "credential reference", old: "FPRINT_UPSTREAM_SECRET", new: "FPRINT_UPSTREAM_SECRET2"},
		{name: "client token reference", old: "FPRINT_CLIENT_SECRET", new: "FPRINT_CLIENT_SECRET2"},
		{name: "http listen", old: "127.0.0.1:8080", new: "127.0.0.1:8090"},
		{name: "operations listen", old: "127.0.0.1:8081", new: "127.0.0.1:8091"},
		{name: "storage directory", old: "/var/lib/mcp-bridge", new: "/var/lib/mcp-bridge-2"},
		{name: "catalog refresh", old: "refresh_interval: 60s", new: "refresh_interval: 61s"},
		{name: "search top k", old: "default_top_k: 5", new: "default_top_k: 6"},
		{name: "embedding revision", old: "model_revision: r1", new: "model_revision: r2"},
		{name: "upstream default timeout", old: "call_timeout: 30s", new: "call_timeout: 31s"},
		{
			name: "credential shape",
			old:  "      bearer: {env: FPRINT_UPSTREAM_SECRET}",
			new:  "      headers:\n        x-api-key: {env: FPRINT_UPSTREAM_SECRET}",
		},
		{name: "pins", old: "pins:\n  - {server: alpha, tool: read}", new: "pins: []"},
		{
			name: "added server",
			old:  "client_auth:",
			new:  "  - id: beta\n    endpoint: https://beta.invalid/mcp\n    credentials:\n      bearer: {env: FPRINT_UPSTREAM_SECRET}\nclient_auth:",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := strings.Replace(fingerprintBase, tc.old, tc.new, 1)
			if input == fingerprintBase {
				t.Fatalf("variant %q did not apply", tc.name)
			}
			if got := mustParseConfig(t, input).Fingerprint(); got == base {
				t.Fatalf("%s change did not change the fingerprint %q", tc.name, got)
			}
		})
	}
}
func TestFingerprintChangesOnServerStatelessToggle(t *testing.T) {
	fingerprintEnv(t)

	falseConfig := strings.Replace(fingerprintBase,
		"endpoint: https://alpha.invalid/mcp\n",
		"endpoint: https://alpha.invalid/mcp\n    stateless: false\n", 1)
	trueConfig := strings.Replace(fingerprintBase,
		"endpoint: https://alpha.invalid/mcp\n",
		"endpoint: https://alpha.invalid/mcp\n    stateless: true\n", 1)
	falseFingerprint := mustParseConfig(t, falseConfig).Fingerprint()
	trueFingerprint := mustParseConfig(t, trueConfig).Fingerprint()
	if falseFingerprint == trueFingerprint {
		t.Fatalf("server stateless toggle did not change the fingerprint: %q", falseFingerprint)
	}
}

// TestFingerprintIgnoresSecretValues pins that only resolved secret values are
// outside the public fingerprint: rotating an upstream or client secret value
// without touching any reference must not read as a configuration change.
func TestFingerprintIgnoresSecretValues(t *testing.T) {
	fingerprintEnv(t)
	base := mustParseConfig(t, fingerprintBase).Fingerprint()

	t.Setenv("FPRINT_UPSTREAM_SECRET", "rotated-upstream")
	t.Setenv("FPRINT_CLIENT_SECRET", "rotated-client")
	t.Setenv("FPRINT_EMBED_SECRET", "rotated-embed")
	if got := mustParseConfig(t, fingerprintBase).Fingerprint(); got != base {
		t.Fatalf("secret value rotation changed the public fingerprint: %q != %q", got, base)
	}
}

// TestServerCredentialFingerprintTracksRotationAndIsolation pins the per-server
// credential identity a reload uses to decide whether a dedicated upstream
// client can be reused (plan §4.10 steps 2 and 5): it changes when that
// server's own credentials rotate, stays put when another server's or the
// inbound client token rotates, is per-server, and is absent for unknown ids.
func TestServerCredentialFingerprintTracksRotationAndIsolation(t *testing.T) {
	fingerprintEnv(t)
	withBeta := strings.Replace(fingerprintBase,
		"client_auth:",
		"  - id: beta\n    endpoint: https://beta.invalid/mcp\n    credentials:\n      bearer: {env: FPRINT_UPSTREAM_SECRET2}\nclient_auth:", 1)

	base := mustParseConfig(t, withBeta)
	alpha, ok := base.ServerCredentialFingerprint("alpha")
	if !ok {
		t.Fatal("ServerCredentialFingerprint(alpha) = not found")
	}
	if !hexDigest.MatchString(alpha) {
		t.Fatalf("credential fingerprint %q is not a lowercase 64-hex digest", alpha)
	}
	beta, ok := base.ServerCredentialFingerprint("beta")
	if !ok {
		t.Fatal("ServerCredentialFingerprint(beta) = not found")
	}
	if alpha == beta {
		t.Fatal("distinct servers sharing no secret produced the same credential fingerprint")
	}
	if _, ok := base.ServerCredentialFingerprint("missing"); ok {
		t.Fatal("ServerCredentialFingerprint(missing) reported a fingerprint")
	}

	t.Setenv("FPRINT_UPSTREAM_SECRET", "rotated-upstream")
	rotated := mustParseConfig(t, withBeta)
	if got, _ := rotated.ServerCredentialFingerprint("alpha"); got == alpha {
		t.Fatal("upstream secret rotation did not change alpha's credential fingerprint")
	}
	if got, _ := rotated.ServerCredentialFingerprint("beta"); got != beta {
		t.Fatalf("alpha's rotation changed beta's credential fingerprint: %q != %q", got, beta)
	}

	t.Setenv("FPRINT_CLIENT_SECRET", "rotated-client")
	clientRotated := mustParseConfig(t, withBeta)
	rotatedAlpha, _ := rotated.ServerCredentialFingerprint("alpha")
	if got, _ := clientRotated.ServerCredentialFingerprint("alpha"); got != rotatedAlpha {
		t.Fatalf("inbound client token rotation changed alpha's credential fingerprint: %q != %q", got, rotatedAlpha)
	}
}

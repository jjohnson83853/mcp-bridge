package config_test

import (
	"crypto/sha256"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
)

const valid = `schema_version: 1
servers:
  - id: alpha
    endpoint: https://alpha.invalid/mcp
    credentials:
      bearer: {env: TEST_UPSTREAM_SECRET}
client_auth:
  bearer_tokens:
    - {env: TEST_CLIENT_SECRET}
pins: []
`

func TestParseValidFlatConfigAndRedactsSecrets(t *testing.T) {
	t.Setenv("TEST_UPSTREAM_SECRET", "private-upstream-value")
	t.Setenv("TEST_CLIENT_SECRET", "private-client-value")
	cfg, err := config.Parse([]byte(valid), ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private-upstream-value", "private-client-value"} {
		if strings.Contains(cfg.String(), secret) {
			t.Fatalf("String leaked secret %q", secret)
		}
	}
}
func TestServerStatelessDefaultAndExplicit(t *testing.T) {
	t.Setenv("TEST_UPSTREAM_SECRET", "private-upstream-value")
	t.Setenv("TEST_CLIENT_SECRET", "private-client-value")

	cases := []struct {
		name      string
		stateless string
		want      bool
	}{
		{name: "omitted", want: true},
		{name: "explicit false", stateless: "false", want: false},
		{name: "explicit true", stateless: "true", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := valid
			if tc.stateless != "" {
				input = strings.Replace(input,
					"endpoint: https://alpha.invalid/mcp\n",
					"endpoint: https://alpha.invalid/mcp\n    stateless: "+tc.stateless+"\n", 1)
			}
			cfg, err := config.Parse([]byte(input), ".")
			if err != nil {
				t.Fatal(err)
			}
			if got := cfg.Servers[0].ServerStateless(); got != tc.want {
				t.Errorf("ServerStateless() = %t, want %t", got, tc.want)
			}
		})
	}
}


func TestParseRejectsInvalidDocuments(t *testing.T) {
	t.Setenv("TEST_UPSTREAM_SECRET", "upstream-secret")
	t.Setenv("TEST_CLIENT_SECRET", "client-secret")
	t.Setenv("TEST_CLIENT_ALIAS", "client-secret-alias")
	cases := []struct{ name, input string }{
		{"unknown retired namespaces", strings.Replace(valid, "pins: []", "namespaces: []", 1)},
		{"duplicate key", strings.Replace(valid, "schema_version: 1", "schema_version: 1\nschema_version: 1", 1)},
		{"version", strings.Replace(valid, "schema_version: 1", "schema_version: 2", 1)},
		{"server id", strings.Replace(valid, "id: alpha", "id: Alpha", 1)},
		{"secret reference overlap", strings.Replace(valid, "TEST_CLIENT_SECRET", "TEST_UPSTREAM_SECRET", 1)},
		{"secret value overlap", strings.Replace(valid, "TEST_CLIENT_SECRET", "TEST_CLIENT_ALIAS", 1)},
		{"literal secret", strings.Replace(valid, "{env: TEST_UPSTREAM_SECRET}", "plain-secret", 1)},
		{"YAML alias", strings.Replace(valid, "schema_version: 1", "schema_version: 1\nhttp: &http\n  listen: 127.0.0.1:8080\noperations: *http", 1)},
		{"YAML merge key", strings.Replace(valid, "pins: []", "http:\n  <<: {listen: 127.0.0.1:8080}\npins: []", 1)},
		{"multiple docs", valid + "---\nschema_version: 1\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "secret value overlap" {
				t.Setenv("TEST_CLIENT_ALIAS", "upstream-secret")
			}
			if _, err := config.Parse([]byte(tc.input), "."); err == nil {
				t.Fatal("Parse succeeded; want rejection")
			}
		})
	}
}

func TestParseRejectsMissingEnvironmentSecret(t *testing.T) {
	os.Unsetenv("TEST_MISSING_SECRET")
	input := strings.Replace(valid, "TEST_UPSTREAM_SECRET", "TEST_MISSING_SECRET", 1)
	if _, err := config.Parse([]byte(input), "."); err == nil {
		t.Fatal("missing secret accepted")
	}
}

func TestParseRejectsBoundsTypesURLsAndWeights(t *testing.T) {
	t.Setenv("TEST_UPSTREAM_SECRET", "upstream")
	t.Setenv("TEST_CLIENT_SECRET", "client")
	cases := []struct{ name, input string }{
		{"session ceiling", strings.Replace(valid, "pins: []", "http:\n  max_sessions: 4097\npins: []", 1)},
		{"search timeout ceiling", strings.Replace(valid, "pins: []", "search:\n  timeout: 31s\npins: []", 1)},
		{"weights sum", strings.Replace(valid, "pins: []", "search:\n  keyword_weight: 0.4\n  embedding_weight: 0.5\npins: []", 1)},
		{"numeric coercion", strings.Replace(valid, "schema_version: 1", "schema_version: \"1\"", 1)},
		{"nonfinite", strings.Replace(valid, "pins: []", "search:\n  keyword_weight: .nan\n  embedding_weight: 0\npins: []", 1)},
		{"URL API key", strings.Replace(valid, "https://alpha.invalid/mcp", "https://alpha.invalid/mcp?api-key=private", 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := config.Parse([]byte(tc.input), "."); err == nil {
				t.Fatal("Parse succeeded; want rejection")
			}
		})
	}
}

// TestParseBoundsDurations pins each duration key to its own window: session
// lifetimes (plan §4.2 default 10m) must be accepted, while request-path
// timeouts keep their existing ceilings.
func TestParseBoundsDurations(t *testing.T) {
	t.Setenv("TEST_UPSTREAM_SECRET", "upstream")
	t.Setenv("TEST_CLIENT_SECRET", "client")
	cases := []struct {
		name    string
		input   string
		wantKey string // "" means Parse must succeed
	}{
		{"plan sample session idle 10m accepted", strings.Replace(valid, "pins: []", "http:\n  session_idle_timeout: 10m\npins: []", 1), ""},
		{"session idle window edge 1s accepted", strings.Replace(valid, "pins: []", "http:\n  session_idle_timeout: 1s\npins: []", 1), ""},
		{"session idle window edge 1h accepted", strings.Replace(valid, "pins: []", "http:\n  session_idle_timeout: 1h\npins: []", 1), ""},
		{"session idle just above 1h rejected", strings.Replace(valid, "pins: []", "http:\n  session_idle_timeout: 1h1s\npins: []", 1), "http.session_idle_timeout"},
		{"session idle just below 1s rejected", strings.Replace(valid, "pins: []", "http:\n  session_idle_timeout: 999ms\npins: []", 1), "http.session_idle_timeout"},
		{"session idle zero rejected", strings.Replace(valid, "pins: []", "http:\n  session_idle_timeout: 0s\npins: []", 1), "http.session_idle_timeout"},
		{"search timeout ceiling kept", strings.Replace(valid, "pins: []", "search:\n  timeout: 31s\npins: []", 1), "search.timeout"},
		{"upstream call timeout ceiling kept", strings.Replace(valid, "pins: []", "upstream_defaults:\n  call_timeout: 121s\npins: []", 1), "upstream_defaults.call_timeout"},
		{"embedding timeout ceiling kept", strings.Replace(valid, "pins: []", "embedding:\n  timeout: 31s\npins: []", 1), "embedding.timeout"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.Parse([]byte(tc.input), ".")
			if tc.wantKey == "" {
				if err != nil {
					t.Fatalf("Parse rejected a permitted duration: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Parse succeeded; want rejection")
			}
			// The rejected key is carried by the location metadata and the
			// bounds explanation stays a static reason (spec §3.1D).
			if !strings.Contains(err.Error(), `key="`+tc.wantKey+`"`) {
				t.Fatalf("error %q does not name the key %s", err, tc.wantKey)
			}
			if !strings.Contains(err.Error(), "outside permitted duration bounds") {
				t.Fatalf("error %q lacks the bounds message", err)
			}
		})
	}
}

func TestParseRejectsWrongScalarAndLiteralCredentialHeaders(t *testing.T) {
	t.Setenv("TEST_UPSTREAM_SECRET", "upstream")
	t.Setenv("TEST_CLIENT_SECRET", "client")
	cases := []string{
		strings.Replace(valid, "pins: []", "search:\n  default_top_k: \"10\"\npins: []", 1),
		strings.Replace(valid, "credentials:\n      bearer: {env: TEST_UPSTREAM_SECRET}", "credentials:\n      headers:\n        X-API-Key: literal", 1),
	}
	for _, input := range cases {
		if _, err := config.Parse([]byte(input), "."); err == nil {
			t.Fatalf("Parse accepted invalid scalar/credential header: %s", input)
		}
	}
}

func TestParseNormalizesURLWithoutDroppingPathOrQuery(t *testing.T) {
	t.Setenv("TEST_UPSTREAM_SECRET", "upstream")
	t.Setenv("TEST_CLIENT_SECRET", "client")
	input := strings.Replace(valid, "https://alpha.invalid/mcp", "HTTPS://ALPHA.INVALID/Mixed/Path?mode=safe", 1)
	cfg, err := config.Parse([]byte(input), ".")
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Servers[0].Endpoint; got != "https://alpha.invalid/Mixed/Path?mode=safe" {
		t.Fatalf("normalized endpoint=%q", got)
	}
}

func TestParseFileResolvesPrivateRelativeSecretOnceAndRedacts(t *testing.T) {
	t.Setenv("TEST_CLIENT_SECRET", "client-value")
	directory := t.TempDir()
	secretPath := directory + "/upstream.token"
	if err := os.WriteFile(secretPath, []byte("upstream-file-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := directory + "/bridge.yaml"
	input := strings.Replace(valid, "bearer: {env: TEST_UPSTREAM_SECRET}", "bearer: {file: upstream.token}", 1)
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cfg.String(), "upstream-file-value") {
		t.Fatal("Config.String leaked file secret")
	}
	if !strings.Contains(cfg.String(), "file:upstream.token") {
		t.Fatalf("Config.String omitted reference name: %s", cfg)
	}
}

func TestParseRejectsNonPrivateSecretFile(t *testing.T) {
	t.Setenv("TEST_CLIENT_SECRET", "client-value")
	directory := t.TempDir()
	secretPath := directory + "/upstream.token"
	if err := os.WriteFile(secretPath, []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := directory + "/bridge.yaml"
	input := strings.Replace(valid, "bearer: {env: TEST_UPSTREAM_SECRET}", "bearer: {file: upstream.token}", 1)
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.ParseFile(path); err == nil {
		t.Fatal("world-readable secret file accepted")
	}
}

func TestParseRejectsSymlinkSecretFile(t *testing.T) {
	t.Setenv("TEST_CLIENT_SECRET", "client-value")
	directory := t.TempDir()
	target := directory + "/target"
	link := directory + "/link"
	if err := os.WriteFile(target, []byte("upstream\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	path := directory + "/bridge.yaml"
	input := strings.Replace(valid, "bearer: {env: TEST_UPSTREAM_SECRET}", "bearer: {file: link}", 1)
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.ParseFile(path); err == nil {
		t.Fatal("symlink secret file accepted")
	}
}

func TestParseRejectsSecretFileOwnedByAnotherUser(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root to create a foreign-owned test file")
	}
	t.Setenv("TEST_CLIENT_SECRET", "client-value")
	directory := t.TempDir()
	secretPath := directory + "/upstream.token"
	if err := os.WriteFile(secretPath, []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(secretPath, 65534, 65534); err != nil {
		t.Skipf("cannot create foreign-owned file: %v", err)
	}
	path := directory + "/bridge.yaml"
	input := strings.Replace(valid, "bearer: {env: TEST_UPSTREAM_SECRET}", "bearer: {file: upstream.token}", 1)
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.ParseFile(path); err == nil {
		t.Fatal("foreign-owned secret file accepted")
	}
}

func TestParseValidatesSequenceElementTypesAndPositiveDurations(t *testing.T) {
	t.Setenv("TEST_UPSTREAM_SECRET", "upstream")
	t.Setenv("TEST_CLIENT_SECRET", "client")
	cases := []struct {
		name, input string
	}{
		{"numeric host", strings.Replace(valid, "pins: []", "http:\n  allowed_hosts: [8080]\npins: []", 1)},
		{"boolean origin", strings.Replace(valid, "pins: []", "http:\n  allowed_origins: [false]\npins: []", 1)},
		{"zero refresh interval", strings.Replace(valid, "pins: []", "catalog:\n  refresh_interval: 0s\npins: []", 1)},
		{"negative cache ttl", strings.Replace(valid, "pins: []", "search:\n  query_cache_ttl: -1s\npins: []", 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := config.Parse([]byte(tc.input), "."); err == nil {
				t.Fatal("Parse succeeded; want rejection")
			}
		})
	}
}

func TestParseResolvesRelativeStorageAgainstConfigDirectory(t *testing.T) {
	t.Setenv("TEST_UPSTREAM_SECRET", "upstream")
	t.Setenv("TEST_CLIENT_SECRET", "client")
	cfg, err := config.Parse([]byte(strings.Replace(valid, "pins: []", "storage:\n  directory: ./state\npins: []", 1)), "/etc/mcp-bridge/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.Storage["directory"], "/etc/mcp-bridge/state"; got != want {
		t.Fatalf("storage directory=%v, want %q", got, want)
	}
}
func TestParseRejectsURLFragmentsAndCredentialQueryParameters(t *testing.T) {
	t.Setenv("TEST_UPSTREAM_SECRET", "upstream")
	t.Setenv("TEST_CLIENT_SECRET", "client")
	for _, endpoint := range []string{
		"https://alpha.invalid/mcp#",
		"https://alpha.invalid/mcp?key=private",
	} {
		t.Run(endpoint, func(t *testing.T) {
			input := strings.Replace(valid, "https://alpha.invalid/mcp", endpoint, 1)
			if _, err := config.Parse([]byte(input), "."); err == nil {
				t.Fatalf("Parse accepted endpoint %q", endpoint)
			}
		})
	}
}

func TestConfigUpperBoundsAreInclusive(t *testing.T) {
	t.Setenv("TEST_UPSTREAM_SECRET", "upstream")
	t.Setenv("TEST_CLIENT_SECRET", "client")
	cases := []struct {
		name, section, field, maximum string
	}{
		{"sessions", "http", "max_sessions", "4096"},
		{"calls", "upstream_defaults", "max_concurrent_calls", "128"},
		{"catalog bytes", "catalog", "max_schema_bytes", "262144"},
		{"snapshot bytes", "storage", "max_snapshot_bytes", "268435456"},
		{"dimensions", "embedding", "dimensions", "4096"},
		{"batch", "embedding", "batch_size", "128"},
		{"embedding concurrency", "embedding", "max_concurrency", "32"},
		{"cache", "search", "query_cache_entries", "4096"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, value := range []struct {
				number string
				valid  bool
			}{{tc.maximum, true}, {incrementDecimal(tc.maximum), false}} {
				input := strings.Replace(valid, "pins: []", tc.section+":\n  "+tc.field+": "+value.number+"\npins: []", 1)
				_, err := config.Parse([]byte(input), ".")
				if (err == nil) != value.valid {
					t.Fatalf("value %s err=%v, want valid=%v", value.number, err, value.valid)
				}
			}
		})
	}
}

func incrementDecimal(value string) string {
	number := 0
	for _, digit := range value {
		number = number*10 + int(digit-'0')
	}
	number++
	return strconv.Itoa(number)
}

func TestParseRejectsCaseInsensitiveDuplicateCredentialHeaders(t *testing.T) {
	t.Setenv("TEST_UPSTREAM_SECRET", "upstream")
	t.Setenv("TEST_CLIENT_SECRET", "client")
	input := strings.Replace(
		valid,
		"      bearer: {env: TEST_UPSTREAM_SECRET}",
		"      bearer: {env: TEST_UPSTREAM_SECRET}\n      headers:\n        X-API-Key: {env: TEST_UPSTREAM_SECRET}\n        x-api-key: {env: TEST_UPSTREAM_SECRET}",
		1,
	)
	if _, err := config.Parse([]byte(input), "."); err == nil {
		t.Fatal("case-insensitive duplicate header names accepted")
	}
}

func TestClientTokenHashesExposeFingerprintsNotTokenValues(t *testing.T) {
	t.Setenv("TEST_UPSTREAM_SECRET", "upstream")
	t.Setenv("TEST_CLIENT_SECRET", "client-private-value")
	cfg, err := config.Parse([]byte(valid), ".")
	if err != nil {
		t.Fatal(err)
	}
	hashes := cfg.ClientTokenHashes()
	if len(hashes) != 1 || hashes[0] != sha256.Sum256([]byte("client-private-value")) {
		t.Fatalf("client hashes=%v", hashes)
	}
}

package config_test

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
)

const credentialConfig = `schema_version: 1
servers:
  - id: alpha
    endpoint: https://alpha.invalid/mcp
    credentials:
      bearer: {env: CRED_ALPHA_BEARER}
      headers:
        X-Api-Key: {env: CRED_ALPHA_API_KEY}
        X-Tenant: {file: tenant.txt}
    limits:
      max_concurrent_calls: 3
      call_timeout: 45s
  - id: beta
    endpoint: https://beta.invalid/mcp
client_auth:
  bearer_tokens:
    - {env: CRED_CLIENT_TOKEN}
pins: []
`

func credentialEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CRED_ALPHA_BEARER", "alpha-bearer-value")
	t.Setenv("CRED_ALPHA_API_KEY", "alpha-api-key-value")
	t.Setenv("CRED_CLIENT_TOKEN", "client-token-value")
	t.Chdir(t.TempDir())
	if err := os.WriteFile("tenant.txt", []byte("tenant-value\n"), 0o600); err != nil {
		t.Fatalf("write tenant: %v", err)
	}
}

// TestServerCredentialsResolveOnlyThatServersOwnReferences pins that the command
// layer receives the resolved bearer and header values of one server, and never
// another server's or the inbound client's values (plan §4.2/§4.6).
func TestServerCredentialsResolveOnlyThatServersOwnReferences(t *testing.T) {
	credentialEnv(t)
	cfg, err := config.Parse([]byte(credentialConfig), ".")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	credentials, err := cfg.ServerCredentials("alpha")
	if err != nil {
		t.Fatalf("ServerCredentials: %v", err)
	}
	if credentials.Bearer != "alpha-bearer-value" {
		t.Fatalf("bearer = %q; want the resolved alpha value", credentials.Bearer)
	}
	if got := credentials.Headers.Get("X-Api-Key"); got != "alpha-api-key-value" {
		t.Fatalf("X-Api-Key = %q; want the resolved header value", got)
	}
	if got := credentials.Headers.Get("X-Tenant"); got != "tenant-value" {
		t.Fatalf("X-Tenant = %q; want the resolved file value", got)
	}
	if credentials.Headers.Get("Authorization") != "" {
		t.Fatal("the credential headers must not invent an Authorization header")
	}

	bare, err := cfg.ServerCredentials("beta")
	if err != nil {
		t.Fatalf("ServerCredentials(beta): %v", err)
	}
	if bare.Bearer != "" || len(bare.Headers) != 0 {
		t.Fatalf("beta credentials = %+v; want none", bare)
	}

	if _, err := cfg.ServerCredentials("gamma"); err == nil {
		t.Fatal("an unknown server id must be rejected")
	}
}

// TestServerCredentialsFailClosedOnAnEmptySecret pins that a server credential
// referencing an empty secret is rejected while the document is admitted, so no
// client can ever be opened with a blank credential (plan §4.6).
func TestServerCredentialsFailClosedOnAnEmptySecret(t *testing.T) {
	credentialEnv(t)
	t.Setenv("CRED_ALPHA_API_KEY", "")
	if _, err := config.Parse([]byte(credentialConfig), "."); err == nil {
		t.Fatal("an empty credential secret must be rejected at parse")
	}
}

// TestServerLimitsReportOnlyTheKeysTheServerDeclares pins that per-server limits
// are overrides: an undeclared key reports unset, never a zero value the caller
// would mistake for an explicit limit.
func TestServerLimitsReportOnlyTheKeysTheServerDeclares(t *testing.T) {
	credentialEnv(t)
	cfg, err := config.Parse([]byte(credentialConfig), ".")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	limits, err := cfg.ServerLimits("alpha")
	if err != nil {
		t.Fatalf("ServerLimits: %v", err)
	}
	if limits.MaxConcurrentCalls == nil || *limits.MaxConcurrentCalls != 3 {
		t.Fatalf("max_concurrent_calls = %v; want 3", limits.MaxConcurrentCalls)
	}
	if limits.CallTimeout == nil || *limits.CallTimeout != 45*time.Second {
		t.Fatalf("call_timeout = %v; want 45s", limits.CallTimeout)
	}

	bare, err := cfg.ServerLimits("beta")
	if err != nil {
		t.Fatalf("ServerLimits(beta): %v", err)
	}
	if bare.MaxConcurrentCalls != nil || bare.CallTimeout != nil {
		t.Fatalf("beta limits = %+v; want every key unset", bare)
	}
	if _, err := cfg.ServerLimits("gamma"); err == nil {
		t.Fatal("an unknown server id must be rejected")
	}
}

// TestServerCredentialsRejectAMalformedHeaderName pins that a header name the
// transport would refuse is rejected before a client is built.
func TestServerCredentialsRejectAMalformedHeaderName(t *testing.T) {
	credentialEnv(t)
	document := `schema_version: 1
servers:
  - id: alpha
    endpoint: https://alpha.invalid/mcp
    credentials:
      bearer: {env: CRED_ALPHA_BEARER}
      headers:
        "X Bad": {env: CRED_ALPHA_API_KEY}
client_auth:
  bearer_tokens:
    - {env: CRED_CLIENT_TOKEN}
pins: []
`
	if _, err := config.Parse([]byte(document), "."); err == nil {
		t.Fatal("a malformed header name must be rejected at parse")
	}
}

var _ http.Header = config.Credentials{}.Headers

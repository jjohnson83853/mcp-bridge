package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/auth"
	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/search"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// OAuth environment values the wire tests pin; the middleware and metadata
// handlers read these through auth.NewConfigFromEnv.
const (
	oauthIssuer      = "https://id.bullprog.com"
	oauthResource    = "https://mcp.bullprog.com"
	oauthScope       = "mcp:connect"
	oauthKeyID       = "oauth-test-key-1"
	oauthTestSubject = "wire-test-user"
)

// oauthChallenge is the exact unauthenticated challenge header the middleware
// must emit; the assertion demands byte equality so the wire cannot drift.
const oauthChallenge = `Bearer realm="mcp", resource_metadata="` + oauthResource + `/.well-known/oauth-protected-resource"`

// oauthTestHarness owns one serving bridge wired for OAuth: an in-process
// JWKS endpoint backed by a fresh RSA keypair, the environment the auth
// package reads, and the started process under test.
type oauthTestHarness struct {
	jwks        *httptest.Server
	jwksFetches *atomic.Int64
	private     *rsa.PrivateKey
	public      jwk.Key
	httpURL     string
	opsURL      string
	ctx         context.Context
	cancel      context.CancelFunc
	done        chan int
	stdout      *syncWriter
	stderr      *syncWriter
}

// startOAuthHarness generates the signing key, publishes its JWKS, writes one
// operator document and starts the bridge with the OAuth environment set.
func startOAuthHarness(t *testing.T, serverIDs ...string) *oauthTestHarness {
	t.Helper()
	if len(serverIDs) == 0 {
		serverIDs = []string{"alpha"}
	}

	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate signing key: %v", err)
	}
	public, err := jwk.FromRaw(&private.PublicKey)
	if err != nil {
		t.Fatalf("derive public jwk: %v", err)
	}
	if err := public.Set(jwk.KeyIDKey, oauthKeyID); err != nil {
		t.Fatalf("set kid: %v", err)
	}
	if err := public.Set(jwk.AlgorithmKey, jwa.RS256); err != nil {
		t.Fatalf("set alg: %v", err)
	}
	if err := public.Set(jwk.KeyUsageKey, jwk.ForSignature); err != nil {
		t.Fatalf("set usage: %v", err)
	}
	set := jwk.NewSet()
	if err := set.AddKey(public); err != nil {
		t.Fatalf("build jwks set: %v", err)
	}
	payload, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("marshal jwks: %v", err)
	}
	var jwksFetches atomic.Int64
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		jwksFetches.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	t.Cleanup(jwks.Close)

	if _, exists := os.LookupEnv("MCP_RESOURCE_URL"); !exists {
		// Default origin-only resource fixture; a caller may preset a
		// path-bearing resource URL to observe the conditional challenge.
		t.Setenv("MCP_RESOURCE_URL", oauthResource)
	}
	t.Setenv("POCKET_ID_JWKS_URL", jwks.URL+"/.well-known/jwks.json")
	t.Setenv("POCKET_ID_ISSUER", oauthIssuer)
	t.Setenv("MCP_REQUIRED_SCOPE", oauthScope)
	t.Setenv("SERVE_CLIENT_TOKEN", serveClientToken)
	t.Setenv("SERVE_EMBEDDING_KEY", serveEmbeddingKey)

	readSchema := `{"type":"object","properties":{"limit":{"type":"integer"}},"additionalProperties":true}`
	upstream := fakeUpstream(t, "read", readSchema)
	embeddings := fakeEmbeddings(t)

	httpAddress := freeAddress(t)
	operationsAddress := freeAddress(t)
	directory := t.TempDir()
	configPath := filepath.Join(directory, "bridge.yaml")
	storageDirectory := filepath.Join(directory, "state")
	configuredServers := make([]bridgeServerSpec, len(serverIDs))
	for index, id := range serverIDs {
		configuredServers[index] = bridgeServerSpec{id: id, endpoint: upstream.URL}
	}
	writeBridgeConfig(t, configPath, httpAddress, operationsAddress, storageDirectory, embeddings.URL, configuredServers)

	ctx, cancel := context.WithCancel(context.Background())
	stdout, stderr := &syncWriter{}, &syncWriter{}
	done := make(chan int, 1)
	go func() { done <- runContext(ctx, []string{"--config", configPath}, stdout, stderr) }()

	harness := &oauthTestHarness{
		jwks:        jwks,
		jwksFetches: &jwksFetches,
		private:     private,
		public:      public,
		httpURL:     "http://" + httpAddress,
		opsURL:      "http://" + operationsAddress,
		ctx:         ctx,
		cancel:      cancel,
		done:        done,
		stdout:      stdout,
		stderr:      stderr,
	}
	t.Cleanup(harness.stop)
	return harness
}

// stop cancels the run context and waits for the process to exit.
func (h *oauthTestHarness) stop() {
	h.cancel()
	select {
	case status := <-h.done:
		if status != 0 {
			panic(fmt.Sprintf("bridge exit status = %d (stderr=%s)", status, h.stderr.String()))
		}
	case <-time.After(30 * time.Second):
		panic("bridge did not shut down after cancellation")
	}
}

// awaitLive blocks until the inbound listener answers its liveness probe
// without credentials.
func (h *oauthTestHarness) awaitLive(t *testing.T) {
	t.Helper()
	waitFor(t, "live inbound listener", func() bool {
		status, body, ok := probeBody(t, h.httpURL+"/livez")
		return ok && status == http.StatusOK && strings.Contains(body, "live")
	})
}

// awaitReady blocks until the configured upstream has been discovered and
// published, so MCP requests have a catalog to answer from.
func (h *oauthTestHarness) awaitReady(t *testing.T) {
	t.Helper()
	waitFor(t, "readiness of the configured server", func() bool {
		status, body, ok := probeBody(t, h.opsURL+"/readyz")
		return ok && status == http.StatusOK && strings.Contains(body, `"status":"ready"`) && strings.Contains(body, "alpha")
	})
}

// oauthToken mints an RS256 token signed by the harness key with the given
// claims; overrides replace the default claim values.
func (h *oauthTestHarness) oauthToken(t *testing.T, now time.Time, overrides map[string]any) string {
	t.Helper()
	builder := jwt.NewBuilder().
		Issuer(oauthIssuer).
		Subject(oauthTestSubject).
		Audience([]string{oauthResource}).
		IssuedAt(now).
		Expiration(now.Add(time.Hour)).
		Claim("email", oauthTestSubject+"@example.com").
		Claim("scope", oauthScope)
	for key, value := range overrides {
		builder = builder.Claim(key, value)
	}
	token, err := builder.Build()
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	signingKey, err := jwk.FromRaw(h.private)
	if err != nil {
		t.Fatalf("wrap private key: %v", err)
	}
	if err := signingKey.Set(jwk.KeyIDKey, oauthKeyID); err != nil {
		t.Fatalf("set private kid: %v", err)
	}
	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256, signingKey))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return string(signed)
}

// oauthPost performs one POST against the inbound edge with the given bearer
// value; an empty token sends no Authorization header at all.
func oauthPost(t *testing.T, url, token, body string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, url+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("POST %s/mcp: %v", url, err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

// oauthJSONBody reads and decodes one response body into a JSON object,
// unwrapping the SSE envelope ("event: ...\ndata: {...}") the streamable
// transport answers with for initialize responses.
func oauthJSONBody(t *testing.T, response *http.Response) map[string]any {
	t.Helper()
	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(response.Body); err != nil {
		t.Fatalf("read response body: %v", err)
	}
	raw := buffer.String()
	for _, line := range strings.Split(raw, "\n") {
		if data, ok := strings.CutPrefix(line, "data: "); ok && strings.TrimSpace(data) != "" {
			raw = data
			break
		}
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("decode response body %q: %v", buffer.String(), err)
	}
	return payload
}

// requireChallenge asserts the exact 401 shape: status, body and the byte
// exact WWW-Authenticate challenge.
func requireChallenge(t *testing.T, response *http.Response, what string) {
	t.Helper()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("%s: status = %d; want 401", what, response.StatusCode)
	}
	if got := response.Header.Get("WWW-Authenticate"); got != oauthChallenge {
		t.Fatalf("%s: WWW-Authenticate = %q; want %q", what, got, oauthChallenge)
	}
}

func TestProtectedResourceMetadataWire(t *testing.T) {

	resource := oauthResource + "/mcp"
	t.Setenv("MCP_RESOURCE_URL", resource)

	harness := startOAuthHarness(t)
	harness.awaitLive(t)
	for _, path := range []string{
		"/.well-known/oauth-protected-resource",
		"/.well-known/oauth-protected-resource/mcp",
		"/mcp/.well-known/oauth-protected-resource",
	} {
		t.Run(path, func(t *testing.T) {
			response, err := http.Get(harness.httpURL + path)
			if err != nil {
				t.Fatalf("GET %s: %v", path, err)
			}
			defer response.Body.Close()

			if response.StatusCode != http.StatusOK {
				t.Fatalf("GET %s: status = %d, want %d", path, response.StatusCode, http.StatusOK)
			}
			if got := response.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("GET %s: Content-Type = %q, want %q", path, got, "application/json")
			}

			var metadata struct {
				Resource string `json:"resource"`
			}
			if err := json.NewDecoder(response.Body).Decode(&metadata); err != nil {
				t.Fatalf("decode metadata for %s: %v", path, err)
			}
			if metadata.Resource != resource {
				t.Errorf("GET %s: resource = %q, want %q", path, metadata.Resource, resource)
			}
		})
	}

	challenge := oauthPost(t, harness.httpURL, "", `{}`)
	if challenge.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated MCP request: status = %d, want %d", challenge.StatusCode, http.StatusUnauthorized)
	}
	wantChallenge := `Bearer realm="mcp", resource_metadata="` + resource + `/.well-known/oauth-protected-resource"`
	if got := challenge.Header.Get("WWW-Authenticate"); got != wantChallenge {
		t.Errorf("WWW-Authenticate = %q, want %q", got, wantChallenge)
	}
	if !strings.Contains(harness.stdout.String(), `"metadata_target_registered":"yes"`) {
		t.Error("challenge diagnostics do not classify the metadata target as registered")
	}
}

func TestServeOAuthMissingAuthorizationDiagnostic(t *testing.T) {
	rawLogOutput := &syncWriter{}
	previousLogOutput := log.Writer()
	log.SetOutput(rawLogOutput)
	t.Cleanup(func() { log.SetOutput(previousLogOutput) })

	harness := startOAuthHarness(t)
	harness.awaitLive(t)
	harness.awaitReady(t)

	response := oauthPost(t, harness.httpURL, "", `{}`)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing Authorization: status = %d; want %d", response.StatusCode, http.StatusUnauthorized)
	}
	if got := response.Header.Get("WWW-Authenticate"); got != oauthChallenge {
		t.Fatalf("missing Authorization: WWW-Authenticate = %q; want %q", got, oauthChallenge)
	}

	diagnosticID := requireDiagnosticID(t, response)
	completions := waitForDiagnosticCompletion(t, harness, "missing-credential http_complete", diagnosticID)
	if len(completions) != 1 {
		t.Fatalf("missing-credential completion count = %d; want exactly 1", len(completions))
	}
	requireOuterOAuthRecord(t, completions[0], http.StatusUnauthorized, "credentials_missing", "missing", "not_evaluated")
	for key, want := range map[string]any{
		"auth_mode":                  "oauth",
		"challenge_kind":             "resource_metadata",
		"metadata_target_registered": "yes",
	} {
		if got := completions[0][key]; got != want {
			t.Errorf("missing-credential http_complete %s = %v; want %v", key, got, want)
		}
	}
	if strings.Contains(rawLogOutput.String(), "auth: missing Authorization header") {
		t.Error("missing-credential request emitted a duplicate unstructured auth diagnostic")
	}
}

func TestServeOAuthRouteNormalizationAndRootProbe(t *testing.T) {
	harness := startOAuthHarness(t)
	harness.awaitLive(t)
	harness.awaitReady(t)

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	for _, path := range []string{"/mcp", "/mcp/", "/"} {
		t.Run(path, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, harness.httpURL+path, strings.NewReader(`{}`))
			if err != nil {
				t.Fatalf("build POST %s request: %v", path, err)
			}
			request.Header.Set("Content-Type", "application/json")
			response, err := client.Do(request)
			if err != nil {
				t.Fatalf("POST %s: %v", path, err)
			}
			defer response.Body.Close()

			requireChallenge(t, response, "POST "+path)
			if location := response.Header.Get("Location"); location != "" {
				t.Errorf("POST %s Location = %q; want empty", path, location)
			}
		})
	}
}

// TestServeOAuthWire exercises the full inbound edge through the composition
// root: metadata discovery, bearer authentication against a real JWKS
// endpoint, scope enforcement, the static-token fallback, and the
// unauthenticated health routes.
func TestServeOAuthWire(t *testing.T) {
	harness := startOAuthHarness(t)
	harness.awaitLive(t)
	harness.awaitReady(t)

	now := time.Now().UTC()

	// 1. Metadata discovery: the protected-resource document advertises this
	// deployment's resource, authorization server, scopes and bearer methods.
	status, body := getBody(t, harness.httpURL+"/.well-known/oauth-protected-resource")
	if status != http.StatusOK {
		t.Fatalf("metadata status = %d; want 200", status)
	}
	var metadata struct {
		Resource               string   `json:"resource"`
		AuthorizationServers   []string `json:"authorization_servers"`
		ScopesSupported        []string `json:"scopes_supported"`
		BearerMethodsSupported []string `json:"bearer_methods_supported"`
	}
	if err := json.Unmarshal([]byte(body), &metadata); err != nil {
		t.Fatalf("decode metadata %q: %v", body, err)
	}
	if metadata.Resource != oauthResource {
		t.Fatalf("metadata resource = %q; want %q", metadata.Resource, oauthResource)
	}
	if len(metadata.AuthorizationServers) != 1 || metadata.AuthorizationServers[0] != oauthIssuer {
		t.Fatalf("metadata authorization_servers = %v; want [%s]", metadata.AuthorizationServers, oauthIssuer)
	}
	if !containsString(metadata.ScopesSupported, oauthScope) {
		t.Fatalf("metadata scopes_supported = %v; want %q present", metadata.ScopesSupported, oauthScope)
	}
	if len(metadata.BearerMethodsSupported) != 1 || metadata.BearerMethodsSupported[0] != "header" {
		t.Fatalf("metadata bearer_methods_supported = %v; want [header]", metadata.BearerMethodsSupported)
	}

	// 2. No credentials: the anonymous request is refused with the exact
	// unauthenticated challenge.
	requireChallenge(t, oauthPost(t, harness.httpURL, "", `{}`), "anonymous request")

	// 3. Expired token: verification rejects it with the same challenge.
	expired := harness.oauthToken(t, now.Add(-2*time.Hour), nil)
	requireChallenge(t, oauthPost(t, harness.httpURL, expired,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"oauth-wire","version":"1"}}}`), "expired token")

	// 4a. Wrong issuer: a well-signed token from another issuer is refused.
	foreign := harness.oauthToken(t, now, map[string]any{"iss": "https://evil.example.com"})
	requireChallenge(t, oauthPost(t, harness.httpURL, foreign, `{}`), "wrong issuer")

	// 4b. Invalid signature: a token signed by an unknown key is refused.
	roguePrivate, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rogue key: %v", err)
	}
	rogueToken := harness.oauthToken(t, now, nil)
	if err != nil {
		t.Fatalf("build rogue token: %v", err)
	}
	unsigned, err := jwt.Parse([]byte(rogueToken), jwt.WithVerify(false))
	if err != nil {
		t.Fatalf("parse token for re-signing: %v", err)
	}
	rogueSigned, err := jwt.Sign(unsigned, jwt.WithKey(jwa.RS256, roguePrivate))
	if err != nil {
		t.Fatalf("sign rogue token: %v", err)
	}
	requireChallenge(t, oauthPost(t, harness.httpURL, string(rogueSigned), `{}`), "invalid signature")

	// 5. Missing scope: an otherwise valid token without mcp:connect is
	// refused with the insufficient_scope challenge.
	unscoped := harness.oauthToken(t, now, map[string]any{"scope": "openid email"})
	scopedOut := oauthPost(t, harness.httpURL, unscoped, `{}`)
	if scopedOut.StatusCode != http.StatusForbidden {
		t.Fatalf("missing scope: status = %d; want 403", scopedOut.StatusCode)
	}
	wantScopeChallenge := `Bearer error="insufficient_scope", scope="` + oauthScope + `"`
	if got := scopedOut.Header.Get("WWW-Authenticate"); got != wantScopeChallenge {
		t.Fatalf("missing scope: WWW-Authenticate = %q; want %q", got, wantScopeChallenge)
	}

	// 6. Valid token with the required scope: the request reaches the MCP
	// server, which answers an initialize and issues a session id.
	authorized := harness.oauthToken(t, now, nil)
	accepted := oauthPost(t, harness.httpURL, authorized,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"oauth-wire","version":"1"}}}`)
	if accepted.StatusCode != http.StatusOK {
		payload, _ := readAll(accepted)
		t.Fatalf("authorized request: status = %d; want 200 (body=%s)", accepted.StatusCode, payload)
	}
	result := oauthJSONBody(t, accepted)
	if result["jsonrpc"] != "2.0" {
		t.Fatalf("initialize response = %v; want jsonrpc 2.0", result)
	}
	if _, ok := result["result"]; !ok {
		t.Fatalf("initialize response lacks result: %v", result)
	}
	sessionID := accepted.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatalf("initialize omitted session ID")
	}

	// A follow-up call on that session is admitted with the same token; the
	// transport answers a notification with 202 Accepted.
	followUp := oauthPost(t, harness.httpURL, authorized,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if followUp.StatusCode != http.StatusOK && followUp.StatusCode != http.StatusAccepted {
		payload, _ := readAll(followUp)
		t.Fatalf("initialized notification: status = %d; want 200 or 202 (body=%s)", followUp.StatusCode, payload)
	}

	// 7. Static token: the configured client token in the YAML document is
	// admitted through the dual-mode fallback untouched.
	static := oauthPost(t, harness.httpURL, serveClientToken,
		`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"oauth-wire","version":"1"}}}`)
	if static.StatusCode != http.StatusOK {
		payload, _ := readAll(static)
		t.Fatalf("static token: status = %d; want 200 (body=%s)", static.StatusCode, payload)
	}
	if oauthJSONBody(t, static)["jsonrpc"] != "2.0" {
		t.Fatalf("static token response = %v; want jsonrpc 2.0", oauthJSONBody(t, static))
	}

	// 8. Health routes answer without credentials on the inbound edge.
	for _, route := range []struct{ path, body string }{
		{"/livez", "live\n"},
		{"/healthz", "ok\n"},
	} {
		status, body := getBody(t, harness.httpURL+route.path)
		if status != http.StatusOK {
			t.Fatalf("GET %s: status = %d; want 200", route.path, status)
		}
		if body != route.body {
			t.Fatalf("GET %s: body = %q; want %q", route.path, body, route.body)
		}
	}

	// The JWKS endpoint saw traffic: the middleware fetched the key set at
	// least once during the run.
	if fetched := harness.jwksFetches.Load(); fetched == 0 {
		t.Fatalf("jwks endpoint served %d requests; want at least one", fetched)
	}
}

// containsString reports whether needle is present in haystack.
func containsString(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}

// readAll drains one response body into a string, tolerating failures.
func readAll(response *http.Response) (string, error) {
	var buffer bytes.Buffer
	_, err := buffer.ReadFrom(response.Body)
	return buffer.String(), err
}
func TestServeOAuthSessionBoundAdmissionLifecycle(t *testing.T) {
	const (
		resourceURL  = "https://mcp.bullprog.com/mcp"
		metadataPath = "/mcp/.well-known/oauth-protected-resource"
	)
	t.Setenv("MCP_RESOURCE_URL", resourceURL)
	t.Setenv("MCP_SESSION_ADMISSION_ENABLED", "true")
	harness := startOAuthHarness(t)
	harness.awaitReady(t)

	contains := func(value, substring string) bool {
		for i := 0; i+len(substring) <= len(value); i++ {
			if value[i:i+len(substring)] == substring {
				return true
			}
		}
		return false
	}
	post := func(token, sessionID, body string) *http.Response {
		request, err := http.NewRequest(http.MethodPost, harness.httpURL+"/mcp", bytes.NewBufferString(body))
		if err != nil {
			t.Fatalf("create MCP request: %v", err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		if sessionID != "" {
			request.Header.Set("Mcp-Session-Id", sessionID)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("send MCP request: %v", err)
		}
		return response
	}
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"session-bound-admission","version":"1"}}}`

	// Probe the protected resource without credentials and discover its
	// canonical protected-resource metadata URL from the challenge.
	discovery := post("", "", initialize)
	if discovery.StatusCode != http.StatusUnauthorized {
		payload, _ := readAll(discovery)
		discovery.Body.Close()
		t.Fatalf("anonymous discovery probe: status = %d; want 401 (body=%s)", discovery.StatusCode, payload)
	}
	metadataURL := "https://mcp.bullprog.com" + metadataPath
	challenge := discovery.Header.Get("WWW-Authenticate")
	discovery.Body.Close()
	if challenge == "" || !contains(challenge, metadataURL) {
		t.Fatalf("discovery challenge = %q; want canonical metadata URL %q", challenge, metadataURL)
	}

	// The harness serves the canonical metadata path locally while advertising
	// the configured canonical resource URL to clients.
	metadataRequest, err := http.NewRequest(http.MethodGet, harness.httpURL+metadataPath, nil)
	if err != nil {
		t.Fatalf("create metadata request: %v", err)
	}
	metadataResponse, err := http.DefaultClient.Do(metadataRequest)
	if err != nil {
		t.Fatalf("fetch protected-resource metadata: %v", err)
	}
	if metadataResponse.StatusCode != http.StatusOK {
		payload, _ := readAll(metadataResponse)
		metadataResponse.Body.Close()
		t.Fatalf("protected-resource metadata: status = %d; want 200 (body=%s)", metadataResponse.StatusCode, payload)
	}
	metadata := oauthJSONBody(t, metadataResponse)
	metadataResponse.Body.Close()
	if got := metadata["resource"]; got != resourceURL {
		t.Fatalf("metadata resource = %v; want canonical resource %q", got, resourceURL)
	}

	now := time.Now()
	token := harness.oauthToken(t, now, nil)
	initialized := post(token, "", initialize)
	if initialized.StatusCode != http.StatusOK {
		payload, _ := readAll(initialized)
		initialized.Body.Close()
		t.Fatalf("initialize: status = %d; want 200 (body=%s)", initialized.StatusCode, payload)
	}
	if response := oauthJSONBody(t, initialized); response["jsonrpc"] != "2.0" {
		initialized.Body.Close()
		t.Fatalf("initialize response = %v; want JSON-RPC 2.0", response)
	}
	sessionID := initialized.Header.Get("Mcp-Session-Id")
	initialized.Body.Close()
	if sessionID == "" {
		t.Fatal("initialize omitted Mcp-Session-Id")
	}

	initializedNotification := post("", sessionID, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if initializedNotification.StatusCode != http.StatusOK && initializedNotification.StatusCode != http.StatusAccepted {
		payload, _ := readAll(initializedNotification)
		initializedNotification.Body.Close()
		t.Fatalf("initialized notification: status = %d; want 200 or 202 (body=%s)", initializedNotification.StatusCode, payload)
	}
	initializedNotification.Body.Close()

	listTools := func(token string) *http.Response {
		return post(token, sessionID, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	}
	listed := listTools("")
	if listed.StatusCode != http.StatusOK {
		payload, _ := readAll(listed)
		listed.Body.Close()
		t.Fatalf("tools/list without Authorization: status = %d; want 200 (body=%s)", listed.StatusCode, payload)
	}
	listResult := oauthJSONBody(t, listed)
	listed.Body.Close()
	result, ok := listResult["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/list response lacks result: %v", listResult)
	}
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatalf("tools/list catalog = %v; want at least one tool", result["tools"])
	}
	firstTool, ok := tools[0].(map[string]any)
	if !ok {
		t.Fatalf("tools/list first tool = %v; want an object", tools[0])
	}
	toolName, ok := firstTool["name"].(string)
	if !ok || toolName == "" {
		t.Fatalf("tools/list first tool has no name: %v", firstTool)
	}
	callBody := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"` + toolName + `","arguments":{}}}`
	called := post("", sessionID, callBody)
	if called.StatusCode != http.StatusOK {
		payload, _ := readAll(called)
		called.Body.Close()
		t.Fatalf("tools/call without Authorization: status = %d; want 200 (body=%s)", called.StatusCode, payload)
	}
	called.Body.Close()

	// A fresh valid token on the active session renews the binding; subsequent
	// tokenless traffic must remain admitted.
	rotatedToken := harness.oauthToken(t, now.Add(time.Second), nil)
	rotated := listTools(rotatedToken)
	if rotated.StatusCode != http.StatusOK {
		payload, _ := readAll(rotated)
		rotated.Body.Close()
		t.Fatalf("token rotation: status = %d; want 200 (body=%s)", rotated.StatusCode, payload)
	}
	rotated.Body.Close()
	afterRotation := listTools("")
	if afterRotation.StatusCode != http.StatusOK {
		payload, _ := readAll(afterRotation)
		afterRotation.Body.Close()
		t.Fatalf("tokenless request after rotation: status = %d; want 200 (body=%s)", afterRotation.StatusCode, payload)
	}
	afterRotation.Body.Close()

	unknownSession := post("", "unknown-session-id", `{"jsonrpc":"2.0","id":4,"method":"tools/list"}`)
	if unknownSession.StatusCode != http.StatusUnauthorized {
		payload, _ := readAll(unknownSession)
		unknownSession.Body.Close()
		t.Fatalf("unknown session without Authorization: status = %d; want 401 (body=%s)", unknownSession.StatusCode, payload)
	}
	if got := unknownSession.Header.Get("WWW-Authenticate"); !strings.HasPrefix(got, "Bearer realm=\"mcp\"") || !contains(got, "resource_metadata=") {
		unknownSession.Body.Close()
		t.Fatalf("unknown session challenge = %q; want Bearer realm=\"mcp\" with resource_metadata", got)
	}
	unknownSession.Body.Close()

	invalidToken := post("invalid-token", sessionID, `{"jsonrpc":"2.0","id":5,"method":"tools/list"}`)
	if invalidToken.StatusCode != http.StatusUnauthorized {
		payload, _ := readAll(invalidToken)
		invalidToken.Body.Close()
		t.Fatalf("invalid token on active session: status = %d; want 401 (body=%s)", invalidToken.StatusCode, payload)
	}
	invalidToken.Body.Close()
}

func TestServeOAuthActionDiscoveryEndToEnd(t *testing.T) {
	const (
		resourceURL    = "https://mcp.bullprog.com/mcp"
		resourceOrigin = "https://mcp.bullprog.com"
		metadataPath   = "/mcp/.well-known/oauth-protected-resource"
	)
	t.Setenv("MCP_RESOURCE_URL", resourceURL)
	harness := startOAuthHarness(t)
	harness.awaitReady(t)

	initializeRequest := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"action-discovery","version":"1"}}}`
	publicID, internalID, diagnosticID := diagCanaries(91)
	challengeResponse := diagPost(t, harness, "", "", map[string]string{
		"X-Request-ID": publicID,
	}, initializeRequest)
	defer challengeResponse.Body.Close()
	if challengeResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated initialize: status = %d; want 401", challengeResponse.StatusCode)
	}
	metadataURL := resourceOrigin + metadataPath
	wantChallenge := `Bearer realm="mcp", resource_metadata="` + metadataURL + `"`
	if got := challengeResponse.Header.Get("WWW-Authenticate"); got != wantChallenge {
		t.Fatalf("WWW-Authenticate = %q; want %q", got, wantChallenge)
	}

	ownedDiagnosticID := requireDiagnosticID(t, challengeResponse, publicID, internalID, diagnosticID)
	records := waitForDiagnosticCompletion(t, harness, "action discovery challenge", ownedDiagnosticID)
	if len(records) != 1 {
		t.Fatalf("challenge diagnostic completions = %d; want exactly 1", len(records))
	}
	requireCompletionField(t, records[0], "challenge_target", "path_appended_well_known")
	requireCompletionField(t, records[0], "metadata_target_registered", "yes")

	metadataTarget := strings.TrimPrefix(metadataURL, resourceOrigin)
	metadataResponse, err := http.Get(harness.httpURL + metadataTarget)
	if err != nil {
		t.Fatalf("GET metadata target %q: %v", metadataTarget, err)
	}
	defer metadataResponse.Body.Close()
	if metadataResponse.StatusCode != http.StatusOK {
		t.Fatalf("metadata status = %d; want 200", metadataResponse.StatusCode)
	}
	if got := metadataResponse.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("metadata Content-Type = %q; want application/json", got)
	}
	metadata := oauthJSONBody(t, metadataResponse)
	if metadata["resource"] != resourceURL {
		t.Fatalf("metadata resource = %v; want %q", metadata["resource"], resourceURL)
	}

	for _, audience := range []string{resourceURL, resourceOrigin} {
		token := harness.oauthToken(t, time.Now(), map[string]any{"aud": audience})
		accepted := oauthPost(t, harness.httpURL, token, initializeRequest)
		if accepted.StatusCode != http.StatusOK {
			payload, _ := readAll(accepted)
			t.Fatalf("initialize with audience %q: status = %d; want 200 (body=%s)", audience, accepted.StatusCode, payload)
		}
		result := oauthJSONBody(t, accepted)
		if result["jsonrpc"] != "2.0" {
			t.Fatalf("initialize with audience %q: jsonrpc = %v; want 2.0", audience, result["jsonrpc"])
		}
		if rpcResult, ok := result["result"].(map[string]any); !ok || rpcResult == nil {
			t.Fatalf("initialize with audience %q: result = %v; want a JSON object", audience, result["result"])
		}
		if sessionID := accepted.Header.Get("Mcp-Session-Id"); sessionID == "" {
			t.Fatalf("initialize with audience %q omitted Mcp-Session-Id", audience)
		}
	}
}
func TestServeOAuthSessionAuthorizationCeilingWire(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	claims := &auth.Claims{Subject: "wire-user", Issuer: "https://issuer.example"}
	claimsByToken := map[string]*auth.Claims{
		"token-one": claims,
		"token-two": claims,
	}
	tokenHashes := [][32]byte{sha256.Sum256([]byte("token-one")), sha256.Sum256([]byte("token-two"))}
	inbound := service.NewInboundHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Mcp-Session-Id") == "" {
			w.Header().Set("Mcp-Session-Id", "wire-session")
		}
		w.WriteHeader(http.StatusAccepted)
	}), tokenHashes, service.InboundOptions{Now: func() time.Time { return now }})
	authentication := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var admittedClaims *auth.Claims
		var ok bool
		if authorization := r.Header.Get("Authorization"); authorization != "" {
			admittedClaims, ok = claimsByToken[strings.TrimPrefix(authorization, "Bearer ")]
		} else {
			admittedClaims, ok = inbound.SessionClaims(r.Header.Get("Mcp-Session-Id"))
		}
		if !ok || admittedClaims == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		inbound.ServeHTTP(w, r.WithContext(auth.ContextWithClaims(r.Context(), admittedClaims)))
	})
	server := httptest.NewServer(authentication)
	defer server.Close()

	send := func(token, sessionID string) (int, http.Header) {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, server.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		if sessionID != "" {
			request.Header.Set("Mcp-Session-Id", sessionID)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, response.Body)
		return response.StatusCode, response.Header.Clone()
	}

	status, headers := send("token-one", "")
	if status != http.StatusAccepted || headers.Get("Mcp-Session-Id") != "wire-session" {
		t.Fatalf("initialize status=%d session=%q, want 202 and wire-session", status, headers.Get("Mcp-Session-Id"))
	}
	now = now.Add(8*time.Hour + time.Second)
	if status, _ = send("", "wire-session"); status != http.StatusUnauthorized {
		t.Fatalf("tokenless request after ceiling status=%d, want 401", status)
	}
	if status, _ = send("token-two", "wire-session"); status != http.StatusAccepted {
		t.Fatalf("fresh bearer renewal status=%d, want 202", status)
	}
	now = time.Date(2026, time.January, 2, 19, 4, 5, 0, time.UTC)
	if status, _ = send("", "wire-session"); status != http.StatusAccepted {
		t.Fatalf("tokenless request after renewal status=%d, want 202", status)
	}
}
func TestChatGPTOAuthAndActionDiscoveryWire(t *testing.T) {
	const resourceURL = "https://mcp.bullprog.com"

	t.Setenv("MCP_RESOURCE_URL", resourceURL)
	t.Setenv("MCP_SESSION_ADMISSION_ENABLED", "true")
	t.Setenv("MCP_REQUIRED_SCOPE", "mcp:connect")

	h := startOAuthHarness(t)
	h.awaitLive(t)
	h.awaitReady(t)
	token := h.oauthToken(t, time.Now().UTC(), map[string]any{"aud": resourceURL, "scope": "mcp:connect"})

	post := func(body, token, sessionID string) *http.Response {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, h.httpURL+"/mcp", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		if sessionID != "" {
			request.Header.Set("Mcp-Session-Id", sessionID)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}

	probe := post(`{}`, "", "")
	defer probe.Body.Close()
	if probe.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated probe status = %d; want 401", probe.StatusCode)
	}
	if got := probe.Header.Get("WWW-Authenticate"); !strings.Contains(got, `resource_metadata="https://mcp.bullprog.com/.well-known/oauth-protected-resource"`) {
		t.Fatalf("unauthenticated probe WWW-Authenticate = %q; want origin metadata target", got)
	}

	discovery, err := http.Get(h.httpURL + "/.well-known/oauth-protected-resource")
	if err != nil {
		t.Fatalf("GET protected resource metadata: %v", err)
	}
	defer discovery.Body.Close()
	if discovery.StatusCode != http.StatusOK {
		t.Fatalf("protected resource metadata status = %d; want 200", discovery.StatusCode)
	}
	metadata := oauthJSONBody(t, discovery)
	if got := metadata["resource"]; got != resourceURL {
		t.Fatalf("protected resource metadata resource = %v; want %q", got, resourceURL)
	}

	initialize := post(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"chatgpt","version":"1.0"}}}`, token, "")
	defer initialize.Body.Close()
	if initialize.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d; want 200", initialize.StatusCode)
	}
	sessionID := initialize.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("expected session ID")
	}

	notification := post(`{"jsonrpc":"2.0","method":"notifications/initialized"}`, "", sessionID)
	defer notification.Body.Close()
	if notification.StatusCode != http.StatusAccepted {
		t.Fatalf("initialized notification status = %d; want 202", notification.StatusCode)
	}
	notificationBody, err := io.ReadAll(notification.Body)
	if err != nil {
		t.Fatalf("read initialized notification body: %v", err)
	}
	if strings.TrimSpace(string(notificationBody)) != "" {
		t.Fatalf("initialized notification body = %q; want empty", notificationBody)
	}

	toolList := post(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`, "", sessionID)
	defer toolList.Body.Close()
	if toolList.StatusCode != http.StatusOK {
		t.Fatalf("tools/list status = %d; want 200", toolList.StatusCode)
	}
	toolListBody := oauthJSONBody(t, toolList)
	if toolListBody["jsonrpc"] != "2.0" {
		t.Fatalf("tools/list jsonrpc = %v; want 2.0", toolListBody["jsonrpc"])
	}
	result, ok := toolListBody["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/list result = %v; want JSON object", toolListBody["result"])
	}
	if _, ok := result["tools"].([]any); !ok {
		t.Fatalf("tools/list result tools = %v; want JSON array", result["tools"])
	}

	negativeResponse := post(`{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`, "", "unknown-bogus-session")
	defer negativeResponse.Body.Close()
	captured := h.stdout.String()
	if negativeResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unknown-session request status = %d; want 401", negativeResponse.StatusCode)
	}
	diagID := negativeResponse.Header.Get("mcp-diagnostic-id")
	if diagID == "" {
		t.Fatal("unknown-session response omitted mcp-diagnostic-id")
	}
	matchingRecords := 0
	for _, line := range strings.Split(strings.TrimSpace(captured), "\n") {
		var record struct {
			RequestID     string `json:"request_id"`
			FailureStage  string `json:"failure_stage"`
			FailureReason string `json:"failure_reason"`
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		if record.RequestID != diagID {
			continue
		}
		matchingRecords++
		if record.FailureStage != "outer_oauth" || (record.FailureReason != "credentials_missing" && record.FailureReason != "session_absent") {
			t.Errorf("diagnostic record for %q has failure_stage=%q failure_reason=%q; want outer_oauth/credentials_missing or session_absent", diagID, record.FailureStage, record.FailureReason)
		}
	}
	if matchingRecords != 1 {
		t.Fatalf("stdout contained %d diagnostic records for request_id %q; want exactly one (stdout=%q)", matchingRecords, diagID, captured)
	}
}
func TestChatGPTDeveloperModeStatelessDiscoveryWire(t *testing.T) {
	const resourceURL = "https://mcp.bullprog.com"

	t.Setenv("MCP_RESOURCE_URL", resourceURL)
	t.Setenv("MCP_STATELESS", "true")

	h := startOAuthHarness(t)
	h.awaitLive(t)
	h.awaitReady(t)
	token := h.oauthToken(t, time.Now().UTC(), map[string]any{"aud": "https://mcp.bullprog.com", "scope": "mcp:connect"})

	post := func(method, toolName, body string) *http.Response {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, h.httpURL+"/mcp", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Mcp-Protocol-Version", "2026-07-28")
		request.Header.Set("Mcp-Method", method)
		if toolName != "" {
			request.Header.Set("Mcp-Name", toolName)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}

	discovery := post("server/discover", "", `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"wire-client","version":"1"},"io.modelcontextprotocol/clientCapabilities":{"sampling":{}}}}}`)
	defer discovery.Body.Close()
	if discovery.StatusCode != http.StatusOK {
		t.Fatalf("server/discover status = %d; want 200", discovery.StatusCode)
	}

	toolList := post("tools/list", "", `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{"sampling":{}}}}}`)
	defer toolList.Body.Close()
	toolListBody := oauthJSONBody(t, toolList)
	if toolListBody["jsonrpc"] != "2.0" {
		t.Fatalf("tools/list jsonrpc = %v; want 2.0", toolListBody["jsonrpc"])
	}
	if id, ok := toolListBody["id"].(float64); !ok || int(id) != 2 {
		t.Fatalf("tools/list id = %v; want 2", toolListBody["id"])
	}
	if toolListBody["error"] != nil {
		t.Fatalf("tools/list error = %v; want nil", toolListBody["error"])
	}
	result, ok := toolListBody["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/list result missing or not object: %v", toolListBody)
	}
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatalf("tools/list tools missing or empty: %v", result)
	}

	toolCall := post("tools/call", "search_tools", `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search_tools","arguments":{"query":"test"},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{"sampling":{}}}}}`)
	defer toolCall.Body.Close()
	if toolCall.StatusCode != http.StatusOK {
		t.Fatalf("tools/call status = %d; want 200", toolCall.StatusCode)
	}
}

func TestDiagnosticDepsWiringCoverage(t *testing.T) {
	ctx := context.Background()
	nowFn := func() time.Time { return time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC) }
	idFn := func() string { return "test-id" }
	deps := diagnosticDeps{Now: nowFn, NewID: idFn}
	ctx = withDiagnosticDeps(ctx, deps)
	if gotDeps, ok := diagnosticDepsFromContext(ctx); !ok || gotDeps.Now == nil {
		t.Fatal("expected diagnostic deps in context")
	}
	if diagnosticNowFromContext(ctx)() != nowFn() {
		t.Fatal("diagnosticNowFromContext mismatch")
	}
	if diagnosticNewIDFromContext(ctx)() != "test-id" {
		t.Fatal("diagnosticNewIDFromContext mismatch")
	}
	if !isSafeServerID("alpha-1") || isSafeServerID("") || isSafeServerID("Alpha") || isSafeServerID("tool_1") {
		t.Fatal("isSafeServerID validation failure")
	}
}
func TestWiringEdgeCases(t *testing.T) {
	err1 := errors.New("err1")
	err2 := errors.New("err2")
	if firstError(nil, nil) != nil || firstError(err1, nil) != err1 || firstError(nil, err2) != err2 || firstError(err1, err2) != err1 {
		t.Fatal("firstError logic failure")
	}
	holder := newConfigHolder()
	reg := newUpstreamRegistry(holder)
	s := config.Server{ID: "test", Endpoint: "http://example.com"}
	if _, err := reg.newClient(s); err == nil {
		t.Fatal("expected error with no staged config")
	}
	if _, err := reg.discover(s); err == nil {
		t.Fatal("expected error with no client for server")
	}
	if _, err := reg.discoverPrompts(s); err == nil {
		t.Fatal("expected error with no client for server")
	}
	if err := reg.close(nil); err != nil {
		t.Fatalf("close(nil) = %v", err)
	}

	// Exercise all branches of restartOnlyField
	cases := []struct {
		name      string
		candidate config.Command
		want      string
	}{
		{"identical", config.Command{}, ""},
		{"http", config.Command{HTTPListen: "1"}, "http.listen"},
		{"ops", config.Command{OperationsListen: "1"}, "operations.listen"},
		{"dir", config.Command{StorageDirectory: "1"}, "storage.directory"},
		{"snap", config.Command{MaxSnapshotBytes: 1}, "storage.max_snapshot_bytes"},
		{"refresh", config.Command{RefreshInterval: 1}, "catalog.refresh_interval"},
		{"rate", config.Command{OpsRequestsPerSecond: 1}, "operations.requests_per_second"},
		{"burst", config.Command{OpsBurst: 1}, "operations.burst"},
		{"search", config.Command{Search: config.SearchSettings{DefaultTopK: 1}}, "search"},
		{"embed", config.Command{Embedding: config.EmbeddingSettings{Model: "1"}}, "embedding"},
	}
	base := config.Command{}
	for _, tc := range cases {
		if got := restartOnlyField(base, tc.candidate); got != tc.want {
			t.Fatalf("restartOnlyField(%s) = %q, want %q", tc.name, got, tc.want)
		}
	}

	// Exercise configHolder methods
	if holder.publishedSettings() != nil {
		t.Fatal("expected nil publishedSettings initially")
	}
	holder.commit() // commit without staged is a no-op
	if s := holder.settings(); s.MaxRequestBytes != 0 {
		t.Fatal("expected empty settings initially")
	}
	if st := holder.stagedSettings(); st.MaxRequestBytes != 0 {
		t.Fatal("expected empty stagedSettings initially")
	}

	cmd := config.Command{MaxRequestBytes: 1024, MaxSessions: 10}
	holder.stage(&config.Config{}, cmd)
	if b := holder.bounds(); b.MaxRequestBytes != 1024 || b.MaxSessions != 10 {
		t.Fatal("bounds mismatch")
	}
	if st := holder.stagedSettings(); st.MaxRequestBytes != 1024 {
		t.Fatal("stagedSettings mismatch")
	}
	holder.commit()
	if pub := holder.publishedSettings(); pub == nil || pub.MaxRequestBytes != 1024 {
		t.Fatal("expected publishedSettings after commit")
	}
	if s := holder.settings(); s.MaxRequestBytes != 1024 {
		t.Fatal("settings mismatch")
	}
	// embedder error branches (wiring.go:261, 271, 274)
	embedFn := embedder(&mockEmbedProvider{}, search.ModelIdentity{}, config.EmbeddingSettings{Timeout: time.Second})
	if res, err := embedFn(nil); res != nil || err != nil {
		t.Fatal("expected nil, nil on empty documents")
	}

	embedFnErr := embedder(&mockEmbedProvider{err: errors.New("embed error")}, search.ModelIdentity{}, config.EmbeddingSettings{Timeout: time.Second})
	if _, err := embedFnErr([]search.Document{{RawName: "a"}}); err == nil {
		t.Fatal("expected error from embed provider")
	}

	embedFnLen := embedder(&mockEmbedProvider{vectors: [][]float64{}}, search.ModelIdentity{}, config.EmbeddingSettings{Timeout: time.Second})
	if _, err := embedFnLen([]search.Document{{RawName: "a"}}); err == nil {
		t.Fatal("expected length mismatch error from embed provider")
	}

	// publishObserver line 371
	obs := publishObserver(nil, nil, nil, nil)
	obs(nil)
}

type mockEmbedProvider struct {
	vectors [][]float64
	err     error
}

func (m *mockEmbedProvider) Embed(ctx context.Context, inputs []string) ([][]float64, error) {
	return m.vectors, m.err
}
func TestChatGPTSessionAdmissionDiscoveryBurstWire(t *testing.T) {
	const resourceURL = "https://mcp.bullprog.com"
	t.Setenv("MCP_RESOURCE_URL", resourceURL)
	t.Setenv("MCP_SESSION_ADMISSION_ENABLED", "true")
	t.Setenv("MCP_REQUIRED_SCOPE", "mcp:connect")

	now := time.Now().UTC()
	var clockMu sync.Mutex
	simulatedNow := now
	testServeNow = func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return simulatedNow
	}
	t.Cleanup(func() { testServeNow = nil })
	advanceClock := func(duration time.Duration) {
		clockMu.Lock()
		simulatedNow = simulatedNow.Add(duration)
		clockMu.Unlock()
	}

	h := startOAuthHarness(t)
	h.awaitLive(t)
	h.awaitReady(t)

	validToken := h.oauthToken(t, now, map[string]any{"aud": resourceURL, "scope": "mcp:connect", "exp": now.Add(10 * time.Minute).Unix()})
	insufficientScopeToken := h.oauthToken(t, now, map[string]any{"aud": resourceURL, "scope": "openid", "exp": now.Add(10 * time.Minute).Unix()})

	statusCounts := make(map[int]int)
	var rejectedHeaders []string
	var diagnosticIDs []string
	post := func(path, body, token, sessionID string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, h.httpURL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		statusCounts[resp.StatusCode]++
		if resp.StatusCode >= http.StatusBadRequest {
			rejectedHeaders = append(rejectedHeaders, fmt.Sprint(resp.Header))
		}
		diagnosticIDs = append(diagnosticIDs, resp.Header.Get("mcp-diagnostic-id"))
		return resp
	}

	var allBodies []string

	// 1. Probe without session header -> 401 outer_oauth/credentials_missing, session_header: absent
	probe := post("/mcp", "{}", "", "")
	probeBody, _ := io.ReadAll(probe.Body)
	_ = probe.Body.Close()
	allBodies = append(allBodies, string(probeBody))
	if probe.StatusCode != http.StatusUnauthorized {
		t.Fatalf("probe status=%d, want 401", probe.StatusCode)
	}
	requireChallenge(t, probe, "probe")
	if string(probeBody) != "unauthorized\n" {
		t.Fatalf("probe body=%q, want unauthorized response", probeBody)
	}

	// 2. Initialize with valid OAuth token -> 200 OK, Mcp-Session-Id, session_state: created
	initResp := post("/mcp", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"chatgpt","version":"1.0"}}}`, validToken, "")
	initBody, _ := io.ReadAll(initResp.Body)
	_ = initResp.Body.Close()
	allBodies = append(allBodies, string(initBody))
	if initResp.StatusCode != http.StatusOK {
		t.Fatalf("init status=%d, want 200", initResp.StatusCode)
	}
	if recordID := initResp.Header.Get("mcp-diagnostic-id"); recordID == "" {
		t.Fatal("initialize response omitted mcp-diagnostic-id")
	}
	sessionID := initResp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("init response missing Mcp-Session-Id header")
	}

	// 3. Tokenless discovery burst -> 200 OK
	discResp := post("/mcp", `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`, "", sessionID)
	discBody, _ := io.ReadAll(discResp.Body)
	_ = discResp.Body.Close()
	allBodies = append(allBodies, string(discBody))
	if discResp.StatusCode != http.StatusOK {
		t.Fatalf("tokenless discovery burst status=%d, want 200", discResp.StatusCode)
	}
	jsonRPCBody := string(discBody)
	for _, line := range strings.Split(jsonRPCBody, "\n") {
		if strings.HasPrefix(line, "data: ") {
			jsonRPCBody = strings.TrimPrefix(line, "data: ")
			break
		}
	}
	var discoveryReply struct {
		JSONRPC string          `json:"jsonrpc"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal([]byte(jsonRPCBody), &discoveryReply); err != nil {
		t.Fatalf("tokenless discovery body is not valid JSON-RPC (%s): %v", discResp.Header.Get("Content-Type"), err)
	}
	if discoveryReply.JSONRPC != "2.0" || len(discoveryReply.Result) == 0 {
		t.Fatalf("tokenless discovery returned invalid JSON-RPC response")
	}

	// 4. Unknown session ID -> 401 outer_oauth/session_absent
	unknownResp := post("/mcp", `{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`, "", "unknown-session-1234")
	unknownBody, _ := io.ReadAll(unknownResp.Body)
	_ = unknownResp.Body.Close()
	allBodies = append(allBodies, string(unknownBody))
	if unknownResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unknown session status=%d, want 401", unknownResp.StatusCode)
	}
	requireChallenge(t, unknownResp, "unknown session")
	if string(unknownBody) != "unauthorized\n" {
		t.Fatalf("unknown session body=%q, want unauthorized response", unknownBody)
	}

	// 5. An unauthenticated static-token session cannot admit tokenless discovery.
	staticInit := post("/mcp", `{"jsonrpc":"2.0","id":4,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"chatgpt","version":"1.0"}}}`, serveClientToken, "")
	staticBody, _ := io.ReadAll(staticInit.Body)
	_ = staticInit.Body.Close()
	allBodies = append(allBodies, string(staticBody))
	if staticInit.StatusCode != http.StatusOK {
		t.Fatalf("static-token init status=%d, want 200", staticInit.StatusCode)
	}
	unauthenticatedSessionID := staticInit.Header.Get("Mcp-Session-Id")
	if unauthenticatedSessionID == "" {
		t.Fatal("static-token init response missing Mcp-Session-Id header")
	}
	unauthenticatedResp := post("/mcp", `{"jsonrpc":"2.0","id":5,"method":"tools/list","params":{}}`, "", unauthenticatedSessionID)
	unauthenticatedBody, _ := io.ReadAll(unauthenticatedResp.Body)
	_ = unauthenticatedResp.Body.Close()
	allBodies = append(allBodies, string(unauthenticatedBody))
	requireChallenge(t, unauthenticatedResp, "unauthenticated session")
	if string(unauthenticatedBody) != "unauthorized\n" {
		t.Fatalf("unauthenticated session body=%q, want unauthorized response", unauthenticatedBody)
	}

	// 6. Insufficient OAuth scope returns 403; an authenticated nonexistent route returns 404.
	scopeResp := post("/mcp", `{"jsonrpc":"2.0","id":6,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"chatgpt","version":"1.0"}}}`, insufficientScopeToken, "")
	scopeBody, _ := io.ReadAll(scopeResp.Body)
	_ = scopeResp.Body.Close()
	allBodies = append(allBodies, string(scopeBody))
	if scopeResp.StatusCode != http.StatusForbidden {
		t.Fatalf("insufficient scope status=%d, want 403", scopeResp.StatusCode)
	}
	if string(scopeBody) != "forbidden: insufficient scope\n" {
		t.Fatalf("insufficient scope body=%q, want forbidden response", scopeBody)
	}
	wantScopeChallenge := `Bearer error="insufficient_scope", scope="mcp:connect"`
	if got := scopeResp.Header.Get("WWW-Authenticate"); got != wantScopeChallenge {
		t.Fatalf("insufficient scope challenge=%q, want %q", got, wantScopeChallenge)
	}
	notFoundResp := post("/nonexistent-endpoint", `{}`, validToken, "")
	notFoundBody, _ := io.ReadAll(notFoundResp.Body)
	_ = notFoundResp.Body.Close()
	allBodies = append(allBodies, string(notFoundBody))
	if notFoundResp.StatusCode != http.StatusNotFound {
		t.Fatalf("nonexistent route status=%d, want 404", notFoundResp.StatusCode)
	}

	// 7. Expire a newly-created session by advancing the injected server clock beyond its deadline.
	expiredInit := post("/mcp", `{"jsonrpc":"2.0","id":7,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"chatgpt","version":"1.0"}}}`, validToken, "")
	expiredInitBody, _ := io.ReadAll(expiredInit.Body)
	_ = expiredInit.Body.Close()
	allBodies = append(allBodies, string(expiredInitBody))
	if expiredInit.StatusCode != http.StatusOK {
		t.Fatalf("expiry-session init status=%d, want 200", expiredInit.StatusCode)
	}
	expiredSessionID := expiredInit.Header.Get("Mcp-Session-Id")
	if expiredSessionID == "" {
		t.Fatal("expiry-session init response missing Mcp-Session-Id header")
	}
	advanceClock(9 * time.Hour)
	expiredResp := post("/mcp", `{"jsonrpc":"2.0","id":8,"method":"tools/list","params":{}}`, "", expiredSessionID)
	expiredBody, _ := io.ReadAll(expiredResp.Body)
	_ = expiredResp.Body.Close()
	allBodies = append(allBodies, string(expiredBody))
	requireChallenge(t, expiredResp, "expired session")
	if string(expiredBody) != "unauthorized\n" {
		t.Fatalf("expired session body=%q, want unauthorized response", expiredBody)
	}
	if statusCounts[http.StatusForbidden] == 0 || statusCounts[http.StatusNotFound] == 0 {
		t.Fatalf("wire status counts omit required rejection: 403=%d 404=%d", statusCounts[http.StatusForbidden], statusCounts[http.StatusNotFound])
	}

	captured := h.stdout.String()
	type diagnosticRecord struct {
		Event           string `json:"event"`
		RequestID       string `json:"request_id"`
		FailureStage    string `json:"failure_stage"`
		FailureReason   string `json:"failure_reason"`
		SessionHeader   string `json:"session_header"`
		SessionState    string `json:"session_state"`
		BindingRelation string `json:"binding_relation"`
		SessionRefState string `json:"session_ref_state"`
	}
	records := make(map[string][]diagnosticRecord)
	for _, line := range strings.Split(strings.TrimSpace(captured), "\n") {
		var record diagnosticRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil || record.Event != "http_complete" {
			continue
		}
		records[record.RequestID] = append(records[record.RequestID], record)
	}
	type diagnosticExpectation struct {
		failureStage, failureReason, sessionHeader, sessionState, bindingRelation, sessionRefState string
	}
	assertDiagnostic := func(index int, name string, want diagnosticExpectation) {
		t.Helper()
		if index >= len(diagnosticIDs) || diagnosticIDs[index] == "" {
			t.Errorf("%s response omitted its diagnostic correlation ID", name)
			return
		}
		matches := records[diagnosticIDs[index]]
		if len(matches) != 1 {
			t.Errorf("%s has %d correlated completion records, want exactly one", name, len(matches))
			return
		}
		got := matches[0]
		if (want.failureStage != "" && got.FailureStage != want.failureStage) ||
			(want.failureReason != "" && got.FailureReason != want.failureReason) ||
			(want.sessionHeader != "" && got.SessionHeader != want.sessionHeader) ||
			(want.sessionState != "" && got.SessionState != want.sessionState) ||
			(want.bindingRelation != "" && got.BindingRelation != want.bindingRelation) ||
			(want.sessionRefState != "" && got.SessionRefState != want.sessionRefState) {
			t.Errorf("%s diagnostic has unexpected admission fields", name)
		}
	}
	assertDiagnostic(0, "probe", diagnosticExpectation{failureStage: "outer_oauth", failureReason: "credentials_missing", sessionHeader: "absent"})
	assertDiagnostic(1, "initialize", diagnosticExpectation{sessionState: "created", bindingRelation: "not_evaluated", sessionRefState: "available"})
	assertDiagnostic(2, "discovery", diagnosticExpectation{sessionHeader: "present", sessionState: "existing", bindingRelation: "match"})
	assertDiagnostic(3, "unknown session", diagnosticExpectation{failureStage: "outer_oauth", failureReason: "session_absent", sessionHeader: "present", sessionState: "absent"})
	assertDiagnostic(5, "unauthenticated session", diagnosticExpectation{failureStage: "outer_oauth", failureReason: "session_fingerprint_mismatch", sessionHeader: "present", sessionState: "existing"})
	assertDiagnostic(6, "insufficient scope", diagnosticExpectation{failureStage: "outer_oauth", failureReason: "scope_insufficient"})
	assertDiagnostic(9, "expired session", diagnosticExpectation{failureStage: "outer_oauth", failureReason: "jwt_expired", sessionHeader: "present", sessionState: "existing"})

	privateKeyDER := x509.MarshalPKCS1PrivateKey(h.private)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyDER})
	fixtureSecrets := []string{
		validToken,
		insufficientScopeToken,
		serveClientToken,
		sessionID,
		"unknown-session-1234",
		unauthenticatedSessionID,
		expiredSessionID,
		string(privateKeyDER),
		string(privateKeyPEM),
		"BEGIN RSA PRIVATE KEY",
	}
	secretsAndHashes := make([]string, 0, 2*len(fixtureSecrets))
	for _, secret := range fixtureSecrets {
		if secret == "" {
			continue
		}
		secretsAndHashes = append(secretsAndHashes, secret, fmt.Sprintf("%x", sha256.Sum256([]byte(secret))))
	}
	for _, body := range allBodies {
		for _, secret := range secretsAndHashes {
			if strings.Contains(body, secret) {
				t.Error("fixture secret leaked in an HTTP response body")
			}
		}
	}
	for _, headers := range rejectedHeaders {
		for _, secret := range secretsAndHashes {
			if strings.Contains(headers, secret) {
				t.Error("fixture secret leaked in rejected response headers")
			}
		}
	}
	for _, secret := range secretsAndHashes {
		if strings.Contains(captured, secret) {
			t.Error("fixture secret leaked in stdout diagnostics")
		}
	}
	testServeNow = nil
}
func TestOpenAIToolSchemaCompatibilityWire(t *testing.T) {
	const resourceURL = "https://mcp.bullprog.com"
	t.Setenv("MCP_RESOURCE_URL", resourceURL)
	t.Setenv("MCP_SESSION_ADMISSION_ENABLED", "true")
	t.Setenv("MCP_REQUIRED_SCOPE", "mcp:connect")

	servers32 := make([]string, 32)
	for index := range servers32 {
		servers32[index] = fmt.Sprintf("server-%02d", index)
	}
	servers32[0] = "alpha"

	h := startOAuthHarness(t, servers32...)
	h.awaitLive(t)
	h.awaitReady(t)

	now := time.Now().UTC()
	token := h.oauthToken(t, now, map[string]any{
		"aud":   resourceURL,
		"scope": "mcp:connect",
		"exp":   now.Add(10 * time.Minute).Unix(),
	})
	post := func(body, sessionID string) *http.Response {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, h.httpURL+"/mcp", strings.NewReader(body))
		if err != nil {
			t.Fatalf("build MCP request: %v", err)
		}
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		if sessionID != "" {
			request.Header.Set("Mcp-Session-Id", sessionID)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("POST /mcp: %v", err)
		}
		t.Cleanup(func() { _ = response.Body.Close() })
		return response
	}

	initialize := post(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"openai-schema-compatibility","version":"1"}}}`, "")
	if initialize.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d; want 200", initialize.StatusCode)
	}
	initializeReply := oauthJSONBody(t, initialize)
	if initializeReply["jsonrpc"] != "2.0" {
		t.Fatalf("initialize jsonrpc = %v; want 2.0", initializeReply["jsonrpc"])
	}
	sessionID := initialize.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("initialize response omitted Mcp-Session-Id")
	}

	discovery := post(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`, sessionID)
	if discovery.StatusCode != http.StatusOK {
		t.Fatalf("tools/list status = %d; want 200", discovery.StatusCode)
	}
	discoveryReply := oauthJSONBody(t, discovery)
	discoveryResult, ok := discoveryReply["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/list result = %T; want JSON object", discoveryReply["result"])
	}
	tools, ok := discoveryResult["tools"].([]any)
	if !ok {
		t.Fatalf("tools/list result.tools = %T; want array", discoveryResult["tools"])
	}
	inputSchemas := make(map[string]map[string]any, len(tools))
	for index, rawTool := range tools {
		tool, ok := rawTool.(map[string]any)
		if !ok {
			t.Fatalf("tools[%d] = %T; want JSON object", index, rawTool)
		}
		name, ok := tool["name"].(string)
		if !ok {
			t.Fatalf("tools[%d].name = %T; want string", index, tool["name"])
		}
		switch name {
		case "search_tools", "describe_tool", "call_tool":
			inputSchema, ok := tool["inputSchema"].(map[string]any)
			if !ok {
				t.Fatalf("%s inputSchema = %T; want JSON object", name, tool["inputSchema"])
			}
			if _, exists := inputSchema["$schema"]; exists {
				t.Errorf("%s inputSchema unexpectedly contains $schema", name)
			}
			inputSchemas[name] = inputSchema
		}
	}
	for _, name := range []string{"search_tools", "describe_tool", "call_tool"} {
		if _, ok := inputSchemas[name]; !ok {
			t.Fatalf("tools/list omitted %q", name)
		}
	}

	searchProperties, ok := inputSchemas["search_tools"]["properties"].(map[string]any)
	if !ok {
		t.Fatalf("search_tools properties = %T; want JSON object", inputSchemas["search_tools"]["properties"])
	}
	servers, ok := searchProperties["servers"].(map[string]any)
	if !ok {
		t.Fatalf("search_tools servers schema = %T; want JSON object", searchProperties["servers"])
	}
	for _, bound := range []string{"minItems", "maxItems"} {
		if _, exists := servers[bound]; exists {
			t.Errorf("search_tools servers schema unexpectedly contains %s", bound)
		}
	}

	callProperties, ok := inputSchemas["call_tool"]["properties"].(map[string]any)
	if !ok {
		t.Fatalf("call_tool properties = %T; want JSON object", inputSchemas["call_tool"]["properties"])
	}
	arguments, ok := callProperties["arguments"].(map[string]any)
	if !ok {
		t.Fatalf("call_tool arguments schema = %T; want JSON object", callProperties["arguments"])
	}
	if arguments["type"] != "object" {
		t.Errorf("call_tool arguments type = %v; want object", arguments["type"])
	}
	if additionalProperties, exists := arguments["additionalProperties"]; !exists || additionalProperties != true {
		t.Errorf("call_tool arguments additionalProperties = %v (present %t); want true", additionalProperties, exists)
	}
	if description, ok := arguments["description"].(string); !ok || strings.TrimSpace(description) == "" {
		t.Errorf("call_tool arguments description = %v; want non-empty string", arguments["description"])
	}

	callSearch := func(id int, arguments map[string]any) (map[string]any, map[string]any) {
		t.Helper()
		requestBody, err := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"method":  "tools/call",
			"params": map[string]any{
				"name":      "search_tools",
				"arguments": arguments,
			},
		})
		if err != nil {
			t.Fatalf("marshal search_tools call: %v", err)
		}
		response := post(string(requestBody), sessionID)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("tools/call status = %d; want 200", response.StatusCode)
		}
		reply := oauthJSONBody(t, response)
		result, ok := reply["result"].(map[string]any)
		if !ok {
			t.Fatalf("tools/call result = %T; want JSON object", reply["result"])
		}
		return reply, result
	}
	requireIsError := func(result map[string]any, want bool, input string) {
		t.Helper()
		if isError, present := result["isError"]; present {
			if value, ok := isError.(bool); !ok || value != want {
				t.Fatalf("tools/call %s isError = %v; want %t", input, isError, want)
			}
		} else if want {
			t.Fatalf("tools/call %s omitted isError; want true", input)
		}
	}

	emptyReply, emptyResult := callSearch(3, map[string]any{"query": "test", "servers": []string{}})
	requireIsError(emptyResult, false, "with empty servers")
	omittedReply, omittedResult := callSearch(4, map[string]any{"query": "test"})
	requireIsError(omittedResult, false, "with servers omitted")
	emptyJSON, err := json.Marshal(emptyReply["result"])
	if err != nil {
		t.Fatalf("marshal empty-servers result: %v", err)
	}
	omittedJSON, err := json.Marshal(omittedReply["result"])
	if err != nil {
		t.Fatalf("marshal omitted-servers result: %v", err)
	}
	if !bytes.Equal(emptyJSON, omittedJSON) {
		t.Errorf("search_tools result with servers:[] = %s; with servers omitted = %s; want identical results", emptyJSON, omittedJSON)
	}

	_, result32 := callSearch(5, map[string]any{"query": "test", "servers": servers32})
	requireIsError(result32, false, "with 32 servers")

	servers33 := make([]string, 33)
	for index := range servers33 {
		servers33[index] = fmt.Sprintf("server-%02d", index)
	}
	_, result33 := callSearch(6, map[string]any{"query": "test", "servers": servers33})
	requireIsError(result33, true, "with 33 servers")
	structured, ok := result33["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("33-server error structuredContent = %T; want JSON object", result33["structuredContent"])
	}
	if structured["code"] != "invalid_arguments" {
		t.Errorf("33-server error code = %v; want invalid_arguments", structured["code"])
	}
	if message, ok := structured["message"].(string); !ok || !strings.Contains(message, "servers exceeds maximum of 32 entries") {
		t.Errorf("33-server error message = %v; want it to contain the maximum-server message", structured["message"])
	}
}

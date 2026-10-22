package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
	opsURL  string
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan int
	stdout  *syncWriter
	stderr  *syncWriter
}

// startOAuthHarness generates the signing key, publishes its JWKS, writes one
// operator document and starts the bridge with the OAuth environment set.
func startOAuthHarness(t *testing.T) *oauthTestHarness {
	t.Helper()

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

	t.Setenv("POCKET_ID_JWKS_URL", jwks.URL+"/.well-known/jwks.json")
	t.Setenv("POCKET_ID_ISSUER", oauthIssuer)
	t.Setenv("MCP_RESOURCE_URL", oauthResource)
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
	writeBridgeConfig(t, configPath, httpAddress, operationsAddress, storageDirectory, embeddings.URL, []bridgeServerSpec{{id: "alpha", endpoint: upstream.URL}})

	ctx, cancel := context.WithCancel(context.Background())
	stdout, stderr := &syncWriter{}, &syncWriter{}
	done := make(chan int, 1)
	go func() { done <- runContext(ctx, []string{"--config", configPath}, stdout, stderr) }()

	harness := &oauthTestHarness{
		jwks:        jwks,
		jwksFetches: &jwksFetches,
		private:     private,
		public:  public,
		httpURL: "http://" + httpAddress,
		opsURL:  "http://" + operationsAddress,
		ctx:     ctx,
		cancel:  cancel,
		done:    done,
		stdout:  stdout,
		stderr:  stderr,
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
	requireChallenge(t, oauthPost(t, harness.httpURL, expired, `{}`), "expired token")

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

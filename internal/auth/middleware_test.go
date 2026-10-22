package auth

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// testConfig is the shared Config used by middleware tests.
var testConfig = Config{
	PocketIDIssuer:   "https://id.example.com",
	MCPResourceURL:   "https://mcp.example.com",
	MCPRequiredScope: "mcp:connect",
}

// mustSignToken signs tok with the given key using RS256 and returns the
// compact serialisation as a string.
func mustSignToken(t *testing.T, priv any, tok jwt.Token) string {
	t.Helper()
	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256, priv))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return string(signed)
}

// mustBuildToken builds a token with the given claims and returns it.
func mustBuildToken(t *testing.T, apply func(*jwt.Builder)) jwt.Token {
	t.Helper()
	b := jwt.NewBuilder()
	if apply != nil {
		apply(b)
	}
	tok, err := b.Build()
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	return tok
}

// mustPrivKeyJWK converts a raw RSA private key into a jwk.Key carrying the
// test-kid identifier and RS256 algorithm, so signed tokens automatically
// include the matching kid header.
func mustPrivKeyJWK(t *testing.T, priv *rsa.PrivateKey) jwk.Key {
	t.Helper()
	key, err := jwk.FromRaw(priv)
	if err != nil {
		t.Fatalf("convert private key: %v", err)
	}
	if err := key.Set(jwk.KeyIDKey, "test-kid"); err != nil {
		t.Fatalf("set kid: %v", err)
	}
	if err := key.Set(jwk.AlgorithmKey, jwa.RS256); err != nil {
		t.Fatalf("set alg: %v", err)
	}
	return key
}

// newAuthTest provides the middleware under test with an in-process key set.
func newAuthTest(t *testing.T, opts ...Option) (func(http.Handler) http.Handler, *rsaKeyPair) {
	t.Helper()
	priv, _, set := mustTestKey(t)
	mw := NewAuthMiddleware(testConfig, StaticKeySet{Set: set}, opts...)
	return mw, &rsaKeyPair{priv: mustPrivKeyJWK(t, priv)}
}

type rsaKeyPair struct {
	priv jwk.Key
}

// requestWithAuth issues a request through the middleware and returns the
// recorder for assertion.
func requestWithAuth(t *testing.T, mw func(http.Handler) http.Handler, next http.Handler, authHeader string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)
	return rec
}

func TestChallengeMetadataURL(t *testing.T) {
	// Arrange
	cases := []struct {
		name        string
		resourceURL string
		want        string
	}{
		{"plain url", "https://mcp.example.com", "https://mcp.example.com/.well-known/oauth-protected-resource"},
		{"trailing slash", "https://mcp.example.com/", "https://mcp.example.com/.well-known/oauth-protected-resource"},
		{"already has path", "https://mcp.example.com/api", "https://mcp.example.com/api/.well-known/oauth-protected-resource"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := challengeMetadataURL(tc.resourceURL)

			// Assert
			if got != tc.want {
				t.Errorf("challengeMetadataURL(%q) = %q, want %q", tc.resourceURL, got, tc.want)
			}
		})
	}
}

func TestIsBypassed(t *testing.T) {
	// Arrange
	bypassed := []string{
		"/.well-known/oauth-protected-resource",
		"/livez",
		"/readyz",
		"/healthz",
		"/livez/",
		"/readyz/",
	}
	notBypassed := []string{"/", "/mcp", "/api/mcp", "/healthz/extra"}

	// Act / Assert
	for _, path := range bypassed {
		if !isBypassed(path) {
			t.Errorf("isBypassed(%q) = false, want true", path)
		}
	}
	for _, path := range notBypassed {
		if isBypassed(path) {
			t.Errorf("isBypassed(%q) = true, want false", path)
		}
	}
}

func TestAudienceMatches(t *testing.T) {
	// Arrange
	target := "https://mcp.example.com"
	cases := []struct {
		name     string
		audience []string
		target   string
		want     bool
	}{
		{"exact match", []string{"https://mcp.example.com"}, target, true},
		{"trailing slash in token", []string{"https://mcp.example.com/"}, target, true},
		{"trailing slash in target", []string{"https://mcp.example.com"}, target + "/", true},
		{"multiple audiences match", []string{"other", target}, target, true},
		{"no match", []string{"https://other.example.com"}, target, false},
		{"empty audience", nil, target, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := audienceMatches(tc.audience, tc.target)

			// Assert
			if got != tc.want {
				t.Errorf("audienceMatches(%v, %q) = %v, want %v", tc.audience, tc.target, got, tc.want)
			}
		})
	}
}

func TestParseScopes(t *testing.T) {
	// Arrange
	cases := []struct {
		name  string
		claim string
		value any
		want  []string
	}{
		{"scope as space string", "scope", "read write", []string{"read", "write"}},
		{"scp as string array", "scp", []any{"read", "write"}, []string{"read", "write"}},
		{"scp as []string", "scp", []string{"read", "write"}, []string{"read", "write"}},
		{"missing both", "", nil, nil},
		{"unknown value type", "scope", 42, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			var tok jwt.Token
			if tc.claim == "" {
				tok = mustBuildToken(t, nil)
			} else {
				tok = mustBuildToken(t, func(b *jwt.Builder) {
					_ = b.Claim(tc.claim, tc.value)
				})
			}

			// Act
			got := parseScopes(tok)

			// Assert
			if tc.want == nil {
				if len(got) != 0 {
					t.Errorf("parseScopes() = %v, want empty", got)
				}
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseScopes() = %v (len %d), want %v (len %d)", got, len(got), tc.want, len(tc.want))
			}
			for i, v := range tc.want {
				if got[i] != v {
					t.Errorf("parseScopes()[%d] = %q, want %q", i, got[i], v)
				}
			}
		})
	}
}

func TestMiddlewareBypassedPaths(t *testing.T) {
	// Arrange
	mw, _ := newAuthTest(t)
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})
	handler := mw(next)

	paths := []string{
		"/.well-known/oauth-protected-resource",
		"/livez",
		"/readyz",
		"/healthz",
		"/livez/",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			// Arrange
			nextCalled = false
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			// Act
			handler.ServeHTTP(rec, req)

			// Assert
			if !nextCalled {
				t.Errorf("next not called for bypassed path %q", path)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}
		})
	}
}

func TestMiddlewareMissingAuthHeader(t *testing.T) {
	// Arrange
	mw, _ := newAuthTest(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next should not be called without auth")
	})

	// Act
	rec := requestWithAuth(t, mw, next, "")

	// Assert
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	want := fmt.Sprintf(`Bearer realm="mcp", resource_metadata="%s"`, challengeMetadataURL(testConfig.MCPResourceURL))
	if got := rec.Header().Get("WWW-Authenticate"); got != want {
		t.Errorf("WWW-Authenticate = %q, want %q", got, want)
	}
}

func TestMiddlewareMalformedHeader(t *testing.T) {
	// Arrange
	mw, _ := newAuthTest(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next should not be called for malformed header")
	})

	malformed := []string{"Basic abc", "bearer", "Token xyz", "Bearer"}

	for _, header := range malformed {
		t.Run(header, func(t *testing.T) {
			// Act
			rec := requestWithAuth(t, mw, next, header)

			// Assert
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
			if rec.Header().Get("WWW-Authenticate") == "" {
				t.Error("WWW-Authenticate header missing")
			}
		})
	}
}

func TestMiddlewareNonJWTToken(t *testing.T) {
	// Arrange
	mwNoFallback, _ := newAuthTest(t)
	mwFallback, _ := newAuthTest(t, WithStaticFallback(true))

	nextNoFallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next should not be called for non-JWT without fallback")
	})
	nextCalled := false
	nextFallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Act
	recNoFallback := requestWithAuth(t, mwNoFallback, nextNoFallback, "Bearer opaque-token")
	recFallback := requestWithAuth(t, mwFallback, nextFallback, "Bearer opaque-token")

	// Assert
	if recNoFallback.Code != http.StatusUnauthorized {
		t.Errorf("status without fallback = %d, want %d", recNoFallback.Code, http.StatusUnauthorized)
	}
	if !nextCalled {
		t.Error("next not called for non-JWT with static fallback")
	}
	if recFallback.Code != http.StatusOK {
		t.Errorf("status with fallback = %d, want %d", recFallback.Code, http.StatusOK)
	}
}

func TestMiddlewareInvalidSignature(t *testing.T) {
	// Arrange
	mw, _ := newAuthTest(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next should not be called with invalid signature")
	})

	// Arrange: token signed by a different key than the keyset contains.
	tok := mustBuildToken(t, func(b *jwt.Builder) {
		_ = b.Issuer(testConfig.PocketIDIssuer)
		_ = b.Audience([]string{testConfig.MCPResourceURL})
		_ = b.IssuedAt(time.Now())
		_ = b.Expiration(time.Now().Add(time.Hour))
	})
	// Sign with a fresh key so the signature cannot verify against our keyset.
	freshPriv, _, _ := mustTestKey(t)
	token := mustSignToken(t, mustPrivKeyJWK(t, freshPriv), tok)

	// Act
	rec := requestWithAuth(t, mw, next, "Bearer "+token)

	// Assert
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("WWW-Authenticate header missing")
	}
}

func TestMiddlewareWrongIssuer(t *testing.T) {
	// Arrange
	mw, keys := newAuthTest(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next should not be called with wrong issuer")
	})

	tok := mustBuildToken(t, func(b *jwt.Builder) {
		_ = b.Issuer("https://evil.example.com")
		_ = b.Audience([]string{testConfig.MCPResourceURL})
		_ = b.IssuedAt(time.Now())
		_ = b.Expiration(time.Now().Add(time.Hour))
	})
	token := mustSignToken(t, keys.priv, tok)

	// Act
	rec := requestWithAuth(t, mw, next, "Bearer "+token)

	// Assert
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareWrongAudience(t *testing.T) {
	// Arrange
	mw, keys := newAuthTest(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next should not be called with wrong audience")
	})

	tok := mustBuildToken(t, func(b *jwt.Builder) {
		_ = b.Issuer(testConfig.PocketIDIssuer)
		_ = b.Audience([]string{"https://other.example.com"})
		_ = b.IssuedAt(time.Now())
		_ = b.Expiration(time.Now().Add(time.Hour))
	})
	token := mustSignToken(t, keys.priv, tok)

	// Act
	rec := requestWithAuth(t, mw, next, "Bearer "+token)

	// Assert
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareExpiredToken(t *testing.T) {
	// Arrange
	mw, keys := newAuthTest(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next should not be called with expired token")
	})

	tok := mustBuildToken(t, func(b *jwt.Builder) {
		_ = b.Issuer(testConfig.PocketIDIssuer)
		_ = b.Audience([]string{testConfig.MCPResourceURL})
		_ = b.IssuedAt(time.Now().Add(-2 * time.Hour))
		_ = b.Expiration(time.Now().Add(-time.Hour))
	})
	token := mustSignToken(t, keys.priv, tok)

	// Act
	rec := requestWithAuth(t, mw, next, "Bearer "+token)

	// Assert
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareClockSkewTolerance(t *testing.T) {
	// Arrange: token expired 30s ago passes under the 60s skew tolerance.
	mw, keys := newAuthTest(t)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	tok := mustBuildToken(t, func(b *jwt.Builder) {
		_ = b.Issuer(testConfig.PocketIDIssuer)
		_ = b.Audience([]string{testConfig.MCPResourceURL})
		_ = b.IssuedAt(time.Now().Add(-2 * time.Minute))
		_ = b.Expiration(time.Now().Add(-30 * time.Second))
		_ = b.Claim("scope", fmt.Sprintf("openid %s", testConfig.MCPRequiredScope))
	})
	token := mustSignToken(t, keys.priv, tok)

	// Act
	rec := requestWithAuth(t, mw, next, "Bearer "+token)

	// Assert
	if !called {
		t.Error("next not called for token within clock skew tolerance")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMiddlewareMissingRequiredScope(t *testing.T) {
	// Arrange
	mw, keys := newAuthTest(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next should not be called without required scope")
	})

	tok := mustBuildToken(t, func(b *jwt.Builder) {
		_ = b.Issuer(testConfig.PocketIDIssuer)
		_ = b.Audience([]string{testConfig.MCPResourceURL})
		_ = b.IssuedAt(time.Now())
		_ = b.Expiration(time.Now().Add(time.Hour))
		_ = b.Claim("scope", "read write")
	})
	token := mustSignToken(t, keys.priv, tok)

	// Act
	rec := requestWithAuth(t, mw, next, "Bearer "+token)

	// Assert
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	want := fmt.Sprintf(`Bearer error="insufficient_scope", scope="%s"`, testConfig.MCPRequiredScope)
	if got := rec.Header().Get("WWW-Authenticate"); got != want {
		t.Errorf("WWW-Authenticate = %q, want %q", got, want)
	}
}

func TestMiddlewareValidTokenInjectsClaims(t *testing.T) {
	// Arrange
	mw, keys := newAuthTest(t)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		claims := ClaimsFromContext(r.Context())
		if claims == nil {
			t.Error("expected claims in context, got nil")
			return
		}
		if claims.Subject != "user-123" {
			t.Errorf("subject = %q, want %q", claims.Subject, "user-123")
		}
		if claims.Email != "user@example.com" {
			t.Errorf("email = %q, want %q", claims.Email, "user@example.com")
		}
		if claims.Issuer != testConfig.PocketIDIssuer {
			t.Errorf("issuer = %q, want %q", claims.Issuer, testConfig.PocketIDIssuer)
		}
		if len(claims.Audience) != 1 || claims.Audience[0] != testConfig.MCPResourceURL {
			t.Errorf("audience = %v, want [%q]", claims.Audience, testConfig.MCPResourceURL)
		}
		if len(claims.Scopes) != 2 {
			t.Errorf("scopes = %v, want 2 items", claims.Scopes)
		}
		w.WriteHeader(http.StatusOK)
	})

	tok := mustBuildToken(t, func(b *jwt.Builder) {
		_ = b.Subject("user-123")
		_ = b.Issuer(testConfig.PocketIDIssuer)
		_ = b.Audience([]string{testConfig.MCPResourceURL})
		_ = b.IssuedAt(time.Now())
		_ = b.Expiration(time.Now().Add(time.Hour))
		_ = b.Claim("email", "user@example.com")
		_ = b.Claim("scope", fmt.Sprintf("openid %s", testConfig.MCPRequiredScope))
	})
	token := mustSignToken(t, keys.priv, tok)

	// Act
	rec := requestWithAuth(t, mw, next, "Bearer "+token)

	// Assert
	if !called {
		t.Error("next not called for valid token")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if strings.Contains(rec.Body.String(), "unauthorized") {
		t.Error("unexpected unauthorized body for valid token")
	}
}

func TestMiddlewareKeySetProviderError(t *testing.T) {
	// Arrange
	mw := NewAuthMiddleware(testConfig, StaticKeySet{})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next should not be called when keyset provider fails")
	})

	// Act
	rec := requestWithAuth(t, mw, next, "Bearer a.b.c")

	// Assert
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("WWW-Authenticate header missing")
	}
}

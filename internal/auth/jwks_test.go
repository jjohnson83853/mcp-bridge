package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

// mustTestKey generates an RSA keypair and its JWKS public representation,
// failing the test when either step errors.
func mustTestKey(t *testing.T) (*rsa.PrivateKey, jwk.Key, jwk.Set) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	key, err := jwk.FromRaw(&priv.PublicKey)
	if err != nil {
		t.Fatalf("convert public key: %v", err)
	}
	if err := key.Set(jwk.KeyIDKey, "test-kid"); err != nil {
		t.Fatalf("set kid: %v", err)
	}
	if err := key.Set(jwk.AlgorithmKey, jwa.RS256); err != nil {
		t.Fatalf("set alg: %v", err)
	}

	set := jwk.NewSet()
	if err := set.AddKey(key); err != nil {
		t.Fatalf("add key to set: %v", err)
	}
	return priv, key, set
}

// jwksServer serves the given set as an application/json JWKS document.
func jwksServer(t *testing.T, set jwk.Set) *httptest.Server {
	t.Helper()
	payload, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("marshal jwks: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestStaticKeySetReturnsWrappedSet(t *testing.T) {
	// Arrange
	_, _, set := mustTestKey(t)
	provider := StaticKeySet{Set: set}

	// Act
	got, err := provider.KeySet(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("KeySet() error = %v", err)
	}
	if got != set {
		t.Fatal("KeySet() returned a different set")
	}
}

func TestStaticKeySetNilReturnsError(t *testing.T) {
	// Arrange
	provider := StaticKeySet{}

	// Act
	got, err := provider.KeySet(context.Background())

	// Assert
	if err == nil {
		t.Fatal("KeySet() error = nil, want error")
	}
	if got != nil {
		t.Fatal("KeySet() returned a non-nil set alongside the error")
	}
}

func TestNewJWKSCacheFetchesRemoteKeySet(t *testing.T) {
	// Arrange
	_, _, set := mustTestKey(t)
	srv := jwksServer(t, set)
	ctx := context.Background()

	cache, err := NewJWKSCache(ctx, srv.URL)
	if err != nil {
		t.Fatalf("NewJWKSCache() error = %v", err)
	}

	// Act
	got, err := cache.KeySet(ctx)

	// Assert
	if err != nil {
		t.Fatalf("KeySet() error = %v", err)
	}
	fetched, ok := got.LookupKeyID("test-kid")
	if !ok {
		t.Fatal("KeySet() did not include test-kid")
	}
	var pub rsa.PublicKey
	if err := fetched.Raw(&pub); err != nil {
		t.Fatalf("extract public key: %v", err)
	}
	if pub.N == nil {
		t.Fatal("fetched key is not an RSA public key")
	}
}



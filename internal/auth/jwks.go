package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
)

// KeySetProvider supplies the JSON Web Key Set used to verify tokens.
type KeySetProvider interface {
	KeySet(ctx context.Context) (jwk.Set, error)
}

// StaticKeySet serves a fixed key set, typically built in-process for tests.
type StaticKeySet struct {
	Set jwk.Set
}

// KeySet returns the wrapped set, erroring when it was never provided.
func (s StaticKeySet) KeySet(_ context.Context) (jwk.Set, error) {
	if s.Set == nil {
		return nil, fmt.Errorf("keyset is nil")
	}
	return s.Set, nil
}

// JWKSCache fetches and refreshes a remote JWKS document, reusing cached
// keys between refresh intervals.
type JWKSCache struct {
	cache *jwk.Cache
	url   string
}

// NewJWKSCache registers jwksURL with a background cache that refreshes at
// most once every 15 minutes.
func NewJWKSCache(ctx context.Context, jwksURL string) (*JWKSCache, error) {
	c := jwk.NewCache(ctx)
	if err := c.Register(jwksURL, jwk.WithMinRefreshInterval(15*time.Minute)); err != nil {
		return nil, fmt.Errorf("register jwks url: %w", err)
	}
	return &JWKSCache{cache: c, url: jwksURL}, nil
}

// KeySet returns the cached key set for the registered URL, fetching on
// first use and after the refresh interval elapses.
func (c *JWKSCache) KeySet(ctx context.Context) (jwk.Set, error) {
	return c.cache.Get(ctx, c.url)
}

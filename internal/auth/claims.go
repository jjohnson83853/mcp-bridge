package auth

import (
	"context"

	"github.com/lestrrat-go/jwx/v2/jwt"
)

// Claims carries the authenticated principal's identity and granted scopes.
type Claims struct {
	Subject  string
	Email    string
	Issuer   string
	Audience []string
	Scopes   []string
	RawToken jwt.Token
}

// contextKey is an unexported type so only this package can set or read
// claims in a context.
type contextKey struct{}

// ContextWithClaims returns a new context carrying the given claims.
func ContextWithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, contextKey{}, claims)
}

// ClaimsFromContext extracts the claims stored in ctx, or nil when absent.
func ClaimsFromContext(ctx context.Context) *Claims {
	if ctx == nil {
		return nil
	}
	claims, _ := ctx.Value(contextKey{}).(*Claims)
	return claims
}

// SubjectFromContext returns the subject of the claims in ctx, or an empty
// string when no claims are present.
func SubjectFromContext(ctx context.Context) string {
	if claims := ClaimsFromContext(ctx); claims != nil {
		return claims.Subject
	}
	return ""
}

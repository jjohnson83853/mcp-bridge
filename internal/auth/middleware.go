package auth

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwt"
)

// Option customises the auth middleware at construction time.
type Option func(*middlewareOptions)

// middlewareOptions holds the runtime configuration for the auth middleware.
type middlewareOptions struct {
	staticFallback bool
}

// WithStaticFallback lets opaque (non-JWT) bearer tokens through untouched,
// for deployments that also accept tokens verified elsewhere.
func WithStaticFallback(enabled bool) Option {
	return func(o *middlewareOptions) {
		o.staticFallback = enabled
	}
}

// challengeMetadataURL returns the OAuth protected-resource metadata URL for
// the given resource URL.
func challengeMetadataURL(resourceURL string) string {
	return strings.TrimRight(resourceURL, "/") + "/.well-known/oauth-protected-resource"
}

// isBypassed reports whether path skips authentication entirely.
func isBypassed(path string) bool {
	switch strings.TrimSuffix(path, "/") {
	case "/.well-known/oauth-protected-resource", "/livez", "/readyz", "/healthz":
		return true
	default:
		return false
	}
}

// audienceMatches reports whether the token's audience includes the resource
// URL, ignoring trailing slashes on either side.
func audienceMatches(tokenAudience []string, targetURL string) bool {
	target := strings.TrimRight(targetURL, "/")
	for _, item := range tokenAudience {
		if strings.TrimRight(item, "/") == target {
			return true
		}
	}
	return false
}

// parseScopes extracts the scope list from the token's "scope" (space
// separated string) or "scp" (string array) claim.
func parseScopes(tok jwt.Token) []string {
	val, ok := tok.Get("scope")
	if !ok {
		val, ok = tok.Get("scp")
	}
	if !ok {
		return nil
	}
	switch scopes := val.(type) {
	case string:
		return strings.Fields(scopes)
	case []any:
		parsed := make([]string, 0, len(scopes))
		for _, item := range scopes {
			parsed = append(parsed, fmt.Sprint(item))
		}
		return parsed
	case []string:
		return scopes
	default:
		return nil
	}
}

// NewAuthMiddleware returns middleware that verifies bearer JWTs against the
// supplied KeySetProvider, validates issuer, audience and scope, and injects
// the resulting Claims into the request context.
func NewAuthMiddleware(cfg Config, keys KeySetProvider, opts ...Option) func(http.Handler) http.Handler {
	options := &middlewareOptions{}
	for _, opt := range opts {
		opt(options)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isBypassed(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			emit401 := func() {
				metaURL := challengeMetadataURL(cfg.MCPResourceURL)
				w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="mcp", resource_metadata="%s"`, metaURL))
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				emit401()
				return
			}
			token := strings.TrimSpace(authHeader[7:])

			if strings.Count(token, ".") != 2 {
				if options.staticFallback {
					next.ServeHTTP(w, r)
					return
				}
				emit401()
				return
			}

			set, err := keys.KeySet(r.Context())
			if err != nil {
				emit401()
				return
			}

			tok, err := jwt.Parse(
				[]byte(token),
				jwt.WithKeySet(set),
				jwt.WithValidate(true),
				jwt.WithIssuer(cfg.PocketIDIssuer),
				jwt.WithAcceptableSkew(time.Minute),
			)
			if err != nil {
				emit401()
				return
			}

			if !audienceMatches(tok.Audience(), cfg.MCPResourceURL) {
				emit401()
				return
			}

			scopes := parseScopes(tok)
			if cfg.MCPRequiredScope != "" && !slices.Contains(scopes, cfg.MCPRequiredScope) {
				w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer error="insufficient_scope", scope="%s"`, cfg.MCPRequiredScope))
				http.Error(w, "forbidden: insufficient scope", http.StatusForbidden)
				return
			}

			claims := &Claims{
				Subject:  tok.Subject(),
				Issuer:   tok.Issuer(),
				Audience: tok.Audience(),
				Scopes:   scopes,
				RawToken: tok,
			}
			if emailVal, ok := tok.Get("email"); ok {
				claims.Email = fmt.Sprint(emailVal)
			}

			r = r.WithContext(ContextWithClaims(r.Context(), claims))
			next.ServeHTTP(w, r)
		})
	}
}

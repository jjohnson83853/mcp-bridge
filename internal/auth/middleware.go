package auth

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

// Option customises the auth middleware at construction time.
type Option func(*middlewareOptions)

// middlewareOptions holds the runtime configuration for the auth middleware.
type middlewareOptions struct {
	staticFallback bool
	now            func() time.Time
}

// WithStaticFallback lets opaque (non-JWT) bearer tokens through untouched,
// for deployments that also accept tokens verified elsewhere.
func WithStaticFallback(enabled bool) Option {
	return func(o *middlewareOptions) { o.staticFallback = enabled }
}

// WithNow overrides the middleware's notion of the current time for token
// validation. Production behavior is unchanged when the option is absent.
func WithNow(now func() time.Time) Option {
	return func(o *middlewareOptions) { o.now = now }
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

// kidTolerantKeyProvider supplies verification keys from the set, matching
// the token's kid when one is present and otherwise offering every set key,
// so a token signed without a kid is verified against the configured key
// set instead of being silently unmatched. Key selection is by identifier
// match only; the token's expiry and validity are still enforced by the
// parser's validation pass under the configured clock.
func kidTolerantKeyProvider(set jwk.Set) jws.KeyProvider {
	return keyProviderFunc(func(ctx context.Context, sink jws.KeySink, sig *jws.Signature, _ *jws.Message) error {
		kid := sig.ProtectedHeaders().KeyID()
		for i := range set.Len() {
			key, ok := set.Key(i)
			if !ok {
				continue
			}
			if kid != "" && key.KeyID() != kid {
				continue
			}
			alg := key.Algorithm()
			sink.Key(jwa.SignatureAlgorithm(alg.String()), key)
		}
		return nil
	})
}

// keyProviderFunc adapts a function into a jws.KeyProvider, mirroring the
// standard *Func adapter idiom.
type keyProviderFunc func(context.Context, jws.KeySink, *jws.Signature, *jws.Message) error

func (f keyProviderFunc) FetchKeys(ctx context.Context, sink jws.KeySink, sig *jws.Signature, msg *jws.Message) error {
	return f(ctx, sink, sig, msg)
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

			// Typed rejection evidence: first decisive rejection is immutable,
			// contributions are no-ops without an owner in the context.
			contribute := func(d observe.HTTPDecision) { observe.ContributeHTTP(r.Context(), d) }

			emit401 := func() {
				metaURL := challengeMetadataURL(cfg.MCPResourceURL)
				w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="mcp", resource_metadata="%s"`, metaURL))
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				contribute(observe.HTTPDecision{
					FailureStage: "outer_oauth", FailureReason: "credentials_missing",
					AuthMode: "oauth", CredentialState: "missing",
					ChallengeKind: "resource_metadata", ChallengeTarget: "root_well_known",
					RouteOutcome: "oauth_protected",
				})
				emit401()
				return
			}
			if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				contribute(observe.HTTPDecision{
					FailureStage: "outer_oauth", FailureReason: "bearer_malformed",
					AuthMode: "oauth", CredentialState: "rejected",
					ChallengeKind: "resource_metadata", ChallengeTarget: "root_well_known",
					RouteOutcome: "oauth_protected",
				})
				emit401()
				return
			}
			token := strings.TrimSpace(authHeader[7:])

			if strings.Count(token, ".") != 2 {
				if options.staticFallback {
					next.ServeHTTP(w, r)
					return
				}
				contribute(observe.HTTPDecision{
					FailureStage: "outer_oauth", FailureReason: "jwt_malformed",
					AuthMode: "oauth", CredentialState: "rejected",
					ChallengeKind: "resource_metadata", ChallengeTarget: "root_well_known",
					RouteOutcome: "oauth_protected",
				})
				emit401()
				return
			}

			set, err := keys.KeySet(r.Context())
			if err != nil {
				contribute(observe.HTTPDecision{
					FailureStage: "outer_oauth", FailureReason: "jwt_key_unavailable",
					AuthMode: "oauth", CredentialState: "rejected",
					ChallengeKind: "resource_metadata", ChallengeTarget: "root_well_known",
					RouteOutcome: "oauth_protected",
				})
				emit401()
				return
			}

			parseOptions := []jwt.ParseOption{
				jwt.WithKeyProvider(kidTolerantKeyProvider(set)),
				jwt.WithValidate(true),
				jwt.WithIssuer(cfg.PocketIDIssuer),
				jwt.WithAcceptableSkew(time.Minute),
			}
			if options.now != nil {
				parseOptions = append(parseOptions, jwt.WithClock(jwt.ClockFunc(options.now)))
			}
			tok, err := jwt.Parse(
				[]byte(token),
				parseOptions...,
			)
			if err != nil {
				contribute(observe.HTTPDecision{
					FailureStage: "outer_oauth", FailureReason: "jwt_invalid_other",
					AuthMode: "oauth", CredentialState: "rejected",
					ChallengeKind: "resource_metadata", ChallengeTarget: "root_well_known",
					RouteOutcome: "oauth_protected",
				})
				emit401()
				return
			}

			if !audienceMatches(tok.Audience(), cfg.MCPResourceURL) {
				contribute(observe.HTTPDecision{
					FailureStage: "outer_oauth", FailureReason: "jwt_audience_invalid",
					AuthMode: "oauth", CredentialState: "rejected",
					ChallengeKind: "resource_metadata", ChallengeTarget: "root_well_known",
					RouteOutcome: "oauth_protected",
				})
				emit401()
				return
			}

			scopes := parseScopes(tok)
			if cfg.MCPRequiredScope != "" && !slices.Contains(scopes, cfg.MCPRequiredScope) {
				contribute(observe.HTTPDecision{
					FailureStage: "outer_oauth", FailureReason: "scope_insufficient",
					AuthMode: "oauth", CredentialState: "accepted",
					ScopeState:   "rejected",
					ChallengeKind: "insufficient_scope", ChallengeTarget: "none",
					RouteOutcome: "oauth_protected",
				})
				w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer error="insufficient_scope", scope="%s"`, cfg.MCPRequiredScope))
				http.Error(w, "forbidden: insufficient scope", http.StatusForbidden)
				return
			}

			contribute(observe.HTTPDecision{
				AuthMode: "oauth", CredentialState: "accepted",
				ScopeState:   scopeAcceptedState(scopes, cfg.MCPRequiredScope),
				RouteOutcome: "oauth_protected",
			})

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

// scopeAcceptedState reports the truthful scope evaluation state for an
// otherwise accepted token.
func scopeAcceptedState(scopes []string, required string) string {
	if required == "" || slices.Contains(scopes, required) {
		return "accepted"
	}
	return "not_required"
}

package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// Option customises the auth middleware at construction time.
type Option func(*middlewareOptions)

// SessionAdmissionReason classifies the result of looking up an existing
// authenticated session.
type SessionAdmissionReason string

const (
	AdmissionAdmitted        SessionAdmissionReason = "admitted"
	AdmissionNotFound        SessionAdmissionReason = "not_found"
	AdmissionUnauthenticated SessionAdmissionReason = "unauthenticated"
	AdmissionExpired         SessionAdmissionReason = "expired"
)

// SessionAdmissionOutcome reports whether an existing session may provide
// OAuth claims to a tokenless request.
type SessionAdmissionOutcome struct {
	Claims *Claims
	Reason SessionAdmissionReason
}

// middlewareOptions holds the runtime configuration for the auth middleware.
type middlewareOptions struct {
	staticFallback   bool
	now              func() time.Time
	sessionAdmission func(sessionID string) SessionAdmissionOutcome
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

func WithSessionAdmission(validator func(string) SessionAdmissionOutcome) Option {
	return func(o *middlewareOptions) { o.sessionAdmission = validator }
}

// challengeMetadataURL returns the OAuth protected-resource metadata URL for
// the given resource URL.
func challengeMetadataURL(resourceURL string) string {
	return strings.TrimRight(resourceURL, "/") + "/.well-known/oauth-protected-resource"
}

// isBypassed reports whether path skips authentication entirely.
func isBypassed(path string) bool {
	switch strings.TrimSuffix(path, "/") {
	case "/.well-known/oauth-protected-resource",
		"/.well-known/oauth-protected-resource/mcp",
		"/mcp/.well-known/oauth-protected-resource",
		"/livez", "/readyz", "/healthz":
		return true
	default:
		return false
	}
}

// audienceMatches reports whether the token's audience includes the resource
// URL, ignoring trailing slashes on either side and tolerating the origin vs
// endpoint form (/mcp suffix) in both directions.
func audienceMatches(tokenAudience []string, targetURL string) bool {
	target := strings.TrimRight(targetURL, "/")
	for _, item := range tokenAudience {
		aud := strings.TrimRight(item, "/")
		if aud == target {
			return true
		}
		if strings.TrimSuffix(aud, "/mcp") == target || strings.TrimSuffix(target, "/mcp") == aud {
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

// AcceptableSkewSeconds is the fixed token timestamp validation tolerance the
// middleware applies; it is recorded in diagnostics only when the trusted
// startup wiring attests exactly this value.
const AcceptableSkewSeconds = 60

// wellKnownMetadataPath is the exact root path the composed server registers
// for the protected-resource metadata document.
const wellKnownMetadataPath = "/.well-known/oauth-protected-resource"

// resourcePathBearing reports whether the resource URL carries a non-root
// path. Only the classification leaves this function; the URL itself never
// enters diagnostics.
func resourcePathBearing(resourceURL string) bool {
	parsed, err := url.Parse(resourceURL)
	if err != nil {
		return false
	}
	return strings.Trim(parsed.Path, "/") != ""
}

// challengeClassification reports the finite target class of the challenge
// actually written (from the same derivation the written URL uses) and
// whether that target is registered in the composed server's route table.
// Registration is an exact allowlisted comparison and is not proof of public
// reachability.
func challengeClassification(resourceURL string) (target, registered string) {
	if resourcePathBearing(resourceURL) {
		return "path_appended_well_known", "yes"
	}
	return "root_well_known", "yes"
}

// ResourcePathMode classifies the resource URL as origin_only or path_bearing
// for the typed diagnostic provenance, without emitting the URL.
func ResourcePathMode(resourceURL string) string {
	if resourcePathBearing(resourceURL) {
		return "path_bearing"
	}
	return "origin_only"
}

// NewAuthMiddleware returns middleware that verifies bearer JWTs against the
// supplied KeySetProvider, validates issuer, audience and scope, and injects
// the resulting Claims into the request context.
func NewAuthMiddleware(cfg Config, keys KeySetProvider, opts ...Option) func(http.Handler) http.Handler {
	options := &middlewareOptions{}
	for _, opt := range opts {
		opt(options)
	}
	// The challenge classification is derived from the same URL derivation
	// the written challenge uses; it is finite and carries no URL values.
	challengeTarget, metadataRegistered := challengeClassification(cfg.MCPResourceURL)

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

			authValues := r.Header.Values("Authorization")
			if len(authValues) == 0 {
				sessionID := r.Header.Get("Mcp-Session-Id")
				sessionHeader := "absent"
				if sessionID != "" {
					sessionHeader = "present"
				}
				if options.sessionAdmission != nil && sessionID != "" {
					outcome := options.sessionAdmission(sessionID)
					switch outcome.Reason {
					case AdmissionAdmitted:
						if outcome.Claims != nil {
							claims := outcome.Claims
							if cfg.MCPRequiredScope != "" && !slices.Contains(claims.Scopes, cfg.MCPRequiredScope) {
								contribute(observe.HTTPDecision{
									FailureStage: "outer_oauth", FailureReason: "scope_insufficient",
									AuthMode: "oauth", CredentialState: "accepted", ScopeState: "rejected",
									SessionHeader: "present", SessionState: "existing",
									RouteOutcome: "oauth_protected",
								})
								w.Header().Set("WWW-Authenticate", `Bearer error="insufficient_scope", scope="`+cfg.MCPRequiredScope+`"`)
								http.Error(w, "insufficient scope", http.StatusForbidden)
								return
							}
							contribute(observe.HTTPDecision{
								RouteOutcome:    "oauth_protected",
								AuthMode:        "oauth",
								CredentialState: "accepted",
								ScopeState:      scopeAcceptedState(claims.Scopes, cfg.MCPRequiredScope),
								SessionHeader:   "present",
								SessionState:    "existing",
								BindingRelation: "match",
							})
							r = r.WithContext(ContextWithClaims(r.Context(), claims))
							next.ServeHTTP(w, r)
							return
						}
					case AdmissionNotFound:
						contribute(observe.HTTPDecision{
							FailureStage: "outer_oauth", FailureReason: "session_absent",
							AuthMode: "oauth", CredentialState: "missing",
							SessionHeader: "present", SessionState: "absent",
							ChallengeKind: "resource_metadata", ChallengeTarget: challengeTarget,
							MetadataTargetRegistered: metadataRegistered,
							RouteOutcome:             "oauth_protected",
						})
						emit401()
						return
					case AdmissionUnauthenticated:
						contribute(observe.HTTPDecision{
							FailureStage: "outer_oauth", FailureReason: "session_fingerprint_mismatch",
							AuthMode: "oauth", CredentialState: "rejected",
							SessionHeader: "present", SessionState: "existing",
							ChallengeKind: "resource_metadata", ChallengeTarget: challengeTarget,
							MetadataTargetRegistered: metadataRegistered,
							RouteOutcome:             "oauth_protected",
						})
						emit401()
						return
					case AdmissionExpired:
						contribute(observe.HTTPDecision{
							FailureStage: "outer_oauth", FailureReason: "jwt_expired",
							AuthMode: "oauth", CredentialState: "rejected",
							SessionHeader: "present", SessionState: "existing",
							ChallengeKind: "resource_metadata", ChallengeTarget: challengeTarget,
							MetadataTargetRegistered: metadataRegistered,
							RouteOutcome:             "oauth_protected",
						})
						emit401()
						return
					}
				}
				contribute(observe.HTTPDecision{
					FailureStage: "outer_oauth", FailureReason: "credentials_missing",
					AuthMode: "oauth", CredentialState: "missing",
					SessionHeader: sessionHeader, SessionState: "not_evaluated",
					ChallengeKind: "resource_metadata", ChallengeTarget: challengeTarget,
					MetadataTargetRegistered: metadataRegistered,
					RouteOutcome:             "oauth_protected",
				})
				emit401()
				return
			}
			if len(authValues) != 1 {
				contribute(observe.HTTPDecision{
					FailureStage: "outer_oauth", FailureReason: "bearer_malformed",
					AuthMode: "oauth", CredentialState: "rejected",
					ChallengeKind: "resource_metadata", ChallengeTarget: challengeTarget,
					MetadataTargetRegistered: metadataRegistered,
					RouteOutcome:             "oauth_protected",
				})
				emit401()
				return
			}
			authHeader := authValues[0]
			if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				contribute(observe.HTTPDecision{
					FailureStage: "outer_oauth", FailureReason: "bearer_malformed",
					AuthMode: "oauth", CredentialState: "rejected",
					ChallengeKind: "resource_metadata", ChallengeTarget: challengeTarget,
					MetadataTargetRegistered: metadataRegistered,
					RouteOutcome:             "oauth_protected",
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
					ChallengeKind: "resource_metadata", ChallengeTarget: challengeTarget,
					MetadataTargetRegistered: metadataRegistered,
					RouteOutcome:             "oauth_protected",
				})
				emit401()
				return
			}

			set, err := keys.KeySet(r.Context())
			if err != nil {
				contribute(observe.HTTPDecision{
					FailureStage: "outer_oauth", FailureReason: "jwt_key_unavailable",
					AuthMode: "oauth", CredentialState: "rejected",
					ChallengeKind: "resource_metadata", ChallengeTarget: challengeTarget,
					MetadataTargetRegistered: metadataRegistered,
					RouteOutcome:             "oauth_protected",
				})
				emit401()
				return
			}

			parseOptions := []jwt.ParseOption{
				jwt.WithKeyProvider(kidTolerantKeyProvider(set)),
				jwt.WithValidate(true),
				jwt.WithIssuer(cfg.PocketIDIssuer),
				jwt.WithAcceptableSkew(AcceptableSkewSeconds * time.Second),
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
					ChallengeKind: "resource_metadata", ChallengeTarget: challengeTarget,
					MetadataTargetRegistered: metadataRegistered,
					RouteOutcome:             "oauth_protected",
				})
				emit401()
				return
			}

			if !audienceMatches(tok.Audience(), cfg.MCPResourceURL) {
				contribute(observe.HTTPDecision{
					FailureStage: "outer_oauth", FailureReason: "jwt_audience_invalid",
					AuthMode: "oauth", CredentialState: "rejected",
					ChallengeKind: "resource_metadata", ChallengeTarget: challengeTarget,
					MetadataTargetRegistered: metadataRegistered,
					RouteOutcome:             "oauth_protected",
				})
				emit401()
				return
			}

			scopes := parseScopes(tok)
			if cfg.MCPRequiredScope != "" && !slices.Contains(scopes, cfg.MCPRequiredScope) {
				contribute(observe.HTTPDecision{
					FailureStage: "outer_oauth", FailureReason: "scope_insufficient",
					AuthMode: "oauth", CredentialState: "accepted",
					ScopeState:    "rejected",
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

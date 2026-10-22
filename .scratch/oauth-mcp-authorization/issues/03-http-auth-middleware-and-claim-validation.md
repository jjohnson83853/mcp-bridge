# 03: HTTP authentication middleware and claim validation

**What to build:** In `internal/auth/jwks.go` and `internal/auth/middleware.go`, implement JWKS background caching/refresh and `AuthMiddleware(next http.Handler) http.Handler`:
- Check for bypassed endpoints (`/.well-known/oauth-protected-resource`, `/livez`, `/readyz`, `/healthz`).
- Extract `Authorization: Bearer <token>`.
- Missing or malformed: return `HTTP 401 Unauthorized` with `WWW-Authenticate: Bearer realm="mcp", resource_metadata="<MCP_RESOURCE_URL>/.well-known/oauth-protected-resource"`.
- If token is an opaque string and static fallback is enabled, pass request to `next` for static token checking.
- If token is a JWT, validate signature via remote JWKS cache, `iss == POCKET_ID_ISSUER`, `aud` contains/matches `MCP_RESOURCE_URL` (normalizing trailing slashes), and `exp` with 60-second clock skew grace period.
- If `MCP_REQUIRED_SCOPE != ""`, verify token's `scope` claim contains the required scope; return `HTTP 403 Forbidden` with `WWW-Authenticate: Bearer error="insufficient_scope", scope="<MCP_REQUIRED_SCOPE>"` if missing.
- Inject validated `Claims` into `r.Context()`.

**Blocked by:** 01.

Status: resolved
Source: OAuth 2.1 MCP Authorization Specification (`.scratch/oauth-mcp-authorization/spec.md`).

- [x] In `internal/auth/jwks.go`, initialize `jwk.NewCache(ctx)` registering `POCKET_ID_JWKS_URL` with background auto-refresh and support for injecting a test key set.
- [x] In `internal/auth/middleware.go`, implement `NewAuthMiddleware(cfg Config, keySet jwk.Set, options ...Option) func(http.Handler) http.Handler`.
- [x] Implement bypass logic for `/.well-known/oauth-protected-resource`, `/livez`, `/readyz`, `/healthz`.
- [x] Implement 401 challenge response with exact header `WWW-Authenticate: Bearer realm="mcp", resource_metadata="<MCP_RESOURCE_URL>/.well-known/oauth-protected-resource"`.
- [x] Implement claim validation: `iss`, `aud` (string or array, trailing slash normalized), `exp` with 60s skew tolerance.
- [x] Implement scope verification for `MCP_REQUIRED_SCOPE` (supporting space-delimited string and array of strings) returning 403 with `WWW-Authenticate: Bearer error="insufficient_scope", scope="<MCP_REQUIRED_SCOPE>"`.
- [x] In `internal/auth/middleware_test.go`, add comprehensive unit tests covering missing tokens, expired tokens, signature mismatches, audience mismatches, clock skew tolerance, scope validation, and context claim injection using ephemeral test RSA keys.

## Answer

Implemented in `internal/auth/jwks.go` and `internal/auth/middleware.go`. Verified in `internal/auth/jwks_test.go` and `internal/auth/middleware_test.go` with 99.0% statement coverage across `internal/auth`.

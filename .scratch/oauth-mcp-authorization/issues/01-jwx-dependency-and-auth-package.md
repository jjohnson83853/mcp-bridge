# 01: Add JWX dependency and internal auth package

**What to build:** Add `github.com/lestrrat-go/jwx/v2` to `go.mod`. In `internal/auth/config.go`, define `Config` for OAuth settings with environment variable loaders and defaults (`POCKET_ID_ISSUER`, `POCKET_ID_JWKS_URL`, `MCP_RESOURCE_URL`, `MCP_REQUIRED_SCOPE`). In `internal/auth/claims.go`, define the `Claims` model and context helpers (`ClaimsFromContext`, `ContextWithClaims`, `SubjectFromContext`).

**Blocked by:** None.

Status: resolved

Type: task

Source: OAuth 2.1 MCP Authorization Specification (`.scratch/oauth-mcp-authorization/spec.md`).

- [x] Add `github.com/lestrrat-go/jwx/v2` to `go.mod` via `go get github.com/lestrrat-go/jwx/v2@v2.1.7` and run `go mod tidy`.
- [x] In `internal/auth/config.go`, define `Config` struct holding `PocketIDIssuer`, `PocketIDJWKSURL`, `MCPResourceURL`, `MCPRequiredScope`.
- [x] In `internal/auth/config.go`, implement `NewConfigFromEnv()` reading environment variables with defaults:
  - `POCKET_ID_ISSUER`: default `"https://id.bullprog.com"`
  - `POCKET_ID_JWKS_URL`: default `"https://id.bullprog.com/.well-known/jwks.json"`
  - `MCP_RESOURCE_URL`: default `"https://mcp.bullprog.com"`
  - `MCP_REQUIRED_SCOPE`: default `""`
- [x] In `internal/auth/claims.go`, define `Claims` struct (`Subject`, `Email`, `Issuer`, `Audience []string`, `Scopes []string`, `RawToken jwt.Token`) and context functions `ContextWithClaims`, `ClaimsFromContext`, and `SubjectFromContext`.
- [x] In `internal/auth/config_test.go`, add unit tests verifying environment loading and fallback defaults.
- [x] In `internal/auth/claims_test.go`, add unit tests verifying context injection and extraction.

## Answer

Implemented in `internal/auth/config.go` and `internal/auth/claims.go` with dependencies recorded in `go.mod` / `go.sum`. Unit tests in `internal/auth/config_test.go` and `internal/auth/claims_test.go` pass with 100% statement coverage.

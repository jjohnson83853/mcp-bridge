# 04: Inbound transport context integration and session binding

**What to build:** In `internal/service/inbound.go`, update `InboundHandler` to recognize authenticated claims in request context. When `auth.ClaimsFromContext(r.Context())` is present, bypass internal static token matching while binding session affinity (`Mcp-Session-Id`) to the SHA-256 hash of the Bearer token.

**Blocked by:** 03.

Status: resolved

Type: task

Source: OAuth 2.1 MCP Authorization Specification (`.scratch/oauth-mcp-authorization/spec.md`).

- [x] In `internal/service/inbound.go:authenticate()`, check `claims := auth.ClaimsFromContext(r.Context())`.
- [x] If claims are non-nil: compute `tokenHash := sha256.Sum256([]byte(bearerToken))` and return `tokenHash, true`.
- [x] If claims are nil: proceed with existing static token hash matching against `h.acceptedTokens()`.
- [x] Update 401 challenge header emitted by `InboundHandler` on session failure to preserve `WWW-Authenticate` format or challenge attributes.
- [x] In `internal/service/inbound_test.go`, add unit tests asserting that context-authenticated requests bypass static token hashes and successfully create/maintain sessions.
- [x] Verify that all existing unit tests in `internal/service` continue to pass without regression.

## Answer

Implemented in `internal/service/inbound.go:authenticate()`. Verified in `internal/service/inbound_test.go` with `TestInboundHandlerContextClaimsBypass` covering empty static tokens, non-matching static tokens, negative header controls, and full session affinity. All existing tests pass without regression.

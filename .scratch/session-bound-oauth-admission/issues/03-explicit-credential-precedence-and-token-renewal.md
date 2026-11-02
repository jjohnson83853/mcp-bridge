# 03 — Explicit Credential Precedence & Dynamic Token Renewal

**What to build:** End-to-end credential lifecycle handling on active sessions. If a request presents an invalid, expired, or malformed Bearer token alongside an active session ID, it is immediately rejected with HTTP 401/403 (explicit credentials always take precedence and are never rescued by session admission). If a request presents a fresh valid Bearer token whose identity context (`iss`, `sub`, client ID, audience, scopes) matches the active session, the session's bound token fingerprint and 8-hour authorization timer are refreshed without interrupting the ongoing MCP session.

**Blocked by:** 02 — Session Admission Seam & Tokenless Post-Init Discovery Burst

**Status:** ready-for-agent

- [ ] Any request presenting an `Authorization` header triggers unconditional credential evaluation before any session lookup.
- [ ] Requests presenting an invalid, expired, or malformed Bearer token are rejected with HTTP 401 Unauthorized, even if an active, valid `Mcp-Session-Id` header is present.
- [ ] Requests presenting a token with insufficient scope are rejected with HTTP 403 Forbidden, even if an active `Mcp-Session-Id` header is present.
- [ ] Presenting a fresh valid Bearer token alongside an active `Mcp-Session-Id` validates that the token's identity context (`iss`, `sub`, client ID, audience, and scopes) matches the session.
- [ ] Upon match, the session's bound token fingerprint is updated to the new token's SHA-256 hash and the authorization deadline is reset to `now + 8h` without tearing down or replacing protocol session state.
- [ ] If a valid Bearer token with a conflicting identity or client is presented with an existing session ID, the request is rejected with HTTP 401 to prevent cross-identity session hijacking.
- [ ] Unit and loopback wire tests in `internal/auth/middleware_test.go`, `internal/service/inbound_test.go`, and `cmd/mcp-bridge/serve_oauth_test.go` assert credential precedence and successful token renewal.

# 04 — Authorization Ceiling & Scope Policy Enforcement

**What to build:** Enforces the 8-hour maximum authorization ceiling and dynamic scope checks on session-admitted requests. Beyond 8 hours from the last Bearer token validation, tokenless requests receive an HTTP 401 OAuth challenge while session protocol state is preserved until idle expiry. Furthermore, stored scopes on session-admitted calls are rechecked against the server's current required scope (`MCP_REQUIRED_SCOPE`), immediately rejecting requests with HTTP 403 `scope_insufficient` if the policy is not satisfied.

**Blocked by:** 02 — Session Admission Seam & Tokenless Post-Init Discovery Burst, 03 — Explicit Credential Precedence & Dynamic Token Renewal

**Status:** ready-for-agent

- [ ] Each authenticated session record records an absolute authorization deadline set to 8 hours from the most recent successful Bearer token validation.
- [ ] Session-only requests arriving before the 8-hour deadline and within the sliding idle window are admitted.
- [ ] Tokenless requests arriving after the 8-hour deadline are rejected with HTTP 401 Unauthorized carrying an RFC 9728 `WWW-Authenticate` challenge.
- [ ] Protocol session state survives the 8-hour authorization deadline until normal idle expiry, allowing a matching fresh Bearer token (per Ticket 03) to re-authorize the session.
- [ ] Session-admitted requests re-evaluate stored scopes against the server's current `MCP_REQUIRED_SCOPE` on every request.
- [ ] If server policy increases the required scope and the session lacks it, the request is rejected with HTTP 403 Forbidden and `scope_insufficient` diagnostics.
- [ ] Unit and loopback wire tests in `internal/auth/middleware_test.go`, `internal/service/inbound_test.go`, and `cmd/mcp-bridge/serve_oauth_test.go` assert deadline expiration, scope rechecks, and challenge emission.

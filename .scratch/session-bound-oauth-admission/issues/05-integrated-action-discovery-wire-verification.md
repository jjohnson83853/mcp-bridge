# 05 — Integrated Action Discovery Wire Verification

**What to build:** Complete end-to-end integration wire test against the composed bridge server harness validating the entire ChatGPT lifecycle: unauthenticated challenge $\rightarrow$ metadata retrieval $\rightarrow$ OAuth initialization $\rightarrow$ tokenless post-init discovery burst (`notifications/initialized` and `tools/list`) $\rightarrow$ tokenless tool execution $\rightarrow$ token renewal $\rightarrow$ authorization expiry $\rightarrow$ rejection of unauthenticated bogus sessions.

**Blocked by:** 01 — Canonical Endpoint Resource Metadata Alignment, 02 — Session Admission Seam & Tokenless Post-Init Discovery Burst, 03 — Explicit Credential Precedence & Dynamic Token Renewal, 04 — Authorization Ceiling & Scope Policy Enforcement

**Status:** ready-for-agent

- [ ] End-to-end wire test in `cmd/mcp-bridge/serve_oauth_test.go` exercises the complete lifecycle against the composed bridge server:
  1. Discovery probe to `/mcp` receives HTTP 401 with `WWW-Authenticate` challenge pointing to canonical metadata.
  2. Client follows metadata URL and retrieves payload with `resource` pointing to the `/mcp` canonical endpoint.
  3. Client initializes session via `POST /mcp` with a valid Bearer token, receives HTTP 200 OK, and extracts `Mcp-Session-Id`.
  4. Client sends `notifications/initialized` with only `Mcp-Session-Id` (no `Authorization`), receiving HTTP 200 OK or 202 Accepted.
  5. Client sends `tools/list` with only `Mcp-Session-Id`, receiving HTTP 200 OK with the catalog containing configured pinned tools.
  6. Client sends `tools/call` with only `Mcp-Session-Id`, receiving HTTP 200 OK with execution results.
  7. Client rotates token and sends fresh Bearer token alongside `Mcp-Session-Id`, successfully renewing the session binding.
  8. A request with a bogus/unknown session ID and no `Authorization` returns HTTP 401 with `credentials_missing` without allocating a session.
  9. A request with an invalid Bearer token and active session returns HTTP 401, confirming credential precedence.
  10. When `MCP_SESSION_ADMISSION_ENABLED` is false, tokenless requests fail with HTTP 401.
- [ ] Structured completion records on stdout confirm truthful provenance: `credential_state: "accepted"`, `scope_state: "accepted"`, `session_state: "existing"`, `binding_relation: "match"`.
- [ ] All unit, integration, and race detector tests pass clean across the repository (`go test -race ./...`).

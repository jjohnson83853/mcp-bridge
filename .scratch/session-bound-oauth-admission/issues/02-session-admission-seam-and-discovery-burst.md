# 02 — Session Admission Seam & Tokenless Post-Init Discovery Burst

**What to build:** The core tokenless discovery burst under the `MCP_SESSION_ADMISSION_ENABLED` opt-in gate. After authenticating an `initialize` handshake and receiving an `Mcp-Session-Id`, a client can send follow-up requests (`notifications/initialized` returning 200/202, `tools/list` returning 200 with pinned tools, and `tools/call` returning 200) presenting only the `Mcp-Session-Id` header without an `Authorization` header. Requests presenting unknown or unauthenticated session IDs fail with HTTP 401 `credentials_missing` without allocating or rehydrating session records in the session store.

**Blocked by:** None — can start immediately

**Status:** ready-for-agent

- [ ] Configuration and environment variable `MCP_SESSION_ADMISSION_ENABLED` (boolean, default false) controls session-bound admission.
- [ ] Authentication middleware accepts an optional session admission validator hook returning authenticated claims.
- [ ] When enabled, requests to `/mcp` with an empty `Authorization` header and a valid, server-issued OAuth `Mcp-Session-Id` are admitted and proceed to the streamable handler.
- [ ] Post-initialization notification `notifications/initialized` without an `Authorization` header returns HTTP 200 OK or 202 Accepted.
- [ ] Subsequent `tools/list` and `tools/call` requests without an `Authorization` header return HTTP 200 OK with the tool catalog (containing pinned tools) and execution results.
- [ ] Requests without an `Authorization` header and an unknown, unauthenticated, or empty `Mcp-Session-Id` are rejected with HTTP 401 Unauthorized and `credentials_missing` diagnostics.
- [ ] The inbound session store strictly guards the rehydration branch so that unauthenticated requests never allocate or insert records in `h.sessions`.
- [ ] Session-admitted requests record `credential_state: "accepted"`, `scope_state: "accepted"`, `session_state: "existing"`, and `binding_relation: "match"`.
- [ ] When the gate is disabled, requests without an `Authorization` header fail with HTTP 401 `credentials_missing` regardless of session presence.
- [ ] Unit and loopback wire tests in `internal/auth/middleware_test.go`, `internal/service/inbound_test.go`, and `cmd/mcp-bridge/serve_oauth_test.go` assert tokenless admission, unknown session rejection, and absence of spurious session creation.

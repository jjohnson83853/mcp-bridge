# Session-Bound OAuth Admission and RFC 9728 Canonical Resource Alignment

Status: ready-for-agent

<!-- specdd:requirements:start -->
## 1. Requirements

### R1 — Canonical Endpoint Resource Metadata Alignment
Protected resource metadata responses served at root `/.well-known/oauth-protected-resource` and path-bearing discovery routes MUST advertise a canonical `resource` URL ending in `/mcp`. When `MCP_RESOURCE_URL` is configured with an origin URL (e.g. `https://tool.bullprog.com`), the handler normalizes and advertises `https://tool.bullprog.com/mcp` across all paths. Production default configuration pins `DefaultMCPResourceURL` to `https://mcp.bullprog.com/mcp`. Bidirectional audience tolerance accepts tokens targeted at both the base origin and the canonical endpoint path.

### R2 — Session Admission Seam & Tokenless Post-Init Discovery Burst
An opt-in configuration option `client_auth.session_admission_enabled` and environment variable `MCP_SESSION_ADMISSION_ENABLED` (boolean, default: false) controls session-bound admission. The authentication middleware provides an optional hook `WithSessionAdmission(func(sessionID string) (*auth.Claims, bool))`. When enabled, requests to `/mcp` with an empty `Authorization` header and a valid, server-issued OAuth `Mcp-Session-Id` are admitted and proceed to the streamable handler. Post-init `notifications/initialized` returns HTTP 200 or 202, and subsequent `tools/list` and `tools/call` requests return HTTP 200. Unknown or unauthenticated session IDs fail with HTTP 401 `credentials_missing` without allocating or rehydrating session records in the session store. Structured completion records emit `credential_state: "accepted"`, `scope_state: "accepted"`, `session_state: "existing"`, and `binding_relation: "match"`. When disabled, tokenless requests fail with HTTP 401.

### R3 — Explicit Credential Precedence & Dynamic Token Renewal
Any request presenting an `Authorization` header triggers unconditional credential evaluation before any session lookup. Invalid, expired, or malformed tokens are rejected with HTTP 401 Unauthorized, even if an active `Mcp-Session-Id` header is present. Tokens with insufficient scope are rejected with HTTP 403 Forbidden. Presenting a fresh valid Bearer token alongside an active `Mcp-Session-Id` validates that the token's identity context (`iss`, `sub`, client ID, audience, and scopes) matches the session; upon match, the session's bound token fingerprint is updated to the new token's SHA-256 hash and the authorization deadline is reset to `now + 8h` without tearing down or replacing protocol session state. Conflicting identities are rejected with HTTP 401.

### R4 — Authorization Ceiling & Scope Policy Enforcement
Each authenticated session record records an absolute authorization deadline set to 8 hours from the most recent successful Bearer token validation. Tokenless requests arriving before the 8-hour deadline and within sliding idle window are admitted. Tokenless requests arriving after the 8-hour deadline are rejected with HTTP 401 carrying an RFC 9728 `WWW-Authenticate` challenge. Protocol session state survives until normal idle expiry. Stored scopes are rechecked against the server's current `MCP_REQUIRED_SCOPE` on every request; insufficient scope returns HTTP 403 `scope_insufficient`.

### R5 — Integrated Action Discovery Wire Verification
End-to-end loopback wire tests in `cmd/mcp-bridge/serve_oauth_test.go` exercise the complete lifecycle against the composed bridge server: probe receives 401 challenge -> retrieves canonical `/mcp` metadata -> initializes session via valid Bearer -> tokenless `notifications/initialized` (200/202) -> tokenless `tools/list` (200) -> tokenless `tools/call` (200) -> token renewal -> bogus session rejection (401) -> invalid token rejection (401) -> opt-in gate enforcement (401 when false). All unit, integration, and race detector tests pass clean across repository (`go test -race ./...`).

**Non-goals:** live manual interactive testing against ChatGPT in CI; token passthrough to upstream MCP servers; anonymous execution of tools without valid initial OAuth handshake; relaxing pinned tools requirement.
<!-- specdd:requirements:end -->

<!-- specdd:design:start -->
## 2. Design

### 2.1 Configuration and Opt-In Gate (`internal/auth/config.go`, `cmd/mcp-bridge/serve.go`)
- Add `SessionAdmissionEnabled bool` to `auth.Config`, loaded from `MCP_SESSION_ADMISSION_ENABLED` (default false) and `DefaultMCPResourceURL = "https://mcp.bullprog.com/mcp"`.
- Support `client_auth.session_admission_enabled` in bridge YAML config.

### 2.2 Canonical Resource Identifier Normalization (`internal/auth/metadata.go`)
- Normalize `cfg.MCPResourceURL` so that both `originPayload` and `endpointPayload` advertise the canonical `/mcp` resource URL when queried at root or path-bearing endpoints.
- Update `internal/auth/metadata_test.go` and `cmd/mcp-bridge/serve_oauth_test.go` to assert canonical `/mcp` URL advertisement.

### 2.3 Authentication Middleware Admission Hook (`internal/auth/middleware.go`)
- Introduce option `WithSessionAdmission(func(sessionID string) (*auth.Claims, bool))`.
- When `authHeader == ""`:
  - If `options.sessionAdmission != nil` and `r.Header.Get("Mcp-Session-Id") != ""`:
    - Query hook with session ID.
    - If valid claims returned:
      - Check scopes against `cfg.MCPRequiredScope`. If insufficient, emit 403 Forbidden `scope_insufficient`.
      - If scope accepted, inject claims into request context and emit decision with `credential_state: "accepted"`, `scope_state: "accepted"`, `session_state: "existing"`, `binding_relation: "match"`.
      - Call `next.ServeHTTP(w, r)` and return.
  - If hook returns false or hook is absent, proceed to emit 401 `credentials_missing`.
- If `authHeader != ""` is present, always validate credentials first.

### 2.4 Inbound Handler Session Liveness, Renewal & Guarded Rehydration (`internal/service/inbound.go`)
- Update `sessionRecord` to track:
  - `oauthAuthenticated bool`
  - `serverIssued bool`
  - `claims *auth.Claims`
  - `authDeadline time.Time`
- Expose session admission validator method on `*InboundHandler` or constructor callback wired in `cmd/mcp-bridge/serve.go`.
- Validate authorization ceiling (`now < bound.authDeadline` where `authDeadline = now + 8*time.Hour`).
- Guard `!exists` branch so tokenless unauthenticated requests never allocate or rehydrate sessions in `h.sessions`.
- On valid Bearer token request with existing session ID, verify identity context matches (`iss`, `sub`, `aud`), renew `bound.token = candidate` and `bound.authDeadline = now + 8*time.Hour`.
<!-- specdd:design:end -->

<!-- specdd:tasks:start -->
## 3. Tasks

### T1 — Canonical Endpoint Resource Metadata Alignment
- **Inputs:** Approved spec; Ticket 01; existing `internal/auth/metadata.go`, `internal/auth/config.go`
- **Files:** `internal/auth/metadata.go`, `internal/auth/metadata_test.go`, `internal/auth/config.go`, `cmd/mcp-bridge/serve_oauth_test.go`
- **Verification:** Unit and wire tests asserting `/mcp` canonical resource across root and path-bearing endpoints.
- **ACs:** AC-1

### T2 — Session Admission Seam & Tokenless Post-Init Discovery Burst
- **Inputs:** Approved spec; Ticket 02; `internal/auth/middleware.go`, `internal/service/inbound.go`, `cmd/mcp-bridge/serve.go`
- **Files:** `internal/auth/middleware.go`, `internal/auth/middleware_test.go`, `internal/service/inbound.go`, `internal/service/inbound_test.go`, `cmd/mcp-bridge/serve.go`, `cmd/mcp-bridge/serve_oauth_test.go`
- **Verification:** Loopback wire test demonstrating tokenless post-init burst (`notifications/initialized` 200/202, `tools/list` 200) under `MCP_SESSION_ADMISSION_ENABLED=true`. Unknown sessions rejected with 401 without allocation.
- **ACs:** AC-2

### T3 — Explicit Credential Precedence & Dynamic Token Renewal
- **Inputs:** Approved spec; Ticket 03; `internal/auth/middleware.go`, `internal/service/inbound.go`
- **Files:** `internal/auth/middleware.go`, `internal/auth/middleware_test.go`, `internal/service/inbound.go`, `internal/service/inbound_test.go`, `cmd/mcp-bridge/serve_oauth_test.go`
- **Verification:** Unit/wire tests asserting bad bearer rejected immediately (401/403) despite valid session; valid matching bearer refreshes `boundToken` and resets `authDeadline` to `now + 8h`. Mismatched identity rejected.
- **ACs:** AC-3

### T4 — Authorization Ceiling & Scope Policy Enforcement
- **Inputs:** Approved spec; Ticket 04; `internal/auth/middleware.go`, `internal/service/inbound.go`
- **Files:** `internal/auth/middleware.go`, `internal/auth/middleware_test.go`, `internal/service/inbound.go`, `internal/service/inbound_test.go`, `cmd/mcp-bridge/serve_oauth_test.go`
- **Verification:** Unit/wire tests asserting 8-hour authorization ceiling rejection (401 with challenge) while protocol session survives until idle expiry. Dynamic scope checks return 403 `scope_insufficient`.
- **ACs:** AC-4

### T5 — Integrated Action Discovery Wire Verification
- **Inputs:** Approved spec; Ticket 05; all prior tasks complete
- **Files:** `cmd/mcp-bridge/serve_oauth_test.go`
- **Verification:** End-to-end integration test exercising complete ChatGPT lifecycle against composed server: probe 401 -> canonical metadata -> OAuth init -> tokenless notifications/initialized -> tokenless tools/list -> tokenless tools/call -> token renewal -> bogus session 401 -> disabled gate 401. Full suite clean under `go test -race ./...`.
- **ACs:** AC-5
<!-- specdd:tasks:end -->

<!-- specdd:envelope:start -->
## 4. Envelope
Allowed paths:
- `cmd/mcp-bridge/serve.go`
- `cmd/mcp-bridge/serve_oauth_test.go`
- `internal/auth/config.go`
- `internal/auth/config_test.go`
- `internal/auth/metadata.go`
- `internal/auth/metadata_test.go`
- `internal/auth/middleware.go`
- `internal/auth/middleware_test.go`
- `internal/config/config.go`
- `internal/service/inbound.go`
- `internal/service/inbound_test.go`
- `.scratch/session-bound-oauth-admission/*`
<!-- specdd:envelope:end -->

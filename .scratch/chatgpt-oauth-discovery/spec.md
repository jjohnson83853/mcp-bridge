# ChatGPT Action Discovery and RFC 9728 OAuth Protected Resource Routing

<!-- specdd:requirements:start -->
## 1. Requirements

### R1 — RFC 9728 Multi-Path Metadata Discovery and Resource Advertisement
Expose the protected-resource metadata handler on all three discovery paths: `/.well-known/oauth-protected-resource`, `/.well-known/oauth-protected-resource/mcp`, and `/mcp/.well-known/oauth-protected-resource`. For an origin-only `MCPResourceURL`, root metadata MUST advertise the configured base URL and the two `/mcp`-bearing paths MUST advertise `<MCPResourceURL>/mcp`. For a path-bearing configured URL already ending in `/mcp`, all three paths MUST advertise that configured URL. All three paths MUST bypass bearer authentication and MUST be classified as registered (`metadata_target_registered: "yes"`) in challenge diagnostics.

### R2 — Bidirectional Audience Tolerance
The authentication middleware MUST accept bearer tokens whose audience matches the target resource URL directly (ignoring trailing slashes) or whose audience and target differ only by a `/mcp` suffix in either direction. Unrelated audiences MUST be rejected with `jwt_audience_invalid` and HTTP 401. Signature, issuer and expiry validation MUST precede audience matching; scope validation MUST follow it. No suffix tolerance may bypass those checks.

### R3 — Route Normalization and Root Probe Handling
Register `/mcp` and `/mcp/` directly to the authenticated inbound handler without HTTP 301/302/307/308 redirects that could strip `Authorization`. Register `POST /{$}` (Go 1.22+ exact-root pattern) through the authentication middleware so a root discovery probe receives an HTTP 401 challenge with `WWW-Authenticate` rather than a 404 or redirect. Keep the catch-all `/` registration for unmatched paths.

### R4 — Bounded Structured Missing-Credential Diagnostics
When a request reaches a protected endpoint without an `Authorization` header, the bridge MUST emit exactly one structured `http_complete` diagnostic record (schema `oauth_diag_v1`) on stdout carrying bounded request classification: `http_method`, `path_class`, `route`, `failure_stage: "outer_oauth"`, `failure_reason: "credentials_missing"`, `auth_mode: "oauth"`, `credential_state: "missing"`, `challenge_kind: "resource_metadata"`, `challenge_target`, `metadata_target_registered`, `request_id`/`correlation_state`, `level: "warn"`, and `diag_schema: "oauth_diag_v1"`. The bridge MUST NOT emit a duplicate unstructured diagnostic stream (no standard `log.Printf`/stderr line) and MUST NOT include raw path, query, host, user-agent, header, realm, scope, token or body values. The record MUST join to the response `Mcp-Diagnostic-Id` header when correlation is available.

**Non-goals:** extending the closed `oauth_diag_v1` schema with raw request values; new Prometheus metrics, dashboards or alerts; Pocket ID server changes; MCP protocol stream or upstream tool execution changes; RFC 7591 dynamic client registration; storage checksum verification; deployment or infrastructure changes; live ChatGPT client verification.
<!-- specdd:requirements:end -->

<!-- specdd:design:start -->
## 2. Design

### 2.1 Routing and Composition Root (`cmd/mcp-bridge/serve.go`)
In `inboundMux`:
- Register `auth.NewProtectedResourceHandler(authCfg)` on all three RFC 9728 discovery paths:
  - `/.well-known/oauth-protected-resource`
  - `/.well-known/oauth-protected-resource/mcp`
  - `/mcp/.well-known/oauth-protected-resource`
- Register `inboundMux.Handle("/mcp", authMiddleware(inbound))` and `inboundMux.Handle("/mcp/", authMiddleware(inbound))` directly so neither `/mcp` nor `/mcp/` triggers an HTTP 301/307 redirect.
- Register `inboundMux.Handle("POST /{$}", authMiddleware(inbound))` using the Go 1.22+ exact-match root pattern to ensure root POST probes receive an authenticated 401 challenge without conflicting with `/mcp/`.
- Preserve `inboundMux.Handle("/", inbound)` as the catch-all router.

### 2.2 Protected Resource Metadata Handler (`internal/auth/metadata.go`)
- Pre-compute two immutable JSON payloads at constructor initialization:
  - `originPayload`: `Resource = cfg.MCPResourceURL`
  - `endpointPayload`: `Resource = cfg.MCPResourceURL + "/mcp"` (appended only if `/mcp` suffix is not already present)
- In `ServeHTTP`, inspect `r.URL.Path`: if it contains `/mcp`, serve `endpointPayload`; otherwise serve `originPayload`.
- Set `Content-Type: application/json` and contribute `observe.HTTPDecision{RouteOutcome: "metadata"}`.

### 2.3 Authentication Middleware & Diagnostics (`internal/auth/middleware.go`)
- Update `isBypassed` to include all three well-known metadata paths alongside `/livez`, `/readyz`, and `/healthz`.
- Update `challengeClassification` to return `("path_appended_well_known", "yes")` when `resourcePathBearing(resourceURL)` is true.
- Update `audienceMatches`: trim trailing slashes, then compare with bidirectional `/mcp` suffix tolerance (`TrimSuffix(aud, "/mcp") == target || TrimSuffix(target, "/mcp") == aud`).
- For missing `Authorization` (`authHeader == ""`), rely exclusively on `internal/observe` typed decision records (`failure_stage: "outer_oauth"`, `failure_reason: "credentials_missing"`). Do not emit duplicate unstructured `log.Printf` logs.
<!-- specdd:design:end -->

<!-- specdd:tasks:start -->
## 3. Tasks

### T1 — RFC 9728 Multi-Path Metadata Discovery and Path-Aware Advertisement
- **Inputs:** Approved spec; Ticket 01; existing `internal/auth/metadata.go`, `cmd/mcp-bridge/serve.go`
- **Files:** `internal/auth/metadata.go`, `internal/auth/middleware.go`, `cmd/mcp-bridge/serve.go`, `internal/auth/metadata_test.go`, `cmd/mcp-bridge/serve_oauth_test.go`
- **Verification:** Unit tests asserting dual payload generation for root vs path-bearing URLs; wire tests asserting HTTP 200 and application/json on all three paths. Confirm `metadata_target_registered: "yes"` in challenge classification.
- **ACs:** AC1, AC2, AC3

### T2 — Bidirectional Audience Tolerance and Missing-Credential Safe Diagnostics
- **Inputs:** Approved spec; Ticket 02; existing `internal/auth/middleware.go`
- **Files:** `internal/auth/middleware.go`, `internal/auth/middleware_test.go`, `cmd/mcp-bridge/serve_oauth_test.go`
- **Verification:** Unit tests for audience permutations (origin vs endpoint, trailing slashes, unrelated domains). Remove duplicate raw `log.Printf` statements. Verify `oauth_diag_v1` typed completion records without sensitive data leakage.
- **ACs:** AC4, AC5, AC6

### T3 — Route Normalization and Root POST Discovery Probe Handling
- **Inputs:** Approved spec; Ticket 03; existing `cmd/mcp-bridge/serve.go`
- **Files:** `cmd/mcp-bridge/serve.go`, `cmd/mcp-bridge/serve_oauth_test.go`
- **Verification:** Wire tests asserting `POST /` receives HTTP 401 challenge without redirects; `/mcp` and `/mcp/` handled directly without 301/307. Confirm Go ServeMux pattern `POST /{$}` initializes without conflict.
- **ACs:** AC7

### T4 — Integrated Action Discovery Wire Verification and Diagnostic Provenance
- **Inputs:** Approved spec; Ticket 04; all prior tasks complete
- **Files:** `cmd/mcp-bridge/serve_oauth_test.go`, `cmd/mcp-bridge/serve_provenance_test.go`
- **Verification:** End-to-end wire test simulating full post-OAuth discovery flow with `MCP_RESOURCE_URL=https://mcp.bullprog.com/mcp`: 401 challenge on `/mcp` -> follow `resource_metadata` to retrieve metadata -> send valid JSON-RPC `initialize` request to `/mcp` with Bearer token -> assert HTTP 200 and valid JSON-RPC result with `Mcp-Session-Id` header and expected diagnostic provenance.
- **ACs:** AC8
<!-- specdd:tasks:end -->

<!-- specdd:envelope:start -->
## 4. Envelope
Allowed paths:
- `cmd/mcp-bridge/serve.go`
- `cmd/mcp-bridge/serve_oauth_test.go`
- `cmd/mcp-bridge/serve_provenance_test.go`
- `internal/auth/metadata.go`
- `internal/auth/metadata_test.go`
- `internal/auth/middleware.go`
- `internal/auth/middleware_test.go`
- `.scratch/chatgpt-oauth-discovery/*`
<!-- specdd:envelope:end -->

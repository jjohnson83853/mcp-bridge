# ChatGPT Resource Indicator Alignment and RFC 9728 Discovery

<!-- specdd:status:start -->
## 1. STATUS
- **Spec ID:** `chatgpt-resource-indicator-alignment`
- **Revision:** `r15`
- **State:** `APPROVED`
- **Objective:** Align the bridge's OAuth 2.0 Protected Resource metadata discovery and challenge targets with origin-scoped clients (such as OpenAI ChatGPT Custom Actions) while preserving canonical RFC 9728 endpoint routes for strict endpoint-scoped clients and maintaining trailing-slash normalization compatibility.
- **Canonical Execution State:** Governed by `.omp/plans/execution-state.json` per `docs/agents/issue-tracker.md:20–23`.
<!-- specdd:status:end -->

<!-- specdd:requirements:start -->
## 2. REQUIREMENTS

### R1 — Context-Aware Resource Metadata Advertisement and Normalization
Configuration values `cfg.MCPResourceURL` MUST be normalized by stripping any trailing slashes (`cleanResource := strings.TrimRight(cfg.MCPResourceURL, "/")`), consistent with `challengeMetadataURL` and `ResourcePathMode` in `internal/auth/middleware.go`. Mode detection determines path-bearing configuration if `strings.HasSuffix(cleanResource, "/mcp")`.

When `cfg.MCPResourceURL` is an origin-only URL (e.g. `https://tool.bullprog.com` or `https://tool.bullprog.com/`):
1. Root discovery queries (`GET /.well-known/oauth-protected-resource`) MUST advertise `resource: "https://tool.bullprog.com"`.
2. Path-bearing discovery queries (`GET /.well-known/oauth-protected-resource/mcp` and compatibility alias `GET /mcp/.well-known/oauth-protected-resource`) MUST advertise `resource: "https://tool.bullprog.com/mcp"`.
3. The `WWW-Authenticate` challenge header emitted for unauthenticated requests against origin-configured endpoints MUST point to the root metadata endpoint (`/.well-known/oauth-protected-resource`).

When `cfg.MCPResourceURL` is a path-bearing URL (ending in `/mcp` or `/mcp/`, e.g. `https://tool.bullprog.com/mcp` or `https://tool.bullprog.com/mcp/`):
4. All three discovery routes MUST advertise that normalized path-bearing URL (`https://tool.bullprog.com/mcp`), never creating double slashes (such as `//mcp`).
5. The `WWW-Authenticate` challenge MUST point to the path-appended metadata endpoint (`/mcp/.well-known/oauth-protected-resource`).

#### Compatibility Rationale and RFC 9728 Exceptions
- Under origin configuration (`MCP_RESOURCE_URL="https://mcp.bullprog.com"` or `"https://mcp.bullprog.com/"`):
  - Root discovery advertises the origin resource (`https://mcp.bullprog.com`), allowing clients like OpenAI ChatGPT Custom Actions to validate their RFC 8707 origin-scoped access tokens.
  - Strict RFC 9728 §3.3 clients validating that challenge-discovered metadata on `/mcp` identifies the originally requested URL will reject the origin resource advertised at `/.well-known/oauth-protected-resource`. Strict clients requiring endpoint-scoped identity MUST either configure path-bearing mode (`MCP_RESOURCE_URL="https://mcp.bullprog.com/mcp"`) or discover metadata via the canonical RFC 9728 endpoint route `/.well-known/oauth-protected-resource/mcp`.
  - Under path-bearing configuration, root discovery serves the path-bearing identifier (`https://mcp.bullprog.com/mcp`), which diverges from strict derived-discovery origin identity for root queries.
- Consequence: This design intentionally prioritizes cross-system interoperability for origin-scoped AI clients (ChatGPT) while preserving canonical RFC 9728 endpoint routes for strict endpoint clients and ensuring robust handling of trailing slashes.

### R2 — Integrated ChatGPT Wire Verification
A loopback HTTP wire test `TestChatGPTOAuthAndActionDiscoveryWire` in `cmd/mcp-bridge/serve_oauth_test.go` MUST exercise the end-to-end ChatGPT discovery and execution lifecycle against the composed bridge server with `MCP_RESOURCE_URL="https://mcp.bullprog.com"` and `MCP_SESSION_ADMISSION_ENABLED="true"`:
1. Initial unauthenticated probe to `POST /mcp` receives HTTP 401 Unauthorized with `WWW-Authenticate: Bearer realm="mcp", resource_metadata="https://mcp.bullprog.com/.well-known/oauth-protected-resource"`.
2. Client queries `GET /.well-known/oauth-protected-resource` and receives metadata advertising origin resource `https://mcp.bullprog.com`.
3. Client initializes with an access token scoped to `https://mcp.bullprog.com` -> returns HTTP 200 OK and issues `Mcp-Session-Id`.
4. Client sends rapid post-init discovery burst presenting `Mcp-Session-Id` with no `Authorization` header:
   - `notifications/initialized` returns HTTP 202 Accepted with an empty body (per MCP 2025-11-25 streamable HTTP specification).
   - `tools/list` returns HTTP 200 OK with a valid JSON-RPC tools list result.
5. Follow-up `POST /mcp` presenting an unknown or unauthenticated `Mcp-Session-Id` without an `Authorization` header returns HTTP 401 Unauthorized; the response `mcp-diagnostic-id` header correlates with JSON field `request_id` in exactly one stdout diagnostic record asserting `failure_stage: "outer_oauth"` and `failure_reason: "credentials_missing"`.
6. Existing wire test `TestServeOAuthWire` in `cmd/mcp-bridge/serve_oauth_test.go` line 410 is migrated to assert that root discovery returns `oauthResource` when configured with origin URL.

### Non-Goals
- Changes to Pocket-ID or external OAuth identity providers.
- Interactive end-to-end testing with third-party proprietary services (such as ChatGPT UI) in automated CI.
- Dynamic client registration (RFC 7591) or authorization server metadata generation.
<!-- specdd:requirements:end -->

<!-- specdd:evidence:start -->
## 3. EVIDENCE AND UNCERTAINTY

### Verified Facts
- Live container telemetry (`0936382b33ae` and `f1f46921b08b`) confirmed ChatGPT requested `resource=https://tool.bullprog.com` in Pocket-ID authorization exchange (`GET /authorize?response_type=code&client_id=...&resource=https%3A%2F%2Ftool.bullprog.com...`).
- Live telemetry confirmed `/.well-known/oauth-protected-resource` served `"resource": "https://tool.bullprog.com/mcp"`, triggering client-side rejection ("Authentication succeeded, action discovery failed").
- Session admission (`MCP_SESSION_ADMISSION_ENABLED="true"`) is functional and verified in `internal/service/inbound.go` and `internal/auth/middleware.go`.
- Wire test harness `startOAuthHarness` in `cmd/mcp-bridge/serve_oauth_test.go` successfully simulates in-process JWKS, Pocket-ID issuer, and streamable HTTP.
- Diagnostic encoder physically emits `time`, `level` (`"info"`, `"warn"`, `"error"`), `event`, and `request_id` (`internal/observe/httpdiagnostic_encode.go:67–81`); response header `mcp-diagnostic-id` joins to `request_id` (`cmd/mcp-bridge/serve_oauth_diagnostics_test.go:60–68`).
- Diagnostic severity is governed by `internal/observe/httpdiagnostics.go:795–825`: 2xx default is `"info"` (promoted to `"warn"` on tool error), 4xx/auth/panic unwinding without response is `"warn"`, and 5xx is `"error"`.
- `cmd/covcheck` default gate is confirmed at 90.0% in `cmd/covcheck/main.go:23`.
- `sha256sum --strict` is supported and exits non-zero on improperly formatted checksum lines.
- `internal/auth/middleware.go:49–50` and `middleware.go:88–94` trim trailing slashes via `strings.TrimRight(resourceURL, "/")` for challenge generation and `ResourcePathMode` classification.

### Auto-Decisions [AUTO-DECISION]
1. `[AUTO-DECISION]`: Context-aware payload selection in `NewProtectedResourceHandler` distinguishes origin URL configuration from path-bearing URL configuration after stripping trailing slashes. Rationale: Avoids breaking clients expecting the origin resource while keeping path-bearing configuration consistent for strict clients, and prevents double slashes (`//mcp`) when trailing slashes are present in environment variables. Consequence: Both origin-scoped and endpoint-scoped clients function without code branching outside `metadata.go`.
2. `[AUTO-DECISION]`: Notification responses for `notifications/initialized` return HTTP 202 Accepted with an empty body. Rationale: Matches the MCP 2025-11-25 streamable HTTP transport specification. Consequence: MCP clients strictly parsing HTTP status codes receive standard streamable notifications.

### Inferences & Assumptions
- [INFERENCE]: We assume ChatGPT's client-side OAuth runner will proceed to the discovery burst once `resource` in metadata matches `resource` in token request (`https://tool.bullprog.com`), based on the RFC 8707 / RFC 9728 specification and verified container logs.
<!-- specdd:evidence:end -->

<!-- specdd:design:start -->
## 4. DESIGN

### 4.1 Metadata Handler Route Selection and Normalization (`internal/auth/metadata.go`)
- In `NewProtectedResourceHandler(cfg Config)`:
  - Clean the configured resource URL:
    ```go
    cleanResource := strings.TrimRight(cfg.MCPResourceURL, "/")
    isPathBearing := strings.HasSuffix(cleanResource, "/mcp")
    originPayload := buildMetadata(cleanResource)

    endpointResource := cleanResource
    if !isPathBearing {
        endpointResource += "/mcp"
    }
    endpointPayload := buildMetadata(endpointResource)
    ```
  - In `ServeHTTP`:
    - If `isPathBearing`: serve `endpointPayload` for all requests.
    - Otherwise, if `strings.Contains(r.URL.Path, "/mcp")`: serve `endpointPayload`.
    - Otherwise (root discovery path `/.well-known/oauth-protected-resource` under origin configuration): serve `originPayload`.
    - Set `Content-Type: application/json` and status `200 OK`.

### 4.2 Challenge Target Derivation (`internal/auth/middleware.go`)
- Unauthenticated challenge headers (`WWW-Authenticate`) emitted by the authentication middleware direct clients to `challengeMetadataURL(cfg.MCPResourceURL)`:
  - If `cfg.MCPResourceURL` is origin-only (e.g. `https://mcp.bullprog.com` or `https://mcp.bullprog.com/`), challenge points to `/.well-known/oauth-protected-resource`.
  - If `cfg.MCPResourceURL` ends in `/mcp` (or `/mcp/`), challenge points to `/mcp/.well-known/oauth-protected-resource`.
- Retains bidirectional audience matching (`audienceMatches`) and session-bound admission hook (`options.sessionAdmission`).

### 4.3 Wire Test Alignment (`cmd/mcp-bridge/serve_oauth_test.go`)
- Update `TestServeOAuthWire` line 410:
  - For root metadata discovery (`/.well-known/oauth-protected-resource`) under origin `oauthResource`, assert `metadata.Resource == oauthResource`.
- Add `TestChatGPTOAuthAndActionDiscoveryWire` implementing the 6-step lifecycle described in R2 and AC-5.
### 4.4 Allowed Envelope
- `internal/auth/metadata.go`
- `internal/auth/metadata_test.go`
- `cmd/mcp-bridge/serve_oauth_test.go`
- `cmd/mcp-bridge/serve_hybrid_reload_test.go`
- `internal/storage/store_test.go`
- `coverage.out`
- `.scratch/chatgpt-resource-indicator-alignment/*`
<!-- specdd:design:end -->

<!-- specdd:acceptance:start -->
## 5. ACCEPTANCE CRITERIA

- **AC-1**: When `cfg.MCPResourceURL` is an origin URL (tested with both `https://api.example.org` and `https://api.example.org/`), `GET /.well-known/oauth-protected-resource` returns HTTP 200 with JSON payload containing `"resource": "https://api.example.org"` (normalized without trailing slash), `"authorization_servers": [cfg.PocketIDIssuer]`, and supported scopes/bearer methods. (Maps to R1.1, verified by T1 in `internal/auth/metadata_test.go`).
- **AC-2**: When `cfg.MCPResourceURL` is an origin URL (tested with both `https://api.example.org` and `https://api.example.org/`), both canonical endpoint discovery `GET /.well-known/oauth-protected-resource/mcp` and compatibility alias `GET /mcp/.well-known/oauth-protected-resource` return HTTP 200 with JSON payload containing `"resource": "https://api.example.org/mcp"`. (Maps to R1.2, verified by T1 in `internal/auth/metadata_test.go`).
- **AC-3**: When `cfg.MCPResourceURL` is a path-bearing URL (tested with both `https://api.example.org/mcp` and `https://api.example.org/mcp/`), all three discovery routes (`/.well-known/oauth-protected-resource`, `/.well-known/oauth-protected-resource/mcp`, and `/mcp/.well-known/oauth-protected-resource`) return HTTP 200 with `"resource": "https://api.example.org/mcp"` (normalized without double slashes). (Maps to R1.4, R1.5, verified by T1 in `internal/auth/metadata_test.go` and retained wire check `TestProtectedResourceMetadataWire` in `cmd/mcp-bridge/serve_oauth_test.go:304–310`).
- **AC-4**: In `cmd/mcp-bridge/serve_oauth_test.go`, `TestServeOAuthWire` line 410 asserts that root discovery under origin `oauthResource` returns `metadata.Resource == oauthResource`. (Maps to R1.1, R2.6, verified by T1 in `cmd/mcp-bridge/serve_oauth_test.go`).
- **AC-5**: Loopback wire test `TestChatGPTOAuthAndActionDiscoveryWire` in `cmd/mcp-bridge/serve_oauth_test.go` exercises the exact 6-step ChatGPT flow defined in R2:
  1. Initial unauthenticated probe to `POST /mcp` receives HTTP 401 with `WWW-Authenticate: Bearer realm="mcp", resource_metadata="https://mcp.bullprog.com/.well-known/oauth-protected-resource"`. (Asserts R1.3, R2.1).
  2. `GET /.well-known/oauth-protected-resource` returns `"resource": "https://mcp.bullprog.com"`. (Asserts R2.2).
  3. `POST /mcp` (`initialize`) with Bearer token scoped to origin resource succeeds (`200 OK`) and returns `Mcp-Session-Id`. (Asserts R2.3).
  4. Follow-up `POST /mcp` `notifications/initialized` with `Mcp-Session-Id` and omitted `Authorization` header returns HTTP 202 Accepted with an empty body. (Asserts R2.4).
  5. Follow-up `POST /mcp` `tools/list` with `Mcp-Session-Id` and omitted `Authorization` header returns HTTP 200 OK with valid JSON-RPC tool list. (Asserts R2.4).
  6. Follow-up `POST /mcp` with invalid or unknown `Mcp-Session-Id` without `Authorization` header returns HTTP 401 Unauthorized; the response `mcp-diagnostic-id` header correlates with JSON field `request_id` in exactly one stdout diagnostic record asserting `failure_stage: "outer_oauth"` and `failure_reason: "credentials_missing"`. (Asserts R2.5).
- **AC-6**: Complete repository verification gate satisfies:
  - `go test -race ./...` (exit 0)
  - `go vet ./...` (exit 0)
  - `go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...` (exit 0)
  - `go run ./cmd/covcheck` (exit 0, aggregate coverage ≥ 90.0%). (Maps to R2, verified by T2 across workspace).
<!-- specdd:acceptance:end -->

<!-- specdd:tasks:start -->
## 6. TASK LEDGER

### Traceability Mapping Ledger
| Requirement ID | Acceptance Criteria IDs | Implementing Task ID | Verification Method |
|---|---|---|---|
| R1.1, R1.2 | AC-1, AC-2, AC-4 | T1 | Table-driven unit tests in `internal/auth/metadata_test.go` (including trailing-slash variants) and wire test `TestServeOAuthWire` |
| R1.3 | AC-5.1 | T2 | Wire assertion of origin challenge on unauthenticated `/mcp` probe in `TestChatGPTOAuthAndActionDiscoveryWire` |
| R1.4, R1.5 | AC-3 | T1 (retained wire check) | Table unit test in `internal/auth/metadata_test.go` and existing `TestProtectedResourceMetadataWire` (`serve_oauth_test.go:304–310`) |
| R2.1..R2.5 | AC-5 | T2 | Integration test `TestChatGPTOAuthAndActionDiscoveryWire` (`serve_oauth_test.go`) |
| R2.6 | AC-4 | T1 | Wire test assertion migration in `TestServeOAuthWire` (`serve_oauth_test.go:410`) |
| R2 (Full Gate) | AC-6 | T2 | Full workspace verification gate (race test, vet, coverage profile, covcheck ≥ 90%) |

### T1 — Context-Aware Resource Metadata Advertisement and Normalization
- **Objective:** Serve origin URL on root discovery when origin-configured, and endpoint URL on path-bearing discovery routes, while normalizing any trailing slashes to prevent double slashes.
- **Dependencies:** None
- **Inputs:** Approved spec r15; Ticket 01; `internal/auth/metadata.go`, `internal/auth/metadata_test.go`, `cmd/mcp-bridge/serve_oauth_test.go`
- **Files:** `internal/auth/metadata.go`, `internal/auth/metadata_test.go`, `cmd/mcp-bridge/serve_oauth_test.go`
- **Allowed Scope:** Surgical updates to metadata handler route selection/normalization and corresponding test expectations.
- **Prohibited Scope:** Modifying session admission logic, token validation, or unrelated routing.
- **RED Verification Sequence:**
  1. In `internal/auth/metadata_test.go`:
     - Update root discovery expectation under origin configuration to expect origin URL `https://api.example.org` (leaving production code unchanged).
     - Add test cases covering `https://api.example.org/` and `https://api.example.org/mcp/` to the test table.
  2. Execute: `go test -v -count=1 ./internal/auth -run TestProtectedResourceHandler`
  3. Observe RED failure: `Resource = "https://api.example.org/mcp"; want "https://api.example.org"`.
- **Implementation Deliverable:** Normalization and context-aware payload selection in `internal/auth/metadata.go`; updated line 410 in `cmd/mcp-bridge/serve_oauth_test.go`.
- **GREEN Verification Sequence:**
  1. Update `internal/auth/metadata.go` to normalize `cfg.MCPResourceURL` with `strings.TrimRight` and serve `originPayload` for root discovery requests when configured with origin URL.
  2. Update line 410 of `cmd/mcp-bridge/serve_oauth_test.go` to assert `metadata.Resource == oauthResource`.
  3. Execute: `go test -v -count=1 ./internal/auth -run TestProtectedResourceHandler` -> PASS (GREEN).
  4. Execute: `go test -v -count=1 ./cmd/mcp-bridge -run TestServeOAuthWire` -> PASS (GREEN).
- **Covered ACs:** AC-1, AC-2, AC-3, AC-4

### T2 — Integrated ChatGPT Wire Verification
- **Objective:** Add comprehensive loopback wire test exercising the complete 6-step ChatGPT lifecycle and enforce the aggregate coverage gate.
- **Dependencies:** T1
- **Inputs:** Approved spec r15; Ticket 02; `cmd/mcp-bridge/serve_oauth_test.go`; T1 completed
- **Files:** `cmd/mcp-bridge/serve_oauth_test.go`, `cmd/mcp-bridge/serve_hybrid_reload_test.go`, `internal/storage/store_test.go`, `internal/service/progressive_session_test.go`, `coverage.out`
- **Allowed Scope:** Adding `TestChatGPTOAuthAndActionDiscoveryWire` to `cmd/mcp-bridge/serve_oauth_test.go`, edge tests in `cmd/mcp-bridge/serve_oauth_test.go`, `cmd/mcp-bridge/serve_hybrid_reload_test.go`, `internal/storage/store_test.go`, `internal/service/progressive_session_test.go`, and running full repository verification commands.
- **Prohibited Scope:** Modifying application production code under `cmd/mcp-bridge/*.go` (non-test) or `internal/**/*.go` (non-test).
- **Implementation Deliverable:** `TestChatGPTOAuthAndActionDiscoveryWire` asserting steps 1 through 6 of AC-5.
- **Verification Sequence:**
  1. Add `TestChatGPTOAuthAndActionDiscoveryWire` to `cmd/mcp-bridge/serve_oauth_test.go`.
  2. Execute: `go test -v -count=1 ./cmd/mcp-bridge -run TestChatGPTOAuthAndActionDiscoveryWire` -> PASS (GREEN).
  3. Execute complete repository verification gate:
     - `go test -race ./...` -> exit 0
     - `go vet ./...` -> exit 0
     - `go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...` -> exit 0
     - `go run ./cmd/covcheck` -> exit 0 (aggregate coverage ≥ 90.0% satisfied).
- **Covered ACs:** AC-5, AC-6
<!-- specdd:tasks:end -->

<!-- specdd:envelope:start -->
Allowed Envelope:
- `internal/auth/metadata.go`
- `internal/auth/metadata_test.go`
- `cmd/mcp-bridge/serve_oauth_test.go`
- `cmd/mcp-bridge/serve_hybrid_reload_test.go`
- `internal/storage/store_test.go`
- `internal/service/progressive_session_test.go`
- `coverage.out`
<!-- specdd:envelope:end -->

<!-- specdd:observability:start -->
## 7. OBSERVABILITY

### Logging — Inbound Auth and Metadata Components
- **Collection / Shipping Path:** `internal/observe/httpdiagnostic.go` encodes structured records and writes newline-delimited JSON directly to process standard output under a mutex. There is no asynchronous buffer; writes are synchronous to stdout. In container deployment, Docker captures stdout and Vector forwards records to Loki (`job="vector-logs"`, `app="mcp-bridge"`).
- **JSON Physical Schema:** Relevant physical keys (subset; complete schema defined in `internal/observe/httpdiagnostic_encode.go:67–84`):
  - Physical keys: `time`, `level` (`"info"`, `"warn"`, `"error"`), `event`, `message`, `service`, `environment`, `version`, `duration_ms`, `http_outcome`, `http_status`, `http_method`, `route`, `request_id`, `diag_schema: "oauth_diag_v1"`, `route_outcome`, `failure_stage`, `failure_reason`, `auth_mode`, `credential_state`, `scope_state`, `session_header`, `session_state`, `binding_relation`.
- **Severity Policy:** Governed by `internal/observe/httpdiagnostics.go:795–825`:
  - HTTP 2xx completions emit at `level: "info"`, except when promoted to `level: "warn"` on tool errors (`upstream == "tool_error"`).
  - HTTP 4xx rejections, auth admission failures, client disconnects, and unhandled panic unwinding without response emit at `level: "warn"`.
  - Internal server errors (HTTP 5xx) and unhandled delivery errors emit at `level: "error"`.
- **Event Bindings:**
  - Unauthenticated 401 rejections: `failure_stage: "outer_oauth"`, `failure_reason: "credentials_missing"`, `auth_mode: "oauth"`, `credential_state: "missing"`, `level: "warn"`. Bound to AC-5, verified by T2.
  - Discovery requests: `route_outcome: "metadata"`, `level: "info"`. Production handler contributes this outcome in `internal/auth/metadata.go:44`. (Note that AC-1, AC-2, and AC-3 verify the HTTP response bodies and status codes, and do not inspect stdout diagnostics per the justified exception below).
  - Session-admitted requests: `route_outcome: "oauth_protected"`, `credential_state: "accepted"`, `session_state: "existing"`, `binding_relation: "match"`, `level: "info"`. Bound to AC-5, verified by T2.
- **Request / Event Correlation:** Every 401 response writes HTTP response header `mcp-diagnostic-id: <hex>`. In automated wire verification (AC-5 / T2), the test extracts this header value and verifies correlation with physical JSON field `request_id: <hex>` output on stdout (matching the precedent at `cmd/mcp-bridge/serve_oauth_test.go:332–338` and `serve_oauth_diagnostics_test.go:60–68`).

### Loki
- **Bounded Labels:** `service="mcp-bridge"`, `source_subsystem="mcp-bridge"`, `job="vector-logs"`. Unbounded values (`request_id`, `session_id`) stay in JSON payload fields, never labels.
- **Retention:** Governed by host Loki daemon policy (30 days default in homelab).
- **Ingestion-Failure Handling:** Standard output writes are synchronous to local container stdout. If downstream Vector collector or Loki is unavailable, Docker stdout logging retains container logs according to Docker json-file log rotation; downstream log delivery failures do not block or abort inbound HTTP request execution.

### Prometheus
- **Scrape Path:** Ops listener `/metrics` served on configured operations port (default 9090).
- **Metric Definitions:** `bridge_requests_total{server,operation,outcome}` (`internal/observe/metrics.go:137–140`) tracks overall bridge operations.
- **Bounded Labels:** `server`, `operation`, `outcome`.
- **Justified Metric Exceptions:** No new Prometheus metrics or latency histograms are added. Reason: per-request HTTP route/status labels would cause high-cardinality label explosion across dynamic multi-server MCP environments. Consequence: Prometheus monitors process up/ready status and aggregate operations; fine-grained OAuth and discovery triage relies on LogQL queries on Loki.

### Operations and Verification Exceptions
- **Justified Delivery/Scrape Verification Exception:** Live Vector log shipping to Loki, external Loki ingestion, and Prometheus scraping are not executed during automated local unit and wire testing. Reason: Automated tests execute in an isolated environment without external daemon networks, Vector agents, or Prometheus collectors. Consequence: End-to-end log shipping and scrape availability remain unverified in the local test suite and depend on the container deployment environment. Local wire test AC-5 verifies the emission directly at the process stdout boundary.
- **Justified Dashboard/Alert Verification Exception:** Automated test execution does not trigger live Grafana alert evaluation. Reason: Alerts run asynchronously in host Grafana evaluating historical Loki streams. Consequence: Alert rule firing remains unverified in local tests; reliance rests on diagnostic vocabulary compatibility (`failure_stage: "outer_oauth"`, `failure_reason: "credentials_missing"`).
- **Justified Tracing/Health Exception:** Distributed OpenTelemetry tracing is omitted (the bridge is an edge daemon communicating over synchronous JSON-RPC; no trace collector exists). Service health endpoints (`/healthz`, `/readyz`) are unauthenticated and unchanged by this feature. Reason: Metadata alignment does not alter service readiness or probe behavior. Consequence: Existing health probes continue functioning normally.
- **Justified Metadata Diagnostic Log Assertion Exception:** AC-1, AC-2, and AC-3 assert HTTP status and metadata payload but do not parse stdout diagnostics. Reason: Metadata discovery queries return HTTP 200 with standard RFC 9728 JSON; failure diagnostics are only emitted on rejection paths (AC-5). Consequence: Unit tests remain focused and fast without redundant stdout scraping on successful metadata responses.
- **Verification Tie-Ins:**
  - AC-5 / T2 proves valid JSON emission, stdout delivery, schema `oauth_diag_v1`, and diagnostic correlation between response header `mcp-diagnostic-id` and JSON `request_id`.
<!-- specdd:observability:end -->

<!-- specdd:release_and_recovery:start -->
## 8. RELEASE AND RECOVERY

### Pre-Task Capture
Before mutating any file, execute this fail-closed capture script:
```sh
set -eu
BACKUP_DIR=$(mktemp -d .scratch/chatgpt-resource-indicator-alignment/.backup_XXXXXX)
if [ ! -d "$BACKUP_DIR" ]; then
  echo "ERROR: Failed to allocate unique backup directory" >&2
  exit 1
fi

git rev-parse HEAD > "$BACKUP_DIR/commit_sha"
sha256sum internal/auth/metadata.go internal/auth/metadata_test.go cmd/mcp-bridge/serve_oauth_test.go > "$BACKUP_DIR/pre_hashes"

# Strictly validate that each required path appears exactly once with a 64-hex SHA-256 digest
COUNT_METADATA=$(grep -c -E '^[0-9a-f]{64}  internal/auth/metadata\.go$' "$BACKUP_DIR/pre_hashes" || true)
COUNT_METADATA_TEST=$(grep -c -E '^[0-9a-f]{64}  internal/auth/metadata_test\.go$' "$BACKUP_DIR/pre_hashes" || true)
COUNT_SERVE_OAUTH_TEST=$(grep -c -E '^[0-9a-f]{64}  cmd/mcp-bridge/serve_oauth_test\.go$' "$BACKUP_DIR/pre_hashes" || true)
TOTAL_LINES=$(wc -l < "$BACKUP_DIR/pre_hashes")

if [ "$COUNT_METADATA" -ne 1 ] || [ "$COUNT_METADATA_TEST" -ne 1 ] || [ "$COUNT_SERVE_OAUTH_TEST" -ne 1 ] || [ "$TOTAL_LINES" -ne 3 ]; then
  echo "ERROR: pre_hashes does not contain exactly one valid checksum record for each of the 3 required paths" >&2
  exit 1
fi

cp internal/auth/metadata.go "$BACKUP_DIR/metadata.go"
cp internal/auth/metadata_test.go "$BACKUP_DIR/metadata_test.go"
cp cmd/mcp-bridge/serve_oauth_test.go "$BACKUP_DIR/serve_oauth_test.go"

# Materialize and strictly verify transformed backup hashes without pipeline masking
sed -e 's|internal/auth/|'"$BACKUP_DIR/"'|' -e 's|cmd/mcp-bridge/|'"$BACKUP_DIR/"'|' "$BACKUP_DIR/pre_hashes" > "$BACKUP_DIR/backup_check_hashes"
sha256sum --strict -c "$BACKUP_DIR/backup_check_hashes"

if [ -f "coverage.out" ]; then
  cp coverage.out "$BACKUP_DIR/coverage.out"
  sha256sum coverage.out > "$BACKUP_DIR/coverage_hash"
  COUNT_COV=$(grep -c -E '^[0-9a-f]{64}  coverage\.out$' "$BACKUP_DIR/coverage_hash" || true)
  TOTAL_COV=$(wc -l < "$BACKUP_DIR/coverage_hash")
  if [ "$COUNT_COV" -ne 1 ] || [ "$TOTAL_COV" -ne 1 ]; then
    echo "ERROR: coverage_hash does not contain exactly one valid checksum record for coverage.out" >&2
    exit 1
  fi
  (cd "$BACKUP_DIR" && sha256sum --strict -c coverage_hash)
  echo "true" > "$BACKUP_DIR/had_coverage"
else
  echo "false" > "$BACKUP_DIR/had_coverage"
fi
```
If any command fails, execution aborts immediately and no source files are modified.

### Rollback Trigger
Rollback is invoked ONLY if final repository verification (AC-6) fails and cannot be repaired within the authorized envelope. Intermediate test failures during RED phases of TDD are expected and do NOT trigger rollback.

### Rollback Procedure
Execute this fail-closed rollback script:
```sh
set -eu
if [ -z "${BACKUP_DIR:-}" ] || [ ! -d "$BACKUP_DIR" ]; then
  echo "ERROR: BACKUP_DIR is unset or not a directory" >&2
  exit 1
fi

# 1. Pre-validate manifest files and records exist
if [ ! -f "$BACKUP_DIR/metadata.go" ] || [ ! -f "$BACKUP_DIR/metadata_test.go" ] || [ ! -f "$BACKUP_DIR/serve_oauth_test.go" ]; then
  echo "ERROR: Missing required source backups in $BACKUP_DIR" >&2
  exit 1
fi
if [ ! -f "$BACKUP_DIR/pre_hashes" ] || [ ! -f "$BACKUP_DIR/had_coverage" ]; then
  echo "ERROR: Missing pre_hashes or had_coverage manifest in $BACKUP_DIR" >&2
  exit 1
fi

# 2. Pre-validate that each expected path appears exactly once with a valid 64-hex SHA-256 digest
COUNT_METADATA=$(grep -c -E '^[0-9a-f]{64}  internal/auth/metadata\.go$' "$BACKUP_DIR/pre_hashes" || true)
COUNT_METADATA_TEST=$(grep -c -E '^[0-9a-f]{64}  internal/auth/metadata_test\.go$' "$BACKUP_DIR/pre_hashes" || true)
COUNT_SERVE_OAUTH_TEST=$(grep -c -E '^[0-9a-f]{64}  cmd/mcp-bridge/serve_oauth_test\.go$' "$BACKUP_DIR/pre_hashes" || true)
TOTAL_LINES=$(wc -l < "$BACKUP_DIR/pre_hashes")

if [ "$COUNT_METADATA" -ne 1 ] || [ "$COUNT_METADATA_TEST" -ne 1 ] || [ "$COUNT_SERVE_OAUTH_TEST" -ne 1 ] || [ "$TOTAL_LINES" -ne 3 ]; then
  echo "ERROR: pre_hashes does not contain exactly one valid checksum record for each of the 3 required paths" >&2
  exit 1
fi

# 3. Pre-validate had_coverage format and coverage files if true
COVERAGE_STATE=$(cat "$BACKUP_DIR/had_coverage")
if [ "$COVERAGE_STATE" != "true" ] && [ "$COVERAGE_STATE" != "false" ]; then
  echo "ERROR: Invalid had_coverage marker ($COVERAGE_STATE) in $BACKUP_DIR" >&2
  exit 1
fi

if [ "$COVERAGE_STATE" = "true" ]; then
  if [ ! -f "$BACKUP_DIR/coverage.out" ] || [ ! -f "$BACKUP_DIR/coverage_hash" ]; then
    echo "ERROR: Missing coverage backup or hash in $BACKUP_DIR" >&2
    exit 1
  fi
  COUNT_COV=$(grep -c -E '^[0-9a-f]{64}  coverage\.out$' "$BACKUP_DIR/coverage_hash" || true)
  TOTAL_COV=$(wc -l < "$BACKUP_DIR/coverage_hash")
  if [ "$COUNT_COV" -ne 1 ] || [ "$TOTAL_COV" -ne 1 ]; then
    echo "ERROR: coverage_hash does not contain exactly one valid checksum record for coverage.out" >&2
    exit 1
  fi
  (cd "$BACKUP_DIR" && sha256sum --strict -c coverage_hash)
fi

# 4. Pre-validate that the backups in BACKUP_DIR are completely intact BEFORE touching working files
sed -e 's|internal/auth/|'"$BACKUP_DIR/"'|' -e 's|cmd/mcp-bridge/|'"$BACKUP_DIR/"'|' "$BACKUP_DIR/pre_hashes" > "$BACKUP_DIR/restore_check_hashes"
sha256sum --strict -c "$BACKUP_DIR/restore_check_hashes"

# 5. Restore source files only after all pre-validations pass
cp "$BACKUP_DIR/metadata.go" internal/auth/metadata.go
cp "$BACKUP_DIR/metadata_test.go" internal/auth/metadata_test.go
cp "$BACKUP_DIR/serve_oauth_test.go" cmd/mcp-bridge/serve_oauth_test.go

# 6. Post-restore verification: ensure working tree files match pre_hashes exactly
sha256sum --strict -c "$BACKUP_DIR/pre_hashes"

# 7. Restore or remove coverage.out strictly
if [ "$COVERAGE_STATE" = "true" ]; then
  cp "$BACKUP_DIR/coverage.out" coverage.out
  sha256sum --strict -c "$BACKUP_DIR/coverage_hash"
else
  rm -f coverage.out
fi

# 8. Delete backup directory only after all restores verify clean
rm -rf "$BACKUP_DIR"
git status --porcelain
```

### Recovery Script Fixture Verification
In an isolated test fixture:
1. Valid 3 distinct hashes: Passes validation and backup checks.
2. Duplicate path replacing another required path (e.g. 3 copies of `internal/auth/metadata.go`): `COUNT_METADATA_TEST` is 0, causing immediate non-zero exit and aborting rollback before any file modification.
3. Malformed digest (e.g. `x  internal/auth/metadata.go`): `COUNT_METADATA` is 0 and `sha256sum --strict -c` fails, halting execution immediately with non-zero exit code.
<!-- specdd:release_and_recovery:end -->

<!-- specdd:critic_handoff:start -->
## 9. CRITIC HANDOFF

### Addressed Critic Findings
- **C1 (RFC 9728 Compatibility):** Resolved in Section 2 (R1). Explicitly documents strict RFC 9728 §3.3 client failure under origin configuration and identifies path-bearing mode / canonical endpoint discovery as strict alternatives.
- **C2 (Notification Status):** Resolved in Section 2 (R2.4) and Section 5 (AC-5.4). Requires HTTP 202 Accepted with an empty body.
- **C3 (Deterministic RED/GREEN):** Resolved in Section 6 (T1). T1 updates assertion first, observes RED, applies fix, observes GREEN. T2 provides additive integration proof.
- **C4 (Observability Contract & Severity):** Resolved in Section 7. Physical schema keys accurately match `internal/observe/httpdiagnostic_encode.go:67–84` as a labeled subset of relevant keys. Severity policy aligns with `httpdiagnostics.go:795–825` (`level: "warn"` for 4xx/auth/panic unwinding without response; 200 tool error promotion noted). Explicit justified exceptions with reasons and consequences for unverified live daemon log scraping and alert evaluation. Event binding for metadata clarifies that AC-1..3 assert HTTP bodies without redundant diagnostic scraping.
- **C5 (Exact Path Completeness & Strict Digest Validation):** Resolved in Section 8. Both capture and rollback enforce exact path completeness (`COUNT_METADATA == 1`, `COUNT_METADATA_TEST == 1`, `COUNT_SERVE_OAUTH_TEST == 1`, `TOTAL_LINES == 3`), enforce single valid digest for coverage (`COUNT_COV == 1`, `TOTAL_COV == 1`), use `sha256sum --strict -c` across all backup/restore/coverage checks, materialize transformed manifests without pipeline masking, verify backups before touching workspace files, verify restored files before cleanup, and document verified fixture cases (valid, duplicate, malformed).
- **C6 (Nine-Section Structure & Traceability):** Resolved in Sections 1 through 9. Complete nine-section structure with Envelope nested in subsection 4.4, stable AC-1 through AC-6 IDs, explicit Traceability Mapping Ledger linking R1.3 to AC-5.1 / T2, and task bindings.
- **C7 (Trailing-Slash Normalization Compatibility):** Resolved in Section 2 (R1), Section 4.1, Section 5 (AC-1, AC-2, AC-3), and Section 6 (T1). Strips trailing slashes via `cleanResource := strings.TrimRight(cfg.MCPResourceURL, "/")` before classification and endpoint derivation, preventing double slashes (such as `//mcp`) for inputs like `https://api.example.org/` or `https://api.example.org/mcp/`. T1 table test matrix explicitly covers both origin-with-slash and endpoint-with-slash cases.

### Review Targets
- Trailing-slash normalization in Section 2 (R1), Section 4.1, Section 5 (AC-1..3), and Section 6 (T1).
- Exact path completeness validation and strict digest verification in Section 8.
- Conformance of Section 7 (Observability) with physical JSON schema subset label, severity policy, and justified exceptions.
- Traceability mapping in Section 6.
<!-- specdd:critic_handoff:end -->

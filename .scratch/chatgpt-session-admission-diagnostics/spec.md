# Specification: ChatGPT Session Admission Observability & Diagnostic Transparency

<!-- specdd:status:start -->
## 1. STATUS
- **Spec ID:** `chatgpt-session-admission-diagnostics`
- **Revision:** `r10`
- **Status:** ready-for-agent
- **Objective:** Eliminate diagnostic opacity during ChatGPT action discovery failures by replacing generic `failure_reason: "credentials_missing"` and `session_state: "not_evaluated"` with typed, differentiated session admission outcomes (`session_absent`, `session_fingerprint_mismatch`, `jwt_expired`), verifying handshake lifecycle evidence, preserving the fixed 64-slot schema, asserting zero credential leakage, and ensuring deterministic composed wire verification.
<!-- specdd:status:end -->

<!-- specdd:requirements:start -->
## 2. REQUIREMENTS

### R1 — Typed Session Admission Outcome Taxonomy
The boundary between the service inbound layer (`internal/service/inbound.go`) and the authentication middleware (`internal/auth/middleware.go`) MUST classify the exact state of the session store when evaluating tokenless requests:
- `AdmissionAdmitted`: The session ID exists in `h.sessions`, has non-nil OAuth claims, and `h.now().Before(rec.authDeadline)`.
- `AdmissionNotFound`: The session ID is not present in `h.sessions`.
- `AdmissionUnauthenticated`: The session ID exists in `h.sessions`, but was created without bound OAuth claims (`rec.claims == nil`).
- `AdmissionExpired`: The session ID exists in `h.sessions` with claims, but `!h.now().Before(rec.authDeadline)`.
The expiry boundary condition is settled intentionally as `!h.now().Before(rec.authDeadline)`, ensuring requests at or after the exact deadline are rejected. Unit tests MUST verify before, at, and after the deadline.
A backward-compatible `SessionClaims(sessionID string) (*auth.Claims, bool)` wrapper is retained in `InboundHandler`.

### R2 — Differentiated Outer OAuth Rejection Diagnostics
When a request arrives without an `Authorization` header (`len(authValues) == 0`):
1. If an `Mcp-Session-Id` header is present (`sessionID != ""`):
   - Record `session_header: "present"` across all rejection branches.
   - If session admission is configured (`options.sessionAdmission != nil`):
     - On `AdmissionNotFound`: emit `failure_stage: "outer_oauth"`, `failure_reason: "session_absent"`, `credential_state: "missing"`, `session_state: "absent"`.
     - On `AdmissionUnauthenticated`: emit `failure_stage: "outer_oauth"`, `failure_reason: "session_fingerprint_mismatch"`, `credential_state: "rejected"`, `session_state: "existing"`.
     - On `AdmissionExpired`: emit `failure_stage: "outer_oauth"`, `failure_reason: "jwt_expired"`, `credential_state: "rejected"`, `session_state: "existing"`.
     - On `AdmissionAdmitted`: admit request with claims, recording `session_state: "existing"`, `binding_relation: "match"`.
2. If session admission is unconfigured (`options.sessionAdmission == nil`) or no session ID is present (`sessionID == ""`):
   - Retain `failure_stage: "outer_oauth"`, `failure_reason: "credentials_missing"`, `credential_state: "missing"`, `session_state: "not_evaluated"`.
3. In `cmd/mcp-bridge/serve.go`, parse `MCP_SESSION_ADMISSION_ENABLED` using case-insensitive equality (`strings.EqualFold(v, "true") || v == "1"`).

### R3 — Initialize Handshake Lifecycle & Evidence Preservation
1. In `internal/service/inbound.go`, when an `initialize` response sets the `Mcp-Session-Id` header:
   - Contribute structured session evidence directly to the single terminal completion record: `SessionState: "created"`, `BindingRelation: "not_evaluated"` (since no comparison ran at creation time), and `RefState` reflecting reference availability (`refEvidenceState(ref, refAvailable)`). No secondary log event is emitted.
2. When operating in stateless transport mode where no `Mcp-Session-Id` response header is emitted:
   - Record `SessionState: "not_supplied"` and `SessionBinding: "unavailable"`.

### R4 — Security, Wire Compatibility & Zero Credential Leakage
1. HTTP 401 responses for all rejected tokenless requests MUST return unchanged generic response bodies (`"unauthorized\n"`) with standard `WWW-Authenticate: Bearer realm="mcp", resource_metadata="..."` headers.
2. Differentiated rejection reasons MUST remain strictly internal to stdout JSON diagnostic records (`http_complete`). The protocol-required `Mcp-Session-Id` header is permitted on successful HTTP 200 `initialize` responses; across all rejected-response surfaces (HTTP 401, 403, and 404) and in all diagnostic JSON records (`http_complete`), zero raw bearer tokens, raw session IDs, private keys, and zero SHA-256 hashes of credentials may appear.
3. Diagnostic records MUST comply strictly with the fixed 64-slot schema (`numHTTPSlots = 64`) and stay within the 4096-byte record budget without adding or renaming slot keys.
<!-- specdd:requirements:end -->

<!-- specdd:design:start -->
## 3. ACCEPTANCE CRITERIA

- **AC1 (Taxonomy & Expiry Boundary):** `SessionAdmission(sessionID)` returns `AdmissionNotFound`, `AdmissionUnauthenticated`, `AdmissionExpired`, and `AdmissionAdmitted` matching exact internal state under controlled clock times. Tests explicitly verify before-deadline (`AdmissionAdmitted`), exact-deadline (`AdmissionExpired`), and after-deadline (`AdmissionExpired`).
- **AC2 (Absent Session Wire):** Tokenless request with an unknown `Mcp-Session-Id` produces HTTP 401, body verbatim `"unauthorized\n"`, byte-exact `WWW-Authenticate` challenge, and a single diagnostic record asserting `failure_stage: "outer_oauth"`, `failure_reason: "session_absent"`, `session_header: "present"`, `session_state: "absent"`.
- **AC3 (Unauthenticated Session Wire):** Tokenless request with a session ID created without OAuth claims produces HTTP 401, body verbatim `"unauthorized\n"`, byte-exact `WWW-Authenticate` challenge, and a single diagnostic record asserting `failure_stage: "outer_oauth"`, `failure_reason: "session_fingerprint_mismatch"`, `session_header: "present"`, `session_state: "existing"`.
- **AC4 (Expired Session Wire):** Tokenless request with an expired session ID produces HTTP 401, body verbatim `"unauthorized\n"`, byte-exact `WWW-Authenticate` challenge, and a single diagnostic record asserting `failure_stage: "outer_oauth"`, `failure_reason: "jwt_expired"`, `session_header: "present"`, `session_state: "existing"`.
- **AC5 (Admitted Session Wire):** Tokenless request with a valid, authenticated session ID produces HTTP 200, valid JSON-RPC response, and a single diagnostic record asserting `session_header: "present"`, `session_state: "existing"`, `binding_relation: "match"`.
- **AC6 (Handshake Lifecycle Wire):** Statefully initialized handshake response asserting HTTP 200 emits exactly one completion record with `session_state: "created"`, `binding_relation: "not_evaluated"`, and `session_ref_state: "available"`. Stateless initialization emits `session_state: "not_supplied"` and `session_binding: "unavailable"`.
- **AC7 (Zero Leakage Across 401/403/404 Surfaces):** Wire assertions execute concrete requests producing at least one 401, at least one 403 (insufficient scope), and at least one 404 (unknown route). Assertions scan all rejected HTTP bodies, response headers, and correlated diagnostic completion JSON records, verifying 0 occurrences of raw fixture tokens, raw session IDs, fixture private keys, and their SHA-256 hashes.
- **AC8 (Full Suite & Coverage Gate):** Full workspace test suite passes cleanly under `go test -race ./...` and aggregate line coverage generated by `go test -race -coverprofile=coverage.out -coverpkg=./... ./...` meets `cmd/covcheck` (`>= 90.0%`).
<!-- specdd:design:end -->

<!-- specdd:tasks:start -->
## 4. TASKS & SEQUENCING

- **T1:** `issues/01-session-admission-result-taxonomy.md` (unblocked) — Implement `SessionAdmissionReason` and `SessionAdmissionOutcome` in `internal/auth/middleware.go` and `internal/service/inbound.go`. Settle boundary `!now.Before(deadline)` with before/at/after tests. Migrate callers in `cmd/mcp-bridge/serve.go` and `internal/auth/middleware_test.go`. [AC1]
- **T2:** `issues/02-middleware-differentiated-rejection-diagnostics.md` (blocked by T1) — Implement differentiated outer OAuth rejection diagnostics, byte-exact 401 challenges, and robust boolean environment parsing. RED test in `middleware_test.go` before fix, GREEN after. [AC2, AC3, AC4, AC7]
- **T3:** `issues/03-initialize-handshake-lifecycle-diagnostics.md` (blocked by T1) — Ensure truthful `SessionState: "created"`, `BindingRelation: "not_evaluated"`, and `refEvidenceState` handling on handshake completion across stateful and stateless modes (`session_binding: "unavailable"`) within the single terminal record. Characterization tests verify existing baseline. [AC6]
- **T4:** `issues/04-end-to-end-discovery-wire-verification-and-coverage.md` (blocked by T2, T3) — End-to-end loopback wire tests in `cmd/mcp-bridge/serve_oauth_test.go` asserting all admission branches, deterministic clock installation before harness start, `serveClientToken` static session fixture, challenge headers, zero leakage assertions across exercised 401/403/404 surfaces, and `>= 90.0%` coverage gate. [AC1–AC8]
<!-- specdd:tasks:end -->
<!-- specdd:envelope:start -->
### Allowed Envelope
- `internal/auth/middleware.go`
- `internal/auth/middleware_test.go`
- `internal/service/inbound.go`
- `internal/service/inbound_test.go`
- `cmd/mcp-bridge/serve.go`
- `cmd/mcp-bridge/serve_test.go`
- `cmd/mcp-bridge/serve_oauth_test.go`
- `cmd/mcp-bridge/serve_oauth_diagnostics_test.go`
- `docs/runbooks/chatgpt-integration.md`
- `coverage.out`
<!-- specdd:envelope:end -->

## 5. Non-Goals
- Changing Pocket-ID token issuance or dynamic client registration.
- Introducing unauthenticated tool execution without an initial valid OAuth handshake.
- Modifying the fixed 64-slot schema or creating dynamic untyped slots.
- Modifying Caddy reverse proxy configurations or network routing.

## 6. Rollback Strategy
1. The flag `MCP_SESSION_ADMISSION_ENABLED` is evaluated at process startup in `cmd/mcp-bridge/serve.go:192–193`. If diagnostic or admission regressions occur, setting `MCP_SESSION_ADMISSION_ENABLED=false` and restarting the container immediately reverts the bridge to strict per-request Bearer token validation without code changes or binary recompilation.
2. In the Go source layer, `SessionClaims(sessionID string) (*auth.Claims, bool)` is preserved on `*InboundHandler`, allowing the middleware hook to revert to binary evaluation if needed.
3. Rollback never weakens authentication: unauthenticated or unknown sessions always fail closed with HTTP 401 Unauthorized.

## 7. Observability Contract

### 7.1 Architecture & Operational Contract Inheritance
Logging and diagnostics inherit the operational contracts established in `internal/observe/` (`httpdiagnostic_types.go`, `httpdiagnostic_encode.go`, `log.go`):
- All stdout diagnostic records follow the JSON log envelope: `event: "http_complete"`.
- Severity level is selected dynamically per `internal/observe/httpdiagnostics.go:792–825`: `level: "error"` for HTTP 5xx or server delivery faults; `level: "warn"` for HTTP 4xx, client/admission rejections, or upstream tool errors with HTTP 200; and `level: "info"` for clean HTTP 2xx/3xx completions.
- Requests allocate a fixed-slot byte buffer formatted strictly into 64 typed fields.

### 7.2 Delivery & Binding to Acceptance Criteria and Tasks
- **Handshake Evidence (`AC6`, `T3`):** Handshake creation evidence (`session_state: "created"`, `binding_relation: "not_evaluated"`, `session_ref_state: "available"` in stateful mode; `session_state: "not_supplied"`, `session_binding: "unavailable"` in stateless mode) is bound to T3 and asserted in loopback wire verification.
- **Rejection Evidence (`AC2`, `AC3`, `AC4`, `AC7`, `T2`, `T4`):** Differentiated failure reasons (`session_absent`, `session_fingerprint_mismatch`, `jwt_expired`) are written at the `outer_oauth` rejection boundary and verified via loopback wire tests in T4.
- **Admission Evidence (`AC5`, `T4`):** Clean session admission (`session_state: "existing"`, `binding_relation: "match"`) is verified on tokenless discovery bursts in T4.

### 7.3 Single Terminal Record & Delivery Semantics
- **Single Completion Attempt:** All diagnostic contributions write into the request-local `HTTPDecision` context owned by the `diagnostics.Claim` lease (`internal/service/inbound.go:349–355`). Exactly one terminal `http_complete` record is finalized and attempted for write per completed request, preventing duplicate log events during multi-layer returns, panic unwinding, or early 401 rejections.
- **Write Failure Semantics:** In accordance with `internal/observe/log.go:73–75`, writes are performed synchronously to stdout under a mutex; write errors are ignored without buffering or retry. The contract guarantees one write attempt per request, not buffered downstream delivery.

### 7.4 Fixed Schema Budget, Retention & Labels
- **Schema & Size Budget:** Emitted records strictly reuse existing schema slots in `internal/observe/httpdiagnostic_encode.go` (slot 33 `failure_reason`, slot 37 `session_header`, slot 38 `session_state`, slot 39 `binding_relation`, slot 40 `session_ref_state`). Maximum physical record length is capped at 4096 bytes (`httpRecordBudget = 4096`). Non-HTTP records or records violating the schema fail safely without emitting partial or malformed JSON bytes.
- **Retention Policy:** Container-level stdout logs are retained by the Docker daemon JSON-file driver under rotation (`max-size: 10m`, `max-file: 3`). Ingested Loki logs are retained for 30 days under the cluster retention policy.
- **Bounded Label Contract:** The Loki stream index is strictly bounded to the 5 static low-cardinality labels configured in the vector logging agent: `{env, host, job="vector-logs", service_name="vector-logs", source_subsystem="mcp-bridge"}`. All variable attributes (`request_id`, `session_ref`, `client_ip`, `failure_reason`) reside strictly in the JSON payload body.

### 7.5 Justified Operational Exceptions
- **External Log Delivery (Vector/Loki):** In-cluster log ingestion by Vector and indexing in Loki are operationalized post-deployment; CI tests verify physical stdout emission directly (`serve_oauth_test.go`). *Reason:* CI test runners do not host Vector or Loki daemons. *Consequence:* Wire tests assert exact JSON stdout records, guaranteeing schema validity before shipping. If Vector is unavailable in production, logs are retained in Docker log files without blocking bridge operations.
- **Metrics & Scrape Verification:** Existing Prometheus metric `bridge_requests_total` (`internal/observe/metrics.go:137–140`) remains unchanged. Metric registration and emission are verified via unit tests in `internal/observe/metrics_test.go`. Full Prometheus scrape loop verification is an infrastructure concern. *Reason:* Metric schema and scrapers are already operational; no new metrics are added. *Consequence:* Scrape intervals and metric schemas remain backwards-compatible.
- **Health & Readiness Endpoints:** Health endpoints (`/livez`, `/readyz`, `/healthz`) are routed through the composed `inboundMux` (`cmd/mcp-bridge/serve.go:248`), emitting completion records with `route_outcome: "health"` or `"bypass"`. *Reason:* Uniform request accounting. *Consequence:* Probes generate diagnostic records but do not allocate session state or trigger OAuth evaluations.
- **Distributed Tracing:** Distributed OpenTelemetry tracing is excluded. *Reason:* `mcp-bridge` is a single-binary Go service using synchronous in-process pipelines. *Consequence:* Request correlation relies entirely on server-generated `request_id` and diagnostic session pseudonyms (`session_ref`).
- **Freeform Error Strings:** Freeform error messages are strictly excluded from HTTP diagnostic slots. *Reason:* Arbitrary error text causes high-cardinality label explosion in metrics and risks credential leakage. *Consequence:* All errors are mapped into finite closed enumerations (`httpFailureReasonValues`).
- **Alerting & Dashboards:** Grafana dashboards and Prometheus alert configurations are maintained out-of-band in infrastructure repositories. *Reason:* Repository boundary covers bridge source and standalone wire tests. *Consequence:* Verification establishes truthful telemetry generation in source code; alert tuning occurs post-rollout.

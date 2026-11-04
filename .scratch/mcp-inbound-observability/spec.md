# MCP Inbound Observability Enrichment

Status: ready-for-agent
Revision: r8

<!-- specdd:requirements:start -->
## 1. Requirements

### R1 — Unified Inbound Operation Classification & Metric Recording
The inbound request handler MUST classify incoming requests into a closed finite enum across Prometheus metrics, legacy HTTP evidence, and the typed `http_complete` diagnostic record:
- `"initialize"` -> `"initialize"`
- `"notifications/initialized"` -> `"initialized"`
- `"tools/list"` -> `"tools_list"`
- `"tools/call"` -> `"tools_call"`
- `"resources/list"` -> `"resources_list"`
- `"resources/read"` -> `"resources_read"`
- `"ping"` -> `"ping"`
- `"prompts/list"` -> `"prompts_list"`
- `"prompts/get"` -> `"prompts_get"`
- GET requests requesting an SSE stream (`Accept: text/event-stream`) -> `"sse_stream"`
- Unparseable, empty, or non-standard bodies -> `"request"`

Prometheus counter `bridge_requests_total{server="_bridge", operation="<operation>", outcome="<outcome>"}` MUST record the mapped operation for every routed request. The typed completion record MUST emit `operation: "<mapped_value>"` in its serialized JSON.

### R2 — Client Family Classification
The HTTP diagnostics owner MUST classify client `User-Agent` headers into a closed, low-cardinality `client_family` enum using strict priority substring matching:
1. Contains `"openai"` (case-insensitive) -> `"openai"`
2. Contains `"claude"` or `"anthropic"` (case-insensitive) -> `"claude"`
3. Contains `"cursor"` (case-insensitive) -> `"cursor"`
4. Contains `"omp/"` -> `"omp"`
5. Contains `"mozilla"`, `"chrome"`, or `"safari"` (case-insensitive) -> `"browser"`
6. Non-empty string -> `"other"`
7. Empty / absent -> `"absent"`

Raw User-Agent strings MUST NEVER be logged or reflected in diagnostic records.

### R3 — Exact HTTP Response Bytes
The terminal `http_complete` diagnostic record MUST include the exact integer count of bytes written (`http_bytes: <int64>`) alongside the coarse logarithmic bucket (`http_bytes_bucket: <string>`). For zero-write responses, `http_bytes` MUST be `0`. For partial or failed writes, `http_bytes` MUST record the exact byte count observed before the error.

### R4 — Exposed Tool Count Recording & Failure Omission
On successful `tools/list` requests, the tool surface exposure handler MUST record the integer count of exposed tools returned in the response page (`exposed_tools: <int>`) linked by request context and internal request ID. In the terminal `http_complete` record, `exposed_tools` MUST be emitted ONLY when `operation == "tools_list"` AND `httpOutcome == "success"`. On non-`tools/list` operations, unhandled list errors, or response delivery write failures, `exposed_tools` MUST be omitted. For a successful zero-count contribution (verified at the diagnostics boundary), `exposed_tools` MUST be emitted as integer `0`.

### R5 — End-to-End Loopback Wire Verification, Docs & Coverage Gate
Loopback HTTP wire tests in `cmd/mcp-bridge/serve_oauth_test.go` and compatibility metrics tests in `cmd/mcp-bridge/serve_logging_compatibility_test.go` MUST verify:
1. Real HTTP requests produce terminal diagnostic records with populated `operation`, `client_family`, `exposed_tools`, and exact `http_bytes`.
2. Prometheus `/metrics` scrapes reflect incremented `bridge_requests_total` counters partitioned by the new operation enum.
3. `README.md` and `CHANGELOG.md` are updated to document the enriched fields and metric repartitioning.
4. Full repository tests pass with race detector enabled (`go test -race -p 1 -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...`) and meet the aggregate coverage gate (`go run ./cmd/covcheck` >= 90.0%).

**Non-goals:** modifying reverse proxy configuration; emitting raw User-Agent headers; logging individual tool call prompt arguments; creating external distributed tracing spans.
<!-- specdd:requirements:end -->

<!-- specdd:acceptance:start -->
## 2. Acceptance Criteria

- **AC-1 (Inbound Method Classification):** In `inboundOperation`, JSON-RPC requests with methods `"initialize"`, `"notifications/initialized"`, `"tools/list"`, `"tools/call"`, `"resources/list"`, `"resources/read"`, `"ping"`, `"prompts/list"`, and `"prompts/get"` are classified to their respective snake_case operations. GET requests with `Accept: text/event-stream` are classified as `"sse_stream"`. Malformed or unknown methods return `"request"`.
- **AC-2 (Prometheus Counter Ingestion):** `RecordRequest(operation, outcome)` in `internal/observe/metrics.go` admits all new operations into `requestOperations` and increments `bridge_requests_total{server="_bridge", operation="<op>", outcome="<outcome>"}`. Compatibility test in `cmd/mcp-bridge/serve_logging_compatibility_test.go:TestServeLoggingCompatibility/Metrics` verifies scraped counter increments for `initialize`, `initialized`, and discrete prompt operations, expanding the delta assertion loop to include them.
- **AC-3 (Client Family Classification):** In `internal/observe/httpdiagnostics.go`, `client_family` is evaluated from `User-Agent` in strict priority order: `openai`, `claude`, `cursor`, `omp`, `browser`, `other`, `absent`. Overlapping User-Agents (e.g. `Mozilla/5.0 ... Claude/1.0`) resolve to the higher-precedence family (`claude`).
- **AC-4 (Exact Response Bytes Emission):** In terminal `http_complete` records, `http_bytes` is emitted as an exact integer matching `len(response_body)`. Partial writes report exact bytes written before error; zero-write responses report `0`.
- **AC-5 (Exposed Tools Count on tools/list):** On successful `tools/list` completion, `exposed_tools` records the exact integer count of exposed tools returned in the response page. Delivery failure (e.g. client disconnect / write error), list error (e.g. invalid cursor), or non-`tools/list` requests omit `exposed_tools`. A zero-tool contribution emits integer `0`. In `internal/observe/httpdiagnostics.go`, `ContributeExposedTools` protects mutation with an atomic `!owner.finished` guard under `ownerStateMu`.
- **AC-6 (End-to-End Loopback Wire Verification & Docs):** In `cmd/mcp-bridge/serve_oauth_test.go`, loopback HTTP wire requests verify:
  1. `POST /mcp` with `tools/list` produces a terminal diagnostic record with `operation: "tools_list"`, `exposed_tools: 3` (matching the 3 registered built-in tools), and `http_bytes` matching the exact response body byte count.
  2. `POST /mcp` with invalid cursor produces a JSON-RPC error response and omits `exposed_tools`.
  3. Requests with `User-Agent: openai-connectors/1.0` produce `client_family: "openai"`.
  4. Requests with `User-Agent: Mozilla/5.0 (Macintosh) Claude/1.0` produce `client_family: "claude"`.
  5. Non-`tools/list` requests omit `exposed_tools`.
  `README.md` documents `operation`, `client_family`, `http_bytes`, and `exposed_tools` in the transport fields section. `CHANGELOG.md` records an Unreleased entry for the new fields and metric repartitioning.
- **AC-7 (Repository Verification & Coverage Gate):** Full repository tests pass clean (`go test -race -p 1 -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...`) and aggregate source coverage meets or exceeds 90.0% (`go run ./cmd/covcheck`).
<!-- specdd:acceptance:end -->

<!-- specdd:design:start -->
## 3. Design

### 3.1 Inbound Operation Classifier & Prometheus Metrics (`internal/service/inbound.go`, `internal/observe/metrics.go`)
- In `internal/service/inbound.go:inboundOperation(r *http.Request, captured *bodyCapture) string`:
  - If `r.Method == http.MethodGet` and `strings.Contains(r.Header.Get("Accept"), "text/event-stream")`: return `"sse_stream"`.
  - Unmarshal captured request body to extract `"method"`.
  - Map `"initialize"` -> `"initialize"`, `"notifications/initialized"` -> `"initialized"`, `"tools/list"` -> `"tools_list"`, `"tools/call"` -> `"tools_call"`, `"resources/list"` -> `"resources_list"`, `"resources/read"` -> `"resources_read"` `"ping"` -> `"ping"`, `"prompts/list"` -> `"prompts_list"`, `"prompts/get"` -> `"prompts_get"`, fallback `"request"`.
- In `internal/observe/metrics.go`:
  - Expand `requestOperations` to admit: `"request"`, `"prompts_list"`, `"prompts_get"`, `"initialize"`, `"initialized"`, `"tools_list"`, `"tools_call"`, `"resources_list"`, `"resources_read"`, `"ping"`, `"sse_stream"`.
- In `cmd/mcp-bridge/serve_logging_compatibility_test.go`:
  - Update `compatibilityRequestCounters` allowlist to permit the expanded operations.
  - Update `wantDelta` to expect `initialize/success: 1` and `initialized/success: 1` instead of counting under `request/success`.
  - Expand the delta-check loop in `testCompatibilityMetrics` to iterate `[]string{"request", "prompts_list", "prompts_get", "initialize", "initialized"}`.

### 3.2 Typed Diagnostics & Slot Encoding (`internal/observe/`)
- In `internal/observe/httpdiagnostics.go`:
  - Expand `operationVocabulary` to include: `"initialized"`, `"resources_list"`, `"resources_read"`, `"ping"`, `"sse_stream"`.
  - Implement `classifyClientFamily(userAgent string) string` implementing the priority substring matches: `openai` -> `claude` (or `anthropic`) -> `cursor` -> `omp/` -> `browser` -> `other` -> `absent`.
  - In `Claim()`, evaluate `classifyClientFamily(r.UserAgent())` and pass through `httpOwnerInit{clientFamily: ...}`.
  - In `buildSnapshot()`:
    - Set `diagnostic.Operation = operation`.
    - Set `diagnostic.ClientFamily = o.clientFamily`.
    - Set `diagnostic.HTTPBytes = o.bytes`.
    - Only if `o.legacyOperation == "tools_list" && httpOutcome == "success" && o.exposedTools != nil`, set `diagnostic.ExposedTools = o.exposedTools`.
  - Implement `ContributeExposedTools(ctx context.Context, requestID string, count int)`:
    - Attempt lookup on context first (`ownerFromContext(ctx)`).
    - If unattached, look up by `ownerForRequestID(requestID)`.
    - Under `ownerStateMu.Lock()`, verify `!owner.finished`. If not finished, store `owner.exposedTools = &count`.
- In `internal/observe/httpwriter.go`:
  - Add `clientFamily string` and `exposedTools *int` to `httpOwner`.
  - Add `clientFamily string` to `httpOwnerInit`.
  - In `(*httpOwner).init(state httpOwnerInit)`, set `o.clientFamily = state.clientFamily`.
- In `internal/observe/httpdiagnostic_types.go`:
  - Add to `HTTPDiagnostic`:
    - `Operation string`
    - `ClientFamily string`
    - `HTTPBytes int64`
    - `ExposedTools *int`
  - Define `clientFamilyVocabulary = []string{"openai", "claude", "cursor", "omp", "browser", "other", "absent"}`.
- In `internal/observe/httpdiagnostic_encode.go`:
  - Add slots for `operation`, `client_family`, `http_bytes`, and optional `exposed_tools` to `slotKeys`.
  - Append new slots after existing core slots to preserve physical indexes 0–60 for existing tests (or update `session_slot_diag_test.go` index assertions accordingly).
  - Update `numHTTPSlots` and slot indexing accordingly.
  - In `httpEncoder.build()`, validate and fill the slots.
- In `internal/observe/log.go`:
  - Expand the legacy `finite(entry.Operation, ...)` vocabulary to match `operationVocabulary`.
- In `internal/observe/session_slot_diag_test.go`:
  - Update maximal fixture in `sessionMaximalFixture()` to populate valid `Operation: "request"` and `ClientFamily: "openai"`.
  - Verify that hard-coded slot assertions remain aligned.

### 3.3 Tool Exposure Recording (`internal/service/session_exposure.go`)
- In `receivingMiddleware(next mcp.MethodHandler) mcp.MethodHandler`:
  - When `isList` is true:
    - Call `result, err := t.ownerToolListPage(projection, cursor)`.
    - When `err == nil && result != nil`:
      - Extract internal request ID from `listReq.Extra.Header.Get("X-Mcp-Bridge-Internal-Request-Id")` if present.
      - Call `observe.ContributeExposedTools(ctx, requestID, len(result.Tools))`.
    - Return `result, err`.

### 3.4 Wire Verification Tests & Documentation (`cmd/mcp-bridge/serve_oauth_test.go`, `README.md`, `CHANGELOG.md`)
- Add integration test `TestInboundObservabilityWire` in `serve_oauth_test.go`:
  - Verify that `tools/list` HTTP request produces `http_complete` record containing `operation: "tools_list"`, `exposed_tools: 3`, and `http_bytes == len(respBody)`.
  - Verify that `tools/list` with invalid cursor produces a JSON-RPC error response and omits `exposed_tools`.
  - Verify that follow-up valid request has clean state without leaked tool counts.
  - Verify that requests with `User-Agent: openai-connectors/1.0` emit `client_family: "openai"`.
  - Verify that requests with `User-Agent: Mozilla/5.0 (Macintosh) Claude/1.0` emit `client_family: "claude"` (verifying priority over `browser`).
  - Verify that non-`tools/list` requests (e.g. `ping`) omit `exposed_tools`.
- Update `README.md`:
  - Document `operation`, `client_family`, `http_bytes`, and `exposed_tools` under transport fields.
  - Note the Prometheus metric repartitioning under `bridge_requests_total`.
- Update `CHANGELOG.md`:
  - Record entry under `[Unreleased]` documenting the new diagnostic fields and Prometheus operation repartitioning.
<!-- specdd:design:end -->

<!-- specdd:tasks:start -->
## 4. Task Ledger

### T1 — Inbound Method Operation Classification & Unified Prometheus Metrics
- **Assigned to:** engineer
- **Requires:** []
- **Effects:** write `internal/service/inbound.go`, `internal/service/inbound_test.go`, `internal/observe/metrics.go`, `cmd/mcp-bridge/serve_logging_compatibility_test.go`
- **Acceptance:** AC-1, AC-2
- **Verification:**
  - RED: `go test -v -count=1 -run TestInboundOperationClassification ./internal/service`
  - GREEN: `go test -v -count=1 -run "TestInboundOperationClassification|^TestServeLoggingCompatibility$$" ./internal/service ./cmd/mcp-bridge`
- **Receipt:** `.omp/artifacts/receipt_T1.json`

### T2 — Client Family Classification, Operation Slot & Exact Response Bytes
- **Assigned to:** engineer
- **Requires:** []
- **Effects:** write `internal/observe/httpdiagnostics.go`, `internal/observe/httpdiagnostic_types.go`, `internal/observe/httpdiagnostic_encode.go`, `internal/observe/httpwriter.go`, `internal/observe/log.go`, `internal/observe/httpdiagnostics_test.go`, `internal/observe/session_slot_diag_test.go`
- **Acceptance:** AC-3, AC-4, AC-5
- **Verification:**
  - RED: `go test -v -count=1 -run TestClientFamilyAndExactBytes ./internal/observe`
  - GREEN: `go test -v -count=1 -run "TestClientFamilyAndExactBytes|TestChargedBytesEqualEncodedRecord|TestSessionReferenceOmission" ./internal/observe`
  - Specific boundary tests:
    - Zero-byte write: asserts `http_bytes: 0`.
    - Partial write failure: asserts exact bytes written before error, `http_outcome: "incomplete"`, and `exposed_tools` omitted.
    - Zero-tool emission: asserts `exposed_tools: 0` when count 0 is contributed.
    - Late contribution guard: asserts contribution after `owner.finish()` is rejected and does not mutate the terminal record.
    - Priority overlap: asserts `Mozilla/5.0 ... Claude/1.0` maps to `claude`.
    - Non-list request: asserts `exposed_tools` is omitted.
- **Receipt:** `.omp/artifacts/receipt_T2.json`

### T3 — Exposed Tool Count & Wire Observability Verification
- **Assigned to:** engineer
- **Requires:** [T1, T2]
- **Effects:** write `internal/service/session_exposure.go`, `cmd/mcp-bridge/serve_oauth_test.go`, `README.md`, `CHANGELOG.md`, `coverage.out`
- **Acceptance:** AC-5, AC-6, AC-7
- **Verification:**
  - RED: `go test -v -count=1 -run TestInboundObservabilityWire ./cmd/mcp-bridge`
  - GREEN: `go test -v -count=1 -run TestInboundObservabilityWire ./cmd/mcp-bridge && go test -race -p 1 -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./... && go run ./cmd/covcheck`
  - Specific wire tests:
    - `POST /mcp` `tools/list`: asserts `operation: "tools_list"`, `exposed_tools: 3`, and `http_bytes == len(respBody)`.
    - `POST /mcp` `tools/list` with invalid cursor: asserts protocol error returned and `exposed_tools` is omitted.
    - Subsequent clean request verifies no leaked state.
    - `POST /mcp` `ping`: asserts `operation: "ping"` and `exposed_tools` is omitted.
    - Client families: `openai-connectors/1.0` -> `openai`, `Mozilla/5.0 ... Claude/1.0` -> `claude`.
- **Receipt:** `.omp/artifacts/receipt_T3.json`
<!-- specdd:tasks:end -->

<!-- specdd:envelope:start -->
## 5. Envelope & Governance

- **Allowed Files:**
  - `internal/service/inbound.go`
  - `internal/service/inbound_test.go`
  - `internal/service/session_exposure.go`
  - `internal/observe/metrics.go`
  - `internal/observe/httpdiagnostics.go`
  - `internal/observe/httpdiagnostic_types.go`
  - `internal/observe/httpdiagnostic_encode.go`
  - `internal/observe/httpwriter.go`
  - `internal/observe/log.go`
  - `internal/observe/httpdiagnostics_test.go`
  - `internal/observe/session_slot_diag_test.go`
  - `cmd/mcp-bridge/serve_logging_compatibility_test.go`
  - `cmd/mcp-bridge/serve_oauth_test.go`
  - `README.md`
  - `CHANGELOG.md`
  - `.scratch/mcp-inbound-observability/**`
  - `.omp/artifacts/receipt_T1.json`
  - `.omp/artifacts/receipt_T2.json`
  - `.omp/artifacts/receipt_T3.json`
  - `coverage.out`
- **Forbidden Files:**
  - `internal/catalog/**`
  - `internal/search/**`
  - `configs/**`
  - `internal/observe/httpwriter_facades_generated.go`
  - Reverse proxy configurations (`Caddyfile`)
- **Rollback Plan:**
  - Target and revert only the reviewed feature commit SHA(s) on `main` using `git revert --no-edit <SHA>`, preserving user changes and unrelated branch state.
<!-- specdd:envelope:end -->

## 6. Release & Rollback Considerations

- **Deployment Mechanism:** Local single-binary replacement in the `mcp-bridge` container.
- **Rollback Invariant:** If rolled back via `git revert`, previous metric scraping contracts and fixed-slot diagnostic formats resume without schema or state corruption.
- **Availability Expectation:** Standard container restart upon deployment; brief connection interruption during container process restart is expected and standard for single-replica homelab deployments. Zero downtime is not guaranteed; rolling replacement requires external multi-instance ingress which is out of scope.

<!-- specdd:observability:start -->
## 7. Observability Contract

### 7.1 Metrics Scraper & Label Contract
- Existing Prometheus metric `bridge_requests_total{server="_bridge", operation="<operation>", outcome="<outcome>"}` is exposed on the operations listener (`:9090/metrics`).
- Bound to AC-2 and verified in T1 via `cmd/mcp-bridge/serve_logging_compatibility_test.go:TestServeLoggingCompatibility/Metrics`.
- The `operation` label values are strictly bounded to the finite set: `request`, `initialize`, `initialized`, `tools_list`, `tools_call`, `resources_list`, `resources_read`, `ping`, `prompts_list`, `prompts_get`, `sse_stream`.
- Unrecognized operations are refused by `RecordRequest` and never create dynamic label cardinality.

### 7.2 Structured Completion Logs & Query Interface
- Emitted to stdout as a single JSON object per completed HTTP request (`event: "http_complete"`).
- Ingested by Vector from container stdout and indexed in Loki. Queryable in Grafana using:
  `{app="mcp-bridge"} |= "http_complete" | json | client_family="openai" | operation="tools_list"`.
- Automated test verification exercises the bridge boundary (capturing stdout in test harnesses).
- Bound to AC-3, AC-4, AC-5, AC-6 and verified in T2/T3 via `internal/observe/httpdiagnostics_test.go` and `cmd/mcp-bridge/serve_oauth_test.go:TestInboundObservabilityWire`.

### 7.3 Privacy, Cardinality & Ingestion Failure Guarantees
- Raw User-Agent strings, client IPs, authentication tokens, and RPC payload bodies are NEVER emitted in metric labels or log records.
- Total byte size of the typed `http_complete` record remains strictly within `httpRecordBudget = 4096` bytes. If budgeting limits are ever exceeded, optional fields are deterministically dropped using `dropForBudget()` rather than crashing or emitting partial bytes.
- Delivery failures on response writes (e.g. broken client pipe) record `http_outcome: "incomplete"` and omit `exposed_tools`.
- Log shipping failure & backpressure policy: The bridge writes synchronously to stdout via `l.output.Write`; short-write or sink errors are nonfatal (ignored without retrying). Downstream collector buffering (Vector) handles transport buffering outside the bridge process; if stdout blocks, backpressure can delay request finalization.
- Severity mapping inherits `severityLevel()` in `httpdiagnostics.go`: 2xx requests emit `info`, tool errors or unobserved status emit `warn`, and 5xx / delivery crashes emit `error`.
- Retention policy inherits the homelab Loki deployment default (30 days), requiring no bridge-level mutation.

### 7.4 Traceability & Justified Exceptions
- `request_id` correlation links the inbound edge, auth middleware, and downstream MCP operations.
- Health endpoints (`/livez`, `/healthz`, `/readyz`) remain bypassed from request counting.
- Justified Operational Exceptions:
  1. **Loki Retention & Delivery:** Deployed Loki delivery and 30-day retention are external homelab infrastructure guarantees. Automated CI/CD verifies stdout capture at the binary boundary.
  2. **Dashboard & Alert Migration:** Existing external homelab Grafana dashboards and Prometheus alerts reside in external infrastructure outside the bridge repository. In-repo verification is bounded to AC-2 (Prometheus `/metrics` scrape assertions in `serve_logging_compatibility_test.go:TestServeLoggingCompatibility/Metrics`). Consequence of shipping: existing alert rules or dashboard panels that filter strictly on `operation="request"` will see reduced counts as traffic is partitioned into `initialize`, `tools_list`, etc., while queries aggregating over all operations (e.g. `sum(rate(bridge_requests_total[5m]))`) remain unaffected. Post-deployment dashboard migration is assigned to operator discretion.
<!-- specdd:observability:end -->

## 8. Evidence & Uncertainty

- **Evidence Anchors:**
  - `internal/observe/metrics.go:42,138-140,183-185`: Confirms metric name `bridge_requests_total` and `requestOperations` allowlist.
  - `cmd/mcp-bridge/serve_logging_compatibility_test.go:239,289`: Confirms compatibility test assertion on counter deltas.
  - `internal/observe/httpdiagnostics.go:373-377,612-616`: Confirms `operationVocabulary` and `!owner.finished` guard under `ownerStateMu`.
  - `internal/observe/httpwriter.go:94`: Confirms owner initialization via `httpOwnerInit`.
  - `internal/service/session_exposure.go:47-54,224`: Confirms `ownerToolListPage` return type and exposure middleware seam.
- **Assumptions:** Downstream log collector (Vector) continues to parse stdout JSON streams without custom delimiters.

## 9. Reviewer Handoff

- **Target Reviewers:** `critic`, `code-reviewer`
- **Key Decision Points:**
  - Bounded operation enum across Prometheus metrics, legacy evidence, and typed HTTP completion logs.
  - Case-insensitive specific-first User-Agent family classification (`openai` > `claude` > `cursor` > `omp` > `browser` > `other` > `absent`).
  - Strict omission of `exposed_tools` on write failures, list errors (invalid cursor), or non-`tools/list` operations.
  - Atomic finalization guard (`!owner.finished` under `ownerStateMu`) on exposed tool contributions.
  - Slot preservation in `httpdiagnostic_encode.go` and fixture migration in `session_slot_diag_test.go`.
  - Documentation updates in `README.md` and `CHANGELOG.md` for enriched transport fields and metric repartitioning.

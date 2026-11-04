# Homelab LLM Ergonomics & Token Optimization

Status: ready-for-agent

<!-- specdd:requirements:start -->
## 1. Requirements

### R1 — Optional catalog_revision on describe_tool and call_tool
1. The built-in tool input schemas `DescribeToolInputSchema()` and `CallToolInputSchema()` in `internal/catalog/schema.go` MUST remove `"catalog_revision"` from their `"required"` parameter arrays. `tool_name` remains required on both; `arguments` remains required on `call_tool`.
2. The schema pattern for `"catalog_revision"` MUST permit empty string or exact 64-character lowercase hex (`^$|^[a-f0-9]{64}$`).
3. When an invocation of `describe_tool` or `call_tool` omits `catalog_revision` or provides an empty string, the handler in `internal/service/tools.go` MUST implicitly bind the request to the currently published catalog revision (`assembly.CatalogRevision()`) and execute normally.
4. When `catalog_revision` is provided and non-empty:
   - If the string does not match the valid 64-character lowercase hex format (`len == 64` and all characters in `[0-9a-f]`), it MUST be rejected at handler admission with `CodeInvalidArguments` (`invalid_arguments`), `isError: true`, and zero downstream dispatch.
   - If the string is a valid 64-hex string but does not match `assembly.CatalogRevision()`, it MUST be rejected with `codeCatalogChanged` (`catalog_changed`), `isError: true`, and zero downstream dispatch.

### R2 — Semantic Wire Name (<server>__<tool>) Publication with Deterministic Fallback
1. Tools published in `search_tools` results (`internal/service/tools.go:searchPayload`) MUST be named using their canonical semantic wire name `config.WireToolName(server, rawName)` (`<server>__<tool>`) whenever `len(server + "__" + rawName) <= 64` and the name contains only valid identifier characters (`^[a-z][a-z0-9-]{0,31}__[a-zA-Z0-9_-]{1,94}$`).
2. If the wire name exceeds 64 characters or contains invalid identifier characters, the tool name MUST fall back deterministically to `core.PublicName(server, rawName)` (`t_<base32>` hash) without mutating logging infrastructure or emitting redundant per-request diagnostic noise.
3. Dynamically revealed tools committed to a session's projection (`internal/service/projection.go:preparePinProjection`) MUST register their projected tool name as `config.WireToolName(interest.serverID, interest.rawName)` whenever within bounds, falling back deterministically to `core.PublicName` if exceeding bounds.
4. Dual lookup resolution in `internal/service/tools.go:resolveBinding` MUST be preserved so that calls referencing either the wire name or the legacy `t_<base32>` hash continue to resolve to the same underlying catalog binding.

### R3 — Pruned search_tools and describe_tool Payloads
1. In `internal/service/tools.go:searchPayload`:
   - The root `backends: []` cluster health array MUST be completely removed from the serialized response.
   - Per-result items MUST remove `last_success_at`, `catalog_status`, and redundant per-item `catalog_revision`.
   - Per-result items MUST retain `tool_name`, `server`, `upstream_name`, `description`, `score`, `pinned`, and `availability` (`"available"` | `"unavailable"`).
   - The root object MUST retain `catalog_revision`, `mode`, and `degraded_reasons` once.
2. In `internal/service/tools.go:describeTool`:
   - The response payload MUST remove `last_success_at` and `catalog_status`.
   - The response payload MUST retain `server`, `catalog_revision`, `tool_name`, `upstream_name`, `description`, `pinned`, `availability`, `validation_status`, and `input_schema` (preserving `output_schema`, `metadata`, and `metadata_override_fields` when present).

### R4 — Streamlined Generic call_tool Response Envelope
1. In `internal/service/tools.go:callTool`, the generic execution response payload MUST remove `catalog_revision`, `catalog_status`, and `availability`.
2. The outer payload MUST retain `server`, `tool_name`, `request_id`, and `upstream_result` to preserve wire compatibility for clients expecting the `upstream_result` envelope.

### R5 — End-to-End Wire Verification, Documentation, and Coverage Gate
1. Loopback HTTP wire tests in `cmd/mcp-bridge` MUST verify:
   - `describe_tool` and `call_tool` succeed over live HTTP wire when `catalog_revision` is omitted or empty.
   - `describe_tool` and `call_tool` fail with `catalog_changed` when a mismatched valid 64-hex revision is supplied.
   - `describe_tool` and `call_tool` fail with `invalid_arguments` when a malformed revision (e.g. non-hex string) is supplied.
   - `search_tools` over live HTTP returns semantic wire names for matching tools, strips `backends: []`, and includes `availability`.
   - `tools/list` reflects semantic wire names for newly revealed dynamic tools following a successful search.
   - `http_complete` and `mcp_complete` diagnostic log entries correlate via identical `request_id` for both `describe_tool` and `call_tool`, and HTTP completion records report `upstream_state: "not_attempted"` for `describe_tool`.
2. `README.md` MUST be updated to accurately document optional `catalog_revision`, semantic wire naming with 64-char fallback, and the streamlined discovery and call payloads.
3. The repository test suite MUST pass under `go test -race ./...`, `go vet ./...`, and `go run ./cmd/covcheck` MUST confirm aggregate source coverage ≥ 90.0%.

**Non-goals:** Inlining full multi-tool input schemas into `search_tools` (held off to prevent prompt blowup); removing `Content` from tool results (held off to prevent blinding standard MCP clients); modifying upstream server implementations; modifying the closed `internal/observe` logger vocabulary.
<!-- specdd:requirements:end -->

<!-- specdd:acceptance:start -->
## 2. Acceptance Criteria

- **AC-1 (Schema Optionality for catalog_revision):** In `internal/catalog/schema.go`:
  - `DescribeToolInputSchema()["required"]` equals `[]any{"tool_name"}`.
  - `CallToolInputSchema()["required"]` equals `[]any{"tool_name", "arguments"}`.
  - `"catalog_revision"` property schema defines pattern `^$|^[a-f0-9]{64}$` to admit empty string and valid 64-hex string.
- **AC-2 (Service Execution with Omitted catalog_revision & Correlation):** In `internal/service/tools.go` and `internal/service/direct_execution.go`:
  - A `describe_tool` request with omitted or empty `catalog_revision` succeeds (`isError: false`) and returns the tool description with the current catalog revision, emitting `mcp_complete` with `operation: "describe_tool"`, `outcome: "success"`, `reason: "none"`, and matching `request_id` extracted from `req.Extra.Header["X-Mcp-Bridge-Internal-Request-Id"]`. It contributes upstream evidence `upstream_state: "not_attempted"`.
  - A `call_tool` request with omitted or empty `catalog_revision` succeeds (`isError: false`) and dispatches upstream, emitting `mcp_complete` with `operation: "call_tool"`, `outcome: "success"`, `reason: "none"`, and matching `request_id`.
  - Supplying a mismatched 64-hex `catalog_revision` to `describe_tool` or `call_tool` returns `isError: true` and `code: "catalog_changed"` with zero upstream dispatch, emitting exactly one `mcp_complete` diagnostic record with `stage: "tool_admission"`, `outcome: "error"`, `reason: "catalog_changed"`, matching `request_id`, and contributes `upstream_state: "not_attempted"`.
  - Supplying a malformed `catalog_revision` (non-empty and not 64-hex) to `describe_tool` or `call_tool` returns `isError: true` and `code: "invalid_arguments"` with zero upstream dispatch, emitting exactly one `mcp_complete` diagnostic record with `stage: "tool_admission"`, `outcome: "error"`, `reason: "invalid_arguments"`, matching `request_id`, and contributes `upstream_state: "not_attempted"`.
- **AC-3 (Semantic Wire Name Publication & Bounds):**
  - In `searchPayload`, unpinned tools whose wire name `server__upstream_name` has length $\le 64$ characters and valid identifier characters report `tool_name: "<server>__<upstream_name>"`.
  - Tools whose wire name exceeds 64 characters or contains invalid characters fall back deterministically to `t_<base32>` hash format.
  - In `preparePinProjection`, discovered unpinned tools committed to a session's projection are named `<server>__<upstream_name>` when within bounds, or `t_<base32>` when exceeding bounds.
  - Dual lookup in `resolveBinding` resolves both `<server>__<upstream_name>` and `t_<base32>` to the same binding.
- **AC-4 (Pruned search_tools Payload):**
  - In `searchPayload`, the returned map contains `"catalog_revision"`, `"mode"`, `"degraded_reasons"`, and `"results"`. The key `"backends"` is absent (`_, exists := payload["backends"]` is `false`).
  - Each item in `"results"` contains `"tool_name"`, `"server"`, `"upstream_name"`, `"description"`, `"score"`, `"pinned"`, and `"availability"`.
  - Keys `"last_success_at"`, `"catalog_status"`, and per-item `"catalog_revision"` are absent from each item in `"results"`.
- **AC-5 (Pruned describe_tool Payload):**
  - The map returned by `describeTool` does not contain `"last_success_at"` or `"catalog_status"`.
  - It contains `"server"`, `"catalog_revision"`, `"tool_name"`, `"upstream_name"`, `"description"`, `"pinned"`, `"availability"`, `"validation_status"`, and `"input_schema"`.
  - Preserves `"output_schema"`, `"metadata"`, and `"metadata_override_fields"` when present in the underlying binding or pin metadata.
- **AC-6 (Streamlined call_tool Envelope):**
  - The structured payload returned by generic `callTool` does not contain `"catalog_revision"`, `"catalog_status"`, or `"availability"`.
  - It contains `"server"`, `"tool_name"`, `"request_id"`, and `"upstream_result"`.
- **AC-7 (Wire Verification & Documentation):**
  - Wire tests in `cmd/mcp-bridge` assert:
    - Invoking `describe_tool` and `call_tool` over live HTTP with omitted or empty `catalog_revision` returns HTTP 200 with JSON-RPC result (`isError == false`).
    - Invoking `describe_tool` and `call_tool` with mismatched valid 64-hex revision returns `isError: true` and `code: "catalog_changed"`.
    - Invoking `describe_tool` and `call_tool` with malformed revision returns `isError: true` and `code: "invalid_arguments"`.
    - Invoking `search_tools` over live HTTP returns results containing wire names, includes `"availability"`, and omits `"backends"`.
    - Invoking `tools/list` after dynamic discovery lists the unpinned tool under its `<server>__<tool>` wire name.
    - Wire completion diagnostic logs emit `http_complete` and `mcp_complete` with matching `request_id` and correct admission reasons (`catalog_changed` on stale revision, `invalid_arguments` on malformed revision, `none` on success), and `http_complete` reports `upstream_state: "not_attempted"` for `describe_tool`.
  - `README.md` documents optional `catalog_revision`, semantic wire naming, and pruned payloads.
- **AC-8 (Regression Suite & Coverage Gate):**
  - Full race test suite `go test -race ./...` passes with exit code 0.
  - `go vet ./...` passes with exit code 0.
  - Coverage check `go run ./cmd/covcheck` verifies aggregate source-line coverage ≥ 90.0% against fresh `coverage.out`.
<!-- specdd:acceptance:end -->

<!-- specdd:design:start -->
## 3. Design

### 3.1 Catalog Schemas (`internal/catalog/schema.go`)
- In `internal/catalog/schema.go`:
  - Update `CatalogRevisionPattern = "^$|^[a-f0-9]{64}$"`.
  - In `DescribeToolInputSchema()`:
    - Update `"required"` from `[]any{"tool_name", "catalog_revision"}` to `[]any{"tool_name"}`.
  - In `CallToolInputSchema()`:
    - Update `"required"` from `[]any{"tool_name", "catalog_revision", "arguments"}` to `[]any{"tool_name", "arguments"}`.
  - Ensure `"catalog_revision"` property schema retains pattern `CatalogRevisionPattern`.

### 3.2 Service Layer Invocations & Correlation (`internal/service/tools.go`, `internal/service/direct_execution.go`)
- In `isValidCatalogRevision(rev string) bool`:
  - `if len(rev) != 64 { return false }`
  - `return validHex(rev)`
- Standardize admission logging helper:
  `func (t *ToolSurface) logAdmissionFailure(ctx context.Context, requestID, operationID, corrState string, start time.Time, operation string, reason string)`
  - Parameterize `Operation: operation` (`"describe_tool"` or `"call_tool"`).
  - Emits `mcp_complete` with `stage: "tool_admission"`, `outcome: "error"`, `reason: reason`, and `mcp_method: "tools_call"`.
  - Links operation via `t.contributeOperation(requestID, operationID, "not_attempted")`.
- Update `internal/service/direct_execution.go:34,39`:
  - Call `e.surface.logAdmissionFailure(ctx, e.requestID, e.operationID, e.correlation, e.requestStart, "call_tool", "invalid_arguments")`.
- In `describeTool(ctx context.Context, req *mcp.CallToolRequest)`:
  - Extract server-owned request identity and correlation state from carrier header, matching `callTool:644-652`:
    ```go
    var requestID string
    corrState := "unavailable"
    if req != nil && req.Extra != nil && req.Extra.Header != nil {
        carrier := req.Extra.Header.Get("X-Mcp-Bridge-Internal-Request-Id")
        if len(carrier) == 32 && validHex(carrier) {
            requestID = carrier
            corrState = "valid"
        }
    }
    operationID := observe.CorrelationID("")
    start := time.Now()
    ```
  - If `wire.CatalogRevision == ""`:
    - `wire.CatalogRevision = assembly.CatalogRevision()`
  - Else:
    - If `!isValidCatalogRevision(wire.CatalogRevision)`:
      - `t.logAdmissionFailure(ctx, requestID, operationID, corrState, start, "describe_tool", "invalid_arguments")`
      - return `errorResult(search.CodeInvalidArguments, "catalog_revision is not a valid 64-character hexadecimal digest", core.ExecutionNotStarted), nil`
    - If `wire.CatalogRevision != assembly.CatalogRevision()`:
      - `t.logAdmissionFailure(ctx, requestID, operationID, corrState, start, "describe_tool", "catalog_changed")`
      - return `errorResult(codeCatalogChanged, "the catalog revision changed", core.ExecutionNotStarted), nil`
  - On success:
    - Contribute truthful upstream state for local describe: `t.contributeOperation(requestID, operationID, "not_attempted")`
    - If `t.logger != nil`: emit `mcp_complete` with `request_id: requestID`, `operation_id: operationID`, `operation: "describe_tool"`, `outcome: "success"`, `reason: "none"`, and `correlation_state: corrState`.
  - In `payload` map construction: remove `"catalog_status"` and `"last_success_at"`. Keep `"availability": health.Availability`.
- In `callTool(ctx context.Context, req *mcp.CallToolRequest)`:
  - Extract `requestID`, `operationID`, and `corrState` using existing carrier logic (`lines 644-654`).
  - If `wire.CatalogRevision == ""`:
    - `wire.CatalogRevision = assembly.CatalogRevision()`
  - Else:
    - If `!isValidCatalogRevision(wire.CatalogRevision)`:
      - `t.logAdmissionFailure(ctx, requestID, operationID, corrState, start, "call_tool", "invalid_arguments")`
      - return `errorResult(search.CodeInvalidArguments, "catalog_revision is not a valid 64-character hexadecimal digest", core.ExecutionNotStarted), nil`
    - If `wire.CatalogRevision != assembly.CatalogRevision()`:
      - `t.logAdmissionFailure(ctx, requestID, operationID, corrState, start, "call_tool", "catalog_changed")`
      - return `errorResult(codeCatalogChanged, "the catalog revision changed", core.ExecutionNotStarted), nil`
  - In outer `payload` map construction: remove `"catalog_revision"`, `"catalog_status"`, and `"availability"`. Keep `"server"`, `"tool_name"`, `"request_id"`, and `"upstream_result"`.

### 3.3 Semantic Wire Name Publication (`internal/service/tools.go`, `internal/service/projection.go`)
- Helper function `effectiveWireName(serverID, rawName string) string`:
  - `wire := config.WireToolName(serverID, rawName)`
  - If `len(wire) <= 64` and valid identifier characters (letters, numbers, hyphens, underscores): return `wire`.
  - Otherwise, return `core.PublicName(serverID, rawName)`.
- In `searchPayload(response search.Response)`:
  - Iterate `response.Results`. For each result:
    - `toolName := effectiveWireName(result.Server, result.UpstreamName)`
    - Keep `availability: result.Availability`.
- In `preparePinProjection` (`internal/service/projection.go`):
  - In the unpinned dynamic interest loop:
    - `specs = append(specs, projectionToolSpec{ name: effectiveWireName(interest.serverID, interest.rawName), binding: binding })`.

### 3.4 Regression & Wire Test Updates
- In `internal/catalog/catalog_test.go`:
  - Assert that `catalog_revision` is absent from `required`.
  - Assert that `""` matches `CatalogRevisionPattern` and invalid strings fail.
- In `internal/service/tools_test.go`:
  - Verify omitted, empty string, valid matching, valid stale, and malformed `catalog_revision` for both `describe_tool` and `call_tool`.
  - Verify `mcp_complete` diagnostic logging with reason `"invalid_arguments"` on malformed revision and `"catalog_changed"` on stale revision with matching `request_id`.
  - Verify `describe_tool` success contributes `upstream_state: "not_attempted"`.
  - Verify pruned search and describe payload keys.
- In `internal/service/progressive_projection_test.go` and `internal/service/progressive_session_test.go`:
  - Update expected tool names in dynamic projections from legacy `t_...` to wire names `alpha__...`.
- In `cmd/mcp-bridge/serve_endpoint_rebind_test.go`:
  - Update `awaitDiscovery` and `callCurrentTool` calls to expect wire names for discovered tools.
- In `cmd/mcp-bridge/serve_test.go` and `cmd/mcp-bridge/serve_oauth_test.go`:
  - Add end-to-end wire tests for omitted revision, wire names, pruned payloads, and diagnostic log joins asserting `upstream_state: "not_attempted"` for describe requests.
- In `README.md`:
  - Update tool surface, discovery, and invocation sections to reflect optional `catalog_revision`, semantic wire naming, and streamlined envelopes.
<!-- specdd:design:end -->

<!-- specdd:tasks:start -->
## 4. Task Ledger

### Traceability Mapping
| Requirement | Acceptance Criteria | Task |
|---|---|---|
| R1 | AC-1, AC-2 | T1 |
| R2 | AC-3 | T2 |
| R3 | AC-4, AC-5 | T3 |
| R4 | AC-6 | T3 |
| R5 | AC-7, AC-8 | T4 |

### T1 — Optional catalog_revision Admission and Describe Correlation
- **Inputs:** Approved spec r7; Ticket 01; `internal/catalog/schema.go`, `internal/service/tools.go`, `internal/service/direct_execution.go`, `internal/catalog/catalog_test.go`, `internal/service/tools_test.go`
- **Files:**
  - `internal/catalog/schema.go`
  - `internal/catalog/catalog_test.go`
  - `internal/service/tools.go`
  - `internal/service/direct_execution.go`
  - `internal/service/tools_test.go`
  - `.omp/artifacts/receipt_homelab_T1.json`
- **Implementation Deliverable:**
  - Update `CatalogRevisionPattern = "^$|^[a-f0-9]{64}$"` in `internal/catalog/schema.go`.
  - Remove `"catalog_revision"` from required properties in `DescribeToolInputSchema()` and `CallToolInputSchema()`.
  - In `describeTool` and `callTool` (`internal/service/tools.go`), implement empty-string/omitted defaulting to `assembly.CatalogRevision()`, format validation rejecting malformed hex with `invalid_arguments` and `mcp_complete` diagnostic emission, and equality validation rejecting stale revisions with `catalog_changed`.
  - Standardize `logAdmissionFailure` for truthful `operation: "describe_tool"` and `operation: "call_tool"` attribution; migrate `internal/service/direct_execution.go:34,39`.
  - In `describeTool`, extract server-owned request identity from carrier header (`req.Extra.Header["X-Mcp-Bridge-Internal-Request-Id"]`), mint operation identity, and contribute truthful `upstream_state: "not_attempted"`.
  - Unit tests asserting optionality, empty string acceptance, default to live revision, malformed hex rejection, stale revision rejection, and correlation joins with `upstream_state: "not_attempted"`.
- **Verification Commands:**
  - `go test -v -count=1 ./internal/catalog/...`
  - `go test -v -count=1 ./internal/service/... -run 'TestToolSurface(Describe|Call)'`
- **Covered ACs:** AC-1, AC-2

### T2 — Semantic Wire Name Publication and Dynamic Projection
- **Inputs:** Approved spec r7; Ticket 02; Task T1 complete; `internal/service/tools.go`, `internal/service/projection.go`, `internal/service/tools_test.go`, `internal/service/progressive_discovery_test.go`, `internal/service/progressive_session_test.go`, `internal/service/progressive_projection_test.go`, `internal/service/progressive_concurrency_lifecycle_test.go`, `cmd/mcp-bridge/serve_endpoint_rebind_test.go`
- **Files:**
  - `internal/service/tools.go`
  - `internal/service/projection.go`
  - `internal/service/tools_test.go`
  - `internal/service/progressive_discovery_test.go`
  - `internal/service/progressive_session_test.go`
  - `internal/service/progressive_projection_test.go`
  - `internal/service/progressive_concurrency_lifecycle_test.go`
  - `cmd/mcp-bridge/serve_endpoint_rebind_test.go`
  - `.omp/artifacts/receipt_homelab_T2.json`
- **Implementation Deliverable:**
  - Implement `effectiveWireName` helper applying $\le 64$ characters and valid character bounds with deterministic hash fallback.
  - Publish wire names in `searchPayload` for all results.
  - Project wire names in `preparePinProjection` for dynamically revealed unpinned tools.
  - Update service unit tests, progressive projection tests, and rebind tests to assert wire names on discovery and in dynamic `tools/list`.
- **Verification Commands:**
  - `go test -v -count=1 ./internal/service/...`
  - `go test -v -count=1 ./cmd/mcp-bridge -run 'TestBridgeRebindsServerEndpointOnRestartAndReload'`
- **Covered ACs:** AC-3

### T3 — Prune Search, Describe, and Call Payloads
- **Inputs:** Approved spec r7; Ticket 03; Tasks T1, T2 complete; `internal/service/tools.go`, `internal/service/tools_test.go`
- **Files:**
  - `internal/service/tools.go`
  - `internal/service/tools_test.go`
  - `.omp/artifacts/receipt_homelab_T3.json`
- **Implementation Deliverable:**
  - In `searchPayload`, strip `backends: []`, `last_success_at`, `catalog_status`, and `catalog_revision` from each result row. Retain `availability`.
  - In `describeTool`, strip `last_success_at` and `catalog_status`. Preserve `output_schema` and metadata when present.
  - In `callTool`, strip `catalog_revision`, `catalog_status`, and `availability` from the outer payload.
  - Update unit and service test assertions to verify exact pruned payload keys.
- **Verification Commands:**
  - `go test -v -count=1 ./internal/service/...`
- **Covered ACs:** AC-4, AC-5, AC-6

### T4 — Integrated Wire Verification, Documentation, and Coverage Gate
- **Inputs:** Approved spec r7; Ticket 04; Tasks T1, T2, T3 complete; `cmd/mcp-bridge/serve_test.go`, `cmd/mcp-bridge/serve_oauth_test.go`, `README.md`
- **Files:**
  - `cmd/mcp-bridge/serve_test.go`
  - `cmd/mcp-bridge/serve_oauth_test.go`
  - `README.md`
  - `coverage.out`
  - `.omp/artifacts/receipt_homelab_T4.json`
- **Implementation Deliverable:**
  - Wire tests asserting omitted/empty `catalog_revision`, stale-revision HTTP rejection, malformed-revision HTTP rejection, wire name dynamic exposure, pruned payloads, and diagnostic correlation joins over live HTTP asserting `upstream_state: "not_attempted"` for describe requests.
  - Update `README.md` to reflect new ergonomics contracts.
  - Full repository verification: race suite, vet, and coverage gate check.
- **Verification Commands:**
  - `go test -race ./...`
  - `go vet ./...`
  - `go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...`
  - `go run ./cmd/covcheck`
- **Covered ACs:** AC-7, AC-8
<!-- specdd:tasks:end -->

<!-- specdd:envelope:start -->
## 5. Envelope
Allowed paths:
- `internal/catalog/schema.go`
- `internal/catalog/catalog_test.go`
- `internal/service/tools.go`
- `internal/service/direct_execution.go`
- `internal/service/projection.go`
- `internal/service/tools_test.go`
- `internal/service/progressive_discovery_test.go`
- `internal/service/progressive_session_test.go`
- `internal/service/progressive_projection_test.go`
- `internal/service/progressive_concurrency_lifecycle_test.go`
- `cmd/mcp-bridge/serve_test.go`
- `cmd/mcp-bridge/serve_oauth_test.go`
- `cmd/mcp-bridge/serve_endpoint_rebind_test.go`
- `README.md`
- `coverage.out`
- `.omp/artifacts/receipt_homelab_T1.json`
- `.omp/artifacts/receipt_homelab_T2.json`
- `.omp/artifacts/receipt_homelab_T3.json`
- `.omp/artifacts/receipt_homelab_T4.json`
<!-- specdd:envelope:end -->

<!-- specdd:observability:start -->
## 6. Observability

### Logging & Diagnostic Schemas
- **Collection & Shipping Path:** Inbound HTTP requests and MCP operations emit structured JSON records synchronously to standard output (`os.Stdout`) via `observe.Logger`.
- **Schema Contracts & Formatting:**
  - Outer HTTP completions emit typed 65-slot positional JSON records conforming to `internal/observe/httpdiagnostic_encode.go`.
  - MCP operational completions (`mcp_complete`) emit structured JSON records conforming to `observe.Entry` (`internal/observe/log.go:92–104`) with UTC RFC3339Nano `time`, `level`, `service: "mcp-bridge"`, `environment`, `version`, `event: "mcp_complete"`, `message`, `request_id`, `duration_ms`, `outcome`, `reason`. Unbounded identifiers (`request_id`, `operation_id`) stay in fields, never in stream labels.
- **Sink Synchronization & Outage Behavior:**
  - Both logging lanes perform synchronous writes under an unexported mutex (`l.mu.Lock()`):
    - *Typed HTTP lane (`logHTTP`):* writes 65-slot positional bytes directly via `l.output.Write(...)`. Returned write errors are ignored (`_, _ = l.output.Write(...)`).
    - *Legacy MCP lane (`logLegacyEntry`):* serializes via `l.encoder.Encode(value)`. If write returns an error, it recreates the encoder (`l.encoder = json.NewEncoder(l.output)`).
  - *Stall & Backpressure Behavior:* Because writes are synchronous under mutex, a blocked or stalled writer (e.g. stdout pipe buffer full) will backpressure and delay the active goroutine and concurrent logging callers. Returned write errors are nonfatal to request execution, but blocked sink I/O delays caller completion.
- **Severity Policy & Admission Classifications:**
  - Successful `describe_tool` or `call_tool` executions (including those with omitted/empty `catalog_revision`) emit `event: "mcp_complete"` at `level: "info"`, `outcome: "success"`, `reason: "none"`. Successful `describe_tool` contributes truthful `upstream_state: "not_attempted"` since it does not invoke an upstream backend.
  - Stale `catalog_revision` rejections emit `event: "mcp_complete"` at `level: "warn"`, `stage: "tool_admission"`, `outcome: "error"`, `reason: "catalog_changed"`, and contribute `upstream_state: "not_attempted"`.
  - Malformed `catalog_revision` rejections emit `event: "mcp_complete"` at `level: "warn"`, `stage: "tool_admission"`, `outcome: "error"`, `reason: "invalid_arguments"`, and contribute `upstream_state: "not_attempted"`.
  - Truthful operation attribution: `operation: "describe_tool"` for `describe_tool` admission, `operation: "call_tool"` for `call_tool` admission. Both extract server-owned request IDs from carrier header `req.Extra.Header["X-Mcp-Bridge-Internal-Request-Id"]` to join cleanly with `http_complete` logs.
  - Zero downstream dispatch occurs on both rejection branches; no sensitive tool payload or credential data is ever emitted in diagnostic records.
- **Verification Mapping:** Verified via AC-2 in T1 (`internal/service/tools_test.go`) and AC-7 in T4 (`cmd/mcp-bridge/serve_test.go`).

### Justified Exceptions
- **Prometheus Metrics Scrape & Latency Histograms:**
  - *Exception:* No new metric series or latency histograms are introduced; dedicated HTTP scrape verification of `/metrics` is excluded from the feature scope.
  - *Reason:* All inbound tool invocations (`tools_call`, `describe_tool`, `search_tools`) arrive via JSON-RPC over HTTP and are counted under the existing bounded whitelist `bridge_requests_total{server="_bridge", operation="tools_call", outcome="success" | "failure"}` in `internal/observe/metrics.go:38–50`. Latency is observed per-event via `duration_ms` in structured log records. Metric registry encoding is independently verified in `internal/observe/metrics_test.go`.
  - *Consequence:* Scrape endpoint serialization is not re-exercised in wire tests; operators inspect latency distributions through log queries (e.g. Loki log-derived quantiles over `duration_ms`) rather than Prometheus histogram buckets. Unchanged scrape configuration assurance is bounded by unit coverage in `metrics_test.go`.
- **Dashboards & Alert Rules:**
  - *Exception:* No new dashboard panels, alerting rules, or alert verification tests are added.
  - *Reason:* Existing homelab Grafana dashboards monitor top-level bridge error rate (`rate(bridge_requests_total{outcome="failure"}[5m])`) and process health. Because the feature modifies only tool discovery payloads and input parameter validation without altering error metric semantics, existing alert rules remain valid without modification.
  - *Consequence:* Alert triggering and dashboard panel rendering are not verified within this feature; operational visibility depends on external monitoring infrastructure.
- **Loki In-Process Spooling & Retention:**
  - *Exception:* `mcp-bridge` does not include an in-process Loki client, local disk buffer, or log retention scheduler.
  - *Reason:* The bridge operates as a 12-factor service delegating log shipping to stdout. Log collection (e.g. Promtail / Vector daemonset) and retention (typically 30 days) are governed globally by the host container deployment environment.
  - *Consequence:* Verification in repository tests confirms stdout emission and JSON formatting, but cannot verify external Loki delivery or retention. If container stdout is discarded during an abrupt container runtime crash, unbuffered logs may be lost.
- **OpenTelemetry Distributed Tracing:**
  - *Exception:* No OpenTelemetry spans or traceparent headers are emitted.
  - *Reason:* End-to-end request correlation across HTTP and MCP layers is owned via server-minted 32-hex `request_id` passed via `X-Mcp-Bridge-Internal-Request-Id` and `Mcp-Diagnostic-Id`.
  - *Consequence:* APM distributed tracing trees are not generated; correlation relies on deterministic log joins on `request_id`.
- **Health & Readiness Checks:**
  - *Exception:* `/livez` and `/readyz` handlers and probes are unchanged.
  - *Reason:* Tool parameter optionality and payload pruning are pure memory transformations; they do not alter upstream server connectivity checks or catalog health evaluation.
  - *Consequence:* Readiness probes continue to report actual backend availability without synthetic probe masking.

### Rollback & Session Reconnect Procedure
- Reverting the binary restores mandatory `catalog_revision` validation, legacy `t_<base32>` hash names, and verbose search/describe/call payloads.
- *Client Recovery Action:* Clients that adopted optional revision behavior must restore explicit revision handling; clients caching `<server>__<tool>` names must reinitialize and rediscover via `search_tools` or `tools/list` to acquire the legacy `t_<base32>` names. Simple transport reconnection without client logic restoration does not restore compatibility for revision-omitting callers.
<!-- specdd:observability:end -->

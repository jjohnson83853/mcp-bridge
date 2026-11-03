# OpenAI Tool Schema Compatibility

Status: ready-for-agent

<!-- specdd:requirements:start -->
## 1. Requirements

### R1 — Root Dialect Omission & Standard Tool Manifest Alignment
All built-in tool parameter schemas (`search_tools`, `describe_tool`, and `call_tool`) MUST omit the root `$schema` keyword entirely. In Model Context Protocol architectures, `inputSchema` represents an embedded tool parameter definition within an enclosing JSON-RPC manifest, rather than a standalone document. Omitting `$schema` matches reference MCP client SDK conventions (Anthropic Messages API, OpenAI Function Calling, and official TypeScript/Python SDKs) and prevents client-side compiler rejections.

### R2 — Array Parameter Bounds Normalization & Decoder Enforcement
The `servers` array parameter in `SearchToolsInputSchema` MUST remove client-facing `minItems: 1` and `maxItems: 32` keywords. The schema MUST retain `type: "array"`, item regex pattern `^[a-z][a-z0-9-]{0,31}$`, and `uniqueItems: true`.
Server-side bounds enforcement in `internal/search/search.go:DecodeRequest` MUST enforce:
1. An empty `servers: []` slice MUST be treated as an omitted filter (`request.Servers = nil`), returning results equivalent to a search where `servers` was completely omitted.
2. A populated `servers` slice with 1 to 32 entries MUST be accepted and filtered accordingly.
3. Any request providing more than 32 server entries MUST be rejected with `invalidArguments("servers exceeds maximum of 32 entries")` which maps to a tool execution failure (`isError: true`, `code: "invalid_arguments"`).

### R3 — Generic Argument Object Definition
The `arguments` parameter in `CallToolInputSchema` MUST be defined as:
```json
"arguments": {
  "type": "object",
  "additionalProperties": true,
  "description": "Arguments for the upstream tool invocation"
}
```
This satisfies OpenAI function-calling object schema requirements while allowing arbitrary tool arguments to be forwarded downstream.

### R4 — Parameter Optionality & String Constraints
Standard parameter optionality MUST be preserved: `query` remains required in `search_tools`; `top_k` and `servers` remain optional properties. All string length constraints (`minLength: 1`, `maxLength: 2048`) and regex patterns (`pattern`) on `query`, `tool_name`, `catalog_revision`, and `server` MUST remain in place.

### R5 — End-to-End Action Discovery Wire Verification & Coverage Gate
End-to-end loopback HTTP wire tests in `cmd/mcp-bridge/serve_oauth_test.go` MUST verify:
1. `POST /mcp` for `tools/list` returns tool definitions where `$schema`, `minItems`, and `maxItems` are completely absent (comma-ok check, not nil equality), and `arguments` has `additionalProperties: true` with description.
2. Invoking `search_tools` with `servers: []` succeeds with `isError: false` and matches the results of omitting `servers`.
3. Invoking `search_tools` with up to 32 valid configured servers succeeds with `isError: false`, while 33 servers returns a tool error (`isError: true`, `code: "invalid_arguments"`).
4. All repository tests pass cleanly under `go test -race ./...` and aggregate source coverage meets or exceeds the required 90.0% gate (`go run ./cmd/covcheck`).

**Non-goals:** modifying upstream tool schemas from third-party MCP servers; altering search embedding or ranking logic; adding OpenAI-specific user-agent endpoint forks.
<!-- specdd:requirements:end -->

<!-- specdd:acceptance:start -->
## 2. Acceptance Criteria

- **AC-1 (Root $schema Omission):** `SearchToolsInputSchema()`, `DescribeToolInputSchema()`, and `CallToolInputSchema()` do not contain a `$schema` key. In Go map representation, `_, exists := schema["$schema"]` evaluates to `false`.
- **AC-2 (Array Bounds Normalization):** In `SearchToolsInputSchema()`, the `servers` property map does not contain `minItems` or `maxItems` keys (`_, exists := servers["minItems"]` is `false`). `type: "array"`, `uniqueItems: true`, and `items.pattern` are preserved.
- **AC-3 (Arguments Object Definition):** In `CallToolInputSchema()`, `arguments` contains `"type": "object"`, `"additionalProperties": true`, and `"description": "Arguments for the upstream tool invocation"`.
- **AC-4 (String Constraints & Optionality Preservation):**
  - `query` requires `minLength: 1`, `maxLength: 2048`, and is listed in `required`.
  - `top_k` and `servers` are defined in `properties` but omitted from `required`.
  - `tool_name` requires pattern `^[a-z0-9_-]{1,64}$`.
  - `catalog_revision` requires pattern `^[a-f0-9]{64}$`.
- **AC-5 (Decoder Server Bounds & Empty Tolerance):** In `internal/search/search.go:DecodeRequest`:
  - `servers: []` results in `request.Servers == nil`.
  - Exactly 32 servers is parsed without error.
  - Exactly 33 servers returns `*search.Failure` with `Code: "invalid_arguments"` and message `"servers exceeds maximum of 32 entries"`.
- **AC-6 (Wire Discovery Payload Conformance):** In `cmd/mcp-bridge/serve_oauth_test.go`, loopback wire requests to `tools/list` return JSON-RPC responses whose tool definitions satisfy AC-1, AC-2, and AC-3 over live HTTP wire.
- **AC-7 (Wire Search Execution Equivalence & Boundaries):** In `cmd/mcp-bridge/serve_oauth_test.go`:
  - Wire invocation of `search_tools` with `servers: []` returns `isError: false` matching omitted filter.
  - Wire invocation of `search_tools` with up to 32 valid configured servers returns `isError: false`.
  - Wire invocation of `search_tools` with 33 servers returns `isError: true` and `code: "invalid_arguments"`.
- **AC-8 (Repository Verification & Coverage Gate):** `go test -race ./...` passes clean, `go vet ./...` passes clean, and `go run ./cmd/covcheck` confirms aggregate repository source coverage ≥ 90.0%.
<!-- specdd:acceptance:end -->

<!-- specdd:design:start -->
## 3. Design

### 3.1 Catalog Schema Definitions (`internal/catalog/schema.go`)
- In `SearchToolsInputSchema()`, `DescribeToolInputSchema()`, and `CallToolInputSchema()`:
  - Remove `"$schema": Draft202012`.
- In `SearchToolsInputSchema()`:
  - Remove `"minItems": 1` and `"maxItems": 32` from the `"servers"` property map.
- In `CallToolInputSchema()`:
  - Update `"arguments"` to:
    ```go
    "arguments": map[string]any{
        "type":                 "object",
        "additionalProperties": true,
        "description":          "Arguments for the upstream tool invocation",
    },
    ```

### 3.2 Server-Side Request Decoding (`internal/search/search.go`)
- In `DecodeRequest(raw json.RawMessage, cfg Config)`:
  - When `wire.Servers != nil`:
    - If `len(*wire.Servers) == 0`: set `request.Servers = nil` (treating empty slice as omitted filter).
    - If `len(*wire.Servers) > 32`: return `Request{}, invalidArguments("servers exceeds maximum of 32 entries")`.
    - Otherwise: `request.Servers = append([]string(nil), (*wire.Servers)...)`.

### 3.3 Test Updates (`internal/catalog/catalog_test.go`, `internal/search/search_test.go`, `internal/service/tools_test.go`)
- In `internal/catalog/catalog_test.go`:
  - Update `assertSchema` to assert `if _, exists := schema["$schema"]; exists { t.Fatalf("%s has $schema key", name) }`.
  - In `TestSearchToolsSchemaMatchesContract`, assert `_, hasMin := servers["minItems"]; if hasMin { t.Fatal(...) }` and `_, hasMax := servers["maxItems"]; if hasMax { t.Fatal(...) }`.
  - In `TestCallToolSchemaMatchesContract`, assert `arguments["additionalProperties"] == true` and `arguments["description"] != ""`.
- In `internal/search/search_test.go`:
  - Add test case verifying that `{"query":"x","servers":[]}` succeeds and yields `request.Servers == nil`.
  - Add test case verifying that 32 servers succeeds, while 33 servers returns `CodeInvalidArguments`.
- In `internal/service/tools_test.go`:
  - Update any built-in schema assertions expecting `$schema` to assert its absence.
<!-- specdd:design:end -->

<!-- specdd:tasks:start -->
## 4. Task Ledger

### Traceability Mapping
| Requirement | Acceptance Criteria | Task |
|---|---|---|
| R1 | AC-1 | T1 |
| R2 | AC-2, AC-5 | T1 |
| R3 | AC-3 | T1 |
| R4 | AC-4 | T1 |
| R5 | AC-6, AC-7, AC-8 | T2 |

### T1 — Tool Input Schema Alignment and Bounds Normalization
- **Inputs:** Approved spec r3; Ticket 01; `internal/catalog/schema.go`, `internal/catalog/catalog_test.go`, `internal/search/search.go`, `internal/search/search_test.go`, `internal/service/tools_test.go`
- **Files:** `internal/catalog/schema.go`, `internal/catalog/catalog_test.go`, `internal/search/search.go`, `internal/search/search_test.go`, `internal/service/tools_test.go`
- **RED Verification Sequence:**
  1. In `internal/catalog/catalog_test.go`, update `assertSchema` to fail if `$schema` key is present.
  2. Run: `go test -v -count=1 ./internal/catalog -run TestSearchToolsSchemaMatchesContract` -> observe RED failure (`has $schema key`).
  3. In `internal/search/search_test.go`, add assertions for `servers: []` and 33 servers.
  4. Run: `go test -v -count=1 ./internal/search -run TestDecodeRequest` -> observe RED failure.
- **Implementation Deliverable:**
  - Update `internal/catalog/schema.go` to omit `$schema`, remove `minItems`/`maxItems`, set `additionalProperties: true` on `arguments`.
  - Update `internal/search/search.go` to handle empty servers and enforce 32 server bound.
  - Update tests in `catalog_test.go`, `search_test.go`, and `tools_test.go`.
- **GREEN Verification Sequence:**
  1. Run `go test -v -count=1 ./internal/catalog/...` -> PASS (GREEN).
  2. Run `go test -v -count=1 ./internal/search/...` -> PASS (GREEN).
  3. Run `go test -v -count=1 ./internal/service/... -run TestToolSurfaceAdvertisesExactlyItsThreeTools` -> PASS (GREEN).
- **Covered ACs:** AC-1, AC-2, AC-3, AC-4, AC-5

### T2 — End-to-End Action Discovery Wire Verification
- **Inputs:** Approved spec r3; Ticket 02; Task T1 complete; `cmd/mcp-bridge/serve_oauth_test.go`
- **Files:** `cmd/mcp-bridge/serve_oauth_test.go`
- **Implementation Deliverable:**
  - In `cmd/mcp-bridge/serve_oauth_test.go`, add or update wire tests verifying that `tools/list` over live HTTP serves schemas satisfying AC-1, AC-2, and AC-3.
  - Add wire tests for `search_tools` execution with `servers: []` (success), up to 32 valid configured servers (success), and 33 servers (returns `isError: true`, `code: "invalid_arguments"`).
- **Verification Sequence:**
  1. Run `go test -v -count=1 ./cmd/mcp-bridge -run TestChatGPT` -> PASS (GREEN).
  2. Run full repository verification:
     - `go test -race ./...` -> exit 0
     - `go vet ./...` -> exit 0
     - `go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...` -> exit 0
     - `go run ./cmd/covcheck` -> exit 0 (coverage ≥ 90.0%).
- **Covered ACs:** AC-6, AC-7, AC-8
<!-- specdd:tasks:end -->

<!-- specdd:envelope:start -->
## 5. Envelope
Allowed paths:
- `internal/catalog/schema.go`
- `internal/catalog/catalog_test.go`
- `internal/search/search.go`
- `internal/search/search_test.go`
- `internal/service/tools_test.go`
- `cmd/mcp-bridge/serve_oauth_test.go`
- `coverage.out`
- `.scratch/openai-tool-schema-compatibility/*`
- `.omp/plans/openai-tool-schema-compatibility/*`
- `.omp/artifacts/receipt_schema_T1.json`
- `.omp/artifacts/receipt_schema_T2.json`
<!-- specdd:envelope:end -->

<!-- specdd:observability:start -->
## 6. Observability

### Logging & Diagnostics
- Tool discovery and call completions emit structured records through `internal/observe/httpdiagnostics.go` directly to process stdout.
- Physical keys conform strictly to `internal/observe/httpdiagnostic_encode.go:67–84`: `time`, `level`, `service`, `environment`, `version`, `duration_ms`, `http_outcome`, `http_status`, `http_method`, `route`, `request_id`, `diag_schema: "oauth_diag_v1"`, `upstream_state`.
- Invalid parameter failures for `search_tools` (such as >32 servers) return HTTP 200 with an MCP tool execution failure (`isError: true`, `code: "invalid_arguments"`), logged at `level: "warn"` under `upstream_state: "tool_error"`.
- Zero raw user prompts, bearer tokens, or client credentials are logged.

### Justified Exceptions
- Prometheus metrics are unchanged by schema adjustments (zero new metrics; no high-cardinality route labels).
- Live Vector shipping and Loki indexing are not executed in automated local tests (tests assert stdout JSON output directly).
- Distributed tracing is omitted (edge daemon with synchronous JSON-RPC).
<!-- specdd:observability:end -->

<!-- specdd:release_and_recovery:start -->
## 7. Release and Recovery

### Rollback Plan
If client regressions are observed post-deployment:
1. Revert the specific feature commit on `main` using `git revert <commit_sha>` (owned by `git-master`), leaving unrelated commits untouched.
2. Run full verification suite (`go test -race ./...` and `go run ./cmd/covcheck`).
3. Push to `origin/main`. Redeployment to production requires explicit operator authorization.
<!-- specdd:release_and_recovery:end -->

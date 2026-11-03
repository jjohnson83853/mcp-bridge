# MCP Inbound Observability Enrichment

Status: ready-for-agent

## Problem Statement

When investigating production issues or client integration failures (such as ChatGPT action discovery or client disconnects), operators inspecting Loki logs and Prometheus metrics cannot easily identify what happened from `mcp-bridge` telemetry alone:
1. **Generic Operation Name**: Every inbound POST request to `/mcp` (except prompts) logs `operation: "request"`. It is impossible to distinguish an `initialize` handshake, a `notifications/initialized` signal, a `tools/list` discovery query, or a `tools/call` invocation without enabling verbose packet dumps.
2. **Missing Client Attribution**: Structured HTTP completion logs do not record the client family or user agent. Operators must manually cross-reference identity provider token issue timestamps to deduce whether a request originated from ChatGPT, Claude, Cursor, or an internal monitoring probe.
3. **Coarse Byte Buckets**: Response sizes are grouped into wide logarithmic buckets (e.g. `le_16k` to `le_256k`). A normal 17 KB response falls into the same bucket as an oversized 250 KB payload, obscuring whether the bridge served 3 built-in tools or an unexpected 50-tool catalog dump.
4. **SSE Stream Ambiguity**: Streaming GET requests are classified as generic `request` operations, conflating long-lived streaming connections with discrete sub-second RPC calls in metrics and logs.

## Solution

Enhance the internal observability pipeline within `mcp-bridge` while preserving its strict security, zero-leakage, and low-cardinality guarantees:
1. **Unified MCP JSON-RPC Operation Enum**: Expand `inboundOperation` in the inbound request handler to map standard MCP methods into a hardcoded, closed finite enum across both Prometheus metrics and Loki diagnostic records: `initialize`, `initialized`, `tools_list`, `tools_call`, `resources_list`, `resources_read`, `ping`, `prompts_list`, `prompts_get`, `sse_stream`, and `request`.
2. **Classify Client User-Agent Families**: Add a typed, closed `ClientFamily` attribute to diagnostic completion records (`openai`, `claude`, `cursor`, `omp`, `browser`, `other`, `absent`) using strict priority substring matching, allowing instant client filtering in Loki without exposing high-cardinality raw User-Agent strings.
3. **Emit Tool Count on `tools/list`**: Record the integer count of exposed tools (`exposed_tools`) returned in each `tools/list` response, providing unambiguous evidence of catalog exposure per session.
4. **Emit Exact HTTP Response Bytes**: Include the exact response byte count (`http_bytes: <int>`) in the completion diagnostic record alongside the coarse logarithmic byte bucket (`http_bytes_bucket`).

## User Stories

1. As an operator debugging a client connection failure, I want `http_complete` log entries to state whether the request was `tools/list`, `initialize`, or `tools/call`, so that I can immediately determine which protocol step failed.
2. As an operator querying Loki logs, I want to filter by `client_family: "openai"` or `client_family: "claude"`, so that I can isolate external AI platform traffic from internal monitoring probes.
3. As an operator investigating action discovery errors, I want to see `exposed_tools: 3` in `tools/list` completion records, so that I can confirm the bridge did not exceed client tool count limits.
4. As an SRE monitoring Prometheus metrics, I want request counts partitioned by standard MCP method (`operation="tools_list"`, `operation="tools_call"`, etc.), so that I can track usage and latency per MCP operation type.
5. As an SRE monitoring Prometheus metrics, I want SSE streaming connections labeled as `operation="sse_stream"`, so that long-lived streaming durations do not distort discrete RPC latency histograms.
6. As a security engineer, I want all newly added log and metric labels to use closed, finite enums, so that malicious clients cannot cause label cardinality explosion or inject arbitrary text into monitoring systems.
7. As a security engineer, I want raw User-Agent strings to be normalized into a finite set of known families using specific-first precedence, so that client system details or tracking headers are never persisted in log streams.
8. As an operator reading structured logs, I want to see the exact byte count (`http_bytes: 1845`) for each completed HTTP request alongside `http_bytes_bucket`, so that I can verify payload sizes without guesswork.
9. As a developer writing regression tests, I want loopback wire tests to assert the presence and accuracy of `operation`, `client_family`, `exposed_tools`, and `http_bytes` in captured diagnostic records.
10. As an operator running downstream log analytics, I want existing JSON log schema keys to remain backward-compatible, so that existing Grafana dashboards and alerts continue functioning.
11. As a developer, I want unparseable or non-JSON request bodies to fall back safely to `operation: "request"` without throwing exceptions or panicking.
12. As an operator inspecting authentication failures, I want client family attribution to be recorded even when a request is rejected at the outer OAuth middleware layer.

## Implementation Decisions

- **Unified Operation Enum**:
  - In `internal/service/inbound.go:inboundOperation`, expand the method switch to match:
    - `"initialize"` -> `"initialize"`
    - `"notifications/initialized"` -> `"initialized"`
    - `"tools/list"` -> `"tools_list"`
    - `"tools/call"` -> `"tools_call"`
    - `"resources/list"` -> `"resources_list"`
    - `"resources/read"` -> `"resources_read"`
    - `"ping"` -> `"ping"`
    - `"prompts/list"` -> `"prompts_list"`
    - `"prompts/get"` -> `"prompts_get"`
    - GET requests with SSE headers -> `"sse_stream"`
    - Default / unparseable -> `"request"`
  - Both Prometheus metrics (`RecordRequest(operation, outcome)`) and structured diagnostic logs (`observe.HTTPLegacyEvidence{Operation: operation}`) receive this exact classification.
- **Client Family Classification Precedence**:
  - In `internal/observe/httpdiagnostics.go`, inspect `r.UserAgent()` using case-insensitive priority substring matching:
    1. Contains `"openai"` -> `"openai"`
    2. Contains `"claude"` or `"anthropic"` -> `"claude"`
    3. Contains `"cursor"` -> `"cursor"`
    4. Contains `"omp/"` -> `"omp"`
    5. Contains `"mozilla"`, `"chrome"`, or `"safari"` -> `"browser"`
    6. Non-empty string -> `"other"`
    7. Empty / absent -> `"absent"`
  - Add `ClientFamily string` to `HTTPDecision` and emit it as `client_family` in the structured JSON diagnostic output.
- **Tool Count & Byte Recording**:
  - When completing `tools/list` in the tool surface / exposure handler (`internal/service/session_exposure.go`), record `exposed_tools: len(pageItems)` in the request diagnostic context.
  - In `HTTPDiagnostics`, emit `http_bytes: int64` alongside the existing `http_bytes_bucket: string` in the terminal `http_complete` record.
- **Zero-Allocation & Immutability Guarantees**:
  - Retain the existing single-owner diagnostics lifecycle (`Claim` -> `Contribute` -> `Finish`).
  - Contributions remain no-ops if no owner is present.

## Testing Decisions

- **What Makes a Good Test**:
  - Tests must verify external observable behavior by inspecting the stdout/stderr JSON completion records emitted by the server during real HTTP requests.
  - Tests must verify that diverse User-Agent headers (e.g. `Mozilla/5.0 ... Claude/1.0`, `openai-connectors/1.0`) map accurately to their designated `client_family` enum values based on priority rules.
  - Tests must verify that `initialize`, `notifications/initialized`, `tools/list`, and `tools/call` produce their respective `operation` labels in both metrics and structured logs.
  - Tests must verify that `tools/list` completion records contain `exposed_tools: 3` and exact `http_bytes`.
  - Tests must confirm that malformed JSON or empty bodies safely produce `operation: "request"` and do not cause crashes.
- **Modules Tested**:
  - `internal/observe`: Unit tests for `client_family` classification rules, `exposed_tools` contribution, and exact `http_bytes` formatting.
  - `internal/service`: Inbound handler tests verifying operation classification across standard MCP methods and SSE GET requests.
  - `cmd/mcp-bridge`: Loopback HTTP integration tests verifying that end-to-end requests emit diagnostic records with populated `operation`, `client_family`, `exposed_tools`, and `http_bytes`.
- **Prior Art**:
  - `internal/observe/httpdiagnostics_test.go` exercises diagnostic capture and field emission.
  - `cmd/mcp-bridge/serve_oauth_test.go` asserts specific fields in captured stdout diagnostic records.

## Out of Scope

- Modifying Caddy reverse proxy configuration or host syslog/systemd settings.
- Emitting raw un-sanitized User-Agent strings.
- Tracing or logging individual tool call payload arguments (which may contain user-sensitive prompts or secrets).
- Changing Prometheus metric names or breaking existing Prometheus metric scrapers.

## Further Notes

- All changes are completely internal to the Go binary (`mcp-bridge`) and require no external container, infrastructure, or proxy alterations.
- Complies with the bridge's observability contract: fixed-slot schemas, bounded cardinality, and zero raw credential leakage.

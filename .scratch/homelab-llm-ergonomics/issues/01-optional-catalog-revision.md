# 01 — Optional catalog_revision Admission and Describe Correlation

Status: ready-for-agent
Type: task
Blocked by: None

**What to build:**
Make `catalog_revision` optional in `DescribeToolInputSchema` and `CallToolInputSchema` in `internal/catalog/schema.go`, and update `CatalogRevisionPattern = "^$|^[a-f0-9]{64}$"`.
In `internal/service/tools.go` and `internal/service/direct_execution.go`:
- For `describe_tool` and `call_tool`, if `catalog_revision` is empty or omitted, bind implicitly to `assembly.CatalogRevision()` and execute normally.
- If non-empty:
  - If invalid hex format (`len != 64` or not hex), reject with `invalid_arguments` and emit `mcp_complete` diagnostic with `stage: "tool_admission"`, `outcome: "error"`, `reason: "invalid_arguments"` and zero downstream dispatch.
  - If valid 64-hex format but does not match `assembly.CatalogRevision()`, reject with `catalog_changed` and emit `mcp_complete` diagnostic with `stage: "tool_admission"`, `outcome: "error"`, `reason: "catalog_changed"` and zero downstream dispatch.
- Standardize admission logging with `logAdmissionFailure(ctx context.Context, requestID, operationID, corrState string, start time.Time, operation string, reason string)` for truthful `operation: "describe_tool"` and `operation: "call_tool"` attribution; migrate callers in `internal/service/direct_execution.go:34,39`.
- In `describeTool`, extract server-owned request identity from carrier header `req.Extra.Header["X-Mcp-Bridge-Internal-Request-Id"]` (matching `callTool:644-652`), mint independent `operationID`, and log both successful describe and admission rejection events with truthful operation attribution and extracted `request_id`.
- On successful `describe_tool`, contribute truthful `upstream_state: "not_attempted"` since describe executes against local catalog definitions without downstream dispatch.

**Acceptance Criteria:**
- [ ] `DescribeToolInputSchema()["required"]` is `[]any{"tool_name"}`.
- [ ] `CallToolInputSchema()["required"]` is `[]any{"tool_name", "arguments"}`.
- [ ] `DescribeToolInputSchema` and `CallToolInputSchema` validate `catalog_revision` pattern allows `""` and valid 64-character lowercase hex string.
- [ ] When `catalog_revision` is omitted or empty on `describe_tool`, it succeeds and returns the tool description for the current catalog revision, emitting `mcp_complete` with `operation: "describe_tool"`, `reason: "none"`, and matching `request_id`, contributing `upstream_state: "not_attempted"`.
- [ ] When `catalog_revision` is omitted or empty on `call_tool`, it executes upstream normally with the current catalog revision, emitting `mcp_complete` with `operation: "call_tool"`, `reason: "none"`, and matching `request_id`.
- [ ] When `catalog_revision` is invalid format (not 64-hex) on `describe_tool` or `call_tool`, rejects with `invalid_arguments`, zero downstream dispatch, and `mcp_complete` log with `reason: "invalid_arguments"`, matching operation, matching `request_id`, and `upstream_state: "not_attempted"`.
- [ ] When `catalog_revision` is valid 64-hex but stale on `describe_tool` or `call_tool`, rejects with `catalog_changed`, zero downstream dispatch, and `mcp_complete` log with `reason: "catalog_changed"`, matching operation, matching `request_id`, and `upstream_state: "not_attempted"`.
- [ ] Direct execution in `internal/service/direct_execution.go` calls standardized `logAdmissionFailure` cleanly with `operation: "call_tool"`.

**Verification Commands:**
```sh
go test -v -count=1 ./internal/catalog/...
go test -v -count=1 ./internal/service/... -run 'TestToolSurface(Describe|Call)'
```

**Envelope:**
- `internal/catalog/schema.go`
- `internal/catalog/catalog_test.go`
- `internal/service/tools.go`
- `internal/service/direct_execution.go`
- `internal/service/tools_test.go`
- `.omp/artifacts/receipt_homelab_T1.json`

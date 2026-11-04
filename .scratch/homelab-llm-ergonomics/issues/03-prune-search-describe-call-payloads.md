# 03 — Prune Search, Describe, and Call Payloads

Status: ready-for-agent
Type: task
Blocked by: 02

**What to build:**
Prune redundant SRE telemetry and cluster health dumps from tool responses.
1. In `searchPayload` (`internal/service/tools.go`), strip `backends: []`, `last_success_at`, `catalog_status`, and redundant per-item `catalog_revision`. Retain `availability: "available" | "unavailable"`.
2. In `describeTool` (`internal/service/tools.go`), strip `last_success_at` and `catalog_status`. Retain `availability`, `input_schema`, and preserve `output_schema` and metadata when present.
3. In `callTool` (`internal/service/tools.go`), strip `catalog_revision`, `catalog_status`, and `availability` from the outer envelope, retaining `server`, `tool_name`, `request_id`, and `upstream_result`.

**Acceptance Criteria:**
- [ ] `searchPayload` response omits `"backends"`.
- [ ] Each result row in `searchPayload` omits `"last_success_at"`, `"catalog_status"`, and `"catalog_revision"`.
- [ ] Each result row in `searchPayload` contains `"availability"`.
- [ ] `describeTool` response omits `"last_success_at"` and `"catalog_status"`.
- [ ] `describeTool` response preserves `"output_schema"` and metadata when present.
- [ ] `callTool` outer payload omits `"catalog_revision"`, `"catalog_status"`, and `"availability"`.
- [ ] Unit and service tests in `internal/service/` pass.

**Verification Commands:**
```sh
go test -v -count=1 ./internal/service/...
```

**Envelope:**
- `internal/service/tools.go`
- `internal/service/tools_test.go`
- `.omp/artifacts/receipt_homelab_T3.json`

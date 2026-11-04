# 03 — Exposed Tool Count and Wire Observability Verification

**What to build:**
Record the integer count of exposed tools (`exposed_tools`) on `tools/list` request completion records via context and internal request ID linkage, omitting it on failed writes or non-list requests. Verify end-to-end via loopback HTTP integration tests that real requests across different client families and MCP methods emit the exact expected structured JSON log lines and Prometheus metric labels.

**Blocked by:**
- 01 — Inbound Method Operation Classification and Unified Prometheus Metrics
- 02 — Client Family Classification, Operation Slot and Exact Response Bytes

**Status:** ready-for-agent

- [ ] Tool surface exposure handler in `internal/service/session_exposure.go` contributes `exposed_tools: len(result.Tools)` via `observe.ContributeExposedTools(ctx, requestID, ...)` on successful `tools/list` pagination.
- [ ] Terminal `http_complete` diagnostic lines include `exposed_tools: <int>` ONLY when `operation == "tools_list"` AND `httpOutcome == "success"`. Non-list requests, list errors, and write delivery failures omit `exposed_tools`.
- [ ] Composed wire test in `cmd/mcp-bridge/serve_oauth_test.go:TestInboundObservabilityWire` verifies that `POST /mcp` with `tools/list` produces a diagnostic record containing `operation: "tools_list"`, `exposed_tools: 3`, and exact `http_bytes` matching the response body byte length.
- [ ] Composed wire test verifies that requests with `User-Agent: openai-connectors/1.0` emit `client_family: "openai"`.
- [ ] Composed wire test verifies that requests with `User-Agent: Mozilla/5.0 (Macintosh) Claude/1.0` emit `client_family: "claude"`.
- [ ] Full repository test suite passes clean with race detector enabled (`go test -race -p 1 -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...`) and meets coverage gate (`go run ./cmd/covcheck` >= 90.0%).

# 03 — Exposed Tool Count and Wire Observability Verification

**What to build:**
Record the integer count of exposed tools (`exposed_tools`) on `tools/list` request completion records, and verify end-to-end via loopback HTTP integration tests that real requests across different client families and MCP methods emit the exact expected structured JSON log lines and Prometheus metric labels.

**Blocked by:**
- 01 — Inbound Method Operation Classification and Unified Prometheus Metrics
- 02 — Client Family Classification and Exact Response Bytes

**Status:** ready-for-agent

- [ ] Tool surface exposure handler in `internal/service/session_exposure.go` contributes `exposed_tools: len(tools)` to the completion diagnostic context on `tools/list` requests.
- [ ] Terminal `http_complete` diagnostic lines include `exposed_tools: <int>` when `operation == "tools_list"` (and null / omitted on non-tool-list requests).
- [ ] Composed wire test in `cmd/mcp-bridge/serve_oauth_test.go` verifies that `POST /mcp` with `tools/list` produces a diagnostic record containing `operation: "tools_list"`, `exposed_tools: 3`, and exact `http_bytes`.
- [ ] Composed wire test verifies that requests with `User-Agent: openai-connectors/1.0` emit `client_family: "openai"`.
- [ ] Composed wire test verifies that requests with `User-Agent: Claude/1.0` emit `client_family: "claude"`.
- [ ] Full repository test suite passes clean (`go test -race ./...`).

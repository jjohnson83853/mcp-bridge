# 03: Direct preloaded tool execution dispatch

**What to build:** An AI client can invoke a preloaded tool directly by its wire name (`<server_id>__<upstream_tool_name>`) via standard MCP `tools/call` without wrapping arguments in a `call_tool` envelope. The bridge validates arguments against the stored schema and forwards execution directly to the upstream server.

**Blocked by:** 02.

**Status:** complete

Type: task
Source: Hybrid tool discovery specification, revision HYBRID-DISCOVERY-r3. Coverage: User Stories 10–13, 18–21, 28–30; Implementation Decisions 4, 7.

- [x] Route direct tool calls matching registered preloaded wire names directly to the owning upstream backend.
- [x] Validate incoming tool call arguments against the tool's cached `inputSchema` before dispatching. If arguments are invalid, return an MCP error result with `isError: true` without hitting the backend.
- [x] Acquire an upstream execution lease enforcing configured timeouts, per-server concurrency limits, and circuit breaker health.
- [x] Forward the raw upstream tool name and arguments to the upstream backend's `tools/call` endpoint.
- [x] Unpack the upstream response and return the exact content elements to the caller as an MCP `CallToolResult`.
- [x] Inject upstream credentials securely; verify synthetic secrets never leak into client responses or log output.
- [x] Emit structured diagnostic logs for direct preloaded executions, recording duration, status, and outcome.
- [x] Add integration tests at the HTTP seam (`POST /mcp`) testing successful direct invocation, invalid argument rejection, upstream error forwarding, and timeout handling.

## Verification Evidence
- Integration tests: `go test -v ./cmd/mcp-bridge -run '^TestServeHybridDirectPreloadedToolExecution$' -count=1` -> ok (0.039s; Success, InvalidArgument, UnknownTool all pass).
- Both hybrid tests: `go test ./cmd/mcp-bridge -run '^TestServeHybrid' -count=1` -> ok.
- Full suite: `go test ./cmd/mcp-bridge/... ./internal/... -count=1` -> 11 packages ok.
- Coverage gate: `go run ./cmd/covcheck -profile=/tmp/coverage.out` -> 91.4% (3772/4129), gate 90%: true.

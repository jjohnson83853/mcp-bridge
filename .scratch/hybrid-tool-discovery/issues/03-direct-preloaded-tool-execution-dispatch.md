# 03: Direct preloaded tool execution dispatch

**What to build:** An AI client can invoke a preloaded tool directly by its wire name (`<server_id>__<upstream_tool_name>`) via standard MCP `tools/call` without wrapping arguments in a `call_tool` envelope. The bridge validates arguments against the stored schema and forwards execution directly to the upstream server.

**Blocked by:** 02.

**Status:** ready-for-agent

Type: task

Source: Hybrid tool discovery specification, revision HYBRID-DISCOVERY-r1. Coverage: User Stories 10–13, 18–21, 28–30; Implementation Decisions 4, 7.

- [ ] Route direct tool calls matching registered preloaded wire names directly to the owning upstream backend.
- [ ] Validate incoming tool call arguments against the tool's cached `inputSchema` before dispatching. If arguments are invalid, return an MCP error result with `isError: true` without hitting the backend.
- [ ] Acquire an upstream execution lease enforcing configured timeouts, per-server concurrency limits, and circuit breaker health.
- [ ] Forward the raw upstream tool name and arguments to the upstream backend's `tools/call` endpoint.
- [ ] Unpack the upstream response and return the exact content elements to the caller as an MCP `CallToolResult`.
- [ ] Inject upstream credentials securely; verify synthetic secrets never leak into client responses or log output.
- [ ] Emit structured diagnostic logs for direct preloaded executions, recording duration, status, and outcome.
- [ ] Add integration tests at the HTTP seam (`POST /mcp`) testing successful direct invocation, invalid argument rejection, upstream error forwarding, and timeout handling.

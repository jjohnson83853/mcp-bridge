# 02: Assembly catalog and tools/list preloaded exposure

**What to build:** The bridge's MCP `tools/list` response eagerly advertises all preloaded pinned tools with their canonical wire names, upstream descriptions, and complete cached `inputSchema`, alongside the bridge's 3 meta-tools (`search_tools`, `describe_tool`, `call_tool`).

**Blocked by:** 01.

**Status:** complete

Type: task
Source: Hybrid tool discovery specification, revision HYBRID-DISCOVERY-r3. Coverage: User Stories 7–9, 27; Implementation Decisions 3, 6, 7.

- [x] Extend catalog assembly to retain full `ToolDefinition` (including raw `inputSchema` and description) for each resolved pinned tool in the active `Assembly`.
- [x] Ensure startup fails closed if any configured pin does not exist in the corresponding upstream server catalog.
- [x] Update `ToolSurface.Register` and `tools/list` handler to return both the 3 bridge meta-tools and the preloaded pinned tools from the active generation.
- [x] Ensure preloaded tools in `tools/list` carry their namespaced wire name (`<server_id>__<upstream_tool_name>`), upstream description, and complete `inputSchema` without schema truncation or loss of parameter properties.
- [x] Enforce schema byte size bounds on preloaded tools, rejecting or failing closed on oversized schemas.
- [x] Add integration tests at the HTTP composition-root seam (`cmd/mcp-bridge`) verifying that an MCP client calling `tools/list` over `POST /mcp` receives all 3 meta-tools plus the exact preloaded tools with full schemas from local fake upstreams.

## Verification Evidence
- Integration test: `go test ./cmd/mcp-bridge -run '^TestServeHybridPreloadedToolsListExposure$' -count=1` -> ok (0.035s)
- Full suite: `go test ./cmd/mcp-bridge/... ./internal/... -count=1` -> 10 packages ok (2.6s)
- Coverage gate: `go run ./cmd/covcheck -profile=/tmp/coverage.out` -> 91.8% (3728/4062), gate 90%: true

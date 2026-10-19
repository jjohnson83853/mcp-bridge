# 02: Assembly catalog and tools/list preloaded exposure

**What to build:** The bridge's MCP `tools/list` response eagerly advertises all preloaded pinned tools with their canonical wire names, upstream descriptions, and complete cached `inputSchema`, alongside the bridge's 3 meta-tools (`search_tools`, `describe_tool`, `call_tool`).

**Blocked by:** 01.

**Status:** ready-for-agent

Type: task

Source: Hybrid tool discovery specification, revision HYBRID-DISCOVERY-r1. Coverage: User Stories 7–9, 27; Implementation Decisions 3, 6, 7.

- [ ] Extend catalog assembly to retain full `ToolDefinition` (including raw `inputSchema` and description) for each resolved pinned tool in the active `Assembly`.
- [ ] Ensure startup fails closed if any configured pin does not exist in the corresponding upstream server catalog.
- [ ] Update `ToolSurface.Register` and `tools/list` handler to return both the 3 bridge meta-tools and the preloaded pinned tools from the active generation.
- [ ] Ensure preloaded tools in `tools/list` carry their namespaced wire name (`<server_id>__<upstream_tool_name>`), upstream description, and complete `inputSchema` without schema truncation or loss of parameter properties.
- [ ] Enforce schema byte size bounds on preloaded tools, rejecting or failing closed on oversized schemas.
- [ ] Add integration tests at the HTTP composition-root seam (`cmd/mcp-bridge`) verifying that an MCP client calling `tools/list` over `POST /mcp` receives all 3 meta-tools plus the exact preloaded tools with full schemas from local fake upstreams.

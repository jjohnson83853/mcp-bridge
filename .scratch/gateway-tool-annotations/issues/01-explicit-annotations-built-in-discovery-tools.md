# 01 — Explicit Annotations for Built-in Discovery Tools

**What to build:**
Ensure the bridge's built-in discovery tools (`search_tools` and `describe_tool`) explicitly advertise accurate, non-conflicting advisory annotations to connecting MCP clients. Both tools must emit `readOnlyHint: true`, `destructiveHint: false`, `idempotentHint: true`, and `openWorldHint: false` over the HTTP `/mcp` protocol, preventing client-side execution gates from assuming destructive or open-world actions on catalog discovery.

**Blocked by:** None — can start immediately

**Status:** ready-for-agent

- [ ] `search_tools` registration explicitly defines `readOnlyHint: true`, `destructiveHint: false`, `idempotentHint: true`, and `openWorldHint: false`.
- [ ] `describe_tool` registration explicitly defines `readOnlyHint: true`, `destructiveHint: false`, `idempotentHint: true`, and `openWorldHint: false`.
- [ ] HTTP JSON-RPC `tools/list` wire responses return explicit boolean false for `destructiveHint` and `openWorldHint` on both discovery tools.
- [ ] Existing wire test assertions asserting omitted `destructiveHint` or `openWorldHint: true` on built-in tools are updated to assert the new explicit values.
- [ ] Public README table documenting built-in annotations is updated to match.

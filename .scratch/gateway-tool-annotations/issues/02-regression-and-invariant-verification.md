# 02 — Regression and Invariant Verification Across Upstream Metadata Layers

**What to build:**
Comprehensive verification that the explicit annotations configured on `search_tools` and `describe_tool` do not regress or alter upstream tool metadata, progressive reveal session projections, or server default and exact-tool override layers. Ensure generic `call_tool` remains conservatively annotated, and confirm all integration and unit tests pass across the repository.

**Blocked by:** 01 — Explicit Annotations for Built-in Discovery Tools

**Status:** ready-for-agent

- [ ] Upstream tool annotations discovered from remote servers pass through unchanged without synthetic default injection.
- [ ] Server-level default patches and exact-tool override patches (`tool_metadata`) continue to apply correctly on pinned and revealed tools.
- [ ] Generic `call_tool` retains its conservative annotations (`destructiveHint: true`, `idempotentHint: false`, `openWorldHint: true`).
- [ ] Full project test suite (`go test ./...`) passes cleanly with exit code 0.

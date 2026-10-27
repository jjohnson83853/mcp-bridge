# 05: Support exact-tool exceptions over server defaults

**What to build:** An operator of a mixed read/write server can correct individual pinned tools without relabeling their siblings, using predictable upstream-to-server-default-to-tool precedence through reload, refresh, and restart.

**Blocked by:** 04: Apply server-level defaults to pinned tools.

**Status:** ready-for-agent

Type: task
Source: Tool Metadata specification, METADATA-r4. Coverage: R3-R11; AC3-AC11, AC13 (completes combined override scenarios).

Execution prerequisite: required implementation approvals remain separate from the approved ticket breakdown.

- [ ] Support the server-owned tool_metadata.tools mapping keyed by exact raw tool name. Each target must be a configured pin on that same server; allow tool-only patches when no defaults exist, without introducing wildcard or cross-server selectors.
- [ ] Apply fixed precedence from current SDK-normalized upstream metadata to server defaults to exact-tool patch. Use the same supported-field, recursive-overlay, scalar/array replacement, omission, false/empty, and protocol-valid null semantics as server defaults.
- [ ] A mixed read/write fixture demonstrates that an exact write-tool exception overrides a broad server default while sibling reads retain their intended values. Other servers, unpinned tools, built-ins, and generic dispatch remain unchanged.
- [ ] Direct tools/list and describe_tool agree on effective metadata. Normalized upstream values remain separate, and applied field provenance accurately distinguishes server-default from tool-specific corrections.
- [ ] Atomically reject unknown or non-pin targets, invalid field types/keys, and duplicate mappings/selectors; rejected candidates leave the accepted generation intact and emit actionable redacted errors.
- [ ] Removing a tool patch or member restores inheritance from current defaults/source. Removing defaults preserves remaining explicit tool exceptions. Test upstream/default/tool disagreements, explicit false, member removal, and complete entry removal.
- [ ] Actual reload, metadata-only refresh, failed updates, pin/tool/server removal, and restart publish correct precedence without stale correction resurrection or hidden binding-only skips. Source persistence and existing lifecycle/session behavior remain intact.
- [ ] Retain deterministic behavioral RED followed by unchanged GREEN through controlled MCP list/describe wire checks, lifecycle/negative cases, and focused boundary tests where necessary. Descriptor inspection performs no upstream operation.
- [ ] Update operator documentation/changelog with exact-tool configuration, mixed-service examples, precedence, provenance, removal inheritance, and operator assertion risks. Reuse existing failure reporting; add no trust bypass, safety inference, telemetry, or transport changes.

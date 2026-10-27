# 02: Classify bridge-owned tools accurately

**What to build:** Clients inspecting the bridge can distinguish read-only catalog discovery from a generic dispatcher capable of invoking upstream writes, without changing execution or granting trust.

**Blocked by:** None (no technical dependency on storage or upstream metadata preservation).

**Status:** ready-for-agent

Type: task
Source: Tool Metadata specification, METADATA-r4. Coverage: R1, R7-R10; AC1, AC9-AC11, AC13.

Execution prerequisite: required requirements/design/tasks approvals and applicable critic reviews remain unrecorded. Approval of this ticket breakdown authorizes publication only, not implementation.

- [ ] The actual MCP tool list advertises search_tools and describe_tool as read-only and idempotent, with conservative open-world classification; no misleading destructive claim is added to these read-only tools.
- [ ] The generic call_tool descriptor explicitly remains non-read-only, potentially destructive, non-idempotent, and open-world, even when a particular request targets a read-only upstream tool.
- [ ] Wire tests use the existing real-bridge HTTP seam and controlled upstreams. Catalog search/description reads execute zero upstream operations; generic and direct-pin execution behavior remains unchanged.
- [ ] Names, input schemas, authentication, authorization, supported handshake/session behavior, and existing direct-pin descriptors are not altered by this correction.
- [ ] Retain deterministic behavioral RED evidence before production changes and unchanged GREEN evidence afterward; run the focused existing regressions appropriate to this slice.
- [ ] Update operator documentation and the repository's changelog convention to explain the classifications, advisory nature of hints, and why generic dispatch cannot safely be marked universally read-only. Do not enable client/server trust bypass or promise zero Gemini prompts.
- [ ] Keep any existing failure reporting redacted and unchanged; introduce no new telemetry or transport/session policy.

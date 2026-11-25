# 04 — Eliminate bridge-within-bridge discovery recursion

**Type:** research
**Priority:** P1
**Status:** ready-for-agent
**Blocked by:** None — can start immediately.

**What to build:** Determine why a production client reached `search_tools`/`describe_tool` only by calling `call_tool` against an indexed bridge tool, which wrapped every result in a nested untrusted-content envelope, and correct the responsible layer. The delivered outcome is a verified path where the client invokes the bridge’s own discovery tools directly, sees each result once, and does not pay a second wrapper or quoting layer. Either the client configuration is corrected and documented, or a bridge-side defect is identified with evidence and routed to a repair ticket.

**Notes:** This repository already registers the three discovery tools as first-class server tools; the recursion is therefore a routing, deployment, or client-version question. `call_tool` may legitimately invoke any indexed upstream tool, including another bridge, so do not remove that capability to suppress the symptom. Treat untrusted-content marking as a deliberate boundary: preserve it where the content genuinely crosses a trust boundary, and record exactly where it was redundant.

- [ ] The reported session’s discovery path is traced to a specific client, configuration, or bridge version, with evidence cited from deployed configuration or client tool declarations.
- [ ] The root cause is stated as a fact or explicitly marked unverified with the missing artifact named.
- [ ] A reproduction exists for the recursive path and for the corrected path.
- [ ] The corrected path is demonstrated end to end: direct discovery invocation, single result envelope, no nested bridge envelope.
- [ ] Any client/configuration fix is documented where operators configure this bridge.
- [ ] If a bridge defect is confirmed instead, the finding names the exact component and the follow-up repair work; this ticket does not silently widen into that repair.

# 03 — Explicit stateful mode and legacy-protocol boundary are deterministic

Type: task
Status: ready-for-agent

**Parent:** `.scratch/mcp-bridge-defects/backlog.md` MB-001.

**What to build:** The same header-bound tool succeeds when an operator explicitly configures the server as stateful, and an upstream that negotiates a protocol version which cannot carry parameter headers produces a named, actionable failure instead of a generic tool or protocol error.

**Blocked by:** 02 — the emission mechanism chosen there determines the stateful and legacy paths.

- [ ] With `stateless: false`, the header-bound tool succeeds end-to-end and its upstream receives the parameter values.
- [ ] When the negotiated protocol version cannot carry parameter headers, the bridge answers with a specific, payload-free failure code (or an explicit startup/discovery rejection) rather than `tool_reported_error` or `upstream_protocol_error` with no explanation.
- [ ] The documented behavior states which protocol versions carry parameter headers and what an operator must do for older upstreams.
- [ ] Tests cover the explicit-stateful success path and the legacy-protocol branch, and assert the named code rather than message text.

# 01 — Reproduce MB-001 against a header-annotated peer and capture the wire

Type: research
Status: ready-for-agent

**Parent:** `.scratch/mcp-bridge-defects/backlog.md` MB-001 (header-bound tool parameters do not traverse the bridge).

**What to build:** A deterministic, live-free reproduction of the MB-001 failure against a controllable peer, with the transport facts captured. The reproduction must show that a tool declaring `x-mcp-header` parameters fails through normal `call_tool` in default configuration while a body-only tool on the same peer succeeds, and must record what the bridge actually sent.

**Blocked by:** None — can start immediately.

- [ ] An integration test drives the bridge's own Tool server against a peer that advertises the protocol version required for parameter headers, registers one tool with header-bound parameters (`owner`, `repo`) and one body-only control tool.
- [ ] In default stateless mode, the header-bound call fails and the control call succeeds in the same window; the test fails for that reason (RED), and the failure does not depend on the live github upstream.
- [ ] The test records, for the failing call: negotiated protocol version, presence or absence of `Mcp-Param-owner` and `Mcp-Param-repo`, and the outbound arguments body.
- [ ] The same probe runs with `stateless: false`, so an absent tool-definition cache is distinguishable from a protocol version that cannot carry parameter headers.
- [ ] The recorded evidence states explicitly which of the two mechanisms was missing for each mode; no inference is presented as observed.

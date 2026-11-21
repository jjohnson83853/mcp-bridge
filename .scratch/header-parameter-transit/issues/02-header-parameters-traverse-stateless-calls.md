# 02 — Header parameters reach a header-bound upstream in default stateless mode

Type: task
Status: ready-for-agent

**Parent:** `.scratch/mcp-bridge-defects/backlog.md` MB-001.

**What to build:** A client calling a tool whose schema marks parameters `x-mcp-header` gets a successful result through the bridge's normal `call_tool` path in default configuration. The upstream receives those values through the mechanism its negotiated protocol requires, while the bridge keeps its stateless default and its restart resilience.

**Blocked by:** 01 — the reproduced failure and captured wire facts define the fix.

- [ ] With default configuration, the header-bound tool used by ticket 01 succeeds end-to-end and returns its upstream result; the header values arrive in the form the peer requires.
- [ ] The body-only control tool on the same peer keeps its previous behavior, including headers and result content.
- [ ] No persistent upstream session is pinned to keep the fix working; the existing restart-resilience behavior for the default stateless mode still passes.
- [ ] Regression tests cover the header-bound call, the control call, and the restart case; the tests fail on the pre-fix build and pass after it.
- [ ] Failure paths for the header call remain payload-free and use the existing safe code vocabulary.

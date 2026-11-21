# 04 — Deployed github upstream verified end-to-end

Type: task
Status: ready-for-agent

**Parent:** `.scratch/mcp-bridge-defects/backlog.md` MB-001.

**What to build:** The shipped build proves on the live deployment that the original symptom is gone: the header-bound github tool returns content while the query-only control tool on the same server still works.

**Blocked by:** 03 — verification runs against the completed fix.

- [ ] `github__get_file_contents(owner=jjohnson83853, repo=mcp-bridge, path=README.md)` returns file content, not a tool error or connection closure.
- [ ] `github__search_repositories(query=mcp-bridge)` succeeds in the same observation window.
- [ ] Bridge structured logs for the header call show no `tool_reported_error`, and the catalog revision used is recorded.
- [ ] The verification names the deployed image and container identity, and states whether a restart or reload was used.

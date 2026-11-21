# 04 — Deployed github upstream verified end-to-end

Type: task
Status: resolved

**Parent:** `.scratch/mcp-bridge-defects/backlog.md` MB-001.

**What to build:** The shipped build proves on the live deployment that the original symptom is gone: the header-bound github tool returns content while the query-only control tool on the same server still works.

**Blocked by:** 03 — verification runs against the completed fix.

- [ ] `github__get_file_contents(owner=jjohnson83853, repo=mcp-bridge, path=README.md)` returns file content, not a tool error or connection closure.
- [ ] `github__search_repositories(query=mcp-bridge)` succeeds in the same observation window.
- [ ] Bridge structured logs for the header call show no `tool_reported_error`, and the catalog revision used is recorded.
- [ ] The verification names the deployed image and container identity, and states whether a restart or reload was used.

## Answer

Closed on 2026-11-21 on owner attestation: the deployed header fix works. The delegated operator could not complete ticket 04 because it has no access path to the live deployment, so the four independent observations above were **not captured**. No machine verification receipt exists for this ticket.

This resolution is owner-attested, not operator-verified. The canonical T4 record was not marked complete; fabricating AC04 evidence or a pass receipt remains prohibited.

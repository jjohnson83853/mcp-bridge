# 07 — Catalog resource gauge

**Type:** task

**What to build:** Resource discovery becomes as observable as tool discovery: per server, the bridge reports how many static resources and how many templates it currently serves, so an upstream that silently stops exposing resources is visible as a drop to zero.

**Blocked by:** `resource-passthrough/01`.

**Status:** ready-for-agent

- [ ] `bridge_catalog_resources{server, kind}` is exported with `kind` in the closed set `static` and `template`.
- [ ] Values are updated on the same refresh lifecycle as `bridge_catalog_tools`: set on a complete refresh, zeroed or marked consistent with the existing empty/failed-refresh semantics used by the tool gauge.
- [ ] Resource names, titles, and URIs never enter labels; `server` uses the bounded server set.
- [ ] A server that exposes zero resources reports 0 rather than leaving a stale non-zero value.
- [ ] Tests cover refresh success, empty catalog, refresh failure, and label refusal.
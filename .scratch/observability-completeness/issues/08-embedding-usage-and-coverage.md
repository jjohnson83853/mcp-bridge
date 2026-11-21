# 08 — Embedding usage and coverage metrics

**Type:** task

**What to build:** Hybrid search either uses embeddings or degrades, and that must be measurable. Request counters split document from query embeddings and success/failure/cache-hit, while a per-server coverage ratio shows how much of the index actually has a current vector.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] `bridge_embedding_requests_total{server, kind, outcome}` is exported with `kind` in `document` and `query`, and `outcome` in `success`, `failure`, `cache_hit`.
- [ ] `bridge_embedding_coverage_ratio{server}` reports the fraction of that server's current indexed rows holding a vector valid for the live model identity, in `[0, 1]`.
- [ ] Coverage is recomputed on index changes, catalog refresh, and model-identity change; a model change drops stale coverage until rows are re-embedded.
- [ ] A failed embedding request increments `failure` and never raises coverage; a cached vector counts as `cache_hit` and as covered.
- [ ] `server` and both label sets are validated against closed vocabularies and the bounded server set.
- [ ] Tests cover a forced provider failure lowering coverage, a cache hit, a model change, and label refusal.
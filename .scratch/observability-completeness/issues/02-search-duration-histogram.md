# 02 — Search duration histogram with degradation mode

**Type:** task

**What to build:** Search latency is observable and its quality degradation is not silent: every `search_tools` call reports its duration under the mode it actually used, so an embedding endpoint stall shows up immediately as `bm25_fallback`.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] `bridge_search_duration_seconds{mode}` is exported with buckets `0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5`.
- [ ] `mode` is a closed set of `hybrid`, `bm25_fallback`, and `cache_hit`, and is reported truthfully: provider failure, timeout, or unavailable embeddings produce `bm25_fallback`, never `hybrid`.
- [ ] A response served from the search cache is observed as `cache_hit`; a recomputed hybrid search is never mislabelled as cached.
- [ ] One observation per `search_tools` call, including failures, with no observation for requests rejected before search begins.
- [ ] Tests cover the fallback path under a forced embedding failure, the cache-hit path, and rejection of an unknown mode label.
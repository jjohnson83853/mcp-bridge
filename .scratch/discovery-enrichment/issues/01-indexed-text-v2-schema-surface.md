# 01 — Indexed text v2: schema surface in the search document

**Type:** task

**What to build:** Discovery stops depending on prose descriptions. The text that BM25 and embeddings see for each tool gains its input-schema surface — top-level property names, enum values, and property descriptions — so `"logql"`, `"vmid"`, or `"entity_id"` retrieve the right tool even when the description never says it.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] Indexed text includes raw name, description, top-level input property names, enum values, and property descriptions, bounded by a configurable character budget per document.
- [ ] The content identity of the indexed text is versioned (a new formatter identity); every cached vector built from v1 text is invalidated and re-embedded before it can be served again.
- [ ] Existing vector-reuse semantics are preserved for v2: a schema-only change that does not alter indexed text keeps the vector; row identity still tracks the exact definition digest.
- [ ] The `tools/list` wire payload is byte-identical to today apart from no intentional change — enrichment is search-only.
- [ ] A pathological schema (deep nesting, huge enums) cannot exceed the budget or grow index memory without bound.
- [ ] Tests cover a query that only matches a property name, an enum-only match, and confirmation that the client-facing descriptor did not grow.
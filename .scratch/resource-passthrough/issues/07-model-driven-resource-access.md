# 07 — Model-driven resource access: `search_resources` and `read_resource`

**Type:** task

**What to build:** Agents that only execute tools can still discover and fetch reference content. Static resources and resource templates are indexed into the existing hybrid search, and two built-in read-only tools expose them.

**Blocked by:** 01, 03, `discovery-enrichment/03`.

**Status:** ready-for-agent

- [ ] Static resources and templates are indexed with server, name/title, description, `mimeType`, and declared template variable names, refreshed with the resource catalog.
- [ ] `search_resources` accepts a query, an optional server allowlist, and a bounded limit, and returns ranked entries carrying kind (`static`|`template`), bridge URI, title/description, owning server, and score-derived ordering.
- [ ] `read_resource` accepts a bridge URI only and returns the same content a `resources/read` call would; it refuses raw upstream URIs and unknown servers with a clear error.
- [ ] Both tools declare `readOnlyHint: true` and correct destructive/open-world hints under the existing gateway annotation conventions.
- [ ] Eligibility matches tools: resources of an unready or disabled server are never returned.
- [ ] Embedding outage degrades resource search to BM25 without failing the tool call.
- [ ] Resource documents and vectors use versioned content identity, so a changed resource re-embeds and a removed resource loses its vector.
- [ ] Tests cover a natural-language query matching a template, `read_resource` round trip, and BM25 fallback.
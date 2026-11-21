# 01 — Namespaced resource discovery and `resources/list`

**Type:** task

**What to build:** Every ready server's static resources are discovered during the catalog refresh cycle and served from the bridge as a single union with deterministic, collision-free bridge URIs. Two upstreams both exposing `file:///README.md` must appear as two distinct resources, each resolving to exactly one upstream and its original raw URI.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] `resources/list` returns resources only from servers whose catalog is ready; an unreachable or failing server contributes none and the rest still serve.
- [ ] The bridge URI is a pure function of (server id, raw URI), stable across restart, reload, and process migration, and decodes losslessly for percent-encoded, Unicode, and multi-segment URIs.
- [ ] `name`, `title`, `description`, `mimeType`, `size`, `icons`, and `annotations` (`audience`, `priority`, `lastModified`) survive the round trip unchanged.
- [ ] Discovery is bounded per server (page cap and total cap); a hostile or oversized upstream listing cannot exceed the bound or abort the whole refresh.
- [ ] Discovered resources persist in the durable catalog, so a restart serves the last-known resource set before the first refresh completes.
- [ ] A per-server resources opt-out exists, consistent with the existing prompts-disable convention, and a disabled server's resources are absent from the list and unreadable.
- [ ] Tests cover: cross-server URI collision, encode/decode round trip, partial refresh failure, bound enforcement.
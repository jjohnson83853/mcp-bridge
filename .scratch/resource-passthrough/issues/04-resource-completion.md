# 04 — Argument completion for resource references (`completion/complete`)

**Type:** task

**What to build:** When a client asks for argument completion against a resource or resource template reference, the bridge routes it to the server that owns the referenced bridge URI, so clients get live entity IDs, note slugs, and similar values.

**Blocked by:** 02.

**Status:** ready-for-agent

- [ ] `completion/complete` with `ref.type == "ref/resource"` resolves the bridge URI to its owning server and forwards the argument name and value.
- [ ] Variables declared by the template are offered for completion; the bridge adds no invented variables or values.
- [ ] Completion results are bounded and merged deterministically when a template spans multiple values; ordering matches upstream.
- [ ] A server without completion support, an unknown reference, or a reference to a disabled server yields an empty completion result, not an error.
- [ ] Upstream protocol or transport failure surfaces as an MCP error distinct from "no matches".
- [ ] Tests cover template argument completion, unsupported upstream, unknown reference, and upstream failure.
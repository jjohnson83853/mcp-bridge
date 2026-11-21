# 07 — Schema normalization for model comprehension

**Type:** task

**What to build:** `describe_tool` and progressive reveal present schemas models can fill on the first attempt: local `$ref`s inlined, `anyOf: [{type: "string"}, {type: "null"}]` collapsed to a nullable string, pointless wrappers removed — with upstream validation unchanged.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] Presented schemas inline local `$ref` definitions, collapse nullable `anyOf` to a single type with nullability, and simplify trivially empty `anyOf`/`oneOf` compositions, under bounded recursion and output size.
- [ ] Normalization is presentation-only: the original schema remains the sole basis for upstream validation and dispatch, byte-identical to what the upstream published.
- [ ] Constructs the normalizer does not understand are passed through untouched rather than dropped or approximated.
- [ ] A cycle in `$ref` recursion or an oversized `$defs` set degrades to the original schema for that tool instead of failing the request.
- [ ] Tests cover a FastMCP-style optional field, a `$defs`/`$ref` chain, an invalid payload still rejected, and a cyclic schema falling back unchanged.
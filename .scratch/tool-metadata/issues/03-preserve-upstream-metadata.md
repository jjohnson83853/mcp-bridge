# 03: Preserve upstream metadata end to end

**What to build:** A client receives each backend's supported tool metadata consistently through direct pinned discovery and describe_tool, including after metadata refresh, outages, durable restore, and restart, instead of losing metadata at the bridge.

**Blocked by:** 01: Resolve storage, session, and reporting prerequisites.

**Status:** ready-for-agent

Type: task
Source: Tool Metadata specification, METADATA-r4. Coverage: R2, R3, R5-R11; AC2, AC3, AC5-AC11, AC13.

Execution prerequisite: required implementation approvals and the relevant evidence-backed design remain prerequisites. Do not change the parent specification or invent a storage migration to bypass a compatibility finding.

- [ ] Carry all metadata represented by the pinned SDK through discovery, immutable catalog generations, compatible durable records, restore, and client-visible descriptors: description, title, icons, supported annotations, and arbitrary supported nested _meta values. Include the existing output schema on direct pinned descriptors.
- [ ] Integrate the shared immutable metadata representation and necessary ownership-boundary changes into this complete behavior; do not leave an ingress-only representation or a second compatibility path.
- [ ] Preserve annotation-object absence where represented and destructive/open-world nil, false, and true distinctions. Disclose SDK normalization of scalar read-only/idempotent omission and false without claiming original key-presence fidelity or changing SDK/environment behavior.
- [ ] Direct tools/list and describe_tool agree on effective supported metadata. Describe also exposes separately identified SDK-normalized upstream metadata and empty correction provenance when no overrides apply. Upstream claims are not rewritten using tool-name heuristics.
- [ ] Metadata-only upstream changes and removals advance accepted descriptor state and appear on same-session explicit re-list without requiring a schema/routing change. Removed tools, pins, or servers disappear through existing reconciliation; no universal push/cache-invalidation guarantee is added.
- [ ] Controlled upstream failure retains complete last-known-good metadata and honest stale/failure status while healthy peers advance. Rejected persistence or reload cannot publish a partially admitted candidate, and existing errors remain actionable and redacted.
- [ ] Prove evidence-approved legacy record compatibility, metadata round-trip integrity, restart behavior, and existing delayed startup pin exposure. No fabricated legacy annotations or discarded safety-significant false values are introduced.
- [ ] Preserve canonical and opaque identities, routing, schemas, authentication/authorization, results, and upstream initialization/transport/session contracts across multiple controlled backends.
- [ ] Retain behavioral RED followed by unchanged GREEN at the existing MCP HTTP seam, with focused serialization/compatibility cases only where needed. Descriptor reads execute no upstream operation; no live mutable backend is used.
- [ ] Update operator documentation/changelog with supported metadata inventory, normalization, source/effective provenance, refresh/restart behavior, and limits. Reuse existing reporting without new telemetry.

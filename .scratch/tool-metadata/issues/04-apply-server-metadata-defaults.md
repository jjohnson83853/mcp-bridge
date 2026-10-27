# 04: Apply server-level defaults to pinned tools

**What to build:** An operator can configure metadata defaults for any server's pinned tools and see those corrections consistently in direct discovery and describe_tool, while retaining the backend's source claims and safely removing corrections later.

**Blocked by:** 03: Preserve upstream metadata end to end.

**Status:** ready-for-agent

Type: task
Source: Tool Metadata specification, METADATA-r4. Coverage: R3-R11; AC3-AC11, AC13 (server-default behavior).

Execution prerequisite: required implementation approvals remain separate from ticket publication. This slice implements complete defaults behavior; exact-tool exceptions are delivered by ticket 05.

- [ ] Accept the optional server-owned tool_metadata mapping with a defaults patch, using the specification's supported description/title/icons/annotations/_meta field contract. Existing configurations remain valid; routing, schemas, authentication, initialization identity, and credentials are not override targets.
- [ ] Derive effective pinned metadata from current SDK-normalized upstream values followed by that server's defaults. Recursively overlay supplied object members, replace arrays/scalars as specified, distinguish absent fields from false/empty values, and do not introduce a null-as-deletion language.
- [ ] Publish the same effective values in direct tools/list and describe_tool, with upstream metadata unchanged and field provenance identifying server-default corrections. Persist source data, never effective defaults as upstream truth.
- [ ] Defaults apply only to that server's configured pins. Prove isolation from another server sharing raw tool names, unpinned descriptors, built-ins, and generic call_tool. A server with no pins may retain defaults without affecting unpinned tools.
- [ ] Atomically validate supported keys/types and duplicate mappings before admitting a candidate. Invalid defaults reject the entire candidate, retain prior accepted descriptors, and produce bounded redacted errors rather than payload echoes.
- [ ] Real reload adds, changes, and removes defaults or members; completed publication updates same-session re-list despite unchanged binding/schema. Removal inherits current upstream values, not previous effective values.
- [ ] Refresh, rejected updates, pin/server removal, and restart retain the correct source/default distinction without stale claims or departed exposure returning. Keep actual session/notification behavior and existing failure reporting.
- [ ] Retain deterministic RED followed by unchanged GREEN, controlled list/describe exchanges, applicable negative/lifecycle regressions, and zero real-upstream mutations.
- [ ] Update operator documentation/changelog with defaults configuration, removal/inheritance, provenance, and the risk that server-wide assertions can misclassify mixed read/write tools. Do not install automatic read-only defaults, infer exceptions, or enable trust bypass.

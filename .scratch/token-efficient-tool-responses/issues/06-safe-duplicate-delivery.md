# 06 — Duplicate result reuse without suppressing execution

**Type:** task
**Priority:** P2
**Status:** ready-for-agent
**Blocked by:** 01 — Bounded tool results with recoverable paging.

**What to build:** When an identical tool invocation is repeated and the newly produced result is byte-identical to one already delivered in the same session, the bridge returns a short reference to the earlier result plus a small preview instead of re-sending the full payload. The earlier content remains retrievable through the ticket 01 retrieval mechanism, and the reference is a bridge-issued identifier rather than a client transcript identifier. Execution is never skipped: the upstream call still happens and its side effects are unaffected, so the optimization cannot change behavior for mutating tools.

**Notes:** Identity is the exact (server, tool, arguments, catalog revision) tuple plus the produced result bytes, not arguments alone; identical arguments with a changed result must deliver the new result. Comparison happens after execution, so no call is elided. Handle or reference lifetime must survive ordinary context compaction but expire under the same policy as ticket 01, with an explicit expired error. Results that were already paged are not re-collapsed into another layer of indirection.

- [ ] A repeated invocation whose result is byte-identical returns a reference plus bounded preview, and the response says the result was already delivered.
- [ ] The upstream call still executes for every invocation, including mutating tools; only delivery is deduplicated.
- [ ] A repeated invocation with identical arguments but a different result delivers the new full result and does not serve the stale one.
- [ ] References are session-scoped, unguessable, and rejected for other sessions.
- [ ] The referenced content is retrievable in full through the existing bounded retrieval path, and an expired reference fails with a distinct typed error.
- [ ] Results that were themselves paged are not wrapped in a second layer of indirection.
- [ ] Tests cover: byte-identical reuse, changed-result delivery, mutation execution count, cross-session rejection, expiry, and non-nesting with paging.

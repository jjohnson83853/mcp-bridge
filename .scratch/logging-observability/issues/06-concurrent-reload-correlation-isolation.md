# 06: Keep concurrent and reloaded request chains isolated

**What to build:** Operators can reliably attribute concurrent tool completions and response shapes to their original requests, even when clients reuse public IDs, calls finish in reverse order, and a catalog reload overlaps in-flight work.

**Blocked by:** 04 — Diagnose upstream and client-facing result shapes.

Status: complete

Type: task

Source: Logging observability specification, revision LOG-OBS-r4 (supersedes the original r1 source reference; approved ticket scope and dependencies unchanged). Coverage: AC9, concurrent/reload portions of AC8 and AC22; relevant AC5, AC11, AC14–AC21 and AC24. Ticket publication approval does not authorize implementation; applicable project gates still apply.

- [ ] Two simultaneously admitted requests deliberately reuse the same valid lowercase-hex public X-Request-ID and JSON-RPC request ID, using isolated valid sessions when required. Distinct fake-upstream barriers force reverse completion. Their internal request/operation/backend chains remain disjoint and attach to the correct upstream completion and final shape.
- [ ] Caller request IDs, JSON-RPC IDs, session IDs and public envelope IDs are neither emitted as internal correlation nor used as global registry keys. Existing public ID acceptance/return behavior is unchanged. Multiple operations on a valid transport retain distinct operation identities and the correct parent chain.
- [ ] A catalog publication overlaps blocked in-flight calls. Each record retains the admitted binding/configured server, appropriate original generation and original operation context; later calls use the newly admitted publication. Immutable approval sets do not mix names across generations or expose stale caller-submitted metadata. Do not change reload, catalog revision, leasing, draining or dispatch policy to make logging simpler.
- [ ] The real HTTP/MCP composition-root harness verifies both rejected and accepted reload behavior already guaranteed by the parent contract. Captured record joins, distinct upstream barriers and meaningful shape variants prove attribution without relying on timestamps, completion order alone or loose substring matches. No new production test interface or duplicate test-only composition root is introduced.
- [ ] Record counts demonstrate one terminal event per actual boundary and one backend record per actual dispatch, with no retry or duplicate caused by concurrency. Shared approval/correlation state has no unbounded registry or avoidable payload copy; relevant concurrent/reload tests pass under the race detector.
- [ ] Synthetic secrets in reused caller IDs, concurrent arguments, hostile identity values, reload failure text and nested result content are absent from all captured output. Existing diagnostics, metrics labels/scrape policy, authentication/session rules, public responses, networking and LibreChat-facing contracts remain unchanged.
- [ ] Update operator documentation/changelog with concurrent correlation and generation interpretation. Retain deterministic RED/GREEN evidence for any newly corrected observable behavior; already-passing negative safety cases remain regression evidence rather than fabricated RED. Smoke the actual local concurrent/reload path, run focused/full-suite/race checks with at least 90% full-suite source-line coverage, and complete applicable code/Go/security review. Do not claim actual LibreChat rendering, deployed collector delivery or retention verification.

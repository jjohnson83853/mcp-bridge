# 09 - Pin Blocker Detail

Status: ready-for-agent
Blocked-by: 08

## What to build
A failed pin check names every blocking pin, not just the first. Reporting one pin per attempt forces a fix-edit-restart cycle per pin, which is the loop this ticket removes. Each unresolved mandatory pin is reported with its safe server and tool identity and its configuration source location - the pin's `pins[i]` property path - so the operator edits the file directly instead of searching it.

Not every unresolved pin is a missing tool. A pin whose server discovery failed is unverifiable, and the record says so while naming the shared discovery cause; only a successful, complete tool listing may establish that a named tool is absent. For each server that has a missing pin, the record also lists the tool names actually discovered on that server, so the correct replacement name is visible in the same log line; large lists are split across bounded records that share a refresh ID, server ID and chunk index and count, with no silent truncation.

Every failure states whether the server blocks publication - an unavailable server carrying a mandatory pin rejects the candidate - or leaves a degraded candidate publishable. All evidence in one failure is correlated to a single attempt and a single configuration revision; results are never mixed with an older catalog generation.

## Acceptance Criteria
- [ ] AC09-1 (09/AC-1; R21, R25): several unresolved mandatory pins → ALL reported via `pin_blocker_detail` (AD18), each with server and tool identity, not only the first; every such record carries `level` `warn` per AD18/ruling 16 — a `pin_blocker_detail` emitted at any other level (in particular `debug`) fails this criterion. test.
- [ ] AC-2: Each reported pin carries its configuration source location: the pin's `pins[i]` property path.
- [ ] AC-3: A pin whose server discovery failed is reported as unverifiable with the discovery cause, never as tool-absent.
- [ ] AC-4: A successfully discovered server that lacks the named tool is reported as tool-absent.
- [ ] AC-5: For each server with a missing pin, the actual discovered tool names are reported; large lists are split across records carrying a shared refresh ID, server ID and chunk index/count, each chunk record carrying at most 64 names (this AC's own oracle bound, asserted per chunk record) so `chunk_count` ≥ 2 whenever a listing exceeds 64, with no silent truncation.
- [ ] AC-6: Discovered tool names are listed only for servers with missing pins, never for healthy servers.
- [ ] AC-7: Each failure states whether it rejected the candidate or left a degraded candidate publishable.
- [ ] AC-8: All evidence in one failure correlates to one attempt and one configuration revision; no result mixes catalog generations.

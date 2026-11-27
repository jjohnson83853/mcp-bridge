VERDICT: REQUEST CHANGES

Subject: specification mode, initial pass on `.scratch/token-efficient-tool-responses/spec.md` (r30-replacement, on-disk bytes). Read-only; no build/test executed — findings are document-vs-repo claims.

## Findings

**[HIGH] F1 — R7 retirement triggers are incomplete and one is mislabeled; AC43 inherits the gap.**
R7 says retirement has "exactly two production triggers … expiry reaping … and an epoch advance retiring the live owner (inbound.go:802-812, epochs advancing at projection.go:189 and :201)". The repo has three production retirement call sites on the inbound path: `retireLiveOwnerAtInboundLock` at `internal/service/inbound.go:1144` (inside `reapExpiredLocked`, the reaping case), at `:808` (the `missingSession` branch — not an epoch advance), and at `:962` (HTTP `DELETE` session termination). `projection.go:189/201` only increment `nextEpoch` while *creating* owners; `record.epoch == 0` gates the only `newLiveOwner` installs (`inbound.go:609/850/921`), so no epoch advance ever retires an existing owner. Consequence: a session terminated by `DELETE` never retires its retention namespace — no tombstone is installed, entries and reservations stay charged until `handle_ttl`, and the "post-epoch-bump" rejection class (R4) is produced by exactly that delete/re-create path, so the namespace producing it is the one left unretired. AC43's "both trigger paths" is not a completeness oracle. Fix: enumerate all three inbound call sites in R7, add the `DELETE` case to AC43, and either wire the sweep at each call site (T3's only inbound write root) or add `projection.go`'s shared retire seam (`:863/:877`) as a write root.

**[MEDIUM] F2 — R8's truncation rule is not total under an admitted config; R11 has no lower bound for `max_upstream_code_bytes`.**
R8 claims the rule "selects exactly one prefix for every input", measuring the escaped prefix plus the escaped literal `[truncated]` (11 bytes) against `max_upstream_code_bytes`. R11(f)/AC34 admit any value `> 0`. For `max_upstream_code_bytes ≤ 11` no prefix satisfies the inequality, so the rendered `upstream_code` is undefined (empty field, marker-less, or over-bound), and no AC covers it. Fix: add relation (k) `max_upstream_code_bytes ≥ 11` in R11 with its own typed diagnostic and a T1 AC, or state the degenerate rendering (e.g., field omitted) explicitly.

**[MEDIUM] F3 — Contradictory severity for the vocabulary-violation record, unoracled.**
R12 pins the replacement record to "original level"; §7 pins "terminals, rejections and vocabulary violations warn". For any non-warn offending record these disagree, and AC13(b) never asserts `level`, so neither reading can fail its oracle. Fix: pin `level: warn` in R12 and §7 and add the assertion to AC13(b).

**[LOW] F4 — Event-name registry requirement lives outside R12, and §7 overstates its oracle.**
The closed `eventMessage` registry and its violation behaviour appear only in §7, while AC13's `Covers` is R12, which defines only the `operation` registry; §7's "the registry equals the emitted set is asserted by AC13's runtime oracle" is stronger than AC13(a) (equality with a frozen list) plus AC13(c) (two producers exercised). Fix: state the event registry in R12 (or a new requirement) and correct the equality claim, or add producer coverage.

## Verified invariants (present/traceable, no finding)
- Independent byte oracle: R1/R2 + literal-byte ACs AC1, AC2, AC6, AC40 ("not a retained artifact, not re-marshaled").
- Retired-marker lifetime/accounting: R7/R16 half-open rule applied verbatim by AC5/AC8/AC18/AC39, fixed 256-byte charges, born-dead-marker clause — sound apart from F1.
- Pinned `upstream_code`: R8/R9 + AC9/AC10/AC29 (serialized-length assertion, `[truncated]` inside bound) — apart from F2.
- Finite labels with producers: R12's 31 values match the 18 in `internal/observe/httpdiagnostics.go:660-664` + 11 verified emitters + 2 new; AC13/AC44 are handler-free.
- Per-terminal bounds: R10 sizing with R11(g)'s pinned widths + AC11/AC26.
- Ceiling order: R11(a) + AC12.
- Task write roots name existing files (`validate_edge_test.go`, `builtin_guidance_test.go`, `logging_contract_test.go`, etc.), and README anchors `:277/:325/:441/:468` and inbound `:24/:522/:676/:767/:770` were locatable.

Positive: the single-sourcing discipline (R3/R5/R10 sets, R11(g) referenced not restated) and the golden-literal oracles are materially stronger than self-referential tests.

### Recommendation
REQUEST CHANGES
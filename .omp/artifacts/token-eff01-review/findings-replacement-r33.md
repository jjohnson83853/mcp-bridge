VERDICT: REQUEST CHANGES

**Subject:** specification mode, initial pass on `.scratch/token-efficient-tool-responses/spec.md` (r33-replacement, on-disk bytes). Read-only: I ran no commands, so I did not recompute the packet digest or execute any oracle.

The r32 F1–F4 closures hold: §7 now pins a field set for all twelve events (explicitly “none” for the four rejections), T3 says “all three R7 trigger paths,” T5 names the file hosting `TestReadmeInventoryAndPromise`, and R10 pins the terminal-evaluation order. The 31-value arithmetic (18+11+2), `eventMessage` 24 / pin-map 23 → 36, both README anchors, and the `config-codes` block in `docs/logging-contract.md` are consistent with the tree.

### Findings

**[MEDIUM] R33-F1 — the naturally cleared marker’s aggregate release has no oracle.**
Sections: §2 R16; §4 AC5, AC39. Continuation of r21-F2.
R16 promises that at `now ≥ transition + tombstone_ttl` a marker “is cleared, its 256-byte charge is released.” AC5 asserts only the retrieval code across that deadline; AC39’s exact before/after aggregate delta is expressly the *cap-eviction* commit (“each cap eviction releases its 256-byte charge at the evicting commit”); AC8 supplies the analogous delta oracle only for tombstones. Behavior did not change since r21; the prior “sound” verdict overstated AC39’s scope, which I re-inspected against AC5/AC8/AC39.
Consequence: an implementation that clears the marker’s retrieval state but leaks its 256 bytes passes every AC; repeated expiry leaks aggregate capacity until a legitimate entry commit is spuriously refused with `result_aggregate_capacity_exceeded`.
Fix: extend AC5 or AC39 with an injectable-clock before/after aggregate assertion at the half-open deadline, plus one entry commit that needs the freed 256 bytes and is admitted before any sweep — mirroring AC8.

**[MEDIUM] R33-F2 — AC43 asserts a tombstone unconditionally, contradicting R7/AC38.**
Sections: §4 AC43 vs §2 R7/R6 and §4 AC38.
AC43 says that after any of the three inbound triggers “the tombstone is counted in accounting per R7” and the namespace is retired. R7 and AC38 require an aggregate-infeasible retirement to remain un-finalized: “no tombstone is installed or counted, nothing from the attempt persists,” retried by a later sweep. AC43 carries no feasibility qualifier.
Consequence: under aggregate pressure a DELETE/expiry/missing-session trigger cannot satisfy both ACs; forcing tombstone installation violates R7 and the hard aggregate bound, while deferring fails AC43 as written.
Fix: qualify AC43 with “when the tombstone is admissible per R7; an infeasible retirement stays un-finalized per AC38 and is retried.” Keep its `invalid_handle` assertion, which follows from owner binding after the session is deleted.

**[LOW] R33-F3 — AC38’s handle-resolution clause is unobservable at the only handle-accepting surface.**
Sections: §4 AC38 vs §2 R4 and §3 owner binding.
AC38 says an un-finalized retirement leaves handles “resolving by entry/marker state.” Every R7 trigger deletes the session before retiring, so a later `retrieve_result` request carries no stamped epoch (or a new epoch) and fails binding into `invalid_handle` regardless of entry/marker state; the clause is true only at the T2 store API with the original owner.
Consequence: a handler-level reading demands retrievability through a deleted owner, contradicting R4/§3’s no-epoch binding rule and session isolation.
Fix: state AC38 is a store-level assertion with the original owner, or replace the clause with “the store still returns the typed cause for that owner; handler lookups fail binding per R4.”

**[LOW] R33-F4 — `handle_present` is permitted and documented but excluded from every event’s exact field set.**
Sections: §7 preamble and Events; T2’s `docs/logging-contract.md` field vocabulary; §4 AC13.
§7 says “only `handle_present` may appear” in place of a handle and T2 documents it, yet all twelve exact field sets omit it and AC13 fails any “extra fields.”
Consequence: an implementer following §7 can emit it and fail AC13; one following AC13 documents a field with no producer.
Fix: remove `handle_present` from §7/T2, or add it to one named event’s exact set with a producer assertion.

### Carried invariants

Independent byte oracle (R1/R2; AC1/2/6/37/40), pinned `upstream_code` (R8/R9; AC9/10/29), finite label registries with producers (R12; AC13/AC44), per-terminal preview bounds (R10; AC11/26), and ceiling-order validation (R11(a); AC12) are present and falsifiable. Retired-marker lifetime is sound; marker *accounting* is incomplete at natural clearing (R33-F1).

Task write roots check out: T1’s `internal/config/validate_edge_test.go` exists, the logging-contract config-codes block exists, and T5 names its test host. I found no unowned write and no material duplication; §7’s restatement of R12’s registry is consistent (R12 remains the single source).

Positive: metadata sets and sizing rules are single-sourced, byte oracles are literals rather than re-marshals, and the derived tool-count rewrite removes literal pin drift.

REQUEST CHANGES
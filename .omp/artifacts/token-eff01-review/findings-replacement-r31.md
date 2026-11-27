VERDICT: REQUEST CHANGES

Specification mode, initial pass on `.scratch/token-efficient-tool-responses/spec.md` (r31-replacement, on-disk bytes). Read-only; no build/test run. Prior ledger: `findings-replacement-r30.md`.

**[HIGH] F1 — R8 truncates on decoded length but bounds the escaped serialized form.**
Affects: R8, AC9 (§2 R8 truncation paragraph; AC9).
Defect: "A value longer than `max_upstream_code_bytes` is truncated" keys the trigger to decoded length, while the next sentence bounds "the escaped serialized contribution … not … its decoded length" and guarantees it "never exceeds `max_upstream_code_bytes`."
Consequence: a decoded-short, escaping-heavy value (ten `"` with bound 11 decodes to 10) is never truncated; its escaped contribution is 20 bytes, so a preview/page/terminal can exceed the ceiling R11(g) assumed. AC9's over-bound fixture is selected by decoded length, so the declared oracle passes over the hole.
Fix: pin the trigger to "whenever the JSON-escaped serialized contribution plus the marker exceeds `max_upstream_code_bytes`", and add an AC9 fixture that is decoded-short but escaped-over-bound.

**[HIGH] F2 — R11(k) now exists, but its owning task, registered code, and config row were not updated; AC34 cannot pass as scoped.**
Affects: R11(k), AC34, §6 `max_upstream_code_bytes`, T1.
Defect (§2 R11, §6 row, §5 T1): R11 names eight new codes including `upstream_code_marker_bound`; T1 says "every R11 relation (a)–(j)" and "all seven new codes", omitting (k) and that code, while T1's RED lists `TestConfigRelationsPositiveKeys` (AC34). §6's row still says only "> 0 … marker inside the bound", although R11(k) asserts "§6's … row includes this floor" — it does not.
Consequence: implementing T1 as scoped leaves `max_upstream_code_bytes` < 11 admitted and the code unregistered; R8's rule then has no feasible prefix (marker = 11), so "exactly one prefix" and R11(g)'s feasibility guarantee fail at runtime.
Fix: T1 covers (a)–(k) and registers all eight codes; §6's row states ≥ 11. (Reopens r30-F2, partially closed.)

**[MEDIUM] F3 — R7/AC43 relabel the missing-session branch as an "epoch-change" trigger.**
Affects: R7, AC43, T3 (§2 R7 call-site paragraph).
Defect: R7 cites "the epoch-change branch retiring the live owner (inbound.go:802-812; epochs advance at projection.go:189 and :201)". `inbound.go:802-812` is inside `if mappedSDKRequest && responseObserver.missingSession()`; `projection.go:189/:201` only `c.nextEpoch++` in `newPublisher`/`newLiveOwner` and retire nothing.
Consequence: AC43's trigger (2) names a path that does not exist, so a reader may wire retirement on an epoch bump and leave the real missing-session rewrite path unscoped. The other sites (reap 522/767 → cleanup 676/770; DELETE 950-962) are accurate.
Fix: relabel trigger (2) as the missing-session rewrite branch and delete the epoch-advance clause. (Residual of r30-F1.)

**[MEDIUM] F4 — Four declared events have no producer/emission oracle.**
Affects: R12 (third registry), §7 Events, AC13(c).
Defect (§7 Events; AC5, AC16, AC18, AC24, AC25): §7 declares `result_expired`, `result_evicted`, `invalid_handle`, `invalid_arguments` with "emission is T3's", mapping them to store-level ACs that assert returned codes, not emitted event records or fields; AC13(c)'s field-set clause enumerates only `result_paged`, `result_page_served` and terminal events. R12 claims every named event's producing path "is exercised by its owning AC".
Consequence: T3 can omit these emissions and every declared oracle stays green — a finite-label set without proven producers.
Fix: extend AC13(c)/`TestLogLabelProducers` to assert emission and exact field sets for these four, or add a T3 AC. Also correct §7's "registry equals the emitted set is asserted by AC13": AC13 asserts the frozen 31-list plus two producers, not emitted-set equality (residual of r30-F4).

**Six invariants.** All present and testable. Independent byte oracle (AC1/AC2/AC6/AC37), retired-marker lifetime/accounting (R7/R16, AC5/AC8/AC18/AC39), per-terminal preview bounds (R10, AC11/AC26), and ceiling order (R11(a), AC12) are sound. Pinned `upstream_code` is compromised by F1; finite labels by F4. Every AC names a concrete test oracle; literal-byte oracles are used where implementation independence matters. Task write roots are otherwise exact files with sequential dependencies.

**Prior-ledger status.** r30-F3 (violation-record level) verified closed: R12 pins `warn` and AC13(b) asserts it. r30-F1, r30-F2, and r30-F4 remain partially open as F3, F2, and F4 above.

### Recommendation
REQUEST CHANGES
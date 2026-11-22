## Code Review Summary
**Mode:** specification; **pass:** repair (r22). Read-only; no implementation.
**Files:** 12 candidate inputs + source anchors. **Issues:** 3 LOW (0 CRITICAL/HIGH/MEDIUM).
**Limits:** no hashing/shell tool, so supplied SHA-256 exact-byte identity is unverified (C2 lane limit). §9.19's 11 non-spec digests match the r22 list; its spec digest is the r21 registration by design — no manifest inconsistency. No tests/spec-lint run.

### Closure
- **C1 CLOSED.** D11 :68, R5 :19, §4(3) :83 use an active-expansion stack: push a splice, pop on full target resolution; on-stack revisit = true cycle, popped revisit = finite re-splice. (b-realias) :122 reconstructs `snapshot.json→alias/../alias/../q5` (q5 protected, q4 evicted, link moved, read-through); (b-cycle) true ancestor cycle; (b-revisit) three-fold finite. No normative global visited-set test remains; the :122 "already-visited" wording is descriptive and the suite fails a global rule. Budget exact: 4096 splices/configured-path walk, own `Readlink` not a splice, `.`/empty/`..`/non-symlink uncounted; 4096-deep stack. All r21 termination classes and complete-chain protection retained.
- **C3 CLOSED.** D12 :69 makes (iii) a moved-chain-element condition only; (b-xA2)/(b-xB2)/(b-xB3) consistent; no operative change.
- **Regression OK.** R1–R9, AC1-1..10, AC2-1..9, T1–T3 unchanged; §6.1 same 14-file union (4+6+4); std-lib only (go 1.25.0); Load/Save/LoadPrompts/SavePrompts/ensureReplaceable, C6/CR15, C7 and §9.11 checkpoints unchanged; ticket mirror stale as documented (:7,:11,:91). Key anchors spot-verified (`store.go:104-106/185`, `envelope.go:103/136/244/575`, `log.go:228/275`, `wiring.go:486`, `startup_failure.go:26-33`).

### Issues
**[LOW][verification]** (b-budget) cannot distinguish budget termination from chain completion.
File: `.scratch/snapshot-recovery/spec.md:122`. Confidence: HIGH.
k4098 is regular/non-conforming and k* are never candidates, so every stated observable is identical whether the walk trips or completes; "BEFORE the chain ends" has no FAILS clause. The counting rule itself is exact.
Fix: point k4097 at the oldest conforming seed (q5); completion would then protect q5 and fail the "q5 evicted" assertion.

**[LOW][statement-precision]** D11 says the 4097-link chain "trips the budget first", then "whichever bound the implementation checks first".
File: `.scratch/snapshot-recovery/spec.md:68`. Confidence: HIGH. Both 4096 bounds trip at the same 4097th attempt; the AC accepts either class. Fix: state either bound may report at the 4097th attempt.

**[LOW][scope]** R9's refresher/second-run no-op is unconditional, but D12 (i) exempts loop-class links: a mutual `snapshot.json↔prompts.json` pair leaves the prompts link unreconciled; it resolves to the republished catalog (SavePrompts refuses) and run 2 quarantines it as `corrupt_json` with a new record.
File: `.scratch/snapshot-recovery/spec.md:23,69`. Confidence: HIGH (trace); scope rests on the open §9.11 (i) checkpoint.
Fix: scope R9 to paths absent post-pass or reconciled; align §9.11's "already dangling" wording with D12's loop/escape/directory list. Pre-existing r20, not r22-introduced; no id/envelope change.

### Positive Observations
Outcome-based oracles (bytes, survivor sets, stream order); exact splice counting; C1 fixtures pin both the fix and the true-cycle guard.

### Recommendation
APPROVE

## Code Review Summary

**Mode:** specification (initial), `snapshot-recovery-spec-r20@r20`; no implementation. **Limits:** no SHA-256/shell tool, so supplied digests are byte-unverified; no tests/spec-lint run; ticket mirror is stale by design (r15 heading, r7/r3 refs) and was not treated as contract. Anchors spot-verified in `store.go`, `envelope.go`, `log.go`, `startup_failure.go`.

### Closure
- **C1 CLOSED.** R2/R5/D10/D11/§4(3)/§8(2)/T1 agree: every hop resolves component-wise from `filepath.Dir` of the link read (root for absolute), with `..` on the resolved parent. Reconstructed `/s/sub/alias -> /s/deep`, `snapshot.json -> sub/alias/../q5` reaches `/s/q5`: q5 protected by entry identity plus cleaned path and survives, q4 evicted, link moved, read-through non-dangling (`AC1-10(b-alias):122`). `(b-alias2)` escape terminates before q5: absent branch, no quarantine/retention, five seeds intact, no reconciliation. `(b-alias-loop)` terminates on identity revisit: link moves as `read_error`, c5 evicted. Dangling component, ELOOP, mid-path not-a-directory and 4096-component budget are stated harmless. No operative lexical resolution remains; "lexical" survives only as the protection path fallback.
- **C7 CLOSED in substance.** D12:69 records pre-pass link status, chain, termination class, existing-regular-target identity and a pre-move moved-entry set; conditions (i)-(iv); fatal-free path only; own-family reserve+rename with `Reason=orphaned_link`; one record; R3-fatal failures with pass records already emitted; then ordinary retention. `AC1-10(b-xB2)/(b-xB3):122` pin both move-through directions, result order and counts; `AC2-9:131` pins exit 0, two warn records in order, fresh valid catalog, fresh regular `prompts.json` accepted by `SavePrompts` (no refusal wrap), and second-run no new quarantine. Guard confirmed (`store.go:122-126`; absent yields `ErrNoSnapshot`, `:86-88`). Owner checkpoint recorded OPEN, not claimed (§8:202; §9.11/§9.16).
- **L-1..L-4 closed; regression OK.** Narrowed "not itself a recorded chain element" wording (R5/AC1-3); ordinary count pre-existing-4 and mixed count the full unprotected set, with AC counts re-derived; over-protection oracle seeded (AC1-3:115); R2 backtick gone; LoadPrompts anchors 95-96/98-99/101-102/104-106 correct. No R/AC/T id changed; 14-file envelope (T1 4 + T2 6 + T3 4); std-lib only; Load/Save/LoadPrompts/SavePrompts/ensureReplaceable unchanged; C6/CR15 retained; six reasons consistent (R6/R8/T2/AC1-6/AC2-2); pin 129->130.

### Issues
**[MEDIUM][statement-contradiction]** C7's exclusion list disables its own fixtures and contradicts the quarantine rule.
File: `.scratch/snapshot-recovery/spec.md:69` (D12 ii), `:83` (§4(3)), `:15-16` (R2); AC at `:122`. Confidence HIGH.
The walk defines "non-conforming final target" as any regular target that is not a conforming quarantine name, which includes `snapshot.json`/`prompts.json` - exactly the `(b-xB2)/(b-xB3)` targets; read literally condition (ii) never passes, so reconciliation cannot fire. R2's "dangling, loop, escape, non-conforming, or directory ... is NOT moved" also contradicts R2's own stat-error rule and `(b-eloop)/(b-vac)` Mode 1, where such links ARE moved.
Fix: remove "non-conforming" from both exclusion lists (keep dangling/loop/escape/directory), or scope it to out-of-state-directory targets; write "NOT reconciled" instead of "NOT moved" in R2.

**[LOW][statement-precision]** `(b-xA2)` gives the wrong non-reconciliation reason.
File: `.scratch/snapshot-recovery/spec.md:122`. Confidence HIGH.
The target `prompts.json` WAS moved, so condition (iii) passes; the real failure is (iv): the configured snapshot link itself moved, so the entry no longer exists as a symlink.
Fix: cite condition (iv).

**[LOW][editorial]** Checksum attribution off by one.
File: `internal/storage/store.go:103-105` (spec §3). Confidence HIGH.
`ChecksumValue` is `:103`, guard `:104`, return `:105`; §3 says `:104`/`:105`/`:106`. Range `:104-106` is still correct.
Fix: reattribute.

### Advisory LOWs
4096 budget and mid-path not-a-directory lack seeded oracles. No fixture exercises post-reconciliation retention with pre-existing files. AC1-3 alias-identity (`:115`) keeps lexical-era "lexically unequal" wording; entry identity carries it. D12's fatal list omits post-rename sync failure (covered by R3's "exactly like a family pass's", unnamed).

### Positive
C1 fixture is end-to-end (q5 preserved, q4 evicted, link moved, read-through); escape/cycle cases observably harmless; `(b-xB3)` proves reconciliation runs after both passes; (i)/(ii) owner split explicit and unclaimed.

### Recommendation
REQUEST CHANGES

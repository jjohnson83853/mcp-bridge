# SPEC DELTA — SNAPREC-SPEC r24 → r25 (round 15, out-of-band; canonical execution state remains r16 and is not bound to this round)

## CHANGE LOG — r24 → r25

(non-normative; for the coordinator's revision ledger. Four items, all owner-recorded for this round: the four-ticket decomposition authorization of 2026-11-21 — "authorize planner spec update" — grants specification authorship only; no implementation, waiver, review retry, or pipeline transition.)

1. **Four-ticket decomposition (owner-authorized) — APPLIED.** §6's T1–T3 set is re-decomposed into four behavior-owned tasks **T1–T4** (closing text: Block 29). Every R/AC id is preserved and owned by exactly one task: R6/R7/R9 → T1; R5 → T2; R2/R8 → T3; R1/R3/R4 → T4; AC1-1..AC1-4 + AC1-7 → T1; AC1-5/AC1-6/AC1-8/AC1-9 + AC1-10(base and the protection/resolution subcases) → T2; AC1-10(b-x group, b-dangle) + AC2-9 (all subcases, incl. the non-mutual carve-out pin) → T3; AC2-1..AC2-8 → T4. Dependencies: T1 → T2, T3; T2 → T3; T1,T2,T3 → T4. **The §6.1 14-file write envelope is UNCHANGED; T4 adds a task id without adding files.** Stale pre-r25 task references in live (non-historical) sections are re-pointed (Blocks 5–8, 22, 24–26, 32–33, 36); historical round entries 9.9–9.21 keep their pre-r25 meanings (noted in the new §9.22, Block 38).
2. **CT24-C5 (HIGH, lifecycle hazard) — RE-APPLIED** (round 14's text was never persisted; the working spec is still r24 and the defect is still open). The moved-entry set is re-keyed: every renamed entry is recorded by its **ORIGINAL CONFIGURED PATH alongside its pre-move entry identity**; ordinary (iii)/(iv) and mutual (m-iii)/(m-iv) partner-versus-self is decided from the pre-pass record's **recorded chain-entry paths, never a bare inode match**. The counterexample (`snapshot.json` a symlink targeting `snapshot.json`; `prompts.json` a hard link to that symlink; both pre-pass chains loop; pass 1 quarantines snapshot `read_error`; prompts then dangles and its pass no-ops; the inode-only guard reads prompts as already moved, excludes the reconciliation, and after admission republishes snapshot the unchanged `SavePrompts` guard refuses and the restart re-quarantines prompts — the self-healing failure reproduced) is **stated in D12** (Block 15), echoed in R2 (Block 10), §4's `RecoverRejected` contract (Block 18), §4(3)'s pre-pass-records and reconciliation paragraphs (Blocks 19–20), §4's rejected alternatives (Blocks 21, 16), §8(2) (Block 34), T3's objective/deliverable (Block 29), §7 (Block 31), and AC2-9 (Block 27), and **pinned by the new AC2-9 mutual hard-linked-symlink variant** (exactly [snapshot `read_error`, prompts `orphaned_link`]; preserved link text via `Readlink`; fresh regular `prompts.json` accepted by direct `SavePrompts`; second-run no-op; no missing-source fatal; counting discrimination pinned — both-moved and self-moved still ZERO, clean pair still exactly ONE; named test added to the verification line). **Retention protection (D10/D11 — entry identity + cleaned path) is separate and unchanged.**
3. **CT24-C8 (LOW, evidence precision) — APPLIED.** The r22→r23 change log's "still passing under the stricter key" now reads that the fixtures **remain required planned oracles**; no executed GREEN is implied (Block 39; aligned with §9.6 — tests planned, none run).
4. **CR24 LOWs (previously unassigned) — APPLIED.** §9.21's `orphaned_label` token fixed to the actual reason value **`orphaned_link`** (Block 37); AC1-10(b-dangle) now cites **(m-ii)+(m-iii)** — the deciding conditions — rather than (m-iii) alone (Block 23), mirroring AC1-1(d)'s CR23-F5 fix.

**Open items carried unchanged:** the retention-unlink owner checkpoint (§8(2)/§9.11), the originally-dangling NON-mutual carve-out (§9.11, pinned by AC2-9's non-mutual subcase), and the requested non-OpenAI escalated critic (§9.12) remain OPEN; the mutual-link reconciliation remains the only owner direction claimed; no review, approval, owner confirmation beyond the recorded decomposition authorization, or test execution is claimed this round.

## DELTA BLOCKS

**Block 1 — §1 title**
- section: `# SNAPREC-SPEC-r24 — snapshot-recovery (revision, plan round 13)` (line 1)
- oldAnchor:
```
# SNAPREC-SPEC-r24 — snapshot-recovery (revision, plan round 13)
```
- newText:
```
# SNAPREC-SPEC-r25 — snapshot-recovery (revision, plan round 15)
```

**Block 2 — §1 Spec ID / revision bullet**
- section: §1 STATUS, first bullet
- oldAnchor:
```
- **Spec ID / revision:** `SNAPREC-SPEC-r24`, revision **r24**, plan round **13** (dispatched by the coordinator's round-13 request `SnapshotRecoverySpecRound13` — the SECOND round of the reset 12–14 plan-loop budget; the plan-round cap accounting is the coordinator's ledger, not this planner's).
```
- newText:
```
- **Spec ID / revision:** `SNAPREC-SPEC-r25`, revision **r25**, plan round **15** (dispatched by the coordinator's round-15 request `SnapshotRecoverySpecRound15`, out-of-band — the canonical execution state remains r16 and is not bound to this round; the plan-round cap accounting is the coordinator's ledger, not this planner's).
```

**Block 3 — §1 Round bullet (complete replacement)**
- section: §1 STATUS, Round bullet
- oldAnchor:
```
- **Round:** r24 revises the reviewed r23 candidate in place — round 13, the SECOND round of the reset 12–14 plan-loop budget. It closes the six recorded CR23 findings as ONE semantic correction of the reconciliation predicate plus its supporting definitions and oracles,
```
- newText:
```
- **Round:** r25 revises the working r24 candidate in place — round 15, dispatched out-of-band (the canonical execution state remains r16; round 14's CT24 text was recorded but never persisted, so the working spec is still r24 and both CT24 findings are re-applied here rather than carried as closed). Four changes, with **no change to any R/AC id and no change to the §6.1 14-file write envelope**: (1) **the owner-authorized four-ticket decomposition** (owner authorization 2026-11-21, "authorize planner spec update", recorded in `.omp/artifacts/snapshot-recovery-ticket-decomposition-r25.md`) — §6's T1–T3 tasks are re-decomposed into four behavior-owned tasks T1–T4; every R/AC id is preserved by mapping, each owned by exactly one task; T4 is a NEW TASK ID that adds no file; dependencies become T1 → T2, T3; T2 → T3; T1,T2,T3 → T4. (2) **CT24-C5 (HIGH, lifecycle hazard) — re-applied**: the moved-entry set is re-keyed — every renamed entry is recorded by its ORIGINAL CONFIGURED PATH alongside its pre-move entry identity, and ordinary (iii)/(iv) and mutual (m-iii)/(m-iv) partner-versus-self is decided from the pre-pass record's recorded chain-entry paths, never a bare inode match; the hard-linked-symlink counterexample is stated in D12 and pinned by a new AC2-9 mutual hard-linked-symlink variant; retention protection (D10/D11) is separate and unchanged. (3) **CT24-C8 (LOW, evidence precision)**: the r22→r23 change log's "still passing under the stricter key" now reads that the fixtures remain required planned oracles — no executed GREEN is implied (aligned with §9.6). (4) **the CR24 LOWs**: §9.21's `orphaned_label` token is fixed to `orphaned_link`, and AC1-10(b-dangle) cites (m-ii)+(m-iii) rather than (m-iii) alone. No new source read is required this round: every r25 amendment is design/AC/decision/task text level — the CT24-C5 counterexample and re-key resolve by spec-text reasoning over the r24 D12 rules and the `link(2)` hard-link capability cited from the r22/r23 critic evidence, and the four-ticket decomposition applies the owner-authorized proposal verbatim in mapping. **The escalated-critic request, the retention-unlink owner checkpoint, and the originally-dangling NON-mutual carve-out remain OPEN exactly as recorded; the mutual-link reconciliation inside startup recovery remains OWNER-DIRECTED (the r23 dispatch), and NO owner confirmation is claimed beyond the recorded four-ticket decomposition authorization; no review, approval, or test execution is claimed this round.** The r10 hazard classes remain advisory only (§9).
```

**Block 4 — §3 VERIFIED header, r25 disclosure**
- section: §3 VERIFIED intro parenthetical, tail
- oldAnchor:
```
**r24 likewise reads no new source: every r24 amendment is design/AC/decision text level — the CR23-F1/F2 predicate correction, the F3 oracles, the F4 key definition, the F5 citation, and the F6 spelling resolve by spec-text reasoning over the r23 reconciliation and resolver rules):**
```
- newText:
```
**r24 likewise reads no new source: every r24 amendment is design/AC/decision text level — the CR23-F1/F2 predicate correction, the F3 oracles, the F4 key definition, the F5 citation, and the F6 spelling resolve by spec-text reasoning over the r23 reconciliation and resolver rules; r25 likewise reads no new source: every r25 amendment is design/AC/decision/task text level — the CT24-C5 re-key and counterexample resolve by spec-text reasoning over the r24 D12 rules and the `link(2)` hard-link capability cited from the r22/r23 critic evidence, and the four-ticket decomposition applies the owner-authorized proposal (`.omp/artifacts/snapshot-recovery-ticket-decomposition-r25.md`, owner authorization 2026-11-21) verbatim in mapping):**
```

**Block 5 — §3 wiring.go bullet, task-anchor re-point**
- section: §3 VERIFIED, `cmd/mcp-bridge/wiring.go:486-493` bullet, tail
- oldAnchor:
```
the existing remediation the `snapshot_unavailable` envelope case must mirror (T2 anchor, CR13-06).
```
- newText:
```
the existing remediation the `snapshot_unavailable` envelope case must mirror (T3 anchor under the r25 decomposition — the envelope-vocabulary task; CR13-06).
```

**Block 6 — §3 validErrorInfo bullet, task re-point**
- section: §3 VERIFIED, `internal/observe/envelope.go:136-141` bullet
- oldAnchor:
```
the retention record's `state_directory` fields must match T2's new `runtimeErrorInfo` case field-for-field or the record falls back (basis for AC1-6's full-shape oracle, CR13-05)
```
- newText:
```
the retention record's `state_directory` fields must match T3's new `runtimeErrorInfo` case field-for-field (the envelope-vocabulary task under the r25 decomposition) or the record falls back (basis for AC1-6's full-shape oracle, CR13-05)
```

**Block 7 — §3 startup_failure.go bullet, task-anchor re-point**
- section: §3 VERIFIED, `cmd/mcp-bridge/startup_failure.go:26-33` bullet, tail
- oldAnchor:
```
the canonical `state_directory` remediation for the envelope case and AC1-6's pin (T2 anchor, CR13-06).
```
- newText:
```
the canonical `state_directory` remediation for the envelope case and AC1-6's pin (T3 anchor under the r25 decomposition — the envelope-vocabulary task; CR13-06).
```

**Block 8 — §3 recovery.go bullet, task-set update**
- section: §3 VERIFIED, `internal/storage` package-listing bullet
- oldAnchor:
```
- `internal/storage` has **no `recovery.go`** (package listing); recovery is net-new. T1/T2/T3 write effects are pending.
```
- newText:
```
- `internal/storage` has **no `recovery.go`** (package listing); recovery is net-new. T1–T4 write effects are pending.
```

**Block 9 — §3 [AUTO-DECISION] header, r25 amendment record**
- section: §3 [AUTO-DECISION] intro parenthetical, tail
- oldAnchor:
```
**amended this round (r24: D11 — the expansion-key containing-directory context pinned to the directory's `lstat` identity with the resolved path as tie-break/fallback (CR23-F4); D12 — the live-symlink-entry requirement and the moved-entry guard extended to the mutual class, with the exact zero/one reconciliation counting rule (CR23-F1/F2); no other decision amended; all r23 amendments stand)**):
```
- newText:
```
**amended r24 (D11 — the expansion-key containing-directory context pinned to the directory's `lstat` identity with the resolved path as tie-break/fallback (CR23-F4); D12 — the live-symlink-entry requirement and the moved-entry guard extended to the mutual class, with the exact zero/one reconciliation counting rule (CR23-F1/F2)); amended this round (r25: D12 — the moved-entry set re-keyed by original configured path + pre-move entry identity, partner-versus-self decided from the pre-pass record's recorded chain-entry paths, never a bare inode match (CT24-C5); no other decision amended; all r24 amendments stand)**):
```

**Block 10 — §2 R2, re-key echo (mutal passage)**
- section: §2 R2, mutual-loop reconciliation passage
- oldAnchor:
```
is DANGLING after both family passes — its entry still a LIVE symlink (`lstat`), not itself in the moved-entry set, with its loop partner — a chain element other than the configured entry itself — matched in the moved-entry set (D12 (m-i)–(m-iv); r24, CR23-F1/F2: EVERY reconciliation path, ordinary or mutual, requires the configured entry to still exist post-passes as a symlink; an absent entry — including one this run itself moved — is a reconciliation no-op with the pre-pass class recorded as the reason)
```
- newText:
```
is DANGLING after both family passes — its entry still a LIVE symlink (`lstat`), not itself in the moved-entry set (r25, CT24-C5: the set is keyed by each renamed entry's ORIGINAL CONFIGURED PATH alongside its pre-move entry identity — a hard link sharing a moved entry's inode at a different configured path is a different entry, never a false self-match), with its loop partner — a chain element other than the configured entry itself — matched in the moved-entry set by its RECORDED CHAIN-ENTRY PATH against the moved entries' original configured paths, never a bare inode match (r25, CT24-C5; D12 (m-i)–(m-iv); r24, CR23-F1/F2: EVERY reconciliation path, ordinary or mutual, requires the configured entry to still exist post-passes as a symlink; an absent entry — including one this run itself moved — is a reconciliation no-op with the pre-pass class recorded as the reason)
```

**Block 11 — §3 D12 header**
- section: §3 [AUTO-DECISION] D12, header
- oldAnchor:
```
- **D12 — Orphaned configured-link reconciliation (added r20, C7; amended r21 — M1/A4; amended r22 — C3; amended r23 — C5: the mutual loop-partner detection and reconciliation, owner-directed; amended r24 — CR23-F1/F2: the live-symlink-entry requirement and the moved-entry guard on the mutual class):**
```
- newText:
```
- **D12 — Orphaned configured-link reconciliation (added r20, C7; amended r21 — M1/A4; amended r22 — C3; amended r23 — C5: the mutual loop-partner detection and reconciliation, owner-directed; amended r24 — CR23-F1/F2: the live-symlink-entry requirement and the moved-entry guard on the mutual class; amended r25 — CT24-C5: the moved-entry set re-keyed by original configured path + pre-move entry identity, partner-versus-self from the recorded chain-entry paths — never a bare inode match):**
```

**Block 12 — §3 D12, pre-pass records + moved-entry key**
- section: §3 D12, pre-pass sentence
- oldAnchor:
```
whether its entry is a symlink, its walked chain (every visited link entry's identity and cleaned path), the chain's termination class, and — when the chain terminated at an **existing regular final target** — that target's entry identity (the pre-pass resolution identity and non-dangling link status). Every entry this run renames is recorded in a moved-entry set by its pre-move entry identity.
```
- newText:
```
whether its entry is a symlink, its walked chain (every visited link entry's identity AND cleaned path — the recorded chain-entry paths are the r25 partner-versus-self discriminator, CT24-C5), the chain's termination class, and — when the chain terminated at an **existing regular final target** — that target's entry identity (the pre-pass resolution identity and non-dangling link status). Every entry this run renames is recorded in a moved-entry set **keyed by its ORIGINAL CONFIGURED PATH alongside its pre-move entry identity (r25, CT24-C5): a moved entry is identified by the pair (the configured path it was moved from, its pre-move `lstat` entry identity), never by its inode alone — a hard link sharing the moved entry's inode at a different configured path is a different entry and never a false self-match.**
```

**Block 13 — §3 D12, ordinary detection (iii)/(iv) re-key**
- section: §3 D12, ORDINARY detection conditions
- oldAnchor:
```
(iii) at least one chain element **other than the configured link entry itself** was moved by this run (identity match against the moved-entry set; if the configured entry itself moved, its quarantine already happened and the path is absent — nothing to reconcile), and (iv) post-pass the configured path's entry still exists as a symlink and `Stat` through it fails `fs.ErrNotExist`.
```
- newText:
```
(iii) at least one chain element **other than the configured link entry itself** was moved by this run — decided from the pre-pass record's recorded chain-entry paths against the moved-entry set's original configured paths, never a bare inode match (r25, CT24-C5; if the configured entry itself moved, its quarantine already happened and the path is absent — nothing to reconcile), and (iv) post-pass the configured path's entry still exists as a symlink and is not itself in the moved-entry set under the original-configured-path + pre-move-identity key (r25, CT24-C5), and `Stat` through it fails `fs.ErrNotExist`.
```

**Block 14 — §3 D12, mutual detection (m-iii)/(m-iv) re-key**
- section: §3 D12, MUTUAL-LOOP detection conditions
- oldAnchor:
```
(m-iii) at least one chain element **other than the configured entry itself** was moved by this run (identity match against the moved-entry set — the loop partner, typically the other configured path quarantined `read_error` by its own pass), and (m-iv) post-pass the configured path's entry **still exists as a symlink (`lstat`) and `Stat` through it fails `fs.ErrNotExist`** —
```
- newText:
```
(m-iii) at least one chain element **other than the configured entry itself** was moved by this run — the loop partner, typically the other configured path quarantined `read_error` by its own pass, matched from the pre-pass record's recorded chain-entry paths against the moved-entry set's original configured paths, never a bare inode match (r25, CT24-C5), and (m-iv) post-pass the configured path's entry **still exists as a symlink (`lstat`), is not itself in the moved-entry set under the original-configured-path + pre-move-identity key (r25, CT24-C5), and `Stat` through it fails `fs.ErrNotExist`** —
```

**Block 15 — §3 D12, CT24-C5 counterexample (inserted before the case-(i) carve-out)**
- section: §3 D12, after the exact-counting-rule paragraph
- oldAnchor:
```
**Originally-broken configured links — case (i): pre-pass chain class dangling, escape, or directory, AND loop-class links whose loop partner was NOT moved by this run (the NON-mutual class; the r23 mutual extension reconciles only the moved-partner case) — are NOT reconciled**
```
- newText:
```
**CT24-C5 counterexample (r25 — why the inode-only key is rejected): `snapshot.json` is a symlink targeting `snapshot.json`, and `prompts.json` is a hard link to that symlink (`link(2)`) — same inode and target text, a different directory entry; both pre-pass chains resolve in the loop class. Pass 1 quarantines the snapshot link entry as `read_error`; `prompts.json` then dangles and its pass no-ops on the absent branch. Under an inode-only moved-entry key the moved snapshot entry shares `prompts.json`'s inode, so the guard reads prompts as already moved and excludes the mutual reconciliation; admission republishes a fresh `snapshot.json`, `prompts.json` then resolves to catalog bytes, the unchanged `SavePrompts` guard (`store.go:122-126`) refuses on every refresh, and the restart re-quarantines prompts — the C5 self-healing failure reproduced after admission. The original-configured-path + pre-move-identity key excludes it: the moved entry is keyed `(<state>/snapshot.json, its pre-move link identity)`, which matches no recorded prompts chain-entry path as `prompts.json` itself, so (m-iv) passes and (m-iii) reads the partner from the recorded chain-entry paths (the moved `snapshot.json` is a prompts chain element) — exactly one reconciliation fires, `[snapshot read_error, prompts orphaned_link]`, pinned by the AC2-9 mutual hard-linked-symlink variant. Retention protection (D10/D11) is unaffected: entry identity + cleaned path, unchanged.** **Originally-broken configured links — case (i): pre-pass chain class dangling, escape, or directory, AND loop-class links whose loop partner was NOT moved by this run (the NON-mutual class; the r23 mutual extension reconciles only the moved-partner case) — are NOT reconciled**
```

**Block 16 — §3 D12, rejected alternative (inode-only moved-entry key)**
- section: §3 D12, Rejected alternatives, tail
- oldAnchor:
```
**reconciling an absent or self-moved configured entry (rejected, r24 CR23-F1/F2 — the rename of a missing source is an `ENOENT` R3 fatal and a spurious extra record; the live-symlink-entry gate with the moved-entry guard on both detections is chosen instead)**.
```
- newText:
```
**reconciling an absent or self-moved configured entry (rejected, r24 CR23-F1/F2 — the rename of a missing source is an `ENOENT` R3 fatal and a spurious extra record; the live-symlink-entry gate with the moved-entry guard on both detections is chosen instead)**; **keying the moved-entry set on entry identity alone (rejected, r25 CT24-C5 — the counterexample above: a hard link sharing the moved entry's inode at a different configured path false-matches the self-guard, excludes the mutual reconciliation, and reproduces the C5 self-healing failure after admission republishes the catalog; the original-configured-path + pre-move-identity key with partner-versus-self decided from the recorded chain-entry paths is chosen instead — retention protection unchanged)**.
```

**Block 17 — §3 D12, consequence sentence (r25 oracle note)**
- section: §3 D12, Consequence, mutual-pair clause
- oldAnchor:
```
and the both-moved zero-reconciliation variant (CR23-F2/F3)), and AC2-9's non-mutual subcase pins the documented limitation of the OPEN carve-out rather than a false pass.**
```
- newText:
```
and the both-moved zero-reconciliation variant (CR23-F2/F3); r25 adds the mutual hard-linked-symlink variant (CT24-C5 — the re-keyed moved-entry identity pinned end-to-end: `[snapshot read_error, prompts orphaned_link]`, the `Readlink` preserved target text, the direct `SavePrompts` acceptance, the second-run no-op, no missing-source fatal, and the counting discrimination — clean pair exactly ONE, both-moved and self-moved still ZERO)), and AC2-9's non-mutual subcase pins the documented limitation of the OPEN carve-out rather than a false pass.**
```

**Block 18 — §4 `RecoverRejected` contract, re-key echo**
- section: §4 Storage contract, `RecoverRejected` bullet, reconciliation clause
- oldAnchor:
```
each gated on a live post-pass symlink entry at the configured path and on the moved-entry set (r24, CR23-F1/F2: an absent or self-moved configured entry is a recorded no-op, never reconciled — zero reconciliations when both configured entries moved))
```
- newText:
```
each gated on a live post-pass symlink entry at the configured path and on the moved-entry set (r24, CR23-F1/F2: an absent or self-moved configured entry is a recorded no-op, never reconciled — zero reconciliations when both configured entries moved; r25, CT24-C5: the set is keyed by original configured path + pre-move entry identity and partner-versus-self is decided from the recorded chain-entry paths — never a bare inode match))
```

**Block 19 — §4(3) pre-pass records, re-key echo**
- section: §4(3), Pre-pass records paragraph
- oldAnchor:
```
**Pre-pass records (r20-C7/D12 + r23-C5): the same population walk records, for each configured family path, its entry kind (symlink or not), its chain, its termination class, and — on an existing regular final target — that target's entry identity (the pre-pass resolution identity and non-dangling status), all before any move; every entry this run renames joins the moved-entry set by its pre-move identity.**
```
- newText:
```
**Pre-pass records (r20-C7/D12 + r23-C5): the same population walk records, for each configured family path, its entry kind (symlink or not), its chain — every visited link entry's identity AND cleaned path, the recorded chain-entry paths being the partner-versus-self discriminator (r25, CT24-C5) — its termination class, and — on an existing regular final target — that target's entry identity (the pre-pass resolution identity and non-dangling status), all before any move; every entry this run renames joins the moved-entry set keyed by its ORIGINAL CONFIGURED PATH alongside its pre-move entry identity — never the inode alone (r25, CT24-C5).**
```

**Block 20 — §4(3) reconciliation paragraph, re-key echo**
- section: §4(3), Post-pass orphan reconciliation paragraph, gating clause
- oldAnchor:
```
a non-dangling existing-regular-final-target chain containing a moved element other than itself, OR (r23-C5, owner-directed) a loop/stat-error-class chain whose moved loop partner (a chain element other than itself) this run quarantined — and, in both classes, whose configured entry STILL EXISTS post-passes as a symlink (`lstat`) and `Stat`s through it `fs.ErrNotExist` (r24, CR23-F1/F2: the r23 `or is absent` clause is DELETED
```
- newText:
```
a non-dangling existing-regular-final-target chain containing a moved element other than itself, OR (r23-C5, owner-directed) a loop/stat-error-class chain whose moved loop partner (a chain element other than itself) this run quarantined — in either arm the moved partner/element is matched from the pre-pass record's recorded chain-entry paths against the moved-entry set's original configured paths, never a bare inode match (r25, CT24-C5) — and, in both classes, whose configured entry STILL EXISTS post-passes as a symlink (`lstat`), is not itself in the moved-entry set under the original-configured-path + pre-move-identity key (r25, CT24-C5: a hard-linked different configured entry is never a false self-match), and `Stat`s through it `fs.ErrNotExist` (r24, CR23-F1/F2: the r23 `or is absent` clause is DELETED
```

**Block 21 — §4 Alternatives, rejected inode-only moved-entry key**
- section: §4 Alternatives considered, tail
- oldAnchor:
```
the live-symlink-entry (`lstat`) gate with the moved-entry guard on both detections is chosen instead — an absent configured entry is a recorded no-op)**.
```
- newText:
```
the live-symlink-entry (`lstat`) gate with the moved-entry guard on both detections is chosen instead — an absent configured entry is a recorded no-op)**; **keying the moved-entry set on entry identity alone (rejected, r25 CT24-C5 — the hard-linked-symlink counterexample: `prompts.json` hard-linked to the quarantined `snapshot.json` symlink shares its inode, so an identity-only guard reads prompts as already moved, excludes the mutual reconciliation, and reproduces the C5 self-healing failure after admission republishes the catalog; the original-configured-path + pre-move-identity key, with partner-versus-self decided from the recorded chain-entry paths, is chosen instead — retention protection (D10/D11) unchanged: entry identity + cleaned path)**.
```

**Block 22 — §5 AC1-2, task re-point**
- section: §5 AC1-2, sixth-value clause, tail
- oldAnchor:
```
its envelope admission is AC2-2/T2's six-reason table.** FAILS if any seeded class yields a different reason.
```
- newText:
```
its envelope admission is AC2-2/T3's six-reason table (the envelope-vocabulary task under the r25 decomposition).** FAILS if any seeded class yields a different reason.
```

**Block 23 — §5 AC1-10(b-dangle), CR24 citation fix**
- section: §5 AC1-10, (b-dangle) subcase, no-reconciliation clause
- oldAnchor:
```
(**and no reconciliation — ordinary or mutual: the prompts pre-pass chain class is dangling, so condition (ii) fails, and no loop partner moved, so (m-iii) fails — the C7 case-(i) carve-out, AC1-1d, pinned by AC2-9's non-mutual subcase**)
```
- newText:
```
(**and no reconciliation — ordinary or mutual: the prompts pre-pass chain class is dangling, so the ordinary condition (ii) fails; the mutual no-fire is decided by (m-ii)+(m-iii) — the chain is dangling, not loop/stat-error class, so (m-ii) fails, and no loop partner moved, so (m-iii) fails (r25, CR24-LOW; the same omission CR23-F5 corrected in AC1-1(d)) — the C7 case-(i) carve-out, AC1-1d, pinned by AC2-9's non-mutual subcase**)
```

**Block 24 — §5 AC2-2, reason-table subcase task re-point**
- section: §5 AC2-2, reason-table subcase, tail
- oldAnchor:
```
each emitted record admits its `failure_reason` without envelope fallback — mirroring the same six `edge_test.go` admission cases (T2).**
```
- newText:
```
each emitted record admits its `failure_reason` without envelope fallback — mirroring the same six `edge_test.go` admission cases (T3 — the envelope-vocabulary task under the r25 decomposition).**
```

**Block 25 — §5 AC2-2, record-shape subcase task re-point**
- section: §5 AC2-2, `snapshot_unavailable` record-shape subcase
- oldAnchor:
```
**`snapshot_unavailable` record-shape subcase (CR15-04; R4/R8): `edge_test.go` (T2) asserts the full four-field `ErrorInfo` shape for `snapshot_unavailable`
```
- newText:
```
**`snapshot_unavailable` record-shape subcase (CR15-04; R4/R8): `edge_test.go` (T3 — the envelope-vocabulary task under the r25 decomposition) asserts the full four-field `ErrorInfo` shape for `snapshot_unavailable`
```

**Block 26 — §5 AC2-8, ownership heading**
- section: §5 AC2-8 heading
- oldAnchor:
```
- **AC2-8 (R3,R2) — deterministic storage-level seams, both families (T1-owned).**
```
- newText:
```
- **AC2-8 (R3,R2) — deterministic storage-level seams, both families (T4-owned under the r25 decomposition; the oracles land in `internal/storage/recovery_test.go` — an envelope file, no new file).**
```

**Block 27 — §5 AC2-9, new mutual hard-linked-symlink variant (inserted before the non-mutual subcase)**
- section: §5 AC2-9, after the mutual-class post-reconciliation retention variant
- oldAnchor:
```
**Non-mutual originally-dangling limitation subcase (added r23, CR22-LOW/C5 — pins the documented OPEN limitation, NOT a false pass):
```
- newText:
```
**Mutual hard-linked-symlink variant (added r25, CT24-C5; R2/R3/R6/D12 — the re-keyed moved-entry identity pinned end-to-end): seed `snapshot.json` as a relative symlink whose target text is `snapshot.json` (self-referential) and `prompts.json` as a HARD LINK to that symlink entry (std-lib `os.Link` — same inode and target text, a different directory entry); both pre-pass chains resolve in the loop class. `runContext` serve: exit **0**; pass 1 quarantines the snapshot link entry as `read_error` (`Stat` fails `ELOOP`), the prompts pass then no-ops on the absent branch (`Stat(prompts.json)` resolves through the absent `snapshot.json` to `ErrNotExist`) — stderr carries **exactly two** `startup_snapshot_rejected` records in deterministic order [snapshot `read_error`, prompts `orphaned_link`] and **zero** `startup_failed`: the prompts entry is NOT read as already moved (the moved snapshot entry is keyed `(<state>/snapshot.json, its pre-move link identity)`; the prompts entry lives at a different original configured path, so the moved-entry guard does not false-match the shared inode — the CT24-C5 exclusion), while its recorded chain-entry path `snapshot.json` matches the moved entry's original configured path, so partner-versus-self reads PARTNER and the mutual detection fires; the dangling prompts link entry — a present entry, no missing source, no `ENOENT` R3 fatal — is reserve+renamed into a prompts-family conforming quarantine name. `Readlink(<prompts quarantine path>)` returns exactly the original prompts-link target text `snapshot.json` (the link bytes preserved by the rename; FAILS if the entry is not a symlink or the target text differs); poll (deadline 5s, interval 25ms) until a fresh **valid** `snapshot.json` exists and `Load` succeeds AND a fresh **regular** `prompts.json` exists with `LoadPrompts` succeeding; a direct `SavePrompts` of a valid new prompt state returns nil — **not** the `storage: refusing to overwrite untrusted prompt snapshot` wrap (the C5 self-healing failure the inode-only key would reproduce after admission republishes the catalog does NOT occur; the refusal FAILs). Second `runContext`: exit 0, no new quarantine, no new record. **Counting discrimination pinned (r25, CT24-C5): the clean mutual pair still produces exactly ONE reconciliation (the partner alone); this hard-linked layout likewise ONE; the both-moved variant and a self-moved (self-referential single-link) run still produce ZERO `orphaned_link` records.** FAILS on zero reconciliations (the inode-only moved-entry key reproduces the CT24-C5 exclusion — this oracle FAILs it), a third record or any `orphaned_link` shortfall, a `startup_failed`/`ENOENT` fatal, the `Readlink` target-text mismatch, the `SavePrompts` refusal, either publish missing, or any new quarantine/record on the second run.** **Non-mutual originally-dangling limitation subcase (added r23, CR22-LOW/C5 — pins the documented OPEN limitation, NOT a false pass):
```

**Block 28 — §5 AC2-9, verification line**
- section: §5 AC2-9, Verify line
- oldAnchor:
```
Verify: `go test ./cmd/mcp-bridge/ -run 'TestSnapshotRecoveryPromptsOnly|TestSnapshotRecoveryOrphanedLink|TestSnapshotRecoveryMutualLink|TestSnapshotRecoveryMutualBothMoved|TestSnapshotRecoveryNonMutualDangling'`.
```
- newText:
```
Verify: `go test ./cmd/mcp-bridge/ -run 'TestSnapshotRecoveryPromptsOnly|TestSnapshotRecoveryOrphanedLink|TestSnapshotRecoveryMutualLink|TestSnapshotRecoveryMutualBothMoved|TestSnapshotRecoveryMutualHardlinked|TestSnapshotRecoveryNonMutualDangling'`.
```

**Block 29 — §6 TASK LEDGER (complete replacement of the intro + T1/T2/T3 blocks, between `<!-- specdd:tasks:start -->` and `<!-- specdd:tasks:end -->`)**
- section: §6, from the intro sentence through the former T3 block
- oldAnchor:
```
One engineer owns production code and permanent tests across all three tasks (W01); tasks T1/T2 are independent, T3 depends on both. All write paths are inside the owner-approved envelope; **no other write path is authorized** — a genuinely required additional path is returned as a machine-readable blocker naming path/reason, never added as a task.
```
- newText:
```
One engineer owns production code and permanent tests across all four tasks (W01). **Decomposition (r25, owner-authorized — the four-ticket re-decomposition of the former T1–T3 set; `.omp/artifacts/snapshot-recovery-ticket-decomposition-r25.md`, owner authorization 2026-11-21): dependencies are T1 → T2, T3; T2 → T3; T1,T2,T3 → T4** (T2 depends on T1; T3 depends on T1 and T2; T4 depends on all three). **The §6.1 write envelope is UNCHANGED — the same owner-approved 14 files; T4 is a NEW TASK ID that adds no file** (task boundaries and test ownership move; no write path is added or removed). All write paths are inside the envelope; **no other write path is authorized** — a genuinely required additional path is returned as a machine-readable blocker naming path/reason, never added as a task.

**Ownership mapping (r25; every R/AC id appears exactly once and is owned by exactly one task):** R6, R7, R9 → **T1**; R5 → **T2**; R2, R8 → **T3**; R1, R3, R4 → **T4**. AC1-1, AC1-2, AC1-3, AC1-4, AC1-7 → **T1**; AC1-5, AC1-6, AC1-8, AC1-9, and AC1-10 base + (b-realias), (b-revisit), (b-cycle), (b-hardalias), (b-hardalias2), (b-hardcycle), (b-budget) + the unlisted protection/resolution subcases (b-prot), (b-chain), (b-chain2), (b-eloop), (b-sub), (b-sub2), (b-alias), (b-alias2), (b-alias-loop), (b-mid), (b-mix), (b-vac) → **T2**; AC1-10(b-x) — (b-xA), (b-xB), (b-xA2), (b-xB2), (b-xB3), (b-xB-res) — and (b-dangle) → **T3**; AC2-9 (all subcases, including the non-mutual carve-out pin and the r25 hard-linked-symlink variant) → **T3**; AC2-1..AC2-8 → **T4**. Cross-references are fixtures, not dual ownership (each id is owned once above): AC1-3's eviction subcases and AC1-9(b)'s collision oracle exercise T2's invariants from T1's files; AC1-6's envelope-level admission cases ride T3's R8 vocabulary and its `TestEmitRecoveryRecords` half rides T4's emitter contract; AC2-8's oracles land in `internal/storage/recovery_test.go`.

- **T1 — quarantine core: classify, preserve bytes, warn, continue.**
  - Objective: implement `Store.RecoverRejected`'s core — classification via the shared walk (D7), the non-regular fatal branch (never opened, never renamed), reserve-then-rename quarantine with the `fs.ErrExist` advance rule (D6), stop-on-first-fatal family processing (D2), per-family `<family>.corrupt-<RFC3339Nano UTC Z>.json` naming and the `QuarantineKeep` retention bound, exactly one typed warn record per completed quarantine (the R4 shape as emitted by T4's emitter), and the continue-startup behavior — no restored catalog/prompt state; admission and the refresher publish fresh generations over absent configured paths (R9) — plus the seams/sentinels; extract the shared classification helpers in `store.go` with zero public behavior change.
  - Depends: none. Inputs: this spec §2/§4/§5; `store.go:84-110` (`LoadPrompts` walk — anchors corrected r20, CR19-L-4; re-verified r21), `:112-126` (`SavePrompts` guard — the C7/C5 wedge, read-only), `:149-198` (`Load`), `:201-233` (`Save`), `:295-306` (`ensureReplaceable`), `:309` (`syncDirectory`); `snapshot.go:111` (`PromptSchemaVersion`); `snapshot.go`/`snapshot_v1.go` branch structure.
  - Expected files: `internal/storage/recovery.go` (new), `internal/storage/recovery_test.go` (new, `package storage` for seam injection), `internal/storage/store.go` (helper extraction only), `internal/storage/store_edge_test.go` (additions: AC1-7 negative oracles, AC1-1c/1d symlink cases).
  - Allowed: unexported seam vars; exported `ErrSnapshotUnrecoverable`, `QuarantineKeep`, `Recovery`, `RecoveryFamily`, `RetentionFailure`, `RecoveryResult`, `RecoverRejected` (with the R6 six-value `Reason`, incl. `orphaned_link`); tests may seed FIFOs with std-lib `syscall.Mkfifo` and hard links with std-lib `os.Link` (no new dependency, no root). Prohibited: any observable change to `Load`/`Save`/`ensureReplaceable`/`LoadPrompts`/`SavePrompts`; new dependencies; edits outside the envelope.
  - RED: `recovery_test.go` seeded-class table fails to compile (undefined `RecoverRejected`/`RecoveryResult`/sentinel) — `go test ./internal/storage/ -run TestRecoverRejected` exits non-zero. GREEN: same selector exits 0 with the oracles above.
  - Deliverable + ACs: the quarantine core as specified; **AC1-1, AC1-2, AC1-3, AC1-4, AC1-7** (AC1-3's eviction/protection subcases exercise T2's invariants — cross-referenced, not re-owned); the continue-startup behavior (R9) whose end-to-end effect is verified by AC2-1..AC2-3 (T4-owned).
  - Validation (all must exit 0): `go test ./internal/storage/...`; targeted `go test ./internal/storage/ -run 'TestRecover|TestStore'` (existing store tests unmodified).
  - Decision log: coordinator's feature decisions ledger; cite D1–D3, D5–D7.
  - Blocked (report unsuccessful with machine-readable blocker): if sharing the classification walk would require changing `Load`'s wrapping or any non-envelope file; if a seam cannot stay unexported.
- **T2 — retention and protection.**
  - Objective: land the run-level protected-target set — populated once before the first family pass by walking both configured family paths' COMPLETE backing chains, each hop resolved component-wise from its own base (D11/r20-C1; resolution state + expansion keys per D11, r22 r21-C1, r23-C4, identity-pinned r24 CR23-F4) — protection membership by **entry identity + cleaned path** (independent of the expansion-key shape and of the r25 moved-entry re-key: retention protection is D10/D11 and unchanged by CT24-C5), family bounds with the foreign-name filter, deterministic oldest-first eviction, the retention-failure invariant (select-once / attempt-once / first-failure-ends-pass; bound explicitly permitted to be exceeded on listing/unlink failure — C5/D10), backing-byte preservation, the mixed >4 carve-out (L3), and no deletion cap (C2).
  - Depends: T1. Inputs: this spec §2 (R5)/§4(3)/§5; the `store.go` anchors as T1; D10/D11.
  - Expected files: `internal/storage/recovery.go`, `internal/storage/recovery_test.go`, `internal/storage/store_edge_test.go` (additions) — envelope files, no new file.
  - Allowed: as T1's. Prohibited: as T1's, plus keying the moved-entry set on entry identity alone (r25, CT24-C5 — rejected, see T3); any eviction of a protected chain element (a blocker, never a behavior).
  - RED: the AC1-10 protection/eviction subcase table fails against the core-only engine (e.g. `q5` unlinked, counts deviating) — the protection selectors exit non-zero. GREEN: selectors exit 0.
  - Deliverable + ACs: retention and protection as specified; **AC1-5, AC1-6, AC1-8, AC1-9, and AC1-10 base plus (b-realias), (b-revisit), (b-cycle), (b-hardalias), (b-hardalias2), (b-hardcycle), (b-budget) and the unlisted protection/resolution subcases (b-prot), (b-chain), (b-chain2), (b-eloop), (b-sub), (b-sub2), (b-alias), (b-alias2), (b-alias-loop), (b-mid), (b-mix), (b-vac)** (AC1-9(b)'s collision oracle pins T1's D6 reservation discipline — cross-referenced; AC1-6's envelope-level admission cases ride T3's R8 vocabulary and its `TestEmitRecoveryRecords` half rides T4's emitter contract — cross-referenced fixtures, the id owned once here).
  - Validation (all must exit 0): `go test ./internal/storage/...`; targeted `go test ./internal/storage/ -run 'TestRecoverRetention|TestRecoverRetentionBound|TestRecoverRetentionFailure|TestRecoverRetentionBothFamilies|TestRecoverPromptsRetentionBound|TestRecoverNeverOverwrite|TestRecoverPruneKeepsThisRun'`.
  - Decision log: cite D6 (AC1-9(b) cross-reference), D10, D11.
  - Blocked: if protection cannot be implemented inside `recovery.go` without a non-envelope edit (name path + reason + needed edit); if any oracle would require deleting a protected entry to pass (a defect, not a task).
- **T3 — configured-link reconciliation and envelope vocabulary.**
  - Objective: land the pre-pass chain records, the moved-entry set **keyed by original configured path + pre-move entry identity with partner-versus-self decided from the pre-pass record's recorded chain-entry paths — never a bare inode match (r25, CT24-C5)**, the ordinary run-orphaned (r20-C7/D12) and mutual loop-partner (r23-C5, owner-directed; detection (m-i)–(m-iv)) detections each gated on a live post-pass symlink entry and the moved-entry set (r24, CR23-F1/F2), the own-family reserve+rename reconciliation preserving link bytes (`Reason=orphaned_link` — R6 defines no separate loop reason; zero reconciliations ⇒ zero `orphaned_link` records, r24 CR23-F2), deterministic record order, R3-fatal failure discipline, post-reconciliation retention, the at-most-one counting rule, the second-run no-op, the originally-dangling NON-mutual carve-out left unreconciled (OPEN checkpoint, §9.11) — **and the R8 envelope vocabulary**: the `quarantine_path` field, the six-reason `failure_reason` extension, the `snapshot_unavailable`/`state_directory` runtime classification with the pinned remediation literals, the `snapshot_retention_failed` fixed message `"quarantine retention failed"`, `docs/logging-contract.md` tables/union, and the `EnvelopeFieldNames` count pin 129→130.
  - Depends: T1 (the core that records moved entries and quarantines), T2 (the retention pass that runs after reconciliation and the protection of reconciled entries). Inputs: the `store.go` anchors as T1 (incl. `:112-126` `SavePrompts` guard — read-only); `envelope.go:244-` `addEnvelopeFields`, `:103-` `runtimeErrorInfo`, `:136-141` `validErrorInfo` (exact-equality admission), `:575-706` field array; `log.go:227-276` `eventMessage` (default `"mcp-bridge event"` at `:270-275`); `docs/logging-contract.md` tables; `config_envelope_test.go:166-172` pin; read-only producer anchors `cmd/mcp-bridge/wiring.go:486-493` and `cmd/mcp-bridge/startup_failure.go:26-33` (CR13-06).
  - Expected files: `internal/storage/recovery.go`, `internal/storage/recovery_test.go`, `internal/observe/envelope.go`, `internal/observe/log.go`, `internal/observe/log_test.go`, `internal/observe/edge_test.go`, `docs/logging-contract.md`, `cmd/mcp-bridge/config_envelope_test.go` — envelope files, no new file.
  - Allowed: the detection/reconciliation logic inside `recovery.go`; the R8 optional event-specific envelope additions. Prohibited: `schema_version` change; removing/renaming/retyping any existing field; reconciling a configured link already dangling before the run in a NON-mutual class (owner-flagged OPEN, §9.11); keying the moved-entry set on entry identity/inode alone (r25, CT24-C5 — rejected); new dependencies.
  - RED: the reconciliation oracles fail against the T1+T2 engine — no reconciliation fires, and the r25 hard-linked layout mis-keys the moved-entry guard (the CT24-C5 oracle FAILs); envelope RED: `log_test.go` independent union literal gains `quarantine_path` → `TestEnvelopeFieldNamesMatchesIndependentR8Union` fails (`DeepEqual` 129≠130); new `edge_test.go` admission cases fall back — `go test ./internal/observe/ -run 'TestEnvelopeFieldNames|TestLog'` and the reconciliation selectors exit non-zero. GREEN: selectors exit 0.
  - Deliverable + ACs: reconciliation and vocabulary as specified; **AC1-10(b-xA), (b-xB), (b-xA2), (b-xB2), (b-xB3), (b-xB-res), and (b-dangle)**; **AC2-9 — all subcases: the orphaned-link startup subcase (its `Readlink` assertion), the mutual-pair reconciliation subcase (partner-alone counting, `Readlink`, both-moved zero-reconciliation, seeded mutual-class retention), the r25 mutual hard-linked-symlink variant (CT24-C5: the re-keyed moved-entry identity end-to-end), and the non-mutual originally-dangling limitation subcase (the OPEN carve-out pinned, not falsely passed)**; the R8 envelope-level oracles (six-reason admission table, `snapshot_unavailable` shape case, fixed message) cross-referenced from AC1-2/AC1-6/AC2-2 (ids owned elsewhere).
  - Validation (all must exit 0): `go test ./internal/storage/...`; `go test ./internal/observe/...`; `go test ./cmd/mcp-bridge/ -run TestEnvelopeFieldNames`; targeted `go test ./cmd/mcp-bridge/ -run 'TestSnapshotRecoveryOrphanedLink|TestSnapshotRecoveryMutualLink|TestSnapshotRecoveryMutualBothMoved|TestSnapshotRecoveryMutualHardlinked|TestSnapshotRecoveryNonMutualDangling'`.
  - Decision log: cite D6 (reconciliation reserve+rename), D8/D9 (vocabulary emission/message), D12.
  - Blocked: if the reconciliation cannot be implemented inside `recovery.go` without touching `internal/service/**` or the `SavePrompts` guard (name path + reason + needed edit); if a docs/test contradiction with `logging_contract_test.go`'s closed event map appears (that file is **not** in the envelope — report the path and the exact needed edit as a blocker instead of widening scope).
- **T4 — startup integration and the end-to-end gate (new task id, owner-authorized; adds no file).**
  - Objective: wire the env gate (D4), the single `RecoverRejected` call site, `emitRecoveryRecords` (pinned retention-record literals; the `orphaned_link` reason rides the per-`Recovery` mapping — no new emitter branch; zero reconciliations ⇒ zero `orphaned_link` records, r24 CR23-F2), the cause-preserving `recoveryStateDirectoryFailure` fatal mapping, the startup integration oracles — **and own the cross-ticket end-to-end gate**: the whole startup path proven in one suite (open → recover (quarantine/reconcile) → admission → publish → prompt persistence → restart no-op), the **mutual-pair restart oracle** (fresh valid `snapshot.json`; fresh **regular** `prompts.json` accepted by direct `SavePrompts` — the refusal wrap FAILs), the **both-moved and self-moved no-fire** gate assertions (a self-referential single configured link run: zero reconciliations, zero `orphaned_link` records — a gate assertion, no new AC id), **exactly-once reconciliation counting per run**, and the **cumulative fixtures rule: no fixture may be removed, renamed or weakened when its defect is claimed closed; every closure claim must cite the surviving fixture.**
  - Depends: T1, T2, T3. Inputs: `serve.go:50` (logger), `:96-99` (Open block), `:215` (Durable wiring), `:340` (error mapping); `startup_failure.go:26-33` (`stateDirectoryFailure`, read-only), `:108-112` (`startupFailureForError`, read-only); `main.go:56-66` (errors.As mapping, read-only); `wiring.go:486-493` (off-arm record, read-only).
  - Expected files: `cmd/mcp-bridge/serve.go`, `cmd/mcp-bridge/snapshot_recovery_test.go` (new: `emitRecoveryRecords` unit tests + AC2-1..AC2-7 integration), `cmd/mcp-bridge/serve_edge_test.go` (additions: AC2-4/AC2-5 lanes), `cmd/mcp-bridge/startup_failure_test.go` (addition: pin `stateDirectoryFailure`'s exact record fields + the `recoveryStateDirectoryFailure` chain oracle for the recovery-fatal route) — **plus the AC2-8 oracles landed in `internal/storage/recovery_test.go` (an envelope file, no new file)**.
  - Allowed: the call site, gate, emitter, the `errors.Is(err, storage.ErrSnapshotUnrecoverable)` → `stateDirectoryFailure` mapping and its unexported cause-preserving wrapper helper in `serve.go`. Prohibited: edits to `startup_failure.go`, `wiring.go`, `main.go`, or `internal/service/**` (all read-only; a required change there is a blocker, not a task); removal, renaming, or weakening of any fixture from T1/T2/T3's suites (the cumulative fixtures rule).
  - RED: `snapshot_recovery_test.go` runs `runContext` with a corrupt snapshot expecting exit 0 + one enriched warn record → today exit 1 with `startup_failed`/`internal_error` → assertion failure (`startup exit=1, want 0; stderr contains startup_failed`). `go test ./cmd/mcp-bridge/ -run TestSnapshotRecovery` exits non-zero. GREEN: selector exits 0.
  - Deliverable + ACs: startup recovery wiring and the end-to-end gate; **AC2-1..AC2-8** (AC2-8's storage-level seam oracles land in `internal/storage/recovery_test.go`; AC2-9's subcases remain T3's — the gate re-runs the mutual-pair restart, both-moved/self-moved no-fire, and exactly-once counting assertions as gate assertions over them).
  - Validation (all must exit 0): `go test ./cmd/mcp-bridge/...`; targeted `go test ./cmd/mcp-bridge/ -run 'TestSnapshot|TestServeStartup|TestStartupFailure|TestRecoveryStateDirectoryFailure|TestRecoverSeams|TestRecoverReservationExhaustion|TestRecoverNonRegular'`; the cross-ticket end-to-end suite — all four tickets' suites plus the cumulative fixtures file pass in one run.
  - Decision log: cite D4, D5, D8, D12 (record mapping).
  - Blocked: if `startup_failure.go`/`wiring.go`/`main.go` edits become required (name path + reason + needed edit), or if the refresher republish, the mutual-pair restart oracle, or the second-run no-op — including the both-moved/self-moved no-fire and exactly-once counting gate assertions — cannot be observed without touching non-envelope files.
```

**Block 30 — §6.1 ENVELOPE, allowed task ids**
- section: §6.1, Allowed task ids line
- oldAnchor:
```
- **Allowed task ids:** `CR1`, `CT1`, `T1`, `T2`, `T3`.
```
- newText:
```
- **Allowed task ids:** `CR1`, `CT1`, `T1`, `T2`, `T3`, `T4` (T4 added r25 by the owner-authorized four-ticket decomposition — a task id only: the write set below is UNCHANGED, still exactly the same 14 files).
```

**Block 31 — §7 actionable events, re-key echo**
- section: §7, Actionable events bullet, reconciled-link clause
- oldAnchor:
```
(one record per reconciled family, family attribution from the `Recovery` entry; the mutual record's deterministic order after the snapshot `read_error` record is pinned by AC2-9)**
```
- newText:
```
(one record per reconciled family, family attribution from the `Recovery` entry; the mutual record's deterministic order after the snapshot `read_error` record is pinned by AC2-9; the moved-entry set is keyed by original configured path + pre-move entry identity with partner-versus-self decided from the recorded chain-entry paths — never a bare inode match (r25, CT24-C5; record shape unchanged))**
```

**Block 32 — §7 actionable events, task re-point**
- section: §7, Actionable events bullet, admission clause
- oldAnchor:
```
`failure_reason` from the closed extended vocabulary — all six reasons table-driven through admission, AC2-2/T2). No cause text is admitted into any field (D5).
```
- newText:
```
`failure_reason` from the closed extended vocabulary — all six reasons table-driven through admission, AC2-2/T3 — the envelope-vocabulary task under the r25 decomposition). No cause text is admitted into any field (D5).
```

**Block 33 — §7 operations bullet, implementer update**
- section: §7, Operations/alert-coverage bullet, tail
- oldAnchor:
```
implemented by T2/T3; failure visibility of the logging path itself is unchanged (one write attempt, `logging_write_failed` fallback).
```
- newText:
```
implemented under the r25 decomposition by T2 (AC1-6's retention records), T3 (the R8 vocabulary those records admit through), and T4 (AC2-2/AC2-3/AC2-4/AC2-7); failure visibility of the logging path itself is unchanged (one write attempt, `logging_write_failed` fallback).
```

**Block 34 — §8(2), re-key echo**
- section: §8 Irreversible steps, step (2), mutual-reconciliation clause
- oldAnchor:
```
and never an absent or self-moved entry, r24 CR23-F1/F2: a configured entry this run moved is ineligible, absence a recorded no-op)
```
- newText:
```
and never an absent or self-moved entry, r24 CR23-F1/F2: a configured entry this run moved is ineligible, absence a recorded no-op — ineligibility decided on the moved-entry set keyed by ORIGINAL CONFIGURED PATH + pre-move entry identity, never the inode alone (r25, CT24-C5: a hard link sharing the moved entry's inode at a different configured path is a different entry and never a false self-match; the retention protection in this step is unchanged by the re-key))
```

**Block 35 — §9 intro, r25 disclosure**
- section: §9 CRITIC HANDOFF intro parenthetical, tail
- oldAnchor:
```
r24 likewise reads no new source — every r24 amendment is design/AC/decision text level (the CR23-F1/F2 predicate correction, F3 oracles, F4 key definition, F5 citation, and F6 spelling resolve by spec-text reasoning over the r23 rules)**
```
- newText:
```
r24 likewise reads no new source — every r24 amendment is design/AC/decision text level (the CR23-F1/F2 predicate correction, F3 oracles, F4 key definition, F5 citation, and F6 spelling resolve by spec-text reasoning over the r23 rules); r25 likewise reads no new source — every r25 amendment is design/AC/decision/task text level (the CT24-C5 re-key and counterexample resolve by spec-text reasoning over the r24 D12 rules and the `link(2)` hard-link capability cited from the r22/r23 critic evidence, and the four-ticket decomposition applies the owner-authorized proposal `.omp/artifacts/snapshot-recovery-ticket-decomposition-r25.md` (owner authorization 2026-11-21) verbatim in mapping)**
```
**Block 36 — §9.2, blocked-condition re-point**
- section: §9 item 2, tail
- oldAnchor:
```
`logging_contract_test.go` closed map — the latter is outside the envelope; see T2's blocked condition).
```
- newText:
```
`logging_contract_test.go` closed map — the latter is outside the envelope; see T3's blocked condition (the envelope-vocabulary task under the r25 decomposition)).
```

**Block 37 — §9.21, CR24 token fix**
- section: §9 item 21, counting-rule clause
- oldAnchor:
```
with zero `orphaned_label` records when zero reconciliations fire)
```
- newText:
```
with zero `orphaned_link` records when zero reconciliations fire — token corrected r25, CR24-LOW: R6's closed set has no `orphaned_label` value)
```

**Block 38 — §9, new item 22 (r25 review targets), inserted after item 21**
- section: §9, after item 21's final sentence
- oldAnchor:
```
the non-mutual spelling (**F6** — every former `non-mutal` occurrence now reads `non-mutual`).
```
- newText:
```
the non-mutual spelling (**F6** — every former `non-mutal` occurrence now reads `non-mutual`).
22. **r25-specific review targets (round 15, out-of-band; CT24/CR24 carried):** the moved-entry re-key (**CT24-C5** — confirm R2/D12/§4's `RecoverRejected` contract/§4(3)/§4's rejected alternatives/§8(2)/T3/AC2-9 agree that every renamed entry is recorded by its ORIGINAL CONFIGURED PATH alongside its pre-move entry identity, that ordinary (iii)/(iv) and mutual (m-iii)/(m-iv) partner-versus-self is decided from the pre-pass record's recorded chain-entry paths — never a bare inode match — that the hard-linked-symlink counterexample (`snapshot.json → snapshot.json`, `prompts.json` a hard link to that symlink, both chains looping, pass 1 quarantining snapshot `read_error`, prompts dangling and its pass no-oping, the inode-only guard excluding the mutual reconciliation, admission republishing the catalog, `SavePrompts` refusing, restart re-quarantining prompts) is stated in D12 and excluded by the re-key, and that retention protection (D10/D11 — entry identity + cleaned path) is unchanged and independent of it; confirm the new AC2-9 mutual hard-linked-symlink variant pins exactly [snapshot `read_error`, prompts `orphaned_link`], the `Readlink` preserved target text, the direct `SavePrompts` acceptance, the second-run no-op, no missing-source fatal, and the counting discrimination — clean pair exactly ONE, both-moved and self-moved still ZERO — with `TestSnapshotRecoveryMutualHardlinked` added to the verification line); the evidence-precision wording (**CT24-C8** — no text implies executed GREEN for the strict-key fixtures; §9.6 stands: tests planned, none run); the CR24 LOWs (the `orphaned_link` token, the (m-ii)+(m-iii) citation); and the four-ticket decomposition (**owner-authorized, recorded** — confirm every R/AC id is owned by exactly one of T1–T4, the 14-file envelope is unchanged, T4 adds a task id without adding files, and no review, approval, owner confirmation beyond the recorded decomposition authorization, or test execution is claimed). Historical task-id references inside the round entries 9.9–9.21 retain their pre-r25 meanings (T1 storage engine, T2 envelope vocabulary, T3 serve wiring).
```

**Block 39 — r22→r23 change log, CT24-C8 wording fix**
- section: CHANGE LOG — r22 → r23, C4 entry, tail
- oldAnchor:
```
original fixtures (b-realias)/(b-cycle)/(b-revisit) unchanged and still passing under the stricter key.
```
- newText:
```
original fixtures (b-realias)/(b-cycle)/(b-revisit) unchanged and remaining required planned oracles under the stricter key — no executed GREEN is implied (r25, CT24-C8; aligned with §9.6: tests planned, none run).
```

**Block 40 — new CHANGE LOG section, inserted before the r23 → r24 heading**
- section: after the `---` preceding the change-log area, before `### CHANGE LOG — r23 → r24`
- oldAnchor:
```
### CHANGE LOG — r23 → r24 (non-normative; for the coordinator's revision ledger)
```
- newText:
```
### CHANGE LOG — r24 → r25 (non-normative; for the coordinator's revision ledger; round 15, out-of-band — the canonical execution state remains r16)

- **Four-ticket decomposition (owner-authorized 2026-11-21, "authorize planner spec update"; `.omp/artifacts/snapshot-recovery-ticket-decomposition-r25.md`) — APPLIED IN TEXT.** §6's T1–T3 tasks are re-decomposed into four behavior-owned tasks: **T1 quarantine core** (R6/R7/R9; AC1-1..AC1-4, AC1-7; the continue-startup behavior verified by T4's AC2-1..AC2-3), **T2 retention and protection** (R5, D10/D11; AC1-5, AC1-6, AC1-8, AC1-9, AC1-10 base + (b-realias)/(b-revisit)/(b-cycle)/(b-hardalias)/(b-hardalias2)/(b-hardcycle)/(b-budget) + the unlisted protection subcases (b-prot)/(b-chain)/(b-chain2)/(b-eloop)/(b-sub)/(b-sub2)/(b-alias)/(b-alias2)/(b-alias-loop)/(b-mid)/(b-mix)/(b-vac)), **T3 configured-link reconciliation + R8 envelope vocabulary** (R2/R8, D12; AC1-10(b-x: (b-xA)/(b-xB)/(b-xA2)/(b-xB2)/(b-xB3)/(b-xB-res)) and (b-dangle); AC2-9 all subcases incl. the non-mutual carve-out pin), **T4 startup integration and end-to-end gate** (NEW task id; R1/R3/R4; AC2-1..AC2-8; the cross-ticket end-to-end suite, the mutual-pair restart oracle, both-moved/self-moved no-fire, exactly-once counting per run, and the cumulative fixtures rule — no fixture removed/renamed/weakened on a closure claim; closure claims cite the surviving fixture). Dependencies: T1→T2,T3; T2→T3; T1,T2,T3→T4. **Every R/AC id is preserved and owned by exactly one task; the §6.1 14-file write envelope is UNCHANGED; T4 adds a task id without adding files.** The authorization grants specification authorship only — no implementation, waiver, review retry, or pipeline transition. Stale pre-r25 task references in live sections are re-pointed; historical round entries 9.9–9.21 keep their pre-r25 meanings.
- **CT24-C5 (HIGH, lifecycle hazard — D12 moved-entry identity) — RE-APPLIED IN TEXT (round 14's text was never persisted; the working spec was still r24 and the defect stayed open).** The moved-entry set is re-keyed: every renamed entry is recorded by its ORIGINAL CONFIGURED PATH alongside its pre-move entry identity; ordinary (iii)/(iv) and mutual (m-iii)/(m-iv) partner-versus-self is decided from the pre-pass record's recorded chain-entry paths, never a bare inode match. The counterexample — `snapshot.json` a symlink targeting `snapshot.json`, `prompts.json` a hard link to that symlink (same inode/target text, different entry), both pre-pass chains looping, pass 1 quarantining snapshot `read_error`, prompts dangling and its pass no-oping, the inode-only guard reading prompts as already moved and excluding the reconciliation, admission republishing the catalog, `SavePrompts` refusing, restart re-quarantining prompts — is stated in D12, with why the inode-only key excludes reconciliation and reproduces the self-healing failure, and is pinned by the new AC2-9 mutual hard-linked-symlink variant (exactly [snapshot `read_error`, prompts `orphaned_link`]; `Readlink` preserved link text; fresh regular `prompts.json` accepted by direct `SavePrompts`; second-run no-op; no missing-source fatal; counting discrimination — both-moved and self-moved still ZERO, clean pair still exactly ONE; named test `TestSnapshotRecoveryMutualHardlinked` added to the verification line). Echoed in R2, §4's `RecoverRejected` contract, §4(3)'s pre-pass-records and reconciliation paragraphs, §4's rejected alternatives (the inode-only key rejected), §8(2), T3's objective/deliverable, §7, and §9.22. **Retention protection (D10/D11 — entry identity + cleaned path) is separate and unchanged.**
- **CT24-C8 (LOW, evidence precision) — APPLIED IN TEXT.** The r22→r23 change log's "unchanged and still passing under the stricter key" now reads that the (b-realias)/(b-cycle)/(b-revisit) fixtures remain required planned oracles — no executed GREEN is implied; aligned with §9.6 (tests planned, none run).
- **CR24 LOWs (advisory, previously unassigned) — APPLIED IN TEXT.** §9.21's `orphaned_label` token fixed to the actual reason value `orphaned_link` (R6's closed set); AC1-10(b-dangle) now cites (m-ii)+(m-iii) — the deciding conditions — rather than (m-iii) alone, mirroring AC1-1(d)'s CR23-F5 fix.

**Open items carried unchanged:** the retention-unlink owner checkpoint (§8(2)/§9.11), the originally-dangling NON-mutual carve-out (§9.11, pinned by AC2-9's non-mutual subcase), and the requested non-OpenAI escalated critic (§9.12) remain OPEN; the mutual-link reconciliation remains the only owner direction claimed; no review, approval, owner confirmation beyond the recorded decomposition authorization, or test execution is claimed this round.

### CHANGE LOG — r23 → r24 (non-normative; for the coordinator's revision ledger)
```

## REGIONS NOT UNAMBIGUOUSLY ANCHORABLE

None. Every block above carries a verbatim oldAnchor verified against the raw r24 text and matching exactly once (the two `(T2 anchor, CR13-06)` occurrences are disambiguated by their distinct preceding context in Blocks 5 and 7; the three `AC2-2/T2` occurrences by theirs in Blocks 22, 24, and 32). Deliberate non-edits: the historical §9 round entries 9.9–9.21 (except the CR24-mandated token fix in Block 37) and the historical r23→r24 / r22→r23 change-log bodies (except the CT24-C8 fix in Block 39) are left as recorded history — their pre-r25 task references are scoped by the note in the new §9.22 (Block 38), not silently rewritten.

## SELF-CHECK (acceptance)

- Every R/AC id appears exactly once in the mapping and is owned by exactly one of T1–T4 (Blocks 29, 40): R1/R3/R4→T4, R2/R8→T3, R5→T2, R6/R7/R9→T1; AC1-1..4,7→T1; AC1-5,6,8,9 + AC1-10(base+14 subcases)→T2; AC1-10(b-x group, b-dangle)→T3; AC2-1..8→T4; AC2-9→T3. No id added, removed, or renumbered.
- The 14-file envelope is unchanged; T4 adds a task id without adding files (Blocks 29, 30, 40).
- The C5 counterexample is stated in D12 (Block 15) and pinned by the AC2-9 mutual hard-linked-symlink variant (Block 27); the inode-only alternative is explicitly rejected in D12 (Block 16) and §4's alternatives (Block 21).
- No `orphaned_label` remains (Block 37); the (m-ii)+(m-iii) citation is stated (Block 23).
- Dependencies are T1→T2,T3; T2→T3; T1,T2,T3→T4 (Blocks 29, 40).
- Open items marked as staying open: retention-unlink checkpoint, originally-dangling non-mutual carve-out, requested non-OpenAI escalation (Blocks 3, 40).
- No review, approval, owner confirmation beyond the recorded decomposition authorization, or test execution is claimed anywhere; RED/GREEN entries remain planned oracles.
- Nothing here was executed against files: this is proposed text only (planner is read-only); persistence and spec-lint are the coordinator's.

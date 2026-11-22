<<<BLOCK 1>>>
--- old ---
# SNAPREC-SPEC-r26 — snapshot-recovery (revision, plan round 16)
--- new ---
# SNAPREC-SPEC-r27 — snapshot-recovery (revision, plan round 17)
<<<END>>>

<<<BLOCK 2>>>
--- old ---
- **Spec ID / revision:** `SNAPREC-SPEC-r26`, revision **r26**, plan round **16** (dispatched by the coordinator's round-16 request `SnapshotRecoveryRound16`, out-of-band
--- new ---
- **Spec ID / revision:** `SNAPREC-SPEC-r27`, revision **r27**, plan round **17** (dispatched by the coordinator's round-17 request `SnapshotRecoveryRound17`, out-of-band
<<<END>>>

<<<BLOCK 3>>>
--- old ---
- **Round:** r26 revises the installed r25 candidate in place — round 16, dispatched out-of-band by the coordinator's round-16 request `SnapshotRecoveryRound16` (scoped to the authoring dispatches of rounds 13–15
--- new ---
- **Round:** r27 revises the installed r26 candidate in place — round 17, dispatched out-of-band by the coordinator's round-17 request `SnapshotRecoveryRound17` (scoped to the authoring dispatches of rounds 13–16
<<<END>>>

<<<BLOCK 4>>>
--- old ---
Seven textual corrections closing the bound r25 CodeReviewer findings (`.omp/artifacts/snapshot-recovery-fresh-plan/cr-r25-raw.md`; details in the r25→r26 change log), with
--- new ---
Three textual corrections closing the bound r26 CodeReviewer findings (details in the r26→r27 change log), with
<<<END>>>

<<<BLOCK 5>>>
--- old ---
the 14-file write set unchanged, no envelope path change, and no new dependency (std-lib only)**: (1) this round bullet is split
--- new ---
the 14-file write set unchanged, no envelope path change, and no new dependency (std-lib only)**: (1) §9.8's "Envelope changes: none" headline and body are rewritten (CR26-1) — envelope: no write-path change; `allowedTaskIds` gained T4 at r25 by owner authorization; the 14-file write set is unchanged; (2) the §3 decision ledger and D12's header amendment list record the r26 amendment (CR26-2) — D12: the final target's cleaned path as a chain-entry path (CR25-5); (3) the r25 CR25-5 pre-pass-record clauses are scoped (CR26-3) — the added final-target path is operative only for ordinary (iii); (m-iii) still matches moved link entries. No new source read is required this round: every r27 amendment is spec-text level. **Historical, scoped to r26 — not statements about this r27 text — r26's seven textual corrections closing the bound r25 CodeReviewer findings (`.omp/artifacts/snapshot-recovery-fresh-plan/cr-r25-raw.md`; details in the r25→r26 change log)**: (1) this round bullet is split
<<<END>>>

<<<BLOCK 6>>>
--- old ---
so ordinary (iii)/(m-iii) moved-element path matching can match a moved regular final target (AC1-10(b-xB2)/(b-xB3)) — no semantic redesign of the moved-entry key beyond that;
--- new ---
so ordinary (iii) moved-element path matching can match a moved regular final target (AC1-10(b-xB2)/(b-xB3)) — the added final-target path being operative only for ordinary (iii), (m-iii) still matching moved link entries (scoped r27, CR26-3) — no semantic redesign of the moved-entry key beyond that;
<<<END>>>

<<<BLOCK 7>>>
--- old ---
No new source read is required this round: every r26 amendment is spec-text level.
--- new ---
No new source read was required that round (r26): every r26 amendment was spec-text level.
<<<END>>>


<<<BLOCK 9>>>
--- old ---
amended r25 — CT24-C5: the moved-entry set re-keyed by original configured path + pre-move entry identity, partner-versus-self from the recorded chain-entry paths — never a bare inode match):**
--- new ---
amended r25 — CT24-C5: the moved-entry set re-keyed by original configured path + pre-move entry identity, partner-versus-self from the recorded chain-entry paths — never a bare inode match; amended r26 — CR25-5: the final target's cleaned path recorded as a chain-entry path, operative only for the ordinary (iii) moved-element match):**
<<<END>>>

<<<BLOCK 10>>>
--- old ---
(r26, CR25-5: so ordinary (iii)/(m-iii) moved-element matching against the recorded chain-entry paths can match a regular final target this run moved — AC1-10(b-xB2)/(b-xB3) — with no redesign of the moved-entry key beyond the added recorded path)
--- new ---
(r26, CR25-5 — scoped r27, CR26-3: so ordinary (iii) moved-element matching against the recorded chain-entry paths can match a regular final target this run moved — AC1-10(b-xB2)/(b-xB3) — with no redesign of the moved-entry key beyond the added recorded path; the added final-target path is operative only for ordinary (iii), and (m-iii) still matches moved link entries)
<<<END>>>

<<<BLOCK 11>>>
--- old ---
(r26, CR25-5: ordinary (iii)/(m-iii) moved-element matching against the recorded chain-entry paths can thereby match a moved regular final target — AC1-10(b-xB2)/(b-xB3))
--- new ---
(r26, CR25-5 — scoped r27, CR26-3: ordinary (iii) moved-element matching against the recorded chain-entry paths can thereby match a moved regular final target — AC1-10(b-xB2)/(b-xB3); the added final-target path is operative only for ordinary (iii), and (m-iii) still matches moved link entries)
<<<END>>>

<<<BLOCK 12>>>
--- old ---
8. **Envelope changes:** none — all mirrored ids kept; no R/AC id added, removed, or renamed; T4 was added as a task id at r25 by owner authorization (the owner record `.omp/artifacts/snapshot-recovery-fresh-plan/round-15-seed.json` `ownerAuthorization`, 2026-11-21) and adds no file; the §6.1 file envelope equals the owner-approved 14-file write set, unchanged, with no additional write path requested (the r20 resolver, pre-pass records, reconciliation, and all r20/r21/r22/r23/r24/r25 oracles land inside the existing `recovery.go`/`recovery_test.go`/`snapshot_recovery_test.go` files; the r23 hard-link fixtures need only std-lib `os.Link` — no new dependency). This §9.8 statement applies to its live text only — the historical round entries 9.9–9.21 keep their own pre-r25, round-scoped meanings. No blocker is open.
--- new ---
8. **Envelope:** no write-path change — `allowedTaskIds` gained T4 at r25 by owner authorization; the 14-file write set is unchanged. All mirrored ids kept; no R/AC id added, removed, or renamed; T4 was added as a task id at r25 by owner authorization (the owner record `.omp/artifacts/snapshot-recovery-fresh-plan/round-15-seed.json` `ownerAuthorization`, 2026-11-21) and adds no file; the §6.1 file envelope equals the owner-approved 14-file write set, unchanged, with no additional write path requested (the r20 resolver, pre-pass records, reconciliation, and all r20/r21/r22/r23/r24/r25 oracles land inside the existing `recovery.go`/`recovery_test.go`/`snapshot_recovery_test.go` files; the r23 hard-link fixtures need only std-lib `os.Link` — no new dependency). This §9.8 statement applies to its live text only — the historical round entries 9.9–9.21 keep their own pre-r25, round-scoped meanings. No blocker is open.
<<<END>>>

<<<BLOCK 13>>>
--- old ---
### CHANGE LOG — r25 → r26 (non-normative; for the coordinator's revision ledger; round 16, out-of-band — textual corrections only; no R/AC id changed or added; T4 stays the task id added at r25 by owner authorization; no envelope path change; no new dependency)
--- new ---
### CHANGE LOG — r26 → r27 (non-normative; for the coordinator's revision ledger; round 17, out-of-band — textual corrections only; no R/AC id changed or added; T4 stays the task id added at r25 by owner authorization; no envelope path change; no new dependency)

- **CR26-1 (MEDIUM, §9.8 headline contradiction) — CLOSED IN TEXT.** §9.8 opened "**Envelope changes:** none" while §6.1's `allowedTaskIds` had gained T4 at r25 (CR25-2 restated the body but left the headline claiming none). §9.8's headline and body are rewritten: "**Envelope:** no write-path change — `allowedTaskIds` gained T4 at r25 by owner authorization; the 14-file write set is unchanged."
- **CR26-2 (LOW, decision-ledger amendment gap) — CLOSED IN TEXT.** The r26 D12 amendment (the final target's cleaned path as a chain-entry path, CR25-5) was applied to D12 (§3) and §4(3) but never recorded in the decision ledger. Added at §3's ledger: `amended r26 (D12 — final target's cleaned path as a chain-entry path, CR25-5)`; added at D12's header: `amended r26 — CR25-5`.
- **CR26-3 (LOW, over-scoped clause) — CLOSED IN TEXT.** The r25 CR25-5 clause read "ordinary (iii)/(m-iii)", but the added final-target path is operative only for ordinary (iii) — it lets (iii) match a moved regular final target; (m-iii) still matches moved link entries. Scoped to `ordinary (iii)` at the three live sites (§1 round bullet item (5), §3 D12, §4(3)); the historical r25→r26 change-log entry keeps its round-scoped wording.
- **Housekeeping (r27):** the title, Spec ID, and Round header move to `SNAPREC-SPEC-r27` / revision r27 / plan round 17 (`SnapshotRecoveryRound17`); the prior r26 round description is carried as an explicitly scoped historical paragraph; all earlier history keeps its round-scoped wording.

**Unchanged (r27):** every R/AC id, every task id (T1–T4; T4 stays), the §6.1 14-file envelope, the task decomposition, D1–D12, and all r13–r26 closures carried forward; std-lib only; no review, approval, or test execution is claimed this round.

### CHANGE LOG — r25 → r26 (non-normative; for the coordinator's revision ledger; round 16, out-of-band — textual corrections only; no R/AC id changed or added; T4 stays the task id added at r25 by owner authorization; no envelope path change; no new dependency)
<<<END>>>
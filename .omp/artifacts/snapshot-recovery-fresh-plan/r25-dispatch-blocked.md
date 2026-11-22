# r25 review dispatch — blocked record

**Date:** 2026-11-21
**Feature:** snapshot-recovery (out-of-band plan loop)

## What completed

- Planner round 15 returned a 40-block r24 → r25 delta; full text recovered from the planner session file (the structured result kept only the handoff).
- Delta applied deterministically; the spec was snapshotted before the write.
- Spec-lint: exit 0, the same 20 pre-existing findings as r23/r24 (ticket-mirror drift + shared oracle); no new finding.

## Evidence

- Spec: `.scratch/snapshot-recovery/spec.md`
- SHA-256: `388cd123a474f553a0eee35a8e6932bd26047854ad2f7c6b579a56eabc7041d5`
- Bytes: 327090 (r24: 290216)
- Pre-write archive: `.omp/artifacts/snapshot-recovery-fresh-plan/archive-before-r25-382dac82a08289694048cdc201870086349cf5260e25e31335b8e60f00c79f1e.md`
- Delta: `.omp/artifacts/snapshot-recovery-fresh-plan/r25-delta.md`
- Applier: `.omp/artifacts/snapshot-recovery-fresh-plan/apply-r25-delta.mjs` (40/40 anchors applied; post-condition checks re-run)
- Lint: `.omp/artifacts/spec-lint-r25.out` (20 findings, none new)
- Structure: 9 sections; §6 task ledger T1–T4; §6.1 allowed task ids include T4; write set unchanged.
- Review candidate registered: `snapshot-recovery-spec-r25`, 12 inputs, dispatches CR1/CT1.

## Blocked at review dispatch

1. `task` (native) without wrapper → `E_REVIEW: A required independent review observation is missing or blocked … ambiguous or missing dispatch`.
2. `task` with the full seed as `context` → `E_DISPATCH_METADATA`.
3. `task` with `{workflowDispatch, instructions}` as `context` → `E_STATE_SOURCE` (seed revision r25 vs canonical r16).
4. `prepare-dispatch review-seed-cr25.json` → `{"ok":false,"dispatch_allowed":false,"reasons":[{"code":"E_STATE_SOURCE"}]}`.
5. After those attempts, native `task` with the canonical revision in the wrapper → `E_RETRY_NOT_AUTHORIZED`.

## Diagnosis

The review gate binds a dispatch to the canonical execution state. Canonical state is `r16` with `allowedTaskIds` PLAN1, CR1, CT1, T1, T2, T3 — it does not contain the r25 candidate or task T4, so no emitted payload can pass `E_STATE_SOURCE`; the local retry counter for the reviewer dispatch is now also refusing further attempts. Rounds 10–14 used out-of-band seeds, but that path no longer admits new attempts here.

## Required to proceed

1. Owner authorization to install the r25 plan into canonical state (revision r25, task ids including T4, same 14-file envelope) under the existing transition/installation operation; or
2. An explicit owner directive on the dispatch route to use for this out-of-band round, including how the retry refusal should be cleared.

No review has run on r25. No approval, waiver, implementation authority or completion is claimed. Canonical state was not modified.

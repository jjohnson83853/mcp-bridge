# Decision point after review 22 — the repair loop is generating HIGH-severity defects

Date: 2026-11-26 · Spec: `.scratch/token-efficient-tool-responses/spec.md` r22-replacement (66,228 B)

## Evidence

Review outcomes, severity counts (HIGH/MEDIUM/LOW), document size:

| round | H | M | L | total | spec B |
| --- | --- | --- | --- | --- | --- |
| r16 | 1 | 1 | 2 | 4 | 55.1 K |
| r17 | 2 | 1 | 1 | 4 | 58.5 K |
| r18 | 1 | 1 | 3 | 5 | 61.7 K |
| r19 | 1 | 6 | 1 | 8 | 63.8 K |
| r21 | 1 | 2 | 2 | 5 | 64.7 K |
| r22 | 4 | 2 | 1 | 7 | 66.2 K |

Four HIGHs in r22 is the worst result since r8. Causes are traceable to the last three repairs, not to the original design:

- F1 fill/packing rule unpinned — introduced when r20/r22 tightened literal-byte oracles without defining how a page is filled.
- F2 per-namespace refusal unreachable and carrying the aggregate terminal — introduced by r22 item 4.
- F3 `invalid_handle` no longer encodes eviction vs R16's cap-evicted marker — introduced by r20's reserve rule interacting with r16's cap rule.
- F4 no observable predicate distinguishes session-ful no-epoch from session-less — pre-existing seam, only now surfaced.
- F5/F6 the same two interactions; F7 OD3 arithmetic double counts.

Requirement bodies R1–R16 have been stable and un-contradicted for many rounds. The churn concentrates in mechanism interactions introduced by repair prose (reserve, charge disposition, refusal terminal, event closure).

## What works

`.tmp`-independent mechanical gate `/tmp/spec-trace-check.mjs` passes 13/13 on r22:
requirement coverage, AC→test wiring, test ownership in task ledgers, citation resolution,
deleted-exception regression guards, registry count consistency, §7 event oracles,
width pinning, single-sourcing, envelope coverage, specdd marker balance.

That gate catches the class that consumed r16/r19/r21 (AC gaps, unowned tests,
restatements, missing event oracles) in one second. It does not catch interaction
semantics — the class now failing.

## Options

A. **Decompose (recommended).** Keep R1–R16 as the requirement register; split ACs and
   mechanisms into five task-sized specs (T1–T5), each small enough to review to PASS.
   Tracked as child issues of #115. Preserves the PASS gate; each review object is ~10 KB.

B. **Implement under a recorded waiver.** Freeze r22 as-is, record the seven findings as
   implementer acceptance criteria, and let tests decide the contested semantics (fill rule,
   refusal terminal, cap-eviction code, epoch predicate). Requires an explicit owner waiver;
   the harness will not dispatch a mutating engineer otherwise. Repair prose stops here.

C. **Continue repairing.** Evidence contradicts this: three consecutive rounds increased or
   held severity while the document grew 10%.

No option is a silent acceptance of defects: A and B both carry every open finding forward
with a named owner, and B's waiver records risk acceptance rather than a pass.

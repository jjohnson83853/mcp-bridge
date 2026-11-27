# Ticket 01 spec — review round 5 findings ledger

Candidate revision r5 (`ticket01-bounded-results-v5`), verdict REQUEST CHANGES (bound).
Observation: `.omp/artifacts/token-eff01-review/observation-code-reviewer-r5-2020f2b0ff4b.json`.

## Review budget

The S15 budget was exhausted at five completed reviews. The owner explicitly reset the budget to **5** for this planning task on 2026-11-25 and authorized one repair round plus the reviews it needs. Counters restart at 0 for this allocation; historical rounds remain evidence and are never re-consumed.

## Verified closures (do not regress)

Round-4 R1 (11-file inventory, all count deltas checked against source), R3 (two-phase split), R4, R5 (ownership), R6 (prefix plus `isError`), R7 (three real seams), R8 (positive/negative split). Earlier F1, F2, F8, F12, F15 remain intact.

## Open findings

| ID | Class | Sev | Anchor | Defect |
| --- | --- | --- | --- | --- |
| M1 | statement-contradiction | HIGH | §4.4/§4.5; AC1, AC13 | Config validation checks only `max_metadata_envelope_bytes < delivery_ceiling_bytes`, but R1/AC1 bound the preview by `preview_ceiling_bytes` and §4.2/AC13 require a non-empty content prefix. A loadable config with a preview ceiling below its own metadata (JSON keys, 43-byte handle, `total_bytes` digits, `upstream_code` up to its bound) makes both impossible; only pages have a typed fallback. The envelope is also called constant while its digit widths are derived from `max_retained_result_bytes`, which cannot bound an unretained result's `total_bytes`. |
| M2 | statement-contradiction | MEDIUM | §4.2/§4.6; AC16 | The unretainable branch truncates `upstream_code` to `max_upstream_code_bytes` plus a marker, while R10/§4.6 promise the full code byte-exact in the retained result and AC16 requires the preview to keep "its own `upstream_code`". On that branch no retained result exists, so AC16 fails if the code is whole and the preview bound fails if it is not. |
| L1 | ac-not-falsifiable | LOW | §4.3; AC6 | The description half names no required observable, so a description containing no paging fields still passes `TestRetrieveResultSchema`. |
| L2 | ledger-state-drift | MEDIUM | §3; §4.5; T2 | The "four retirement seams" evidence is wrong for rehydration: `newLiveOwner` runs only when the stored epoch is 0 (`internal/service/inbound.go:609/850/921`), so rehydration never retires an epoch-bearing owner and T2's purge test is vacuous or unsatisfiable. Also, "retired owner yields `invalid_handle`" is not the observable when the caller still presents the retired epoch — exposure admission rejects first. Both observables must be stated. |
| L3 | ledger-state-drift | LOW | §8 | The four collectors and new log events are tied to AC9 (eviction) and AC10 (error class) but no R/AC or R12 test verifies them, so those deliverables can neither pass nor fail. |

## Required repair shape

- M1 is one invariant, not two: a validated configuration must always admit a preview that satisfies R1. State the feasibility constraint over the *preview* envelope including the worst-case `total_bytes` width, and the typed outcome when a page cannot admit a non-empty prefix.
- M2 is a scoping rule, not an added carve-out: the byte-exact full-code promise holds where a retained result exists; the unretainable branch states its own observable for the code.
- L1, L2, L3 must each become either a falsifiable assertion with an owning task or an explicitly advisory item. No deliverable may exist that no task can fail.

# Ticket 01 spec — Critic gate findings (round 1)

Critic verdict: REVISE on candidate r15 (`ticket01-bounded-results-v15`).
Output: `.omp/artifacts/token-eff01-review/observation-critic-r15-04b05df0c288.json`.
CodeReviewer budget: the Critic does not consume CodeReviewer units (S15). Owner raised the CodeReviewer allocation to **10**.

## Verified contract closures (do not regress)

Whole-result escaped-byte accounting; inclusive 8192/8193 boundary; 1-based indices and remaining arithmetic; pinned pages; capacity terminal; error metadata; separate retrieval schema; post-discovery-commit seam.

## Findings (all MAJOR, confidence HIGH)

| ID | Req | Defect | Closing condition |
| --- | --- | --- | --- |
| C1 | R7, AC14, T2 | R7 makes CSPRNG use advisory; a predictable content-independent generator issues distinct 43-char handles and passes every behavioural check, violating O3/R7 unguessability. | Cryptographic unpredictability is mandatory with required inspection evidence; uniqueness is not its oracle. |
| C2 | R9, T2 | Atomic creation/admission/eviction excludes retirement ordering: an admitted upstream call can be paused, its owner retired and purged, and its completion retained under a dead namespace, defeating the live-session aggregate bound. | State the retirement-versus-retention invariant and require a controlled competing-operation test proving no retired-namespace resurrection or retained-state growth. |
| C3 | R12, AC18, T2, T4–T6 | Required log fields `total_bytes` and `max_retained_result_bytes` cannot traverse the existing envelope; unknown fields reject the record (`internal/observe/envelope.go:252-488`), which is outside the write envelope. A marshal-only fixture passes while real logging emits a fallback. | Authorized scope for the envelope file and an assertion of real emission-to-sink delivery with required fields and failure visibility. |
| C4 | R12, AC12, T7 | T7 writes `coverage-ticket01.out` but bare `go run ./cmd/covcheck` reads `coverage.out` (`cmd/covcheck/main.go:22-42`), so the gate may validate stale coverage or fail despite fresh coverage. | The gate consumes exactly the newly generated profile, with one consistent artifact and cleanup path. |
| C5 | R1, T4 | R1 requires `catalog_changed` for ownerless direct projected calls; §4.0 preserves the existing `unknown_tool` (`tools.go:225-235`). | One per-surface rejection matrix preserving the existing direct taxonomy, asserted by code, execution state and zero retention. |

## Recording limitation

The Critic observation could not be appended to `review-sequence.json` through the installed binding path (it reports `critic observation or first-stage review binding is invalid` after the CodeReviewer round validates). The untouched Critic output is preserved above and in the observation artifact, and the finding set is authoritative for this repair. Receipt generation will need that binding resolved before delivery; it does not block the repair or the re-review.

# 14 - Refresh Counts and Age Assertions

Status: needs-info
Type: task

## Why this is deferred
R22 promises that a failed pass distinguishes candidate counts from retained counts and states
the retained age, but the r8 acceptance criteria assert only part of that field set. The
mismatch is between the designed record and what is actually asserted.

## Evidence
- `spec.md:62` (R22) requires the candidate-vs-retained distinction and the retained age.
- `spec.md:189` (AD16) defines `candidate_servers`, `candidate_tools`, `retained_age_ms` on the
  failed-pass records.
- `spec.md:241` (AC08-3) asserts only `retained_generation` and `catalog_tools`.
- `spec.md:240` (AC08-2) requires `next_attempt_ms` on the start **and** outcome records, while
  AD16's `catalog_refresh_completed` carries no such field (§2 of the review).
- Review: `review-code-reviewer-r8-revise.md` §2.

## Question to rule on
Whether AC08-3 is extended to the full five-field set, and whether `next_attempt_ms` belongs on
`catalog_refresh_completed` (AD16 amended) or only on the start record (AC08-2 narrowed).
The planner authors the resulting AD/AC text.

## Comments
Recorded 2026-11-11 from the r8 review.

# 15 - Reload Acceptance Record

Status: needs-info
Type: task

## Why this is deferred
R23/R24 require the log to show a configuration edit reaching the running process, accepted or
rejected. The design defines both halves; no acceptance criterion asserts either.

## Evidence
- `spec.md:64` (R24, owner rulings 10 and 18) and `decisions.md` ruling 10.
- `spec.md:202` (AD21) defines `config_reload_applied` (level `info`, outcome `success`,
  operation `reload`, fields `config_revision`, `trigger`) and the sole authorized extension of
  an r7 record — adding those same two optional fields to `config_reload_rejected`.
- `spec.md:266` maps R24 to AC08-2 and AC11-1..2 only, none of which assert the accepted half or
  the two additive fields.
- Review: `review-code-reviewer-r8-revise.md` §2.

## Question to rule on
Which AC asserts the accepted/rejected distinction with its two fields, and whether that work
stays inside T11's envelope or splits into its own ticket. Note the naming question as well:
`config_revision` (record field) versus `revision` (state field).

## Comments
Recorded 2026-11-11 from the r8 review.

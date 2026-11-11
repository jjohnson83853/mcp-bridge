# 13 - Startup Pin-Evaluation Leg

Status: needs-info
Type: grilling

## Why this is deferred
r8 required startup to report the same pin detail as the reload and refresh paths, but the
startup path performs no pin evaluation at all. The requirement cannot be satisfied as written,
and the review recorded it as a mischaracterization rather than a gap.

## Evidence
- `spec.md:259` (AC11-1) and `spec.md:284` (T11 RED) both premise a pin rejection "at startup".
- `internal/service/reload.go:554-560` sets `next.pins = nil`: no pin is resolved or rejected on
  the startup path, so a "pin rejection at startup" cannot occur.
- Review: `.scratch/structured-logging/review-code-reviewer-r8-revise.md` §3 and §6.

## Question to rule on
(a) Define the startup leg as the first post-startup refresh/discovery pass — narrated by the
    AD16 refresh records that T8 already emits — and drop the startup clause from AC11-1/T11; or
(b) authorize an explicit startup pin check in `internal/service/reload.go`, plus its write
    envelope entry and its own RED test.

Any AC text follows from the ruling; none is authored here.

## Comments
Recorded 2026-11-11 from the r8 review. T11 (`.scratch/structured-logging/issues/11-*`) is the
r8 ticket that must shrink or split accordingly.

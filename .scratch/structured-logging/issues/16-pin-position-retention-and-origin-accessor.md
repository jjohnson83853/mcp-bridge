# 16 - Pin Position Retention and Origin Accessor

Status: needs-info
Type: task

## Why this is deferred
Two r8 criteria require the pin's configuration file and revision, and neither is obtainable from
the structures the loader actually keeps. Satisfying them needs an envelope and design change,
not a test.

## Evidence
- `spec.md:249` (AC09-2) requires the pin's file; `spec.md:196` (AD18) omits `config.path`.
- `spec.md:253` (AC09-8) requires `config_revision` on the evidence; AD18 omits it.
- `internal/config/config.go:66`: `Config.Pins` is `[]map[string]string` — pin nodes are not
  retained, so a position cannot be attributed after load.
- `internal/config/config.go:78` and `:457`: `origin` is unexported with no accessor.
- `internal/config` is absent from the r8 write envelope (`spec.md:295`).
- Review: `review-code-reviewer-r8-revise.md` §3.

## Question to rule on
(a) Retain pin positions and add an `origin` accessor in `internal/config`, widening T9's write
    envelope (and its effect paths); or
(b) re-scope AC09-2/AC09-8 to what the retained structures can expose, and let AD18 drop the
    fields.

## Comments
Recorded 2026-11-11 from the r8 review.

# 17 - r8 Artifact Consistency Drift

Status: needs-info
Type: task

## Why this is deferred
Confirmed inconsistencies between the reviewed revision, the installed projection and the
decision record. None changes behaviour; all of them make the artifacts disagree about what was
authorized, so they must not be fixed one at a time while a narrowing decision is pending.

## Evidence (all verified against the files, not taken from the review on trust)
- `spec.md:76` states state revision `r6`; the installed state is `r8-pin-diagnostics`.
- The installed state's T10 envelope authorizes `internal/upstream/runtime.go`, while the spec
  permits one T10 discovery file (`dedicated.go`, `spec.md:283`, `:295`); T10's effects also add
  `cmd/mcp-bridge/serve.go` and `dedicated_test.go`, absent from the T10 ledger line.
- `spec.md:295` authorizes `internal/service/service_edge_test.go`, which is absent from the
  installed state and from every task effect.
- AC12-1/AC12-2 are `"method":"inspection"` in the projection, while `spec.md:262-263` makes them
  a RED doc-drift test.
- T10's `blockedBy` is now `["T9"]` in the installed projection (verified 2026-11-11 against `.omp/plans/execution-state.json`); the strict-serial edge the strict-serial ruling requires is present, so this drift item is resolved and retained for the audit trail only.
- `decisions.md:160,166` bind packet `26842ba7…`; state and addendum bind `358ee090…`.
- `tasks[T8].blockedBy` is `[]` in the installed projection while the strict-serial ruling requires the T7 edge (decisions.md r8 checkpoint item 1: tickets 08-12 strictly serial in ticket order); `spec.md:165` already cites this gap as recorded here.

## Question to rule on
Whether these are corrected in place or folded into the revision bump that the narrowing decision
will produce anyway. Correcting them first would change the spec hash and force the review
candidate to be rebuilt twice.

## Comments
Last three items of the envelope/state group are coordinator artifacts, inside the writable
envelope; the spec and decisions lines are the planner's and the owner's respectively.

## narrow3 additions (2026-11-11)
- The narrow2 delta (`r8-narrowing2-delta.md` §4) claims an `src-decisions` artifact.sha256 re-bind, but `.omp/plans/execution-state.json` has no `src-decisions` source entry to re-bind (verified 2026-11-11: grep over the state's sources matches only `src-grilling-pin-diagnostics`), while `decisions.md`'s "State binding note" says the file is bound as `src-decisions`. The delta's re-bind claim is void; whether the source entry is added or the note corrected belongs to this ticket.
- `spec.md:165` (the narrow2 owner-deferred-gap line) cites `spec.md:295` as the `internal/service/service_edge_test.go` authorization; that authorization exists only in the spec's write-paths list and is absent from the installed state and every task effect — same drift family as the existing T10-effects evidence bullet; recorded here so the drift list is complete.

# Owner authorization — ticket 01 tasks T2–T5 (2026-11-27)

Owner answer to the continuation decision form, verbatim:

```json
{
  "auth": "t2-t5",
  "note": "each ticket should start with a budget of 10"
}
```

Meaning: authorize sequential implementation of ticket 01 (#115) tasks T2, T3,
T4 and T5, each reporting its verification before the next begins and stopping
only at a failed verification, a genuine blocker, or an exhausted capability.
The note sets each ticket's planning review budget to 10 (S15 override), counted
per ticket across revisions, checkpoints, models and sessions.

Scope reference (coordinator annotation, not owner words): T2–T5's authoritative
write roots and deliverables are spec §5 at revision r40
(`sha256 37dd21cee4cebdd8c5e4771741e05e594feb01292e43269ace89ee11d35ad686`),
each task's `effects.paths` in `.omp/plans/execution-state.json` for revision r40.

Not authorized by this message: review-waiver or acceptance changes; changes to
the r40 spec text; edits outside each task's declared write roots.

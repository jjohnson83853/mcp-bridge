# 01 — Surface unclassified startup failures with a typed, sanitized cause

**What to build:** Every startup exit reports a specific failure class and enough context for an operator to identify the failing stage from one log line. Today every error that is not a config diagnostic or a known publication failure collapses into `startup_failed` / `internal_error` / "no specific class matched", discarding the underlying cause before it reaches stderr. The second bridge instance has been looping on that generic line and cannot be diagnosed from logs. The fix makes the unclassified path additive: a typed code naming the stage (for example snapshot recovery, catalog loop, listener), a sanitized cause summary, and a bounded stage identifier, with no secret value and no raw credential in the output. Genuinely unknown panics keep a distinct unknown-class code so they remain distinguishable from a named stage.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] A startup error that is not a config diagnostic or publication failure emits a specific class naming the stage and a sanitized cause summary.
- [ ] The snapshot-recovery failure path and at least two other startup stages each produce a distinct code in tests.
- [ ] The envelope remains schema-compatible; new fields are additive and no existing code value is repurposed.
- [ ] Tests assert that no configured secret value, bearer token, or credential appears in the emitted line.
- [ ] A genuinely unknown error still emits a distinct unknown-class code (not silently mapped to a named stage).

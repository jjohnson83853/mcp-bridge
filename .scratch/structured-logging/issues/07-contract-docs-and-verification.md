# 07 - Contract Docs and Verification Closeout

Status: ready-for-agent
Blocked-by: 03, 04, 05, 06

## What to build
The logging contract becomes documented and provable. A schema reference records the application-owned envelope, the collector-owned fields the application must never emit, the evolution rule (removed, renamed, retyped, or semantically changed fields increment `schema_version`; optional event-specific additions do not), and example queries keyed on `event`, `error.code`, and `error.kind` rather than message text.

Verification proves the operator-facing behaviour end to end: raw JSON per line, correct stream per level, absence of removed and collector-owned fields, and the pin-ceiling scenario that motivated the change.

## Acceptance Criteria
- [ ] AC-1: Documentation lists every application-owned field, every collector-owned field, and the schema-version evolution rule.
- [ ] AC-2: Documentation includes example queries using `event`, `error.code`, and `error.kind`.
- [ ] AC-3: A test asserts each emitted line is exactly one JSON object with no plaintext prefix.
- [ ] AC-4: A test asserts debug/info go to stdout and warn/error to stderr.
- [ ] AC-5: A test asserts no record contains `time`, `timestamp`, `environment`, `version`, or `subsystem`.
- [ ] AC-6: The 65-pin scenario is covered end to end and names `pins[64]` with its line and column.
- [ ] AC-7: The full suite passes.

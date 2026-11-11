# 06 - Validation Success Record

Status: ready-for-agent
Blocked-by: 01

## What to build
The validation command reports success in the same JSON contract as failure, so automation parses one format. A valid document emits `config_validation_succeeded` with `outcome: "success"` and no `error` object. Configuration contents are never serialized.

## Acceptance Criteria
- [ ] AC-1: A valid document emits exactly one JSON record with `event: "config_validation_succeeded"` and `outcome: "success"`.
- [ ] AC-2: The record contains no `error` object and no configuration values.
- [ ] AC-3: Exit code is 0 and the record goes to stdout.
- [ ] AC-4: An invalid document still emits the failure record on stderr with exit 1.
- [ ] AC-5: Tests assert both outcomes parse as single JSON objects with the expected stream and exit code.

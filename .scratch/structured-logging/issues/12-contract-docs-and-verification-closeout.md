# 12 - Contract Docs and Verification Closeout

Status: ready-for-agent
Blocked-by: 09, 10, 11

## What to build
The logging contract documents the events and event-specific fields this work adds, and states for each whether it is an optional event-specific addition or a schema-version change under the existing evolution rule. The documentation includes example queries keyed on `event`, `error.code` and `error.kind` rather than message text, so an operator can find a rejected refresh or an unresolved pin without knowing the log prose.

Verification closes the work: the full suite passes, and the documented contract matches what the producers emit.

## Acceptance Criteria
- [ ] AC12-1 (12/AC-1; R18, AD20): docs list the added events and event-specific fields, each marked optional-addition vs schema-version change; the doc-drift test additionally asserts the extended `eventMessage` set (all 23 named events per AD19) and each r8 record's pinned operation — `refresh` for `catalog_refresh_started`/`catalog_refresh_completed`/`catalog_refresh_failed`/`upstream_discovery_failed`/`pin_blocker_detail`, `logging` for `logging_write_failed`, `reload` for `config_reload_applied`. test (doc-drift).
- [ ] AC-2: Documentation includes example queries keyed on `event`, `error.code` and `error.kind`.
- [ ] AC12-3 (12/AC-3; R20–R25): the full suite passes — the `go test ./...` exit-0 run is this criterion's OWN oracle (not AC12-1's); the documented-contract-matches-emission half is FOLDED into AC12-1's doc-drift test and is not separately asserted here. test (`go test ./...`).

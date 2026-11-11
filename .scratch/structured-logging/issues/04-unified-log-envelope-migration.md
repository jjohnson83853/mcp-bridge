# 04 - Unified Log Envelope Migration

Status: ready-for-agent
Blocked-by: 01

## What to build
Every application log record adopts the envelope introduced by ticket 01, so one schema covers all severities and one query shape works across the service. This is a wide refactor: expand the new form beside the old, migrate record producers in batches, then delete the old form once no producer remains.

Removed: `time`, `environment`, `version`, the flat `reason` classification, and the flat `error.class` form. Added: `schema_version`, `operation`, `outcome`, and the nested `error` object. Debug and info records go to stdout; warn and error records go to stderr. The fixed HTTP diagnostic lane keeps its own record shape but gains the shared envelope fields and the same stream rule.

Repository-owned consumers — tests, queries, and documentation — migrate in the same change. No aliases survive.

## Acceptance Criteria
- [ ] AC-1: Every emitted record contains `schema_version`, `level`, `service`, `event`, `operation`, and `outcome`.
- [ ] AC-2: No record contains `time`, `timestamp`, `environment`, `version`, `subsystem`, or a flat `reason`/`error.class` field.
- [ ] AC-3: Failure records carry the nested `error` object with `code`, `kind`, and `retryable`.
- [ ] AC-4: Debug and info records are written to stdout; warn and error records to stderr.
- [ ] AC-5: The HTTP diagnostic lane emits the shared envelope fields while preserving its own fixed record shape.
- [ ] AC-6: Each line parses as exactly one JSON object; no record is split, retried, or partially written.
- [ ] AC-7: The full suite passes with no legacy field assertions remaining.

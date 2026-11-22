# Target
Author a FRESH, complete SpecDD specification for feature `snapshot-recovery`, revision id `SNAPREC-SPEC-r12`, as plan round 1 of at most 3. Return the full nine-section specification through native structured yield (`specification`) plus `handoff` <=3072 UTF-8 bytes.

# Owner directive (2026-11-21)
- Fresh new revision. Do NOT recover, reuse, or review r11: no in-place repair of its clauses, no copying its section 6 task ledger or section 9 finding ledger, no review verdict on it.
- At most 3 planner-authored plan revisions. This is round 1. Author exactly one specification now; do not pre-plan round 2.
- No implementation, no file writes, no dispatches, no approvals.

# Objective (from ticket 01)
A snapshot the store refuses is recoverable startup state: quarantine its bytes once before admission, continue startup, and report the act in one typed warning - without weakening `ensureReplaceable` no-overwrite protection (internal/storage/store.go:295-306) or `Load`'s rejection wrapping (internal/storage/store.go:149-195).
Owner decision: `MCP_SNAPSHOT_QUARANTINE` defaults ON; `off` is a fail-closed opt-out for manual inspection.

# Read these authoritative inputs
- .scratch/snapshot-recovery/issues/01-untrusted-snapshot-self-recovery.md - incident, verified code path, owner decision, mirrored AC ids AC1-1..AC2-9. Read fully.
- .scratch/mcp-bridge-defects/backlog.md - MB-002 entry.
- docs/agents/domain.md, docs/logging-contract.md, go.mod.
- internal/storage/store.go (Load, ensureReplaceable, syncDirectory), internal/storage/snapshot_v1.go, internal/storage/snapshot.go.
- internal/service/loop.go startup tolerance; internal/service/reload.go.
- internal/config/config.go; internal/observe/envelope.go, log.go, metrics.go.
- cmd/mcp-bridge/serve.go, cmd/mcp-bridge/startup_failure.go.
- Harness contracts: /home/opencode/.omp/agent/workflow-policy/development-workflow.md (W09E), /home/opencode/.omp/agent/workflow-policy/review-findings-taxonomy.md, and skill://observability-contract (mandatory before section 7).

Verified this session by direct reads: internal/storage has NO recovery.go yet; T1/T2/T3 write effects are pending; loop.go logs `startup_snapshot_rejected` once and continues with no cached state. Re-verify every anchor you cite; line numbers may have drifted.

# Prior review hazards (advisory evidence only, NOT a closure checklist)
.omp/artifacts/snapshot-review-code-review-r10-20261121T170000Z.md: 3 MEDIUM + 5 LOW, notably unoracled symlink branches (corrupt symlinked regular target; broken symlink), an unclassified `os.Stat` branch, statement contradictions, and stale ticket mirrors. Treat these as hazard classes to avoid repeating in a fresh design; do not write 'CLOSED' claims for r11 clauses.

# Envelope (do not widen)
The canonical state's task write effects are the owner-approved file envelope:
- storage set: internal/storage/store.go, internal/storage/recovery.go, internal/storage/recovery_test.go, internal/storage/store_edge_test.go
- bootstrap/observe set: cmd/mcp-bridge/serve.go, cmd/mcp-bridge/snapshot_recovery_test.go, cmd/mcp-bridge/serve_edge_test.go, cmd/mcp-bridge/startup_failure_test.go, cmd/mcp-bridge/config_envelope_test.go, internal/observe/envelope.go, internal/observe/log.go, internal/observe/log_test.go, internal/observe/edge_test.go, docs/logging-contract.md
No other write path. Any genuinely required additional path must be returned as a machine-readable blocker naming the path/reason for coordinator/owner authorization - never added as a task.
Read set (read-only): the projection at .omp/artifacts/snapshot-recovery-state-r11-projection.json `.envelope.readPaths` plus the inputs above. Forbidden effects: network, publish, deploy, schedule, credentials, infrastructure. `maxAttempts` 1.

# Specification requirements
- Nine sections exactly as your role defines: STATUS, REQUIREMENTS, EVIDENCE AND UNCERTAINTY, DESIGN, ACCEPTANCE CRITERIA, TASK LEDGER, OBSERVABILITY, RELEASE AND RECOVERY, CRITIC HANDOFF.
- Section 2: stable R1.. IDs with explicit non-goals. Section 5: observable pass/fail ACs mapped to R IDs and verification methods. Keep the ticket-mirrored acceptance behaviors (quarantine-once, typed warning, continue startup, no-overwrite protection preserved, opt-out fail-closed, retention/prune bounds, schema/checksum/validation rejection classes). Re-derive AC text; keep stable ids where they still fit. If the fresh design needs an id rename/add/remove, list each delta in `handoff` as an owner-visible envelope change; never silently drop a mirrored criterion.
- Section 6: T ids, objective, dependencies, exact paths from the envelope, RED step with expected assertion failure and its GREEN check, deliverable, AC ids, exact validation commands (e.g. `go test ./internal/storage/...`, `go test ./cmd/mcp-bridge/...`, targeted `-run` selectors), decision-log location, blocked conditions. Successful termination requires every named command exit 0; blocked tasks are unsuccessful.
- Deterministic oracles for: direct corrupt file; symlinked regular target (corrupt and valid); broken symlink; symlink to directory; absent path; unreadable/permission/unclassified stat error; concurrent Load/Save and ensureReplaceable race/lifecycle hazards; retention/prune bounds with the 4096-attempt reservation bound and exhaustion behavior; quarantine-name collision; byte preservation and never-overwrite.
- No weakened ACs for GREEN; no code blocks or diffs; no speculative Go.
- Section 7 per the observability-contract checklist: event name(s), typed reason taxonomy, level, stable fields, log delivery, metrics exposition/collection, failure visibility, measurable ACs; justified exception if any item is inapplicable.
- Section 8: env-var default/opt-out, deploy/migration order, rollback, irreversible steps.
- Section 9: highest-risk assumptions, coverage gaps, specific review targets; mark facts verified vs assumed; state that spec-lint and tests were NOT run by you.
- Read-only evidence only: if a decisive fact is missing, return a machine-readable blocker with target file/symbol and resolution step; do not guess and do not ask the user.
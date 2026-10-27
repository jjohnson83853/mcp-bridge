# 01: Resolve storage, session, and reporting prerequisites

**What to build:** A verified compatibility and lifecycle evidence package that lets metadata preservation and overrides proceed without guessing about durable storage, session behavior, or existing failure reporting. This is a read-only research prerequisite, not a product implementation slice.

**Blocked by:** None (ticket dependencies).

**Status:** ready-for-agent

Type: research
Source: Tool Metadata specification, METADATA-r4. Coverage: U2, U3, U5; AC7, AC8, AC10, AC13.

Source access restored on 2026-10-26 after an explicit user-authorized retry. Bounded evidence is recorded in `../prerequisites.md`: the current strict reader disproves additive old-reader compatibility, and verified lifecycle/reporting constraints were returned to the planner. METADATA-r5 now proposes a strict version-2 writer/version-1-or-2 reader and offline restore rollback; that is a selected design, not implemented compatibility proof. This completed research does not authorize implementation or replace independent design review and the separate user checkpoint.

- [x] Establish the actual durable tool-record format, version/integrity validation, legacy-reader/new-reader behavior, and rollback requirements using bounded, anchored evidence.
- [x] Determine whether the proposed additive metadata representation fits the existing compatibility contract. An incompatible result returns to planner/critic review; this ticket does not select or implement a migration.
- [x] Establish actual startup, refresh, reload, session lifetime/defaults, notification negotiation, and explicit re-list behavior. Distinguish configured defaults from validation limits and assumptions.
- [x] Establish existing failure-reporting schema, severity, delivery, retention, and redaction behavior relevant to rejected configuration, discovery failures, persistence failures, and stale catalogs. Do not propose new telemetry.
- [x] Return a bounded evidence report distinguishing verified facts, discrepancies, and unresolved blockers, with source anchors and the consequences for affected design and acceptance criteria.
- [x] Preserve the existing parent specification and application/configuration state. Report through the coordinator; do not infer approvals, change gates, or begin implementation.

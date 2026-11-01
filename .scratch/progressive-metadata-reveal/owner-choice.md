# Owner requirement choice

User selected via interactive question: **Match pinned metadata**.

Apply source → server defaults → exact-tool overrides to progressively revealed targets. Preserve pinned precedence and expand the former pinned-only defaults boundary. This is a requirement-semantic choice only; it is not implementation approval or a requirements/design/tasks checkpoint receipt.

User also directed: **assume dynamic per-tool exposure is necessary**. Treat
this as a design assumption. Gemini-specific client behavior remains
user-reported, not independently verified; do not claim verified approval or
prompt outcomes.

## Session exposure and list atomicity

User explicitly selected:

- **Per client session:** only the session that searches/describes a target may
  see/invoke its revealed descriptor; session deletion/expiry clears it.
- **Atomic `tools/list` snapshot:** clients observe either the old or complete
  new descriptor set, never an intermediate update.

These are binding requirement choices for the planner's next design revision.
They do not authorize implementation or satisfy requirements/design/tasks
checkpoint receipts.

## Bridge integration scope

User selected: **Allow internal bridge changes**.

The planner may consider changes to bridge-owned MCP server/session wiring and
lifecycle cleanup, while preserving the existing MCP wire contract, auth binding,
`MaxSessions`, and idle-expiry semantics. This authorizes planning scope only; it
does not authorize an SDK fork, implementation, or any requirements/design/tasks
checkpoint.

## Pagination cursor compatibility

User explicitly selected: **Allow generation-bound cursors**. The bridge may
issue opaque session- and generation-bound cursor values and return a standard
invalid-cursor error for stale or cross-session continuations. Keep MCP methods
and response shape unchanged. This is a planning/requirements decision only; it
does not authorize implementation or satisfy any SpecDD checkpoint.

## Reload and persistence failure scope

User selected: **Atomic list pages**. Each `tools/list` response must be built
from one immutable descriptor generation; runtime and session projections
advance only after successful runtime acceptance. Preserve existing durable
snapshot/restart semantics, including post-rename directory-sync ambiguity;
do not expand this feature into a cross-system storage transaction redesign.
This is a design-scope decision only, not implementation or checkpoint approval.

## Observability slice deferral

On 2026-10-28 the user directed: “lets archive observability changes, we can come back to those in another slice/”.

Archive the expanded observability planning and evidence obligations in `observability-deferred.md` for a separate slice. Remove that audit and R4-C3 log-schema evidence gap as acceptance/task-ledger blockers for progressive metadata reveal. Preserve existing logging/redaction, metrics, health behavior and applicable regressions; add no telemetry. This direction does not change session design or authorize implementation, user checkpoints or a clean critic verdict.

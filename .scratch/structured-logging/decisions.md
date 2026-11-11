# Structured Logging — Settled Decisions

Source: grilling session with the owner, 2026-11-10. Every item below is an owner decision, not a proposal.

## Problem

`mcp-bridge` rejects a configuration with a plaintext line that names only the section:

```
mcp-bridge: configuration: config="/app/configs/config.yaml" key="pins" code=pin_ceiling property=pins shape=out_of_bounds: pins list may contain at most 64 entries; remaining_stages=not_checked
```

The operator cannot tell which entry was rejected, where it is, or what to change. Runtime logs are already JSON but use a different, unversioned shape.

## Decisions

1. **One schema.** Every application log record and every CLI error uses one newline-delimited JSON envelope. No mixed text/JSON output.
2. **Application-owned envelope fields:** `schema_version`, `level`, `service` (hard-coded `mcp-bridge`), `event`, `message`, `operation`, `outcome`, nested `error`, and event-specific objects.
3. **Collector-owned fields are never emitted by the application:** timestamps, `subsystem`/container identity, host, environment, version. The collector enriches after the application writes.
4. **No application timestamp.** Queries use collector time.
5. **Streams:** debug/info → stdout; warn/error → stderr. One JSON object per line, one write, no retry, no partial output, no plaintext fallback.
6. **Nested error object:** `error.code`, `error.kind`, `error.retryable`, `error.remediation`. `message` is human explanation only; queries and alerts use `event`, `error.code`, `error.kind`.
7. **Configuration diagnostics** carry `config.path`, `config.property_path`, one-based `config.line` and `config.column`, safe validated identifiers, and `config.related` (property path, line, column) for conflicts.
8. **Pin ceiling:** report `property_path: "pins[64]"`, that element's line/column, safe `pin_server`/`pin_tool`, `actual_count`, `max_count`, `first_excess_index`. Not every excess entry.
9. **No sensitive values:** never serialize secret values, URLs, query strings, headers, payloads, raw YAML fragments, or arbitrary Go error text.
10. **Fail-fast, one event.** Validation reports the first error in deterministic document order, emits exactly one detailed event, and exits. No detail-plus-summary pair.
11. **Startup vs reload:** startup rejection is `error` and exits 1; rejected reload is `warn`, discards the candidate, and keeps the active configuration serving.
12. **Non-configuration failures** (listener bind, state directory, embedding provider, tool surface, CLI usage) use the same envelope with stable codes, kinds, retryability, and static remediation. Safe resource identifiers only.
13. **`--help` stays human-readable** and is not an error record.
14. **Validation success** emits `config_validation_succeeded` with `outcome: "success"`, no `error` object, no configuration contents, exit 0.
15. **Exit codes unchanged:** configuration failure remains exit 1.
16. **No panic recovery.** Unhandled panics keep Go's crash report.
17. **No raw-cause diagnostic mode** in this change.
18. **No legacy aliases.** `time`, `environment`, `version`, flat `reason`, and flat `error.class` are removed; repository consumers migrate in the same change.
19. **Schema evolution:** removed, renamed, retyped, or semantically changed fields increment `schema_version`; optional event-specific additions do not. This change starts at `schema_version: 1`.
20. **Docker Compose's `mcp-bridge-network |` prefix is orchestrator output**, not application output. The application does not work around it.

## Envelope

```json
{
  "schema_version": 1,
  "level": "error",
  "service": "mcp-bridge",
  "event": "config_validation_failed",
  "message": "configuration contains more pins than supported",
  "operation": "load_config",
  "outcome": "failure",
  "error": {
    "code": "pin_ceiling",
    "kind": "configuration",
    "retryable": false,
    "remediation": "remove pins at index 64 or later"
  },
  "config": {
    "path": "/app/configs/config.yaml",
    "property_path": "pins[64]",
    "line": 83,
    "column": 5,
    "pin_server": "alpha",
    "pin_tool": "tool-65",
    "actual_count": 65,
    "max_count": 64,
    "first_excess_index": 64
  }
}
```

## Tickets

`.scratch/structured-logging/issues/01..07`. Approved by the owner: seven slices, edges as written, no merges.

## Review gate status at r7-closure1

Blockers B1–B6 were verified resolved by two independent PASS reviews: `.scratch/structured-logging/review-code-reviewer-r7-closure1.md` and natively dispatched `CloseR7Blockers-3` (per-item anchors in both). Advisories A1–A12 remain open as known accepted risks (A6 drift at `spec.md:76` still says `revision: "r6"`; A11 count ×9 vs 7).

**Owner decision (this session):** accept the single genuine independent PASS review as r7's specification review and record the unreachable `reviews.tasks` (critic) gate as an owner-accepted risk, rather than fixing the harness first.

**Blocking constraint — this decision cannot be encoded without fabrication.** `execution-contract-schema.mjs:52` requires `verdict: literal("pass")` and a `criticResponse` receipt (`{origin:"reviewer", verdict:"pass"}`) for BOTH `reviews.specification` and `reviews.tasks`; there is no waiver or exception shape. No critic review exists or is obtainable: `execution-review-sequence.mjs:129` (`positive()`) denies any `critic` dispatch unless that candidate's recorded run is `succeeded`, and `review-runs.json` shows every candidate still `pending` (admission writes `pending` at `execution-gate.ts:785`; `succeeded` is written only from `finishReview(..., true)` at `:768`, reached solely by a correlated terminal `tool_result` with `exitCode 0` at `:874-895`, which async completion does not deliver). Therefore `reviews.tasks` was left `null`; no receipt was authored.

**Consequence:** `.omp/plans/execution-state.json` stays `status: blocked` with `reviews` unset, so engineering dispatch remains gated. Unblocking requires either (a) a workflow-policy fix to reviewer-run terminal finalization in `execution-gate.ts` `handleToolResult` plus `prompt-scope.ts` wiring, or (b) a schema-level, planner-owned waiver shape. Neither is performable from this session root.

## Gate scope decision — T1 gofmt (owner, this session)

**Decision:** the `gofmt -l .` verification gate is scoped to the task's changed files, not the whole repository.

**Basis:** the authoritative T1 verify line in `spec.md:222` requires only `go test ./internal/config -run 'Pin|Ceiling'`, `go test ./cmd/mcp-bridge -run 'CheckConfig|ExampleConfig'`, and `go vet ./...` — it contains no `gofmt` gate. The repo-wide `gofmt -l .` (must be empty) was added by the dispatch builder (`build-dispatch-t1.mjs:71`), not the spec. `gofmt -l .` reports 28 files that are (a) outside T1's authorized write set and (b) unmodified by T1 (`git diff --name-only` lists only T1 files). They are pre-existing repository formatting debt.

**Consequence:** T1 is verified by `gofmt -l` over its changed files (empty) plus the spec's named test/vet commands. The 28 pre-existing unformatted files are out of scope for T1 and are not a T1 blocker. A repo-wide formatting pass, if desired, is a separate ticket.

## T6 success-record operation is `check_config`, not `validate`

**Decision:** the `config_validation_succeeded` record uses `operation: "check_config"`, per spec AD7/AC06/T6. The T6 dispatch text said `operation: validate`; that was a dispatch-authoring error, not a contract change. The engineer followed the approved spec and recorded the discrepancy in `receipt_T6.json`.

**Basis:** `spec.md` AD7, AC06-1, and the T6 ticket pin `check_config`; the dispatch builder's prose was the only source of `validate`.

**Consequence:** no spec amendment. The success envelope omits `error` per the no-error-object contract (tests accept nil/absent).

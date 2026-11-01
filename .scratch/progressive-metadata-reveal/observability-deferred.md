# Deferred observability slice

Status: needs-triage
Disposition: Archived by owner; not a dependency or acceptance gate for progressive metadata reveal.

## Owner instruction

2026-10-28: “lets archive observability changes, we can come back to those in another slice/”

This archives planning obligations, not application changes. No telemetry implementation is reverted, authorized or claimed complete. A future slice requires its own scope and review. Existing telemetry, redaction, health behavior and applicable regressions remain preservation constraints in the active feature. Session design is unchanged by this deferral.

## Provenance

Source: `.scratch/progressive-metadata-reveal/spec.md`, revision `cursor-choice-reconciliation-5`.
Source SHA-256: `6b94f9c22b66ca34223c77cfba9c38334aa1db646e93a75c38058bc1d26baec2`.
Archived obligation: R4-C3 / state blocker `preledger-log-schema-r5`. Evidence was incomplete; deferral is not verification or a clean critic verdict. Historical review/evidence artifacts remain untouched.

## Original operational planning (verbatim)

<!-- specdd:envelope:start -->
The observability-contract skill was loaded before this revision. Preserve existing finite/redacted reporting and no-new-telemetry scope. Protocol list-change notifications are not monitoring signals. E12 is supplied source evidence, not executed verification or independent review.

**Task-binding exception:** implementation task IDs remain deferred before explicit design approval. Preservation obligations map now to PR-AC4/7/8/9/12 and must acquire exact implementing task IDs and verification anchors in the later reviewed ledger. Reason: no implementation/task-breakdown authority exists in this assignment. Consequence: no operational work or verification is authorized or claimed.

### Components, logging and failure visibility

| Affected component | Preserved behavior and acceptance binding |
|---|---|
| Metadata/config/descriptor preparation and registry reconciliation | Existing finite rejection/failure reporting, valid JSON/redaction and truthful rejected/precommit/postcommit outcomes; no metadata/patch/schema dumps or false successful publication. PR-AC4/9/12. |
| Search/describe/direct bound-handler admission and execution | Existing operation/backend diagnostics, terminal classification and correlation/duration where present; one execution result/error owner, no duplicate terminal records and zero upstream execution counts for discovery/list. PR-AC5/7/9/12. |
| Inbound pending association, promotion, owner/cursor and cleanup | Existing HTTP/auth/session diagnostics and finite protocol errors; failed/mismatching initialization, invalid cursor, retired owner and postcommit response failures remain visible without new lifecycle events. Never log bearer/fingerprint values, raw session IDs, owner tokens, cursor/MAC/key contents or sensitive payloads. PR-AC8/9/12. |
| Runtime/catalog/storage/operations | Existing accepted-state status, refresh/reload/storage failures, readiness and scrape behavior remain truthful. An accepted runtime is not a claim of completed projection reconciliation. PR-AC9/12. |

**Collection and schema:** `cmd/mcp-bridge/serve.go:34-38` connects stdout to `observe.NewLogger`. Retain the METADATA-r7 synchronous mutex-protected, best-effort JSON stdout contract, finite existing severities/events, recursive redaction and no replay/in-process retention. `internal/observe/log.go:13-35,92-104` anchors Entry/JSON encoding. However, the supplied supplement does not enumerate the complete emitted keys/types/time/severity format or omission behavior. Those exact facts and affected event/error projections remain **BLOCKED before task-ledger approval**. The coordinator must supply the bounded extract and relevant existing test anchors; no new source reads are assigned to this planner. Do not substitute a guessed schema or add fields/events to satisfy the checklist.

**Required-field exception:** this amendment does not introduce UTC timestamp, level/service/environment/version/event/message, correlation IDs, duration or structured-error fields absent from the existing logger. Reason: preserve the established finite logger and no-new-telemetry requirement. Consequence: correlation remains limited to existing fields. This exception does not waive discovery of the existing exact schema or its JSON/redaction regression checks. Retain existing severity policy and typed warn/reload/failure/invalid_config projections where applicable; exact per-component call-site mapping is required before the ledger. A new failure with no truthful existing protocol/status/diagnostic path is a design blocker, not permission to swallow it or invent an event.

### Loki

Known handoff is JSON stdout only. Collector-to-Loki route, label mapping, retention, ingestion-failure detection and external delivery remain UNKNOWN/NOT VERIFIED. **Exception:** external collection infrastructure is unchanged and outside authorized scope. Consequence: no durable delivery, retention, ingestion recovery or Loki alert behavior is claimed. PR-AC12 must document the limit; mapped existing delivery checks, if evidence/access is later supplied, must verify their actual boundary. Preserve bounded labels; never introduce session/user/request/tool/cursor identifiers as labels. No shipper, buffer, replay or retention change is selected.

### Prometheus

E12 supplies this existing inventory from `internal/observe/metrics.go:27-43,133-172,179-229`:

| Name | Type | Existing labels | Unit/meaning |
|---|---|---|---|
| `bridge_requests_total` | CounterVec | `server,operation,outcome` | Request count |
| `bridge_catalog_tools` | GaugeVec | `server` | Catalog tool count |
| `bridge_catalog_prompts` | Gauge | none | Catalog prompt count |
| `bridge_catalog_age_seconds` | GaugeVec | `server` | Catalog age, seconds |
| `bridge_catalog_refresh_total` | CounterVec | `server,outcome,reason` | Refresh count |
| `bridge_config_reload_total` | CounterVec | `outcome,reason` | Config-reload count |
| `bridge_readiness` | GaugeVec | `server` | Existing readiness state: 0 unavailable, 1 degraded, 2 ready |

Preserve aggregate `_bridge`, accepted server-ID cardinality/syntax constraints (maximum 1024 accepted server IDs), and finite operation/outcome/reason vocabularies (`:27-43,64-87,183-229`). Do not label by raw tool, request/session/epoch/generation, cursor or metadata. No latency histogram/buckets are defined in the supplied inventory; do not fabricate them.

`Metrics.Handler()` is at `internal/observe/metrics.go:170-172`; `/metrics` is registered at `internal/service/ops.go:77-114`. The composition root binds `settings.OperationsListen` and rate-limits operations at `cmd/mcp-bridge/serve.go:217-229`. Existing route regression anchor: `internal/service/ops_test.go:36-40`, `TestOperationsMetricsRouteAndUnknownPaths`. No concrete deployed listen address, external scraper or successful scrape is claimed.

PR-AC9/12 and the later task-bound operational checks must decode the actual `/metrics` exposition, preserve these names/types/label bounds and verify affected accepted/rejected refresh/reload, readiness and request outcomes through existing semantics. Do not reinterpret `bridge_readiness` as a newly invented projection-completeness metric or add a failure label. Reconciliation/admission failure remains visible through existing truthful protocol/diagnostic/status paths; absence of such a path requires planner escalation. This supplied route/series evidence remains subject to independent review before task-ledger approval.

**New-series/latency/saturation exception:** no new exposure throughput, queue, saturation, dependency, latency metric or bucket is introduced. Reason: no new worker/external dependency and the owner excludes new telemetry. Consequence: exposure-specific operational latency/retention/SLO trends are unavailable; deterministic PR-AC6/8/9 checks prove lock release, cleanup and failure isolation, not production SLO monitoring. Existing scrape verification remains required rather than waived.

### Health, tracing, dashboards, alerts and operations

Preserve existing health/readiness behavior and request/trace propagation where present. Client failure to re-list/adopt descriptors is not bridge unreadiness. PR-AC7/9/12 must preserve truthful errors, accepted-runtime/storage status and existing operational regression behavior; exact tests/call-site anchors are a later evidence prerequisite, not engineer discretion.

**Tracing exception:** no new span/exporter is introduced because the change uses existing request/execution paths and telemetry expansion is excluded. Consequence: there is no new projection/pending-association trace view; preserve existing propagation and assess it against supplied existing seams before ledger approval.

**Dashboard/alert exception:** definitions, deployed access and external delivery evidence are not supplied and no dashboard/alert/pager change is authorized. Consequence: no new stale-client/reconciliation alert or external dashboard/alert-delivery result is claimed. PR-AC12 records NOT VERIFIED for those external boundaries; exercise existing behavior only when corresponding definitions/access are actually supplied. No duplicate/noisy events, sensitive data or unbounded labels are allowed.


## Original associated obligations (verbatim)

### Original line 4

**Spec ID:** progressive-metadata-reveal. **Revision:** cursor-choice-reconciliation-5. **Status: BLOCKED — revised planning text for independent re-review; exact operational-schema evidence remains required before task-ledger approval; no implementation authorization.**

### Original line 21

**Feasibility reconciliation:** supplied primary SDK evidence establishes session-aware receiving middleware, a registered SDK session ID before initialize dispatch, and initialize response issuance of that ID; it does not establish the new bridge integrations below. Existing inbound context carries no authentication fingerprint/incarnation, and existing admission rejects IDs absent from `h.sessions` before SDK receiving middleware (`internal/service/inbound.go:298-356,388-393`). §4.9 therefore selects a new typed private context carrier and narrowly scoped initialized-pending admission branch, with ordinary post-response promotion rather than early `h.sessions` insertion. §4.2 selects a new request-scoped bound-execution adapter; the inspected direct/generic handlers do not already accept the complete captured assembly/binding/schema/lease tuple (`internal/service/tools.go:211-259,663-709`). Both integrations require independent review and later behavioral proof; existing MaxSessions admission, concurrent-initialize behavior, post-response insertion, lastSeen/idle policy, auth binding, DELETE/expiry and terminal retirement remain constraints, not implementation freedoms. Exact operational JSON-schema evidence remains a pre-ledger blocker; the newly supplied metrics/route evidence is recorded but has not received independent review. Requirements, design and execution-task checkpoints remain separate and unapproved for this amendment; historical METADATA approvals are not inherited.

### Original line 81

**E12 — Operations supplement, supplied by coordinator during this amendment:** bounded source retrieval established metric definitions at `internal/observe/metrics.go:27-43,133-172,179-229`, bounded label constraints at `:27-43,64-87,183-229`, and `Metrics.Handler()` at `:170-172`. `/metrics` registration is `internal/service/ops.go:77-114`; existing route test is `internal/service/ops_test.go:36-40`, `TestOperationsMetricsRouteAndUnknownPaths`. `cmd/mcp-bridge/serve.go:34-38` connects stdout to `observe.NewLogger`; `:217-229` binds `settings.OperationsListen` and rate-limits operations. `internal/observe/log.go:13-35,92-104` anchors Entry/legacy JSON encoding, but the supplement does not enumerate the exact complete emitted key/schema contract. Coordinator reports narrow direct-read fallback for serve/log only after repeated codebase-searcher `catalog_changed` responses with `execution_state=not_started`; the other anchors came through MCP. This planner performed no source retrieval. These are supplied source facts, not test, scrape, JSON-delivery or independent-review results.

### Original line 88

- **Remaining evidence — BLOCKED before task-ledger approval:** the metrics names/types/labels and operations route are now supplied in E12, pending independent review. Exact emitted JSON keys, types, timestamp/severity representation, omission/null behavior and component-specific existing event/error projections still need a bounded evidence extract from `internal/observe/log.go:13-35,92-104` and the relevant existing component call sites/tests. The coordinator must supply this fact bundle under the code-access contract; this planner is not authorized to retrieve source. Existing operational preservation tests need exact anchors before a ledger can be approved. Do not invent a new logger/schema/event/series or claim external collection delivery.

### Original line 220

| PR-AC12 | Docs/changelog cover all PR-R11 contracts, notification/cache limits, accepted-runtime/reconciliation separation, cursor restart, operational exceptions and storage/rollback limits. Later authorized engineering retains intended compiling behavioral RED failures and unchanged GREEN, unit/negative/concurrency/lifecycle evidence, real controlled HTTP/MCP integration/E2E discovery→re-list→direct-call flow, existing regressions, required reviews and full command exits. Custom SOURCE-LINE ≥90%; no package/assertion narrowing or statement-percentage substitute. Existing JSON/redaction/failure/readiness/scrape regressions are mapped and exercised where applicable under §7; external collection/dashboard limits are explicit, not invented evidence. | PR-R11, PR-R12 |

### Original line 233

After design approval, the planner must produce a separately reviewed bounded ledger covering the complete shared-server projection/union/admission cutover, authenticated session enrollment/removal, acceptance/reconciliation callers, metadata eligibility/parity, pagination, controlled negative/concurrency/lifecycle tests, docs/changelog and operational preservation. It must map every retained PR-R/PR-AC to dependencies, supplied source/edit anchors, compiling behavioral RED assertion, unchanged GREEN verification, allowed/prohibited effects, recovery and stop conditions. Required operational anchors must be obtained before ledger approval, not delegated as open-ended implementation research. No such work or checkpoint progression is authorized by this draft.

### Original line 234

**R4-C3 pre-ledger evidence gate:** E12 supplies the metrics inventory and `/metrics` route, but exact emitted JSON-schema/event projections still require bounded coordinator evidence and independent review. No executable task breakdown or task-ledger approval may treat this gap as resolved. The later ledger must explicitly migrate superseded pin-only test expectations, exercise pending-association/promotion and captured-handler races, bind §7 checks to task IDs, and retain every PR-R/PR-AC obligation. None of those implementation tasks is issued here.

### Original line 321

- **R4-C3 — STILL BLOCKED before task-ledger approval:** coordinator E12 supplies route/series/type/label anchors and the existing route-test symbol, pending independent review. Exact emitted log schema and affected event/error/test mappings remain missing. No new telemetry or external collection/delivery result is claimed. Section 7 preserves explicit exceptions and requires later task bindings.

### Original line 327

Stable PR-R1..12 and PR-AC1..12 IDs remain. No implementation tasks have been issued. The formerly unapproved r4 binding/provenance/direct-route passages are superseded by this revision; do not assert old pin-only tests or obligations are all unchanged. Requirements/design/tasks approvals remain ungranted for this feature, and all three explicit checkpoints are still required. Exact operational evidence and independent review precede task-ledger approval. Owner selection “Match pinned metadata” establishes requirements semantics only.

### Original line 334

4. Are the supplied `/metrics` inventory/route anchors sufficient once independently reviewed, and what exact existing emitted JSON schema/event/test anchors remain needed before task-ledger approval?

## Original workflow blocker (verbatim)

```json
[
  {
    "id": "preledger-log-schema-r5",
    "taskId": "resolve-bridge-evidence",
    "category": "evidence-failure",
    "sourceIds": [],
    "inputDigest": "2ecbc78b0445fca5c0892d3315e5a409389dbade577eb14d338ec7f6f5d8338d",
    "reason": "R4-C3 remains blocked before task-ledger approval: exact emitted JSON log keys/types/time/severity/omission behavior and affected component event/error/test projections remain unverified.",
    "clearWhen": "Supply bounded source extracts for the exact existing emitted JSON schema and affected event/error/test mappings, then independently review and bind those operational checks before task-ledger approval.",
    "attempts": 0,
    "lastAttemptAt": null
  }
]
```

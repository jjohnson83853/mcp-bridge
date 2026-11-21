<task-result id="ReviewBreakerSpecR1" agent="code-reviewer" status="completed" duration="5m31s">

<output>
## Code Review Summary

**Mode:** Specification review — candidate `upstream-breaker-recovery-spec-r1` r1 (planner DRAFT, `.omp/artifacts/upstream-breaker-recovery-planner-r1.md`, 169 ln) vs ticket 01 (AC-1..AC-7), live source anchors, W09/W09E, findings taxonomy. No implementation exists; all AC/T are planned oracles and the spec never claims executed verification. LSP/codebase-searcher unmounted; evidence = bounded native reads/greps.

**Files Reviewed:** 1 artifact (+ ticket, spec mirror, 9 source anchors as evidence) · **Total Issues:** 7

### By Severity
CRITICAL: 0 · HIGH: 0 · MEDIUM: 1 · LOW: 6

### Issues

**[MEDIUM] AC5 blocked-fallback contradicts the approved envelope** (class: ledger-state-drift/effects)
File: artifact:120 vs :168 · Confidence: HIGH
Issue: T1 pre-authorizes "relocate AC5 to `cmd/mcp-bridge` using the real `registry.discover`", while §10 puts `cmd/mcp-bridge/*` out of scope and gates envelope widening on renewed owner authority. Followed literally, an engineer writes a test outside the approved envelope (W09E authority surface). AC5 is verified feasible as planned, so the branch is latent — but the contradiction stands.
Fix: reword :120 to report BLOCKED-with-evidence and name the relocation as owner-authorized envelope widening before any write.

**[LOW] §3 anchor drift** (class: stale-citation) — File: artifact:36,:104 · Conf: HIGH
Issue: `reload.go:620` resolves to `internal/service/reload.go:618` (off 2, path prefix missing); `serve.go:216-224` truncates the wired `ReloadDeps` block (actual :217-231); `ReldoadDeps` typo. Other anchors verified exact (see Positives). Fix: cite `internal/service/reload.go:618`, `serve.go:217-231`, `ReloadDeps`.

**[LOW] Focused gate skips AC10's oracle** (class: r-ac-coverage-gap) — File: artifact:114 · Conf: HIGH
Issue: the `-run` alternation matches no prefix of `TestFailedProbeEmitsDiscoveryFailureRecordWithoutNewTelemetry` (AC10, :91); suite commands 2–4 still run it. Fix: add `TestFailedProbe` to the alternation.

**[LOW] R5 reload leg untested behaviorally** (class: r-ac-coverage-gap, advisory) — File: artifact:86,:36 · Conf: MEDIUM
Issue: AC5 drives the refresh leg only; the SIGHUP leg is source-anchored only (`reload.go:618` shared build path). Acceptable — no production change touches the legs — but the exception is unrecorded. Fix: record the source-anchor exception in §5/§7.

**[LOW] Panic path can leak the claim** (advisory) — File: artifact:49 (AD2) · Conf: MEDIUM
Issue: `complete` after the body is skipped if `discover` panics, stranding `probeActive=true`; harmless in practice (panic kills the process) and AD2 already allows the defer+set-once equivalent that covers panics. Fix: make the defer form normative.

**[LOW] AC10 metrics oracle unstated** (advisory) — File: artifact:91 · Conf: MEDIUM
Issue: `internal/upstream` never touches `observe.Metrics`; "registry/label set unchanged" is only drivable from `upstream_test` by scraping `observe.NewMetrics(...).Handler()` before/after. Feasible but unnamed. Fix: name the Handler-scrape oracle in AC10.

**[LOW] Ticket AC-5 names an unexported seam** (class: ticket-mirror-drift, advisory) — File: ticket:14 vs artifact:86 · Conf: HIGH
Issue: ticket says `Refresher.runPass()` (unexported, refresh.go:212); spec AC5 correctly uses the public pass path. Fix: record the public-path mapping.

### Open Questions
None — the decisive unknown (AC5 feasibility) was resolved by source verification.

### Positive Observations
- R2 admission/deny and one-shot completion match `BeginProbe` exactly (runtime.go:296-312); `AdmitCall` :237-253 rejects while open/half-open, so calls racing an active probe stay rejected; a mid-probe threshold `RecordFailure` (:266-283) restarts cooldown without enabling a second claim — R6/I4 sound.
- `runtime.go` unchanged is feasible: `BeginProbe` is exported, zero production callers (verified); wrapper never inspects breaker fields.
- AC5 feasible via public seam: `NewRuntime` (reload.go:245), `NewRefresher` (refresh.go:111), `Trigger` (:152), `Passes` (:179); `internal/service` imports no `internal/upstream`.
- Verified: `ServerStateless()` defaults true (config.go:48-52); `sessionFor` never evicts (dedicated.go:294-309, R8 deferral correct); `upstream_discovery_failed` in closed vocabulary (log.go:264); verify.yml:42/48/51 match AC7; decision-log path exists.
- Provenance discipline: DRAFT/no authorization stated, planned vs observed separated, UNKNOWNs carry consequences, ticket AC-1..AC-7 preserved 1:1, no weakened AC.

### Recommendation

REQUEST CHANGES
</output>
</task-result>
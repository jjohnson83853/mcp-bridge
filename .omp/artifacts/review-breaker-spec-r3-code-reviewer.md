## Code Review Summary

**Mode:** Specification, candidate `upstream-breaker-recovery-spec-r3` (r3 DRAFT). **Inputs:** spec §2–§10, ticket 01, live source (runtime.go, dedicated.go, prompt_runtime.go, refresh.go, reload.go, loop.go, wiring.go, serve.go, metrics.go, log.go, dedicated_test.go, verify.yml). **Files Reviewed:** 14
**Total Issues:** 5

### By Severity
- CRITICAL: 0
- HIGH: 0
- MEDIUM: 1 (should fix)
- LOW: 4 (advisory)

### Issues

**[MEDIUM] AC5/AC10 harness omits mandatory setup preconditions**
File: spec §5 AC5+AC10 rows, §6 T1 inputs (`.omp/artifacts/upstream-breaker-recovery-planner-r3.md`)
Confidence: HIGH
Issue: `Runtime.Refresh` adopts only a client held by the runtime's *published* assembly; with no prior publication every pass aborts at `reload.go:587` ("refresh cannot adopt a new client") before `deps.Discover`, the probe never claims, yet `Passes()` still increments (`refresh.go:276`) — so AC5/AC10(b) cannot go GREEN as written and invites a false "public seam not drivable" BLOCKED. Public setup exists (`Runtime.AdmitStartup` `reload.go:502`, `Runtime.Reload` `:479`) but is unstated. Also AC10(b)'s "family name set byte-identical before/after" needs `bridge_catalog_refresh_total` to pre-exist the before-scrape; an empty CounterVec exposes no family (`metrics.go:141,174`), so the live instance must be pre-warmed (healthy setup pass through `RefresherOptions.Metrics`, or explicit warm-up).
Fix: add harness preconditions to §5 AC5/AC10 + §6 T1 inputs: (a) publish the prior assembly holding the same `*Dedicated` via public `AdmitStartup`/`Reload` (`reload.go:479,502,580-587`) before Trigger/Run; (b) pre-warm the metrics instance. No assertion weakened.

**[LOW] Reload leg is behaviorally drivable; source-anchor-only exception understates coverage**
File: spec AD7 / §5 exception / §7. Confidence: HIGH
Issue: `Runtime.Reload` (`reload.go:479`) is public; a ~10-line second case could prove R5's reload leg closes a latched breaker inside the envelope. Recorded reason/consequence is defensible (no production change touches either leg).
Fix: add the reload-leg case, or note the capability was deliberately declined.

**[LOW] AC8 step (i) poll can self-grant a probe**
File: spec §5 AC8(i). Confidence: MEDIUM
Issue: Polling `d.Client().BeginProbe()` before P1 claims (all preconditions then hold) grants the *test* a probe and corrupts the schedule.
Fix: state the ordering — block P1 at the gated transport first (gate arrival proves ownership+claim), then assert denial.

**[LOW] §3 anchor type imprecision**
File: spec §3 metrics bullet. Confidence: HIGH
Issue: "serve.go:236 wires `RefresherOptions.Metrics`" — production wires `LoopOptions.Metrics`; `NewLoop` threads it into `RefresherOptions.Metrics` (`loop.go:76-82`). Substance correct.
Fix: name the actual wired type.

**[LOW] Envelope readPaths omit cited anchor files**
File: execution-state env-1 readPaths vs §3/§6. Confidence: HIGH
Issue: `internal/upstream/prompt_runtime.go:12-53` (§3 evidence, T1 required input) and `.github/workflows/verify.yml:42,48,51` (§3 evidence) are absent from readPaths.
Fix: add both to envelope readPaths or drop from T1 inputs.

### C1/C2 Closure and Reconciliation
- **C1 repaired, source-sound:** every `RecordFailure` branch clearing `probeActive` also sets `openedAt=now` (`runtime.go:266-283`) → admission stays closed throughout (I9 verified); wrapper-local `probeMu`/`probeInFlight` prevents secondary claims; preconditions verified (`BeginProbe` zero production callers; one `Dedicated` per server; refresh reuses held clients). `runtime.go` unchanged ✓; one-shot completion on error/cancellation is normative and load-bearing (callback has no internal set-once, `runtime.go:298-310`) ✓; AC8 gives true RED→GREEN ✓.
- **C2 repaired:** AC10(a) bound to the injected Logger warn sink; AC10(b) bound to the live `*observe.Metrics` (instance-private registry, `metrics.go:141,179`) with byte-identical family/label-key sets and non-vacuous existing-series change; Loki-hop exception carries reason+consequence ✓ (subject to the pre-warm fix above).
- Ticket AC-1..AC-7 mapped (`runPass`→`Trigger`/`Passes`/`Run` recorded); R1–R8→AC1–AC12 trace complete; §6 race/coverage commands cover every planned test name and match the verify.yml gate; envelope matches §10; BLOCKED fallbacks correctly owner-gated.

### Positive Observations
- I9's admission-closed invariant is a sharp, source-verified argument.
- AC8's deterministic clock+gate interleaving is a genuinely discriminating oracle.
- Strict revision discipline: no R/AC/T changes, no envelope widening, exceptions recorded with reason+consequence.

### Recommendation
REQUEST CHANGES

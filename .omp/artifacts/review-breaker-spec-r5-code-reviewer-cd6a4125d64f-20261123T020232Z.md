<task-result id="ReviewBreakerSpecR5" agent="code-reviewer" status="completed" duration="9m29s">

<output>
## Code Review Summary

**Subject mode:** specification (immutable DRAFT; no implementation claimed). **Pass:** repair, round 5.
**Candidate:** `upstream-breaker-recovery-spec-r5`, revision r5 (state sha256 `412b6f9a6cd2…`). Inputs: `.scratch/upstream-breaker-recovery/spec.md`, `.omp/artifacts/upstream-breaker-recovery-planner-r4.md`, `.omp/plans/execution-state.json`, live tree.

**Files Reviewed:** 20 · **Total Issues:** 7 (CRITICAL 0, HIGH 0, MEDIUM 4, LOW 3)

### Closure evidence (r4 → r5)

- **r4 MEDIUM (AC5/AC10 publication path): CLOSED.** `AdmitStartup` is named the sole publication path in spec:41, :111, :116, :124, :151, :212; `Reload` is explicitly rejected with the correct reason (`buildReload` → `in.deps.Discover` `reload.go:618`, failure return `:625`), while `buildStartup` excludes discovery (`reload.go:607-616`; the cited `:616` is the closing brace of the startup outcome append, branch entry `:607`). Live source confirms `AdmitStartup` is discovery-contact-free, and publish + pre-warm **before** latch is consistent in AC5, AC10(b), §5, §6 and §9.
- **r4 LOW (Trigger/Run composition + behavior-level oracle): CLOSED.** spec:122 and AC5 step 7 specify `go ref.Run(ctx)` + `Trigger()` + bounded await + cancel/join and replace `Passes()` with the peer `initializes` delta plus post-pass `AdmitCall`.
- **r4 LOW (buildEnvelope wording/log anchors): PARTIAL.** Vocabulary wording is now correct (`buildEnvelope` rejects level/outcome, event/operation only when empty; unknown event falls to the default message). `NewLogger` `log.go:37` is correct; `eventMessage` default is cited as `:270`, actual `:276`.
- **r4 LOW (numeric anchors): NOT CLOSED** → L1; the §9 claim at spec:202 that anchors were refreshed is false for several symbols.
- **IDs/envelope/runtime.go:** R1–R8, AC1–AC12, T0/T1 are unchanged vs the r4 artifact; §10 is byte-identical (r4:223 vs r5:228); `runtime.go` remains out of the write set and live `Dedicated` still has no probe fields (`dedicated.go:31-38`), so no implementation has begun.
- **Design invariants independently checked:** single-flight ownership is coherent (`BeginProbe` still has zero production callers; only `runtime.go:306` + tests); release-after-complete ordering does not hold `probeMu` across `ServerClient` calls; the tool path is non-probing (`CallTool` → `AdmitCall` only); I9 holds (`RecordFailure` clears `probeActive` only together with `openedAt=now`, `runtime.go:276-294`; `BeginProbe` leaves `openedAt` non-zero, `:306-323`; `AdmitCall` rejects while `halfOpen || !openedAt.IsZero()`, `:247-264`); AC8's gate-first schedule makes step (iv) fail on the pre-r3 design; AC9's post-conditions and AC10's two oracles are correctly connected.

### Issues

**M1 [MEDIUM][statement-contradiction] — AC5/AC10 harness uses a method value of the wrong type.**
File: `.scratch/upstream-breaker-recovery/spec.md:111` (AC5 step 3), `:116` (AC10(b)), `:151` (§6). ID: M1. Confidence: HIGH.
Issue: `ReloadDeps.Discover` is `func(server config.Server) ([]core.ToolDefinition, error)` (`internal/service/reload.go:26`), but `(*Dedicated).Discover` is `func(context.Context) ([]core.ToolDefinition, error)` (`internal/upstream/dedicated.go:72`). `RefreshDeps.Discover = d.Discover` cannot compile, contradicting spec:152 ("assertion failures … not compile errors"). Production adapts it: `registry.discover(server)` wraps `client.Discover(ctx)` (`cmd/mcp-bridge/wiring.go:194-204`). Late discovery; the same literal existed in r4.
Fix: specify the adapter in AC5/AC10/§6, e.g. `Discover: func(server config.Server) ([]core.ToolDefinition, error) { ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout); defer cancel(); return d.Discover(ctx) }`.

**M2 [MEDIUM][ac-not-falsifiable] — the `AdmitStartup` publication literal cannot publish (missing `ModelIdentityDigest`).**
File: spec:41, :111 step 2, :151; `internal/service/reload.go:678`; `internal/storage/reconcile.go:192-193`. ID: M2. Confidence: HIGH.
Issue: `build` always calls `storage.Reconcile` with `ModelIdentityDigest: in.deps.ModelIdentityDigest`, and `ReconcileInput.validate` rejects an empty/non-hex digest. `ReloadDeps{NewClient:…, Now: clock}` leaves it empty, so `AdmitStartup` returns `storage: model identity digest "" is not canonical` before publishing. §3's "requires only `NewClient`+`Now`" is false; every working service test supplies a digest (`internal/service/tools_test.go:129-140`). AC5/AC10(b) never reach publication.
Fix: pin `ModelIdentityDigest: strings.Repeat("f", 64)` in the shared deps and correct spec:41.

**M3 [MEDIUM][ac-not-falsifiable] — refresh deps omit `Now`; AC10(b) can pass with no probe.**
File: spec:111 step 3, :116, :122, :151; `internal/service/refresh.go:54-56,450-478`; `internal/storage/snapshot.go:225-229`. ID: M3. Confidence: HIGH.
Issue: `Runtime.Refresh` rejects `deps.Now == nil` before `build`, so the pass never calls `d.Discover`. AC5's behavior oracle then fails, but AC10(b)'s "series MUST actually appear/increment" is satisfied by `report` falling back to `r.runtime.Current()` (`refresh.go:463-467`), incrementing the pre-existing `bridge_catalog_refresh_total` series (`StatusNone`, `snapshot.go:229`) with no probe — the same false-green class the r4 MEDIUM repaired. `NewRefresher` also requires a `current func() *config.Config` (`refresh.go:111`) the harness text omits.
Fix: pin `RefreshDeps{ReloadDeps: ReloadDeps{Discover: adapter, Now: clock, ModelIdentityDigest: digest}}` and `NewRefresher(rt, func() *config.Config { return cfg }, deps, RefresherOptions{Interval: 0, Metrics: …})`; add a test-local adapter call counter so AC10(b) proves the pass reached `d.Discover`.

**M4 [MEDIUM][statement-contradiction] — AC5's `initializes`-delta oracle needs stateless, which the harness does not pin.**
File: spec:14 (R4 "default"), :111 steps 1/7; `internal/upstream/dedicated.go:23-28` (zero value false), :79-100 (stateful reuses `sessionFor`); `internal/upstream/dedicated_test.go:60-73`; `internal/config/config.go:48-53`. ID: M4. Confidence: HIGH.
Issue: R4's "stateless:true (default)" is the config default (`ServerStateless()`), not the `DedicatedOptions.Stateless` zero value. AC5 builds `d` via `NewDedicated` without pinning `Stateless: true`; the existing helper is stateful. In stateful mode the pre-warm caches the session, so the recovery probe performs no initialize and AC5 step 7's peer `initializes` delta is 0 on a correct implementation.
Fix: pin `DedicatedOptions{Stateless: true, …}` in AC5 and state that R4's default is the config default.

**L1 [LOW][stale-citation] — the claimed r5 anchor refresh did not resolve.**
File: spec:31,33,36,37,40,42,72,122,202. ID: L1. Confidence: HIGH.
Issue: live vs cited — `Discover` 72-164 (cited 71-149; §4:72 instructs extracting that truncated range); `reportDiscoveryFailure` 229-249 (151-196); `Dedicated.Client()` 368 (290); `sessionFor` 372-389 (294-311); `AdmitCall` 247, `RecordFailure` 276-294, `RecordSuccess` 296-304, `BeginProbe` 306-323 (237-252, 266-283, 286-292, 296-314); `serve.go` `newUpstreamRegistry` 183 (134), `ReloadDeps.Discover` 270-272 (217-231), `LoopOptions.Metrics` 285 (236); `metrics.go` private registry 144 (135); `envelope.go` `buildEnvelope` 150, `addEnvelopeFields` 252+ (144-181, 244-333); `log.go` default 276 (270); spec:122 names `RefresherInterval`, which does not exist (field is `RefresherOptions.Interval`). Consequence: §4's extraction range omits the pagination tail/sort/return; the rest is citation drift.
Fix: re-anchor to the live lines and correct the §9 claim.

**L2 [LOW][ac-not-falsifiable] — AC9's "exactly once" is not observable.**
File: spec:115. ID: L2. Confidence: HIGH.
Issue: two completions are indistinguishable from one; only "breaker open, `openedAt` reset, ownership released, re-claim only after cooldown" is falsifiable.
Fix: keep the behavioral assertions and label "exactly once" as the I11 design invariant (or observe a callback counter).

**L3 [LOW][ac-not-falsifiable] — AC10(b) passes identically with and without the probe wrapper.**
File: spec:116. ID: L3. Confidence: HIGH.
Issue: a plain non-probing discovery failure produces the same `upstream_discovery_failed` record and the same `bridge_catalog_refresh_total` increment, so "Drive a failing probe" overstates the oracle (R7 is telemetry-only).
Fix: restate AC10(b) as "failing discovery", or add the adapter counter from M3.

### Open Questions
- Is AC10(b) intentionally telemetry-shape-only? If yes, state it; if not, the M3 counter is required. Confidence LOW on intent.

### Positive Observations
- The publication-path repair is correctly reasoned and consistently propagated; `AdmitStartup` is genuinely contact-free, and the `Reload` rejection is anchored to the right behavior (618/625).
- Single-flight ownership, release-after-complete ordering, panic safety and non-probing tool admission are coherent; `BeginProbe` still has zero production callers, and the change stays inside `dedicated.go`.
- AC8's gate-first schedule avoids the test granting itself a probe; the discriminating assertion (P1 success + P2 plain failure → `AdmitCall` admits) fails on the pre-r3 design.
- Replacing `Passes()` with the behavior-level oracle is the right call.

### Review limits
- Read-only profile with no shell/VCS: I could not run `git status`/diff or recompute the candidate sha; "runtime.go unmodified" is verified as write-set exclusion plus the live pre-implementation state, not as commit history.
- No tests/builds were executed (specification mode); all dynamic claims above are source-anchored.

### Recommendation
REQUEST CHANGES
</output>
</task-result>
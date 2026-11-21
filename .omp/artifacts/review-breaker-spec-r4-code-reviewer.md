<task-result id="ReviewBreakerSpecR4" agent="code-reviewer" status="completed" duration="6m1s">

<output>
All verification complete. Composing the final review.

**Verification summary of my investigation:** All §3–§10 anchors were checked against the live tree — breaker primitives (`BeginProbe` runtime.go:296-314, `AdmitCall` :237-252, `RecordFailure` :266-283, `RecordSuccess` :286-292), the C1 invariant (:270-272/:279-282 both clear `probeActive` **and** re-open), reload publication seams (:479/:502/:580-582/:587/:618/:625), Refresher public seam (:111/:152/:179/:186/:212/:276/:450-478), metrics (:135/:141/:174/:179/:226), loop threading (serve.go:236 → loop.go:78-84 → refresh.go:74), wiring (:139/:177/:192), verify.yml (:42/:48/:51), test oracles (dedicated_test.go package/`newPeer`/`peerState`; runtime_test.go:94; runtime_edge_test.go:145,149; the three AC12 test names), imports (service does **not** import upstream), ticket 01, incident, and backlog all reconcile.

## Code Review Summary
Spec mode; candidate `upstream-breaker-recovery-spec-r4` (planner draft ≡ `.scratch/upstream-breaker-recovery/spec.md`). R1–R8/AC1–AC12/T0–T1 reconciled with ticket 01, incident, backlog.
**Files Reviewed:** 14 · **Total Issues:** 4

### By Severity
CRITICAL 0 · HIGH 0 · MEDIUM 1 · LOW 3

### Issues

[MEDIUM] "(or `rt.Reload`)" publication alternative is not probe-neutral (AC5/AC10 harness)
File: spec.md §5 AC5+AC10 rows, "Harness setup preconditions", §6 harness instructions; reload.go:479,502,616-618,625
Confidence: HIGH
Issue: buildReload publication itself runs `deps.Discover` per server (reload.go:618; :616 excludes only buildStartup) and fails on discovery error (:625). Read literally (AC5: latch→advance→publish→drive), a Reload publication claims the probe and closes the breaker first; the Refresher pass then runs plain discovery — GREEN is caused by the publication, defeating the precondition's purpose ("assert the probe claimed", not bare `Passes()`, which increments anyway, refresh.go:276). In AC10(b) (failing peer) Reload publication fails → runtime unpublished → pass aborts at :587. The "initial healthy pass" pre-warm has the same latch-order trap. `AdmitStartup` is contact-free and order-safe.
Fix: name `AdmitStartup` the sole publication path (drop "(or rt.Reload)" in §5 ×2 and §6) and pin the order — publish + pre-warm while the breaker is closed, latch last; or keep Reload and state its discovery side effect plus ordering/healthy-peer requirement.

[LOW] Trigger-without-Run offered as a driver alternative
File: spec.md §5 AC5 row; §6; refresh.go:152-158,186-210,212
Confidence: HIGH
Issue: `Trigger()` only queues; only `Run`'s select loop executes passes (`runPass` :212 unexported) — a Trigger-only harness hangs at `Passes()==0`.
Fix: state the composition — `go ref.Run(ctx)` (Interval 0) + `Trigger()` + await `Passes()≥1` + cancel; or short-interval cancellable `Run(ctx)`.

[LOW] §3 overstates `buildEnvelope`; stale log anchors
File: spec.md §3; envelope.go:144-181; log.go
Confidence: HIGH
Issue: level/outcome are vocabulary-validated, but event/operation are rejected only when empty — unknown events take the generic default message (log.go:270) and pass. Cited `log.go:44` is `WriteJSONLine` (`NewLogger` :37); `eventMessage` spans :228-272, not 198-264. No feature consequence (no new events planned; AC10(a) asserts the exact record).
Fix: reword ("level/outcome vocabulary-validated; event/operation non-empty; unknown events fall to the default message"); refresh lines.

[LOW] Stale numeric anchors inherited r1→r4 (T1 read inputs)
File: spec.md §3/§6
Confidence: HIGH
Issue (grep-verified): `ServerOptions.Now` :27 (cited 19); `now: opts.Now` :167 (cited 126-150); `IsBusy/IsUnavailable` :115-116 (cited 108-109); `newUpstreamRegistry` wiring.go:139 (cited 137); `NewDedicated` :177 (cited 175); serve.go:134 (cited 133); `BeginProbe` :296-314 (cited −312); `Discover` :71-149 (cited −140); `sessionFor` :294-311 (cited −309). Symbols resolve; ranges mostly overlap.
Fix: one anchor-refresh pass over §3/§6.

### Prior-round closure — all five r3 findings verified resolved (live-source)
MEDIUM: preconditions real and exact — `AdmitStartup` reload.go:502 contact-free, adoption :580-582, abort :587 precedes `deps.Discover` :618, `Passes()` unconditional refresh.go:276; pre-warm real (lazy CounterVec, family metrics.go:174, `RecordRefresh` :226 server-only). LOWs: AD7 (Reload :479 public/drivable; decline recorded with reason+consequence §5/§7); AC8 step-(i) (P1 gate-arrival now precedes the denial poll — a pre-arrival poll would self-grant); `LoopOptions.Metrics` (serve.go:236 → loop.go:78-84 → refresh.go:74); T1/background-anchor split (§6). Breaker design verified sound: zero production `BeginProbe` callers; AD8/I4-I9 ownership + release-after-complete prevent owner clobber; AC8 deterministic, RED pre-r3 at step (iv); `runtime.go` untouched. Ticket AC-1..7 → AC1-AC7 (`runPass`→`Trigger`/`Passes`/`Run`); commands mirror verify.yml:42,48,51 + ticket AC-7; R8/AC11 match `sessionFor` :294-311 (no eviction) + ticket-2 boundary; §7 exceptions all carry reason+consequence; §10 envelope consistent with §6 fallback (BLOCKED-with-evidence; no cmd/ relocation; owner-gated widening). No R/AC/T IDs added/removed.

### Positive Observations
- Precondition and `Passes()` false-GREEN analysis is live-source exact; AC8 gate-first scheduling with the no-pre-arrival-poll warning is the right deterministic discipline; RED paths are specified as assertion failures, not compile errors.

### Recommendation
REQUEST CHANGES
</output>
</task-result>
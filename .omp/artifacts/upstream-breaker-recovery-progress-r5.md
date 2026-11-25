# upstream-breaker-recovery — progress checkpoint (r5)

Saved: 2026-11-25. Work is paused mid-review, before implementation. No production or test
source was changed.

## Canonical state

- `.omp/plans/execution-state.json`: feature `upstream-breaker-recovery`, revision `r5`.
- `authorization: null`, `reviews: null`; tasks CR1/CT1/T1 all `pending`.
- Installed by `execution-state.mjs transition` from
  `.scratch/upstream-breaker-recovery/new-state-r5.json` (validated by `check`; packet digest
  `b9dd134328e222683c37ce29186909d35a3cdf34f6fae5b831a14d5176539894`).
- The outgoing `startup-diagnostics` r8 state was archived to
  `.omp/plans/startup-diagnostics/execution-state.json`.

## Specification

- `.scratch/upstream-breaker-recovery/spec.md` is now **r5** (bytes identical to
  `.omp/artifacts/upstream-breaker-recovery-planner-r5.md`,
  sha256 `412b6f9a6cd2bb600a7f22502a0b5b78f84008fdbc03d16f24df0a4743236138`).
- r5 repaired the r4 CodeReviewer MEDIUM (AC5/AC10 must publish with `AdmitStartup`, not
  `Reload`) plus three LOW findings. R1–R8, AC1–AC12 and the write envelope
  (`internal/upstream/dedicated.go`, `dedicated_test.go`, `breaker_probe_recovery_test.go`)
  are unchanged.

## Reviews

- Candidate `upstream-breaker-recovery-spec-r5` is registered in
  `.omp/plans/review-sequence.json` with both descriptors (CR1 code-reviewer, CT1 critic).
- **CodeReviewer round 5 completed**: `ReviewBreakerSpecR5`, status succeeded,
  invocationId `df577a33-5776-41fd-8e67-4a06b2dd395e` (recorded in
  `.omp/plans/review-runs.json`).
- Untouched output: `.omp/artifacts/review-breaker-spec-r5-code-reviewer-cd6a4125d64f-20261123T020232Z.md`
- Verdict: **REQUEST CHANGES** — 0 CRITICAL, 0 HIGH, 4 MEDIUM, 3 LOW. All four MEDIUM are
  harness-feasibility defects in AC5/AC10, not breaker-design defects:
  - **M1** `RefreshDeps.Discover`/`ReloadDeps.Discover` is `func(config.Server) ([]core.ToolDefinition, error)`; `(*Dedicated).Discover` is `func(context.Context) (...)`. `Discover: d.Discover` cannot compile; an adapter is required.
  - **M2** `AdmitStartup` publication needs `ModelIdentityDigest` (non-empty hex) in `ReloadDeps`; the r5 harness omits it, so publication fails before `deps.Discover`.
  - **M3** `Runtime.Refresh` rejects `deps.Now == nil`; AC10(b) can pass without any probe because `report` falls back to `runtime.Current()`. `NewRefresher` also requires `current func() *config.Config`.
  - **M4** `DedicatedOptions.Stateless` zero value is false; `stateless:true` is only the config default. AC5 must pin `Stateless: true`, otherwise the `initializes` delta is 0.
  - LOW: stale numeric anchors (L1), AC9 "exactly once" unobservable (L2), AC10(b) identical with/without probe (L3).
- The CR observation is **not yet bound**: `review-sequence.json` has no round entry for this
  candidate (`reviewHistory` count 0). Binding contract: `execution-state.mjs bind` with
  `{binding, runBinding, status, options}`; `binding` fields per `reviewBindingFields` in
  `workflow-policy/execution-review-sequence.mjs` (candidateId, reviewerId, authorIds, inputs,
  dispatches, verdict, findings, format `installed`, source {path,sha256}, recordedAt);
  `runBinding` = candidateId, invocationId above, fingerprint
  `631ecee0df91fa8c0f1a51101503851a22c8b0b7950bcb440efbc91fd138c39d`, reviewerId
  `code-reviewer`, agent `code-reviewer`, revision `r5`.
- Critic (CT1) was **not** dispatched.

## Budget and blockers

- Completed CodeReviewer rounds on this planning task: r1, r2, r3, r4, r5 = **5**, i.e. the
  RULES.md S15 default budget is exhausted.
- r5 verdict is not PASS, so S14's "two independent PASS reviews" prerequisite is unmet and
  T1 cannot be dispatched. Fresh MEDIUM findings also require another planner repair cycle.
- Resuming requires an owner decision: authorize further spec repair rounds (and say how many),
  accept the residual findings with a recorded residual register, or drop/reshape the ticket.

## Artifacts created this session (all uncommitted before this checkpoint commit)

- `.omp/artifacts/build-breaker-r5-state.mjs` — builds the r5 candidate state from the planner artifact.
- `.omp/artifacts/register-breaker-r5-candidate.mjs` — registers the r5 candidate + descriptors.
- `.omp/artifacts/breaker-managed-code-review-seed-r5.json`, `...-critic-seed-r5.json`,
  `...-dispatch-request-r5.json`, `...-code-review-native-r5.json`, `...-code-review-payload-r5.json`.
- `.omp/artifacts/upstream-breaker-recovery-planner-r5.md` (frozen copy).
- `.omp/artifacts/review-breaker-spec-r5-code-reviewer-cd6a4125d64f-20261123T020232Z.md`.
- `.scratch/upstream-breaker-recovery/new-state-r5.json`.

## Resume sequence (after the owner decision)

1. Bind the completed CR5 observation with the inputs above.
2. If repair is authorized: planner r6 closes M1–M4 (+ L1–L3 as text), rebuild the candidate
   state with the same builder, transition/install, rerun CR then Critic.
3. If resume without repair is authorized: dispatch CT1 against the exact r5 candidate
   (`.omp/artifacts/breaker-managed-critic-seed-r5.json`), then decide on residuals.
4. Only after both reviews satisfy the applicable rule set: milestone authorization, then
   engineer T1 (bounded to `internal/upstream/dedicated.go` + the two test files).

## Not done

No implementation, no tests, no gate marker change, no commit of source, no deployment.

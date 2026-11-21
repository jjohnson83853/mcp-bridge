### Verdict
REVISE

Spec ID: upstream-breaker-recovery-spec-r2
Revision: r2
Findings by ID: C1, C2

Reviewed `.scratch/upstream-breaker-recovery/spec.md` against ticket 01, both CodeReviewer reports, and live source.

**Round-1 closure:** All seven requested amendments are present: owner-gated AC5 fallback, corrected citations, focused-test selection, reload coverage exception, normative deferred completion, named metrics oracle, and public Refresher mapping. However, naming the metrics oracle does not make it effective (C2), and the root concurrency invariant remains false (C1).

**C1 — MAJOR: concurrent failure invalidates probe ownership. Confidence: HIGH.**
Spec §4/I4 and R6 claim `probeActive` guarantees one in-flight probe. But `RecordFailure` clears both `halfOpen` and `probeActive` on authentication or threshold failures (`internal/upstream/runtime.go:266-283`); completion unconditionally overwrites current state (`:294-312`).

Reachable schedule:
1. A previously admitted tool/prompt call remains outstanding while other failures open the breaker.
2. After cooldown, discovery P1 claims a probe and blocks.
3. The outstanding call fails, clearing P1's reservation and restarting cooldown.
4. After another cooldown, P2 claims while P1 is still running.
5. P1 succeeds; its callback clears P2's reservation and opens admission while P2 remains active.

This violates R3/R6 and AC8. Prompt requests are additional writers (`internal/upstream/prompt_runtime.go:32-53`). The earlier review's “cooldown prevents another claim” argument covers only the immediate instant, not the complete transaction.

**Amendment:** Specify ownership across every failure writer and callback, including stale-completion and newer-failure precedence. Extend AC8 with this deterministic, clock-driven interleaving and assertions that admission remains closed throughout an active probe. Revise AD2/I4/T1 consistently; obtain owner authorization if repair requires widening the `runtime.go` exclusion. Do not weaken R6.

**C2 — MAJOR: AC10's telemetry oracle is disconnected. Confidence: HIGH.**
Spec §5/§7 scrapes a newly created `observe.NewMetrics` instance that discovery never receives. Each instance owns a private registry (`internal/observe/metrics.go:141-180`), so equal scrapes cannot establish probe telemetry behavior. Section 7 also names the Loki path without delivery verification or a justified delivery-verification exception.

**Amendment:** Exercise the existing Refresher metrics callbacks against the scraped instance; distinguish allowed existing counter/series changes from prohibited new instrumentation. Bind delivery verification to AC10/T1, or document a proportionate exception with reason and consequence.

**Coverage/limits:** Ordinary success/error/cancellation accounting, session-policy preservation, public refresh-test feasibility, reload exception, and rollback are coherent. No implementation or runtime verification exists; no commands or mutations performed.

T1 is not execution-ready. Revisions require fresh candidate-bound reviews; owner checkpoints remain binding. Request coordinator escalation to configured non-OpenAI `spec_critic_escalated` for the substantive concurrency disagreement.

### Recommendation
REQUEST CHANGES

REVIEWER: code-reviewer
PACKET: c46c23dc18196a9cfa7029c19fc7e6f9656091e62ffabf0aea410ef3b563fe75
VERDICT: revise

Mode: specification review of r8-pin-diagnostics-narrow4. Read: spec.md (sha256 `a927292cd4618510d082661c932b784ef74e8b7047b93b0f71940dabb5a1ec21`, matches state `spec.sha256`), decisions.md, issues/08-12, execution-state.json (revision narrow4, packet c46c23dc…), narrow4 delta, prior narrow3 review. Read-only; no hashes recomputed.

**1. MARKERS — pass.** `specdd:` occurs only at spec.md:40/67, 169/204, 271/287, 289/303: four unique pairs, ordered, non-nested, no other token.

**2. COVERAGE — pass.** R20→AC08-1/AC09-7 … R25→AC08-1,7/AC11-3 (:267). Every AC08-1..AC12-3 names its ticket and has exactly one state owner (state:507-600 T8, 601-705 T9, 707-770 T10, 772-809 T11, 811-860 T12), matching the spec EC ids (:240-265).

**2b. NARROWING CLOSURE — revise.**
Closed: H2 (issues/10:17); H1 spec-side (R24:64, AC11-1:260, AC11-2:261, issues/11:7,12,13); G3 (AC09-5:251, issues/09:18); G4 (R23:63); G5 (AC10-4:258); M1 spec (AC08-1:240, AC08-7:246); M2 (issues/08:14, 09:18, 12:14); L2 (R23:63). M3 declined and recorded (decisions.md:176; backlog ticket 15) — not grounds.
UNCLOSED: H1's task-ledger leg. spec.md:285 still reads "parity test drives the SAME pinned-catalog rejection through the SIGHUP reload and periodic refresh paths", although delta:66-70 promised the config-validation-pin RED. That comparison is unwritable: a runtime AddPins rejection logs "configuration reload rejected" (loop.go:204) → `reload_rejected`, no `config` (wiring.go:461-467); `config_reload_rejected` fires only for `*config.Diagnostics` (wiring.go:428-438). Fix: apply delta:70's T11 RED replacement verbatim at :285.
L1 unclosed: :284/:285 lack the promised envelope-wide prohibited clauses (delta:63-64, :70). Fix: append those replacement tails.

**3. ACs — pass.** AC11-1 now compares reachable carriers; AC08-7 is falsifiable; all are testable and producer-named. Advisory: issues/08:20 (AC-7) still carries the vacuous "only at debug level" form after :246 was fixed — mirror spec AC08-7.

**4. RULINGS — pass.** No metric change (spec:298-299, T8:282, T10:284; decisions checkpoint item 3); no `schema_version` change (AD20:201, :299); no raw error text (R8:48, T8:282, T9:283, AC10-3:257); additive-only r7 records (AD21:203, T11:285, :299); no induced-failure gate (T12:286, :66). The L1 omission permits nothing, so fidelity holds.

**5. ENVELOPE/STATE — revise (low).** State edges (T8 []→T9[T8]→T10[T9]→T11[T9,T10]→T12[T9,T10,T11]; state:342-481) enforce strictly serial dispatch; T8/T9/T11/T12 write paths match. T10 write extras (runtime.go, serve.go, dedicated_test.go) and missing service_edge_test.go are recorded (spec:164, issues/17) — deferred, not grounds. Unrecorded: T10's dependency [T9] (state:411-413) contradicts ledger "Deps: T8" (:284), envelope "T9,T10 after T8" (:292) and issues/10 "Blocked-by: 08", while the L3-edited prose (:273) says T10 after T9. Fix: :284→"Deps: T9"; :292→"T9 after T8; T10 after T9"; issues/10:3→"Blocked-by: 09". Advisory: AC10-5 needs a rejection hook wired in serve.go (`Published` fires only on success, refresh.go:211; serve.go:135,232-234); serve.go…

**6. DEFECT SITES — pass.** refresh.go:195-205 (runPass emits nothing), :210-242 (err only gates publication; :224 falls back to `Current()`); reload.go:561-564 text-only wrap; core.go:308-313 first-only text; wiring.go:373-407 publish-only readiness; ops.go:19 `Reason,omitempty` with finite classes refresh.go:17-28; dedicated.go:122-175 raw wrapped discovery errors, no stage/status; log.go:185-218 lacks catalog/pin/discovery events. Every claimed defect exists.

Open questions (low, non-blocking): AD19:199 "17→24" versus 16 distinct events in log.go:185-218; R25 level asserted only for `catalog_refresh_failed` (AC08-7); T9's all-pin enumeration originates in core.AddPins (core.go:308-313), outside T9's file list — feasible via core.Binding/PublicName but should be stated.

Severity: CRITICAL 0, HIGH 1 (H1/L1 task-ledger text), MEDIUM 2 (:284/:292 contradiction; issues/08:20), LOW 3.
Positive: AC11-1's retarget is precise and evidence-backed; AC08-7 is now falsifiable; M3's decline is properly recorded with a backlog home; the defect inventory is accurate.

### Recommendation
REQUEST CHANGES

[Some lines truncated to 768 chars. Use agent://ReviewR8Narrow4Spec:raw:20-20 to read line 20 whole]

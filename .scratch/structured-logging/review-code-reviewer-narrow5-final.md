## Spec review — r8-pin-diagnostics-narrow5 (specification only; no implementation in scope)

REVIEWER: code-reviewer
PACKET: 35ab8658c9df716c26c0d309076de239fd62cc32e44681b417a8ef93f6de0519
VERDICT: revise

Mode: specification. state:4 revision and state:11 spec sha256 83cd82… match the packet. Issues: CRITICAL 0, HIGH 1, MEDIUM 2, LOW 2.

1 MARKERS — PASS. Exactly 8 tokens: spec:40/67, :169/204, :271/287, :289/303; unique, ordered, non-nested, own lines.

2 COVERAGE — PASS. :267 maps R20→AC08-1/AC09-7, R21→AC08-6/AC09-1..6,8, R22→AC08-1..6, R23→AC10-1..5, R24→AC08-2/AC11-1..2, R25→AC08-1,7/AC11-3. Each AC08-1..AC12-3 has exactly one owner (state:530-857; tasks:369-506).

2b NARROWING CLOSURE — REVISE. Closed in spec: level asserts :247/:255; T9 debug+AddPins scope :283 (core.go:302-311 first-only verified; core.go not a T9 write path); induced-failure wording :284/:285/:299; AD19 16→23 :199 (log.go:191-219 = 16 names verified); AD21 fallback :203 (log.go:30-34 verified); AC12-3 oracle :265; AC10-5 hook :259 (refresh.go:210-241, err read only at :211). UNCLOSED: the AC repairs exist only in the spec — issues/09:14, 10:14, 12:14 keep pre-narrow5 text while spec:208 declares 1:1 spec↔ticket AC mapping; issues/12:14 still makes the AC-1 doc-drift test AC12-3's oracle, exactly what :265 retracts. Unrecorded: state:165 R24 summary keeps the SIGHUP↔periodic-refresh parity superseded at spec:64 and decisions narrow1-3; absent from issue 17's deferred list.

3 ACs — REVISE. :247/:255/:265/:266 are falsifiable and producer-named. F1 [HIGH] spec:265 vs issues/12:14 contradiction; F2 [MEDIUM] spec:247/:255 vs issues/09:14, 10:14 (level obligations missing ticket-side). F5 [LOW] :267 omits AC09-1/AC10-1 from R25.

4 RULINGS — PASS. No metric changes :299, T8:282, T10:284, T11:285; no schema_version AD20:201, :299; no raw error text R8:48, T9:283; additive-only AD21:203, T11:285; no debug-only T9/T10/T11; induced-failure matches ruling 19. M3 decline recorded (decisions:176) → deferred, not grounds.

5 ENVELOPE/STATE — PASS with recorded deferrals. Effects mirror :278-286 except state:359 T10 adds upstream/runtime.go, cmd/mcp-bridge/serve.go, upstream/dedicated_test.go (spec:165/issue 17); spec:295 service_edge_test.go orphan (recorded); state:355 T8 blockedBy [] vs "Deps: T7" (recorded). F3 [MEDIUM] state:165. Edges state:386/:421/:455/:487 = :273/:292; strictly serial holds.

6 DEFECT SITES — PASS. Verified: refresh.go:210-241 err only compared at :211, failed pass falls to runtime.Current(); :195-206 runPass emits none; reload.go:562-564 text-only wrap; core.go:302-311; wiring.go:373-408 publish-only readiness; ops.go:19 omitempty; log.go:191-219 no catalog/pin event. F4 [LOW] :259 calls :211 the "failed-pass branch" (it is the success guard).

Fix (smallest): copy spec:247/:255/:265 texts into issues/09:14, 10:14, 12:14; replace state:165 with spec:64 or append both to issue 17. No R/AC/T/AD id change.

Positive: spec-side repairs are accurate, cited lines verified against source, and every prior non-drift finding is genuinely closed.

### Recommendation
REQUEST CHANGES

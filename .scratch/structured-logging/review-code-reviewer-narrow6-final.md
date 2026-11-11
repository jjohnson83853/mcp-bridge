## Spec review — r8-pin-diagnostics-narrow6 (spec only; read-only, no checks run)

REVIEWER: code-reviewer
PACKET: 4b3770469ee9da8fc1b23650c2d0db2492d685341b146f79636831e7c021ca42
VERDICT: revise

Inputs: spec.md (05a4a8e9…), decisions.md, issues/08-12, execution-state.json, narrow6 delta, narrow5 review. Findings: MEDIUM 3, LOW 3.

1 MARKERS — PASS. 8 tokens, own lines, unique/ordered/non-nested: :40/:67, :169/:204, :271/:287, :289/:303.

2 COVERAGE — PASS. :267 maps R20→AC08-1/AC09-7, R21→AC08-6/AC09-1..6,8, R22→AC08-1..6, R23→AC10-1..5, R24→AC08-2/AC11-1..2, R25→AC08-1,7/AC09-1/AC10-1/AC11-3; state `ownerTaskIds` give exactly one owner per AC (AC08*→T8, AC09*→T9, AC10*→T10, AC11*→T11, AC12*→T12). Mapping defect F4.

2b NARROWING CLOSURE — PASS. Closed: issues/09:14, 10:14, 12:14 now mirror :247/:255/:265 (F1/F2); :265 makes `go test ./...` AC12-3's own oracle; :267 adds AC09-1/AC10-1 (F5); state requirements[R24] now equals :64 (F3); :259 now reads "success guard … falls through to `runtime.Current()`" (F4), verified refresh.go:210-224 (err read only at :211). Recorded, not grounds: ticket-17 drift (spec:166), M3 decline (decisions narrow4 §5), r7 A1-A12 (:3).

3 ACs — PASS. AC08-1..AC12-3 are falsifiable and producer-named; AC09-5 carries its own ≤64 oracle; AC10-5 asserts behavior, not implementation.

4 RULINGS — REVISE. Ledger prohibitions/blockers match (no metric change :282-285/:299, no schema_version AD20:201, no raw text T8:282/T9:283, additive-only AD21/T11). Gaps:
- F1 [MEDIUM, HIGH] AD21:203 sends warn-level `logging_write_failed` to "the lane opposite the one the failed record routed to" — stdout after a failed stderr warn/error write — contradicting D5/R4 (:44); no AC pins the fallback stream. Fix: declare the D5 exception in AD21 + §7 and assert the lane in AC11-3.
- F2 [MEDIUM, HIGH] Ruling 12 (candidate counts vs retained counts) is in AD16:193 (`candidate_servers`/`candidate_tools`) but not in R22:62 or AC08-3:242; no AC asserts those fields. Fix: extend AC08-3 (spec:242, issues/08:19).

5 ENVELOPE/STATE — PASS. T8-T12 effects mirror :282-286 (T8 5, T9 6, T11 7, T12 4 paths) except recorded drift (T10 adds runtime.go/serve.go/dedicated_test.go; service_edge_test.go orphan at :296; T8 blockedBy []). Edges (:273/:292, state) form one chain T8<T9<T10<T11<T12 ⇒ strictly serial. Defects:
- F3 [MEDIUM, MEDIUM] AD19:199 extends `eventMessage` 16→23 and adds operations `refresh`/`logging`, but no AC asserts the r8 records' operation/message; AC04-1's oracle is r7-producer-scoped (:234). Fix: let AC12-1's doc-drift test assert the eventMessage set and each r8 record's operation.
- F4 [LOW, HIGH] :267 claims R25→AC09-1/AC10-1, yet :247 tags R21 only and :255 R23 only; state `requirementIds` match the tags. Fix: add R25 at :247/:255 and in state AC09-1/AC10-1.
- F5 [LOW, MEDIUM] AD16 makes `next_attempt_ms` unconditional on `catalog_refresh_failed`; R23:63/AD17/AC10-4 condition it on a scheduled retry, so a retryable-false pin rejection would misstate. Fix: make AD16's field conditional.

6 DEFECT SITES — PASS. Verified: refresh.go:195-205 emits no pass records; :210-224 err read only :211, fallback :224; reload.go:562-564 text-only wrap; core.go:309-311 first-only; wiring.go:373-408 readiness on publish only (serve.go:135); ops.go:18-19 `omitempty`; log.go:185-219 16 names, no catalog/pin event; dedicated.go:122-134 raw text, no stage; Trigger:134/watch:191 exist. Note: AC11-1's :428-438/:461-467 sit 2-5 lines off post-implementation (cases :433/:460) — informational.

Positive: narrow6 is exactly its delta (no R/AC/T/AD id change), the AC10-5 correction is accurate, and every narrow5 finding is genuinely closed.

### Recommendation
REQUEST CHANGES

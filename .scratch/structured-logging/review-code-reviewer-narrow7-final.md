## Review — SPEC-STRUCTURED-LOGGING r8-pin-diagnostics-narrow7
Spec-only, read-only. Findings: HIGH 1, MEDIUM 1, LOW 3.

REVIEWER: code-reviewer
PACKET: 08c91efdc41f83fd6d141e3d71ad544b4c147fed0d4d39a6b9d8438ae4371144
VERDICT: revise

1 MARKERS — pass. Four pairs, 8 tokens, own lines, unique, ordered, non-nested: :40/:67, :169/:204, :271/:287, :289/:303.

2 COVERAGE — pass. :267 maps R20→AC08-1/AC09-7; R21→AC08-6/AC09-1..6,8; R22→AC08-1..6; R23→AC10-1..5; R24→AC08-2/AC11-1..2; R25→AC08-1,7/AC09-1/AC10-1/AC11-3. State `ownerTaskIds` give exactly one owner each (AC08*→T8 … AC12*→T12), and the T8-T12 `acceptanceIds` sets are disjoint. LOW: :267 omits AC12-1..3, which the state tags R18/R5/R20-R25.

2b NARROWING CLOSURE — revise. All five prior groups are closed in the spec: AD21:203 + §7:312 declare the D5 lane exception and AC11-3:262 asserts it (F1); AC08-3:242 + issues/08:16 add the ruling-12 candidate counts (F2); AC12-1:263 asserts the 23-name eventMessage set and per-record operations (F3); :247/:255 add R25 (F4); AD16:193 makes `next_attempt_ms` conditional (F5). Defect: three closures never reach the tickets the tasks execute — issues/09:14 carries R21 only, 10:14 R23 only; 11:14 AC-3 lacks the opposite-lane clause; 12:12 AC-1 lacks the eventMessage/operation doc-drift obligation. [HIGH] for 11:14 (a same-lane fallback passes the ticket, defeating ruling 17), [MEDIUM] for 12:12, [LOW] for the tags. narrow6 closed this identical class by mirroring 09/10/12; narrow7's delta §2 mirrors only issues/08. Smallest fix: mirror those four amended lines into the tickets.

3 ACs — pass. AC08-1..AC12-3 are falsifiable and producer-named; AC09-5 carries its own ≤64 oracle; AC10-5 is behavioral and its hook is real (refresh.go:210-224, `err` read only at :211, fallback :219-224).

4 RULINGS — pass. Ledger prohibitions match decisions: T8 no metric/reload.go-wiring/schema_version + R8 blocker (:282), T9 no raw text/no debug-only (:283), T10 metric/schema/debug/induced-failure (:284), T11 additive-only AD21 (:285), T12 (:286). LOW: §7:315 still calls the counter gap "acceptance pending owner ruling"; decisions.md r8 checkpoint item 3 ACCEPTED it — replace that phrase.

5 ENVELOPE/STATE — pass. State packet 08c91efd…, all four section revisions narrow7; T8-T12 effects mirror :283-286; blockedBy T8<T9<T10<T11<T12 is one total chain ⇒ strictly serial. Owner-deferred, not grounds (issue 17 / spec:165): T8 `blockedBy` [] (missing T7 edge), T10 `runtime.go`/`serve.go`/`dedicated_test.go`, `service_edge_test.go` absent from effects, decisions' 26842ba7 packet mismatch. LOW: issue 17's own evidence is stale (T10 `blockedBy` is now ["T9"]).

6 DEFECT SITES — pass. refresh.go:210-224 (err never recorded), `runPass` :195-204 (no records), reload.go:563-565 (text-only wrap), core.go:311 (first-only), wiring.go:373-409 (readiness on publish only), ops.go:19 (omitempty), log.go:185-220 (16 names, no catalog/pin event), dedicated.go `Discover` (raw text, no stage), loop.go:191 / refresh.go:134 (triggers). The cited reload.go:562-564 is one line off; the defect is real.

Positive: AD20's 20-key list covers AD16-18 including `endpoint`; AC12-3 is its own `go test ./...` oracle.

### Recommendation

REQUEST CHANGES

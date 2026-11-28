# Ticket 05 — planning and review ledger (#119)

Feature: `token-efficient-tool-responses` ticket 05, default field projections for allowlisted list operations.
Canonical execution state: not yet installed (planning candidate phase).

## Authority and inputs

- Owner selection: current session, "lets start 119" (2026-11-27). This selects the ticket for planning; implementation still requires milestone authorization.
- Owner review budget: **15 completed CodeReviewer reviews for this ticket**, explicit owner instruction 2026-11-27: "set budget to 15 for the planner" (S15 owner-set positive integer).
- Authoritative intake: GitHub `jjohnson83853/mcp-bridge#119`, body sha256 `caf4384624c8ce814fe3df8a9019c2a410c976e6aaeddf276c5fffefc0e0fd81`; comment 2026-11-27T16:15:49Z.
- Refinement: `.omp/artifacts/token-eff05/refinement.md` (READY).
- Stored-schema evidence: `.omp/artifacts/token-eff05/schema-portainer_endpointlist.json`, `schema-portainer_docker_proxy.json`.
- Parent feature spec: #157; ticket 01 bounded paging delivered (`.omp/plans/token-efficient-tool-responses/execution-state.json` r40).

## Review budget accounting

| Unit | Stage | Invocation | Candidate/revision | Outcome |
| --- | --- | --- | --- | --- |
| — | initial refinement | `RefineTicket119` | issue body digest above | READY |
| — | planner | `PlanTicket119` | r1 pending | pending |

Consumed CodeReviewer reviews: **0 / 15**. Critic consumes no budget unit.
Prior lineage: none (new ticket). No budget reset; count is cumulative across sessions.

## Findings / decisions

- 2026-11-27: Proxmox/OPNsense allowlist entries excluded (ticket's conditional inclusion not exercised; decisive schemas/payloads not captured). Revisit only with owner authority.

## Review round 1 (2026-11-28)

- CodeReviewer `ReviewTicket119SpecR1` on candidate `token-efficient-tool-responses-ticket05-spec-r1` (spec sha256 a31552a04731e3229057265f6bd7642a998003c02454327d98e9c5fdc030ef3f): **REQUEST CHANGES**, review seq 103.
- Findings: F1 HIGH (raw structuredContent remains unprojected; AC1 partial), F2 HIGH (paging/terminal drops disclosure; no default+paged oracle), F3 MEDIUM (Status/GroupID not evidenced; narrow or evidence), F4 MEDIUM (fields+non-list precedence ambiguity), F5 MEDIUM (decision table incomplete: IsError/select/no-match), F6 MEDIUM (counter tool label undefined for rejection), F7 MEDIUM (1–16 unique not enforced or tested), F8 LOW (file counts/deps/RED wording).
- Source: `.omp/artifacts/token-eff05/review-cr-r1-source-4fc401e5-20261128T030855Z.json`.
- Consumed CodeReviewer reviews: **1 / 15**. Next: planner r2 repair of F1–F8 with no new R/AC/T IDs; then fresh candidate + CodeReviewer round 2.

## Review round 2 (2026-11-28)

- CodeReviewer `ReviewTicket119SpecR2` on candidate `token-efficient-tool-responses-ticket05-spec-r2` (spec sha256 dec0aed9929bf662192aafc435cc58a046183ad1423624c473692deb8a1a4bab): **REQUEST CHANGES**.
- Closures verified: F1, F3, F4, F5, F7, F8 CLOSED; F6 not closed (N3); F2 partial (N2, N4).
- Findings: N1 HIGH (default-all-missing vs fields-unknown contradiction), N2 HIGH (disclosure sizing uses raw 16x64; escaped width/feasibility and R7 byte-identity), N3 MEDIUM (counter label for non-matching docker_proxy path), N4 MEDIUM (terminal metadata cannot carry entry-threaded disclosure), N5 MEDIUM (result_handle_test.go missing from write set), N6 MEDIUM (D-table missing carrier-failure rows; full+error precedence), N7 LOW (projection:{} / full:false undefined), N8 LOW (stale anchors).
- Transport note: two aborted eval-driven launches produced failed run records (no substantive output); the completed review is `ReviewTicket119SpecR2-3` (7m37s). Failed launches did not consume budget.
- Source: `.omp/artifacts/token-eff05/review-cr-r2-source-f3a60598-20261128T033817Z.json` (actual hash path in round record).
- Consumed CodeReviewer reviews: **2 / 15**. Next: planner r3 repair of N1–N8 (no new R/AC/T IDs); then fresh candidate + CodeReviewer round 3.

## Review round 3 (2026-11-28)

- CodeReviewer `ReviewTicket119SpecR3` on candidate `token-efficient-tool-responses-ticket05-spec-r3` (spec sha256 7636a826da9bc79f0123f629e41a67481ef0a42e05113cb5cb857d485727c922): **REQUEST CHANGES**.
- Closed: N1, N3, N4, N5, N6. Regressed: N7→N9 (fields:[] absent vs rejected), F7 reopened. Not closed: N2 (encoded maximal disclosure exceeds preview/terminal headroom), N8 (anchors). New: N10 (aggregate vs per-item skip predicate), N11 (pinned/direct execution path unprojected vs AC1), N12 (terminalResult arity breaks bounded_delivery_test.go outside write set), N13 (projection_reason enum not enumerated).
- Consumed CodeReviewer reviews: **3 / 15**. Next: planner r4 repair of N2/N8/N9/N10/N11/N12/N13 (no new R/AC/T IDs); then fresh candidate + CodeReviewer round 4.

## Review round 4 (2026-11-28)

- CodeReviewer `ReviewTicket119SpecR5` on candidate `token-efficient-tool-responses-ticket05-spec-r5` (spec sha256 8bbb0907933a14a34f4bbcc8ca596696d467ac34604d1dbf8cca53b0fce0e64f): **REQUEST CHANGES**.
- Closed: N9, N10, N12, N13 count, AC verbatim, write-set arithmetic. Residual/reopened: F-A HIGH (direct carrier keeps unprojected `{"result":"<full list>"}` while disclosing applied=true), F-B MEDIUM (D-table compressed away; aggregate reason unbound), F-C MEDIUM (skip disclosure channel contradiction), F-D MEDIUM (config feasibility minimum rises ~730→~1352 with no AC/§8 note), F-E MEDIUM (anchors still stale), F-F/F-G/F-H LOW.
- Consumed CodeReviewer reviews: **4 / 15**. Next: planner r6 repair of F-A…F-H; then fresh candidate + CodeReviewer round 5.

## Review round 5 (2026-11-28)

- CodeReviewer `ReviewTicket119SpecR6` on candidate `token-efficient-tool-responses-ticket05-spec-r6` (spec sha256 0fd10087215d6c849e122f3ddaedb65b0bd318d37a76ad05a50b6fe41bbbb708): **REQUEST CHANGES**.
- Closed: F-B, F-D, F-F, F-G. Partial: F-E (bounds range), F-A/F-C/F-H (R6-1..R6-3). Residuals: R6-1 MEDIUM (disclosure schema vs reason/AC1 shape), R6-2 MEDIUM (multiplexer `upstream_result` whole-member re-derivation + README), R6-3 MEDIUM (thread engine decision out-of-band; forged matching direct disclosure), R6-4..R6-7 LOW (channel wording, citation residue, counter outcome domain, cap documentation/clamp).
- Consumed CodeReviewer reviews: **5 / 15**. Next: planner r7 repair of R6-1..R6-7; then fresh candidate + CodeReviewer round 6.

## Review round 6 (2026-11-28)

- CodeReviewer `ReviewTicket119SpecR7` on candidate `token-efficient-tool-responses-ticket05-spec-r7` (spec sha256 be2c8e287cc257371198e9ec024dc1bbc015a16fa7b29ad95c6e5f54fe1c5d6f): **REQUEST CHANGES**, 0 HIGH / 1 MEDIUM / 3 LOW.
- Closed: R6-1..R6-7 and open question (with R7-1 channel contradiction residual, R7-2/R7-3 citations, R7-4 fields-when-not-applied). Round >=3 is schedule-advisory under W09; Critic is now eligible.
- Consumed CodeReviewer reviews: **6 / 15**. Next: Critic on the exact r7 candidate/inputs; then repair R7-1..R7-4 if Critic REVISE or planner closes them before implementation.

## Review round 7 (2026-11-28)

- CodeReviewer `ReviewTicket119SpecR8` on candidate `token-efficient-tool-responses-ticket05-spec-r8` (spec sha256 dccb027dbf8341eb94d7d45e2128f45ff29df0c552b3aa732606d8ef282555a3): **REQUEST CHANGES**, 0 HIGH / 1 MEDIUM / 2 LOW.
- Closed: R7-1..R7-4 and open (i). Residuals: F1 MEDIUM (forged matching-entry applied:true claim unachievable), F2 LOW (§7 fallback for row 20), F3 LOW (entry value for no-match), plus AC4 byte-identity wording.
- Consumed CodeReviewer reviews: **7 / 15**. Next: planner r9 repair; Critic remains blocked until a positive CR observation.

## Review round 8 (2026-11-28)

- CodeReviewer `ReviewTicket119SpecR9` on candidate `token-efficient-tool-responses-ticket05-spec-r9` (spec sha256 92ec8bc32784a98fb99d1d98ce8943d1f47c13682b6b8befa3b9001789c91d18): **REQUEST CHANGES**, 0 HIGH / 2 MEDIUM / 1 LOW.
- Closed: F1 (rule), F2, F3, AC4 wording. Residuals: R9-1 MEDIUM (direct-path collision lacks precedence over carrier/select/aggregate rows), R9-2 MEDIUM ("byte-identical to upstream" unachievable; use value-identical), R9-3 LOW (non-object SC disclosure target).
- Consumed CodeReviewer reviews: **8 / 15**. Next: planner r10 repair; Critic remains blocked pending a positive CR observation.

## Review round 9 (2026-11-28)

- CodeReviewer `ReviewTicket119SpecR10` on candidate `token-efficient-tool-responses-ticket05-spec-r10` (spec sha256 154e3710131491f9c3736afa13e0f3055df86e11cb37eec389048910e15649ae): **REQUEST CHANGES**, 0 HIGH / 2 MEDIUM / 3 LOW.
- Closed: R9-1, R9-2, R9-3 classification. Residuals: CR-R10-1 MEDIUM (P ordered where no writable carrier; non-object SC + rows 6/15), CR-R10-2 MEDIUM (AC4 direct override asserted on both paths), CR-R10-3+4+5 LOW (T2 wording, override L+C wording, unreachable rows 15/17).
- Consumed CodeReviewer reviews: **9 / 15**. Next: planner r11; Critic remains blocked pending a positive CR observation.

## Review round 10 (2026-11-28)

- CodeReviewer `ReviewTicket119SpecR11` on candidate `token-efficient-tool-responses-ticket05-spec-r11` (spec sha256 a8064baa97a4beeee35a6d549a00ca71c8f53dfbc64b76d02a2e86afa49248a3): **REQUEST CHANGES**, 0 HIGH / 3 MEDIUM / 1 LOW.
- Closed: CR-R10-1..4, skip-mode. Residuals: CR-R11-1 override scope names fields rows, CR-R11-2 SC-absent vs writable-carrier, CR-R11-3 non-object SC vs multiplexer envelope, CR-R11-4 README enumeration; two open questions (direct-error collision, bound-violation reason).
- Consumed CodeReviewer reviews: **10 / 15**. Next: planner r12; Critic remains blocked pending positive CR.

## Review round 11 (2026-11-28)

- CodeReviewer `ReviewTicket119SpecR12` on candidate `token-efficient-tool-responses-ticket05-spec-r12` (spec sha256 e4d4152534c3a1c46754138b3b6766ce5313252f3f4cf6f6cd18617bf6dc40d4): **REQUEST CHANGES**, 0 HIGH / 1 MEDIUM / 0 LOW.
- Closed: CR-R11-1, CR-R11-2, CR-R11-4, opens (i)/(ii). Residual: CR-R12-1 MEDIUM (multiplexer non-object upstream SC: APPLY vs row 6 SKIP contradiction).
- Consumed CodeReviewer reviews: **11 / 15**. Next: planner r13; then CR round 12.

## Review round 12 (2026-11-28)

- CodeReviewer `ReviewTicket119SpecR13` on candidate `token-efficient-tool-responses-ticket05-spec-r13` (spec sha256 24c3299fe058a34aa62535373b2320dec5681bf0e2b0331a8cd309817394c58a): **REQUEST CHANGES**, 0 HIGH / 1 MEDIUM / 0 LOW.
- Closed: CR-R12-1 root invariant, open (i) for fields rows, open (ii). Residual: CR-R13-1 MEDIUM (`fields` pinning contradicts `fields:[]` on default-mode skip/override rows).
- Consumed CodeReviewer reviews: **12 / 15**. Next: planner r14; then CR round 13.

## Review round 13 (2026-11-28)

- CodeReviewer `ReviewTicket119SpecR14` on candidate `token-efficient-tool-responses-ticket05-spec-r14` (spec sha256 cfca2e6b5b316afe48adcb7a679dea2479f2f835ee1d39f543a907bb3e5fc2fe): **REQUEST CHANGES**, 0 HIGH / 1 MEDIUM / 0 LOW.
- Closed: CR-R13-1 at R2/schema/D-table/AC6/T1/§10. Residual: CR-R14-1 MEDIUM (row 15's `fields` flat "entry default list" at r14:6,16,128 vs requested list at :61/:95/:103). Open LOW: static-bounds vs runtime-budget precedence.
- Consumed CodeReviewer reviews: **13 / 15**. Next: planner r15; then CR round 14.

## Review round 14 (2026-11-28)

- CodeReviewer `ReviewTicket119SpecR15` on candidate `token-efficient-tool-responses-ticket05-spec-r15` (spec sha256 09de7e7271dc4b9e5cdc3cd7dcf450580c671c47bb86ae8fc4063b05e9099281): **REQUEST CHANGES**, 0 HIGH / 1 MEDIUM / 0 LOW.
- Closed: CR-R14-1, static-vs-budget precedence. Residual: CR-R15-1 MEDIUM (pre-dispatch order not total: select_conflict vs malformed_request composites; non-matching static bounds).
- Consumed CodeReviewer reviews: **14 / 15**. Final budgeted review: planner r16 then CR round 15. If not APPROVE, stop and report budget exhaustion per S15.

## Review round 15 (2026-11-28) — FINAL

- CodeReviewer `ReviewTicket119SpecR16` on candidate `token-efficient-tool-responses-ticket05-spec-r16` (spec sha256 261372326838b070b6832275f1c25619d8eee4701438a86b7121cb986569e346): **APPROVE**, 0 blocking (4 LOW advisory: L2 shorthand, AC4 L5 precedence wording, "two steps" phrasing, §9 counts/labels; one LOW schema-vs-ladder ownership note).
- CR-R15-1 and all prior closures verified. Write set 3 NEW + 12 MODIFIED = T1∪T2∪T3; exclusions intact.
- Consumed CodeReviewer reviews: **15 / 15** (owner-set budget exhausted; APPROVE achieved on the final unit).
- Next: Critic (no budget unit) on the exact r16 candidate/inputs; then milestone implementation authorization.

## Critic r16 (2026-11-28)

- Critic `CriticTicket119SpecR16-2` on r16 (sha256 261372326838b070b6832275f1c25619d8eee4701438a86b7121cb986569e346): **REVISE**.
- C1 (MAJOR, = CR-R16-1): L2 "`full` + `fields`" contradicts AC3's accepted `{full:false, fields:[...]}`; pin `full:true` + present `fields` at every L2 site and add the false-valued composite to T1/T2.
- C2 (MAJOR): AC5's endpoint `Type` evidence was inference-only while §8 made confirmation optional. **Factual gap closed by live observation**: endpointlist-observed-20261128T093237Z.json (sha256 335bc53f28112105abf4bfaee1f5cb018435077c15041aa2f04997d3bc0278ca) — all three endpoints expose Id/Name/Type (Type=1). Spec must still cite this evidence and make AC5's member-name assertion fail on missing Type without changing partial-miss behavior.
- CodeReviewer budget: 15/15 consumed. No repair/re-review dispatched; owner decision required (S15).

## Round 15 (r19, 2026-11-28)

- CodeReviewer `ReviewTicket119SpecR19`: **APPROVE**, seq 118; 0 blocking (CR-R19-1 MEDIUM advisory at AC3:100 summary; CR-R19-2/3 LOW). Budget now 16/16.
- Critic `CriticTicket119SpecR19`: **REVISE** — same C1, residual only at AC3:100 summary ("`fields`+`full` rejected" reads as member presence, contradicting `{full:false,fields:["Id"]}` apply). C2 closed. Exact amendment supplied: "`fields` with a present `select` is rejected after L1/L2; `full:true` with a present `fields` member is rejected at L2."
- Next: owner budget decision for one wording-only r20 + final CR, then Critic.

## Round 16 (r20, 2026-11-28)

- CodeReviewer `ReviewTicket119SpecR20`: **APPROVE**, seq 119; 0 blocking (CR-R20-1..3 LOW). Budget 17/19.
- Critic `CriticTicket119SpecR20`: **PASS** (no findings); bound as canonical critic observation (invocation 5161c089, source sha 0f15c597…).
- `record-reviews` REFUSED: candidate inputs omitted `internal/observe/log_test.go` and `internal/observe/metrics_test.go`, both required by state T3 write paths. Root cause is the seed builder's file list, now fixed for subsequent revisions. r20 candidate inputs are immutable (no writer to add inputs), so canonical receipts need a r21 candidate built from the corrected list.

## Implementation T1 (2026-11-28)

- Engineer `EngineerT1-3` (after two provider-auth failures and one budget-clarification IRC round) created `internal/service/field_projection.go` (386 lines) and `field_projection_test.go` (271 lines) only.
- Verification run by coordinator: `go test ./internal/service -run 'TestFieldProjection' -count=1` => ok 0.007s; `gofmt -l` on both files => empty.

## Implementation T3 (2026-11-28)

- Owner amendment (recorded via `authorize`): T3 effects + envelope gain `cmd/mcp-bridge/config_envelope_test.go`; envelope count 132→133.
- Engineer path: EngineerT3 stalled at the 42-read threshold, then made partial edits; EngineerT3Repair2 fixed envelope.go and RecordProjection but failed TestProjectionMetricsBoundedLabelsAndOutcomes.
- Read-only debugger `DebugT3Metrics`: root cause was expected label order (Prometheus sorts outcome,server,tool), plus gofmt on log.go/metrics.go. No production defect.
- EngineerT3Fix applied the six expected strings + gofmt. Coordinator verification: `go test ./internal/observe` PASS; `go test ./cmd/mcp-bridge -run Envelope` PASS; gofmt clean. T3 recorded done with AC6/AC7 test evidence.
- Note: the full `go test ./cmd/mcp-bridge` run also failed unrelated `TestLegacyParamHeaderCallFailsWithNamedCode` (parameter-header unsupported vs backend_unavailable); treated as pre-existing/unrelated, not fixed.

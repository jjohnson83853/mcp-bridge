r8-pin-diagnostics-narrow4b spec review (R20–R25, AC08-1..AC12-3, T8–T12). Read-only; hashes not recomputed.
REVIEWER: code-reviewer
PACKET: 8a3bba8d6242fc62e72beb63d9f61b1fb86bf12b2e5b0fd9bf91eaf60ca62c8a
VERDICT: revise

1 MARKERS – pass. Tokens only at spec.md:40/67,169/204,271/287,289/303: 4 unique pairs, correctly ordered, non-nested, own lines.

2 COVERAGE – pass. :267 maps R20–R25; AC08-1..AC12-3 (:240-265) name their ticket and each has exactly one state owner (state:369-375,403-410,440-444,474-476,504-506). R25 clause 1 is asserted only for catalog_refresh_failed (:246).

2b NARROWING CLOSURE – revise. Closed: H1 spec (:64,:260,:261; issues/11:7,12,13); H1 ledger (:285 RED now the config-validation pin document via SIGHUP + --check-config, +config_revision/trigger); H2 (issues/10:17); G3 (:251, issues/09:18); G4 (:63); G5 (:258); M1 spec (:240,:246); M1 ledger/env/ticket (:284 Deps: T9; :292 T9 after T8, T10 after T9; issues/10:4); M2 incl. issues/08:20 mirror; L1 (:284/:285 clauses); L2 (:63); L3 (:273). Deferred and recorded, not grounds: tickets 13–19 (backlog.md; M3 at decisions.md:176/ticket 15; drift :165/issues/17). UNCLOSED: the prior R25 open question, still unrecorded: no AC09/AC10 asserts `level` (:247-259) and T9 (:283) lacks the "no debug-only diagnostics" clause T10/T11 carry, so a debug-level pin_blocker_detail passes every AC. Fix: append "and `level` `warn` (AD18)" to AC09-1 and "(AD17)" to AC10-1 (no new id), else a backlog row.

3 ACs – revise (same root). AC08/AC11 are falsifiable and producer-named; AC09/AC10 cannot fail on R25 level. AC12-3 reuses AC12-1's oracle.

4 RULINGS – revise (minor). Metric freeze (T8:282,T10:284,T11:285), no schema_version (AD20:201,:299), no raw text (R8:48,T9:283), additive-only (AD21:203,T11:285), no induced-failure review gate (T12:286) match decisions.md. But :284/:285/:299 forbid "induced-failure test infrastructure" while T10/T11 REDs and AC10-1/AC11-3 require inducing those failures; ruling 19 bars a review gate, not doubles. Fix: "no induced-failure review gate or shared fault-injection harness; per-test doubles named in the RED permitted".

5 ENVELOPE/STATE – pass. Edges state:355,386-388,421-423,455-458,487-491 = :273/:284/:292 and tickets 08-12 Blocked-by; strictly serial. issues/17 drift is owner-deferred (:165), not grounds (T10 extra paths, missing service_edge_test.go, T8 blockedBy [] vs :282). state:4 and :175/:185 packet match; spec hash binding (:11) not recomputed.

6 DEFECT SITES – pass. refresh.go:195-206 none; :210-241 err unread, fallback runtime.Current() :224; reload.go:562-564 text-only wrap (identity only core.go:311); ops.go:19 Reason omitempty; wiring.go:373-407 publish-only; dedicated.go:113,151,158-171 raw discovery errors, no stage/status; log.go:185-219 no catalog/pin event. Other unaddressed LOW: :199 "17 -> 24" vs 16 names (log.go:185-219; correct only if the default row counts); T9 must replace first-only core.AddPins (:311) within its files; :203 "fallback stream" undefined; AC10-5 needs a hook (refresh.go:211).

### Recommendation
REQUEST CHANGES

VERDICT: REVISER

**Mode:** specification · initial pass · on-disk `spec.md` (reports `r10-replacement`; packet digest `352dbacb…` not independently recomputed — no hashing tool granted). Reviewed against O1–O9/R1–R12, siblings 02/05/06, and the cited seams.

**Closures:** r9 F2 (four-surface producers), F3 (named diagnostic codes), F5 (aggregate eviction order), F7 (expired/evicted precedence), F8 (T5 write roots) are closed. F4 is half-closed (AC1 now parameterizes four surfaces; the boundary remains, F3 below). F6 residual is F4 below. Five of the six carried invariants are present and oracled; the sixth (retired-marker accounting) is F1.

---

**F1 [HIGH] [lifecycle-hazard] Marker installation has no defined aggregate-admission outcome, and OD3's headroom arithmetic cannot bound markers.**
Sections: §2 R6/R7/R16, §8 OD3; AC18/AC38. R6 requires every tombstone *and sweep marker* to be admitted against `max_bytes_aggregate`; R16 requires an expired/evicted handle to return its typed code "while the entry or marker exists". But R6/R7 state an outcome only for entries (terminal) and tombstones (retire without a tombstone); a marker commit still infeasible after clearing expired records and FIFO-evicting the namespace's entries has none — and evicting entries to admit a marker creates another marker (circular). OD3's `(max_sessions + 1) × 256` headroom is bounded by sessions, while live markers accrue per eviction/expiry inside `tombstone_ttl`, i.e. by throughput.
Consequence: either the "HARD" bound is exceeded, or the marker is silently skipped and `result_evicted`/`result_expired` degrades to `invalid_handle`; AC18 passes only because it is qualified "while entry or marker exists", contradicting R16's "regardless of sweep timing".
Fix: state the infeasible-marker outcome, and bound live markers per namespace (e.g. ≤ `max_entries_per_namespace`, folded into a load-validated relation), replacing OD3's session-based headroom formula.

**F2 [MEDIUM] [task-grant] The six new diagnostic codes break the documented closed registry, which no task owns.**
Sections: §2 R11, §5 T1. `DiagnosticCodes()` is single-sourced with `docs/logging-contract.md` (`<!-- config-codes:start -->`, :395) by `cmd/mcp-bridge/logging_contract_test.go:116-121`. T1 adds `ceiling_order`, `entry_cap_order`, `namespace_cap_order`, `ttl_invalid`, `aggregate_order`, `envelope_feasibility` to `internal/config/diagnostics.go`, but the doc is only a T2 write root, and no task names its validation command.
Consequence: `go test ./...` is red at T1, so T1's "must exit 0" gate is unsatisfiable; alternatively the registry silently diverges.
Fix: add the config-codes block of `docs/logging-contract.md` to T1's write roots and name each task's validation command set.

**F3 [MEDIUM] [ac-not-falsifiable] The ceiling+1 delivery decision has no oracle.**
Sections: §2 R1/R3; AC1, AC2, AC3; refinement R2 ("8192 passes, 8193 pages"). AC1 fixes at/below-ceiling pass-through; AC2's above-ceiling fixture only measures bytes and runs at T1 (no seam); no AC requires a ceiling+1 result to page.
Consequence: an off-by-one that passes through at exactly ceiling+1 satisfies every AC while violating ticket AC 1.
Fix: add a ceiling+1 golden fixture to AC1/AC3 (or a dedicated AC) asserting preview+handle.

**F4 [MEDIUM] [statement-contradiction] The `operation` domain claim contradicts AC13.**
Sections: §2 R12, §7 Finite labels vs AC13. R12 asserts "operation takes exactly the values {call_tool, search_tools, describe_tool, retrieve_result}" and §7 repeats it; AC13 requires the closed vocabulary to retain all existing values (the code has 18, including `search` and `describe`) plus the two new ones.
Consequence: pruning to four breaks existing producers and AC13; both texts cannot be satisfied.
Fix: scope the four-value claim to paging records and state the vocabulary retains every existing value.

**F5 [LOW] [ac-not-falsifiable] Paging events and their fields have no producer oracle; §7's cited enforcement is mislocated.**
Sections: §7 Events/Finite labels; AC13/AC14. §7 says unlisted values fail "the envelope vocabulary test (internal/observe union fixtures)", but `internal/observe/log_test.go:46` only compares two static lists; record-level field checking lives in `cmd/mcp-bridge/logging_contract_test.go` (a non-write-root fixture), and no AC asserts the six paging event names or their `total_bytes`/`page`/`total_pages` fields. AC3/AC6 assert payloads, not logs.
Consequence: events can ship with wrong names or fields with every AC green.
Fix: extend AC13's producer test to assert each event's exact field set, or remove the enforcement claim.

---

**Positive observations.** Metadata sets are single-sourced and referenced (R3/R5/R10), avoiding the predecessor's per-surface repetition. AC2/AC6 use a genuinely independent golden literal ("not a retained artifact, not re-marshaled"). R11 relations (a)–(h) now carry explicit codes, and R13's shape→resolution→page precedence is grounded in `requestExposureEpoch`'s actual behaviour.

**Review limits.** Specification mode only; no implementation exists. No shell/hash tool, so candidate-digest drift cannot be checked. Store-internal requirements (tombstone/marker presence) are only observable through the store-level accounting tests, which is acceptable but not black-box behaviour.

### Recommendation
REQUEST CHANGES
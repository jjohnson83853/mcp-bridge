VERDICT: REQUEST CHANGES

**F1 [HIGH] — §5 T1 write roots omit `internal/config/diagnostics.go`, so R11's typed diagnostics cannot exist.**
Section: §5 T1 (write roots), §6, R11.
`internal/config/diagnostics.go:37-103` is a closed `diagnosticCodes` array; `parser_edges_test.go:25-29` fails any produced code absent from `DiagnosticCodes()`. R11 introduces at least `ceiling_order`, `envelope_feasibility` and `aggregate_order`, all of which must be registered there, yet T1 may write only `config.go`, `bounds.go`, `validate_edge_test.go` and the new measure files. Consequence: every T1 config RED test fails on an unregistered code, so T1 cannot reach its declared GREEN. This is the residual of replacement-r5 F1: `config.go` was added, `diagnostics.go` was not.
Fix: add `internal/config/diagnostics.go` to T1's write roots and name the three code registrations. Also state in §6 that `retained_results` is a root-level (process-global) section — currently only T1's "appears in the loaded Config struct" implies it, while `Config` (`config.go:55-66`) and `fields["root"]` (`config.go:648-666`) make the placement decisive.

**F2 [MEDIUM] — §5 T2 write roots omit `internal/observe/metrics_test.go`, yet T2 owns `TestPagingMetricsRegistered`.**
Section: §5 T2 (write roots), AC14.
The metric labels and registration live in `internal/observe/metrics.go`, whose existing tests are `metrics_test.go`/`edge_test.go` (package `observe`, using `NewMetrics`/`exposition`). T2's RED list names `TestPagingMetricsRegistered` but its roots include `log_test.go` only, not `metrics_test.go`; the named test has no legal containing file.
Fix: add `internal/observe/metrics_test.go` to T2's write roots (or rename the RED test to one that belongs in the listed `log_test.go`).

**F3 [MEDIUM] — R6's cross-namespace eviction isolation has no acceptance oracle.**
Section: R6, AC7/AC15.
R6 states "Eviction never crosses namespaces: a namespace's overflow evicts only that namespace's entries", and refinement R9 requires that one session cannot evict another's live handles. AC7 asserts only within-namespace FIFO; AC15 asserts only that aggregate refusal leaves other namespaces untouched. Nothing fills namespace A to its per-namespace bound, forces its FIFO eviction, and asserts namespace B's entries and handles remain retrievable.
Fix: extend AC7 (or add an AC) with that two-namespace invariant, citing R6 and refinement R9.

**F4 [MEDIUM] — R10/§3 do not exactly single-source the terminal `StructuredContent` set, so R11(g)'s feasibility worst case is undefined.**
Section: §3 Terminals, R10, R11(g), AC26.
§3 says a terminal's `StructuredContent` "adds `retention` … and `execution_state`" without naming the base set, while R11(g) enumerates terminal maximal metadata as `retention`, `execution_state`, `total_bytes`, `is_error`, `upstream_code`. It is therefore undefined whether `paging_required` (present in R3's preview set) also appears on terminals. AC26 asserts only `retention`, `execution_state`, `total_bytes`. Consequence: a load-valid maximal-metadata config can render a terminal larger than R11(g) computed, breaking R10's preview bound and the terminal feasibility diagnostic.
Fix: state the terminal metadata set exactly and single-source it as R3/R5 do; make R11(g) use that set; have AC26 assert the exact set (absent fields included).

**F5 [LOW] — R2 still overclaims the measured form (replacement-r5 F7 residual).**
Section: R2.
R2 measures the SDK-marshaled `mcp.CallToolResult` but calls it "exactly the bytes written to the client for that result, so the measure cannot exclude any emitted bytes." The JSON-RPC response framing is emitted and not counted; intake and refinement R1 ask for the exact serialized `tools/call` response. Boundedness survives because the excess is constant, but the documented ceiling is not a strict wire bound and the exclusion is never stated.
Fix: state explicitly that the JSON-RPC envelope is excluded and why (constant, bounded), or include it.

**F6 [LOW] — AC19's Covers column misstates its scope (replacement-r5 F6 residual).**
Section: AC19.
AC19 exercises cross-surface uniformity and per-surface `upstream_code` producers but lists only `R3`. Covers should name R3, R8 and R11.

**F7 [LOW] — §1 STATUS points at stale ledgers.**
Section: §1.
It cites `findings-replacement-r2.md` and `-r3.md`; the highest present is `findings-replacement-r5.md`, so r4/r5 closure verification has no citable pointer. Fix: reference the current ledger (or list all).

**Positive observations.** All six carried invariants are present and testable: independent byte oracle (R2/AC2/AC6), tombstone and marker lifetime plus 256 B accounting (R7/R16/AC5/AC8/AC18), pinned `upstream_code`/ceilings (R8/R9/AC9/AC10/AC29), finite labels with producers (R12/§7/AC13), per-terminal preview bounds (R10/AC11), ceiling-order validation (R11(a)/AC12/AC28). Preview and page metadata are deliberately single-sourced, R13's handle-then-binding-then-page precedence closes an existence leak, and R16 makes expiry/eviction deterministic without depending on sweep scheduling.

REQUEST CHANGES
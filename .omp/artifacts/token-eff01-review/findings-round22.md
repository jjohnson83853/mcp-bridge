VERDICT: PASS
Specification mode; pass: repair (r22). I reviewed the on-disk bytes (the file declares v22; the highest prior ledger is `findings-round11.md`). Prior closures re-verified and not regressed: C1 CSPRNG+receipt, C2 retirement-versus-retention race, C3 envelope/fixture/doc co-ownership plus real-sink AC18, C4 `coverage.out`, C5 per-surface rejection matrix.

## Findings (all LOW, advisory)
**L1 [R9; §4.5, §11-OD2] — false exposure arithmetic.** With `max_bytes_per_session=16,777,216`, `(max_sessions+1)×` it cannot equal `68,719,493,120` (4,096 namespaces give 68,719,476,736; 4,097 give 68,736,253,952); the stated value is 64 GiB+16 KiB and also omits the 1 MiB/namespace tombstone allowance. Consequence: the documented worst-case memory figure is wrong. State the exact product and whether tombstones are inside the per-namespace/aggregate bound.

**L2 [R1; §2 R1, §4.2, AC13/T4] — terminal-preview bound not AC-pinned.** R1 requires each unretainable terminal preview to be ≤ the preview ceiling with a non-empty UTF-8 prefix and preserved `is_error`/`upstream_code`, but AC13's capacity/namespace-retired oracle asserts only the `retention` value and absence of a handle; AC1/AC10 are scoped to the retainable branch. Consequence: an over-ceiling or empty terminal preview passes the named AC (only T4 blocked-prose catches it). State the terminal-preview bound as an AC.

**L3 [R12; AC18, §8] — metrics increment unverified.** AC18 asserts collector registration/scrape with bounded labels, never an increment, while §8 claims every deliverable is AC-falsifiable. Consequence: a registered-but-never-incremented collector passes; paging/eviction volume stays unobservable. Add a post-operation delta oracle or narrow the claim.

**L4 [R12; §3 "Verified facts", T2/AC18] — injection evidence stated as fact.** §3 says `observe.Metrics` already reaches the surface via the `ToolSurfaceConfig` literal at `cmd/mcp-bridge/serve.go:164`; `ToolSurfaceConfig` (tools.go:29-40) has no `Metrics` field and metrics is built at serve.go:132. Consequence: collector delivery reads as proven; only T2's file list carries the work. State it as the injection point T2 adds.

**L5 [R6; T6, §7] — README claim anchors drift.** T6's "complete" claim list cites `:288`, `:337`, `:470`; the claims sit at 287, 332, 468. Consequence: line-based edits miss; quoted negative greps still bound the text.

**OQ (LOW confidence) [R7; §4.3] —** §4.3 lists "tampered" under `invalid_handle` while R7 routes shape-invalid handles to `invalid_arguments`. State that the split is input-derived and store-independent, or the two-phase contract reads inconsistently.

## Verified adequate
1. **Byte accounting:** §4.4 measures the complete `CallToolResult` via SDK `MarshalJSON` (matches SDK `json.Marshal` HTML escaping), copies counted per occurrence (`call_tool` content + `upstream_result`; search/describe content + structured), `total_bytes=len(S)`; AC1/AC2/R2 use that one form. Wrapper exclusion is explicit.
2. **Termination:** pages bounded by the pinned ceiling and exempt from the reloaded ceiling; `envelope+6<ceiling` guarantees per-page progress; `retrieve_result` never paged (§4.5/§7). No nesting path.
3. **Handles:** `crypto/rand` mandatory with receipt evidence; keyed `{sessionID,epoch}`; monotonic `nextEpoch`; one `invalid_handle` shape with zero page bytes; restart empties the store, so a reused session ID cannot resurrect handles.
4. **Storage:** hard per-namespace bytes/entries, FIFO, own-only eviction, advisory aggregate, bounded tombstones, retirement-vs-retention under the admission lock, complete-page-or-typed-error.
5. **Errors:** `isError`/`is_error`/`upstream_code` on preview and pages; full error byte-exact in S; `errorResult` never paged.
6. **Compatibility:** own `additionalProperties:false` schema (43-char base64url handle, page ≥1), description tokens, no `full` knob.
7. **Regression:** discovery seam after `commitDiscovery`, §4.6 invariants, T5 blocked on a pre-commit seam, 11-file surface inventory.
8. **Ledger:** T1–T7 acyclic; each has RED, files, AC IDs and exact commands; every AC1–AC18 oracle is owned; no sibling-ticket or unstated capability.
9. **Docs:** changed (`:279`, `:286-296`, `:322-325`, `:443`, `:467-470`) and preserved claims are named; T6 governs them.

**Limits:** read-only, no shell/test execution, no digest or size computation; `.omp/artifacts/token-eff01-review/{context,manifest}` still bind r1 (44,287 B), so I reviewed the declared-r22 bytes rather than a pinned hash. Worktree unmodified.

## Recommendation
APPROVE
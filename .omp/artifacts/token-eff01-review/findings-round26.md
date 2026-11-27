VERDICT: REVISER

Spec review, r26, repair pass (prior ledger `findings-round25.md`). Reviewed on-disk bytes; the packet pins older digests too, so this review is not hash-bound. Read-only: no shell/digest/test execution.

**Closures re-verified:** r25 F1–F6 and LOW‑1/LOW‑2 closed (envelope derived from the configured `max_upstream_code_bytes` + AC20; T6 per-claim greps; R2 scoped to owner-resolving results; entropy terminal leaves zero entries; shape-invalid vs forged split; README anchors; inbound strip/stamp).

**Verified adequate:** (1) byte accounting — one measured form (§4.4 complete `CallToolResult` via SDK `MarshalJSON`, wrapper excluded), `total_bytes=len(S)`, duplicates counted per occurrence, AC1–AC3 are checks of that form, and `preview<delivery` is load-validated (AC19) so a preview never itself pages; (2) termination — pages ≤ pinned ceiling and exempt from a reloaded ceiling, `retrieve_result` never paged; (3) handles — 32-byte `crypto/rand`, `{sessionID,epoch}` owner, monotonic `nextEpoch`, inbound strip/stamp (`inbound.go:481,596`), one zero-byte `invalid_handle`, reused-ID new epoch; (4) storage — hard per-namespace bytes/entries, advisory aggregate, FIFO, no cross-namespace eviction, one admission lock, immutable reads; (5) errors — `is_error`/`upstream_code` on preview and every page, terminals preserve class, dispatch failures small and never paged; (6) compatibility — new `additionalProperties:false` schema in `catalog/schema.go`, README-documented, no `full` (ticket 05 opt-in); (7) regression — 11-file built-in-surface inventory with counts, seam after `commitDiscovery`, catalog-revision/epoch untouched; (8) ledger — T1–T7 deps coherent, each with RED, validation and AC IDs; (9) docs — all seven anchors exist (`README.md:279,287,322-325,332,443,467,468`).

**Findings**

**F1 [MEDIUM][statement-contradiction] R2, §4.0, §4.3; T4/T5/T6.** `invalid_epoch` is unobservable on exposure-gated surfaces: the receiving middleware rejects an invalid epoch with JSON-RPC −32600 before the handler (`session_exposure.go:26,53-55,126-133`), and only `call_tool` bypasses admission. Consequence: search_tools/describe_tool/direct callers get a protocol error, not the matrix's `invalid_epoch`/`unknown_tool`; AC13's oracle covers only call_tool/direct, and T6 would document a false contract. State the rejecting layer and observable shape per surface, each with an oracle.

**F2 [MEDIUM][ac-not-falsifiable] R1, §4.2, §4.4; AC13, T4/T5.** Terminal previews are only called "bounded"; AC13 pins ≤ preview ceiling for `result_capacity_exceeded` alone, and the envelope budget omits the terminal-only `retention`/`execution_state`. Consequence: an unretainable over-ceiling result can ship a preview above 1536 bytes or an empty prefix. State every preview shape ≤ `preview_ceiling_bytes` by `measureResult`, with the envelope derived from all shapes and asserted per terminal.

**F3 [MEDIUM][unstated-invariant] R3, R4, §4.5; AC3/AC4, T1–T3.** Only the ceiling is pinned per entry; `max_page_envelope_bytes` follows the reloadable `max_upstream_code_bytes`, and load-time feasibility checks the current delivery ceiling. Consequence: a reload raising the code bound can make an old handle's page unfittable within its pinned ceiling → over-ceiling or empty page. Pin envelope bounds with the entry, or reject reloads that make a retained partition infeasible.

**F4 [LOW][ac-not-falsifiable] R12, §8; AC8, AC13, T2–T3.** §8 maps `result_expired`/`result_evicted` to AC8 and namespace-retired/entropy to AC13, but neither asserts emission; AC18's real-sink test names only two events. Four log deliverables are unverified.

**F5 [LOW][ledger-drift] R9, §4.5, §7; T2.** Purge is assigned "via `cleanupRetiredOwner`" (`projection.go:877`), which §7 lists read-only (inbound.go allows "purge hook" only). Name the authorized hook or add projection.go to the write roots.

**F6 [LOW][statement-contradiction] §4.5, §4.2; T1/T4.** `C_envelope = 320` decomposes into server/tool_name/request_id — fields in neither preview nor page metadata — and no AC uses it; two competing envelope budgets risk under-sizing metadata.

## Recommendation
REQUEST CHANGES
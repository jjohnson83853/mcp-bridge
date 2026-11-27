VERDICT: REVISER

Mode: specification; pass: repair (declared r25; prior ledger `findings-round24.md`). Reviewed on-disk r25; the packet's pinned candidate (v1, `4eed48…`) is older, so this review is not hash-bound. Files: candidate spec, review packet, intake, refinement, map/siblings 02/05/06, `.omp/RULES.md`; source anchors in `internal/service/{tools,session_exposure,inbound,direct_execution}.go`, `internal/catalog/schema.go`, `internal/config/bounds.go`, `README.md`.

**Closures re-verified:** r24 F1 (ceiling-order invariant + AC19 + T1), F2 (T4/T5 terminal-preview oracles in AC13), F3 (entropy terminal shape/`execution_state:"unknown"`). **Prior closures preserved:** F1/F4/F5 (r24 numbering).

**Verified adequate:** (1) Byte accounting — one measured form (complete `CallToolResult` via SDK `MarshalJSON`, wrapper excluded), duplicates counted per copy, `total_bytes=len(S)`, and AC1/AC2/AC3 are checks of that same form. (2) Termination — `preview < delivery` is load-validated, pages ≤ pinned ceiling, `retrieve_result` never paged, pages exempt from a reloaded ceiling. (3) Handles — 32-byte `crypto/rand`, owner `(sessionID,epoch)` binding, monotonic `nextEpoch`, one zero-byte `invalid_handle` across unknown/tampered/foreign/other-incarnation (inbound strips client epoch headers at `inbound.go:469` and stamps the admitted epoch, so incarnations are server-issued). (4) Storage — hard per-namespace bytes/entries, advisory aggregate, FIFO own-only eviction, isolation, one-lock atomic admission, immutable reads. (5) Errors — class preserved on preview, pages and all terminals; `errorResult` never paged. (6) Compatibility — new `retrieve_result` schema with `additionalProperties:false` plus handler-side two-phase validation; no `full` collision with ticket 05. (7) Ledger — acyclic, every AC has an owning task, write roots cover every named file, no sibling dependency.

**F1 [MEDIUM][statement-contradiction] R1/R5/R12; §4.2, §4.4, §9, T1.** The preview/page envelope budget bounds `upstream_code` at a fixed 128 (also T1's default) while `max_upstream_code_bytes` is configurable to 512; a legal configuration therefore passes the load-time feasibility check computed at 128 while its real envelope (up to 512-byte code) may leave no room for the required non-empty prefix.
Fix: state that the field bound equals the configured value and derive `max_preview_envelope_bytes`/`max_page_envelope_bytes` from it (or cap the key at 128); cover the rejection with an AC.

**F2 [MEDIUM][ac-not-falsifiable] R6/T6; §6 T6, §7 README claims.** T6's negative greps cover only three claims; the completion claims at `README.md:322-325`, `:467`, `:468` ("complete upstream result", "unaltered…", "complete original `CallToolResult`") have no per-claim oracle, so adding one "at or below the delivery ceiling" sentence passes every check while those falsehoods survive.
Fix: bind each changed claim to its qualification (per-claim negative, or same-line positive, assertion).

**F3 [LOW][statement-contradiction] R2; §2 R2, §4.0.** R2 promises byte-identical delivery for any result ≤ ceiling, but §4.0's new `call_tool` `ownerOK=false` rejection returns `invalid_epoch` even for a small result.
Fix: scope R2 to results whose owner resolves, or name the rejection in R2.

**F4 [LOW][unstated-invariant] R1/R7; §4.5, AC13, T2.** Entropy failure happens at handle generation and nothing requires the terminal to leave zero retained entries, so an unreachable entry may be committed and later evict live handles.
Fix: state zero retained entries for `result_entropy_unavailable`.

**F5 [LOW][ac-gap] R1/T1; §4.4, §9, AC19.** The feasibility-invariant load rejection (`max_*_envelope_bytes + 6 < ceiling`) has no AC or oracle; T1 lists only AC5/AC19. Fix: give it an AC.

**F6 [LOW][ambiguity] R7; §4.3.** "tampered" is listed in the `invalid_handle` scope while R7 routes malformed handles to `invalid_arguments`, and AC7/AC15 do not define the boundary. Fix: define well-formed-forged → `invalid_handle`, shape-invalid → `invalid_arguments`.

Carried, not re-enumerated: r24 LOW‑1 (README anchors `:288`→287, `:338`→332, `:470`→468 still stale) and LOW‑2 (inbound strip/stamp invariant unstated).

Limits: read-only; no shell, test, or digest execution.

## Recommendation
REQUEST CHANGES
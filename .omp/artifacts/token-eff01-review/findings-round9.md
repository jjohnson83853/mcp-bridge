# Ticket 01 spec — review round 9 findings ledger

Candidate revision r9 (`ticket01-bounded-results-v9`), verdict REQUEST CHANGES (bound).
Observation: `.omp/artifacts/token-eff01-review/observation-code-reviewer-r9-3c3f9af42a04.json`.
Review budget (owner allocation 6): **4 of 6 used**, 2 remaining.

## Verified adequate (do not regress)

`measureResult` = SDK marshal of the whole `CallToolResult`, duplicates per occurrence, one form for `total_bytes`, delivery, preview and page fill; termination with no nesting; 32-byte crypto handles with owner+epoch binding, one `invalid_handle`, malformed handled pre-store; hard per-namespace bounds, FIFO, owner-only eviction, atomic create, serialized reads, tombstone taxonomy; `isError`/`upstream_code` on preview and pages; `errorResult` never paged; `additionalProperties:false`; seam after `commitDiscovery`; complete built-in inventory; T1–T7 ledger.

## Open findings

| ID | Class | Sev | Anchor | Defect |
| --- | --- | --- | --- | --- |
| H1 | statement-contradiction | HIGH | §4.5, §4.0, §9, §11, R1/R9 | `maximal_entry_bytes = 3 × max_response_bytes + escape_reserve + fixed_envelope` never quantifies `escape_reserve` or `fixed_envelope`, while §3 admits a transport-capped result serializes larger after duplication and escaping (up to 6× per byte). With a correct reserve the derived entry (~96 MiB) exceeds the adopted 64 MiB aggregate; with an undercount it exceeds the hard per-namespace bound and the three-case terminal set has no "namespace capacity" outcome. |
| M1 | statement-contradiction | MEDIUM | §4.5, §8, R9 | The soft aggregate cap is untied to namespace count: live namespaces are bounded only by `http.max_sessions` (1–4096), each hard-bounded per namespace, so worst-case retained bytes is namespaces × bound, far above the documented 64 MiB. Overshoot is acknowledged only as a counter. |
| M2 | ac-not-falsifiable | MEDIUM | AC6, T6, §6 | T6 never falsifies the three changed completion claims (`README.md:322-325`, `:467`, `:470`); one positive grep can occur anywhere while README still asserts the unqualified completeness claim. |
| L1 | r-ac-coverage-gap | LOW | R7, AC7, T3 | No oracle covers a reused session ID with a new epoch; monotonic non-reuse of `(sessionID, epoch)` is assumed, not stated. |
| L2 | ledger-state-drift | LOW | AC8, T2 | T2 owns `TestTombstoneOverflow` for AC8 but its AC-ID list omits AC8. |
| L3 | statement-contradiction | LOW | R10, §4.2 | `upstream_code` is defined only for a string `code`; numeric or absent codes and multibyte truncation are unassertable. |

## Open question raised (answer in the repair)

AC3/AC4/AC5 do not require an escape-dense page filled to the pinned ceiling, so a raw-length maximal fill that re-escapes above the ceiling could still pass. Name that oracle if intended.

## Direction for the repair (coordinator's read of H1/M1)

The arithmetic only closes if retention stores the **compact** upstream result bytes and pages are produced by deterministic re-serialization of that same value, rather than storing the escape-expanded serialized text. Then the per-entry bound is the transport payload bound, `max_bytes_per_session` can be ≥ that, and the aggregate default is meaningful. Byte accounting for the ceiling still measures the serialized form. The planner must confirm or reject this reading with evidence, and state derivations either way.

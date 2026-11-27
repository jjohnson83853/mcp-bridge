VERDICT: REVISER

Specification mode, repair pass (on-disk STATUS v30). Prior ledger: `findings-round29.md`. Limits: the packet sha256 cannot be recomputed with granted tools; no r28 ledger exists.

**F1 [MEDIUM][ac-not-falsifiable] R1/R12 — §5 AC1/AC2/AC5, §3 AUTO-DECISION 1, T1/T4.** The independent duplicate-bearing measurement oracle exists only in prose: no AC and no task owns a check comparing delivered/measured bytes to a length computed without `measureResult`, while AC1/AC2/AC5 still use `measureResult` as their sole oracle, and T1 cannot observe delivery (T4 owns delivery but has no such test). Consequence: a deduplicating `measureResult` still passes the acceptance set and can ship a ~2× over-ceiling preview; r29-F1 is only partly closed. Fix: name the duplicate-bearing fixture (expected length computed independently) in an AC and in T4's delivery check, asserting actual serialized bytes.

**F2 [MEDIUM][unstated-invariant] R9/AC9 — §4.5 purge/bounds, AC9.** AC9 calls the retired-namespace marker "store-API-only with a stated lifetime — the race window", but the controlled race requires it to outlive the purge (resume-after-purge must still return the retired terminal), and no clearing rule, count/byte bound, or accounting exists; the aggregate-exposure formula counts live namespaces only. Consequence: one marker per retired `(sessionID,epoch)` accumulates for process life, contradicting "no retained-state growth" and understating worst-case store growth. Fix: state the clearing condition and bound it, or include it in per-namespace/aggregate accounting.

**F3 [LOW][ac-not-falsifiable] R6 — T6.** The `availability` full-vs-paged qualification still has only a negative grep, so deleting the sentence passes; r29-F6 is unrepaired. Add the positive qualified-sentence assertion.

**F4 [LOW][statement-contradiction] R8/R9 — §4.5 "Failure modes" vs §4.3/AC9.** The failure list still presents `result_namespace_retired` as a retrieval failure returned as a tool result, while §4.3/AC9 map a retired handle at the wire to `invalid_handle` and make the marker store-only. Consequence: two admissible wire behaviours.

**F5 [LOW][statement-contradiction] R12/AC18 — §8 Prometheus.** `pages_served_total` enumerates `{outcome: failure}`, but AC18 says rejections change neither counter and the help text says increments only after a page commits; the `failure` label has no producer.

**F6 [LOW][ledger-state-drift] §1/§10.** The closure table ends at r27-F5 while STATUS claims v30 supersedes v29 and the decision log records r29 F1–F4; r28/r29 closures are unenumerated.

**Verified adequate:** measured form (SDK `MarshalJSON`, duplication per copy, same measure for 8192 and preview, AC19/AC20); termination (preview<delivery, pages≤pinned ceiling, `retrieve_result` never paged, `C_envelope+6`<ceiling); handles (32-byte `crypto/rand`, 43-char base64url, `{sessionID,epoch}`, monotonic `nextEpoch`, inbound strip/stamp, rehydration new epoch, identical `invalid_handle` AC7/AC14/AC17); storage (hard per-namespace bounds, advisory aggregate, FIFO, no cross-namespace eviction, one admission lock, complete-page-or-error); errors (`isError`/`upstream_code` on preview, pages and terminals; bridge failures small and never paged); compatibility (new `additionalProperties:false` handle/page schema, description tokens, README contract, no `full`); regression (seam after `commitDiscovery`, 11-assertion inventory, AC11/T5); ledger T1–T7 acyclic with named oracles; README changed/preserved claims anchored. Prior closures F1/F2/F8/F9/F10/F12/F14/F15 remain present.

## Recommendation
REQUEST CHANGES
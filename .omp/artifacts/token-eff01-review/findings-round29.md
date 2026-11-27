VERDICT: REVISER

Spec review, dispatch r29 (on-disk STATUS declares v28), initial pass. Read-only; no shell, digest, or test execution; worktree unmodified.

**Verified adequate:** (1) one measured form — complete `CallToolResult` via SDK `MarshalJSON`, wrapper excluded, `total_bytes=len(S)`, preview and page both measured by it, `preview<delivery` load-validated (AC19/AC20); (2) termination — preview < delivery, pages ≤ pinned ceiling, `retrieve_result` never paged, no recursive paging, progress guaranteed by `C_envelope+6`; (3) handles — 32-byte `crypto/rand`, 43-char base64url, owner `{sessionID,epoch}`, monotonic `nextEpoch`, inbound strip/stamp (verified `inbound.go:481-620`), one zero-byte `invalid_handle` for unknown/tampered/foreign/reused-ID/retired-epoch handle (§4.3, AC7/AC14/AC17); (4) storage — hard per-namespace bytes/entries, advisory aggregate, FIFO, no cross-namespace eviction, one admission lock for create/admit/evict/liveness, immutable reads (§4.5, AC9); (5) errors — `is_error`/`upstream_code` on preview and every page, dispatch failures fixed small shape never paged (R10; `errorResult` shape confirmed); (6) compatibility — `additionalProperties:false` `{handle,page}`, no `full` (R6/§4.3/T3); (7) exposure/epoch/catalog admission unchanged (`requiresExposure`/`admitExposureRequest`), seam after `commitDiscovery` (R11); (8) ledger — T1→T2→T3→T4→T5→T6/T7 acyclic, every AC owned, no sibling-ticket dependency; (9) README anchors `:279/:287/:322-325/:332/:443/:467/:468` resolve.

**Findings**

**F1 [MEDIUM][ac-not-falsifiable] R1; §4.4; AC1/AC2/AC5/T1.** "Each distinct serialized form per occurrence; duplicated forms counted per copy" is ambiguous (distinct-form-once vs per-copy), and every ceiling AC uses `measureResult` itself as its oracle, so a deduplicating implementation is self-consistent; §10 itself lists the live-wire boundary test as missing. Consequence: a result whose text block and `structuredContent` carry identical bytes can be measured at ~half its delivered size and ship over the 8192 bound while passing AC1/AC2/AC5. State the invariant (measurement is the length of the SDK-marshaled complete result; every occurrence contributes its own bytes) and require one independent serialized-bytes oracle on a duplicate-bearing fixture.

**F2 [MEDIUM][statement-contradiction] R8/R9; §4.3 vs §4.5/AC9; T2/T3.** §4.5 lists `result_namespace_retired` as a wire-observable retrieval failure and AC9 asserts "pre-retirement handle → `result_namespace_retired`", but §4.3/§4.0 reject the retired-epoch caller at admission and map a live-epoch caller presenting a retired-epoch handle to `invalid_handle`; after purge, no retained marker is defined. Consequence: no admitted caller can observe that code on retrieval, so AC9 is unsatisfiable wire-side. State whether the retired-namespace marker is store-API-only (and its lifetime) or drop it from retrieval failure modes.

**F3 [MEDIUM][unstated-invariant] R4; §4.2/§4.5; AC4.** Only `pinned_delivery_ceiling_bytes` and `C_envelope` are pinned per entry; the page's `upstream_code` rendering derives from the reloadable `max_upstream_code_bytes`, so a reload can change re-retrieved page bytes. Consequence: R4's "identical bytes until expiry or eviction" fails for the same handle/page. State that the per-entry code bound/value is pinned with the entry, and assert byte-identity across that reload.

**F4 [LOW][ac-not-falsifiable] R12; §8/AC18; T2–T5.** `{operation}` and `{outcome}` value domains are never enumerated, so "bounded labels" is uncheckable and a session- or handle-valued label could pass registration tests. State the finite label vocabularies.

**F5 [LOW][ledger-state-drift] STATUS/§10; whole candidate.** STATUS declares `v28` while the dispatch binds r29, and the closure table records only `r27-F5` although STATUS claims closing r27 F1–F5. Consequence: the four claimed closures are unevidenced. Align the declared revision and enumerate each closure.

**F6 [LOW][ac-not-falsifiable] R6; §6 T6.** The `availability` full-versus-paged qualification gets only negative greps while every other changed claim gets a positive sentence assertion, so deleting the sentence passes T6. Add the positive qualified assertion.

**Review limits:** highest on-disk ledger is `findings-round26.md`; r27/r28 ledgers are absent, so r27 F1–F4 closures cannot be verified. The packet's r29 sha256 cannot be recomputed with the granted tools, so on-disk bytes are unbound to the declared revision.

## Recommendation
REQUEST CHANGES
Verdict: REQUEST CHANGES

Specification mode, initial pass on the on-disk `r15-replacement` bytes (§1). No implementation exists; no tests or diagnostics were run. I did not recompute the packet digest (no hashing tool in this profile), so the review is of the file bytes read.

**HIGH — T3's write-root list omits a file that pins the built-in count.**
Section: §5 T3. `cmd/mcp-bridge/serve_hybrid_reload_test.go:226` asserts exactly 4 advertised tools ("3 meta-tools + alpha__read"), and its later waits require `len(names) == 5` then `== 4`. Registering `retrieve_result` shifts all three counts, so T3's stated GREEN ("suite green incl. the updated inventory assertions") is unreachable inside the declared roots; the implementer must either fail the task or edit an unowned file. Fix: add `cmd/mcp-bridge/serve_hybrid_reload_test.go` to T3's write roots with the +1 shift, or make those counts derive from the registered built-ins.

**MEDIUM — retirement and eviction markers prescribe two codes for one state.**
Sections: R7, R16, §3 Retrieval failures. R7 requires any handle retrieval from a retired namespace to return byte-identical `invalid_handle`, with or without a tombstone. R16 requires a handle whose entry or marker is expired/evicted to return `result_expired`/`result_evicted` "regardless of sweep timing". A namespace can retire (session close or epoch bump) while an eviction marker is still live (`tombstone_ttl`, 15m), so one request satisfies both rules; AC18 and AC38 pin opposite outcomes and no AC covers the overlap. Fix: state that retirement clears the namespace's markers and that a tombstone forces `invalid_handle` regardless of marker state; add one AC for retired-namespace-with-live-marker.

**MEDIUM — R11(g)'s "maximal metadata" has no pinned integer widths, so AC23's negation is not reproducible.**
Sections: R11(g), AC23/AC28, §6. The feasibility inequality compares the "maximal-metadata" preview, page and terminal envelope, but the widths of `total_bytes`, `total_pages`, `remaining_pages` and `remaining_bytes` are undefined, and no relation caps `max_retained_result_bytes`. Two implementations can therefore load different configs from the same file, and AC23's "exactly the negation" oracle has to guess the digit widths. Fix: state the maximum used for each numeric field (its configured cap, or a fixed 20-digit encoding) in R11(g), and build AC23's envelope from that rule.

**MEDIUM — R12's closed-enumeration claim is false of the tree, and AC13 does not test the claim.**
Sections: R12, §7, AC13. `internal/service/refresh.go:235,306,342,370,400` emit `Operation: "refresh"`, and `internal/service/reload.go:544,735` emit `prompt_load`/`prompt_persist`; none is in `operationVocabulary` (`internal/observe/httpdiagnostics.go:660`), and `buildEnvelope` (`internal/observe/envelope.go:184-187`) validates operation only for non-emptiness. So "labels take values only from closed enumerations" is already untrue, and AC13's "unlisted value is rejected by the same vocabulary validator" oracles a helper (`finiteEnum`) that the emit path never consults. Fix: scope R12 to the vocabulary's data (or add emit-time normalisation), state the observed drop-vs-reject behaviour, and declare the pre-existing non-vocabulary operations out of scope so T2 does not chase them.

**LOW — new event names have no documentation owner.**
Section: §7 Events. Eleven new event names are introduced, but T2 owns only the field vocabulary and T3/T4 own no row in `docs/logging-contract.md` (whose scope claims "the complete application-owned vocabulary") or the 23-event pin map in `cmd/mcp-bridge/logging_contract_test.go:703-751`. Fix: assign the new event rows and pins to T3/T4 (or T2) by exact file.

**Verified closures and positives.** r14's REV-A (one inequality, ≥6-byte headroom), REV-B (well-formed qualifier), REV-C (AC25 qualification), REV-D (1,000-draw count) and REV-E (single R5 cut-point rule) are closed as stated. The six carried invariants are present and independently oracled: golden-literal byte oracles (AC2/AC6/AC37/AC40), retired/tombstone lifetime and 256-byte accounting (AC5/AC8/AC18/AC38/AC39), pinned `upstream_code` and sizing (AC9/AC10/AC29), per-terminal preview bounds (AC11/AC26), and ceiling-order validation (AC12/AC20–AC23/AC33–AC35/AC41). Metadata sets are single-sourced, and oracles are named tests with literal expectations rather than re-marshals.

**Limits.** Specification mode only; no runtime checks. AC7/AC8/AC13/AC18/AC39 are package-internal oracles, which their task pairing states rather than hides.

### Recommendation
REQUEST CHANGES
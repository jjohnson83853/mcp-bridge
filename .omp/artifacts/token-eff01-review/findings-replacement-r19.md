## Verdict: REQUEST CHANGES

**Subject mode:** specification · **Pass:** initial · candidate `.scratch/token-efficient-tool-responses/spec.md`, r19-replacement (on-disk bytes).

**Verified closures (r18):** the violation record now uses the registered `operation: logging` and the corrected gate anchor (`internal/observe/envelope.go:151`); terminals restate R3's measuring rule; R7 states the tombstone clearing condition; AC23/AC28 are split; `execution_state` is a closed 3-value registry and upstream `completed` is normalized to `unknown` (`internal/upstream/dedicated.go:527-531`). The six carried invariants are present: literal byte oracle (AC2/AC6/AC40), tombstone/marker lifetime+accounting (R7/R16/AC8/AC39), pinned `upstream_code` (R9/AC10/AC29), finite operation/execution_state registries with producers (R12/AC13/AC44), per-terminal bounds (R10/AC11), and all ten ceiling relations, each with a distinct diagnostic and AC.

### Findings

**1. [HIGH][spec-conformance · confidence HIGH] `§2 R16` + `§4 AC18` weaken the authoritative eviction/expiry error contract.**
R16's no-marker branch evicts an entry without a marker and returns `invalid_handle` "rather than `result_evicted`". This contradicts intake O4 and refinement R8 ("expired and evicted handles fail with distinct typed error codes"). It is reachable with a load-accepted config (e.g. `delivery_ceiling_bytes=100`, `max_retained_result_bytes=max_bytes_aggregate=200`, `max_entries_per_namespace=1`): a 150-byte entry is smaller than the 256-byte marker, so the marker is infeasible and the handle collapses into the forged-handle code. Consequence: a client cannot distinguish expiry/eviction from forgery when aggregate headroom is exhausted. Fix: state a rule guaranteeing a 256-byte marker slot for every admitted entry (load relation or admission reserve) so `result_evicted`/`result_expired` are always distinct, or escalate the weakening to the owner; OD3's "documented, not load-validated" note does not preserve O4/R8.

**2. [MEDIUM][AC-gap · confidence HIGH] `§4 AC4` does not test R4's "not content-derived".**
AC4 checks only 43 chars, base64url alphabet, and pairwise distinctness in 1,000 draws. A deterministic hash/HMAC of the result content is 43-char base64url and distinct across distinct fixtures, so a content-derived handle passes. Consequence: refinement R7/G4's non-derivation requirement is unoracled. Fix: assert two byte-identical over-ceiling results under one owner receive different handles, and that the injected entropy-failure seam yields `result_entropy_unavailable`.

**3. [MEDIUM][AC-gap · confidence HIGH] `§4 AC3`/`AC11` do not assert R3/R10's literal byte prefix.**
AC3 says "non-empty `utf8.Valid` prefix"; AC11 says "non-empty prefix". Neither asserts the prefix is of the golden fixture's expected serialization, so a constant valid string passes. Consequence: R3's "byte prefix of the serialized result" is unverifiable. Fix: assert `Content` equals the literal expected byte prefix of the golden fixture (rune-aligned, truncated to the ceiling), per surface and per terminal.

**4. [MEDIUM][AC-gap · confidence HIGH] `§2 R5`/`§3 Page`/`§4 AC6`,`AC37` leave `remaining_*` unasserted and its byte domain undefined.**
R5/§3 define `remaining_pages = total_pages − page` and "`remaining_bytes` = bytes after this page", but AC6/AC37 assert only that the last page is zero. An implementation emitting `0/0` on every page passes, and raw-vs-escaped bytes is unstated. There is no page-metadata exactness oracle analogous to AC3/AC26. Consequence: refinement R3's "reports total/remaining pages, total/remaining bytes" is unverified. Fix: state `remaining_bytes` is on the retained serialized bytes (same domain as `total_bytes`) and assert per-page literals on a 3+-page fixture, plus a page-metadata exactness assertion.

**5. [MEDIUM][finite-labels · confidence HIGH] `§7 Events` names only seven of the twelve new events.**
"The five terminal events" are never enumerated, and no task owns `internal/observe/log.go`'s `eventMessage` registry (T2 green-lights only `reasonVocabulary` edits; T3 owns docs/pin rows). `eventMessage`'s `default: "mcp-bridge event"` silently admits any name, so the `23 → 35` pin map is self-authored. Consequence: a renamed terminal event or a vacuous message passes T3. Fix: enumerate the five terminal event names (e.g., equal to their terminal codes) in §7 and the T3 rows, and add the `eventMessage` entries to a named task's write roots, or state the default message is the contract.

**6. [MEDIUM][task-ownership · confidence HIGH] `§5 T4` write roots cannot deliver AC1/AC13's T4 halves.**
AC1 assigns T4 the "full four-surface parameterization" of `TestSmallResultPassThroughByteIdentical`; AC13 assigns T4 `TestLogLabelProducers` (surface half). Those functions belong to T3's `internal/service/bounded_delivery_test.go`; T4's only test write root is `internal/service/discovery_paging_test.go (new)`, and Go rejects duplicate function names. Consequence: T4's RED cannot compile without editing an out-of-root file. Fix: add `bounded_delivery_test.go` to T4's write roots, or name T4's separate oracles (`...AllSurfaces`, `...Discovery`) and update AC1/AC13.

**7. [MEDIUM][security · confidence MEDIUM] `§7` logging boundary permits the handle value.**
"No payloads, credentials, argument content, or handle values beyond the opaque string" reads as allowing the opaque handle itself to be logged. A handle is a bearer capability for retained (potentially sensitive) result bytes. Fix: state that handle values are never written to any log/envelope field, only a non-reversible marker.

**8. [LOW][contradiction · confidence HIGH] `§2 R4`'s "every handle rejection ... is byte-identical" conflicts with R13/§3's typed `result_expired`/`result_evicted`.**
The five enumerated classes exclude expired/evicted, but the universal phrasing covers them. Fix: scope R4 to the five enumerated classes.

### Positive Observations
The ten config relations are each bound to a distinct diagnostic and AC, the metadata sets are single-sourced (§3 references R3/R5/R10), staging and T1–T3 write roots are precise, and the escaping-aware cut-point rule (R5 + AC37) correctly models double-escaping.

### Recommendation
REQUEST CHANGES
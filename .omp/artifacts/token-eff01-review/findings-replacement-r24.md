**Mode:** specification · **Pass:** initial · **Candidate:** `.scratch/token-efficient-tool-responses/spec.md` (r24-replacement, 69 KB class) · **Inputs used:** spec, packet K1–K10, intake O1–O9, r22 ledger, live source anchors read-only.

VERDICT: REQUEST CHANGES

### Findings

**F1 [MEDIUM] — R16 final paragraph / R6 "Consequence" vs AC5: the typed retrieval code is conditioned on a marker that only a sweep can create.**
Defect: R16 defines the code as returning pages "while the entry exists" and the typed code "while the entry is gone but a live marker exists — regardless of sweep timing". Markers are installed by expiry/eviction processing (R16 opening; R6's 256-byte reservation is accounting only). After `handle_ttl` and before the first sweep materialises the marker, the entry is logically gone and no marker exists: the sentence yields `invalid_handle` (or pages, if "exists" is read physically), while AC5 requires `result_expired`. "Regardless of sweep timing" is self-contradictory because marker existence is sweep-dependent.
Consequence: two conforming implementations differ in a common state; AC5 fails a literal R16 implementation.
Fix: make the code a function of logical state, not marker presence: entry not past `handle_ttl` → pages; past TTL or evicted → the terminal cause's typed code until that marker's lifetime expires, whether or not a sweep has materialised it; only then `invalid_handle`. Replace "while the entry exists" with "while the entry is live".

**F2 [MEDIUM] — AC5 vs AC18 (with R4, §6): an evicted handle observed after `handle_ttl` satisfies both ACs with different codes.**
Defect: AC5 asserts "Handle past `handle_ttl` → `result_expired` while entry or marker exists"; AC18 asserts an evicted handle → exactly `result_evicted` while its marker exists. Because §6 validates only `tombstone_ttl > 0` (no relation to `handle_ttl`), an admitted config with `tombstone_ttl` ≫ `handle_ttl` makes the state reachable (evict at T, marker live, observe past `handle_ttl`).
Consequence: the same state has two normative codes; the "one state yields exactly one code" precedence rule in §3 does not cover cause vs clock.
Fix: state in R16 that the terminal cause is fixed at the transition (an evicted entry never "expires", so eviction's code wins) and scope AC5 to entries that reached `handle_ttl` un-evicted; alternatively add an R11 relation that guarantees eviction markers clear before handle expiry.

**F3 [MEDIUM] — R11(g) and AC28: the "6-byte `\uXXXX` rune" worst case is not realisable in the domain the headroom protects.**
Defect: the headroom bounds re-escaping of the *retained serialized bytes* (R5), which are `encoding/json` output. Raw control bytes, U+2028/U+2029, and (under the escaping actually enabled) `<`/`>`/`&` cannot occur in that text; the realisable worst case when re-escaping it is a raw backslash/quote doubling to 2 bytes. R11(g)'s examples ("a one-byte control rune always") describe the first serialization (R2's measure), not the second.
Consequence: AC28's fixture ("a single rune whose escaped form is the 6-byte sequence named in R11(g)") cannot be constructed, so T3's stated worst-case oracle is unbuildable, and the "exactly 6 bytes" derivation does not support the non-empty guarantee.
Fix: keep 6 bytes as a declared conservative pinned constant while correcting its derivation (state the realisable maximum, 2 bytes), and rewrite AC28 to use a constructible straddling rune at exactly the pinned headroom. AC23's arithmetic is unaffected.

**F4 [LOW] — §7 preamble vs addEnvelopeFields and T2's field vocabulary: the "retention namespace identifier" is a permitted field with no registry entry.**
Defect: §7 says only `handle_present` "and the retention namespace identifier may appear", but the envelope allowlist (`internal/observe/envelope.go:485-487`) returns `false` for any key without a case, so emitting an unnamed identifier field would drop the entire record; T2's documented field vocabulary lists only `total_bytes`, `page`, `total_pages`, `handle_present`.
Fix: name the field and add its allowlist case, docs-union row and T2 vocabulary entry, or state that the identifier is never emitted.

**F5 [LOW] — §7 (Finite labels, final sentence) vs §5 T2: stale conditional on `internal/observe/log.go` write ownership.**
Defect: §7 says log.go joins T2's write roots only if a new reason value is registered and that T2 adds none; T2 nevertheless lists `internal/observe/log.go` unconditionally for the `eventMessage` registry, so the conditional is false and contradicts the task table.
Fix: scope that sentence to `reasonVocabulary` and state `eventMessage` is already T2's root.

### Positive Observations
All six carried invariants are present and testable: literal byte oracles (R1/R2, AC2, AC40), retired-namespace tombstone lifetime and accounting (R7, AC8, AC38, AC43), pinned `upstream_code` (R8/R9, AC9/AC10/AC29), finite label registries with producers (R12, AC13, AC44), per-terminal preview bounds (R10, AC11, AC26), and ceiling-order validation (R11(a), AC12). The r22 closures verify in text: greedy longest-prefix page pinning with `total_pages` read off the pass (F1), removal of the per-namespace refusal terminal (F2), the marker-cap carve-out (F3/F5), the six-class byte-identical rejection set with the `Mcp-Session-Id` discriminator (F4/F6), and OD3's dropped arithmetic (F7). Anchors check out against the tree (`internal/service/inbound.go:24`, `session_exposure.go:146`, `httpdiagnostics.go:660`, `observe/log.go:35`, the `buildEnvelope`/`addEnvelopeFields` gate, and the 23-event pin map). Write roots are file-exact and block-scoped.

**Review limit:** specification bytes and stated source anchors only; no implementation was inspected and no tests were executed.

### Recommendation
REQUEST CHANGES
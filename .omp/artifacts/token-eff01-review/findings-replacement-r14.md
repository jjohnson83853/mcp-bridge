VERDICT: REVISER

Implementation-mode not applicable: specification mode, initial pass on the on-disk `r14-replacement` bytes (STATUS §1). I read the requirements, ACs, tasks, config and observability sections raw (no truncation), and independently checked r13's six closures. No implementation exists; no test was run.

## Findings

**REV-A [MEDIUM] — `envelope_feasibility` carries three incompatible thresholds.**
Section: §2 R11(g) vs §4 AC23, AC28; §5 T1.
Defect: R11(g) requires the maximal-metadata envelope *plus ≥6 bytes of last-rune headroom* to be **<** the ceiling (strict, so headroom ≥7). AC23 defines infeasibility as "envelope at/over its ceiling, before headroom" (rejects only headroom ≤0). AC28 rejects only headroom <6 (so headroom =6 is accepted). A config with exactly 6 bytes of headroom is therefore accepted by AC23 and AC28 and rejected by R11(g).
Consequence: T1's `TestConfigEnvelopeFeasibility` and `TestEnvelopeHeadroomMaximalMetadata` can encode opposite expectations for the same key set; the load gate is not uniquely determined, and the R11(g) failure cannot be reproduced by either named oracle.
Fix: state one inequality in R11(g) — `envelope + 6 ≤ ceiling` (equivalently headroom ≥6) for preview, page and terminal — and make AC23 reject exactly that inequality, with AC28's "<6 rejected" as the stated corollary.

**REV-B [MEDIUM] — the no-epoch `retrieve_result` sentence contradicts R13's shape-first precedence.**
Section: §3 `retrieve_result`/Owner binding vs §2 R13; §4 AC25, AC32.
Defect: §3 says a session-ful request whose inbound layer stamped no epoch "leaves no owner namespace, so **every lookup** fails binding into the shared `invalid_handle`". R13 (and AC25) require handle shape to be validated first, so a shape-invalid handle returns `invalid_arguments` regardless of resolution; AC32 says "never `invalid_arguments` on a well-formed but unresolvable handle", not on a malformed one.
Consequence: for one reachable input — session-ful, no stamped epoch, malformed handle — §3 yields `invalid_handle` and R13/AC25 yield `invalid_arguments`. Two observable codes for one input; T3 cannot satisfy both a §3-derived rejection test and `TestRetrieveShapeInvalid`.
Fix: qualify §3 to "every **well-formed** handle fails binding into `invalid_handle`; a shape-invalid handle still returns `invalid_arguments` per R13", and add the no-epoch row to AC32's precedence matrix.

**REV-C [LOW] — AC25's page-shape clause is stated unconditionally, colliding with AC32.**
Section: §4 AC25 vs AC32; §2 R13.
Defect: AC25 says "(malformed handle; non-integer or <1 page) → `invalid_arguments`", while R13/AC32 require a shape-invalid page on an unresolvable/foreign/expired handle to return `invalid_handle`/`result_expired`/`result_evicted`. AC25 does not pin the handle as resolvable, so its oracle can assert the wrong code.
Fix: qualify AC25's page clause to a resolvable, owner-bound handle, or move page-shape out of AC25 and leave it solely to AC32's precedence table.

**REV-D [LOW] — AC4's distinctness clause is not falsifiable.**
Section: §4 AC4.
Defect: "distinct across many CSPRNG draws" has no draw count and no property a counter-based generator would fail in a finite sample; it cannot discriminate a CSPRNG from a deterministic sequence. R4's genuinely falsifiable properties are the 43-character base64url form and binding rejection.
Fix: pin the draw count and assert character-set plus intra-batch uniqueness, or drop the clause and rely on AC4's binding assertions and AC16.

**REV-E [LOW] — duplicated normative clause with drift risk.**
Section: §3 Page vs §2 R5.
Defect: §3's "cut points chosen against the serialized page value … not a raw slice length" is a verbatim restatement of R5; the document's single-sourcing convention covers only metadata sets, so the two copies are not bound to agree.
Fix: replace the §3 sentence with "per R5's cut-point rule" and keep R5 as the sole normative text.

## Closures and positive observations

r13 findings are closed: REV-1 (T3 RED/GREEN now name AC37, AC40; AC37 pins the golden literal), REV-2 (R11(i), AC41, T1 RED), REV-3 (R6 states entry-commit order only; all marker commits cite R16's single ordered rule; AC7/AC39 cite it), REV-4 (no `(F1)` token remains), REV-5 (AC32 now says "well-formed but unresolvable"), REV-6 (escaping mode pinned to the existing SDK marshaler with a stated 6-byte worst case). The six carried invariants are present and independently oracled: golden-literal byte oracles (AC2, AC6, AC37, AC40), retired/tombstone lifetime and 256-byte accounting (AC5, AC8, AC18, AC38, AC39), pinned `upstream_code` and sizing (AC10, AC29), closed label enumerations with producers (AC13), per-terminal preview bounds (AC11, AC26), ceiling-order validation (AC12, AC20–AC23, AC28, AC33–AC35, AC41). O1–O9 and refinement R1–R12 all map without weakening. Grounding verified: `operationVocabulary` (httpdiagnostics.go:660) contains `call_tool`/`describe_tool` but neither new value; `requestExposureEpoch` parses exactly one header (session_exposure.go:146-176); inbound strips and stamps the epoch (inbound.go:481, 596); `upstream_result` nesting (tools.go:869) and the direct path (direct_execution.go:59-63) match R8; `Register` at tools.go:171 called at serve.go:175; README anchors at 277/441 with both superseded phrases at 325/468; every T2/T3/T4/T5 write root exists as named or is a non-colliding new file.

## Review limits

Specification mode only; no tests, scenarios or diagnostics executed. The packet's r14 digest (3d18750b…) was not recomputed — no hashing tool in this profile — so I reviewed the bytes on disk (STATUS `r14-replacement`). AC7, AC8, AC18, AC39, AC14 and AC13's enumeration half are checkable only by package-internal tests; that is stated in the AC/T-task pairing, not a defect.

### Recommendation

REQUEST CHANGES
## Verdict: REQUEST CHANGES

**Subject mode:** specification · **Pass:** initial · candidate `token-efficient-tool-responses-ticket01-spec` (r18-replacement, on-disk bytes).

The document is materially reviewable: every requirement is falsifiable, the six carried invariants are all present, and I verified its code anchors — README anchors (`## Tool surface` :277, `### Progressive metadata reveal…` :441) and both forbidden phrases (:325, :468); `ToolSurface.Register` (:171) with `serve.go:175` naming no builtin; the closed `diagnosticCodes` array; `operationVocabulary`'s 18 current values and the 18+11+2=31 arithmetic; the 23→35 event pins and 4/5/4 tool-count pins in `serve_hybrid_reload_test.go`; the inbound retirement call sites (:522, :767→:676, :770, :802-812); and `structuredContent.upstream_result` (:869) making R8's `call_tool` producer path real. Three defects nevertheless need repair.

### 1. [HIGH] R12 / AC13(b) — the vocabulary-violation record has no feasible emission route

**Section:** §2 R12 (vocabulary enforcement); §4 AC13(b); §5 T2/T3 write roots. **Requirement:** R12.

R12 requires the offending record be "dropped and replaced" by a record whose `operation` is **empty**, emitted "in the `addEnvelopeFields` path (envelope.go:184-187, which today checks only non-emptiness)". But `buildEnvelope` rejects `entry.Operation == ""` at `internal/observe/envelope.go:161`, *before* reaching the `addEnvelopeFields` call at :184-187. The pinned replacement shape is therefore precisely the shape that path discards. The citation is also inverted: :184-187 is the field-validation call; the non-emptiness gate is :161.

**Observable consequence:** AC13(b) ("the emitted violation record does not carry the injected value (empty `operation`, event `log_vocabulary_violation`)") passes only via an unstated bypass of the builder's own gate. An implementer wiring enforcement where R12 says silently emits nothing and never satisfies the AC — the enforcement mechanism for the finite-label invariant is unspecified.

**Fix:** state that `log_vocabulary_violation` is exempt from the empty-`operation` gate (or name the dedicated construction path and its full field set), and correct the anchor to the actual gate.

### 2. [MEDIUM] R10/R14 — a new, unregistered value in an existing closed field

**Section:** §2 R10, R14; §4 AC26. **Requirement:** R14.

`StructuredContent.execution_state` today has exactly two bridge-rendered values — `not_started` and `unknown` (`internal/service/tools.go:1044-1045`, pinned at `internal/core/core.go:71-76`) — and upstream `CallError.ExecutionState` already uses `completed` for the same post-execution state. Terminals introduce `executed`, a literal that appears nowhere in the tree, with no vocabulary registration and no producer/consumer statement, while R12 meticulously closes `operation` and `outcome`.

**Observable consequence:** a client switching on the documented `{not_started, unknown}` set receives an unrecognised value on the same field, and the bridge carries two synonyms (`completed`, `executed`) for one state — the precise drift the finite-label invariant exists to prevent.

**Fix:** reuse the existing post-execution literal (`completed`), or extend R12 with an `execution_state` closed registry naming both producers and add an AC asserting the exact set across `errorResult` and all five terminals.

### 3. [LOW] R10/AC11 — terminal bounds don't restate R3's measuring rule

**Section:** §2 R10; §4 AC11.

R10 says terminals are "bounded by `preview_ceiling_bytes`" without restating R3's measure (the serialized value after escaping), and AC11 says "preview ≤ preview ceiling", which is readable as a `Content`-length check. A Content-only implementation would satisfy AC11 while exceeding the serialized ceiling.

**Fix:** R10 references R3's measuring rule explicitly; AC11 asserts the serialized terminal result ≤ `preview_ceiling_bytes`.

### 4. [LOW] R7 — the tombstone's clearing condition is claimed but never stated

**Section:** §2 R7.

R7 requires "an explicit clearing condition" and states none; only §6's config table and AC8 carry it (age > `tombstone_ttl` on sweep), whereas R16 states its parallel marker condition inline.

**Fix:** state the tombstone clearing condition in R7, or cite §6 instead of claiming it is present.

### 5. [LOW] AC23/AC28 — one rule, two rejection oracles

**Section:** §4 AC23, AC28.

Both assert R11(g)'s identical negation (<6-byte headroom rejected, typed `envelope_feasibility`) under two test names; AC28's first half is AC23 restated (the document acknowledges this).

**Fix:** keep AC23 as the rejection oracle and reduce AC28 to the satisfaction/rune-straddle half.

### Positive observations
The independent byte oracle is genuinely independent (literal expected bytes/lengths, "no re-marshal through the new path", AC2/AC6/AC40); the escaping-aware cut-point rule (R5 + AC37) correctly models double-escaping in the page envelope; metadata sets are single-sourced and referenced; and the retirement accounting (R7/R16/AC8/AC39/AC43) is observable through admission and retrieval without reading internals.

### Recommendation
REQUEST CHANGES
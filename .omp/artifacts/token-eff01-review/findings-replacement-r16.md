VERDICT: REVISER

Specification mode, initial pass on the on-disk `r16-replacement` bytes (§1). No implementation or tests exist; no runtime checks were possible or claimed. I verified the r15 ledger’s ledgered closures against the current text and grounded citations in the tree.

**F1 — HIGH — Finite operation vocabulary is still not closed; AC13’s producer scan cannot pass.**
Affects R12, AC13, §7 “Finite labels”, T2 GREEN.
Defect: r16 registers five values, but production still emits `startup` (`cmd/mcp-bridge/serve.go:51,65`; `cmd/mcp-bridge/wiring.go:487`), `handshake` (`internal/upstream/dedicated.go:345`), `check_config` (`cmd/mcp-bridge/main.go:75`), `load_config` (`cmd/mcp-bridge/diagnostics_envelope.go:18-24`), and `reload`/`publish` (`cmd/mcp-bridge/wiring.go:476-526`), none of which is in `operationVocabulary` (`internal/observe/httpdiagnostics.go:660-664`).
Anchor: R12 (“the vocabulary equals the emitted set”); AC13 (“enumerates the code’s emitted `Operation` values and asserts the emitted set is a subset of the vocabulary”).
Consequence: AC13’s stated oracle fails on every one of those producers, so `TestLogLabelEnumeration` cannot go green as T2 declares; the “finite labels with producers” invariant the owner carried forward remains unproven, and the §7 sentence that post-change vocabulary “equals the emitted set” is false.
Fix: either register every production operation value and state the exact resulting set/count in R12/AC13, or scope R12/AC13 explicitly to one named lane and drop the cross-lane `refresh`/`prompt_*` additions; make the scan’s scope part of the AC.

**F2 — MEDIUM — No acceptance criterion binds namespace retirement to the inbound hooks, so R7’s production path is untested.**
Affects R7, §3 “Owner binding”, §5 T3.
Defect: R7 makes “namespace retirement” the trigger for tombstones and `result_namespace_retired`, but no requirement states which runtime events retire a namespace (session DELETE, idle timeout, epoch bump), and every retirement oracle (AC8, AC18, AC38, AC32) is store-level; T3 merely asserts “sweep trigger invoked from the inbound retirement hooks” with no RED test.
Anchor: §3 Terminals row `result_namespace_retired`; T3 objective and RED list.
Consequence: the inbound wiring can be absent while all ACs pass, leaving tombstone lifetime/accounting and the retired-namespace terminal reachable only through a T2 API the spec never names — production behavior unverified.
Fix: state the retirement trigger(s) in R7 and add one T3 oracle that a session DELETE or epoch bump retires the retention namespace and installs its tombstone (or explicitly scope retirement to the store API and delete the hook claim).

**F3 — LOW — “An eviction ALWAYS installs its marker” contradicts its own exception.**
Affects R16, AC39.
Defect: the sentence beginning “an eviction ALWAYS installs its marker” is immediately followed by the no-marker branch, so the normative rule is self-contradictory as written (the branch resolves intent, but the requirement does not).
Anchor: R16 first sentence; AC39 same wording.
Consequence: an implementer may treat the no-marker branch as unreachable; the AC resolves it, but the requirement text does not.
Fix: state the qualified rule once — marker unless aggregate admission is infeasible and the namespace holds no other markers.

**F4 — LOW — T5’s RED includes a test T3 already makes green; two owners for one string.**
Affects §5 T3/T5, AC27.
Defect: T3 owns the `retrieve_result` description wording via the exact-string map in `builtin_guidance_test.go`, so `TestRetrievalContractDocumented` (description half) cannot be RED in T5 and duplicates T3’s oracle.
Anchor: T3 write roots; T5 RED line.
Consequence: the task ledger’s falsifiable RED premise is wrong for that test, though the task still exits 0.
Fix: move the description assertion into T3’s RED/GREEN, leaving T5 with `TestReadmeInventoryAndPromise` and the integration test.

**Verified closures and strengths.** r15’s HIGH (write-root omission at `cmd/mcp-bridge/serve_hybrid_reload_test.go`) is closed: T3 now lists the file and the 4→5/5→6/4→5 shift, matching lines 225-226, 238, 262. r15’s pinned-width, precedence, and logging-contract findings are closed. Independent byte oracles (AC2/AC6/AC37/AC40), pinned `upstream_code` and sizing (AC10/AC29), per-terminal preview bounds (AC11/AC26), ceiling-order validation (AC12/AC20-23/AC33-35/AC41), and single-sourced preview/page/terminal metadata sets are present and falsifiable by named tests. Only the finite-label invariant fails, per F1.

REQUEST CHANGES
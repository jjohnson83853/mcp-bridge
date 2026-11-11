## Specification Review — `structured-logging` r7 (independent)

**Verdict: REVISE** — 6 blocking findings, 12 advisory findings.

### Blocking

**B1 — AD1–AD12 normative contracts are absent from r7.**
Location: spec §3 `[AUTO-DECISION] AD1–AD12: unchanged from r6`; §4 contracts referencing AD2/AD5/AD6/AD8/AD10.
Evidence: `spec.md` contains only AD13–AD15; the 60-slot map, AD5 startup-failure table, AD8 28-reason/union/data-loss list, AD6 flag rules, and AD10 conflict mappings exist only in `spec-r6-assembled.md:77-126`. T3/T4 cannot implement AC02-1/AC03-1/AC04-2..8 without inventing them.
Required change: inline AD1–AD12 (or bind r6 as a normative artifact named by section hash) before dispatch.

**B2 — `EnvelopeFieldNames()` derivation is arithmetically wrong and ambiguous.**
Location: §4 `envelope.go` contract (“17 config keys”, “26 retained whitelist keys”); §5 AC04-8; changelog ADV-B.
Evidence: ADV-B enumerates 21 config subkeys (10 + 6 `related` + actual_count/max_count/first_excess_index/diagnostics_incomplete/remaining_stages) and 25 retained whitelist keys; `error`, `config`, `resource` object names are absent; bare-vs-dotted naming is undefined while `server` appears three times.
Required change: publish the exact literal sorted list, a dotted-path rule, the three object names, and corrected counts.

**B3 — T4 “NO shape change” is incompatible with the closed union and AC04-2.**
Location: §4 `wiring.go` contract; §6 T4.
Evidence: `cmd/mcp-bridge/manifest_test.go:74-100` asserts 18 flat keys incl. `config_path`, `location`, `code`, `property`, `related_location`, `diagnostic_index`, none of which is in the ADV-B union; `serve_edge_test.go:211,221` and `serve_test.go:724` assert flat `reason`. With `Entry.Reason` deleted and the builder rejecting out-of-union keys, those rows cannot survive.
Required change: specify T4’s real mapping (`config.*`/`error.*`), migrate those assertions, and delete the “compile-equivalent/NO shape change” claim.

**B4 — Unscoped CLI-output tests break T1/T6 and cannot be fixed by T7.**
Location: §6 T1/T6 file lists; scope-envelope `writePaths`.
Evidence: `cmd/mcp-bridge/main_test.go:29-37` requires success stdout to contain reference names (T6 removes them); `embedding_env_cli_test.go:49` and `embedding_env_process_test.go:65` require plaintext `key="embedding.model"`, `code=reference_unresolved`, `shape=missing` (T1 emits JSON). None of the three files is in any task effect or state `writePaths`; T6’s `go test ./cmd/mcp-bridge/...` fails.
Required change: add all three files to T1/T6 effects and the write scope, or replace their assertions.

**B5 — AC04-8 staged evidence is not bound in the execution state.**
Location: §5 AC04-8; §9 item 7; `.omp/plans/execution-state.json` AC04-8.
Evidence: spec assigns T4/T5/T6/T7 evidence; state has `ownerTaskIds:["T4"]` and T5/T6/T7 `acceptanceIds` omit AC04-8, while T4’s verify excludes the reload/CLI/full-suite producers.
Required change: bind AC04-8 to T4+T5+T6+T7 (or split into per-task ACs) in the state.

**B6 — ADV-H node-source table is factually wrong/incomplete.**
Location: §4 `config.go` contract; §3 ADV-H.
Evidence: `weight_invalid`/`weights_invalid` come from `validateWeights` (`bounds.go:17-61`) via `findScalar` (`config.go:1075+`, returns strings), not `validateBoundsAt`; server-endpoint `url_invalid`/`url_secret_parameter` (`config.go:190-207`) have no assigned node source (only embedding `url_invalid` is listed), while AC02-1 demands positions wherever a node exists.
Required change: name the real node sources (node-returning lookup for weights; root walk for `servers[i].endpoint`) and add both URL classes.

### Advisory

A1 — AC02-6 `message=Reason` conflicts with the decisions envelope’s pin-ceiling message; record the supersession or align.
A2 — AC02-5 no longer asserts ticket 02 AC-5’s `property_path:"<document>"`; add it.
A3 — AC05-5 replaced ticket 05 AC-5’s identical-attribution assertion; restore a cross-path attribution equality test.
A4 — AD13’s `internal_warning` fallback has no AC; add a fallback-shape/negative test.
A5 — AD13’s discrimination rule is unstated (prompt closure carries no error arg; snapshot vs generic share an error arg); name the format-prefix/arg discriminator.
A6 — §3 still claims state `revision: r6`; actual state is r7.
A7 — §7 retention exception states no consequence; state the data-loss/retention window.
A8 — `publication_failed`/`snapshot_unavailable` are outside the closed 28-value table; state direct `ErrorInfo` construction or extend the table.
A9 — T1 lists `serve_edge_test.go:42` (a success assertion) despite prohibiting success-record changes.
A10 — AC07-1 omits ticket 07 AC-1’s collector-owned-field list.
A11 — §6 T4 says `serve_logging_*.go ×9`; repository has 7.
A12 — `serve.go:262` is the `operationsServer` literal; `loop.Start` is at `:265`.

### Prior findings

BLK-A OPEN — prose split added; state AC04-8 binding and T4 shape contradiction remain (B3/B5).
BLK-B RESOLVED — six `NewDedicated` sites and four files are in T4/state.
BLK-C RESOLVED — `tools.go:207` deletion, single `operatorLog` emitter, full-path AC05-5.
BLK-D RESOLVED — lifecycle mappings defined; residual A4/A5.
BLK-E RESOLVED — `load_config` adopted for both flavors.
ADV-A RESOLVED — strict-serial rule in ledger and envelope.
ADV-B OPEN — counts, object names, naming convention (B2).
ADV-C RESOLVED — level/outcome pinned per event.
ADV-D RESOLVED — `startup_publication_failed` row; `Start` does not logf.
ADV-E RESOLVED — `Hint` never set; enumerated loss.
ADV-F RESOLVED — citation/state bullet corrected; residual A6.
ADV-G RESOLVED — `DiagnosticCodes()` registry.
ADV-H OPEN — weight and server-URL node sources (B6).
ADV-I RESOLVED — AC05-5 full-path exactly-one count.

### Verdict
REVISE

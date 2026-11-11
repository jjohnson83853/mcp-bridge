## Specification Review — `structured-logging` r5 (independent)

**Mode:** specification review (pre-implementation)
**Reviewed inputs:** `.scratch/structured-logging/spec.md` (r5), `decisions.md` (D1–D20), `issues/01..07-*.md`, `.omp/plans/execution-state.json` (revision r5), prior reviews `review-code-reviewer.md` (r3) and `review-code-reviewer-r4.md` (r4).
**Repository checks:** `internal/observe`, `internal/config`, `internal/service`, `internal/upstream`, `internal/auth`, `cmd/mcp-bridge`, `README.md`, `CHANGELOG.md`, `docs/`. No files modified.

**Verdict: FAIL (REVISE)** — 5 blocking findings, 9 advisory findings. r5 genuinely resolves K1–K8 and N2–N8; the remaining blockers are the partially-open N1/B7/A7 thread and three cross-task/contract gaps created or left by the r5 rewrites.

---

## Prior-review resolution (verified independently)

| Prior finding | Status | Evidence |
|---|---|---|
| B1 / K1 — compile-coupled files and task effects | **RESOLVED** | State r5 now binds `httpwriter_test.go`/`dedicated_test.go`→T4, `example_config_test.go`→T1/T6, `serve_edge_test.go`→T1/T3/T4/T5/T6, `runtime_edge_test.go`→T3/T4, `httpdiagnostic_types.go`→T4. Every `NewLogger(` call site found in `.go` files (21 files: `internal/observe` 8, `internal/service` 6, `cmd/mcp-bridge` 5 incl. `serve.go`, `internal/auth` 1, `internal/upstream` 1) is in T4 effects. §9 item 8 corrected. |
| B2 / K2 — AD2 arithmetic | **RESOLVED** | `slotKeys` literal (`httpdiagnostic_encode.go:66-84`) verified: removing 0/5/6/10/20/21 and prepending `schema_version` yields exactly the published map — `service`=4, `http_bytes_bucket`=12, `build_state`=17, `error`=55, `http_bytes`=58, `exposed_tools`=59; 60 slots. `numHTTPSlots=65` (`:56`). No off-by-one found. |
| B3 — pin-ceiling identity/precedence | **RESOLVED** | AD9 + AC01-7/AC01-8; current ceiling is before the loop at `config.go:210-216`, loop at `:218-281`, so the move is well-defined. |
| B4 — `renderDiagnostics`/`Error()` | **RESOLVED** | Retained permanently (`diagnostics.go:34-36,90-92`); T2 owns the renderer test. |
| B5 / K4 — closed vocabulary + HTTP lane | **RESOLVED (with residual ambiguity)** | AD8 now enumerates the full 60-slot lane; `diag_environment`/`source_revision` removed (AD12); R6 includes `diagnostics_incomplete`/`remaining_stages`; AC04-8 includes the lane and hostile `protocol_version`. Residual: ADV-1/ADV-7. |
| B6 — startup config rejection | **RESOLVED** | T3 owns `main.go` `errors.As` branch; AC03-7 bound. |
| B7 / A7 / N1 — position attribution | **PARTIALLY OPEN → BLK-1, BLK-5** | Own positions per shape added, but `property_path` still contradicts “offending node”; missing-required-key class has no position rule and is not in the AD10 exception list. |
| B8 — multi-record selection | **RESOLVED** | AC05-6 bound to T5. |
| B9 — README/docs migration | **RESOLVED** | AC07-8 inspection; `README.md` in T7 effects; README `:288-292, :561-563, :757, :804-805` still show removed fields and must migrate. |
| B10 — doc AC method | **RESOLVED** | State: AC07-1/2 `method: test`; AC07-8/9 `inspection`. |
| A1 — `request_id` tie-in | **RESOLVED** | AC04-1 amended; §7 names the assertion. |
| A2 — `message` = static reason | **RESOLVED** | AD8 message bullet; AC02-6. |
| A3 — `--help` stream | **RESOLVED** | AD6 rewritten; verified `main.go` uses `ContinueOnError` + `SetOutput(stderr)` + `return 2`, and `runtime_edge_test.go:17-22` asserts today’s usage/exit. |
| A4 — AD5 per-site table | **RESOLVED** | AD5 lines 83–91 enumerate site→code/kind/retryable/remediation. |
| A5 — R15/R16/R19 acceptance | **RESOLVED** | AC07-9 inspection bound. |
| A6 — T7 decision append | **RESOLVED** | Removed from T7. |
| K3 — lane error source | **RESOLVED** | `legacyEntry.Reason` retained as conversion input; AD2 emission rule matches `httpdiagnostic_encode.go:505-519` exactly. |
| K5 — usage on parse errors | **RESOLVED** | AD6 `io.Discard` + ErrHelp-only rendering; AC03-4 asserts one JSON object and no `Usage of mcp-bridge:`. |
| K6 — runtime ErrorInfo table | **RESOLVED** | All `reasonVocabulary` values handled (27 mapped families + `none`→nil); `publication_failed` mapped to `internal_error`; presence rule matches producer outcomes (`tools.go:807-945`, `direct_execution.go:71-159`). |
| K7 — protocol_version | **RESOLVED** | `protocol.go:12-16` four versions, `AdvertisedProtocolVersions()` returns a copy, `2026-07-28` handled by `NegotiateInitializeOffer`; only `cmd/mcp-bridge/wiring.go` imports `internal/upstream`, so upstream→service is cycle-free. |
| K8 — T2 renderer test | **RESOLVED** | `cmd/mcp-bridge/diagnostics_envelope_test.go` in T2 effects (spec and state). |
| N2–N8 | **RESOLVED** | AC04-1 message; AC04-3 remediation; write list mirrors state + `CHANGELOG.md`; method labels; AC07-1 evolution rule; AC07-2 per-severity examples; AD5 event/operation. |

---

## Blocking findings

### BLK-1 — `weights_invalid` still names the wrong property path for `embedding_negative` (N1/B7/A7 remain open)
**Severity:** blocking · confidence HIGH
**Location:** spec §4 AD10 weights bullet (line 113), §4 renderer contract (line 137), §5 AC02-1 (line 158) and AC02-7 (line 160).
**Evidence:** `internal/config/bounds.go:44-58` computes shapes `keyword_nonpositive` (k≤0), `embedding_negative` (e<0), `sum_not_one`, then emits one record with `Key: "search.keyword_weight"` and `RelatedKey: "search.embedding_weight"` for every shape. AD10 fixes own `Line`/`Column` to the **embedding** node for `embedding_negative` but explicitly keeps `Key: search.keyword_weight`. The §4 renderer maps `config.property_path` from the `Diagnostic` fields (property path derives from `Key`; `renderRecord` uses `record.Key`, `diagnostics.go:121`). AC02-1 requires “`config.property_path`/`line`/`column` at the offending node (… for `weights_invalid` the offending node is per the AD10 per-shape mapping)”. For `embedding_negative` the record would therefore carry `property_path: search.keyword_weight` with the embedding node’s line/column — a contradiction.
**Required change:** Either set the record’s property path per shape (`embedding_negative` → `search.embedding_weight`) and specify how the renderer derives it, or keep `Key` fixed and reword AC02-1/AC02-7 so the own position may differ from `property_path`; state which field AC02-7 asserts for each shape.

### BLK-2 — The upstream handshake logger has no specified production plumbing or nil behavior
**Severity:** blocking · confidence HIGH
**Location:** spec §4 `internal/upstream/dedicated.go` contract (line 136), §5 AC04-8 (line 165), §6 T4 (line 185); §2 R1.
**Evidence:** The only production construction site is `cmd/mcp-bridge/wiring.go:175` — `upstream.NewDedicated(upstream.DedicatedOptions{Server: options, Stateless: ...})` — reached through `newUpstreamRegistry(holder)` (`wiring.go:137`, called from `serve.go:133`, `embedding_env_durable_test.go:49`, `serve_oauth_test.go:1259`). Today `DedicatedOptions.LogWriter` is optional and defaults to `os.Stdout` (`dedicated.go:23,47-53`); the handshake map-encode is at `:244-254`. The design replaces the raw writer with an “owned `*observe.Logger` (constructor accepts it in place of the raw `LogWriter`)”, but never names the call-site/signature change (`newUpstreamRegistry(holder, logger)`), and does not define what a nil/absent logger does. `(*Logger).Log` silently returns on a nil logger, so a missing wire would drop the record with no test failure.
**Required change:** Specify the plumbing (`newUpstreamRegistry(holder, logger)`, pass the composed logger at `serve.go:133`, migrate the two test call sites) and the constructor’s nil contract (require non-nil, or default to a two-writer logger). Add an end-to-end assertion that the handshake record reaches the composed writer.

### BLK-3 — AC07-1/AC04-8 depend on an “exported AD8 vocabulary” that no contract or task creates
**Severity:** blocking · confidence HIGH
**Location:** spec §5 AC07-1 (line 169), AC07-2 (line 170), AC04-8 (line 165), §4 `envelope.go` contract (line 128), §6 T7 (line 188) and scope envelope (line 196).
**Evidence:** No exported field vocabulary exists in `internal/observe`: the whitelist, `slotKeys`, `outcomeVocabulary`, and `reasonVocabulary` are unexported, and the only exported builder-adjacent symbol named in the design is `RuntimeErrorInfo`. AC07-1 requires a mechanical test that “asserts equality with the exported AD8 vocabulary”; AC07-2 requires query examples to reference “only vocabulary fields”. T7’s authorized files are `docs/logging-contract.md`, `README.md`, `CHANGELOG.md`, `cmd/mcp-bridge/logging_contract_test.go` — T7 cannot add a symbol to `internal/observe/envelope.go`. T4 owns that file but its deliverable does not mention a vocabulary export.
**Required change:** Name the exported symbol (for example `observe.EnvelopeFieldNames() []string` or `observe.ClosedVocabulary`) in §4, assign its creation to T4 (or T1) as an explicit deliverable, and reference it by name in AC04-8/AC07-1/AC07-2.

### BLK-4 — The `config_validation_succeeded` record has no specified message/operation/construction
**Severity:** blocking · confidence HIGH
**Location:** spec §2 R13, §4 AD8 message bullet (line 104), §4 `main.go` contract (line 140), §5 AC04-1 (line 163), AC06-1..5 (line 168), §6 T6 (line 187); decisions D14.
**Evidence:** `config_validation_succeeded` appears nowhere in the Go tree. The success path has no `config.Diagnostic` and therefore no static `Reason`; AD8 extends `eventMessage` only for `upstream_ephemeral_handshake`, `startup_failed`, `cli_usage_failed`, `reload_publication_failed` and says configuration-diagnostic events use the renderer’s `Reason`. The only shared renderer takes `*config.Diagnostics` (line 137), so it cannot build the success record. The envelope builder requires non-empty `event`, `operation`, and `message` (line 128), and AC04-1 asserts a non-empty `message` on **every** record, while AC06 asserts only event/outcome/stream/exit/no-error. The success record’s message and operation are currently to be invented by the implementer.
**Required change:** Specify the success envelope construction, its `operation` (e.g. `check_config`) and its message source (e.g. an `eventMessage("config_validation_succeeded")` entry), and add message/operation assertions to AC06.

### BLK-5 — Missing-required-key rejections have no position rule and are absent from the AD10 exception list
**Severity:** blocking · confidence HIGH (R6 makes the exception list part of the requirement)
**Location:** spec §2 R6 (“Where no source position exists (§4 AD10 exception list), line/column are omitted; the exception itself is part of this requirement”), §4 AD10 exception list (line 114), §5 AC02-1 (line 158).
**Evidence:** `internal/config/bounds.go:21-42`: `weight_invalid` with shape `missing` is emitted when `keyword_weight` or `embedding_weight` is absent. `findScalar` (`config.go:1075-1080`) returns only scalar **values**, so there is no node for the missing key. The thirteen `invalid`/missing returns carry `Key: search.keyword_weight`/`search.embedding_weight` but no line/column. AD10’s exceptions cover only (a) accessor diagnostics, (b) `restart_required`, (c) document-level failures with no parser position — not a missing required key. AC02-1 requires `config.line`/`column` “at the offending node” for every rejection class, which is unsatisfiable for this class.
**Required change:** Add “required key absent” to AD10’s position-availability exceptions (line/column omitted, `property_path` present), or define the fallback node (e.g. the `search` mapping) and assert it in AC02-7.

---

## Advisory findings

### ADV-1 — The “closed AD8 vocabulary” is ambiguous between shared-lane and lane-only keys
**Severity:** advisory-high · confidence HIGH
**Location:** §4 AD8 shared/lane vocabulary (lines 98–99), §5 AC04-8 (line 165); `internal/observe/log.go:137-231`.
**Evidence:** The retained whitelist permits `stage`, `http_status`, `http_method`, `route`, `http_outcome`, `timing_scope`, `http_bytes_bucket` on the **shared** lane, and `internal/service/tools.go` emits `"stage"` on admission/`mcp_complete` records; `internal/observe/log_http_fields_test.go:33,192` builds shared-lane `http_complete` entries with those fields. AD8’s shared list names none of them; its “HTTP-lane vocabulary” is described as exactly the 60-key map. AC04-8’s flat key-set assertion passes only if the vocabulary is the union, but the spec never says so.
**Required change:** State that the closed vocabulary is the union of R2/shared keys, the shared extras, and the 60 lane keys, and whether lane keys are legal on shared records; or add `stage` and the HTTP fields to the shared enumeration.

### ADV-2 — The configuration `error.remediation` table is not enumerated and has no fallback
**Severity:** advisory-high · confidence HIGH
**Location:** §4 renderer contract (line 137), §5 AC02-6 (line 159), AC04-3 (line 164).
**Evidence:** `internal/config` produces on the order of 60 `Diagnostic.Code` values (`config.go`, `bounds.go`, `diagnostics.go`, `command.go`, `credentials.go`, `tool_metadata.go`). The renderer is to hold a “finite table in this file” but the spec does not enumerate it, define a fallback for an unmapped code, or require a total mapping. The envelope builder rejects a record whose `error.remediation` is not static/present (line 128), so a missing entry would suppress a rejection record (R1/R9), and AC02-6 does not assert remediation.
**Required change:** Require a total code→remediation map with a defined default and extend AC02-6 (or a dedicated test) to assert non-empty static remediation for every class.

### ADV-3 — `operatorLog`’s no-diagnostics publication-failure branch has no specified envelope
**Severity:** advisory-high · confidence HIGH
**Location:** §4 AD7 (line 95), §4 `wiring.go` contract (line 142), §5 AC05-1..5 (line 166); `cmd/mcp-bridge/wiring.go:435-458`.
**Evidence:** AD7 says publication-failure-after-acceptance keeps event `reload_publication_failed` with `error.code "internal_error"`, but §4’s `wiring.go` bullet says `operatorLog` emits “ONE `config_reload_rejected` warn envelope via the shared renderer”. The existing `publicationFailed` branch has no `*config.Diagnostics`, so the shared renderer cannot build it; `serve_edge_test.go:208-211` asserts `"reason":"internal_error"` today and must migrate. T5’s ACs cover only invalid candidates.
**Required change:** Describe the publication-failure branch’s envelope (event, code, kind, remediation, message) and add an assertion (AC05 or the migrated operator test).

### ADV-4 — Unspecified `operation` values for reload, publication failure, and validation success
**Severity:** advisory · confidence HIGH
**Location:** §4 AD5 (line 82), AD7 (line 95), §4 renderer (line 137); AC04-1 (line 163).
**Evidence:** The builder requires non-empty `operation`. AD5 assigns only startup/flag/config-branch operations. Reload rejection, `tools.go:207` publication failure, and `config_validation_succeeded` operations are unspecified; `operatorLog` entries currently set no `Operation` (`wiring.go:427-458`), and the handshake is assigned `handshake` only.
**Required change:** Pin the remaining operation values (e.g. `reload`, `publish`, `check_config`) and assert them where the event is asserted.

### ADV-5 — AD12’s field names are ambiguous and one deletion target is outside the write scope
**Severity:** advisory · confidence HIGH
**Location:** §4 AD12 (line 118), §4 `httpdiagnostic_types.go` contract (line 132), scope envelope (line 196).
**Evidence:** `httpdiagnostic_types.go` contains `HTTPProvenance.Environment` (`:23`) and `HTTPProvenance.SourceRevision` (`:24`) as well as `HTTPDiagnostic.DiagEnvironment` (`:137`) and `HTTPDiagnostic.SourceRevision` (`:138`). AD12 says “the `DiagEnvironment`/`SourceRevision` fields are deleted from `httpdiagnostic_types.go`” without qualifying the struct. Deleting both provenance fields would break `cmd/mcp-bridge/provenance.go:60-61` and `resolveProvenance` (`httpdiagnostics.go:83-96`), and `cmd/mcp-bridge/provenance.go` is in no write list.
**Required change:** Name `HTTPDiagnostic.DiagEnvironment` and `HTTPDiagnostic.SourceRevision` explicitly; state that `HTTPProvenance.Environment`/`SourceRevision` remain (now emission-unused) or add `cmd/mcp-bridge/provenance.go` to scope with the removal.

### ADV-6 — `process_id` retention is defensible but should be recorded as an explicit D3 reading
**Severity:** advisory · confidence MEDIUM
**Location:** §4 AD12 (line 118), §9 item 2 (line 226).
**Evidence:** `httpdiagnostics.go:765-767` carries `ProcessState`/`ProcessID` from the process context; the spec justifies the value as “an opaque per-process correlation token … not environment, version, host, or container identity”. D3 says the application never emits “subsystem/container identity”. The token distinguishes application instances, which is the role of container identity, so the judgment needs a stated boundary rather than only a rationale.
**Required change:** State the rule explicitly (e.g. “opaque instance correlation token, never derived from host/container identifiers”) and add it to the collector/enrichment contract documented in T7, or remove it (59 slots) if the owner reads D3 strictly.

### ADV-7 — The claim “no other data is lost” is inaccurate: `Diagnostic.Shape` is dropped
**Severity:** advisory-high · confidence HIGH
**Location:** §4 AD8 data-loss list (line 103), §4 renderer mapping (line 137), §2 R6.
**Evidence:** The config object maps path/property_path/line/column/identifiers/related/counts/completeness, but not `Shape`. Shapes are load-bearing sub-classes: `pin_tool_name` has `invalid_chars` vs `too_long` (`config.go:255,264`), `weight_invalid` has `missing` vs `invalid_numeric` (`bounds.go:24`), `secret_file_policy` has several shapes (`diagnostics.go:305`), etc. Only the static `Reason` (rendered as `message`) distinguishes some of them; `error.code` does not. The AD8 statement that only duplicate typed-field keys are dropped is therefore false.
**Required change:** Add `config.shape` (an R18-safe optional addition) or explicitly document the intentional loss (Reason/message carries the sub-shape) and remove the “no other data is lost” claim.

### ADV-8 — T7’s end-to-end coverage for AC07-3..7 has no named file
**Severity:** advisory · confidence MEDIUM
**Location:** §6 T7 (line 188), scope envelope test list (line 196).
**Evidence:** T7’s only new test file is `logging_contract_test.go`, described as the mechanical AC07-1/AC07-2 doc test. AC07-3..7 require e2e JSON/stream/absence/65-pin assertions. The ledger says “end-to-end test coverage for AC07-3..7” without naming a file, while the envelope warns that files outside the list need an amendment.
**Required change:** Name the file that carries AC07-3..7 (reuse `logging_contract_test.go` and say so, or add a dedicated T7 e2e file), or state that T7 evidence reuses T1/T4 tests by name.

### ADV-9 — Bookkeeping drift: counts, stale assembly note, stale coordinator actions, AC05-6 requirement binding
**Severity:** advisory · confidence HIGH
**Location:** §1 STATUS (“PART 1 of 2”), §6 scope envelope (“Production (20)”, “Tests (60)”), §9 item 11, execution-state AC05-6.
**Evidence:** The file contains §7–§9 although §1 says sections 7–9 are in part 2. The test list enumerates 67 files, not 60 (the production list’s 20 is correct). §9 item 11 says coordinator actions are “required” to amend the state, but the state is already `revision: r5` with the amended effects. State `AC05-6` binds only `R9`, while its body asserts R6’s `diagnostics_incomplete`/`remaining_stages` pair.
**Required change:** Correct the counts/assembly note, mark item 11 as completed or remove it, and bind AC05-6 to `R6` and `R9`.

---

## Requirement / AC / task audit

- **D1–D20 → R1–R19:** complete, with D16/D17/D20 as explicit non-goals (R15/R16/R19). No decision is missing or contradicted, subject to BLK-4 (D14 has a requirement and ACs but no message/operation contract).
- **AC coverage:** every AC01–AC07 has a verification method and owner in the state; every R1–R19 appears as a requirement on at least one AC. Deviation: AC05-6’s state binding omits R6 (ADV-9).
- **Task ledger:** T1→{T2,T3,T4,T6}, T5 after T2+T4, T7 after T3+T4+T5+T6 matches tickets 01–07 exactly; state `blockedBy` matches. T1–T7 effects align with the §6 file lists (N4 resolved), including receipts.
- **N4 write-list check:** state union and spec list agree on production and test paths; the only divergence is the stale counts (ADV-9).
- **Design concreteness:** the envelope/lane/config contracts are largely implementable; the blocking gaps are BLK-1..5 and the advisory omissions above.
- **Observability:** §7 satisfies `skill://observability-contract` for the touched surface — schema, collection path, severity policy, actionable events, bounded labels, Prometheus exception with consequence, retention/shipping exceptions, and AC/T verification tie-ins. The required-fields exception (timestamp/environment/version) is documented with reason and consequence. No unexplained absence found.
- **Critic-handoff risks:** all nine are substantive. Item 1 is now verified correct; item 2 is the real `process_id` judgment (ADV-6); item 3 exposed ADV-1/ADV-7; item 4’s K3 narrowing is a sound documented exception, not an R5 weakening; item 5’s `failf` claim is verified; item 6’s node-capture sites are feasible, with the missing-key exception uncovered (BLK-5); item 7 exposed the upstream plumbing gap (BLK-2); item 8 is corrected and verified; item 9’s B4 retention holds. Item 11 is bookkeeping and partly stale (ADV-9).

## Positive observations

- The r5 changelog is unusually accurate: the corrections it claims (K1 inventory, K2 index map, K3 lane source, K4 lane enumeration, K5 flag behaviour, K6 table, K7 cycle check, K8 test ownership) all check out against the repository.
- The AD2 map is a literal, arithmetically consistent transformation of the current 65-slot list; `service`/`error`/`http_bytes` indices are correct, which removes the highest-risk implementation ambiguity from r4.
- The runtime `ErrorInfo` table is total over `reasonVocabulary` and the outcome-presence rule matches actual producer outcomes, including the `tool_reported_error` and `publication_failed` cases.
- The cycle-free `upstream → service` check is sound: only `cmd/mcp-bridge/wiring.go` imports `internal/upstream`, and `AdvertisedProtocolVersions` returns a copy.
- All 21 files that construct a logger, plus the `LogWriter` fixture, are in T4 effects; the r4 compile-coupling failures are genuinely closed.
- §7 is concise and contract-compliant, with exceptions that state both reason and consequence rather than a bare “N/A”.

**Review limit:** no shell was available, so the execution-state `spec.sha256` binding and the worktree state were not recomputed; the state itself is `revision: r5` with the amended task effects. No source files were modified.

### Verdict
REVISE

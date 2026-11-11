## Specification Review — `structured-logging` r6 (independent)

**Mode:** specification review (pre-implementation)
**Reviewed inputs:** `.scratch/structured-logging/spec.md` (r6), `.scratch/structured-logging/decisions.md` (D1–D20), `.scratch/structured-logging/issues/01..07-*.md`, `.omp/plans/execution-state.json` (read as `revision: r6`), prior reviews r3/r4/r5. No files modified.
**Repository checks:** `internal/observe`, `internal/config`, `internal/service`, `internal/upstream`, `internal/auth`, `cmd/mcp-bridge`, `README.md`, `docs/`.

**Verdict: FAIL (REVISE)** — 5 blocking findings, 8 advisory findings. The r6 fixes for BLK-1, BLK-3, BLK-4, BLK-5 and most of ADV-1/2/5/6/7/8 check out against the repository. Two prior findings remain partially open (BLK-2, ADV-3/4/9), and the task ledger still cannot execute as specified.

---

## Prior-review resolution (independently verified)

| Prior finding | Status | Evidence |
|---|---|---|
| BLK-1 — `weights_invalid` property path | **RESOLVED** | AD10 now sets `Key` per shape (`embedding_negative` → `search.embedding_weight`); `bounds.go:44-58` and `renderRecord` (`diagnostics.go:118-121`) confirm the derivation; AC02-1/AC02-7 assert it. |
| BLK-2 — upstream logger plumbing / nil contract | **PARTIALLY OPEN → BLK-B** | Production plumbing (`wiring.go:137,175`, `serve.go:133`) and the nil→error contract are specified, but direct `NewDedicated` test call sites are missing from T4/scope. |
| BLK-3 — exported vocabulary symbol | **RESOLVED** | `observe.EnvelopeFieldNames()` is named in AD8/design, created in T4, and referenced by AC04-8/AC07-1/AC07-2. |
| BLK-4 — success envelope construction | **RESOLVED** | `config_validation_succeeded` absent from Go tree (grep); AD8/design/T6 specify `eventMessage`, `check_config`, direct builder; AC06 asserts message/operation. |
| BLK-5 — missing-key position exception | **RESOLVED** | `bounds.go:17-35` (`missing` shapes) and `findScalar` (`config.go:1075-1089`) confirm no node; AD10 (d) and AC02-7 assert omission. |
| ADV-1 — union vocabulary | **RESOLVED (residual ADV-B)** | AD8 union and `EnvelopeFieldNames`; seven shared HTTP keys are legal on shared records (`log.go:192-226`, `tools.go` `stage`). |
| ADV-2 — total remediation table | **RESOLVED (residual ADV-G)** | AD8/design/AC02-6 require a total table with named default. |
| ADV-3 — publication-failure branch | **PARTIALLY OPEN → BLK-C/BLK-D** | Envelope is specified, but the branch duplicates the `tools.go` producer and leaves other `operatorLog` producers unmapped. |
| ADV-4 — operation pins | **PARTIALLY OPEN → BLK-E** | Six operations pinned, but `load_config` from decisions/ticket 01 is contradicted. |
| ADV-5 — AD12 field names | **RESOLVED** | `httpdiagnostic_types.go:137-138`; `HTTPProvenance` fields at `:23-24` retained; `provenance.go` not in scope. |
| ADV-6 — `process_id` boundary | **RESOLVED** | AD12/T7/AC07-1 record the boundary; repository `README.md:750-767` already describes an independent per-process token. |
| ADV-7 — `Diagnostic.Shape` loss | **RESOLVED (residual ADV-E)** | `config.shape` added; shapes are load-bearing (`config.go:255,264`; `bounds.go:24,37`). |
| ADV-8 — T7 file for AC07-3..7 | **RESOLVED** | `cmd/mcp-bridge/logging_contract_test.go` named in T7. |
| ADV-9 — bookkeeping | **PARTIALLY OPEN → ADV-F** | Counts are now correct (67/20 verified), but §3 still describes r5 state and AC05-6 as R9-only; §9 item 11 is stale. |
| r3 B1–B10 / r4 K1–K8, N1–N8 | **RESOLVED** except the r4 K1 residual scope gap in BLK-B | Every listed `NewLogger(` caller is in T4 effects; direct `NewDedicated` callers were never inventoried. |

---

## Blocking findings

### BLK-A — T4 cannot satisfy AC04-1/AC04-2..7/AC04-8 in the prescribed order
**Severity:** blocking · confidence HIGH
**Location:** spec §5 AC04-1 (line 171), AC04-2..7 (line 172), AC04-8 (line 173); §6 T4 (line 193), T3 (line 192), T5 (line 194), T6 (line 195); `.omp/plans/execution-state.json` (AC04-8 `ownerTaskIds: ["T4"]`, T4 `blockedBy: ["T1"]`).
**Evidence:**
- AC04-1 requires operation pairs for `startup`/`cli` (T3), `reload`/`publish` (T5), `check_config` success (T6); T4 depends only on T1.
- AC04-8 requires representative records from the **reload renderer** (T5's `operatorLog`, `cmd/mcp-bridge/wiring.go:411-470`) and **CLI** (T3), but T4 runs before both.
- AC04-2..7 requires “full suite passes with no legacy-field assertions remaining.” T4 changes `Entry`/`NewLogger`, which forces `wiring.go`'s `operatorLog` to change (`Entry.Reason` at `wiring.go:427-458`), while `cmd/mcp-bridge/manifest_test.go:33-100` still expects 18 flat fields, two lines, and forbids `error` (line 38), and `cmd/mcp-bridge/serve_edge_test.go:208-211` expects flat `"reason":"internal_error"`. Both are migrated only by T5 (spec T5 line 194). T4's `go test ./...` therefore fails.
- `cmd/mcp-bridge/example_config_test.go:67` success assertion is migrated only by T6, yet T4's full-suite criterion includes it.
**Required change:** Scope AC04-1/AC04-7/AC04-8 to T4-owned producers (observe, service tools/direct execution, upstream handshake, HTTP lane) and move cross-producer operation pairs, reload/CLI/success representative records, and the full-suite criterion to T7. Alternatively add an explicit T4→T5→T7 evidence split and state exactly which AC evidence each task records. Also specify T4's minimal `operatorLog` compile migration versus T5's behavior migration; the current “no T5 renderer-loop change in T4” is incompatible with the `Entry.Reason` cutover.

### BLK-B — `NewDedicated` nil-logger fail-fast breaks four test files outside T4 effects/scope
**Severity:** blocking · confidence HIGH
**Location:** spec §4 BLK-2 changelog (line 7), `internal/upstream/dedicated.go` contract (line 144), T4 (line 193), scope envelope write list (line 201); `.omp/plans/execution-state.json` T4 effects.
**Evidence:** `DedicatedOptions.Logger` replaces `LogWriter`, and `NewDedicated` returns an error when `Logger == nil` (line 144). Direct `NewDedicated` calls without a logger exist in:
- `internal/upstream/dedicated_edge_test.go:56` and `:73`
- `internal/upstream/dedicated_wire_edge_test.go:19`
- `internal/upstream/prompts_edge_test.go:35`
- `internal/upstream/tool_metadata_test.go:56`
- `internal/upstream/dedicated_test.go:65` and `:353`

Only `dedicated_test.go` appears in T4 effects and in the 67-file scope list. The spec's BLK-2 claim (line 7) names only the two **registry** call sites (`embedding_env_durable_test.go:49`, `serve_oauth_test.go:1259`), and §9 item 5 repeats “the two migrated test call sites.” T4's stated verification `go test ./internal/upstream/...` (line 193) will fail on the unlisted files.
**Required change:** Add the four files to T4 `effects.paths`, the §6 T4 file list, and the scope envelope, with a logger supplied at every constructor; or change the nil contract to a concrete two-writer default (an owner decision, not a silent fallback). Extend AC04-8's constructor-error assertion to prove the chosen contract on each path.

### BLK-C — `reload_publication_failed` is emitted twice for one publication failure
**Severity:** blocking · confidence HIGH
**Location:** spec AD7 (line 96), AD8 `tools.go:207` bullet (line 143), wiring design (line 148), AC05-5 (line 174), §7 “No duplicate or summary events” (line 219).
**Evidence:** `internal/service/tools.go:207-216` logs the `ToolSurface.Published` failure. The error then propagates through `Loop` (`internal/service/loop.go:114-115,201-203`) and `Refresh` (`internal/service/refresh.go:212-215`) to `Logf: operatorLog(logger)` (`cmd/mcp-bridge/serve.go:234`), which logs again at `cmd/mcp-bridge/wiring.go:449-458`. The spec explicitly moves **both** producers to `reload_publication_failed` (`tools.go` bullet line 143 and wiring design line 148), so one failure produces two warn records. This violates R9/D10 (“exactly one detailed event”) and §7/observability-contract (“Never accept duplicate or noisy events”). AC05-5 only calls `operatorLog` directly (`serve_edge_test.go:208-229`), so no AC catches the duplicate.
**Required change:** Designate exactly one emitter — remove the `tools.go:207` log and let `operatorLog` carry the typed publication record, or suppress `operatorLog` when the wrapped error already carries a typed publication record. Add an AC that exercises the full serve reload and refresh path and asserts exactly one `reload_publication_failed` record.

### BLK-D — `operatorLog`’s remaining production paths have no specified envelope
**Severity:** blocking · confidence HIGH
**Location:** spec AD7 (line 96), wiring design (line 148), §7 actionable events (line 219), T5 (line 194); repository paths below.
**Evidence:** `operatorLog` is reached by more producers than AD7/AC05 define:
- `internal/service/loop.go:146` — `startup_snapshot_rejected: %v`.
- `internal/service/prompts.go:125-127` — `prompt publication rejected: ...`, passed through `cmd/mcp-bridge/serve.go:129`.
- `internal/service/loop.go:204` — any non-publication reload error.

Today all three fall into the fallback entry at `wiring.go:449-458` (`Event: "reload"`, `Reason: "invalid_config"`, no `Operation`). After `Entry.Reason` and the flat manifest are removed, this path cannot compile/pass unchanged, yet AD7 defines only `config_reload_rejected` and `reload_publication_failed`. No event/operation/code/kind/remediation is specified for these records, contrary to R1 (“every application log record”) and R11. `loop.go` and `prompts.go` are not in the write scope.
**Required change:** Define typed mappings for snapshot rejection, prompt publication rejection, and generic reload errors (event, operation, code, kind, retryable, static remediation, message source), add them to §7’s actionable events, and add AC coverage. Either route them through explicit typed producers at `serve.go`/`wiring.go` or specify the classification rule `operatorLog` must apply.

### BLK-E — The pinned operation vocabulary contradicts the approved ticket and the decisions example
**Severity:** blocking · confidence HIGH
**Location:** spec AD5 (line 83), AD7 (line 96), AD8 Operations (line 106), AC04-1 (line 171); `decisions.md:47`; `issues/01-json-error-envelope-and-pin-ceiling-attribution.md:18`.
**Evidence:** The owner decisions’ canonical envelope and ticket 01’s “envelope the contract every later ticket extends” both use `"operation": "load_config"` for the pin-ceiling `config_validation_failed` record. The spec pins `startup` for the serve-side `config_validation_failed` (AD5/AD7) and AC04-1 asserts `startup`; `load_config` appears nowhere in the spec. The spec simultaneously claims the ticket mapping is 1:1 (lines 156, 188). Ticket 01’s ACs do not assert operation, so T1 can emit either value, but T4’s AC04-1 will reject the ticket’s example.
**Required change:** Reconcile explicitly. Either adopt `load_config` for the pin-ceiling/config-validation event and add it to the operation pins/ACs, or record an owner-confirmed supersession of the ticket/decisions example (decisions are owner-settled) and update the ticket reference. Do not leave two normative values for a required field.

---

## Advisory findings

### ADV-A — Parallel-writer hazards are not closed by task edges
**Severity:** advisory · confidence HIGH
**Location:** spec §6 dependency statement (line 188) and task file lists; state `blockedBy`.
**Evidence:** T3 and T4 both depend only on T1 yet both write `cmd/mcp-bridge/serve.go`, `cmd/mcp-bridge/serve_edge_test.go`, and `cmd/mcp-bridge/runtime_edge_test.go`. T3 and T6 both write `cmd/mcp-bridge/main.go`. T4 and T6 both write `serve_edge_test.go`. The ledger says files are edited “in sequence” but provides no edges.
**Required change:** Add ordering edges or state that these tasks are dispatched serially; otherwise two concurrent engineers can conflict in the same file.

### ADV-B — `EnvelopeFieldNames()` exact contents and equality are under-specified
**Severity:** advisory · confidence HIGH
**Location:** spec AD8 union (line 99), BLK-3/design (lines 8, 136), AC04-8 (line 173), AC07-1 (line 177).
**Evidence:** AD8’s union does not mention nested `config`/`resource` subkeys or `config.related` subkeys, while the design/BLK-3 say `EnvelopeFieldNames()` includes them. AC04-8 asserts only subset membership; AC07-1 asserts docs equality to the same function, so the function is its own oracle.
**Required change:** Publish the exact expected sorted set (or derivation) and make a test assert **equality** against an independent expected list, including nested `error`, `config`, `config.related`, and `resource` subkeys.

### ADV-C — Required `level`/`outcome` values for new events are not pinned
**Severity:** advisory · confidence HIGH
**Location:** spec AD5 (line 83), AD7 (line 96), AD8 message bullet (line 104), AC03-1 (line 166), AC06-3 (line 176), R2/R4.
**Evidence:** The `startupFailure` value lists `(event, operation, code, kind, retryable, remediation, resource map)` but no `level` or `outcome`; AC03-1 nevertheless asserts `outcome "failure"`. The success record’s `level` is never stated, though AC06-3 requires stdout and R4 routes by level.
**Required change:** Pin `level`/`outcome` for `startup_failed`, `cli_usage_failed`, and `config_validation_succeeded` (e.g. error/failure and info/success), and include them in the `startupFailure`/builder contract.

### ADV-D — Startup publication failure is mislabeled `reload_publication_failed`
**Severity:** advisory · confidence HIGH
**Location:** spec AD7 (line 96), AD8 `tools.go` bullet (line 143), AC05-5 (line 174).
**Evidence:** `Loop.Start` calls `Published` at `internal/service/loop.go:160-162`; the same `tools.go:207` producer and `operatorLog` path then report a **startup** publication failure with a reload-named event. AD5 does not classify startup publication failure; `serve` returns `ErrPublicationFailed`, and T3's fallback would emit `startup_failed` as well.
**Required change:** Define startup publication handling separately (event/operation/code) or document why the reload event covers both, and add AC coverage.

### ADV-E — “No other data is lost” remains inaccurate
**Severity:** advisory · confidence HIGH
**Location:** spec AD8 data-loss list (line 103); `internal/config/diagnostics.go:70,145-146`; `internal/service/tools.go:207-216`.
**Evidence:** `Diagnostic.Hint` exists and is rendered in `renderRecord` (`diagnostics.go:145-146`) but is absent from the new renderer mapping and from the AD8 list (no producer sets it today, but the claim is still false). The typed `Reason: "publication_failed"` at `tools.go:207-216` is replaced by `internal_error` per AD7, another unreported loss.
**Required change:** Preserve `Hint` (optional `config.hint`) or document the intentional loss; record the `publication_failed` → `internal_error` semantic replacement in the data-loss list.

### ADV-F — Stale bookkeeping and citation drift
**Severity:** advisory · confidence HIGH
**Location:** spec §3 state bullet (line 67), §3 UNKNOWN (“part 2”), §9 item 11 (line 243), BLK-1 citation (line 6).
**Evidence:** §3 says the state is r5 and AC05-6 binds only R9; the actual state is `revision: "r6"` and AC05-6 is `["R6","R9"]`. §9 item 11 still presents coordinator actions as required although the state is already r6. The `renderRecord` `key=%q` citation is `diagnostics.go:120-121`, not `:116-117`.
**Required change:** Update §3/§9 to the actual bound r6 state, remove the completed coordinator actions and “part 2” reference, and correct the line citation.

### ADV-G — The total remediation test has no enumeration source
**Severity:** advisory · confidence MEDIUM
**Location:** spec AC02-6 (line 165), design renderer (line 145).
**Evidence:** `Diagnostic.Code` values are scattered string literals across six files; no exported registry or list exists. A test that “iterates every `Diagnostic.Code` value produced by `internal/config`” must either hard-code a list or exercise every validator; the spec does not define the source, so a new code can silently fall through to the default.
**Required change:** Define a single code registry/constant list (or require the test to enumerate the known code set) and assert the remediation table is total over it.

### ADV-H — Non-conflict rejection positions are not designed per class
**Severity:** advisory · confidence MEDIUM
**Location:** spec R6 (line 38), AC02-1 (line 161), AD10 (lines 111-118), T2 (line 191).
**Evidence:** AC02-1 requires own positions for every rejection class when a source node exists, but AD10 specifies node capture only for the six conflict sites, weights, and the missing-key exception. Classes such as `server_id_invalid`, `server_count`, `schema_version`, `url_invalid`, `pin_fields`, `pin_tool_name`, `reference_unresolved`, secret classes, and bounds have nodes in `root` but no explicit node-source rule in the design.
**Required change:** Add a per-class node-source table (or extend the exception list) so T2 does not have to invent the mapping; assert the populated positions in the class tests.

### ADV-I — AC05-5’s publication test cannot detect the full-path duplicate
**Severity:** advisory · confidence HIGH
**Location:** spec AC05-5 (line 174); `cmd/mcp-bridge/serve_edge_test.go:208-229`.
**Evidence:** The migrated test calls `operatorLog` directly, so it verifies one invocation, not the `tools.go` + loop/refresh chain that produces the duplicate in BLK-C.
**Required change:** Add a full-path assertion (drive a reload/refresh publication failure through `Loop`/serve) that counts the emitted `reload_publication_failed` records.

---

## Positive observations

- **AD2 is arithmetically and structurally exact.** `slotKeys` (`internal/observe/httpdiagnostic_encode.go:66-84`) verifies 65 slots; removing indexes 0/5/6/10/20/21 and prepending `schema_version` yields the published 60-key map with `service`=4, `error`=55, `http_bytes`=58, `exposed_tools`=59. The AD2 emission rule matches `deliveryErrorClass`/slot 60 today.
- **BLK-1/BLK-5 fixes are concrete and repository-grounded.** `bounds.go:17-58`, `findScalar` (`config.go:1075-1089`), and `renderRecord` (`diagnostics.go:118-121`) support the per-shape `Key` and the missing-key exception.
- **The runtime `ErrorInfo` table is total.** All 27 non-`none` values in `reasonVocabulary` (`log.go:262-267`) map exactly once; `none` → nil.
- **AD11 is sound.** `service.AdvertisedProtocolVersions()` is exported (`protocol.go:15-17`), the set is four dates plus `2026-07-28`, and `internal/upstream → internal/service` is cycle-free.
- **AD12’s deletion targets are named precisely** (`httpdiagnostic_types.go:137-138`) and `HTTPProvenance` (`:23-24`) remains for `provenance.go`/`resolveProvenance`.
- **The 67 test / 20 production counts are correct**, and the README stale anchors (`:644-711`, `:757-767`, `:804-805`) are real.
- **§7 satisfies `skill://observability-contract`** for the touched surface: schema, collection path, severity policy, actionable events, bounded labels, Prometheus exception, retention/shipping exceptions, and measurable AC/T tie-ins, each with reason and consequence.
- **The nine critic-handoff risks are substantive.** Items 1–5, 8, and 9 map to verified repository facts; items 5 and 11 exposed the residual upstream-scope and bookkeeping gaps above.

**Review limit:** no shell was available, so the execution-state `spec.sha256`/section hashes and worktree state were not recomputed; the state was read as `revision: r6` with the bindings shown. No source files were modified.

### Verdict
REVISE

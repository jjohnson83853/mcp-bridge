## Specification Review — `structured-logging` r4 (independent)

**Mode:** specification review (pre‑implementation, bounded to the approved contract)
**Reviewed inputs:** `.scratch/structured-logging/spec.md` (r4), `decisions.md` (D1–D20), `issues/01..07-*.md`, `.omp/plans/execution-state.json` (r4 bindings)
**Repository checks:** performed against `internal/observe`, `internal/config`, `internal/service`, `internal/upstream`, `cmd/mcp-bridge`, `README.md`, and the Go 1.24 `flag` package source. No files modified.

**Verdict: FAIL (REVISE)** — 8 blocking findings, 7 advisory findings.

The r4 amendment genuinely resolves B3, B4, B6, B8, B9, B10, A1, A2, A4, A5, A6, and most of A7. The blocking problems are structural: the task ledger still cannot compile/pass in several tasks, AD2 contains arithmetic and contract contradictions, the “closed vocabulary” does not cover the HTTP lane, and two CLI/error paths still violate D1/D5/D6.

---

## Prior-review resolution

| Prior finding | Status | Evidence |
|---|---|---|
| B1 — scope excludes producers/adapters/callers | **STILL OPEN (partial)** | Expanded, but `httpwriter_test.go`, `dedicated_test.go`, `example_config_test.go`, and T1/T3/T6-owned assertions in `serve_edge_test.go` / `runtime_edge_test.go` remain outside their owning task effects. See **K1**. |
| B2 — AD2 slot arithmetic/field set | **STILL OPEN (partial)** | Count 62 is correct, but `service now 3` and `slot 60 error keeps its position` are false. See **K2**; error-object contract also remains contradictory (**K3**). |
| B3 — pin-ceiling identity/precedence | **RESOLVED** | AD9 + AC01-7/AC01-8; both ACs bound to T1 in execution state. |
| B4 — `renderDiagnostics`/`Error()` | **RESOLVED** | Retained permanently; T2 effects include the `requireRendered`/`Error()` tests; no production print site survives T3/T5. |
| B5 — R2 vs AD8 field vocabulary | **STILL OPEN** | AD8 enumerates only seven HTTP-lane fields; the lane emits 40+ more. See **K4**. |
| B6 — startup config rejection ownership | **RESOLVED** | T3 owns `main.go` `errors.As` branch; AC03-7 bound to T3. |
| B7 — R6 conflict attribution | **PARTIAL** | Six sites and exceptions named; T2 still lacks authorization for the required renderer test; weights attribution contradicts “offending node”. See **K8**, **N1**. |
| B8 — R9 multi-record selection | **RESOLVED** | AC05-6 bound to T5. |
| B9 — README/docs migration | **RESOLVED** | AC07-8 bound to T7; `README.md` in T7 effects. |
| B10 — doc AC method mismatch | **RESOLVED** | AC07-1/AC07-2 are `method: test` in execution state; mechanical doc tests specified. |
| A1 — `request_id` preservation | **RESOLVED** | AC04-1 amended; §7 names the assertion. |
| A2 — static reason in `message` | **RESOLVED** | Renderer sets `message` = first record’s `Reason`; AC02-6 amended. |
| A3 — `--help` stream | **NOT RESOLVED** | AD6’s custom `Usage` prints usage on *every* parse error, not just `--help`. See **K5**. |
| A4 — per-site AD5 table | **RESOLVED** | Complete site → code/kind/retryable/remediation table. |
| A5 — R15/R16/R19 acceptance | **RESOLVED** | AC07-9 bound as inspection. |
| A6 — T7 decisions append | **RESOLVED** | Removed from T7. |
| A7 — position exceptions | **PARTIAL** | Exceptions documented and AC02-7 asserts them; renderer-test authorization and weights wording remain (**K8**, **N1**). |

---

## Blocking findings

### K1 — The scope ledger still leaves required repository files unauthorized; four task verifications cannot pass
**Severity:** blocking
**Location:** `.scratch/structured-logging/spec.md` §6 T1/T3/T4/T6 and §9 item 8; `.omp/plans/execution-state.json` task effects for T1/T2/T3/T4/T5/T6.
**Evidence:**
- `internal/observe/httpwriter_test.go:756` calls `NewLogger(&bytes.Buffer{})`. AD1 changes the signature to `NewLogger(stdout, stderr)`. This file is in **no** task’s `effects.paths` and is not in the §6 write list; T4 cannot compile the package.
- `internal/upstream/dedicated_test.go:355` sets `LogWriter: buf`. §4 replaces the raw `LogWriter` with an owned `*observe.Logger`; this test file is in no task’s effects and is not in §6, so `go test ./internal/upstream/...` cannot compile.
- `cmd/mcp-bridge/example_config_test.go:67` asserts the exact plaintext `configuration valid: Config{...}` and `:102` asserts `invalid configuration: ...`. T1 changes the failure path and T6 the success path; the file is owned by **no** task.
- `cmd/mcp-bridge/serve_edge_test.go:42` asserts `configuration valid:` and `:65-66` asserts the `invalid configuration:` prefix. T1/T6 must change these, but the file is only in T4/T5 effects. T1’s verify `go test ./cmd/mcp-bridge -run CheckConfig` matches `TestRunCheckConfigAndMissingFile` and fails; T6’s full `./cmd/mcp-bridge/...` fails.
- `cmd/mcp-bridge/runtime_edge_test.go:18-21` asserts flag exit code `2` and `flag provided but not defined`. T3 (AD6) changes exit to 1 and discards that message; the file is only in T4 effects. T3’s verify regex `'Startup|Usage|Help|RunContext'` matches `TestRunFlagParseErrorWritesUsage` and cannot pass.
- `cmd/mcp-bridge/serve_edge_test.go:132-164` asserts `serve` error prose (`state directory`, `embedding provider`, `inbound listener`). T3 returns `startupFailure` from those sites but is not authorized to edit this file, and the spec does not require `startupFailure.Error()` to preserve those phrases.
- §9 item 8 claims “no repository script depends on usage exit 2” and “only the unrelated test-child guard”; `runtime_edge_test.go:20` is a direct counterexample.
**Required change:** Add the affected files to the owning task effects and §6: `runtime_edge_test.go` → T3; `example_config_test.go` and the check-config assertions in `serve_edge_test.go` → T1/T6; `httpwriter_test.go` and `dedicated_test.go` → T4 (or specify a compatibility constructor/arity that avoids touching them). Correct §9 item 8.

### K2 — AD2’s post-change slot indexes are arithmetically wrong
**Severity:** blocking
**Location:** §4 AD2; §9 item 1; §4 `httpdiagnostic_encode.go` contract.
**Evidence:** `internal/observe/httpdiagnostic_encode.go:64-84` defines `slotKeys` with `time`=0, `level`=1, `service`=4, `environment`=5, `version`=6, flat `reason`=10, `error`=60. Removing 0/5/6/10 and inserting `schema_version` immediately after `level` yields **`service` at index 4**, not 3, and **`error` at index 57**, not 60 (index 60 becomes `http_bytes`). The spec itself says “slot indexes after 0/5/6/10 shift”, contradicting “Slot 60 `error` keeps its position”.
**Impact:** AD2 claims to publish the EXACT final list; a literal implementation places the nested error over `http_bytes` or produces a different table than the budget/encoder tests expect.
**Required change:** Publish the exact new index map (`service`=4, `error`=57) and correct §9 item 1 and T4; or explicitly define a different insertion rule and recompute all constants.

### K3 — The HTTP-lane error object is both contradictory and unsourced after `Entry.Reason` removal
**Severity:** blocking
**Location:** §4 AD2, §4 AD8, §5 AC04-3, §5 AC04-2.
**Evidence:**
- `internal/observe/httpdiagnostic_encode.go:193-201, 505-520` uses `entry.Reason` for slot 10 and slot 60; the error slot is emitted only when correlation is unavailable with `outcome == failure`, or `deliveryErrorClass(entry.Reason)` ∈ `client_disconnected|unknown`. A 401 `client_auth_failed` failure therefore carries **no** error object.
- AD2 states “No HTTP failure record is ever emitted without its `error` object”, and AC04-3 says failure records carry the nested object. The closed table named in AD2 covers only correlation/delivery classes and does not give `code` values.
- AD8 removes `Entry.Reason`. The design does not say where the lane’s class input comes from; `legacyEntry.Reason` is not carried on the reshaped `Entry`, and `build(entry Entry)` would have no source.
**Required change:** Specify the post-migration source of the lane’s error class and a complete failure-reason → `{code, kind, retryable, remediation}` table (including 4xx auth/protocol failures), or explicitly narrow AD2/AC04-3 to delivery/correlation records.

### K4 — R2/AD8’s “closed” vocabulary does not cover the HTTP lane or the `config` object; `diag_environment` violates D3/R3
**Severity:** blocking
**Location:** §2 R2, R3; §4 AD8; §5 AC04-8; §7 required-fields exception.
**Evidence:**
- `slotKeys` contains 65 keys; after AD2 62 remain. AD8 enumerates only `http_status`, `http_method`, `route`, `http_outcome`, `stage`, `timing_scope`, `http_bytes_bucket`. Unlisted emitted keys include `diag_schema`, `diag_environment`, `source_revision`, `build_state`, `process_state`, `process_id`, `operation_relation`, `operation_id_state`, `path_class`, `route_outcome`, `status_state`, `diag_duration_ms`, `duration_capped`, `failure_stage`, `failure_reason`, `auth_mode`, `credential_state`, `scope_state`, `session_header`, `session_state`, `binding_relation`, `session_ref_state`, `challenge_kind`, `challenge_target`, `metadata_target_registered`, `diagnostic_policy_revision`, `effective_policy_revision_state`, `effective_policy_revision`, `policy_source`, `oauth_enabled`, `static_configured`, `scope_required`, `resource_path_mode`, `session_binding`, `clock_skew_seconds`, `upstream_state`, `error_detail_state`, `error_detail`, `operation_id`, `session_ref`, `client_family`, `http_bytes`, `exposed_tools`.
- AC04-8’s producer list (“observe, service tools/direct execution, upstream handshake, reload renderer, CLI”) does not explicitly include the HTTP lane, so R2’s “no key outside that enumeration” is either false or unverified.
- `diag_environment` carries environment identity (`internal/observe/httpdiagnostic_types.go:137`; `httpdiagnostics.go:762`), and `provenance_test.go:120` asserts `lab-1`. D3/R3 say the application never emits environment; §7’s exception claims environment is collector-owned.
- The shared renderer adds `config.diagnostics_incomplete` / `config.remaining_stages` (AD4) although R6’s `config` key list does not include them.
**Required change:** Enumerate the full lane and `config` object vocabulary in AD8/R6, or record an explicit owner-approved exception for the lane and extend AC04-8 to it. Remove `diag_environment` (and decide `source_revision`/`build_state`) or obtain an owner exception.

### K5 — AD6’s custom `Usage` prints plaintext usage on every parse error, violating R1/R4
**Severity:** blocking
**Location:** §4 AD6; §4 `main.go` contract; §5 AC03-1..6; R1/R4.
**Evidence:** Go 1.24 `flag.FlagSet.failf` calls `f.usage()` (`/usr/share/go-1.24/src/flag/flag.go:1058-1061`), and unknown flags route through `failf` (`:1115`). With a custom `Usage` writing to stderr, every unknown-flag/missing-value parse error prints the usage text **and** the `cli_usage_failed` envelope. AD6 claims `SetOutput(io.Discard)` “silences only the flag package's automatic parse-error messages”, but the usage text is not silenced. R1 says no mixed text/JSON on any process path; R12 only exempts `--help`.
**Required change:** Install no custom `Usage` that fires on parse errors; print the unchanged usage text only in the `flag.ErrHelp` branch (e.g., render it into a buffer or call a help-only path), and emit only the envelope for other parse errors. Add AC coverage that stderr for an unknown flag contains exactly one JSON object and no `Usage of mcp-bridge:` text.

### K6 — Runtime failure records have no specified `ErrorInfo` mapping
**Severity:** blocking
**Location:** §4 AD8 (`ErrorInfo` replaces flat reason/class); §5 AC04-3; §2 R5/R11.
**Evidence:** Producers pass `Reason: dispatchErr.Code` with values from the closed `reasonVocabulary` (`internal/service/tools.go:816-847, :827-848, :885-934`; `internal/service/direct_execution.go:72-160`), including `backend_unavailable`, `backend_busy`, `deadline_exceeded`, `invalid_arguments`, `tool_reported_error`. After `Entry.Reason` is removed, these sites must construct `ErrorInfo`, but AD5 enumerates only startup/CLI sites. No runtime reason → `{code, kind, retryable, static remediation}` table exists, no rule says which `outcome` values require `error`, and `tools.go:207-216` (`publication_failed`) is not assigned an event/error mapping despite AD7’s `reload_publication_failed` claim. AC04-3 requires the nested object on failure records, and the …
**Required change:** Add a closed runtime reason → `ErrorInfo` table (and the outcome→error presence rule), or narrow AC04-3/R5 to the enumerated configuration/startup/CLI sites and document the exception. Include `tools.go`’s publication failure explicitly.

### K7 — `protocol_version` is upstream-controlled and its “finite” validation is not specified
**Severity:** blocking
**Location:** §4 AD8 (“New: `backend_id` …, `protocol_version` (finite MCP version string)”), §9 item 7.
**Evidence:** `internal/upstream/dedicated.go:244-252` takes `res.ProtocolVersion` from the upstream initialize result and encodes it. The only closed version list is unexported `service.protocolVersions` (`internal/service/protocol.go:12`), and no allowed set or invalid-value behavior is named for the envelope. An upstream returning an arbitrary string below `"2026-07-28"` would be emitted, contrary to R8; no AC covers it.
**Required change:** Name the closed protocol-version set (and where it lives, avoiding import cycles) and the invalid-value behavior (omit/suppress); add an AC with a hostile `ProtocolVersion`.

### K8 — T2 is not authorized to write the renderer test its ACs require
**Severity:** blocking
**Location:** §5 AC02-6/AC02-7; §6 T2; execution-state T2 effects.
**Evidence:** AC02-6/AC02-7 require `message`/omission renderer assertions; the renderer lives in `cmd/mcp-bridge/diagnostics_envelope.go`. T2 effects include that production file but **no** `cmd/mcp-bridge` test file. `internal/config` tests cannot exercise a `package main` renderer. The §6 envelope says any file outside the list requires a planner amendment.
**Required change:** Add a T2-authorized `cmd/mcp-bridge` test file (e.g., `config_envelope_test.go` or a T2-specific file) to §6 and the execution state, or move the renderer test into an already-authorized T2 file.

---

## Advisory findings

### N1 — `weights_invalid` attribution contradicts “offending node”
**Severity:** advisory (confidence HIGH)
**Location:** §4 AD10, §5 AC02-1/AC02-7; `internal/config/bounds.go:56-61`.
**Evidence:** AD10 fixes own `Line`/`Column` to the `keyword_weight` node and `Related` to `embedding_weight` “for every shape”. For shape `embedding_negative`, the offending node is `embedding_weight`, while AC02-7 says own `line`/`column` are “of the offending node”. The record’s `Key` is always `search.keyword_weight`.
**Fix:** Either define own = the record’s `Key` node for all shapes and reword ACs, or map own per shape (keyword for `keyword_nonpositive`, embedding for `embedding_negative`, keyword with embedding related for `sum_not_one`).

### N2 — `message` presence/value is not acceptance-covered for non-config records; §3’s “per-event message” claim is inaccurate
**Severity:** advisory (confidence HIGH)
**Location:** §2 R2; §3 evidence; §4 `envelope.go`; §5 AC04-1; `internal/observe/log.go:342-350`.
**Evidence:** AC04-1 asserts `schema_version/level/service/event/operation/outcome`, not `message`. `eventMessage` returns `"mcp-bridge event"` for every event except `http_complete` (and the fixed message is not per-event). The builder’s required-field list omits `message`.
**Fix:** Extend AC04-1 to assert non-empty `message` for every record and extend `eventMessage` for new events, or correct §3’s wording.

### N3 — `error.remediation` is required by R5 but not asserted
**Severity:** advisory (confidence HIGH)
**Location:** §2 R5; §5 AC04-3; AD5/AD2 tables.
**Evidence:** AC04-3 asserts only `code`, `kind`, `retryable`.
**Fix:** Add `error.remediation` presence/static-value assertions to AC04-3 and the per-site tests.

### N4 — §6 “final exact” write list is stale relative to the bound execution state
**Severity:** advisory (confidence HIGH)
**Location:** §6 Scope envelope; execution-state T4/T7 effects.
**Evidence:** State T4 authorizes `embedding_env_reload_test.go`, `serve_logging_*.go`, `serve_test.go`, `runtime_edge_test.go`, etc., which §6 does not list; state T7 authorizes `CHANGELOG.md`, absent from §6 T7. Conversely, §6 says any file outside its list needs an amendment while the state already binds a different list.
**Fix:** Update §6 to mirror the bound state or state that task `effects.paths` are authoritative.

### N5 — Method labels disagree between §5 and the execution state
**Severity:** advisory (confidence HIGH)
**Location:** §5 AC07-3..7 / AC07-8..9; execution-state acceptance.
**Evidence:** §5 marks AC07-3..7 as “T + R” while the state binds them `method: test`; §5 marks AC07-8/9 as `R` while the state binds `inspection`. The semantics are close but not identical.
**Fix:** Align §5’s method notation with the state’s `test`/`inspection` evidence kinds.

### N6 — R18’s evolution rule is not mechanically asserted
**Severity:** advisory (confidence MEDIUM)
**Location:** §2 R18; §5 AC07-1.
**Evidence:** AC07-1’s described test extracts the field list and compares it with the vocabulary; it does not assert the evolution-rule statement.
**Fix:** Have the T7 doc test assert the evolution-rule text/phrase, or add an inspection item.

### N7 — §7’s “one alert-ready example per severity class” is not covered by AC07-2
**Severity:** advisory (confidence MEDIUM)
**Location:** §7 Operations; §5 AC07-2.
**Evidence:** AC07-2 asserts examples for `event`, `error.code`, `error.kind` exist and use vocabulary fields; it does not assert a config-rejection and a startup-failure example.
**Fix:** Extend AC07-2 or add a T7 inspection item naming the per-class examples.

### N8 — AD5 does not assign `event`/`operation` values to the startup failure sites
**Severity:** advisory (confidence MEDIUM)
**Location:** §4 AD5; §5 AC03-1/AC03-2; §7 actionable events.
**Evidence:** `startupFailure` carries event/operation, §7 names `startup_failed`, but the per-site table lists only code/kind/retryable/remediation; AC03 does not assert the event.
**Fix:** Add event/operation per site (or state a single `startup_failed` event and operation) and extend AC03-1 to assert it.

---

## Positive observations and verified claims

- **D1–D20 → R1–R19 mapping is complete**, with D16/D17/D20 as explicit non-goals; ticket edges exactly match the task graph (T1 → T2/T3/T4/T6; T5 after T2+T4; T7 last).
- **Execution-state binding is current for r4:** AC01-7/8→T1, AC02-7→T2, AC03-7→T3, AC04-8→T4, AC05-6→T5, AC07-8/9→T7; AC07-1/2 are `method: test`.
- **Verified §3 anchors:** `Entry`/`NewLogger` (`log.go:13-36`), legacy map and whitelist (`:74-215`), `numHTTPSlots=65` and `slotKeys` (`httpdiagnostic_encode.go:55-84`), error slot 60 (`:505-520`), `observeEntry` (`httpdiagnostics.go:430-443`), `main.go` plaintext paths (`:30-58`), `serve.go` sites and `restart_required` (`:50-135, :236-245`), `operatorLog`/18 fields (`wiring.go:411-470`), ceiling before loop and six conflict sites (`config.go:180-281, :681-686, :991-995, :385-404`), `refOccurrence` (`:1037-1040`), `weights_invalid` (`bounds.go:44-61`), accessor diagnostics (`command.go:157-172`, `credentials.go:44-52`), and README lines `278-306, 551-563, 629-645`.
- **B3/AD9 and B4** are correctly designed: validated-or-omitted excess identity with a deterministic tie rule, and permanent `(*Diagnostics).Error()` retention with no production print path after T3/T5.
- **§7 is largely observability-contract compliant:** collection path, shipping exception, required-field exception, bounded labels, no-new-metrics exception, stream policy, actionable events, and AC/T tie-ins are present with reasons/consequences; the gaps are `diag_environment` (K4) and per-severity alert examples (N7).
- **The nine critic-handoff risks are substantive, not decoration:** items 1, 3, 7, and 8 exposed real defects (K2, K4, K7, K1); items 2, 4, 5, 6 remain valid review targets.

---

### Verdict
REVISE

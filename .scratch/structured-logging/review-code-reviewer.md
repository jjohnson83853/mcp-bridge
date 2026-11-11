## Specification Review — `structured-logging` r3

**Mode:** specification review (pre-implementation, bounded to the approved contract)
**Reviewed inputs:** `.scratch/structured-logging/spec.md` (r3), `decisions.md` (D1–D20), issues 01–07, `.omp/plans/execution-state.json`
**Files modified:** none

**Verdict: FAIL (REVISE)** — 10 blocking findings, 6 advisory findings.

The r3 amendment preserves requirement/AC/task content, and the D→R and ticket→task mappings are mostly correct. The blocking problems are not style issues: the bound scope cannot compile or satisfy R1/R2/R5, AD2's HTTP-lane arithmetic contradicts the repository, and one requirement (R6 related attribution) is not acceptance-covered.

---

## Blocking findings

### B1 — The scope envelope excludes required log producers, adapters, and callers; R1 and AC04-7 cannot be met
**Spec:** §2 R1/R2/R5; §4 AD1 and AD8; §6 T4 and “Scope envelope” read/write paths.
**Repository evidence:**
- `internal/service/tools.go:650-703` and `internal/service/direct_execution.go:72-160` construct `observe.Entry{Reason: ..., Fields: ...}` records. AD8 removes `Entry.Reason`, so these stop compiling.
- `internal/upstream/dedicated.go:247-254` writes a separate JSON record directly to a writer defaulting to `os.Stdout` (`dedicated.go:50-53`): `{"level":"info","event":"upstream_ephemeral_handshake","backend_id":...,"duration_ms":...,"protocol_version":...}`. This is an application log record with no envelope and fields (`backend_id`, `protocol_version`) not permitted by R2.
- `internal/observe/httpdiagnostics.go:430-434` converts `legacyEntry` into `Entry{Time: entry.Time, ..., Reason: ...}`; T4's write list omits this production file.
- `internal/auth/middleware_test.go:365,402,472,522` call `observe.NewLogger(&buf)`; AD1 changes the signature to `NewLogger(stdout, stderr)`, so the full suite fails to compile.
- The bound execution-state `readPaths`/`writePaths` include only `internal/observe/`, `internal/config/`, and `cmd/mcp-bridge/`; `internal/service/**`, `internal/upstream/**`, and `internal/auth/**` are outside the envelope. The spec's §6 write list likewise omits `internal/observe/httpdiagnostics.go`.

**Impact:** T4 cannot leave the tree compiling, cannot migrate all producers, and cannot satisfy R1 (“every application log record”) or AC04-7. The `upstream_ephemeral_handshake` record would remain a second schema.
**Required change:** Expand the read/write envelope and T4/T5 ownership to cover `internal/service/**`, `internal/upstream/dedicated.go`, `internal/observe/httpdiagnostics.go`, and affected `internal/auth` tests; define the envelope mapping (including `backend_id`/`protocol_version`) or explicitly preserve a compatibility constructor.

### B2 — AD2's HTTP-lane slot arithmetic and field set contradict the repository and AC04
**Spec:** §4 AD2; §2 R3/R5/R17; §5 AC04-2/AC04-3/AC04-5.
**Repository evidence:**
- `internal/observe/httpdiagnostic_encode.go:55-84`: `numHTTPSlots = 65`; `slotKeys` already contains `"service"` at index 4, `"time"` at 0, `"environment"` at 5, and `"version"` at 6, plus flat `"reason"` at 10 and `"error"` at 60.
- `httpdiagnostic_encode.go:508-519`: the lane's `error` slot is emitted as `{"class":"..."}` (`slotErrorClass`), not `{code, kind, retryable, remediation}`.

**Impact:** “add `schema_version` and `service` (65→67)” is impossible: `service` already exists, and dropping timestamp/reason cannot increase the count. Removing the class slot leaves HTTP failure records with no `error` object, conflicting with AC04-3; retaining the class object conflicts with R5/R17. `environment` and `version` remain in the lane, directly violating R3 and AC04-2.
**Required change:** Publish the exact post-change slot list and count, remove `environment`/`version` (and `time`), and specify the HTTP lane's nested error fields (`code`, `kind`, `retryable`, `remediation`) or an explicit exception. Recompute the budget arithmetic against the final list.

### B3 — Pin-ceiling identity is unvalidated, and ceiling precedence violates R9's first-error rule
**Spec:** §2 R7/R8/R9; §4 pin-ceiling contract; §5 AC01-2/AC01-3.
**Repository evidence:** `internal/config/config.go:210-216` checks `len(cfg.Pins) > 64` before the per-pin validation loop at `218-281`. `Pins` is `[]map[string]string` (`config.go:65`), so `cfg.Pins[64]["server"]`/`["tool"]` are arbitrary YAML strings at that point. A malformed pin at index 0 with 65 total entries yields `pin_ceiling` even though the first document-order error is at index 0.
**Impact:** The design would emit unvalidated attacker/operator-controlled strings as “safe” `pin_server`/`pin_tool`, violating R8 and AC01-3; the first-error requirement R9 is also violated for mixed failures. The 65-valid-pins fixture does not catch either case.
**Required change:** Define and implement validation/omission of the excess entry's identity, define ceiling-vs-entry error precedence under R9, and add tests with (a) a malicious/malformed `pins[64]` identity and (b) a malformed `pins[0]` plus 65 entries.

### B4 — Deleting `renderDiagnostics` removes the only implementation of `(*Diagnostics).Error()`
**Spec:** §4 diagnostics.go contract; §6 T2/T4; R17; AC04-7.
**Repository evidence:** `internal/config/diagnostics.go:90-92` implements `Error()` as `renderDiagnostics(d)`. `*Diagnostics` is returned as `error` throughout `Parse`/`ParseFile` and is required to implement `error`. `parser_edges_test.go:27-45` defines `requireRendered` over `err.Error()`, and many config tests use it (e.g. `api_edges_test.go`, `config_test.go`, `diagnostics_test.go`, `validate_edge_test.go`, `secret_validation_edge_test.go`). T2 names only four config test files; the execution-state T2 write list matches that narrow set.
**Impact:** The stated deletion makes `Parse` non-compilable or silently changes `Error()` with no specified replacement, and the full suite cannot pass. Calling `renderDiagnostics` an “alias” misclassifies an `error` implementation.
**Required change:** Specify the post-change contract for `(*Diagnostics).Error()` (keep it, or replace it with an explicitly defined safe rendering), and add every config test that uses `requireRendered`/`Error()` to T2/T4 scope.

### B5 — R2 “exact field set” versus AD8's event-specific scalars and the deleted field whitelist is unresolved
**Spec:** §2 R2; §4 AD8 and log.go contract; T4.
**Repository evidence:** `internal/observe/log.go:13-26` defines `Entry.Fields map[string]any`; `log.go:137-214` is the per-field whitelist that currently governs counts, shapes, stages, HTTP fields, and correlation fields. AD8 only names `RequestID`/`Generation`/`Duration`/`Server` as surviving event-specific fields and says the legacy whitelist is deleted. R2 lists only `config` and `resource` as event-specific objects, while service producers rely on many `Fields` keys.
**Impact:** T4 must either delete operational metadata (counts, slots, stages, HTTP timing, tool metadata) without approval, or retain an arbitrary map that contradicts R2's “exactly” wording. No AC constrains the resulting field set; AC04-1 only checks six shared fields.
**Required change:** Enumerate the complete envelope field vocabulary (including request/operation/duration/generation/server and the current `Fields` keys) or explicitly approve a typed replacement and data-loss list; state whether `Fields` remains.

### B6 — Startup configuration rejection has no owning task or specified rendering path
**Spec:** §4 main.go/serve.go contracts; AD5/AD7; §5 AC05-4; R10.
**Repository evidence:** `cmd/mcp-bridge/serve.go:81-84` returns `fmt.Errorf("configuration: %w", err)` for startup `load()` failure. `cmd/mcp-bridge/main.go:39-41` currently prints `mcp-bridge: <err>` plaintext. T1 names only `checkConfig` failure; T3 covers non-configuration startup failures and its ACs do not include config diagnostics; T5's write paths exclude `main.go`. AD5's fallback is `internal_error`.
**Impact:** The startup configuration path can remain plaintext or be misclassified as `internal_error`, violating R10/AD7 and AC05-4 (“startup rejection of the same document remains `error` with exit 1”). No task/AC owns the `errors.As(err, *config.Diagnostics)` branch.
**Required change:** Assign startup configuration rendering to T5 (or T3) explicitly, include `main.go` in that task's files, and add an AC that exercises `runContext` with an invalid document.

### B7 — R6 conflict attribution is not covered for all conflict sites, and one site puts the earlier position in the wrong fields
**Spec:** §2 R6; §5 AC02-2; §4 duplicate/disjointness bullet; §3 evidence claim about duplicates.
**Repository evidence:**
- `internal/config/bounds.go:54-57` (`weights_invalid`) sets `RelatedKey` only; no related line/column.
- `internal/config/config.go:392-400` (`credential_overlap`) reports `other.location` and related ref identity, but `refOccurrence` stores only `{reference, location}` (`config.go:1037-1039`), so no earlier node position is retained.
- `internal/config/config.go:991-995` (`header_duplicate`) stores only the earlier entry path in `seen`; no related line/column.
- `internal/config/config.go:681-686` (`yaml_key_duplicate`) sets `Line`/`Column` to the **earlier** key node while `RelatedKey` also names the earlier key. AC02-1 requires the own/offending node position.
- `internal/config/command.go:160-164` and `credentials.go:45-48` emit accessor diagnostics with no line/column; T2's file/site list omits `command.go` and `credentials.go`.
- The §3 claim “duplicates today carry only `RelatedKey` strings” is false for `yaml_key_duplicate`, which already carries the earlier position in `Line`/`Column`.

**Impact:** R6 says `config.related` carries property path, line, and column for conflicts; AC02-2 covers only duplicate pin/key. Weights, overlap, and header conflicts would not be acceptance-verified, and the duplicate-key record would still point at the wrong node.
**Required change:** Enumerate every conflict site (weights, overlap, header, server id, pin, key) and every accessor diagnostic; specify how both nodes are captured; move the earlier position to `RelatedLine`/`RelatedColumn` and set own `Line`/`Column` to the offending node; extend acceptance coverage beyond duplicate pin/key.

### B8 — R9's “first error in deterministic document order” has no acceptance test
**Spec:** §5 Coverage line: “R9→AC01-1/AC04-1 (single event) + AD4 renderer tests”; AD4; T1/T2.
**Repository evidence:** `AC01-1` is the single-record 65-pin case. Multi-record behavior is real (`internal/config/aggregate_edge_test.go:73-96` asserts up to 32 secret-stage records), but no listed AC asserts which record AD4 selects or that selection follows document order. AD4 is an auto-decision, not an acceptance item.
**Impact:** The central D10 fail-fast rule is implemented by assertion only in a non-enumerated “renderer test”; the coverage claim overstates what AC01/AC04 verify.
**Required change:** Add an AC (and owning T1/T2 test) for multi-record selection: one envelope, first document-order record, preserved `diagnostics_incomplete`/`remaining_stages`.

### B9 — R17's documentation-consumer migration is not acceptance-covered
**Spec:** §2 R17; §5 AC07; §6 T7.
**Repository evidence:** `README.md:551-563` documents `environment`/`version`/`service`, `README.md:629-645` documents `time`/`environment`/`version` in HTTP examples, and `README.md:278-306` documents the plaintext CLI and 18-field reload manifest. `AC07-1`/`AC07-2` cover only the new `docs/logging-contract.md`; `AC07-5` only checks emitted records. The execution-state write envelope does include `README.md`, but no AC verifies it.
**Impact:** R17 explicitly requires repository docs/queries to migrate in the same change; a correct implementation could leave README documenting removed fields and still pass every listed AC.
**Required change:** Add an explicit documentation-migration acceptance check (or extend AC07) naming `README.md` and any query docs, or narrow R17 with owner approval (not available here).

### B10 — Documentation ACs contradict the bound execution state
**Spec:** §5: “AC07-1..AC07-7 … T + R”; T7 says the verification method is `R` for documentation checks.
**Repository evidence:** `.omp/plans/execution-state.json:1030-1059` sets `"method": "test"` and `"requiredEvidenceKinds": ["test"]` for `AC07-1` and `AC07-2`.
**Impact:** Documentation requirements are bound to test evidence that cannot exist for prose content; the machine gate and the spec disagree about acceptance evidence.
**Required change:** Set these ACs to the documentation/review evidence kind (or add a test that mechanically validates the documented field list), and align §5 with the execution state.

---

## Advisory findings

### A1 — Observability claim ties `request_id` to an AC that does not assert it
**Spec:** §7 tracing/health bullet: “request correlation fields (`request_id`) are preserved … verified in AC04-1 tests.” **Evidence:** AC04-1 lists only `schema_version/level/service/event/operation/outcome` (§5); `observe.Entry` carries `RequestID` (`log.go:17`). **Required:** name the exact assertion/test for preserved `request_id` (and duration), or remove the claim; this is the skill-required verification tie-in.

### A2 — The shared renderer omits the diagnostic's static reason, although ticket 02 requires static reasons to appear
**Spec:** §4 renderer bullet enumerates `config.*` fields but not `Reason`; AD8 derives `message` from the fixed `eventMessage` table. **Evidence:** `internal/config/diagnostics.go:68-79` defines `Reason` as a static package-owned explanation; ticket 02's “What to build” says “only validated identifiers, finite classes, and static reasons appear.” **Required:** specify that `message` (or another static field) carries the per-diagnostic static reason, or document why the reason is intentionally dropped.

### A3 — AD6's `--help` stream preservation is under-specified
**Spec:** §4 AD6; AC03-5 says `--help` output is unchanged. **Evidence:** `cmd/mcp-bridge/main.go:30` sets flag output to `stderr`; default `Usage` writes to that output. AD6 sets output to `io.Discard` and says “`Usage` is kept,” but does not specify a custom `Usage` writer. Without one, `--help` prints nothing. **Required:** specify that the custom `Usage` writes the unchanged text to `stderr` (or the exact chosen stream).

### A4 — AD5's per-site kind and remediation table is not enumerated
**Spec:** §4 AD5 says kinds are resource/dependency/internal “per site” and remediation is a finite table, but does not assign each site. **Evidence:** the failure sites differ (`serve.go:96` storage, `:102` JWKS, `:112` embedding, `:115-127` tool surface, `:240`/`:245` listeners). **Required:** list each event/code/kind/retryable/remediation tuple so AC03-1's “distinct” assertions are contract-driven rather than invented by the test author.

### A5 — R15/R16/R19 have no acceptance item
**Spec:** §5 explicitly says “R15/R16/R19: explicit non-goals (no AC — verified by absence-of-code review R in T7).” The execution-state `acceptance` array contains no AC for them. **Impact:** the user's requirement “every R has acceptance coverage” is not literally met; the T7 absence review is not an AC. **Required:** add a T7 review acceptance item (e.g. “no panic recovery/raw-cause mode/prefix work-around exists”) or state the non-goal verification limitation explicitly in the ledger.

### A6 — T7's decision-log append is outside the scope envelope
**Spec:** §6 T7 says “Decision log: `.scratch/structured-logging/decisions.md` (append-only additions noted by coordinator)”; the §6 “Scope envelope” write list and execution-state `writePaths` do not include `decisions.md`. **Impact:** an engineer following T7 literally cannot write the named artifact. **Required:** remove the append instruction from T7 or add the file to the write envelope with coordinator ownership.

### A7 — `restart_required` and document/accessor diagnostics remain positionless
**Spec:** R6 and AC02-1 require line/column for every rejection class. **Evidence:** `cmd/mcp-bridge/serve.go:68-74` synthesizes `restart_required` with `Key` but no `Line`/`Column`; `internal/config/config.go:98-116` document-level failures set no position; `command.go:160-164`/`credentials.go` accessors have none. **Required:** specify which classes have an available source position and either populate it or add an explicit, justified exception.

---

## Verified claims and positive observations

- The §3 inventory is accurate on the main anchors: `Entry` fields and single writer (`log.go:13-36`), `Log` routing (`:50-57`), `numHTTPSlots = 65` (`httpdiagnostic_encode.go:55`), `main.go` exit 2 and plaintext paths (`main.go:30-41,49-58`), `serve.go` failure sites (`:96,102,112,115-127,240,245`), `operatorLog`/18-field manifest (`wiring.go:411-470`), ceiling site (`config.go:210-216`), `Diagnostic` fields and `maxDiagnostics = 32` (`diagnostics.go:20-33,39-84`), and 64-vs-65 pin behavior (`pins_bounds_test.go:40-61`).
- D1–D20 map onto R1–R19 with no missing decision, and the ticket dependency edges match the task graph exactly (T1 → T2/T3/T4/T6; T5 after T2+T4; T7 last). The four specdd marker pairs are unique, ordered, non-nested, and on their own lines.
- Critic risk 6 is currently safe: `normalizeURL` returns only static literals (`config.go:558-591`), so `Reason: err.Error()` does not echo the URL or query.
- Critic risk 3 is narrowed: `docValidator` rejects merge keys and aliases (`config.go:663-675,728-731,792-795`), so the anchor/merge edge is not reachable; index alignment still matters for the ordinary sequence case.
- Critic risk 5 is real but no repository script was found depending on exit 2; the only `os.Exit(2)` is the unrelated test-child guard (`embedding_env_process_test.go:18`).
- §7 documents the collector, timestamp/environment/version, Prometheus, and shipping exceptions with reasons and consequences as the observability contract requires. The missing tie is `request_id` (A1).
- The nine critic-handoff items are substantive review targets, not decoration: item 2 exposes the exact `RelatedKey` gaps in B7; item 4 exposes the AD2 arithmetic defect in B2; item 7 exposes the README gap in B9.

### Verdict
REVISE

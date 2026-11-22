## Code Review Summary

**Files Reviewed:** 27 candidate-bound inputs (spec, ticket, backlog, domain/issue-tracker docs, logging contract, go.mod, 20 code/test anchors).
**Mode/pass:** specification only, initial (r13). **Total Issues:** 9 (0 CRITICAL / 2 HIGH / 4 MEDIUM / 3 LOW).

**Closure evidence (all nine r12 findings closed):** SR12-01/C1 — R3/§4 define the unexported `serve.go` wrapper; `main.go:57` `errors.As` unchanged; `startup_failure.go` untouched; AC2-4/AC2-8a assert `errors.Is`/`errors.As`. SR12-02/C2 — R3/§4 fatal before any open/read; AC2-4 FIFO/symlink-dir/loop; AC2-8d both families. SR12-03/C3 — R6/AC1-2 match `store.go:93-106`; `snapshot.go:140` rejects foreign prompt schema; `unsupported_schema` not producible. SR12-04 — R5 + AC1-9c. SR12-05 — AC2-1/2-9 5s/25ms. C4 — §7 explicit exception; `docs/logging-contract.md:607-614` verified. D3 and AC2-8 ownership also closed.

### By Severity
- CRITICAL: 0
- HIGH: 2
- MEDIUM: 4
- LOW: 3

### Issues

**[HIGH][statement-contradiction][Conf: HIGH] Retention parse is family-agnostic (§4 step (3))**
The regex `^(snapshot|prompts)\.corrupt-...` admits both families, while R5 bounds retention **per family**; no family filter is stated. A snapshot pass can count/unlink `prompts.corrupt-*` (irreversible evidence loss). AC1-3/1-8 seed one family; AC1-6 does not pin ≥5 seeds/family. Fix: parse `<family>\.corrupt-...`, declare other-family names foreign, and add an explicit both-families prune/no-cross-delete oracle. (Pre-existing in r11/r12, not introduced by r13.)

**[HIGH][statement-contradiction][Conf: HIGH] Reservation collision vs fatal create failure (R3, §4(2))**
"Reservation-create failure ⇒ fatal" conflicts with AC1-9(b), which requires an existing name to advance to `now+1ns`; only AC2-8(c) exhaustion is fatal. The `fs.ErrExist`-vs-other distinction is missing. A literal implementation aborts on the first collision and fails AC1-9(b). Fix: `fs.ErrExist` advances one candidate; any other create error is fatal; 4096 exhausted fatal; state this in AC2-8(b-res).

**[MEDIUM][statement-contradiction][Conf: HIGH] R7 vs AC2-6 arm 1**
R7 forbids "no recovery or retention record of any kind" when disabled; AC2-6 requires exactly one retained Durable-loader `startup_snapshot_rejected`. Fix: scope R7 to records produced by the recovery/retention path.

**[MEDIUM][missing-state-transition][Conf: HIGH] First-family fatal + second-family corrupt undefined**
D2/§4 and AC2-7 cover only recoverable-first/fatal-second. If the snapshot is fatal (FIFO/directory) and prompts is corrupt, whether prompts is quarantined after the fatal is unspecified. Fix: specify stop-on-first-fatal (recommended) or accumulate, and add the oracle.

**[MEDIUM][unverified-contract][Conf: HIGH] Retention record shape not oracled**
AC1-6 asserts only message/paths. R4's `warn`/`startup`/`failure`, `error.code=state_directory`, `kind=resource`, `retryable=false`, and canonical remediation are unverified; `validErrorInfo` (`envelope.go:200-206`) requires exact equality. Fix: assert the full record shape in AC1-6 and pin the message/remediation text.

**[MEDIUM][traceability][Conf: MEDIUM] T2 lacks the producer anchors for "existing remediation"**
§4 reuses "existing remediation" for `snapshot_unavailable`/`state_directory`, but T2's inputs omit `wiring.go:486-491` and `startup_failure.go:26-33`, and the strings are unquoted. Risk: on-arm/off-arm classification drift. Fix: quote both strings or add the read-only anchors.

**[LOW][provenance][Conf: HIGH] §9.7 is stale**
It claims the ticket header cites r12/sha256 `f020...` and needs re-sync; the supplied ticket's AC heading says "mirror of spec r13" and its header cites r7, with no sha256. Fix: state the actual ticket state.

**[LOW][verifiability][Conf: HIGH] No-block FIFO oracle and fixed message**
AC2-8(d)'s "test's own deadline" is not numeric; AC2-4's 5s poll cannot interrupt a blocking open unless the call runs in a goroutine+select. AC1-6's "equal to the constant" pins no literal (default `"mcp-bridge event"`, `log.go:270-275`). Fix: pin a numeric deadline/mechanism and the message literal (e.g. `"quarantine retention failed"`).

**[LOW][coverage][Conf: HIGH] R8 vocabulary and R1 negatives partially unoracled**
Only `corrupt_json` is emitted at the envelope layer (AC2-2/2-9); the other four `failure_reason` values are tested only at the storage `Reason` layer (AC1-2). AC1-7 omits `LoadPrompts`/`SavePrompts` never-quarantine assertions. Fix: table-drive all five admission cases and add the prompts negatives.

### Open Questions
[LOW] R5/§4 "until exactly `QuarantineKeep−1` pre-existing remain" is unsatisfiable with fewer than four pre-existing files; say "evict while more than four remain". [LOW] AC2-6 does not exercise the empty-string env value that D4 says enables recovery.

### Positive Observations
- The non-regular branch is checked before any open/read; FIFO/device/socket/symlink-to-one are fatal and untouched (AC2-4/2-8d).
- The cause wrapper preserves `errors.As`/`errors.Is` without touching `main.go`, `startup_failure.go`, or `wiring.go`; no cause text enters a record field.
- Prompts taxonomy matches the real walk; future-dated retention and numeric deadlines close SR12-04/05; C4 is an honest, verified exception.
- Envelope 129→130 and the `quarantine_path` slot are verified; `logging_contract_test.go:114` needs no out-of-envelope edit because T2 updates both the docs union and the envelope array.

### Recommendation
REQUEST CHANGES
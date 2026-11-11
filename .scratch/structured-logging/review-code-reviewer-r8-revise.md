## Code Review — r8 specification (`r8-pin-diagnostics`)

**Packet digest reviewed:** `358ee090950567ab87950f7c296d4a966df1e782c7a7dc58b83e1cf1728d4d3c` (matches `execution-state.json` `packet.digest`; authorized by the `owner-authorization-r8.txt` addendum). Spec sha256 `a76abbb7…` matches the state binding. Mode: specification review (Stage 1).

### 1. Marker integrity — pass
Four pairs, unique, correctly ordered, non-nested, each on its own line: `spec.md` 40/67 (requirements), 168/203 (design), 270/286 (tasks), 288/302 (envelope).
- LOW: title `spec.md:1` still says `r7-closure1` while §1 (`:34`) says `r8-pin-diagnostics`.

### 2. Requirement→AC coverage — revise
- R22 candidate-vs-retained counts and age unasserted: AD16 (`:192`) defines `candidate_servers`, `candidate_tools`, `retained_age_ms`; AC08-3 (`:241`) asserts only `retained_generation`/`catalog_tools`. Fix: extend AC08-3 to assert all five.
- R24 reload clause unasserted: AD21 (`:202`) defines `config_reload_applied` and adds `config_revision`/`trigger` to `config_reload_rejected`; no AC tests either. Fix: add AC11-4 (R24, owner T11).
- AC08-2 (`:240`) requires `next_attempt_ms` on start **and** outcome; AD16's `catalog_refresh_completed` (`:191`) omits it (ticket 08 AC-2 also requires it). Fix: add `next_attempt_ms` to that record.
- AC09-2 (`:249`) requires "file"; AD18 (`:196`) omits `config.path`. AC09-8 (`:253`) requires `config_revision` on the evidence; AD18 omits it. Fix: add both keys to AD18.

### 3. AC testability / producing task — revise
- AC11-1/AC11-2 are unimplementable: startup never resolves pins — `reload.go:554-560` sets `next.pins = nil`; a "pin rejection at startup" cannot occur, so T11's `startup path lacks detail` (`:284`) is false. Fix: define the startup leg as the first post-startup discovery pass (narrated by refresh records), or authorize a startup pin check.
- AC09-2 is outside the write envelope: `Config.Pins` is `[]map[string]string` (`config.go:66`), pin nodes are not retained, `origin` is unexported with no accessor (`:78`, `:457`), and `internal/config` is absent from the r8 list (`:295`). Fix: authorize pin-position retention + an origin accessor in `internal/config` and add the file to T9/envelope.
- AC08-1 "warn or error" cannot fail; AD16 pins level by `catalog_state` → assert warn for `retained_published`, error for `none_available`. AC11-2 merely restates AC11-1. AC10-2 "where applicable" leaves the oracle open. AC12-3's "documented contract matches producer emission" is unmechanized; bind it to the AC12-1 doc-drift test.
- T8–T11 lack `Deliverable: ACxx` (`:281-284`; T8 names AD16 only) although AC prefixes and state `acceptanceIds` are 1:1. Fix: add the deliverable lines.

### 4. Ruling fidelity — revise
- Held: no metric changes (`:281`, `:283`, `:298`); no `schema_version` bump (AD20 `:200`); no raw error text (`:282`, AD18); additive-only r7 record extension (AD21 `:202`); no induced-failure gate (T12 `:285`).
- `decisions.md:162` records the owner as having **ACCEPTED** the misleading-counter gap, but the only transcribed answer (`owner-authorization-r8.txt`, Q3) says "Decision pending; no acceptance … is authorized yet". Fix: append the missing owner turn or mark it pending.
- Strict-serial ruling is not enforceable from the state: T9 and T10 are both `blockedBy:["T8"]` (`execution-state.json:333-373`) while sharing `refresh.go`/`log.go`/`envelope.go`. Fix: set T10 `blockedBy:["T9"]`.
- Ruling 12 (candidate vs retained counts) unenforced — see §2.

### 5. Envelope / state consistency — revise
- Dependency edges match the ledger.
- State authorizes `internal/upstream/runtime.go` (`:257`, `:379`), but the spec permits one T10 discovery file (`dedicated.go`, T10 `:283`; `:295`). T10 effects also add `serve.go` and `dedicated_test.go`, absent from the T10 ledger.
- `spec.md:295` authorizes `internal/service/service_edge_test.go`; it is absent from the state envelope and every task effect.
- AC12-1/AC12-2 are `"method":"inspection"` (`:780`, `:793`) versus spec `test (doc-drift)` (`:262-263`) and T12's RED doc-drift test.
- `decisions.md:160,166` still bind packet `26842ba7…`; state and addendum bind `358ee090…`. `spec.md:76` still claims state revision `r6`.

### 6. Defect sites — revise (one mischaracterization)
- Verified accurate: `refresh.go:210-238` (err only nil-checked; failed pass falls through to `runtime.Current()` metrics at `:223`), `runPass` `:195-205` emits no records; `reload.go:562-564` wraps the text identity from `core.go:311`; `wiring.go:373/:394` is the only production `SetServer` caller; `ops.go:19` `omitempty`, `:48-51`.
- Inaccurate: T11's premise that the startup path "lacks detail" — it performs no pin evaluation at all (`reload.go:554-560`). Same fix as §3.

### Recommendation
**REVISE.** Any AC/design edit exceeds the authorization's "no change to acceptance criteria or requirement text beyond the reviewed r8 revision" clause, so the amendment needs fresh owner re-authorization before T8 dispatch.
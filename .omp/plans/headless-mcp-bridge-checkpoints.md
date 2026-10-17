# Headless MCP Bridge — Checkpoint Ledger (SpecDD)

Canonical specification: `headless-mcp-bridge.md`, final revision **HMB-v1 r8**.
All approvals recorded below are user confirmations from the main session on 2026-10-06.
Planning artifacts stay local; never committed/pushed alone.

## Critic verdicts (independent)

| Revision | Critic | Verdict |
|---|---|---|
| r1 | BridgeCritic | REJECT — C1 (stateful inbound vs 2026-07-28 stateless-only handler, go-sdk streamable.go:1504-1548), C2 (default MRTR middleware re-dispatches original tools/call, mrtr.go:18-31); C3/C4 minor. |
| r2 | BridgeCritic2 | APPROVE-WITH-AMENDMENTS (both views) — C1/C2/C3/C4 resolved; C5/C6 prescribed (discover exemption streamable.go:1516-1541; pointer `MultiRoundTrip: &MultiRoundTripOptions{Disabled:true}`; urlElicitationMiddleware re-dispatch ban, client.go:764-864); C7 tracked (U-01 hard T-10 precondition). |
| r3 | BridgeCritic3 | CONFIRMED (both views) — C5/C6 applied verbatim, independently spot-checked against SDK source; one non-blocking symbol-accuracy fix prescribed (clientElicitationMiddleware → urlElicitationMiddleware). |
| r5 | BridgeCritic4 | VERDICT-REQDESIGN APPROVE-WITH-AMENDMENTS (2 MINOR wording: C1 R-20 group label, C2 AC-03 phrasing); VERDICT-TASKS APPROVE. Namespace removal verified COMPLETE (34 occurrences all negation/history; operative sections namespace-free; identity tuple `[server_id, raw_name]` verified). |
| r6 | (coordinator-applied editorial fixes C1/C2 as prescribed by BridgeCritic4; no semantic change) | r5 verdicts stand per critic's non-blocking disposition. |
| r7 | BridgeCritic5 | CONFIRMED (both views) — C5/C6 applied verbatim, independently spot-checked against SDK source; one non-blocking symbol-accuracy fix prescribed; C9 pinned: discover supportedVersions = exactly the four legacy versions as a SET (sequence = SDK `Server.protocolVersions` newest-first, excludes 2026-07-28, no oldest-first assertion); no 400 for latest-discover offers (legacy discover -> fallback initialize @ 2025-11-25/negotiatedVersion cap); raw non-discover initialize with Mcp-Protocol-Version 2026-07-28 IS rejected 400 under method/version-header scope; Phase C gate ordering T-05 -> T-06 -> T-07 -> T-08 -> T-09 -> T-10 (blocked U-01). |
| r8 | (coordinator-applied C9 clarification as confirmed by BridgeCritic5; no semantic behavior change) | Clarifications only; stable IDs/behavior unchanged; **no new approval**. |

## User checkpoints (separate confirmations, 2026-10-06)

1. **Requirements (r4)**: Approved — R-01–R-22 incl. U-06 as presented.
2. **Design (r4)**: Approved — 4-version inbound, 5-version upstream + MRTR disabled, SHA-256 stable names, JSON snapshots, hybrid RRF, breakers, atomic reload, 13-metric observability.
3. **Tasks (r4)**: Approved — T-01–T-10 five-phase TDD ledger, CT-01–CT-06 matrix.
4. **Scope amendment (with the above)**: namespaces REMOVED — single `/mcp` endpoint fronting all configured servers; server-ID provenance in search/describe/call; `servers[]` filter; stable name = `t_`+base32(SHA256([server_id, raw_name])); flat YAML `servers:` + top-level `pins:`.
5. **Clarification (same session)**: single endpoint, all servers (recommended); server ID surfaced as provenance (recommended). Both accepted.
6. **RENEWED approval after r5/r6 revision (same session)**: Requirements (R-07/R-08/R-10, U-06 resolved), Design (§4.2/§4.3/§4.4/§4.11), Tasks (T-03/T-05/T-06/T-07, CT-03) — all three explicitly approved for r6.

## Gate

- `.superpowers/sdd/GATE_OPEN` created locally after the approved task ledger and the above evidence existed (2026-10-06).
- **Gate extension absent**: `.omp/extensions/sdd-gate.ts` does not exist in this project. The machine dispatch block is therefore NOT active; the approval requirements were satisfied and recorded regardless. Installing the extension is optional and requires explicit user request.
- Implementation progress (2026-10-09): Phases A–C GREEN — T-01–T-07 per `go test ./...` (all packages ok, `go vet` clean), plus T-05b (service tool surface: the three bridge tools over a real MCP session) and T-08a/T-08b (config + credential fingerprints, startup admission; embedding pipeline and durable vector lifecycle), plus T-08c (non-fatal durable startup) and T-08d (config session-idle bound + bounds error naming, completed from the `ConfigIdleBound` handoff), each in its own RED->GREEN section below with mutation evidence. Outstanding: the rest of T-08 (composition root, inbound session bounds, host/origin enforcement, dynamic server metrics) and T-09 (consolidated >=90% project coverage gate, race/vet/build, docs/sample). Those sibling slices are in flight, so the repo's `internal/service` and `internal/observe` test packages are transiently RED on their files and service-level verification runs in a throwaway tree copy until they land. T-10 remains BLOCKED on U-01. Phase C gate OPEN: T-05 RED gate was rewritten inline (see the T-05 section) after the earlier fake GREEN set was rejected.
- `GATE_OPEN` and planning artifacts are not independently shippable; commit only with the associated implementation meeting all AC/test/coverage gates.

## Unresolved prerequisites (implementation blockers, not plan blockers)

- U-01: real upstream URLs/auth/versions/capabilities + actual omp negotiated call version (hard T-10 precondition; latest-only omp client would have no call path into the 4-version bridge).
- U-02: exact pinned tool names + verified read-only operations.
- U-03: SDK schema/number/no-replay characterization at T-01/T-04.
- U-04: actual embedding provider contract/model/dimensions/credentials.
- U-05: filesystem/atomicity/resources + task-local omp evidence setup.
- T-10 (real omp e2e) remains BLOCKED until the above are supplied; implementation may proceed on Phases A–D.

## T-05 RED gate — internal/catalog/dispatch_test.go (r8 plan §4.3/§4.4)

- 2026-10-09. Previous file was a fake GREEN gate: targeted only existing
  `catalog` symbols (PublicName/CollidePublicName) with no-op assertions and
  compiled GREEN, which violates the no-fake-RED contract. Replaced.
- New file imports the missing `github.com/jjohnson83853/mcp-bridge/internal/core`
  package and exercises the prescribed Bridge/registration/dispatch/pins API
  with faithful behavioral assertions per plan contract:
  - wrongdigest: full unpadded lowercase base32 over compact canonical JSON
    [server_id,raw_name], 55 chars, no name normalization (R-08).
  - identicalrawnamescollapse: distinct public names across servers with same
    raw name (T-05 RED identicalrawnamescollapse).
  - schema-number: integer/number case matrix preserves number precision
    end-to-end in forwarding (T-05 RED schema-number; T-01 U-03).
  - acrossserverwrongroute: dispatch of a portainer tool calls portainer and
    only portainer even when developer has the same raw name (RED
    acrossserverwrongroute).
  - collisionoverwrite: duplicate raw name within one server rejects the
    affected refresh and retains the LKG binding, no overwrite (RED
    collisionoverwrite).
  - invalidstaledispatch: wrong revision -> `catalog_changed`, zero dispatch;
    unknown public name -> `unknown_tool`, zero dispatch (RED
    invalidstaledispatch).
  - resultenvelopedataaltered: provenance envelope exposes server/revision
    provenance and byte-identical upstream CallToolResult (RED
    resultenvelopedataaltered).
  - pinwrong: EXACT [server_id,raw_name] pin resolution; missing pinned
    definition omitted with safe reason; wrong-server pin never auto-resolves
    (RED pinwrong).
  - repeat: original tools/call is never re-dispatched after execution, even
    when the upstream outcome is a replay trigger (no-replay; RED repeat).
- RED baseline captured:
  `internal/catalog/dispatch_test.go:17:2: no required module provides
  package github.com/jjohnson83853/mcp-bridge/internal/core`
  `FAIL github.com/jjohnson83853/mcp-bridge/internal/catalog [setup failed]`
- Planned GREEN phase: `internal/core` Bridge/registration/dispatch/pins API
  per r8 §4.3/§4.4 flat global model; `go test ./internal/catalog
  ./internal/core` GREEN, then `go test ./internal/catalog ./internal/core
  ./internal/service` for T-05 full prescription.
- No production code changes in this phase.

### 2026-10-09 mid-phase addendum (T-05 defect disposition)
- T05DefectFixup: BLOCKED, no changes. Claimed "55 characters" defect in
  dispatch_test.go was a false positive from the parent turn; the file already
  reads "full unpadded lowercase base32" and no such substring exists. Ledger
  line 53 "55 chars" was the only occurrence and referred to the checkpoints
  wording, not the plan.
- Real contract conflict found: plan §4.3 says 54 chars (t_ + base32 unpadded
  of 32-byte SHA256 = 52 chars -> 54 total) but dispatch_test.go
  TestPublicNameExactDigest asserts len(got)==55 while computing `want` as
  t_+base32(...) — the assertion contradicts the same test's own want string.
  GREEN would fail on len if it implements the correct digest. Disposition:
  in-scope fixup for T05Green (len 55 -> 54), recorded here, not a scope
  expansion. Plan §4.3 "54ASCII characters" wording retained as authoritative.

### 2026-10-09 T-05 GREEN fidelity pass (contract defects vs r8 §4.3/§4.4)

Scope: T-05 RED kept as the gate (`internal/catalog/dispatch_test.go`); GREEN
`internal/core` corrected where the previous GREEN contradicted the plan. All
claims below were reproduced before/after with observable test output.

Build blockers removed (dead weight, recorded, not silent):
- Deleted `internal/core/dispatch_test.go`: untracked leftover of the earlier
  core-package copy of the T-05 RED (self-qualified `core.` inside `package
  core` -> `undefined: core`), superseded by the `internal/catalog/dispatch_test.go`
  replacement already recorded above. It broke `go test ./internal/core`.
- Removed now-unused `encoding/json` import in `internal/core/core_test.go`.

Contract defects fixed in GREEN + pinned in RED:
1. Identity alphabet. `core.PublicName` used `base32.HexEncoding` (chars
   `[0-9a-v]`), violating the plan's own tool_name pattern
   `^t_[a-z2-7]{52}$` (r8 §4.4 lines 230/244; R-08) for ~99.9% of digests.
   GREEN now uses `base32.StdEncoding` (RFC 4648 §6, unpadded, lowercased) and
   the §4.3 "no HTML escaping" canonical form (encoder `SetEscapeHTML(false)`).
   RED evidence: `public name t_2c24...0g76qc1dak5cb0 violates §4.4 pattern
   ^t_[a-z2-7]{52}$`; GREEN: `ok internal/catalog`.
2. Catalog revision format. `Bridge.Revision()` returned `rev_<64hex>` (68
   chars), violating the §4.4 `catalog_revision` pattern `^[a-f0-9]{64}$`.
   GREEN now returns the bare 64 lowercase hex digest.
   RED evidence: `catalog revision rev_330bb44b... violates §4.4 pattern
   ^[a-f0-9]{64}$`.
3. Revision coverage. Revision hashed only `[server_id,raw_name]`, ignoring
   exact definitions; §4.3 requires "sorted bound server identities and exact
   definitions". GREEN now hashes canonical `[server,raw,description,
   input_schema,output_schema]`, whitespace-insensitively (canonicalSchema).
   RED evidence: `schema change did not move the catalog revision`.
4. Execution state vocabulary. Previous GREEN reported `unknown` for admission
   failures and `completed` for post-execution InputRequired — neither is in the
   §4.4 vocabulary (`not_started` / `unknown`). GREEN now: admission failures
   `not_started`; InputRequired after one dispatched call `unknown`. RED now
   pins the literal vocabulary (`TestExecutionStateVocabulary`); the earlier
   RED assertion compared against the same constants it was pinning (vacuous).
   RED evidence: `core.ExecutionNotStarted = "unknown"; want not_started`.
5. Number-aware schema admission (`internal/catalog/validate.go`).
   `exactNumbers` left non-integer `json.Number` values raw; jsonschema-go
   v0.4.3 `jsonType` has no `json.Number` branch, so every fractional number was
   rejected by type ("has type string") and, worse, a number satisfied a
   `{"type":"string"}` schema. GREEN now converts to exact `int64`/`uint64` for
   integral values and to `float64` for fractions below 2^53; larger
   non-integral values keep exact decimal form so numeric bounds stay exact
   (the retained `large decimal exact` case). RED evidence:
   `integer admits zero-fraction: ValidateSchema({"type":"integer"}, 1.0) =
   invalid instance: ... has type "string", want "integer"`.
   Characterization consequence: a fractional value of magnitude >= 2^53 passed
   to a schema that also declares `{"type":"number"}` is still typed as a string
   by the library. Recorded as an upstream-library limitation for critic review;
   no acceptance criterion was weakened.

Consolidation (duplicate implementations of one contract):
- Identity is single-sourced in `internal/core`. Removed `catalog.PublicName`
  and `catalog.CollidePublicName` (zero production callers; the previous RED's
  `TestPublicName*` and `naming_test.go` covered only those duplicates) and
  deleted `internal/catalog/naming_test.go`.
- Removed `NumberCase` / `SchemaNumberMatrix` from `internal/catalog` and
  `NumberCase` from `internal/core`: two mutually contradictory characterization
  artifacts, one of which was asserted only by a tautological RED
  (`TestDispatchSchemaNumberPreserved` built Go literals and asserted its own
  flags). Replaced by a real `ValidateSchema` matrix in the RED.
- `catalog.ToolSchemas` (draft-07 stub with only `$schema`, contradicting §4.4's
  draft 2020-12 schemas and its `[a-z2-7]`/`[a-f0-9]` patterns) replaced by the
  exact §4.4 input schemas: `SearchToolsInputSchema`, `DescribeToolInputSchema`,
  `CallToolInputSchema`, plus the pinned patterns/bounds. Pinned by
  `catalog_test.go` contract tests (including no `namespace` field).

Verification (2026-10-09): `go build ./...`, `go vet ./...`,
`go test -count=1 ./...` (all eight packages ok),
`go test -race ./internal/catalog ./internal/core` ok, `gofmt` clean.
Coverage after this pass: project total 71.2% statements (pre-existing, T-09
gate work outstanding); `internal/core` 8.1% -> 68.6% with the new in-package
`internal/core/bridge_test.go` (rebind rejection, empty raw name, nil upstream,
nil-bridge pins, unescaped canonical identity, revision determinism/description
coverage, canonicalSchema totality, recorder accounting).

## 2026-10-09 — T-06 global hybrid search RED -> GREEN

Plan: r8 §4.7/§4.8, §4.4 `search_tools` output. New package `internal/search`.

RED (`internal/search/search_test.go`, 12 tests, `package search_test`):
compile-RED baseline captured before implementation —
`go test ./internal/search/` -> `no non-test Go files in .../internal/search`,
`FAIL ... [build failed]`. Pinned behaviours: omitted `servers` searches every
configured server; explicit filter narrows and an unconfigured server is
`invalid_filter`; `DecodeRequest` rejects unknown fields (including a legacy
`namespace`) and rejects out-of-bounds `top_k`, empty/blank query, empty query
string and over-long query without clamping; RRF combines both rankings with the
pinned constant and ties break on the stable public name; provider failure
degrades to `keyword` with a finite reason; missing vectors report
`hybrid_partial`; semantic-only matches require vectors; cache key covers global
`catalog_revision` + model identity and is bounded/TTL-scoped; formatter-v1
content hash is versioned.

GREEN: `internal/search/{search.go,bm25.go,vector.go,cache.go}`. Formatter-v1
`ContentHash`; secret-free `ModelIdentity.Digest`; strict `DecodeRequest`; server
filter validated against configured backends *before* scoring; BM25
(k1=1.2, b=0.75, casefolded `rawname + "\n" + description`); exact normalised
cosine over current vectors through the `Provider` seam (no real provider is
fabricated while U-04 is unresolved; the RED uses deterministic doubles);
reciprocal-rank fusion; `hybrid|hybrid_partial|keyword` with finite degraded
reasons; bounded TTL LRU cache keyed by query/normalised servers/global
revision/model digest.

Design decisions pinned in the RED where the plan does not spell them out (no
plan line contradicted): (a) an explicitly empty `servers` array is rejected
fail-closed instead of being read as "omitted"; (b) a document enters the vector
ranking only when cosine > 0 (zero/negative similarity is not a match); (c) mode
is `keyword` with reason `vectors_absent` when no eligible document carries a
current vector; (d) documents in no ranking are omitted from results.
Deferred to T-07 (not asserted here): no-revival of removed servers across a
snapshot swap — that needs the refresh/reconcile path.

Verification (2026-10-09): RED baseline above; `go test -count=1 ./...` 9/9
packages ok; `go vet ./...` clean; `gofmt` clean; `internal/search` 83.7%
statements; project total 71.2% -> 78.4% (T-09 >=90% gate outstanding).
Mutation evidence (both reverted): dropping `hybrid_partial` ->
`TestPartialVectorsReportHybridPartial` fails ("mode = \"hybrid\"; want
hybrid_partial"); admitting zero-score documents (`score >= 0`) ->
`TestSemanticOnlyMatchRequiresVectors` + `TestProviderFailureDegradesToKeyword`
fail ("non-matching query returned 3 results", "keyword fallback exposed the
full catalog").

## 2026-10-09 — T-07 flat durable catalog + membership reconcile RED -> GREEN

Plan: r8 §4.7/§4.9 and §4.10 steps 2-3. New package `internal/storage`
(`snapshot.go`, `store.go`, `reconcile.go`); one addition to `internal/catalog`
(`ValidateSchemaDocument` + `resolveSchema`, reused by `ValidateSchema` so tool
schema admission has a single implementation) and one `.gitignore` line
(`/mcp-bridge`, the root build artifact).

RED, two compile-RED baselines captured before the implementation:
`internal/storage/store_test.go` (7 tests) -> `go test ./internal/storage/` =
`no non-test Go files in .../internal/storage`, FAIL [build failed];
`internal/storage/reconcile_test.go` (6 tests) -> `undefined:
storage.Reconcile/ConfiguredServer/ReconcileInput/ErrRebindTombstoned/
ErrEndpointChanged`. Both `package storage_test`.

Pinned behaviours — store: private 0700 directory and 0600 files; exclusive
single writer (`ErrWriterLocked`, released and reusable after `Close`, which is
idempotent); `snapshot.json` schema 1 with RFC3339Nano UTC `created_at`; a
canonical checksum excluding the checksum field itself; temp-write + fsync +
rename + directory fsync publication leaving no `*.tmp`; a corrupt snapshot or
a leftover partial file is ignored, never adopted and never destructively
overwritten; a failed save preserves the previous trusted bytes and generation;
`Load` rejects tampering, a future `schema_version` and unknown fields
(including a legacy `namespace` column) leaving the rejected file untouched.
Pinned behaviours — reconcile: configured servers are retained with their exact
tool definitions, content identities and current vectors; a departed server is
tombstoned (`BindingAllowed` false on every endpoint) and purged so no tool or
vector of it survives; a newly configured server enters with a first bind and
`catalog_status none`; a tombstoned server is never rebound
(`ErrRebindTombstoned`) and a live server's endpoint identity change is refused
(`ErrEndpointChanged`) with the previous snapshot unmutated; a model identity
change drops vectors but never exact definitions or content hashes;
reconciliation is byte-deterministic for the same input (including across a
save/load restart) and an empty membership shrinks to a valid, fully tombstoned
snapshot; invalid input publishes nothing.

Design decisions pinned in the RED where the plan does not spell them out (no
plan line contradicted): (a) each live server row requires a matching
non-retired binding with the same endpoint fingerprint, and a retired binding
must not keep a live server row; (b) `catalog_status stale` requires a non-empty
safe reason (fresh/none do not); (c) the reconcile input must carry a canonical
64-hex model identity digest — an unknown provider is never presumed and a
missing digest fails closed; (d) reconciled collections are sorted by their
natural key (server_id, public name, tool identity) so a reorder or restart is
byte-stable; (e) `Save` stamps the canonical checksum onto the passed snapshot's
envelope (the envelope carries its own checksum) and a re-save of a loaded
snapshot reproduces identical bytes.

Deferred, explicitly outstanding (not asserted here): "independent
poll-supported notify" coalescing and the persist/pointer swap need the
service-level pointer and the `upstream.ServerClient` refresh host, which only
exist with T-08; no speculative scheduler was built without that host. Also
left to T-09: project-wide >=90% source-line coverage (now 79.4%) and the
README/sample/`cmd` wiring for the storage+reconcile path.

Verification (2026-10-09): RED baselines above; `go test -count=1 ./...` 10/10
packages ok; `go vet ./...` clean; `gofmt -l .` clean; `internal/storage` 84.4%
statements, project total 78.2% -> 79.4% (T-09 gate outstanding).
Mutation evidence (all reverted): checksum comparison neutralised ->
`TestLoadRejectsTamperingAndUnknownFields` fails ("Load tampered = <nil>; want
ErrCorrupt"); tombstone guard removed from `BindingAllowed` ->
`TestTombstoneRejectsRebind` fails ("retired endpoint was allowed to rebind");
removed-server tool purge skipped -> `TestReconcilePurgesRemovalsAndKeeps-
ConfiguredServers` and `TestReconcileIsDeterministicAcrossRestart` fail
("tool ... belongs to unknown server \"portainer\"", i.e. the fail-closed
validation caught the partial state); model-identity vector invalidation skipped
-> `TestReconcileInvalidatesVectorsOnModelIdentityChange` fails ("model identity
change kept 2 vectors").


## 2026-10-09 — T-08a config public + per-server credential fingerprints RED -> GREEN

Plan: r8 §4.2 `config_public_fingerprint`, §4.8 (stored `config_public_fingerprint`,
"no secrets/not auth", `config_public_fingerprint` never auth) and §4.10 steps 1-2
and 5 (the reload needs a public config identity and a per-server credential
identity to decide candidate vs running config and client reuse). The T-07
snapshot took that fingerprint as an opaque input, so this is the producer that
was missing.

RED: `internal/config/fingerprint_test.go` (package `config_test`, 4 tests /
18 subtests). Compile-RED baseline captured before implementation:
`go test ./internal/config/` -> FAIL [build failed], `mustParseConfig(...).Fingerprint
undefined (type *config.Config has no field or method Fingerprint)` and
`... .ServerCredentialFingerprint undefined`.

Pinned behaviours: `Config.Fingerprint()` is a lowercase 64-hex SHA-256 over a
canonical secret-free projection of every public root section (http, operations,
storage, catalog, search, embedding, client_auth, upstream_defaults, servers
[id/endpoint/limits/credentials], pins); it is stable across repeated parses and
independent of comments, quoting, key order and the order of the `servers` and
`pins` lists; every public change is visible (endpoint, server limits, credential
shape, credential/ client-token reference names, http/operations listen, storage
directory, catalog refresh, search top-k, embedding revision, upstream default
timeout, pins, added server); and rotating a resolved secret VALUE does not
change it. `Config.ServerCredentialFingerprint(id)` returns a lowercase 64-hex
digest over the server id and that server's resolved credential values, reports
false for an unknown id or an unresolvable reference (fail closed), is distinct
per server, changes when that server's own credentials rotate, and does not
change when another server's or the inbound client token rotates.

Design decisions pinned in the RED (no plan line contradicted):
(a) the projection carries a `format_version` so a projection change invalidates
stored fingerprints instead of silently comparing incompatible encodings;
(b) the fingerprint is a semantic identity, so `servers`/`pins` are ordered by
id/(server, tool) before hashing, unlike raw file order;
(c) only secret VALUES are excluded — reference names are public configuration
(secret-bearing positions are validated to hold {env:NAME}/{file:PATH} only, so
the projection cannot carry a resolved value);
(d) canonical encoding is `encoding/json` (sorted mapping keys) hashed with
SHA-256, the same convention the storage checksum uses, so no second encoder;
(e) an unencodable projection panics rather than returning a constant, because a
constant would make two different configurations compare equal.

Verification (2026-10-09): RED baseline above; `go test -count=1 ./...` 10/10
packages ok; `go vet ./...` clean; `gofmt -l .` clean; `internal/config` 85.7%
statements.
Mutation evidence (all reverted): dropping `Storage`/`ClientAuth` from the
projection -> `TestFingerprintTracksPublicShapeChanges/{client_token_reference,
storage_directory}` fail; hashing the credential reference instead of the
resolved value -> `TestServerCredentialFingerprintTracksRotationAndIsolation`
fails; removing the server-id sort -> `TestFingerprintIsDeterministicHexAnd
PresentationIndependent` fails ("server entry order changed the fingerprint").

Outstanding (unchanged): T-08 atomic reload/drain/refresh-scheduling slice
(assembly/pointer, LKG on failed reload, drain of replaced clients,
`SIGHUP`) and T-09 (project-wide >=90% source-line coverage, race/vet/build,
README/sample/`cmd` wiring). T-10 remains BLOCKED on U-01.

## 2026-10-09 — T-08b runtime assembly/publication + drain RED -> GREEN

- RED: `internal/service/reload_test.go` (new). Compile-RED baseline captured
  with the GREEN file absent: `undefined: service.Runtime / service.NewRuntime /
  service.ReloadDeps / service.Assembly / service.ServerRuntime`
  (`FAIL github.com/jjohnson83853/mcp-bridge/internal/service [build failed]`).
- GREEN: `internal/service/reload.go` (new; `Runtime`, `Assembly`,
  `ServerRuntime`, `Lease`, `ReloadDeps`, `leaseTracker`).
- RED defect fixed in my own test (recorded, not silently dropped): the
  "catalog collision" case first assumed a cross-server raw-name collision.
  `core.PublicName` is server-namespaced (`internal/core/core.go:85-91`,
  canonical JSON `[server_id,raw_name]`), so that premise is unreachable; the
  case now uses the reachable collision (a duplicate raw name inside one
  server's own definition slice), which `Bridge.Register` rejects.

Invariants pinned by the RED (plan §4.10 steps 1-6, R-17, AC-16/AC-17):
(a) one reload publishes exactly ONE pointer (`Runtime.current`), and the
persisted snapshot precedes publication (asserted as `persist generations =
[0 1]`, using the generation visible at persist time);
(b) a server whose `(endpoint, per-server credential fingerprint)` pair is
unchanged keeps its exact dedicated client object (no second client, no close);
(c) a failed reload of any class (discovery, catalog rejection, client
construction, storage reconcile/persist) preserves the exact working assembly
pointer, generation, config fingerprint, catalog revision, per-server endpoint
and client objects, publishes nothing, persists nothing, and closes every
candidate client it created (leak-free) while closing nothing that belonged to
the working generation;
(d) a superseded generation refuses NEW leases immediately — including for
servers carried forward — while a pinned in-flight lease keeps working; the
replaced/removed client closes exactly once, only after its last lease is
released (`Lease.Release` idempotent, no replay on a retired generation);
(e) reload without a candidate, or without discovery/client-factory/clock,
fails closed and publishes nothing.

Verification (2026-10-09): RED baseline above; `go test -count=2 -race
./internal/service` ok; `go test -count=1 ./...` 10/10 packages ok;
`go vet ./...` clean; `gofmt -l .` clean; `internal/service` 90.6% statements
(`reload.go` 97.9%; `Reload` 97.9%). Project-wide statement coverage is 81.3%:
the >=90% project gate remains T-09's deliverable (current low packages:
`observe` 42.0%, `catalog` 67.7%, `core` 68.6%, `upstream` 72.3%, `cmd` 75.0%).
Mutation evidence (each mutation applied, observed failing, reverted):
publishing before persisting -> `TestReloadPublishesOnePointerAndReusesMatching
Clients` ("persist generations = [1 2]; want [0 1]") and
`TestFailedReloadRetainsExactWorkingAssembly/persist_failure`;
reusing a client on endpoint match alone -> `TestCredentialRotationReplaces
OnlyThatClient` ("rotated credentials reused the old alpha client") and
`.../client_construction_failure`; closing carried-forward clients -> both drain
assertions ("closed clients = [..]; want exactly the removed server's client
once"); dropping the failure-path cleanup -> `.../endpoint_identity_change`
("failed reload leaked the candidate client it created"); letting a retired
generation admit leases -> `TestRemovedServerClientDrainsWithoutReplay` and
`TestRuntimeAcquireReadsOnePointerAndRefusesRetiredGenerations`.

Outstanding (T-08 remainder): `SIGHUP` + `cmd/mcp-bridge` run-loop wiring
(listen, `/mcp` handler, ops listener, graceful shutdown) and T-09
(project-wide >=90% source-line gate via `internal/coveragecheck`, race/vet/
build, README/sample). T-10 remains BLOCKED on U-01.

## 2026-10-09 — T-04b dedicated per-server runtime + core failure channel RED -> GREEN

Scope: the missing half of T-04 (`internal/upstream`) — one dedicated
client/session owner per configured server, plus the `core.Upstream` failure
channel the tool layer needs.

- RED: `internal/upstream/dedicated_test.go` (new). Compile-RED baseline
  captured with the GREEN file absent: `undefined: upstream.Dedicated /
  upstream.NewDedicated / upstream.DedicatedOptions` (`FAIL
  github.com/jjohnson83853/mcp-bridge/internal/upstream [build failed]`).
- GREEN: `internal/upstream/dedicated.go` (new). Fixtures are real MCP peers
  over `mcp.NewStreamableHTTPHandler` (SDK `ServerOptions.PageSize=2` gives
  genuine two-page `tools/list`), not hand-rolled doubles.
- API defect fixed in my own earlier GREEN (recorded): `core.Upstream.CallTool`
  and `Bridge.Dispatch` carried no context, so inbound cancellation could not
  reach the upstream call (plan §4.3 "best-effort cancel/no resend", line 285).
  Both now take `ctx`; `UpstreamOutcome` gained the safe failure channel
  `Failure *UpstreamFailure{Code, ExecutionState}` and `Dispatch` maps it to
  `DispatchError` (retryable always false; `not_started` only when no execution
  was admitted, else `unknown`) per §4.4 line 260.

Invariants pinned by the RED:
(a) one dedicated runtime = exactly one session for its whole lifecycle:
  repeated `Discover` issues exactly one `initialize` (asserted by counting
  `"method":"initialize"` bodies at the peer);
(b) `Discover` returns the COMPLETE catalog across pages in deterministic
  sorted order with schemas stored compact (semantic equality asserted, since
  the SDK decodes `Tool.InputSchema any` before the bridge sees it);
(c) one forward = exactly one upstream execution with byte-exact args
  (`{"limit":1,"ratio":1.10}` reaches the peer unchanged) and the upstream
  `CallToolResult` JSON comes back for the provenance envelope;
(d) input-required is `unsupported_capability`/`unknown` after exactly ONE
  upstream execution — never fulfilled, never re-dispatched;
(e) `Close` is idempotent, closes the session once, and the runtime then fails
  closed immediately (`Discover` -> `backend_unavailable` via `IsUnavailable`,
  `CallTool` -> `backend_unavailable`/`not_started`).

Known limit (documented, not hidden): numeric literals inside the upstream
payload are normalised by the SDK's own `CallToolResult.StructuredContent any`
decode before the bridge receives them; the bridge preserves what it receives.
Client-supplied args bytes are preserved exactly.

Verification (2026-10-09): RED baseline above; `go test -count=1
./internal/upstream` ok (4 new cases); `go test -count=1 ./...` 10/10 packages
ok; `go vet ./...` clean; `gofmt -l .` clean. `internal/upstream` 74.3%
statements; project-wide 81.4% (>=90% remains T-09's deliverable).
Mutation evidence (applied, observed failing, reverted): removing the closed
guard -> `TestDedicatedCloseIsIdempotentAndFailsClosed` ("Discover after Close
succeeded"); stopping after the first page -> `TestDedicatedDiscoveryPaginates
WithOneSession` ("discovered tools=2/2, want 5/5 across pages"); dropping
`UseNumber` -> `TestDedicatedCallForwardsExactArgsOnce` ("upstream received args
... ratio\":1.1, want byte-exact ... 1.10").

Outstanding: T-08c `SIGHUP` + `cmd/mcp-bridge` run-loop wiring (the tool surface
is delivered in the T-05b section below), T-09 gate (project-wide >=90%
source-line coverage, race/vet/build, README/sample). T-10 remains BLOCKED on
U-01.

## T-05b — service tool surface RED->GREEN (2026-10-09)

Delivered: `service.ToolSurface` (`internal/service/tools.go`) registering
exactly the three bridge tools with the exact §4.4 schemas, plus
`ReloadDeps`-time pin resolution (`Assembly.Pins`/`PinnedNames`/`Health`) and
`core.Bridge.Bindings()`.

RED-first evidence: `internal/service/tools_test.go` was written before any
tool-surface code; `go test ./internal/service` then failed with
`undefined: service.ToolSurface / service.ToolSurfaceConfig /
service.NewToolSurface` (6 tests blocked at compile). One RED expectation was
itself wrong and was corrected against observed behaviour rather than against
the implementation being convenient: query `read row` matches BOTH stored tools
(BM25 `OR` over tokens), so the assertion now pins deterministic ORDER
(`read` before `write`) and per-result provenance instead of a single result.
(The first live run also exposed a test-side bug: `CallToolParams.Arguments any`
holding a bare `[]byte` was base64-encoded; the test now passes
`json.RawMessage`.)

GREEN contract, now pinned by tests:
(a) `tools/list` advertises exactly `search_tools`/`describe_tool`/`call_tool`
  with schemas semantically equal to `catalog.*InputSchema()` (verified over a
  real in-memory MCP client/server session, not a direct handler call);
(b) `call_tool` returns the §4.4 provenance envelope: outer `CallToolResult`
  preserves the upstream content blocks and `isError`;
  `structuredContent` = `server, catalog_revision, catalog_status,
  availability, tool_name, request_id, upstream_result` with
  `upstream_result` the COMPLETE original upstream result; exactly ONE upstream
  execution per call with byte-exact forwarded args, and a fresh `request_id`
  per execution;
(c) admission failures (`catalog_changed`, `unknown_tool`, `invalid_arguments`)
  answer `isError: true` + `{code, message, retryable, execution_state}`, with
  `execution_state = not_started` and ZERO upstream executions; a stale
  revision is answered `catalog_changed` even when the arguments are also
  invalid (revision precedes validation);
(d) `describe_tool` serves the stored exact schema with
  `validation_status = supported|unsupported` (+ a fixed reason) and never
  rewrites it; `pinned` reflects the configured pins;
(e) `search_tools` answers from the published generation: `mode = keyword` with
  a finite `degraded_reasons` when no embed provider is configured, ranked
  results with the full §4.4 provenance set, one backend row per configured
  server, and `invalid_filter` for an unconfigured server filter.

Design decisions recorded (not in the plan text verbatim):
- Indexed documents are the PUBLISHED CATALOG BINDINGS of the generation, not
  the durable `Tools` rows: `storage.Reconcile` is not yet fed discovered tools
  (that is the refresh/ingest path), so a search over durable rows would return
  nothing on a cold start. Vectors still come from the durable snapshot,
  matched by `storage.ToolIdentity`, `model_identity_digest`,
  `search.ContentHash(rawName, description)` and dimension count; a mismatch
  simply omits the vector and the search degrades with a finite reason.
- `catalog_status` comes from the durable server row (`fresh|stale|none`),
  `availability` from whether this generation holds a live client for that
  server (`available|unavailable`). `probing` is never published by a
  generation. `call_tool` refuses `backend_unavailable` unless `available`.
- Error codes: core's dispatch set (`catalog_changed`, `unknown_tool`,
  `unsupported_capability`, `backend_unavailable`) plus the tool layer's
  `invalid_arguments`/`invalid_filter` (search codes) and
  `unsupported_schema` for a stored schema that cannot be resolved at call
  time. Validation failures answer a FIXED payload-free message, since the
  validator's own text can quote submitted values.
- `describe_tool` field names: `input_schema`/`output_schema` (the exact stored
  schema, decoded not stringified), `validation_status`, `validation_reason`.
- `last_success_at` is RFC3339Nano UTC, `null` when the server never succeeded.
- Known gap (inherited, reported not hidden): `core.ToolDefinition` carries no
  upstream `annotations`/`meta`, so describe cannot yet echo them; the
  discovery slice (T-04) never captured them and no test covered them.

Verification (2026-10-09): RED baseline above; `go test -count=1 ./...` 10/10
packages ok; `go vet ./...` clean; `go test -race -count=1
./internal/service ./internal/core ./internal/catalog` ok; `internal/service`
85.7% statements, project-wide 81.3% (>=90% remains T-09's deliverable).
Mutation evidence (each applied, observed failing, reverted):
1. disabling `call_tool` revision admission -> `stale revision with invalid
   arguments` ("code = invalid_arguments; want catalog_changed");
2. skipping `catalog.ValidateSchema` -> `missing required argument` and
   `unknown argument field` ("admission failure answered successfully", i.e.
   the invalid args reached upstream);
3. issuing a second `Dispatch` -> "upstream executions = 2; want exactly 1";
4. ignoring `core.AddPins` failure in `Reload` -> "a pin outside the catalog was
   accepted";
5. returning `core.Bridge.Bindings` in map order instead of public-name order
   -> `TestBindingsListsEveryBindingInPublicNameOrder` failed in 3/3 runs
   ("Bindings are not in public-name order"); its copy semantics assertion
   covers the aliasing path.

## T-07b — periodic/notification refresh + coalescing RED->GREEN (2026-10-09)

Delivered: `internal/service/refresh.go` — `RefreshDeps` (reload collaborators
plus a failure classifier), the finite reason vocabulary
(`upstream_unavailable`/`discovery_failed`/`client_missing`), `Runtime.Refresh`
(one pass that keeps a failing server's COMPLETE last known good catalog while
healthy servers advance), and `Refresher` (`Trigger`/`Running`/`Pending`/
`Passes`/`Run`) with the §4.7 "one active + one pending" coalescing contract.
`Reload` and `Refresh` now share one `Runtime.build`; `Reload`'s observable
behaviour is unchanged apart from the new per-server status stamping
(`StampServerSuccess`/`StampServerFailure`) required by §4.9/§4.7.

RED evidence (verbatim, implementation withheld): `internal/service` failed to
build — `internal/service/reload.go:392:14: undefined:
RefreshReasonDiscoveryFailed` — with the four refresh tests unable to compile
(`service.RefreshDeps`, `service.Refresh`, `service.NewRefresher`,
`service.RefresherOptions` undefined). GREEN: `go test ./internal/service/
-run 'TestRefresh|TestRefresher' -count=1` -> 4/4 PASS
(`TestRefreshKeepsLastKnownGoodForFailedServer`,
`TestRefreshPublishesNothingWhenAConfiguredClientIsMissing`,
`TestRefreshKeepsRevisionStableForAnUnchangedCatalog`,
`TestRefresherCoalescesTriggerStormIntoOneActiveAndOnePending`).

Contract pinned by tests:
(a) one pass publishes exactly one new generation: the healthy server's new
  binding is visible, the failed server keeps its last known good binding, its
  status is `stale`, its reason is the classifier's token, and its
  `last_success_at` retains the older success while the healthy server's
  advances (per-server status/age, not a whole-pass verdict);
(b) a refresh NEVER creates or rotates a client: a credential change makes the
  pass fail closed with the published generation, catalog revision, and persist
  count unchanged (membership/credential churn stays a reload);
(c) an unchanged catalog keeps the same revision (no spurious `catalog_changed`
  for callers holding an expected revision);
(d) a trigger storm during an in-flight pass coalesces to exactly one further
  pass: 1 trigger starts the pass and 5 further triggers during it yield
  `discovery calls = 2`, with `Pending()` true while the pass runs and false
  afterwards.

Verification (2026-10-09): `go test -count=1 ./...` 10/10 packages ok; `go vet
./internal/service` clean; project-wide statement coverage 81.1%
(internal/service 83.3%) — >=90% remains T-09's deliverable.
Mutation evidence (each applied, observed failing, reverted):
1. dropping the `buildRefresh` new-client rejection -> "refresh published a
   generation for rotated credentials";
2. stamping every server fresh after a failed discovery -> "beta health =
   {Status:fresh ...}; want stale with the classified reason";
3. widening the pending slot to 8 (no coalescing) -> "discovery calls = 6; want
   exactly 2 (one active plus one coalesced)".
Not yet wired: `cmd/mcp-bridge` still has no SIGHUP/refresh driver (T-08) and the
refresher's per-server Discover must bound itself with the §4.7
`refresh_timeout` because `ReloadDeps.Discover` carries no context.

## 2026-10-09 — T-08 run loop + T-05/T-06 supporting accessors (engineer, inline)

Deliverables (all verified GREEN):
1. `internal/service/loop.go` — `LoopOptions{Load, Deps, Interval, Signals,
   Metrics, ReloadMetrics, Logf}`, `Loop`, `NewLoop`, `Start`, `Reload`,
   `Refresher()`, `Reloads()`. `Runtime.Drain(deps)` added (reload.go) for the
   shutdown path: it retires the published generation so every dedicated client
   closes once its leases release.
2. `internal/service/loop_test.go` — 5 tests: startup admission publishes
   generation 1 and drains both clients exactly once on cancel; a rejected
   candidate (loader error) retains the published generation, catalog, clients,
   and persist count; a signal candidate drops exactly the removed server's
   client and reuses the survivor's; the configured interval drives refresh
   passes without a signal; repeated identical reloads publish one generation
   each with zero client churn.
3. `internal/config/credentials.go` — `Config.ServerCredentials(id)`
   (`Credentials{Bearer, Headers}`) and `Config.ServerLimits(id)`
   (`Limits{*MaxConcurrentCalls, *CallTimeout}`); both fail closed on an unknown
   server id or an unresolvable reference, and the limit accessor distinguishes
   unset from zero. Tests in `internal/config/credentials_test.go` (4).
4. `internal/observe/metrics.go` — recorders `RecordRefresh(server, status,
   reason) bool`, `RecordReload(outcome, reason)`, `SetCatalogAge(server, sec)
   bool`; every label passes the finite vocabulary and an unconfigured server
   label is refused. Tests in `internal/observe/metrics_test.go` (3).

Verification (2026-10-09): `go test -count=1 ./...` 10/10 packages ok; race
clean (`go test -race ./internal/service`); `go build ./...` ok; project-wide
statement coverage 82.9% (was 81.1%).

Open design question escalated to the planner (ruling pending, no code invented):
`Loop.Start` currently uses the strict `Runtime.Reload`, so one unreachable
upstream blocks startup, while plan line 295 states "no all-server startup
barrier". `Runtime.Refresh` cannot serve as the startup admission because it
never creates a client (reload.go ~line 377), and the durable snapshot is only
carried forward in memory (`previousSnapshot(previous)`), so the plan's
"coldcache unavailable until samebound server version-correct connects/discovers"
(line 295) vs "Lastgood memory" (line 326) reading is unresolved. A `Runtime.Seed`
experiment was written, exercised, and reverted: it is not verified behaviour and
is not in the tree.

Still not wired (unchanged from the previous section): `cmd/mcp-bridge` has no
serve path (SIGHUP/refresh/HTTP/ops), and the embedding provider adapter does not
exist.


## 2026-10-09 (r9) — startup admission client creation, publish hook, reload-outcome gate

Verification for the whole entry: `go build ./...` ok; `go test -count=1 ./...`
10/10 packages ok; `go test -race -count=1 ./internal/service` ok; project-wide
statement coverage 81.9% (2001/2443, `internal/coveragecheck.Check` over a full
`go test ./... -coverprofile`), i.e. still below the 90% completion gate.

1. `internal/service/reload.go` (build): the startup admission's client selection
   was nested behind `if in.mode != buildStartup`, so a startup generation created
   NO client (observed `created(alpha)=0`, zero closes at drain). Fixed: adoption
   of a previous live client is what startup forbids (`in.mode != buildStartup`
   now guards only the adoption branch); startup and reload both build a dedicated
   client from the CURRENT credentials. Observed after: one client per server,
   drain closes both exactly once.
2. `internal/service/refresh.go` (`report`): the `Published` hook now fires on
   every successful publish (including the startup-probe pass). Previously it was
   unreachable when `Metrics` was nil, so a probe generation was published with no
   observation.
3. `internal/service/loop_test.go`: the rejected-reload gate now observes the
   plan's reload-outcome metric (§6 `bridge_config_reload_total{outcome,reason}`,
   `ReloadMetrics`) instead of `Reloads()`, which by contract counts ADMITTED
   reloads and therefore can never be 1 for a rejection. The harness wires
   `ReloadMetrics`, and the test additionally pins the finite reason
   `admission_failed` while asserting the published generation is retained (AC-17).
4. Removed the superseded `internal/core/dispatch_test.go` (broken self-qualified
   `core.` copy of the T-05 RED) — the recorded replacement's leftover; with it
   gone `internal/core` builds and its suite passes again.

A throwaway debug file `internal/service/zz_debug_test.go` was written to isolate
(1) and (2) and deleted; no debug scaffolding remains in the tree.

## 2026-10-09 (r9) — cold-start durable catalog/vector retention (T-07/AC-13/14/15/16)

RED gate: `internal/service/durable_test.go` (new) —
`TestStartupServesTheDurableCatalogAndKeepsCurrentVectors`,
`TestStartupTombstonesAServerTheConfigDropped`,
`TestStartupInvalidatesVectorsWhenTheModelIdentityChanged`, and
`TestToolSurfaceUsesOnlyCurrentDurableVectors` (with the cold-start harness
`newAdmittedToolHarness` extracted from `newToolHarness` in
`internal/service/tools_test.go`). Observed RED before the fix: "startup snapshot
kept 0 tool rows for alpha", "startup did not tombstone the dropped server",
"startup kept 0 tool rows of a changed model identity", and search
`mode=keyword degraded_reasons=[vectors_absent]` where hybrid was required.

1. `internal/service/reload.go` (build): the startup admission reconciled against
   `previousSnapshot(previous)` — nil at startup — so `storage.Reconcile` derived
   the next snapshot from nothing and DROPPED the durable Tools/Embeddings rows,
   its tombstones and its generation sequence; only `definitionsFromSnapshot`
   kept the catalog alive. Added `buildInput.reconcileBase`, which reconciles the
   startup mode against `in.durable` (the validated snapshot of the previous run)
   and every other mode against the in-memory previous snapshot. Observed after:
   the published startup snapshot retains the durable tool rows and the vectors
   whose model identity still matches, keeps the dropped server tombstoned
   (`Retired`) with no health row, and drops vectors of a changed model identity.
2. Verification: fix-reverted run reproduces exactly those 4 failures; restored
   run is green. `go build ./...` ok; `go test -count=1 ./...` 10/10 packages ok;
   project-wide statement coverage 82.6% (2020/2446, `internal/coveragecheck.Check`
   over a full `go test ./... -coverprofile`), still below the 90% gate.

Open T-08 findings (composition root, not yet in the slice):
- `cmd/mcp-bridge/main.go` is still the Phase-A `--check-config` stub: no
  `storage.Store`, no `LoopOptions.Durable`/`Persist`, no MCP server/transport, so
  the durable snapshot is never read or written in production and the retained
  catalogs/vectors are unreachable outside tests.
- `internal/service/loop.go` (`Start`) returns a fatal error when
  `LoopOptions.Durable` fails, while its own contract says a corrupt snapshot is
  reported and startup proceeds without cached state (plan §4.9).

## 2026-10-09 (r9) — Phase 1/2/3 dispatch wave (in flight, results not yet verified)

Coordinator reconciliation of the remaining T-08/T-09 work into slices with
disjoint file ownership, so no two workers share a file. All six are running;
nothing below is a verified result yet.

- `InboundSessionBounds` — `internal/service/inbound.go` + a new
  `inbound_bounds_test.go`: per-principal concurrent session and request-rate
  admission (plan §4.5/§4.6, R-18) with `InboundOptions.{MaxSessions,RateLimit,
  Burst}`; the handler must refuse at the limit with a finite reason and must not
  create a session it then rejects. Owns `inbound.go`, so the Host/Origin slice
  is held back to avoid a second writer on the same file.
- `LoopDurableNonFatal` — `internal/service/loop.go` (+ its own new test file):
  make a corrupt/absent durable snapshot non-fatal at `Start` (plan §4.9),
  keeping the finite reason observable, per the open finding above.
- `MetricsDynamicServers` — `internal/observe/metrics.go`: per-server dynamic
  metric registration (plan §6, R-18) instead of a fixed label set.
- `EmbeddingPipeline` — `internal/service/reload.go`/`refresh.go` + new
  `embedding_test.go`: the verified missing production path that CREATES
  `storage.Embedding` rows. Verified before dispatch: `storage.Reconcile` only
  carries valid previous rows (`internal/storage/reconcile.go:120-175`),
  `search.Searcher` embeds the query only (`internal/search/search.go:494`), and
  `internal/service/tools.go:104-133` only projects existing rows, so in
  production every search degrades with `vectors_absent` and R-10/R-13/AC-09/
  AC-13 are unreachable outside fixtures. `ReloadDeps` gains
  `Embed func(ctx, []search.Document) ([]storage.Embedding, error)`; nil/error
  degrades (never rejects) per R-14, reuse is by `ToolIdentity`+`ContentHash`+
  model dimensions, the merge is canonicalised and re-validated, and the
  composition root supplies the provider closure (with its own batching, timeout
  and `bridge_embedding_*` observability).
- `ConfigAllowedHosts` — `internal/config/command.go`/`command_test.go`:
  surface the already-validated `http.allowed_hosts`/`http.allowed_origins`
  (`config.go:194,347`) on `Command` so the run loop can enforce the plan's
  host/origin 403 rule (§4.5 `hostorigin403`, §4.6). Both optional (the plan's
  sample ships `allowed_origins: []`); empty disables the check.
- `ConfigIdleBound` — `internal/config/config.go`/`config_test.go`: defect found
  by running the shipped binary, RED-first fix:

      $ go run ./cmd/mcp-bridge --check-config --config configs/config.example.yaml
      invalid configuration: session_idle_timeout outside permitted duration bounds

  `validateBounds` (`config.go:533-546`) applies a blanket 30s/120s request-
  timeout limit to every `*_timeout` key, but `session_idle_timeout` is a session
  lifetime and the plan's own §4.2 sample (plan line 117) ships `10m`. The
  shipped sample must validate end to end (T-09); the fix gives that key its own
  bound and re-runs `--check-config` on the sample.

Worker incident (recorded, not silent): the first `EmbeddingPipeline` dispatch
(51m28s) returned "TOOL-PLUMBING BLOCKED — all Bash/grep/LSP calls exit 1 with no
output, zero verification possible, task NOT complete" after leaving an abandoned
partial hunk in `internal/service/reload.go` (`in.mode == ""` compared against the
int-typed `buildMode`, plus a call to a non-existent `in.fillVectors`), no `Embed`
field, and no test file. The coordinator removed that hunk (owner: the failed
worker, which declared the slice incomplete) so `go build ./...` is green again,
and re-dispatched the slice to a fresh worker (`EmbeddingPipeline2`) with an
explicit plumbing sanity check first and the leftover state described. Note: the
`internal/service` TEST build is transiently RED on the sibling worker's
`inbound_bounds_test.go` (`undefined: testInbound`,
`InboundOptions.MaxRequestBytes`), which blocks package-test verification for
every service worker until that slice lands — flagging the coupling rather than
touching another worker's file.

Worker incident 2 (recorded): the `InboundSessionBounds` dispatch (1h36m) returned
INCOMPLETE at budget — `inbound.go` rewritten to the amended contract but calling
`h.metrics.RecordInbound`, which `observe.Metrics` does not expose, so the whole
`internal/service` production build was RED and no test file was ever written (its
report also claims `inbound_bounds_test.go` was "NOT created", but that file exists
from its earlier phase, written against the PRE-amendment static-field contract and
referencing an undefined `testInbound` helper). Coordinator repairs, all recorded:
removed the `RecordInbound` call so `go build ./...` is green for the two sibling
workers in the same package; `gofmt -w` on `inbound.go` and
`inbound_bounds_test.go` (both were unformatted); messaged the live
`MetricsDynamicServers` worker to add `RecordInbound(operation, outcome string)
bool` emitting the plan's `bridge_requests_total` (plan line 340) — a one-line
restoration is owed back into `inbound.go` once that lands; re-dispatched the slice
as `InboundBoundsResume` with an incremental-verification working style.

Scope decisions pinned while fixing that slice (plan lines 260/340-353, config
sample lines 114-116): inbound enforces `401`/`403`/`413`/`404` plus the session cap
(`503`) and idle expiry; `429` rate limiting is NOT inbound-handler work —
`internal/service/ops.go:109-120` already provides `RateLimit(next, rps, burst)`
returning `429`, so the composition root wraps the inbound route with it. New
`InboundBounds.AllowedHosts`/`AllowedOrigins` carry the edge check (empty list
disables that check, matching the sample's `allowed_origins: []`), which is why the
host/origin enforcement slice was folded into this one instead of racing it on the
same file.

Deferred until these land (dependency order, not scope reduction):
- `cmd/mcp-bridge/main.go` composition root (the largest remaining piece: config
  → storage store + `Durable`/`Persist`, upstream factories, bridge assembly,
  MCP server/transport, ops server, SIGHUP) is blocked on `InboundOptions` and
  `ReloadDeps.Embed`; dispatching it earlier would pin interfaces the workers own.
- Host/Origin enforcement is blocked on `inbound.go` ownership above.
- README/sample docs wait for the final CLI/endpoint surface from the
  composition root so the text cannot drift from the binary.
- `configs/config.example.yaml` already matches the plan's §4.2 shape (verified by
  reading it); only its validity is in question (above). `README.md` is a 12-byte
  stub and is a T-09 deliverable.

## 2026-10-09 (r9) — T-08b embedding pipeline / durable vector lifecycle RED -> GREEN

Plan: r8 §4.8 (formatter-v1 content hash, per-row vector reuse), §4.9 (Tools/
Embeddings rows are the durable catalog), §4.4 (describe `validation_status`),
§4.5 ("unsupported zero dispatch but searchable/describable. Raw schema
retained"), AC-09/AC-10/AC-13/AC-14/AC-15/AC-16, R-10/R-13/R-14.

Files: `internal/service/embedding.go` (`fillVectors`/`missingVectors`/
`mergeVectors`), `internal/service/index.go` (`indexedToolRows`),
`internal/storage/index.go` (`IndexTools`), the reload wiring
(`reload.go:519` index, `:527` vectors), RED `internal/service/embedding_test.go`
+ `internal/service/durable_test.go`.

RED (observed, before the fix): `TestStartupServesTheDurableCatalogAndKeepsCurrentVectors`
kept 0 current vectors of 2; `TestReloadReusesVectorsAndReembedsOnlyChangedText`
and `TestReloadReusesVectorsAcrossASchemaOnlyChange` ran a full 2-document
embedding pass over an unchanged catalog; `TestReloadKeepsOnlyReusableVectors-
WhenEmbeddingFails` and every `TestReloadRefusesAMalformedEmbeddingPass` subtest
lost a current vector of another tool; `TestStartupNeverAsksTheEmbedder` kept 0
durable vectors.

Defect 1 (production, mine): `storage.IndexTools` kept an embedding row only when
its identity appeared among the rows of the server being indexed, so indexing
server A deleted every vector of server B - with two servers each generation
ended with 0 vectors, every reload re-embedded the whole catalog, and a restart
served none. Fixed: replaced-set retention (an embedding is dropped only when its
identity is a row of this server - before or after the swap - and its content
hash differs; every other identity is untouched). Cross-package coverage
`IndexTools` 91.2%, `fillVectors`/`missingVectors`/`indexedToolRows` 100%.

Defect 2 (cross-slice integration, recorded): the durable catalog's schema
admission demanded a *resolvable* schema, so a stored upstream schema that is
well-formed but unresolvable made the whole candidate snapshot invalid and
failed the reload - contradicting §4.5/AC-10, which require exactly that tool to
stay searchable and describable, with `validation_status: unsupported` and the
raw schema served. Split the catalog admission: `catalog.ValidateSchemaStructure`
(document validity, used by `internal/storage/snapshot.go`) and
`catalog.ValidateSchemaDocument` (resolvability, used by the describe status and
the call admission). The store's own "unresolvable schema" case
(`{"type":5}` -> still rejected) and the six `TestToolSurface*` cases (well-formed
`$ref` into a missing `$defs` -> reload succeeds, describe reports unsupported)
are both green; pinned by
`catalog.TestSchemaAdmissionDistinguishesStructureFromResolvability`.

Retraction (recorded so the earlier claim does not stand): the parent turn's
claim of a "`ratio` defect" in `internal/catalog/dispatch_test.go` (asserted
forwarded `"ratio":1.10` from empty args) was a misread of a stale grep excerpt.
The file's dispatch call passes `{"limit":1,"ratio":1.10}`; RED and GREEN agree;
no defect existed and no fix was made.

Verification (2026-10-09): the repo's `internal/service` test package is
transiently RED on the *sibling* slices' in-flight files (`loop_test.go` is
incomplete - `expected '}', found 'EOF'` at 391:3 - and
`inbound_test.go`/`inbound_bounds_test.go` reference `InboundHandler`/
`InboundOptions.MaxRequestBytes` that do not exist yet), so the slice was
verified in a throwaway copy of the tree (`/tmp/mcp-verify`) with only those
foreign files removed, at identical source state for every file this slice owns
and touches:
- `go test ./internal/catalog/ ./internal/storage/ ./internal/service/ -count=1`
  -> all ok (catalog ok, storage ok, service ok), including all six
  `TestToolSurface*`, the durable/startup suite and the new embedding suite.
- `go build ./...` ok in the repo; `go vet` clean for the three packages;
  `gofmt -l` clean for every file touched here.
- No debug scaffolding remains (`zz_*_test.go` probe files were written and
  deleted inside the copy only).
Remaining failures elsewhere in the copy belong to other slices and are recorded
in their own sections: `internal/observe` test build (metrics
`Handler`/`RecordRefresh` arity) and `internal/config` (`session_idle_timeout`
bounds/naming). Project-wide >=90% coverage (T-09) is still outstanding and
cannot be measured while those packages are RED.

Mutation evidence: both defects were observed as the pre-fix failure state of the
same tests that now pass, i.e. the fix is the only difference - defect 1 reverted
(retention keeping only the indexed server's identities) reproduces "startup
snapshot kept 0 current vectors; want 2" plus the full re-embedding pass on an
unchanged catalog; defect 2 reverted (storage admitting only resolvable schemas)
reproduces the six `TestToolSurface*` reload failures while
`store_test.go`'s "unresolvable schema" case stays green in both directions.

## 2026-10-09 (r9) — T-08c non-fatal durable startup RED -> GREEN

Slice: `internal/service/loop.go` startup + `internal/service/loop_test.go`.
Waived/abandoned draft: the file carried an incomplete half-written test
(`TestLoopStartServesWhenTheDurableSnapshotIsRejected`, lines 343-391, no
trailing newline, `expected '}', found 'EOF'`) that also called a nonexistent
`Loop.SetDurableForTest`; replaced by a real RED against the shipped knob
(`LoopOptions.Durable` is already production API, so no test hook was added).

RED (deterministic, before the production change), in the throwaway copy
`/tmp/mcp-bridge-copy` with the sibling in-flight test files removed
(`inbound_test.go`, `inbound_bounds_test.go`, `ops_test.go`):
`go test -run TestLoopStartsWithoutTheCachedStateOfARejectedDurableSnapshot ./internal/service/`
-> `loop_test.go:370: run loop exited during startup: service: startup snapshot:
storage: storage: snapshot is corrupt: decode: invalid character` (FAIL).

GREEN (same test, same tree, production fix the only difference): `ok`.

Fix: a durable snapshot the store refuses is no longer fatal. `Start` reports it
once through the operator log with the finite reason `startup_snapshot_rejected`
and proceeds with `durable = nil`, so the local admission publishes generation 1
with zero cached tool rows and every configured server not-probed
(`storage.StatusNone`/`service.AvailabilityUnavailable`); the immediate probe then
repopulates (generation > 1). No reload admission is counted
(`reportReload("rejected", ...)` removed for this path only - a refused snapshot is
not a rejected reload). Consistent with plan §4.9/§4.10 ("A corrupt snapshot is
reported as an error and startup proceeds without cached state", "long-term memory
/ startup inval id... last good state is kept in memory"), AC-13, and R-13.

Harness change (test-only, no production surface): `loopLog.logf` now records the
formatted line (`fmt.Sprintf`) instead of the bare format string, so the finite
reason token is assertable; `loopHarnessWith` adds the `Durable` loader knob and
`loopHarness` delegates to it with `nil` (existing callers unchanged).

Verification (2026-10-09), in the copy at identical source state for every file
this slice owns: `go test ./internal/service/ ./internal/catalog/ ./internal/storage/
./internal/core/` -> all ok (service 0.405s: full loop/reload/refresh/tools/durable
suites plus the new test); `go test -run TestLoop ./internal/service/` ok after
`gofmt -w`; `gofmt -l internal/service/ internal/catalog/` clean; in-repo
`go test ./internal/core/` ok (the earlier broken `internal/core/dispatch_test.go`
is gone - `internal/core/` now holds `bridge_test.go`, `core.go`, `core_test.go`
and builds clean, so that earlier blocking defect is closed).

Still RED elsewhere in the copy, owned by sibling slices: `internal/observe` test
build (`Metrics.Handler`) and `internal/config` (`session_idle_timeout` bounds and
error naming). T-09 project-wide >=90% coverage remains outstanding.

Re-verification in the repo itself: `internal/service` still fails to build its
test package on the sibling's in-flight `inbound_bounds_test.go` (`unknown field
MaxRequestBytes in struct literal of type InboundOptions`, `undefined:
sessionTestToken`, `undefined: testInbound`), so the copy result above is the
current evidence; the slice will be re-run in-repo once that file compiles.
`go build ./...` is ok in the repo, and `gofmt -l internal/service/` is clean.

## 2026-10-09 (r9) — T-08d config session-idle bound + bounds error naming RED -> GREEN

Handoff: `ConfigIdleBound` returned INCOMPLETE after 1h54m (`GREEN fix + tests
written; final verification runs not completed. Do not ship.`), blocked by its own
environment (nonce/attribution flakiness), leaving `internal/config` RED with the
worker's fix already in the tree. The coordinator finished the slice in place
(same files, no rewrite) and re-verified it.

RED captured after the handoff (pre-fix tree, deterministic):
`go test -count=1 ./internal/config/` -> 3 failing subcases:
- `plan_sample_session_idle_10m_accepted` and `session_idle_window_edge_1h_accepted`:
  `session_idle_timeout outside permitted duration bounds` (the plan §4.2 documented
  `10m` sample was rejected);
- `embedding_timeout_ceiling_kept`: `error "timeout outside permitted duration
  bounds" does not name embedding.timeout`.

Root causes (both real, in the production code, not the test assertions):
1. Fallthrough: the new `session_idle_timeout` branch was a separate `if`, and the
   following request-path rule matches `strings.HasSuffix(key, "_timeout")` - so
   `session_idle_timeout` was also checked against the 120s request-timeout ceiling
   and any session lifetime above 2m was rejected. Now `if/else if`: the session
   lifetime is checked only against its own 1s..1h window.
2. Unqualified key names: bounds errors reported the bare YAML key, so a rejected
   `embedding.timeout` was indistinguishable from any other `timeout`.
   `validateBounds` now delegates to `validateBoundsAt(node, path)`, which carries
   the dotted path down the walk (`embedding.timeout`, `http.session_idle_timeout`)
   - matching the existing house style (`embedding base_url: ...` at
   `config.go:129`). All `wantError` substrings in the suite still match, and no
   other test in the repo pins a bounds message (checked).

GREEN: `go test -count=1 ./internal/config/` -> ok. `gofmt -l internal/config/`
clean.

End-to-end check of the plan §4.2 sample (the deliverable the worker could not
capture): `go run ./cmd/mcp-bridge --check-config --config <example>` with
`BRIDGE_EMBEDDING_KEY`/`BRIDGE_CLIENT_TOKEN`/`DEVELOPER_UPSTREAM_TOKEN`/
`PORTAINER_UPSTREAM_TOKEN` set and the file secret present at 0600 ->
`configuration valid: Config{schema_version:1 servers:2 secret_refs:[env:... file:...]}`
exit 0. Without those secrets it fails closed
(`invalid configuration: secret must be nonempty UTF-8, <=8KiB, without CR/LF/NUL`),
which is the intended behaviour; `configs/config.example.yaml` needed no edit.

No commit/push: sibling packages are still RED (see below), so the tree is not
shippable.

Mutation evidence (throwaway tree copy `/tmp/cfgmut`, live tree untouched):
reverting fix 1 (`else if` -> separate `if`, i.e. the fallthrough) reproduces
`plan_sample_session_idle_10m_accepted` + `session_idle_window_edge_1h_accepted`
failing with `Parse rejected a permitted duration: http.session_idle_timeout
outside permitted duration bounds`; reverting fix 2 (path-qualified messages back
to the bare key) reproduces `embedding_timeout_ceiling_kept` failing with
`does not name embedding.timeout`. Each fix is load-bearing and the paired RED
states are the pre-fix tree.

Repo-wide `go test -count=1 ./...` at this point: ok for
`cmd/mcp-bridge`, `internal/catalog`, `internal/config`, `internal/core`,
`internal/coveragecheck`, `internal/search`, `internal/storage`,
`internal/upstream`; build-failed for `internal/observe`
(`MetricsDynamicServers`: `Metrics.Handler`/`RecordRefresh` arity) and
`internal/service` (`InboundBoundsResume`: `InboundOptions.MaxRequestBytes`,
`sessionTestToken`, `testInbound`). Both are the running siblings' in-flight RED
files, not this slice.

### 2026-10-09 T-08 composition root (main/serve/wiring) GREEN

Slice: the process entry point that turns one operator document into the serving
bridge of plan §4.2 — `cmd/mcp-bridge/{main.go,serve.go,wiring.go}` plus the
`observe` reload counter it needs.

Delivered:
- `main.go`: `main` -> `run(args, stdout, stderr)`; `--config` and
  `--check-config`; ambient signal context for the serving path.
- `serve.go`: the composition root — one run context, logger, config holder with
  staged/published generations, state store, runtime, readiness state, embedding
  provider, tool surface (three bridge tools), dedicated-upstream registry,
  catalog loop (interval + SIGHUP) over one redirectable loader, persist hook,
  inbound edge with token hashes and live bounds, and the operations listener
  behind the rate limiter; drain on cancellation.
- `wiring.go`: `configHolder` (stage/commit/settings/bounds), `restartOnlyField`
  (§4.10 immutable-edit rejection), `upstreamRegistry` (per-server dedicated
  clients, discover, isolated close), `embedder`, `newEmbeddingProvider`,
  `searchSettings`, `localChecks`, `publishObserver`, `operatorLog`,
  `pumpReloads`, `durableLoader`, `configuredServerIDs`, `orNone`.
- `internal/observe/metrics.go`: `bridge_config_reload_total{outcome,reason}`
  (plan §4.11 metric table) with the loop's finite vocabulary
  (`success|rejected` x `none|invalid_configuration|admission_failed`), so a
  rejected document's text can never become a series.

RED -> GREEN evidence:
- `cmd/mcp-bridge/serve_test.go` (the process-level acceptance for
  §4.2/§4.5/§4.6/§4.7/§4.12) is the gate. Two defects in that artifact were
  corrected, both recorded as in-scope fixups, neither weakening the assertion:
  1. `getBody` fataled inside `waitFor` conditions, so the first poll of a
     listener that had not bound yet failed the test deterministically. Added
     `probeBody` (transport error -> not-ok) and used it for the three polling
     conditions; the asserted status codes and bodies are unchanged.
  2. The test asserted that the pre-reload revision "still answers". That
     contradicts r8 §4.3/plan line 197/537: the revision is GLOBAL, so an
     unrelated membership change invalidates an expected revision and the call
     must fail `catalog_changed` with zero dispatch (`execution_state
     not_started`), never be rerouted silently. Replaced with: stale revision ->
     `catalog_changed`/`not_started`; alpha still served at the NEW revision
     (membership added, never dropped) with its arguments forwarded verbatim.
- Shutdown defect found and fixed in the composition root: draining the
  listeners before observing the loop spent the whole shutdown budget on
  `Shutdown` (an open bridged stream never becomes idle), so the loop drain was
  never observed and `serve` returned "the catalog loop did not drain". Now the
  listeners and the loop drain concurrently under one budget, and each listener
  is `Close`d before its graceful `Shutdown` with the reason recorded inline.
  Evidence: `go test ./cmd/mcp-bridge` went 10.1s FAIL ("the catalog loop did
  not drain") -> ok/0.10s.
- `internal/service/inbound.go` `statusWriter`: `WriteHeader` forwarded every
  call (superfluous `response.WriteHeader` logged for each streamed response)
  and `Write` did not record the implicit 200, so `Flush` emitted a second
  header. Fixed: first status wins, and the first `Write` records 200 exactly as
  net/http does. Evidence: 11 `superfluous response.WriteHeader call from
  ...(*statusWriter).WriteHeader (inbound.go:246)` lines per test run -> none.

Verification: `go build ./...` clean; `go vet ./...` clean;
`go test ./...` ok for every package (`cmd/mcp-bridge` 0.10s).

Gate coverage after this slice: `go test -coverpkg=./... -coverprofile=
coverage.out ./...` + `go run covcheck_tmp.go` -> source-line coverage 85.9%
(2609/3036), gate 90%: false. T-09 remains RED on the coverage gate.

### 2026-10-09 T-09 coverage work + config secret-sequence defect

Coverage (package statement coverage, measured package-local; the project gate is
the source-line number in the final section):

| package | before | after | notes |
| --- | --- | --- | --- |
| internal/config | 86.9% | 95.0% | edge tests + the defect fix below |
| internal/search | 86.0% | 96.6% | |
| internal/observe | 81.3% | 95.3% | |
| internal/service | 86.0% | 90.4% | further round in flight |
| internal/storage | 76.2% | 88.1% | further round in flight |
| internal/catalog | 75.5% | 82.4% | further round in flight |
| internal/core | 75.5% | 79.2% | further round in flight |
| internal/upstream | 74.3% | 82.3% | further round in flight |
| cmd/mcp-bridge | 83.6% | 89.2% | further round in flight |

New tests are additive `*_edge_test.go` files; no existing assertion was weakened
or re-pinned. `go test -race` green per package.

Defect found and fixed (TDD, RED observed first): `internal/config/config.go`
`validateNode`'s sequence branch had an inverted predicate. For
`client_auth.bearer_tokens` (whose children map to section `secret`), the check
rejected only scalars whose tag was NOT `!!str`, so a plain YAML string
(`- plain-secret`) was accepted silently while `- 12345` was rejected. Plan line
189 requires secrets to be exactly `{env: NAME}`/`{file: PATH}` with no literals.
Because `resolveSecrets`/`collectRefs` ignore non-mapping scalars, an inline
token contributed no token hash and no diagnostic: the bridge came up with the
operator's token silently missing (every client 401), which is fail-closed but a
silent misconfiguration. RED: three subcases (`literal_string`,
`quoted_literal_string`, `nested_sequence`) failed with "Parse accepted
non-mapping bearer token ...; want rejection"; GREEN after requiring every
element of a `secret` sequence to be a mapping: `go test ./internal/config/...`
ok, `-race` ok, package coverage 95.0%. Regression test:
`internal/config/secret_sequence_edge_test.go`.

Noted, not changed (no plan requirement): `Parse` accepts zero for
`operations.burst` and for `breaker.consecutive_failures`/`half_open_probes`.
The plan gives sample values but no explicit minimum for these keys, and the
runtime coerces a non-positive breaker threshold to its default, so this is a
design gap rather than a spec violation; recorded for the operator.

Also recorded: `internal/service/ops.go` rate limiting answers `429` without a
`Retry-After` header. The plan specifies only "rate 429" (line 260), so this is
not a deviation; no change made.

## 2026-10-09 (r9) — T-09 consolidated gate (GREEN)

Scope: the plan's T-09 = consolidated accept gate: project-wide source-line
coverage >= 90%, `-race`, `go vet`, `go build`, and docs/sample. No production
behaviour was changed in this section; the deliverable is measurement evidence
plus the two shippable-artifact fixes recorded at the end.

Verification (all commands run from the module root, 2026-10-09):

| Command | Observed |
| --- | --- |
| `go test -count=1 -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...` | exit 0, every package ok |
| `go run ./cmd/covcheck` | `source-line coverage 93.0% (2847/3060), gate 90%: true`, exit 0 |
| `go test -race ./...` | exit 0, no failure/race output |
| `go vet ./...` | exit 0 |
| `go build ./...` | exit 0 |
| `gofmt -l .` | no files |

Per-package statement coverage (measured package-local, `go test -cover`):

| Package | Coverage | Uncovered blocks |
| --- | --- | --- |
| internal/service | 97.9% | error/edge blocks listed below |
| internal/search | 96.6% | - |
| internal/observe | 95.3% | - |
| internal/config | 95.0% | - |
| internal/core | 90.6% | 10 |
| cmd/mcp-bridge | 90.2% | 30 |
| internal/catalog | 88.2% | 7 |
| internal/storage | 88.1% | 36 |
| cmd/covcheck | 87.5% | process wrapper lines |
| internal/upstream | 82.3% | 36 |

The project gate is the source-line number above (project-wide, 93.0%); per
package figures are reported for triage only and the two weakest packages are
documented here rather than padded with trivial tests.

Remaining uncovered blocks and why they are unreachable without a production seam:

- `internal/catalog/validate.go:29.3-29.62, 32.16-34.3, 87.10-89.4,
  117.30-121.5, 122.4-122.25, 129.18-131.5` - the `json.Number` conversion and
  recursive-conversion error branches are gated on `SetString` failing for a
  number that came from a successfully decoded JSON literal; no JSON instance
  reaches them. Three independent coverage passes failed to construct one.
- `internal/upstream` (36 blocks) - the remaining `CallTool` error
  classification branches. Reaching them needs a raw Streamable HTTP responder
  that the go-sdk v1.8.0 client accepts; two attempts failed on content-type /
  session negotiation rather than on a product defect. The covered half
  includes connection failure, non-JSON result, multi-block/structuredContent
  results, invalid JSON-RPC, and cancellation; the safe-failure vocabulary
  itself is pinned end-to-end at the service layer.
- `internal/service` blocks (`embedding.go:93-95`, `loop.go:205-207,212-214`,
  `refresh.go:168-169`, `reload.go:406-408,509-522`, `tools.go:85-87,105-106,
  131-132,163-165,226-228,243-245,247-249,262-264,430-432`) - each is either an
  unreachable internal-inconsistency branch, a silent nil-callback no-op, a
  missing injected clock, or requires an entropy/searcher fault source that is
  not injectable through the public API.
- `internal/storage` (36 blocks) and `cmd/mcp-bridge` (30 blocks) - filesystem
  publish failures beyond directory collision, and run-loop branches whose
  assertions duplicate the process-level acceptance test.
- `internal/core` (10 blocks) - JSON encoding errors on values the catalog
  cannot contain, and recorder paths already exercised through the service
  surface.

Defects fixed earlier in this phase (each with its own RED->GREEN section above):
the inverted secret-sequence predicate in `internal/config`, the listener
`Serve`-error classification in `cmd/mcp-bridge`, and the non-idempotent success
write in `internal/service`.

Two shippable-artifact defects found and fixed in this section:

1. The gate command was a scratch file. `README.md` instructed `go run
   covcheck_tmp.go`, a `//go:build ignore` throwaway, so the documented gate was
   not part of the module and had no tests. It is replaced by the permanent
   `cmd/covcheck` CLI (flags `-root`, `-profile`, `-gate`; relative profiles
   resolve against the root), with `cmd/covcheck/main_test.go` pinning the met /
   below-gate / custom-gate / missing-profile / unknown-flag verdicts. The
   scratch file is deleted and both README and this ledger reference the
   permanent command.
2. The shipped sample did not validate. `configs/config.example.yaml`
   referenced `/run/secrets/developer-api-key`, an absolute path an operator
   cannot create in the documented quickstart, and no test covered the sample,
   so the README's first command failed. The reference is now config-directory
   relative (`secrets/developer-api-key`) and `cmd/mcp-bridge/example_config_test.go`
   stages the shipped sample, materialises its `{env:}`/`{file:}` references, and
   asserts `--check-config` exits 0, reports only reference names, prints no
   secret value, and fails when one reference is unresolved. The README
   documents `{env:}`/`{file:}` resolution (private, operator-owned regular file,
   relative to the document) and `configs/secrets/` is git-ignored.

T-10 (real omp upstream end-to-end) remains BLOCKED on U-01: real upstream
URLs/auth/versions/capabilities and the negotiated call version from the actual
omp client are still not supplied.

## 2026-10-10 (r10) — prompt pass-through: intake (pre-checkpoint)

User request: the bridge must pass along the prompts its proxied upstreams
define, so the LLM reaching the bridge can use them ("if a configured tool has a
prompt for the llm it should be available to the llm even if using this proxy").

This is a **scope change**: parent spec line 17 (Non-goals:
"resources/prompts/sampling/elicitation/roots/tasks proxy") and line 277
("Inbound tools/listChanged where supported, no resources/prompts/tasks") are
contradicted and require a planner revision plus renewed critic and user
approval for the affected views.

Three product-level decisions taken by the user before planning (intake
checkpoint 0, 2026-10-10):

1. Freshness — **mirror into the durable catalog**: prompts discovered on the
   same refresh cadence as tools, last-known-good retention, served from memory
   and durable snapshot, not fanned out live per request.
2. Naming — **stable derived opaque names, like tools**: reuse the existing
   derivation approach with provenance carried in prompt metadata, not verbatim
   upstream names on the wire.
3. Scope — **every server advertising the MCP prompts capability, with a
   per-server opt-out** (capability-gated; one config switch per server).

Evidence gathered inline: the SDK supports both halves (`Server.AddPrompt`/
`RemovePrompts` with `notifications/prompts/list_changed`, `ServerOptions.
HasPrompts`/`Capabilities` for capability advertisement; `ClientSession.
ListPrompts`/`GetPrompt`/`Prompts` and `InitializeResult()` for upstream
capability inspection). The bridge is tools-only end to end today: three static
bridge tools (`internal/service/tools.go:58-70`), upstream discovery via
`ListTools` only (`internal/upstream/dedicated.go:76`), fixed config key
allow-lists (`internal/config/config.go:203-214`).

Planner dispatched (2026-10-10) to author r10 at the planning-only sink
`.omp/plans/headless-mcp-bridge-prompts.md`. No implementation gate is open for
r10: `.superpowers/sdd/GATE_OPEN` authorizes only the approved r8 task ledger
whose work is complete/pushed; r10 requires its own requirements, design, and
task approvals before any implementation dispatch.

### Critic review of r10 requirements and design

Reviewed artifact: `.omp/plans/headless-mcp-bridge-prompts.md`, HMB-v1 r10
increment (requirements/design/acceptance as one revision).

- RC-01 (findings review) — **REJECT**. C1 (HIGH): the whole-snapshot integrity
  model (`internal/storage/snapshot.go:39-47,90-98`, `store.go:73-100`,
  `reconcile.go:41-48`) makes a corrupt or unsupported prompt field fatal to the
  tool catalog, contradicting the increment's own recovery promise; prior-binary
  rollback compatibility was unspecified. C2 (MEDIUM): the recovery promise was
  not traceable to a behavioral acceptance gate (AC-25 / CT-07 / T-12).
  Independently checked and accepted as-is: revision isolation (R-27/AC-28),
  cold-start capability (R-26/AC-26), opt-out config/fingerprint/reload
  (R-27/AC-23/AC-25), metrics cardinality (R-27/AC-29), and the decision to let
  the owning upstream validate `prompts/get` arguments.
- Planner revision (2026-10-10) applied both amendments: prompt state becomes a
  separate versioned/checksummed atomically-published sibling object leaving the
  tool snapshot's bytes/version/checksum/decode path unchanged; AC-25, CT-07 and
  T-12 gained the compatibility/corruption/LKG/rollback-reader cases; RELEASE AND
  RECOVERY states old/new binary compatibility and component-isolated recovery.
- RC-02 (delta re-review) — **ACCEPT**, no findings. C1 and C2 verified closed
  against the inspected storage design; regression scan confirmed stable IDs
  (R-23..R-27, AC-23..AC-29, T-11..T-14, CT-07), unchanged task order, no
  weakened criterion, no new scope, and no change to T-10/U-01. This is not the
  final task-ledger verdict, which is requested separately before the tasks
  checkpoint.

Reviewed revision pin: sha256
`b9ef1aa0eb99690e18e5360f7fae2b254b148a776fbe23126a5ea212514dcf4b`
(80 lines). Any change to the sink invalidates RC-02 for the changed content.

### r10 user checkpoints

- **Checkpoint 1 (requirements) — APPROVED, 2026-10-10.** User approved the
  requirements view as drafted (R-23..R-27 plus the parent non-goal amendment at
  lines 17 and 277), at reviewed revision sha256
  `b9ef1aa0eb99690e18e5360f7fae2b254b148a776fbe23126a5ea212514dcf4b`.
  Scope confirmed by this approval: prompts in scope; resources, sampling,
  elicitation, roots and tasks remain out of scope.
- **Checkpoint 2 (design) — APPROVED, 2026-10-10.** User approved the design view
  as drafted at the same reviewed revision sha256
  `b9ef1aa0eb99690e18e5360f7fae2b254b148a776fbe23126a5ea212514dcf4b`:
  per-server `prompts_disabled` opt-out; capability-gated paginated discovery on
  the tool refresh cadence; separate versioned/checksummed atomically-published
  prompt sibling object with component-isolated recovery; opaque derived names
  with canonical provenance; native `prompts/list`/`prompts/get` served from the
  published mapping with exactly-once forwarding; cold-start capability and
  published-change-only `prompts/list_changed`; bounded prompt metrics.

### Critic review of the r10 task ledger

Reviewed artifact: `.omp/plans/headless-mcp-bridge-prompts.md`, TASK LEDGER
(T-11..T-14), CT-07 and the release/recovery region.

- LC-01 — **REJECT**. L1 (BLOCKING): the mandatory project gate was not
  assigned — `go test ./...` neither measures nor enforces the >=90% source-line
  coverage threshold and omitted race/vet/build. L2 (HIGH): RED/GREEN integrity
  was unstated, so a compile-error, presence-only or unrelated failure could be
  claimed as TDD evidence. L3 (HIGH): docs/regression protection unnamed
  (no artifact targets, no stated protection of prior acceptance or T-10/U-01).
  L4 (MEDIUM): package-wide `-run 'Prompt|prompt'` filters do not prove which
  tests ran.
- Planner revision (ledger region only; requirements/design untouched).
- LC-02 — **REJECT**. L5 (BLOCKING): the coverage gate named the `cmd/covcheck`
  CLI without its `-profile`/`-root` arguments.
- Planner revision (ledger/recovery region only).
- LC-03 — **ACCEPT**, no findings. Verified: paired
  `go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...`
  plus `go run ./cmd/covcheck -profile coverage.out -root .` with the measured
  percentage and >=90% verdict required as CT-07 completion evidence; build, vet
  and race commands present; named behavioral tests with assertion-level RED and
  same-test GREEN under `tdd-guide`; recovery, metrics, docs/sync, sample
  validation and prior-acceptance/T-10/U-01 preservation all assigned.

Reviewed revision pin: sha256
`a204182eae165cfec1e0597202def29e5f8e08a3f4a7874d9ec466a59326f4c8`
(80 lines). Diff-verified: lines 1-57 (requirements/design) are byte-identical to
the revision approved at checkpoints 1 and 2 (sha256
`b9ef1aa0eb99690e18e5360f7fae2b254b148a776fbe23126a5ea212514dcf4b`), so those
approvals and RC-02 remain valid; only the task-ledger/CT-07/recovery region
changed.

- **Checkpoint 3 (task ledger) — APPROVED, 2026-10-10.** User approved the task
  ledger at reviewed revision sha256
  `a204182eae165cfec1e0597202def29e5f8e08a3f4a7874d9ec466a59326f4c8` after the
  LC-03 ACCEPT. All three user checkpoints and both critic verdict sets are now
  recorded for r10; `.superpowers/sdd/GATE_OPEN` carries the corresponding r10
  authorization line. Implementation is confined to T-11..T-14 with the ledger's
  named RED/GREEN evidence and the full mandatory gate (build, vet, race,
  coverage profile + `cmd/covcheck` >=90% source-line).

### T-11/T-12 implementation evidence (r10)

- **T-11 + T-12 — COMPLETE, 2026-10-10**, on branch `main`, uncommitted.
  Files: `internal/config/{config.go,fingerprint.go,prompts_edge_test.go}`,
  `internal/upstream/{dedicated.go,prompts_edge_test.go}`,
  `internal/storage/{snapshot.go,store.go,prompts_edge_test.go}`,
  `internal/service/{reload.go,prompts_edge_test.go}`.
- **RED evidence (recreated).** The implementing agent did not capture a
  pre-implementation RED run, so RED was recreated under the ledger's criteria by
  applying one minimal targeted production mutation per named test, running the
  ledger's identical command, capturing the failing assertion, restoring the file
  byte-identically, and re-running to GREEN. Each failure is an assertion on the
  named behavior, after successful compile, not a missing symbol:
  - `TestPromptConfigOptOutDefaultAndValidation` (removal of the `prompts_disabled`
    boolean type check in `internal/config/config.go` `validateScalarType`):
    `prompts_edge_test.go:28: Parse error = <nil>, want rejection` (subtest `wrong_type`).
  - `TestDiscoverPromptsCapabilityPaginationAndFailure` (removal of the capability
    gate in `internal/upstream/dedicated.go` `DiscoverPrompts`):
    `prompts_edge_test.go:48: prompt requests = 1, want 0` (subtest `non-capable upstream issues no prompt request`).
  - `TestPromptStoreIndependentRecoveryAndRollback` (removal of the checksum
    comparison in `internal/storage/store.go` `LoadPrompts`):
    `prompts_edge_test.go:58: LoadPrompts error = <nil>, want corrupt prompt data rejected` (subtest `checksum-invalid`).
  - `TestPromptRefreshLKGAcrossRestartAndOutage` (startup `LoadPrompts` branch in
    `internal/service/reload.go` `build` forced off):
    `prompts_edge_test.go:59: restored prompts = []storage.Prompt(nil), want persisted prompt`.
  - Additional assertion `TestPromptRefreshPublishesStableOrdering` (removal of the
    deterministic `(ServerID, RawName)` sort in `internal/service/reload.go` `build`):
    `prompts_edge_test.go:92: prompt order = [zeta/b zeta/a alpha/b alpha/a], want [alpha/a alpha/b zeta/a zeta/b]`.
- **GREEN:** after every restore, both ledger commands exit 0
  (`go test ./internal/config ./internal/upstream -run '^(TestPromptConfigOptOutDefaultAndValidation|TestDiscoverPromptsCapabilityPaginationAndFailure)$' -count=1`,
  `go test ./internal/storage ./internal/service -run '^(TestPromptStoreIndependentRecoveryAndRollback|TestPromptRefreshLKGAcrossRestartAndOutage)$' -count=1`),
  `go test ./internal/config ./internal/upstream ./internal/storage ./internal/service -count=1` passes,
  and `gofmt -l ./internal ./cmd` is empty.
- Behaviour notes: prompt state is a separate `prompts.json` object (own schema
  version, checksum, atomic temp/fsync/rename publication, strict decode), the tool
  snapshot bytes/version/checksum/decode path are unchanged, prompt data never
  enters the tool catalog revision, and the published prompt mirror is sorted by
  `(ServerID, RawName)` for deterministic revision/notification behaviour.

### T-13 implementation evidence (r10)

- **T-13 — COMPLETE, 2026-10-10**, on branch `main`, uncommitted. Deliverable
  per increment §T-13: native `AddPrompt`/`RemovePrompts`/`list`/`get`, cold-start
  capability, notifications, stable mapping, safe errors, exactly-once route,
  supported-version evidence.
- Files: `internal/service/prompts_native_test.go` (new),
  `internal/upstream/prompts_native_test.go` (new),
  `internal/upstream/prompt_runtime.go`, `internal/upstream/runtime.go`.
- **Named tests:** `TestPromptNativeListGetAndChanged` + `TestPromptGetRoutingAndSafeErrors`
  in `internal/service` (increment-prescribed command:
  `go test ./internal/service ./internal/upstream -run '^(TestPromptNativeListGetAndChanged|TestPromptGetRoutingAndSafeErrors)$' -count=1`),
  with supporting upstream coverage `TestPromptPeerRequiresSession` and
  `TestPromptGetGuardedWirePaths` (deadline / protocol error / peer death /
  saturated admission / pre-admission cancel / mid-call cancel).
- **RED evidence (recreated by targeted production mutation, restore byte-identical,
  each failure an assertion on the named behaviour after successful compile):**
  - `TestPromptGetGuardedWirePaths/transport_failure_after_the_peer_disappears`
    (dropped the transport-rejection guard in `prompt_runtime.go`):
    `prompts_native_test.go:132: error = upstream_protocol_error, want code=backend_unavailable state=unknown`.
  - `TestPromptGetGuardedWirePaths/deadline_records_one_failure_and_never_retries`
    (`deadline_exceeded` mapped to `backend_unavailable` in `prompt_runtime.go`):
    `prompts_native_test.go:95: error = backend_unavailable, want code=deadline_exceeded state=unknown`.
  - `TestPromptNativeListGetAndChanged` raw-name leak assertion (`prompt_runtime.go`
    equivalent: `internal/service/prompts.go` `Published` appending `definition.RawName`
    to the mirrored description):
    `prompts_native_test.go:82: raw name "raw-z" leaked in response {...}`.
  - `TestPromptGetRoutingAndSafeErrors` sanitized-failure assertion
    (`internal/service/prompts.go` `get` returning the raw upstream error):
    `prompts_native_test.go:233: error leaked "raw-secret-name": calling "prompts/get": raw-secret-name /srv/private/socket transport failure`.
- **GREEN:** every mutation restored byte-identically, then
  `go test ./internal/upstream -count=1` and `go test ./internal/service -count=1` pass,
  `gofmt -l internal/upstream internal/service` empty, and the consolidated gate
  `go test ./... -count=1 -covermode=atomic -coverprofile=coverage.out` plus
  `go run ./cmd/covcheck -profile coverage.out` reports
  **source-line coverage 91.2% (3048/3341), gate 90%: true**.
- **Test-design notes (hang removal, behaviour unchanged):** the parked prompt
  handlers now unblock on a `t.Cleanup` release channel, and the dead-peer subtest
  uses `CloseClientConnections()` then `Close()` (a bare `Close()` blocks on the
  keep-alive request; `CloseClientConnections()` alone lets the SDK reconnect and
  succeed). The mid-call cancellation subtest asserts `failures==0`, matching the
  r8 breaker contract (line 291: breaker counts connect/transport5xx/malformed/deadline,
  excludes cancel).
- **Defect found and fixed (shared, pre-existing, also affected `CallTool`):**
  the SDK aliases `jsonrpc.Error` to `jsonrpc2.WireError` and wraps *transport*
  rejections as `jsonrpc2.ErrRejected` (code -32005, "rejected by transport"), so
  `errors.As(err, &jsonrpc.Error)` matched connection failures. A dead upstream
  was therefore classified `upstream_protocol_error` with **no** breaker fault.
  Observed at the wire before the fix: `attempt=0 raw=calling "prompts/get":
  sending "prompts/get": rejected by transport: Post "http://127.0.0.1:34637":
  dial tcp ... connection refused ... isRPC=true rpcCode=-32005 rpcMessage="rejected by transport"`.
  Fix: `transportRejectedCode = -32005` + `isTransportRejection` in
  `internal/upstream/runtime.go`, applied in both `CallTool` and prompt `GetPrompt`
  so a transport rejection records `FailureTransport` and returns
  `backend_unavailable` (breaker contract line 291 honoured). Full `internal/upstream`
  suite stays green, so no existing `CallTool` expectation regressed.
- **Open finding (outside T-13's named tests, pre-existing from the T-05/T-06
  service surface; reported, not changed):** `internal/service/protocol.go`
  `ValidateCallVersion`, `NegotiateInitializeOffer` and `AdvertisedProtocolVersions`
  have **no production callers** — the inbound version contract that the plan
  attributes to the SDK (`Server.protocolVersions` order, legacy-set narrowing,
  `mcp.CodeUnsupportedProtocolVersion` on latest calls) is enforced by the SDK,
  and the wire tests confirm it. The three helpers are currently dead production
  code exercised only by unit tests; deciding between wiring them as
  defence-in-depth and deleting them (with their unit tests) is a coordinator /
  planner call, not silently taken here.

### T-14 — end-to-end acceptance/observability (RED/GREEN recorded)

**RED (assertion-level, prompt wire absent).** The prompt surface wiring in
`cmd/mcp-bridge/serve.go` was temporarily disabled (`promptSurface.Register(server)`
and `promptSurface.Published(assembly)` short-circuited — a controlled revert,
restored byte-identically afterwards; sha256
`013d45347e6dbcb246a26c82c23eda1c6462b74f18a9d7865704e151d71f20b7` confirms the
restore). `go test ./cmd/mcp-bridge -run '^TestPromptPassThroughCT07$' -count=1`
→ `prompts_acceptance_test.go:328: timed out waiting for both colliding prompts
to be published` — a behavioural assertion failure for an absent CT-07
property, with the test compiled and executed (not build/fixture absence, per
T-14's RED form). Phase 1 still passed in that state because
`service.NewMCPServer` declares `HasPrompts: true` by design (cold-start
capability, AC-26), so the first absent property is publication, not capability.

**GREEN (wire restored).** Same command → `--- PASS: TestPromptPassThroughCT07
(0.85s)`. Re-run three times on the final tree: `ok` at 0.842s / 0.860s / 0.835s
(notification assertions are no longer timing-dependent).

**Targeted T-14 selection** (plan command):
`go test ./cmd/mcp-bridge ./internal/observe ./internal/service -run '^(TestPromptPassThroughCT07|TestPromptMetricsBoundedLabels|TestExampleConfigPassesCheck)$' -count=1`
→ `PASS: TestExampleConfigPassesCheck (0.00s)`, `PASS: TestPromptPassThroughCT07
(0.86s)`, `PASS: TestPromptMetricsBoundedLabels (0.00s)`; `internal/service`
reports `[no tests to run]` because none of the three names live there — the
`cmd/mcp-bridge` and `internal/observe` results are the evidence.

**Mandatory release gate** (plan's RELEASE AND RECOVERY region), all observed:
- `go build ./cmd/mcp-bridge` → ok
- `go vet ./...` → ok
- `go test -race ./...` → ok, 11/11 packages (cmd/covcheck, cmd/mcp-bridge,
  catalog, config, core, coveragecheck, observe, search, service, storage,
  upstream)
- `go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...` → ok
- `go run ./cmd/covcheck -profile coverage.out -root .` → `source-line coverage
  92.6% (3138/3390), gate 90%: true`; a fresh `-count=1` re-run of the same
  coverage command on the final tree reported `92.6% (3139/3390), gate 90%: true`
  (one conditionally executed statement differs between runs; both clear the gate)

**Test-design defects found and fixed while producing that GREEN** (both in
`cmd/mcp-bridge/prompts_acceptance_test.go`; recorded because they change what
the acceptance test can observe):

1. **Client protocol pin.** The acceptance client originally used the SDK
   default, which offers the pre-release `2026-07-28` version. The bridge
   negotiates that down to `2025-11-25` (SDK `negotiatedVersion`, the documented
   §4.5 contract), but go-sdk@v1.8.0 `Server.notifySessions` selects recipients
   by the version the *client offered* in `InitializeParams`
   (`sess.InitializeParams().ProtocolVersion < protocolVersion20260728`), so the
   negotiated-down session is still treated as SEP-2575 and its list-changed
   notifications go only to an active `subscriptions/listen` stream — which the
   legacy initialize fallback never opens, because `Client.Connect` only opens
   one on the `server/discover` path. Net effect: `prompts/list_changed` was
   unobservable for that client and the run was flaky (passed once when a
   debounced notification landed, then failed consistently). The client now
   pins `ProtocolVersion: "2025-11-25"` — one of the four accepted inbound call
   versions (increment OPEN DECISION 1) — with the reason in a code comment. This
   is pinned-SDK session classification, not a bridge defect; the pinned client
   is the client the bridge's inbound contract serves. It also corroborates the
   T-13 open finding that the version contract is enforced by the SDK, not by
   `internal/service/protocol.go` helpers.

2. **Unknown-prompt assertion checked message text, not code.** The SDK server
   itself answers an unregistered prompt name with `unknown prompt %q` and
   `jsonrpc.CodeInvalidParams` (go-sdk@v1.8.0 `mcp/server.go` `getPrompt`), so
   asserting that the message contains `invalid params` failed on a conformant
   bridge. The assertion now unwraps the error with `errors.As` and requires
   `jsonrpc.CodeInvalidParams`. (`session.GetPrompt` wraps the wire error with
   `%w`, so the code survives — `mcp/transport.go`.)

**Documentation/amendment synchronized in the same change:** `README.md` gained
the prompt-surface section (`prompts_disabled` per-server opt-out, opaque `p_`
names, provenance-in-description, sanitized errors, `prompts.json` persistence,
`bridge_catalog_prompts`, the accepted-version notification caveat) and the
prompt acceptance test description; the parent spec
`.omp/plans/headless-mcp-bridge.md` drops `prompts` from its non-goal list and
records the amendment pointer to this increment without restating its technical
content.

---

## Increment: mcp-grafana stack upstream onboarding (2026-10-13)

**Operator ask.** Add the seven mcp-grafana stack servers believed to serve
streamable HTTP on :8000 as template upstreams with env-var credential
placeholders, set a fictitious embedding model, and validate the document.

**Recon** (in-network probes from inside the `mcp` network via the Portainer
docker-proxy exec path; `POST /mcp` `initialize`):

| server | endpoint | result | auth |
|---|---|---|---|
| hass-mcp | `http://hass-mcp:8086/mcp` | 200 | none |
| mqtt-mcp | `http://mqtt-mcp:8000/mcp` | 200 | none |
| firecrawl-mcp | `http://firecrawl-mcp:3000/mcp` | 200 | none |
| ssh-mcp | `http://ssh-mcp:8000/mcp` | 401 | bearer required |
| google-maps-mcp-shttp | `http://google-maps-mcp-shttp:8000/mcp` | 200 | none |
| mcp-netutils | `http://mcp-netutils:3000/mcp` | 404 | SSE server (`--sse --sse-port=3000`); no streamable HTTP |
| mcp-restart-watcher | — | — | not an MCP server (docker-events restart loop) |

Listening ports differ from the assumption for hass-mcp (8086) and
firecrawl-mcp (3000), and google-maps was already a configured upstream
(`google-maps-mcp-shttp`), so the change adds four entries rather than seven.

**Change.** `configs/config.mcp-grafana-stack.yaml`: added `hass-mcp`, `mqtt-mcp`
and `firecrawl-mcp` (no credentials — probes returned 200 unauthenticated) and
`ssh-mcp` (`credentials.bearer` from `SSH_MCP_UPSTREAM_TOKEN`); replaced the
embedding `OPERATOR_SELECTED_MODEL` / `OPERATOR_SELECTED_REVISION` placeholders
with fictitious ids; rewrote the exclusion comment block to the verified state
(netutils SSE-only, restart-watcher not an MCP server, metamcp via aggregator).

**Verification.** `go run ./cmd/mcp-bridge --check-config --config
configs/config.mcp-grafana-stack.yaml` → `configuration valid: ... servers:24`
(20 previously configured + 4). Running the same check without
`SSH_MCP_UPSTREAM_TOKEN` fails closed (`secret must be nonempty UTF-8 ...`), so
the template requires that env var under its real name before it loads.

**Not committed.** Held for the plan's commit step; `GATE_OPEN` and this ledger
remain local.

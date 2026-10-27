Status: ready-for-agent

Revision: METADATA-r7

Technical state: REQUIREMENTS APPROVED at r4; DESIGN independently PASS and explicitly user-approved at r5. **Execution tasks APPROVED at r7 (2026-10-26):** r6 blockers B1-B6 CLOSED by authorized read-only research at HEAD `eb879ef`, evidence `.scratch/tool-metadata/integration-anchors-r7.md` (dated 2026-10-26); the execution-task ledger is anchored to exact source contracts. Independent critic review of the r7 task ledger: PASS, zero blocking findings, 5/5 anchors spot-verified (critic record in session transcript, 2026-10-26). User execution-task checkpoint: explicitly approved via "Approve r7 tasks" selection (2026-10-26). Gates: tickets 02-06 authorized for sequential implementation under TDD; ticket06 owns the integrated final gate (full suite/race/vet/covcheck SOURCE-LINE>=90%). This approval authorizes code work only; the marker is not shippable alone and planning artifacts ship with the implementation.

## Problem Statement

Gemini reportedly repeatedly requests permission for read-only operations through mcp-bridge. Confirmed discovery and publication discard upstream Tool metadata, including annotations, leaving clients without a reliable distinction between discovery/direct reads and potentially mutating dispatch. Second-brain DEF-014 reports the incident, but its blanket insert/replacement safety suggestions are not accepted semantics. Missing annotations plausibly contribute [INFERENCE]; actual Gemini application/version/policy and prompt causality remain unknown.

The bridge needs accurate built-in classification, supported metadata transmitted from every backend through the catalog lifecycle, and explicit metadata overrides for **any pinned server**, including server-scoped defaults and exact-tool exceptions. The result is an advisory client-policy surface, not authorization or a promise of zero confirmations.

## Solution

Retain the approved r5 design: existing discovery, immutable catalog generations, durable catalog, pin publication, reload and MCP wire/session behavior. Keep **SDK-normalized upstream metadata** separate from effective metadata. Derive pinned metadata from upstream → explicit server defaults → exact-tool exceptions. Never persist corrections as upstream truth. Direct pins and describe share an effective view with normalization/provenance disclosed.

Writer emits schema 2; new reader strictly verifies schemas 1 and 2 with version-specific checksums. Old binaries cannot read schema 2. Old-binary rollback restores a retained offline pre-upgrade schema-1 snapshot and matching config, not a downgraded schema-2 file. These are approved design decisions, not implemented or tested feature behavior.

r7 preserves the r6 organization around the **six already published vertical tickets**, not independently shippable horizontal layers. Ticket01 research is complete; no implementation has begun. Exact integration anchors were previously blocked by a source HTTP401 incident; that incident is **resolved** (see dated history below) and all anchors are now admissible via `integration-anchors-r7.md`.

### Requirements contract — approved r4, unchanged

R1-R11 and AC1-AC13 retain their approved meanings; r6 changes neither requirements nor r5 design.

- **R1 — Built-ins:** Advertise `search_tools` and `describe_tool` as read-only/idempotent discovery. Keep generic `call_tool` potentially mutating, potentially destructive, non-idempotent and open-world. Selecting a read target does not statically reclassify it.
- **R2 — Supported metadata:** Forward every metadata field represented by Go MCP SDK v1.8.0 Tool: description, title, icons, annotations and `_meta`. Annotations include title and four standard hints; retain arbitrary supported `_meta` keys/values. Name/input/output schemas remain protected contracts, not override targets. Unsupported top-level/annotation/execution fields are explicitly outside this SDK inventory. Initialize identity/capabilities/instructions, auth, transport/session, arguments/results, resources and prompts are separate contracts.
- **R3 — Fidelity/normalization:** Preserve supported metadata through every server's discovery/catalog/durable restart/describe/direct pins. Retain pointer-hint nil/false/true, especially destructive/open-world explicit false versus true defaults. SDK scalar read-only/idempotent absent and false normalize to MCP-default=false; preserve semantics, not unknowable source key presence. Preserve annotation-object presence where represented, supported nested `_meta` and schemas. Override omission and explicit false remain distinct.
- **R4 — Defaults/exceptions:** Any configured server can have opt-in metadata defaults for its pins plus exact raw-tool-name exceptions. Mixed services can override broad assertions. No other-server, unpinned, built-in or generic-dispatch effect. Validate atomically; explicit false honored, absent patch members inherit. No wildcard/global automatic defaults.
- **R5 — Source/effective:** Keep normalized upstream separate; precedence upstream → server defaults → exact tool patch. Describe shows upstream/effective/provenance; direct list agrees. Tool removal exposes defaults/source; default removal exposes source except retained tool patches. No correction resurrection after restart.
- **R6 — Lifecycle:** Publish source-metadata-only and override-only changes, reload patch changes/removals, remove departed exposure. Preserve startup/refresh/reload failure isolation and actual SDK transport/session behavior. No fabricated freshness or retrospective client-cache guarantee.
- **R7 — Safety:** Advisory metadata is not authorization. No name heuristics, blanket insert idempotency or replacement additivity; retain supported source claims unless explicitly overridden. Document broad server-default/mixed-service risks and possible suppression of useful client warnings.
- **R8 — Client boundary:** Direct pins are differentiable policy subjects, generic dispatch is not. No client/global-policy edits or trust bypass. Actual Gemini smoke is conditional on identified version/policy/safe access, separate from core correctness.
- **R9 — Compatibility:** Preserve canonical `<server_id>__<raw_name>` pins, opaque long-tail identities, coexistence, schemas, routing, auth/results and distinct initialize identity. Existing configs/legacy records stay usable without invented source claims. Use established version/checksum/rollback contract, not guessed compatibility.
- **R10 — Verification:** Deterministic unit/integration/controlled real-wire negative/refresh/reload/restart tests and existing regressions. No mutable real-upstream probe. Full prescribed checks and custom **SOURCE-LINE >=90%** gate before shipping; no aggregate statement-% substitution/package exclusions/weakened assertions. No commands run in this planning slice.
- **R11 — Operations/docs:** Existing finite/redacted lifecycle/config/storage reporting, actionable failures, no sensitive payloads/new telemetry. Document support/normalization/provenance/precedence/removal/lifecycle/policy/operator risk.

## User Stories

1. As a Gemini user, I want read-only discovery annotated so compatible policy can distinguish it from writes. (R1,R8)
2. As a cautious user, I want generic dispatch conservative so write routing never looks universally read-only. (R1,R7)
3. As a frequent reader, I want accurate direct pins so policy can match the operation. (R3,R8)
4. As an upstream operator, I want supported title/icons/annotations/_meta transmitted rather than discarded. (R2,R3)
5. As a multi-backend operator, I want equal preservation for every server rather than a second-brain special case. (R3)
6. As a metadata author, I want destructive/open-world false preserved rather than becoming a true default. (R3)
7. As an auditor, I want scalar normalization disclosed rather than invented source-key provenance. (R3,R7)
8. As an operator, I want defaults configurable on any pinned server without repeating shared assertions. (R4)
9. As a mixed-service operator, I want exact-tool exceptions so write tools override broad defaults. (R4,R7)
10. As an operator, I want absent and false patches distinct so inheritance is predictable. (R4,R5)
11. As an operator sharing raw names across servers, I want exact server isolation. (R4)
12. As an operator, I want defaults limited to pins, not unpinned tools or generic dispatch. (R4,R8)
13. As an auditor, I want source/default/tool provenance so each assertion's origin is clear. (R5,R7)
14. As a client developer, I want describe/direct list agreement. (R5)
15. As an operator removing a tool patch, I want current defaults/source restored. (R5,R6)
16. As an operator removing defaults, I want current source plus remaining explicit tool patches. (R5,R6)
17. As a client, I want metadata-only refreshes visible despite unchanged routing. (R6)
18. As an operator during outages, I want honest stale/failure status. (R6,R11)
19. As an administrator, I want invalid patches atomically rejected rather than partial relabeling. (R4,R7)
20. As an existing user, I want names/schemas/auth/results unchanged. (R9)
21. As a maintainer, I want deterministic existing-wire-seam proof without live mutations. (R10)
22. As a security-conscious operator, I want server-wide assertion risks explicit, not an implicit trust bypass. (R7,R8,R11)
23. As a client operator, I want Gemini observations tied to actual version/policy, not universal claims. (R8)
24. As a maintainer, I want full-suite and custom source-line coverage evidence for all behavior, not a narrow happy path. (R10)

## Implementation Decisions

**Unchanged approved r5 technical contract.** Exact implementation symbols selected for the proposed task view appear only under Further Notes; missing integration evidence does not reopen approved architectural decisions.

### Descriptor representation

1. Extend the canonical tool definition with one immutable supported-metadata value, not a backend-specific table. Carry title/icons/supported annotations/_meta; description remains its existing authoritative field. Preserve pointer/annotation-object presence. Name/schemas are not duplicated as overrideable metadata.
2. Convert SDK metadata once at discovery into a JSON-owned representation suitable for catalog/storage and once back at publication. Copy mutable ownership boundaries, then keep admitted values immutable; no repeated avoidable copying/generic round-trips.
3. Source means SDK-normalized upstream, not original bytes. Scalar read-only/idempotent false retains default=false without fabricated key provenance; destructive/open-world pointers retain nil/false/true. No `MCPGODEBUG` changes or invented annotation objects.
4. Include metadata in revision/hash/equality and every discovery/catalog/clone/persist/restore conversion. Supported inventory remains description/title/icons/annotations/_meta; annotation title/four hints; opaque supported `_meta` keys. No unsupported-field fidelity claim.

### Built-ins and overrides

5. Search/describe read-only=true, idempotent=true, conservative open-world=true, destructive unspecified. Generic call read-only=false, destructive=true, idempotent=false, open-world=true. Direct pins use effective metadata; unpinned describe uses source only.
6. Optional server-owned **`tool_metadata`** owns **`defaults`** patch and **`tools`** exact-raw-name-to-patch mapping. Existing pin entries unchanged; no global table/cross-server selector.
7. Patch allowlist: description/title/icons/annotations/_meta; annotation title/readOnlyHint/destructiveHint/idempotentHint/openWorldHint. Icons match supported SDK shape; `_meta` supported arbitrary JSON. Reject schema/name/auth/initialize fields, invalid types/unknown config keys.
8. Precedence source → defaults → tool. Recursively overlay object members; replace scalars/arrays. Explicit false/empty values override; missing members inherit; protocol-valid null is a value, not deletion. Removing patches restores current prior layers, never old effective state.
9. Defaults affect only same-server current pins and may be configured with no pins while affecting none. Exact entries must match configured pins. Validate duplicate keys/selectors/targets atomically; bounded redacted field diagnostics. Include patches in public config identity, not credential fingerprints.
10. No automatically installed read-only defaults, name-based exception inference or source repair. Document mixed-service exceptions and broad operator risk; annotations never grant execution authority.

### Views, lifecycle and storage

11. Derive once per generation/pin for describe/direct list. Persist source, not effective corrections. Describe effective metadata plus **`upstream_metadata`** (SDK-normalized) and **`metadata_override_fields`** (field path/winning `server_default` or `tool`); empty list when no correction. Search need not gain fields.
12. Compare complete effective SDK descriptor plus routing/binding, including output schema. Re-register source/default/tool-only changes; remove departed exposure. Retain locking/generation handoff and accepted-state agreement; independent requests spanning publication may observe successive generations.
13. **Schema-2 writer; strict schema-1-or-2 new reader; old reader rejects schema 2.** Freeze complete schema-1 disk representation/canonical checksum; schema 2 adds optional source metadata, not effective patches. Config schema version unchanged. Select decoder by version but strictly decode full document/unknown/trailing checks, verify that version's checksum, then validate identities/schema. Never verify legacy bytes through new types or treat future versions as current.
14. Convert verified legacy state into canonical memory with metadata absent; Load alone does not rewrite. Next normal accepted Save writes schema 2 and checksum, possibly during startup. Preserve existing trusted-state replacement, private temporary file, file sync, atomic rename/directory sync. Invalid state cannot be overwritten to force migration; persistence failure rejects admission.
15. Rollback is restore, not reverse migration. Before starting new binary, stop old process and retain exact validated schema-1 snapshot plus matching config separately from active state. New reader accepts either version; old binary never receives schema 2. To roll back, stop new process, preserve schema-2 state separately, restore old backup/config, start old binary. No stripping/downgrade/empty-state workaround/identity erasure.
16. Restore returns to backup generation; intervening local catalog/cache/config progress is absent from old-binary view, newer state retained for forward recovery. It undoes no upstream operations. Fresh schema-2/no verified backup means no state-preserving old-binary rollback; stay on new reader or obtain reviewed recovery. No new backup service.
17. Preserve shared inbound server/nil SDK Streamable HTTP options: stateful sessions, SSE responses, SDK idle timeout zero/no SDK expiry. Separate admission enforces configured SessionIdle/MaxSessions. Config requires session/refresh values; examples 10m/5m and validation bounds are not implicit runtime defaults.
18. Upstream dedicated sessions connect lazily/reuse runtime lifetime; reload/refresh reuse matching clients; refresh cannot rotate/create clients/membership. Cold restore contacts no upstream and defers pins; loop publishes cold generation then immediately triggers existing live refresh resolving pins. No added wait, cold exposure or migration-time discovery.
19. Reload discovery failure rejects whole candidate; refresh retains failed-server complete source/reason while peers advance. Persistence before acceptance; failure marks storage readiness false/propagates error. Only accepted assembly invokes publication; rejected-pass metrics use current assembly. SIGHUP retains watcher/publication without replacing handler.
20. Preserve SDK supported signaling, subscription caveat and legacy re-list fallback. Explicit re-list after successful publication is the guarantee, not every-client notification/cache refresh.

### Reporting

21. Current finite/redacted JSON logger writes stdout synchronously under mutex, best effort. Encoding failure resets for future record, no replay/in-process retention; external collection/retention unknown. No stronger delivery claim.
22. Existing warn/reload/failure/invalid_config with typed config projection or finite generic fallback, no raw errors/arbitrary new severity/events/metadata dumps. Keep recursive redaction.
23. Refresh visibility uses existing snapshot status/reason, readiness/catalog metrics, not a new JSON event. Rejected-pass metrics use accepted assembly; persist failure changes storage readiness/error; reload failure current operator-log/metrics. No metadata/patch payload labels or new telemetry.

## Testing Decisions

### Confirmed seam and baseline result

User confirmed existing real-bridge HTTP MCP seam with controlled upstreams and reload/restart. Requirements r4/design r5 approvals are separate. Existing baseline command supplied in prerequisite evidence: `go test ./cmd/mcp-bridge -run '^TestServeHybrid(PreloadedToolsListExposure|Coexistence|SIGHUPReload)$' -count=1` — PASS/exit0/package2.128s. This is **not metadata/schema2/full-suite/Gemini verification**. Planner ran no command.

Use `/mcp`, actual stateful SSE handshake, protocol2025-11-25, returned `Mcp-Session-Id`, initialized notification HTTP202, then standard JSON-RPC `tools/list` or `tools/call`. Search/describe/call are `tools/call.params.name`, **not JSON-RPC method names**. Complete payloads and the confirmed harness bindings (r7 anchors B5) are in the task view. No new transport seam or live mutable upstream.

### Acceptance criteria — approved outcomes retained

Feature checks **planned / NOT RUN**; baseline seam does not satisfy feature ACs. Actual Gemini smoke conditional on access.

| AC | Observable scenario/outcome | R IDs | Method |
|---|---|---|---|
| AC1 | Built-ins list correct classifications; discovery/description cause zero upstream operations. | R1,R7,R8 | Wire/counters/semantic review. |
| AC2 | Every server/path preserves supported description/title/icons/annotations/_meta/output schema through catalog/persist/restart/describe/direct pins. | R2,R3,R9 | Inventory mapped unit/serialization/wire. |
| AC3 | Scalar default normalization disclosed; pointer nil/false/true preserved; annotation absence/nested extension values retained; omitted patches inherit, explicit false overrides. | R2,R3,R4,R7 | Tables/source/effective/wire. |
| AC4 | One server's defaults affect only its pins; exact mixed-service exception wins; same-name other server/unpinned/built-in/generic unaffected. Repeat different server. | R4,R7 | Config/effective/real wire. |
| AC5 | Source/default/tool disagreement has distinct source/provenance and equal describe/list; removal restores current prior layer, no name/schema/auth/initialize changes. | R4,R5,R9 | Layer/removal wire/source persistence. |
| AC6 | Source/default/tool-only changes without schema/routing change publish/remove fields and update descriptor/revision state. | R3,R5,R6 | Refresh/reload/same-session re-list. |
| AC7 | Real defaults/exceptions reload, departed pin/tool/server and restart retain current layers/no resurrection; startup delayed pin visibility unchanged. | R4,R5,R6,R9 | Watcher/SIGHUP/refresh/restart. |
| AC8 | Invalid/default/tool/target/discovery/persist failure has no coercion/partial success; rejected reload prior generation; failed refresh stale source/reason/healthy peer advancement; actionable redacted reporting. | R6,R7,R11 | Negative tests/reporting. |
| AC9 | Dubious supported source hints survive without heuristics; only explicit corrections change effective claims; docs cover advisory/normalization/precedence/operator risks. | R3,R7,R11 | Fixtures/docs review. |
| AC10 | Existing configs/legacy/coexistence/auth/routing pass; legacy usable without invented claims; names/schemas/results/auth/initialize unchanged; strict actual format compatibility proved. | R9 | Existing regressions/legacy/version/auth fixtures. |
| AC11 | Retained list/describe-call/controlled reload/restart wire evidence; zero mutable real-upstream probes. | R8,R10 | Requests/responses/counters. |
| AC12 | Required full checks pass and custom covcheck proves **SOURCE-LINE>=90%** with required profile; unchanged RED→GREEN; no narrowed assertions/packages or statement-% substitute. | R10 | Exact commands/exits/report. |
| AC13 | No trust/new telemetry/transport change; docs/policy boundary; actual Gemini version/policy/safe observation or explicit NOT RUN/access reason, not a core-test waiver. | R8,R9,R11 | Scope/docs/conditional smoke. |

Traceability unchanged: R1→AC1; R2→AC2,AC3; R3→AC2,AC3,AC6,AC9; R4→AC3-AC5,AC7; R5→AC5-AC7; R6→AC6-AC8; R7→AC1,AC3,AC4,AC8,AC9; R8→AC1,AC11,AC13; R9→AC2,AC5,AC7,AC10,AC13; R10→AC11,AC12; R11→AC8,AC9,AC13.

### TDD and final gate

Authorized engineer writes compiling public-behavior RED before each ticket's production edits, then same test GREEN. No compile/setup failures counted as RED, no copied helper behavior, no weaker assertion/AC. Full wire tests need not await final ticket; each vertical ticket must finish its own end-to-end behavior and focused regressions. TDD guide directs; engineer owns tests/code. Gemini is observational, not deterministic RED/GREEN.

Final exact commands from README449-456, all feature results NOT RUN: `go test ./...`; `go test -race ./...`; `go vet ./...`; `go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...`; `go run ./cmd/covcheck`. Require SOURCE-LINE>=90%. Coordinator's integration engineer runs final gate once after all tickets, not parallel worker storms. Focused tests may run only once their task assignment explicitly authorizes verification; this planner runs none.

## Out of Scope

No app/test/config/gate edits, implementation/worker dispatch, commands, deployments/images/registry operations or independent commit during task planning. No safety heuristics/backend special case, global implicit defaults, wildcard/cross-server/unpinned overrides, name/schema/auth override, extra result/prompt/resource/initialize forwarding, client-policy/trust change, new telemetry, transport/session/dependency/MCPGODEBUG change, literal original scalar-key provenance or universal prompt reduction.

No old-reader schema2 support, checksum relaxation, permissive unknown/trailing decoding, silent downgrade/state deletion, automatic backup service or lossless old-binary rollback promise. No source-auth bypass, credential hunt or local source/LSP fallback after denial. An incomplete task contract is not a request for execution approval, and engineer must not research missing integration facts opportunistically.

## Further Notes

### Approvals and revision provenance

r4 publication/requirements independently PASS; user selected “Approve requirements” at2026-10-26T12:51:28Z. r5 design independently PASS, no material blockers, critic confirmed R/AC unchanged; user explicitly selected **“Approve design”** on2026-10-26 after schema2/strict1-or2/offline backup/restore rollback and unchanged auth/sessions/logging were presented. Preserve both historical checkpoints. Earlier seam and six-ticket publication approvals are not execution-task approval.

r5 reviewed specification SHA256 before bookkeeping: `3baa664e1fc5e094a429dca994d9c9eaa4512277d841c8fc8fa81de52ae475c3`; prerequisite evidence SHA256: `a3552434687e24fe708588961054f74b6772525ca8299dc0209e5f0ed70f0fb7`. These are historical, not hashes of r6/r7.

**r6 task-only revision:** preserves requirements/design technical contract, replaces the old horizontal task scheduling view with the six published vertical tickets and explicit stable-obligation mapping.

**r7 task revision (2026-10-26):** tasks view only; requirements/design approvals untouched. Closes r6 blockers B1-B6 with `.scratch/tool-metadata/integration-anchors-r7.md` (authorized read-only research at HEAD `eb879ef`); records the Published-equality integration decision (see "r7 integration decision" below); confirms the six vertical tickets, dependency graph and T1-T7 mapping against the anchors. r7 has no critic PASS and no user execution-task approval; implementation has not started; gate CLOSED/no gate file changed.

### Current access incident and evidence admissibility

**RESOLVED (2026-10-26):** The earlier coordinator `call_tool` HTTP401 Unauthorized incident (trace `b7f7c64ff2f58c98d91af8c9599161f4`) is closed. Authorized read-only research at HEAD `eb879ef` produced `.scratch/tool-metadata/integration-anchors-r7.md`, which closes B1-B6 and is admissible evidence for r7. The previously reported MetadataConfigAnchors fallback remains inadmissible; it was bypassed and none of its content is cited. Earlier authorized prerequisite retrieval (U2/U3/U5 facts and baseline smoke) remains valid. No source/external/connector probing occurred in this planning slice.

### Stable obligation IDs versus vertical execution tickets

Retain T1-T7 meanings; **do not silently repurpose them as ticket IDs**:

| Stable obligation | Retained meaning | Vertical ticket delivery |
|---|---|---|
| T1 | Read-only U2/U3/U5 evidence closure | Ticket01 COMPLETE; the former B1-B6 source-contract blockage is separately closed (r7), not permission to repeat broad research. |
| T2 | Upstream/core/catalog/durable source preservation | Ticket03 complete end-to-end, not ingress-only. |
| T3 | Config validation, pure effective derivation and provenance | Ticket04 defaults, then ticket05 exact exceptions. Each completes its own visible behavior. |
| T4 | Built-in/effective descriptor publication, describe consistency and lifecycle invalidation | Ticket02 built-ins independently; ticket03 source egress;04 defaults egress;05 exception egress. |
| T5 | Existing real HTTP wire RED/GREEN and controlled lifecycle fixtures | Within each ticket02-05 before that ticket's implementation; consolidated evidence ticket06. Not a horizontal gate delaying all tests to the end. |
| T6 | Docs/changelog and existing failure visibility/redaction | Within each product ticket02-05; final review ticket06. No docs-only substitute for a vertical feature. |
| T7 | Integrated acceptance, full checks/source-line gate/reviews and optional Gemini evidence | Ticket06 only, after02 and05. |

**Published graph retained exactly:** ticket01 has no ticket dependencies and is complete; ticket02 has no technical ticket dependency; ticket03 depends01; ticket04 depends03; ticket05 depends04; ticket06 depends02+05 (05 transitively covers01/03/04). The r4/r5 horizontal T scheduling graph is explicitly superseded as a scheduling view, not as a change to T obligations. Global execution-task approval still blocks all product tickets.

### Integration ownership, inputs and change boundaries

The **engineer assigned by the coordinator** is the sequential integration owner for tickets02-05; coordinator owns gate progression and final ticket06 orchestration. No agents are dispatched by this document. Avoid concurrent writers in shared core/service/storage/config files. Each ticket integrates its own public behavior, tests and docs before moving to its dependent ticket; no representation-only handoff, no temporary shims, no alternate metadata pipeline.

Inputs are approved r4 requirements/r5 design, six published issue checklists, prerequisite evidence and the admissible r7 exact anchors (`.scratch/tool-metadata/integration-anchors-r7.md`). Only application paths explicitly mapped by those anchors may be edited by a later approved implementation assignment. Engineer must use LSP references before exported changes, fresh edit anchors, and the code-access contract; these are edit-safety procedures, not authorization for open-ended discovery.

### r7 exact anchors: closed, admissible

All exact source-contract facts are supplied and admissible in `.scratch/tool-metadata/integration-anchors-r7.md` (research at HEAD `eb879ef`, 2026-10-26). Summary of the load-bearing facts confirmed against the selected API contract:

- **B1 (descriptor/definition/discovery):** `core.ToolDefinition` core.go:27-31 (`RawName, Description, InputSchema, OutputSchema` json.RawMessage), stored as `Binding.Def` core.go:23; `DefinitionDigest` core.go:220; `revisionEntry` core.go:220-233; `canonicalSchema` core.go:241; construction sites upstream/dedicated.go:103-108, service/reload.go:659-663, service/index.go:15-26 (`DefinitionHash: core.DefinitionDigest(...)` at index.go:23). SDK v1.8.0 `mcp.Tool` protocol.go:1895+ (Meta `_meta,omitempty` value, Annotations pointer, Name, Title, Icons, InputSchema/OutputSchema any); `ToolAnnotations` :1960-1990 with pointer `DestructiveHint/OpenWorldHint` and **bare-bool** `IdempotentHint/ReadOnlyHint` (absent-vs-false unrepresentable on the wire — confirms R3 normalization); `Icon` :1484-1496 (`src`, `mime_type`, `sizes`, `theme` omitempty). `Dedicated.Discover` dedicated.go:84, paginated, error on malformed page, sorted by RawName :126, sole caller path wiring.go:197→reload.go:472.
- **B2 (storage freeze):** full Envelope/Binding/Server/Tool/Embedding tags snapshot.go:41-98; `Checksum()` :155-165 (canonical envelope, SHA-256 hex); `Validate()` :252-333; fixture `validSnapshot()` store_test.go:27-63, variant `twoServerSnapshot` reconcile_test.go:33.
- **B3 (service assembly/pin/describe):** Assembly reload.go:71-81; pin join `core.AddPins` reload.go:510-530; `definitionsFromSnapshot` reload.go:639; **`Published()` tools.go:96-134 with binding-only equality :119-122 `reflect.DeepEqual(previous.Binding, next[name].Binding)`**; dispatch :211; describe request :465-468 and revision gate :476-478, payload keys :484-497; search_tools per-result keys :837-848, top-level :863-867.
- **B4 (config):** config.Server config.go:37-43; `pinToolNamePattern` :35; Parse :87 with validateNode :126, duplicate diagnostics (server_id_duplicate :175-180, pin_duplicate :261-269), typed Diagnostics :65; `Fingerprint()` fingerprint.go:51-92 over `publicProjection` + `projectedServer` (secrets excluded), consumed reload.go:391 and storage.Reconcile :541-549. No Config/Server clone exists; Config immutable post-Parse.
- **B5 (harness):** `writeBridgeConfig` serve_test.go:124, `writeBridgeConfigInterval` :132, `bridgeServerSpec` :115, `freeAddress` :51, `fakeEmbeddings` :85, `waitFor` :308, `probeBody` :323; `startHybridBridgeWithUpstream` serve_hybrid_exposure_test.go:48; `mcpSend` exposure_test.go:104 (Bearer `serveClientToken` from SERVE_CLIENT_TOKEN env, Mcp-Session-Id + protocol 2025-11-25); reload helpers `fakeUpstreamReadWrite` serve_hybrid_reload_test.go:26, `toolChangeWatcher` :87-106, `connectBridgeToolChanged` :116, `listToolNames` :135, `listToolsContains` :150, `directCallTool(Text)` :156/:162; `fakeUpstreamReadAndWriteTools` serve_hybrid_coexistence_test.go:24.
- **B6 (docs):** `CHANGELOG.md` `[Unreleased]` (Keep-a-Changelog) exists (commit a49c0dc); feature entries append under Added/Changed. README tool-surface area remains the operator-docs destination.

The selected new API contract below is unchanged; all its former B1-B6 "limitation" cells are resolved by the anchors file. If an engineer finds a genuine contradiction between the anchors and a selected contract at implementation time, return to the planner with the exact anchor — do not silently add a shim.

### r7 integration decision — metadata participates in equality and revision (implements approved decision 12)

Confirmed fact: `Published()` equality (tools.go:119-122) is **binding-only** — `reflect.DeepEqual(previous.Binding, next[name].Binding)`. Metadata-only changes would not republish under the unchanged struct. Concrete mapping, consistent with approved design decision 12 ("compare complete effective SDK descriptor plus routing/binding; re-register source/default/tool-only changes"), without reopening design:

1. **Extend the compared value, not the equality expression.** `core.ToolDefinition` gains `Metadata ToolMetadata` (the selected core contract). Because `Binding.Def` is a `ToolDefinition`, the existing `reflect.DeepEqual` on Binding then automatically covers metadata-only changes for every published binding — no change to tools.go equality logic, no new comparison helper, no new assembly revision counter.
2. **Extend `DefinitionDigest` to include metadata.** `DefinitionDigest(serverID, rawName, def)` (core.go:220) folds a canonical serialization of `Metadata ToolMetadata` (via the same `Clone`+canonical-JSON ownership used everywhere else; zero-value metadata canonicalizes to the metadata-absent form) into the definition hash. Consequently `Tool.DefinitionHash` (service/index.go:23), `revisionEntry` and `Bridge.Revision()` (core.go:187) change on metadata-only source changes, and the describe revision gate (tools.go:476-478) plus same-session explicit re-list see the new catalog revision.
3. **Effective overrides ride the same path.** Pinned effective metadata (source → server defaults → exact tool patch, derived once per generation, decision 11) is materialized into the pinned `Binding.Def.Metadata`; defaults/patch changes alter the public config fingerprint (`tool_metadata` joins `projectedServer`/`publicProjection`, B4), driving the existing candidate reload, and are then caught by the same binding equality and digest. Storage schema 2 persists **source** metadata and hashes computed by the extended `DefinitionDigest`; legacy schema-1 bytes are never re-hashed through new types.
4. **Non-goals:** no `Published()` equality rewrite, no separate metadata revision entry in `revisionEntry`, no publication of metadata-only diffs as new generations beyond the existing register/replacement flow, no new telemetry.

### Selected new implementation API contract — proposed, not existing code

The planner selects the following signatures/types so an engineer is not asked to make architecture choices. They are **new definitions to implement after tasks-critic review and user execution-task approval**, not claims that current code compiles against them. r7 anchors resolve the former integration limitations (noted per row); a genuine contradiction at implementation time returns to the planner, it is not patched locally.

| New/changed target | Concrete selected API/data contract | Slice / integration limitation |
|---|---|---|
| New `internal/core/tool_metadata.go`; change `internal/core/core.go` ToolDefinition | `type ToolMetadata struct { Title string; Icons json.RawMessage; Annotations json.RawMessage; Meta json.RawMessage }`, JSON keys title/icons/annotations/_meta with omission for zero/absent fields. Add `Metadata ToolMetadata` to ToolDefinition. Methods `(m ToolMetadata) Clone() ToolMetadata` and `(m ToolMetadata) Validate() error`; Clone deep-copies raw bytes, Validate admits only the supported shape while retaining normalized scalar/pointer semantics. Description stays in ToolDefinition.Description. | Ticket03/T2. Existing definition hash/clone/revision/reference integration B1; exact SDK Icon field shape B1. |
| New `internal/upstream/tool_metadata.go`; discovery conversion `dedicated.go:103-108` | `func metadataFromSDKTool(tool *mcp.Tool) (core.ToolMetadata,error)`; marshal supported nested containers once, preserve nil annotation object/pointer hints; clone into canonical ownership. Discovery propagates conversion failure through its existing error result, not omission. | Ticket03. Existing Discover method complete signature/error/callsite map still B1; do not add a parallel discovery path. |
| New `internal/service/tool_metadata.go` | `type MetadataOverrideField struct { Path string; Layer string }` with JSON keys `path`/`layer`; Layer exactly `server_default` or `tool`. `type effectiveMetadataView struct { Definition core.ToolDefinition; OverrideFields []MetadataOverrideField }`. `func deriveToolMetadata(def core.ToolDefinition, defaults, tool config.MetadataPatch) (effectiveMetadataView,error)`. Returns owned metadata, preserves source/schema/name, and sorted winning field-path provenance. No patches yields empty nonnil provenance list. | Ticket03 establishes source-only view;04 defaults;05 tools. Exact immutable assembly storage/accessor/callsite attachment B3; no invented current Assembly fields. |
| New `internal/service/tool_metadata.go`; `tools.go` publication/describe | `func sdkToolDescriptor(name string, def core.ToolDefinition) (*mcp.Tool,error)` builds complete effective descriptor including input/output schemas; one shared conversion for pinned publication/describe metadata. No helper may infer hints. Invalid metadata rejects candidate before publication, not a swallowed AddTool error. | Ticket03/T4. Exact generation admission and pin-to-catalog identity join B3 needed before executing integration. |
| New `internal/config/tool_metadata.go`; change existing Server | `type MetadataPatch map[string]json.RawMessage`; `type ToolMetadataConfig struct { Defaults MetadataPatch; Tools map[string]MetadataPatch }` with YAML keys defaults/tools. Add `ToolMetadata *ToolMetadataConfig` with YAML `tool_metadata,omitempty` to Server. Methods `(p *MetadataPatch) UnmarshalYAML(node *yaml.Node) error`, `(p MetadataPatch) Clone() MetadataPatch`, `(p MetadataPatch) Validate() error`; `func (m *ToolMetadataConfig) ValidatePins(serverID string,pins []map[string]string) error`. Absent config/patch remains absent; false/empty members retained; JSON-valued nested `_meta` supports null as a value. Integrate through existing typed diagnostics/duplicate policy, not raw error text. | Ticket04 accepts defaults; ticket05 activates exact tools. Parser/fingerprint/public projection contracts B4 block any code use of these declarations; no source fallback to discover them. |
| New `internal/storage/snapshot_v1.go`, `snapshot_v2.go`, `snapshot_codec.go`; existing Snapshot/Tool/Load/Save | Freeze `snapshotV1`, `envelopeV1`, `bindingV1`, `serverV1`, `toolV1`, `embeddingV1` with **complete exact existing fields/tags/order/checksum representation**, not the partial relay. Add optional `Metadata core.ToolMetadata` to canonical/new schema2 Tool. Retain public `Load() (*Snapshot,error)`, `Save(*Snapshot) error`, `Checksum() (string,error)`, `Validate() error`; writer SchemaVersion2, decoder dispatch accepts1/2. New private `func decodeVersionedSnapshot(data []byte) (*Snapshot,error)`, `func decodeSnapshotV1(data []byte) (*Snapshot,error)`, `func decodeSnapshotV2(data []byte) (*Snapshot,error)`, `func encodeSnapshotV2(snapshot *Snapshot) ([]byte,error)`. Strict per-version DTO decoding/checksum occurs before conversion. | Ticket03. Full legacy DTO/Checksum canonical field order/ensureReplaceable branches and clone paths B2; partial types above are deliberately not implementation-ready DTO definitions. No added schema2 fields to legacy checksum. |

Do not leave the engineer to choose alternate types, patch representation, provenance encoding, version dispatch or transports. If admissible source evidence contradicts one selected API's existing import/ownership contract, return to planner with the exact anchor; do not silently add a shim. New type declarations above are planning contracts only—no scaffold or source file has been created.

### Standard MCP wire requests and controlled fixture contract

All POST requests use the existing harness URL ending **`/mcp`**; real host/port and synthetic client authorization must be provided by the existing harness, not production config. Helper/auth bindings are confirmed (r7 anchors B5: `mcpSend` sends Bearer `serveClientToken` from the SERVE_CLIENT_TOKEN env set by the helpers); confirmed bindings are not permission to omit auth tests. Use `Content-Type: application/json` and the current accepted JSON/SSE response negotiation. Do not change nil SDK options to simplify decoding.

1. Initialize: `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"metadata-fixture","version":"1"}}}`. Capture returned `Mcp-Session-Id`; subsequent requests carry that session and negotiated protocol version as the existing harness does.
2. Initialized notification: `{"jsonrpc":"2.0","method":"notifications/initialized"}`; existing seam expects HTTP202. Do not add an id or treat it as a tool call.
3. List: `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`; decode the existing SSE `data:` response through the real seam. If pagination is present, traverse returned cursor using the same method.
4. Search: `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search_tools","arguments":{"query":"metadata fixture"}}}`. Obtain current catalog_revision and matching tool_name from the search result, not a fabricated opaque id. Confirm exact required search arguments against the confirmed search payload keys (r7 anchors B3: per-result `tool_name, server, upstream_name, description, score, catalog_revision, catalog_status, availability, last_success_at, pinned`); do not create a raw `search_tools` method.
5. Describe: `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"describe_tool","arguments":{"tool_name":"<identity returned by search>","catalog_revision":"<revision returned by search>"}}}`. Angle-bracket strings here denote test-bound runtime values, not a permitted production fallback. Extract the actual bridge result shape from existing harness/schema B3, not an assumed raw response root.
6. Generic execution regression only against the controlled recording fake: `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"call_tool","arguments":{"tool_name":"<fixture identity>","catalog_revision":"<fixture revision>","arguments":{}}}}`. Direct pin likewise uses `tools/call` with `params.name="alpha__read"` and fixture-valid args, not a new HTTP endpoint. Metadata reads must leave operation counter at0; execution regression increments only fake counter.

Use controlled servers alpha/beta, shared raw names read/write, alpha pinned read+write, beta same-name pin, and alpha unpinned spare. Source fixture title/description/nested `_meta`, annotation nil and destructive/openWorld nil/false/true cases; scalar normalization table. Icon JSON must use the actual SDK Icon shape (now confirmed: `src`, `mime_type`, `sizes`, `theme` per anchors B1). Mixed override scenario: alpha defaults readOnlyHint=true; alpha write exception readOnlyHint=false/destructiveHint=true/idempotentHint=false, source retained; beta/spare/generic unchanged. Include source/default/tool disagreements and member/entry removal in both directions. Do not classify real insert/replace operations by name.

### Vertical execution ledger — PROPOSED / BLOCKED only on task review + user approval

r7 closes the former B1-B6 evidence blockers (anchors admissible, see above); the ledger's per-ticket targets, test names, RED assertions and commands were re-checked against the anchors and stand as written below, with only the per-ticket amendments noted. Tickets02-06 remain blocked by **tasks-checkpoint critic review and separate user execution-task approval** — nothing else. Ticket01 completed research is not reopened. Commands below are **planned/NOT RUN**; expected RED is an assertion on behavior, never missing symbols or failed setup. The harness bindings named per ticket are confirmed in the anchors (B5).

#### Ticket01 — research complete, no implementation

Path `.scratch/tool-metadata/issues/01-resolve-compatibility-prerequisites.md`; no technical blockers/deps. Stable T1; R6/R9/R11; AC7/AC8/AC10/AC13. Deliverable already `.scratch/tool-metadata/prerequisites.md`, r5 design PASS/user-approved. No new command, code change, repeated research or gate action. The later source-mapping blockers (B1-B6) are now closed via r7 anchors; the former HTTP401 is resolved and no longer a constraint.

#### Ticket02 — independently classify bridge-owned tools

Path `.scratch/tool-metadata/issues/02-classify-bridge-owned-tools.md`; **no ticket dependency**. T4 built-ins/T5 wire/T6 docs; R1/R7-R10; AC1/AC9-AC11/AC13. Integration owner engineer.

- Production target `internal/service/tools.go:66-89` only for built-in descriptors; leave source/pin implementation and schemas/auth/dispatch handlers unchanged. Add direct mcp.ToolAnnotations values at existing registration sites; no helper abstraction required for three constant classifications.
- Test target new `cmd/mcp-bridge/serve_hybrid_metadata_test.go`: `TestServeHybridMetadataBuiltinAnnotations`; existing regression targets exposure129-177, reload177-225, coexistence59. Build on the confirmed harness helpers (r7 anchors B5: `writeBridgeConfig`, `bridgeServerSpec`, `freeAddress`, `fakeEmbeddings`, `startHybridBridgeWithUpstream`, `mcpSend` with Bearer `serveClientToken`/SERVE_CLIENT_TOKEN env, `probeBody`/`waitFor`), not a parallel server.
- RED: tools/list lacks expected annotations; assert read-only/idempotent on search/describe, conservative openWorld, generic false/true/false/true. Search/describe standard tools/call leave fake operation count0; schema/name/auth/initialize/pins unchanged. Fake-only dispatch regression remains intact.
- GREEN: annotate registration; same assertions pass. Planned focused command `go test ./cmd/mcp-bridge -run '^(TestServeHybridMetadataBuiltinAnnotations|TestServeHybridPreloadedToolsListExposure|TestServeHybridCoexistence|TestServeHybridSIGHUPReload)$' -count=1`; no live upstream.
- Complete operator docs in existing README tool surface area; changelog destination confirmed (r7 anchors B6): append under `CHANGELOG.md` `[Unreleased]` Added/Changed. Explain advisory hints, generic potential mutation/no prompt guarantee/no trust bypass. No missing harness mapping remains; stop only on any temptation to change transport/schema. Ticket02 may later execute independently of03, but global task approval is not yet granted.

#### Ticket03 — complete supported metadata preservation, including strict durable versioning

Path `.scratch/tool-metadata/issues/03-preserve-upstream-metadata.md`; depends **01** only, not02. T2+T4 source egress+T5+T6; R2/R3/R5-R11; AC2/AC3/AC5-AC11/AC13. Engineer owns complete discovery-to-client cutover.

- Targets: core definition core.go:27-31/new tool_metadata.go; upstream dedicated.go:103-108/new metadata conversion; catalog hash/revision/clone/read conversions confirmed at core.go:220-233/241, index.go:15-26, reload.go:659-671 — including the **r7 integration decision**: extend `DefinitionDigest` to fold in canonical metadata and rely on the existing `Published()` Binding `reflect.DeepEqual` (tools.go:119-122) with `Metadata` now inside `ToolDefinition`; storage snapshot/store anchors (snapshot.go:41-98,155-165,252-333; store.go:145-202) and selected versioned codec files above; service source restore/index/pin/describe/publication anchors (reload.go:639-671, index.go:15-26, tools.go:96-134/465-497) with the confirmed pin join `core.AddPins` reload.go:510-530. No layer is declared complete until direct list/describe/restart works.
- New tests `internal/core/tool_metadata_test.go`, `internal/upstream/tool_metadata_test.go`, `internal/storage/snapshot_metadata_test.go`, `internal/storage/store_metadata_test.go`, `internal/service/tool_metadata_test.go`, shared command wire file. Selected names: `TestToolMetadataCloneAndPresence`, `TestDiscoverToolMetadata`, `TestSnapshotV1ChecksumCompatibility`, `TestSnapshotV2MetadataIntegrity`, `TestStoreMetadataFailClosed`, `TestStoreMetadataAtomicUpgrade`, `TestMetadataSourceRefresh`, `TestServeHybridMetadataSourceRoundTrip`, `TestServeHybridMetadataRestart`.
- Frozen test assets selected: `internal/storage/testdata/tool-metadata/schema1.json` and `schema1.sha256`, produced **before production schema edits** from the exact existing valid fixture/legacy checksum, then immutable. Do not invent a checksum or regenerate legacy fixture through new schema2 types. B2 is closed (r7 anchors): the complete original DTO/tags/checksum are confirmed (snapshot.go:41-98,155-165,252-333) and the current fixture constructor is `validSnapshot()` store_test.go:27-63 (variant `twoServerSnapshot` reconcile_test.go:33) — build the frozen fixture from it. Add a test-only frozen old decoder to prove schema2 rejection, not to ship a deprecated production path.
- RED: source title/annotations/_meta/output schema missing at direct list/describe; pointer false lost or normalized source omitted; source-only change stale despite same routing; pre-schema2 code rejects schema2 fixture; old-style checksum must still pass baseline on schema1. Only assertions that truly fail count as RED; compatibility tests already passing are existing regressions, not fabricated failures.
- Implement selected new metadata API, all conversions/revisions/clone ownership, schema2 writer/strict1or2 reader/version-specific checksum, source-only effective/provenance, descriptor-aware pin publication/output schema. Same source copy visible on every backend and durable restore; no overrides persisted; describe provenance empty without patches.
- Negative matrix: schema1/2 invalid checksum, metadata tamper, unknown field, trailing bytes, future version/invalid identity; Load does not rewrite; rejected Save/admission preserves prior trusted bytes/generation/readiness semantics. Valid legacy Load preserves identities/metadata absence, next accepted Save produces schema2, frozen old reader rejects schema2, restore rehearsal retains newer file separately and restores legacy backup. Controlled persistence failure rejects, source outage keeps complete stale/reason while peer advances, cold pins deferred/immediate normal refresh.
- Planned focused commands: `go test ./internal/core ./internal/upstream ./internal/catalog ./internal/storage ./internal/service -run 'Metadata|SnapshotV1ChecksumCompatibility|SnapshotV2MetadataIntegrity' -count=1`; `go test ./cmd/mcp-bridge -run '^TestServeHybridMetadata(SourceRoundTrip|Restart)$' -count=1`. Run existing package regressions once per approved slice assignment; main final full suite stays ticket06.
- Docs/changelog in same vertical deliverable: supported inventory/SDK normalization/source view, schema2 strict reader matrix/pre-startup offline backup/restore generation loss and stateful sessions. Former B1/B2/B3/B5/B6 contract gaps are closed (r7 anchors); wire helpers for `TestServeHybridMetadataSourceRoundTrip`/`TestServeHybridMetadataRestart` are the confirmed B5 set. Never use permissive legacy reader, checksum recomputation via new DTO, state deletion, repeated upstream calls or transport workaround to get GREEN.

#### Ticket04 — complete server-default correction behavior

Path `.scratch/tool-metadata/issues/04-apply-server-metadata-defaults.md`; depends **03**. T3 defaults/T4 effective publication/T5/T6; R3-R11; AC3-AC11/AC13 server-default cases.

- Targets selected config metadata file/Server field, confirmed config paths (r7 anchors B4): YAML parser/`validateNode` config.go:87-132, duplicate diagnostics (server_id_duplicate :175-180, pin_duplicate :261-269), typed Diagnostics :65, `Fingerprint()` fingerprint.go:51-92 with `publicProjection`/`projectedServer` (tool_metadata joins public identity there); service pure derivation and immutable generation source/effective map confirmed B3 (reload.go:71-81); same list/describe/publication paths as03. Existing pin format/auth/credentials/storage source unchanged. This ticket accepts/implements defaults; exact exceptions are delivered05, not advertised as available before that ticket completes.
- New `internal/config/tool_metadata_test.go` and existing service/wire test targets: `TestToolMetadataDefaultsParse`, `TestToolMetadataDefaultsValidation`, `TestToolMetadataDefaultsPublicFingerprint`, `TestMetadataServerDefaults`, `TestServeHybridMetadataServerDefaults`.
- RED first with raw YAML strings/public wire assertions (no missing-production-type failures): new defaults config rejected/not recognized or effective fields unchanged; require explicit false/empty/nested object/array semantics; duplicate/invalid members fail atomically via existing typed finite diagnostics, no payload/raw error echo. Config-only default change must affect public identity, not credential identity.
- GREEN implements server-owned defaults parser/validation/identity and shared pure effective view. Wire alpha pinned read+write receive explicit defaults; alpha spare/beta same raw/generic/built-ins unchanged; no-pin server defaults allowed without affecting long-tail. Describe source retained and provenance server_default for applied fields; corrected values never become durable source.
- Real reload add/change/member removal/full removal updates same-session list despite same binding; removal inherits current source, refresh updates non-overridden values, rejected update keeps prior effective generation; restart derives from current config/source; depart/remove pin/server does not resurrect. Existing reporting policy only.
- Planned commands `go test ./internal/config ./internal/service -run 'ToolMetadataDefaults|MetadataServerDefaults' -count=1`; `go test ./cmd/mcp-bridge -run '^TestServeHybridMetadataServerDefaults$' -count=1`. Docs/changelog: concrete tool_metadata.defaults, inheritance/removal/broad assertions/mixed-service exception warning. Former B3/B4/B5/B6 gaps are closed (r7 anchors); no engineer-selected parser/fingerprint fallback, metadata secrets in config diagnostics or global read-only/trust change.

#### Ticket05 — complete exact-tool exceptions over defaults

Path `.scratch/tool-metadata/issues/05-support-exact-tool-exceptions.md`; depends **04**. T3 exceptions/T4+T5+T6; R3-R11; AC3-AC11/AC13 combined layering.

- Same approved config/service/wire components; activate exact tools mapping and ValidatePins according to selected signature, no wildcard selector. Valid tool-only patch without defaults; unknown/non-pin/cross-server targets fail before candidate admission. Pin-validation/public identity integration is confirmed (r7 anchors B4: `pinToolNamePattern` config.go:35, pin_duplicate :261-269, fingerprint projection fingerprint.go:51-92), no raw errors.
- Selected tests `TestToolMetadataExceptionsParse`, `TestToolMetadataExceptionsValidation`, `TestToolMetadataExceptionsPublicFingerprint`, `TestMetadataExactToolPrecedence`, `TestServeHybridMetadataExactToolExceptions`.
- RED: exact patch unsupported/ignored or write inherits wrong broad read-only default. Alpha defaults read-only=true with exact write=false/destructive=true/idempotent=false; read remains default, beta/spare/built-ins/generic unchanged. Source/default/tool disagree; winning provenance per path; omitted exception inherits, explicit false wins; nested merge/array replace semantics shared, not reimplemented differently.
- GREEN completes selected source→defaults→tool derivation and same generation view. Remove member/entire tool patch to current defaults; remove defaults while remaining tool patch retained; upstream source changes do not overwrite explicit patch; reload/refresh/failed candidate/pin-tool-server removal/restart preserve correct state, no correction resurrection. Bad targets/types/duplicate mappings cannot partly publish.
- Planned commands `go test ./internal/config ./internal/service -run 'ToolMetadataExceptions|MetadataExactToolPrecedence' -count=1`; `go test ./cmd/mcp-bridge -run '^TestServeHybridMetadataExactToolExceptions$' -count=1`. Docs/changelog concrete tools syntax/precedence/removal/operator mixed-service risk and client policy boundary. Former B3/B4/B5/B6 prerequisites are closed (r7 anchors); no auth/schema/transport/global-default workaround.

#### Ticket06 — integrated acceptance and release evidence, not a substitute feature

Path `.scratch/tool-metadata/issues/06-integrated-acceptance-and-release-evidence.md`; depends **02+05**. T7, consolidated T5/T6 evidence; all R/AC1-13. Coordinator owns go/no-go; assigned integration engineer performs once-only check sequence under explicit verification assignment.

- Reuse all unchanged per-ticket RED/GREEN, real `/mcp` controlled wire scenarios; retain requests/responses/counts, source/effective/provenance, defaults/exceptions/removal/outage/rejected persistence/reload/restart/legacy integrity and rollback rehearsal results. No new alternate harness or real mutable probes.
- Exact final commands from approved contract: `go test ./...`; `go test -race ./...`; `go vet ./...`; `go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...`; `go run ./cmd/covcheck`. Keep command+exit outputs and custom SOURCE-LINE>=90 report. No package/coverage metric substitution, test weakening or old fixture regeneration.
- Required general/Go/security reviews and correction evidence; no schema/auth/routing/result/initialize regression, telemetry/trust/transport change. Verify docs/changelog concrete config/recovery matrix and existing best-effort finite logging/readiness/metrics; no false delivery guarantee. Resolve every mandatory AC before shipping; no feature hidden behind this evidence-only ticket.
- Optional actual Gemini safe observation records product/version/policy/outcome; unavailable access exact NOT RUN reason, not “prompts fixed.” No client policy/global config change.
- Release evidence is not deployment/image/registry authority. Stop on any failed AC, unsafe format/restore behavior, source-line<90, missing review or unapproved checkpoint; coordinator returns to planner/critic for contract changes rather than narrowing tests. Former helper/anchor gaps are closed (r7 anchors); an engineer who encounters a genuine contradiction returns it to the planner rather than adapting locally.

### Former task-contract blockers B1-B6 — CLOSED (r7, dated history)

All six r6 blockers were closed on 2026-10-26 by authorized read-only research at HEAD `eb879ef`; the complete file:line evidence set is `.scratch/tool-metadata/integration-anchors-r7.md` and its load-bearing facts are summarized in "r7 exact anchors: closed, admissible" above. Record once here:

- **B1** (core/catalog digest/revision/clone, discovery, SDK Icon/annotation shapes) — closed; confirmed `DefinitionDigest`/`revisionEntry`/`canonicalSchema`, definition construction sites, `ToolAnnotations`/`Icon` shapes, bare-bool hints.
- **B2** (legacy storage DTO freeze/fixture) — closed; complete tags/checksum/Validate plus `validSnapshot()` fixture constructor confirmed.
- **B3** (assembly/pin/publication/describe/search shapes) — closed; including the load-bearing fact that `Published()` equality is binding-only (tools.go:119-122), resolved by the r7 integration decision above.
- **B4** (config parser/diagnostics/fingerprint) — closed; `Parse`/`validateNode`/duplicate diagnostics/typed Diagnostics/`Fingerprint()` projection confirmed.
- **B5** (wire harness helpers/auth/session bindings) — closed; all helper names/signatures/line ranges confirmed, including Bearer auth and operation-count via existing fake.
- **B6** (changelog/docs destination) — closed; `CHANGELOG.md` `[Unreleased]` (Keep-a-Changelog, commit a49c0dc) plus README tool-surface area confirmed.

The former HTTP401 access incident is resolved; no connector retry, credential search or unauthorized fallback occurred. The only remaining gates on tickets02-06 are tasks-checkpoint critic review and separate user execution-task approval.

### Evidence, uncertainty and operations/recovery retention

Existing `.scratch/tool-metadata/prerequisites.md` remains source for U2 strict schema1/checksum/Save atomicity, U3 stateful SSE/timeout0/admission/config/session/startup/refresh/reload, U5 stdout finite best-effort/redaction/readiness/metrics and executed baseline smoke. That evidence remains valid; the later denial is resolved and exact anchors are admissible as recorded above.

U1 current SDK inventory/normalization resolved (anchors B1 confirm bare-bool hints); U2 incompatibility resolved by approved schema2/strict dual-reader/restore contract, **new-format tests NOT RUN**; U3 lifecycle/defaults resolved with subscription delivery caveat; U4 exact custom source-line gate resolved, full feature checks NOT RUN; U5 reporting boundary resolved, external retention unknown; U6 actual Gemini unavailable/unknown. B1-B6 are closed execution-contract evidence; their closure is not a regression of r4 requirements/r5 design approval and grants no implementation permission.

Observability retained: T4/T6 within each ticket, AC8/9/13; current stdout synchronous finite schema/vocabulary, recursive redaction, warn/reload/failure/invalid_config projection/generic fallback; no raw error or metadata payload. Encoder failure resets for future record only, no replay/retention guarantee. Refresh visibility current status/reason/readiness/catalog metrics, rejected-pass metrics accepted assembly; persist readiness false/error, reload operator path. No new Prometheus/Loki paths/names/labels/buckets/retention/alerts/dashboards or generic extra JSON schema/IDs. Required-field/delivery exceptions remain justified by approved no-new-telemetry boundary and existing finite schema, not an assertion that every desired field is present. Existing negative/JSON/redaction tests prove current behavior where mapped; no stronger shipping/log-delivery claim.

Release/recovery retained: before new process starts, stop old process and retain exact validated schema1 snapshot/config separately with sensitive-state permissions. New strict reader1or2 validates old checksum before conversion; writer2 can first persist at startup. Existing atomic-save/trusted-state checks remain. Old reader never receives schema2; rollback stops new process, preserves newer state, restores backup/config and old binary. Returns to backup generation with intervening local progress absent, newer file retained, no upstream effects undone. No valid legacy backup means no state-preserving old rollback; do not manufacture empty state/strip metadata. No release/deployment authority in task planning or ticket06.

### Critic handoff and planner self-check

Review this as the **r7 tasks checkpoint**: published vertical graph intact; T1-T7 obligation meanings retained;01 complete/02 independent/03→04→05/06 joins02+05; per-slice RED/GREEN and docs; standard MCP envelopes; source/effective isolation and versioned integrity preserved. Verify that the selected new APIs fit approved design against the admissible anchors, including the r7 integration decision (DefinitionDigest metadata + existing Binding DeepEqual) — no design reopen, no invented current code. Gate stays CLOSED until this checkpoint passes tasks-critic review and the user grants explicit execution-task approval.

r7 used only the authorized anchors file and prior admitted artifacts; no code/test/config/gate files changed; no tests/build/lint/vet/format commands run. Self-evaluation: accuracy4/5 (all contracts now anchored), completeness4/5 (all six vertical slices executable once approved), clarity4/5 (closed blockers dated once; obligations versus tickets separated), actionability4/5 (ready for tasks-critic/user approval, not engineer), conciseness4/5 (traceability intentionally extensive). Overall4/5; the only remaining blockers are tasks-checkpoint critic review and user execution-task approval.

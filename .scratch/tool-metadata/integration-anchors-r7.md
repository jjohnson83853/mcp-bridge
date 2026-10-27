# Integration anchors — B1–B6 closure (2026-10-26)

Authorized read-only research at HEAD `eb879ef` (scout ExtremeBass; bounded direct reads; Graphify skipped — tree dirty with untracked planning dirs only). This closes spec r6 blockers B1–B6 and feeds the r7 tasks revision. Historical r5 hashes in spec.md are unaffected.

## B1 — descriptor/definition/discovery contract

- `core.ToolDefinition` internal/core/core.go:27-31: `RawName, Description string; InputSchema, OutputSchema json.RawMessage` (no tags). Stored as `Binding.Def` (core.go:23). No Clone ctor; copied by value, schemas are RawMessage aliases.
- `DefinitionDigest(serverID, rawName string, def ToolDefinition) string` core.go:220; feeds `revisionEntry{Server,Raw,Description,Input,Output}` (core.go:220-233; json tags server/raw/description/input_schema/output_schema) via `canonicalSchema` (core.go:241 compact re-encode, UseNumber, invalid/empty→nil); same entry feeds `Bridge.Revision()` core.go:187.
- ToolDefinition construction sites: upstream/dedicated.go:103-108 (discovery); service/reload.go:659-663 (`definitionsFromSnapshot`); service/index.go:15-26 → storage.Tool with `DefinitionHash: core.DefinitionDigest(...)` (index.go:23); reload.go:671 copies `binding.Def` verbatim.
- SDK v1.8.0 `mcp.Tool` protocol.go:1895: embedded `Meta` (`json:"_meta,omitempty"`, value not pointer), `Annotations *ToolAnnotations` :1902, `Name` :1921, `Title` :1946, `Icons []Icon` :1948, `InputSchema any` :1921, `OutputSchema any` :1939. No scalar read-only/idempotent on Tool.
- `ToolAnnotations` :1960-1990: `Title string`; `DestructiveHint *bool` (omitempty); `IdempotentHint bool` (bare, no omitempty); `OpenWorldHint *bool` (omitempty); `ReadOnlyHint bool` (bare). Bare-bool omission only via debug `hintomitempty` — scalar absent-vs-false is unrepresentable on the wire (matches R3 normalization disclosure).
- `Icon` :1484-1496: `Source string json:"src"`, `MIMEType string`, `Sizes []string`, `Theme IconTheme` (last three omitempty). Base `Annotations` :197-214 is a DIFFERENT type from ToolAnnotations.
- `internal/upstream/dedicated.go:84` `func (d *Dedicated) Discover(ctx context.Context) ([]core.ToolDefinition, error)` — paginated ListTools, partial/malformed page = error (:91-103); conversion :103-108 from local package-level `canonicalSchema`; sort by RawName :126. Callers: wiring.go:197 → `ReloadDeps.Discover` (serve.go:186) → reload.go:472. No other Discover path.

## B2 — storage legacy DTO freeze inputs

- Envelope :41-48 tags: schema_version, generation_sequence, created_at, config_public_fingerprint, catalog_revision, snapshot_checksum. Binding :51-55: server_id, endpoint_fingerprint, retired. Server :58-66: server_id, endpoint_fingerprint, negotiated_version, last_success_at, last_attempt_at, catalog_status, reason(omitempty). Tool :69-78: server_id, rawname, publicname, description(omitempty), definition_hash, embedding_content_hash, input_schema(omitempty), output_schema(omitempty). Embedding :81-89: tool_identity, content_hash, model_identity_digest, dimensions, format_version, vector. Snapshot :92-98 embeds Envelope + bindings/servers/tools/embeddings.
- `Checksum()` :155-165: shallow copy, `canonical.Envelope.SnapshotChecksum=""`, `json.Marshal(&canonical)` (struct field order), SHA-256 hex. `Validate()` :252-333: schema version, revisionPattern, non-empty fingerprint, non-zero UTC CreatedAt, unique non-retired bindings, servers match binding fingerprint, status ∈ {fresh,stale,none}, reason-if-stale, tools: known server, publicNamePattern, non-empty rawname, hexDigest(DefinitionHash+EmbeddingContentHash), valid schemas, unique public names.
- Fixture: store_test.go:27-63 `validSnapshot()` (consts configValue/revisionValue/fingerprintValue above :27); variant twoServerSnapshot (reconcile_test.go:33).

## B3 — service assembly/pin/describe join

- Assembly reload.go:71-81: generation uint64; fingerprint string; revision string; auth [][32]byte; catalog *core.Bridge; servers map[string]ServerRuntime; pins []core.ResolvedPin; snapshot *storage.Snapshot; prompts []storage.Prompt.
- Pin join: `core.AddPins(next.catalog, pins)` after `next.catalog.Register` (reload.go:510-530; pins from `in.candidate.Pins` keys "server"/"tool" :510-513); startup defers (next.pins=nil :520).
- `definitionsFromSnapshot` reload.go:639 (binding fingerprint must match endpoint; skips retired); `indexedToolRows` = service/index.go:15-26.
- tools.go `Published` :96-134; idempotent equality :119-122 `reflect.DeepEqual(previous.Binding, next[name].Binding)` — metadata-only changes MUST extend this equality or revisions won't republish.
- `callPinnedTool` :140; dispatch :211 `assembly.catalog.Dispatch(ctx, core.PublicName(...), assembly.CatalogRevision(), args, lease.Server().Client)`.
- Describe wire request :465-468 `{tool_name, catalog_revision}` strict decode; revision gate :476-478; payload :484-497 keys: server, catalog_revision, catalog_status, availability, tool_name, upstream_name, description, pinned, last_success_at, validation_status (+input_schema/output_schema/validation_reason conditional).
- search_tools per-result map :837-848 keys: tool_name, server, upstream_name, description, score, catalog_revision, catalog_status, availability, last_success_at, pinned; top-level :863-867: catalog_revision, mode, degraded_reasons, results (+backends :852). Call result payload :743-751.

## B4 — config integration points

- config.Server config.go:37-43 yaml: id, endpoint, prompts_disabled, credentials(omitempty), limits(omitempty). `pinToolNamePattern` config.go:35.
- `Parse(data, configPath)` config.go:87: size→yaml node→single-doc→root mapping→`docValidator.validateNode` :126 (recursive duplicate-key detection :651-661 yaml_key_duplicate; server_id_duplicate :175-180; pin_duplicate :261-269; header_duplicate :933; typed Diagnostics diagnostics.go:65 RelatedKey…) → strict KnownFields decode :132 → SchemaVersion check.
- `(c Config) Fingerprint()` fingerprint.go:51-92: SHA-256 hex over `publicProjection{FormatVersion, SchemaVersion, HTTP/Operations/Storage/Catalog/Search/Embedding/ClientAuth/UpstreamDefaults/Pins + projectedServer{ID,Endpoint,Limits,PromptsDisabled,Credentials}}`; servers/pins sorted; secrets excluded. Consumed reload.go:391 (`in.candidate.Fingerprint()`) and Envelope.ConfigPublicFingerprint via storage.Reconcile reload.go:541-549.
- No Config/Server Clone exists; Config immutable post-Parse (embeddingSnapshot pointer config.go:57-63).

## B5 — wire harness bindings

- Consts serve_test.go:27-29: serveClientToken="serve-test-client-token", serveEmbeddingKey, serveDimensions=8.
- Temp config: `writeBridgeConfig` serve_test.go:124, `writeBridgeConfigInterval` :132, `bridgeServerSpec` :115, `freeAddress` :51, `fakeEmbeddings` :85.
- Startup: `startHybridBridgeWithUpstream` serve_hybrid_exposure_test.go:48 (in-process runContext; pins alpha/read injected by string replace :66-68; readiness `waitFor`+`probeBody` serve_test.go:308/:323 on /readyz).
- Wire: `mcpSend` exposure_test.go:104 — POST /mcp, `Authorization: Bearer serveClientToken`, Mcp-Session-Id + Mcp-Protocol-Version 2025-11-25; auth token from SERVE_CLIENT_TOKEN env set in helper.
- Reload helpers: serve_hybrid_reload_test.go — `fakeUpstreamReadWrite` :26, `rewriteHybridReloadPins` :54, `toolChangeWatcher` :87/:97/:106, `connectBridgeToolChanged` :116, `listToolNames` :135, `listToolsContains` :150, `directCallTool` :156, `directCallToolText` :162. Coexistence: `fakeUpstreamReadAndWriteTools` serve_hybrid_coexistence_test.go:24, TestServeHybridCoexistence :59.

## B6 — changelog destination

- `CHANGELOG.md` `[Unreleased]` (Keep-a-Changelog) now exists (created with OAUTH-DIAG 02, commit a49c0dc). Feature entries append under Added/Changed there.

## Notes for r7

- No ToolMetadata code exists anywhere yet (all net-new, matching spec's proposed API contract).
- Published() binding-only equality (B3) is a confirmed r7 integration obligation: metadata changes must participate in revision/equality (T2/R3) — spec decision 12 already requires re-register on source/default/tool-only changes.
- SDK bare-bool hints (IdempotentHint/ReadOnlyHint non-pointer) confirm the spec's scalar-normalization stance; pointer hints (Destructive/OpenWorld) preserve nil/false/true.

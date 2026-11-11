import fs from "node:fs";

const root = "/home/opencode/projects/default/mcp-bridge";
const state = JSON.parse(fs.readFileSync(`${root}/.omp/plans/execution-state.json`, "utf8"));
const pathsFor = id => state.tasks.find(t => t.id === id).effects.paths;

const common = {
  statePath: ".omp/plans/execution-state.json",
  schemaVersion: 1,
  projectRoot: root,
  feature: "structured-logging",
  revision: "r7-closure1",
};

const t2 = {
  intent: "Dispatch Task T2 to engineer",
  instructions: "Implement configuration-node attribution for every rejection class per .scratch/structured-logging/spec.md (revision r7-closure1), ticket 02, AD10/AD15, and AC02-1..AC02-7.",
  workflowDispatch: {
    ...common,
    requests: [{ name: "ImplementT2", taskId: "T2", agent: "engineer", effects: { kind: "write", paths: pathsFor("T2") } }],
  },
  tasks: [{
    name: "ImplementT2",
    agent: "engineer",
    task: `# Target
Task T2 — Configuration-node attribution for every rejection class. Feature \`structured-logging\`, spec revision r7-closure1.

Read first, bounded: \`.scratch/structured-logging/spec.md\` §4 AD10 (r7 per-stage table), AD15; §5 AC02-1..AC02-7; §6 T2; \`.scratch/structured-logging/decisions.md\`; \`.scratch/structured-logging/issues/02-*.md\`.

# Change
1. \`internal/config/config.go\`: attach \`config.property_path\`/\`line\`/\`column\` at the offending node for every rejection class per the AD10 r7 per-stage table (structure/bounds/embedding-reference/overlap/secret-stage classes populated; accessor classes and \`weight_invalid\` shape \`missing\` are justified omissions).
2. \`internal/config/diagnostics.go\`: add exported \`DiagnosticCodes() []string\` (AD15) returning the closed sorted set of every \`Diagnostic.Code\` the package produces.
3. \`internal/config/bounds.go\`, \`internal/config/tool_metadata.go\`: positions for bounds/embedding/secret stages.
4. \`cmd/mcp-bridge/diagnostics_envelope.go\`: total remediation table sourced from \`DiagnosticCodes()\` + named default; \`config.shape\`.
5. \`cmd/mcp-bridge/diagnostics_envelope_test.go\` (new): total-remediation test iterating \`DiagnosticCodes()\`.
6. \`internal/config\` test set: \`pins_bounds_test.go\`, \`bounds_edge_test.go\`, \`aggregate_edge_test.go\`, \`parser_edges_test.go\`, \`api_edges_test.go\`, \`validate_edge_test.go\`, \`secret_validation_edge_test.go\`, \`config_test.go\`, \`diagnostics_test.go\`, \`command_test.go\`, \`embedding_env_test.go\`, \`secret_sequence_edge_test.go\` — per-class position assertions per AD10.

# Acceptance (AC02-1..AC02-7)
- AC02-1..AC02-5: every rejection class carries \`config.property_path\`/\`line\`/\`column\` at the offending node WHEN a source node exists (AD10 r7 per-stage table); \`weights_invalid\` \`config.property_path\` equals the offending node's key per shape; duplicates carry \`config.related\`; server/pin-scoped rejections carry validated ids; optional \`config.shape\` present where a shape exists; no secret/URL/query/YAML/decoder prose.
- AC02-6: \`message\` equals the record's static \`Reason\` per class AND a non-empty static \`error.remediation\` for every class; the test iterates \`config.DiagnosticCodes()\` (not a hard-coded list); every emitted \`Diagnostic.Code\` ∈ \`DiagnosticCodes()\`.
- AC02-7: every conflict site emits own + related positions per the AD10 mapping; per-stage populated positions asserted; omission classes (accessor, restart_required, missing-key) asserted.

# RED/GREEN
RED first: per-class position assertions per the AD10 stage table incl. registry-completeness guard — expected failure: missing positions in bounds/embedding/secret stages, unregistered codes. Then implement to GREEN.

# Verify
\`go test ./internal/config/...\`, \`go test ./cmd/mcp-bridge -run DiagnosticsEnvelope\`, \`go vet ./...\`.

# Receipt
Write \`.omp/artifacts/receipt_T2.json\` per the receipt schema (task_id T2, status COMPLETED, evidence entries with command and exitCode).

# Constraints
- Only the files in the dispatch effects. No other production files.
- At most 42 read/search operations before the first edit; if exceeded, stop and report stalled.
- No external SDK/vendor research.`,
    solutionSpace: "Implementation is bounded by the spec: AD10 per-stage position attribution, the AD15 DiagnosticCodes() registry, and the AC02-1..7 tests. No architectural choices remain open.",
  }],
};

const t4 = {
  intent: "Dispatch Task T4 to engineer",
  instructions: "Migrate all T4-owned producers to the unified log envelope per .scratch/structured-logging/spec.md (revision r7-closure1), ticket 04, AD1/AD2/AD8/AD11/AD12/AD13, and AC04-1..AC04-8 (T4-owned evidence only).",
  workflowDispatch: {
    ...common,
    requests: [{ name: "ImplementT4", taskId: "T4", agent: "engineer", effects: { kind: "write", paths: pathsFor("T4") } }],
  },
  tasks: [{
    name: "ImplementT4",
    agent: "engineer",
    task: `# Target
Task T4 — Unified log envelope migration. Feature \`structured-logging\`, spec revision r7-closure1.

Read first, bounded: \`.scratch/structured-logging/spec.md\` §4 AD1/AD2/AD8 (r7 exact union + equality oracle)/AD11/AD12/AD13; §5 AC04-1..AC04-8; §6 T4; \`.scratch/structured-logging/decisions.md\`; \`.scratch/structured-logging/issues/04-*.md\`.

# Change
1. \`internal/observe/log.go\`, \`internal/observe/envelope.go\` (finalizes \`EnvelopeFieldNames()\`), \`internal/observe/httpdiagnostic_encode.go\`, \`internal/observe/httpdiagnostics.go\`, \`internal/observe/httpdiagnostic_types.go\`: migrate to the closed AD8 union; HTTP lane carries exactly the 60-slot AD2 map (\`diag_environment\`/\`source_revision\` removed per AD12).
2. \`internal/service/tools.go\`: sites migrate; :207-216 Entry emission DELETED (AD13).
3. \`internal/service/direct_execution.go\`: migrate.
4. \`internal/upstream/dedicated.go\`: BLK-2 contract; \`protocol_version\` validated against \`append(service.AdvertisedProtocolVersions(), "2026-07-28")\`; hostile/out-of-set value OMITS the key (no unvalidated string serialized).
5. \`cmd/mcp-bridge/serve.go\`, \`cmd/mcp-bridge/wiring.go\`: registry signature + logger threading; T4 closed-vocabulary mapping of \`operatorLog\` rows — typed \`Error\` plus diagnostic identity re-keyed to \`config.*\` dotted keys (NOT shape-preserving; superseded in shape by T5).
6. Tests: the exact test files in the dispatch effects — slot-map/vocabulary tests, \`EnvelopeFieldNames()\` equality vs an INDEPENDENT hard-coded expected sorted list, hostile \`protocol_version\`, nil-logger constructor errors, two-arg \`NewLogger\` cutover, and the typed/nested-shape migrations named in the spec (serve_edge_test.go :195-240, serve_test.go :724-725, manifest_test.go 18-field assertions).

# Acceptance (AC04-1..AC04-8, T4-owned evidence only)
- AC04-1: every T4-owned record has \`schema_version/level/service/event/operation/outcome\` + non-empty \`message\`; assert pinned operations for T4-owned producers (observe shared lane, HTTP lane, service tools/direct-execution incl. handshake). Do NOT assert startup/cli/reload/publish/check_config/load_config pairs (owned by T3/T5/T6/T1).
- AC04-2..AC04-7: no removed/collector-owned fields, nested error per AD8, K3 lane exception, stream-per-level, 60-slot lane shape, one-JSON-per-line — asserted against T4-owned producers with SCOPED suites.
- AC04-8: every emitted record's key set asserted by EQUALITY of \`observe.EnvelopeFieldNames()\` against an INDEPENDENT hard-coded expected sorted list (all nested subkeys; the function is not its own oracle); representative records per T4-owned producer (observe, service tools/direct execution, upstream handshake through the injected logger with hostile \`ProtocolVersion\` omitted, nil-logger constructor error on every migrated constructor path, HTTP lane).

# RED/GREEN
RED first: slot-map/vocabulary tests (incl. \`EnvelopeFieldNames()\` equality vs the independent expected list, hostile \`protocol_version\`, nil-logger constructor errors, two-arg \`NewLogger\` cutover) — expected failure: 65-slot encoder, flat reason, single-arg constructor, absent exported vocabulary. Then implement to GREEN.

# Verify
\`go build ./... && go vet ./...\`, \`go test ./internal/observe/... ./internal/service/... ./internal/upstream/... ./internal/auth/...\`. The FULL-SUITE gate is explicitly NOT a T4 criterion (owned by T7/AC07-7).

# Receipt
Write \`.omp/artifacts/receipt_T4.json\` per the receipt schema (task_id T4, status COMPLETED, evidence entries with command and exitCode).

# Constraints
- Only the files in the dispatch effects. No other production files.
- No T5 behavior/renderer-loop change; no legacy field reintroduction; no new vocabulary keys beyond AD8/AD11.
- At most 42 read/search operations before the first edit; if exceeded, stop and report stalled.
- No external SDK/vendor research.`,
    solutionSpace: "Implementation is bounded by the spec: closed AD8 vocabulary migration, 60-slot HTTP lane, protocol_version validation, and the AC04-1..8 T4-owned evidence. No architectural choices remain open.",
  }],
};

fs.writeFileSync(`${root}/.scratch/structured-logging/dispatch-seed-t2.json`, JSON.stringify(t2, null, 2) + "\n", "utf8");
fs.writeFileSync(`${root}/.scratch/structured-logging/dispatch-seed-t4.json`, JSON.stringify(t4, null, 2) + "\n", "utf8");
console.log("seeds written: t2, t4");

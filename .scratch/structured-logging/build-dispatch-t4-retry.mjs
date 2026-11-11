import fs from "node:fs";

const root = "/home/opencode/projects/default/mcp-bridge";
const state = JSON.parse(fs.readFileSync(`${root}/.omp/plans/execution-state.json`, "utf8"));
const paths = state.tasks.find(t => t.id === "T4").effects.paths;

const seed = {
  intent: "Dispatch Task T4 (retry 1) to engineer",
  instructions: "Migrate all T4-owned producers to the unified log envelope per .scratch/structured-logging/spec.md (revision r7-closure1), ticket 04, AD1/AD2/AD8/AD11/AD12/AD13, and AC04-1..AC04-8 (T4-owned evidence only). This is a surgical retry after a read-budget stall; reconciliation findings are provided so you need not re-derive them.",
  workflowDispatch: {
    statePath: ".omp/plans/execution-state.json",
    schemaVersion: 1,
    projectRoot: root,
    feature: "structured-logging",
    revision: "r7-closure1",
    requests: [{ name: "ImplementT4Retry", taskId: "T4", agent: "engineer", effects: { kind: "write", paths } }],
  },
  tasks: [{
    name: "ImplementT4Retry",
    agent: "engineer",
    task: `# Target
Task T4 — Unified log envelope migration (surgical retry). Feature \`structured-logging\`, spec revision r7-closure1.

Read first, bounded: \`.scratch/structured-logging/spec.md\` §4 AD1/AD2/AD8 (r7 exact union + equality oracle)/AD11/AD12/AD13; §5 AC04-1..AC04-8; §6 T4; \`.scratch/structured-logging/decisions.md\`.

# Prior-attempt findings (USE DIRECTLY — do NOT re-derive)
The previous attempt stalled before editing after 49 read/search operations. It established:
1. \`internal/observe/log.go\` still uses the SINGLE-argument constructor and the legacy record path.
2. The HTTP encoder has 65 slots and STILL retains the AD12 fields \`diag_environment\`/\`source_revision\` (must become exactly the 60-slot AD2 map).
3. \`internal/upstream/dedicated.go\` writes a raw map and serializes \`res.ProtocolVersion\` WITHOUT validation.
These confirm the migration spans the assigned files. Do not re-explore what these three facts already establish.

# Hard read budget
At most 42 read/search operations before your FIRST edit. Front-load your first edit: the FIRST thing you write is the RED test file(s) (e.g. an \`EnvelopeFieldNames()\` equality test against an INDEPENDENT hard-coded expected sorted list, the 60-slot lane-shape test, the hostile \`protocol_version\` test, and the nil-logger constructor test). That edit resets the budget, then continue.

# Change
1. \`internal/observe/log.go\`, \`internal/observe/envelope.go\` (finalizes \`EnvelopeFieldNames()\`), \`internal/observe/httpdiagnostic_encode.go\`, \`internal/observe/httpdiagnostics.go\`, \`internal/observe/httpdiagnostic_types.go\`: migrate to the closed AD8 union; HTTP lane = exactly the 60-slot AD2 map (\`diag_environment\`/\`source_revision\` removed per AD12).
2. \`internal/service/tools.go\`: sites migrate; :207-216 Entry emission DELETED (AD13).
3. \`internal/service/direct_execution.go\`: migrate.
4. \`internal/upstream/dedicated.go\`: BLK-2 contract; validate \`protocol_version\` against \`append(service.AdvertisedProtocolVersions(), "2026-07-28")\`; hostile/out-of-set value OMITS the key (no unvalidated string serialized).
5. \`cmd/mcp-bridge/serve.go\`, \`cmd/mcp-bridge/wiring.go\`: registry signature + logger threading; T4 closed-vocabulary mapping of \`operatorLog\` rows — typed \`Error\` plus diagnostic identity re-keyed to \`config.*\` dotted keys (NOT shape-preserving; superseded in shape by T5).
6. Tests: the exact test files in the dispatch effects — slot-map/vocabulary tests, \`EnvelopeFieldNames()\` equality vs an INDEPENDENT hard-coded expected list, hostile \`protocol_version\`, nil-logger constructor errors, two-arg \`NewLogger\` cutover, and the typed/nested-shape migrations named in the spec (serve_edge_test.go :195-240, serve_test.go :724-725, manifest_test.go 18-field assertions).

# Acceptance (AC04-1..AC04-8, T4-owned evidence only)
- AC04-1: every T4-owned record has \`schema_version/level/service/event/operation/outcome\` + non-empty \`message\`; assert pinned operations for T4-owned producers (observe shared lane, HTTP lane, service tools/direct-execution incl. handshake). Do NOT assert startup/cli/reload/publish/check_config/load_config pairs (owned by T3/T5/T6/T1).
- AC04-2..AC04-7: no removed/collector-owned fields, nested error per AD8, K3 lane exception, stream-per-level, 60-slot lane shape, one-JSON-per-line — asserted against T4-owned producers with SCOPED suites.
- AC04-8: every emitted record's key set asserted by EQUALITY of \`observe.EnvelopeFieldNames()\` against an INDEPENDENT hard-coded expected sorted list (all nested subkeys; the function is not its own oracle); representative records per T4-owned producer (observe, service tools/direct execution, upstream handshake through the injected logger with hostile \`ProtocolVersion\` omitted, nil-logger constructor error on every migrated constructor path, HTTP lane).

# RED/GREEN
RED first (this is your first edit): add the failing tests above — expected failure: 65-slot encoder, flat reason, single-arg constructor, absent exported vocabulary. Then implement to GREEN.

# Verify
\`go build ./... && go vet ./...\`, \`go test ./internal/observe/... ./internal/service/... ./internal/upstream/... ./internal/auth/...\`. The FULL-SUITE gate is explicitly NOT a T4 criterion (owned by T7/AC07-7).

# Receipt
Write \`.omp/artifacts/receipt_T4.json\` per the receipt schema (task_id T4, status COMPLETED, evidence entries with command and exitCode).

# Constraints
- Only the files in the dispatch effects. No other production files.
- No T5 behavior/renderer-loop change; no legacy field reintroduction; no new vocabulary keys beyond AD8/AD11.
- At most 42 read/search operations before the first edit; if exceeded, stop and report stalled.
- No external SDK/vendor research.`,
    solutionSpace: "Implementation is bounded by the spec: closed AD8 vocabulary migration, 60-slot HTTP lane, protocol_version validation, and the AC04-1..8 T4-owned evidence. Reconciliation findings are provided; no architectural choices remain open.",
  }],
};

fs.writeFileSync(`${root}/.scratch/structured-logging/dispatch-seed-t4-retry.json`, JSON.stringify(seed, null, 2) + "\n", "utf8");
console.log("retry seed written");

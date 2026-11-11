import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { canonicalJson, sha } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs";
import { validateShape } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-schema.mjs";

const root = "/home/opencode/projects/default/mcp-bridge";
const feature = "structured-logging";
const revision = "r7-closure1";
const recordedAt = new Date().toISOString();

const v2Sha = t => crypto.createHash("sha256").update(t, "utf8").digest("hex");

const specPath = path.join(root, ".scratch", feature, "spec.md");
const specText = fs.readFileSync(specPath, "utf8");

const bindings = {};
for (const name of ["requirements", "design", "tasks", "envelope"]) {
  const startToken = `<!-- specdd:${name}:start -->`;
  const endToken = `<!-- specdd:${name}:end -->`;
  const start = specText.indexOf(startToken);
  const end = specText.indexOf(endToken);
  if (start === -1 || end === -1) throw new Error(`Missing specdd token for ${name}`);
  if (specText.indexOf(startToken, start + 1) !== -1) throw new Error(`Duplicate start token for ${name}`);
  bindings[name] = { revision, sha256: v2Sha(specText.slice(start + startToken.length, end)) };
}

const branchId = "approved-structured-logging";
const taskIds = ["T1", "T2", "T3", "T4", "T5", "T6", "T7"];
const branch = { id: branchId, answerIds: [], taskIds, envelopeId: "env-1", sectionBindings: bindings };
const packetDigest = sha(canonicalJson({ feature, projectRoot: root, sectionBindings: bindings, branches: [branch] }));

const artifact = rel => ({ path: path.join(root, rel), sha256: v2Sha(fs.readFileSync(path.join(root, rel), "utf8")) });

const sources = [
  {
    id: "src-user",
    kind: "user-message",
    artifact: artifact(".scratch/structured-logging/user-auth.txt"),
    locator: "owner kickoff message and grilling answers, 2026-11-10",
    recordedAt,
  },
  {
    id: "src-decisions",
    kind: "policy",
    artifact: artifact(".scratch/structured-logging/decisions.md"),
    locator: "owner-settled logging contract decisions D1-D20",
    recordedAt,
  },
];

const requirements = [
  ["R1", "Every application log record and CLI error is one newline-delimited JSON envelope, including service, upstream handshake, and HTTP lane records"],
  ["R2", "Application-owned envelope fields are schema_version, level, service, event, message, operation, outcome, nested error, config/resource objects, typed entry fields, and the closed AD8 vocabulary"],
  ["R3", "The application never emits timestamps, subsystem, container identity, host, environment, or version"],
  ["R4", "debug/info to stdout, warn/error to stderr; one JSON object per line, one write, no retry, no plaintext fallback"],
  ["R5", "Nested error carries code, kind, retryable, remediation; message is human-only; the HTTP lane uses the same nested shape"],
  ["R6", "Configuration diagnostics carry path, property path, one-based line and column, safe identifiers, and related location, with documented position exceptions"],
  ["R7", "Pin ceiling reports pins[64] at its document-order position with validated identity only, plus counts and first excess index"],
  ["R8", "No record serializes secret values, URLs, query strings, headers, payloads, raw YAML, or arbitrary Go error text"],
  ["R9", "Fail-fast: first error in deterministic document order, exactly one detailed event, no summary event"],
  ["R10", "Startup rejection is error with exit 1; reload rejection is warn and retains the active configuration"],
  ["R11", "Non-configuration failures use the same envelope with stable codes, kinds, retryability, remediation, and safe resources"],
  ["R12", "--help stays human-readable and emits no JSON record"],
  ["R13", "Validation success emits config_validation_succeeded with no error object and no configuration contents"],
  ["R14", "Exit code for configuration failure stays 1"],
  ["R15", "No panic recovery; Go crash report untouched (non-goal)"],
  ["R16", "No raw-cause diagnostic mode (non-goal)"],
  ["R17", "No legacy aliases: time, environment, version, flat reason, flat error.class are removed with consumers and docs migrated"],
  ["R18", "schema_version starts at 1; breaking field changes increment it; optional additions do not"],
  ["R19", "Docker Compose log prefix is orchestrator output; the application does not work around it (non-goal)"],
].map(([id, summary]) => ({ id, summary }));

const acMap = {
  "AC01-1": ["R1", "R9", "R10", "R14"], "AC01-2": ["R7"], "AC01-3": ["R7"], "AC01-4": ["R3"],
  "AC01-5": ["R7"], "AC01-6": ["R1", "R4"], "AC01-7": ["R7", "R8"], "AC01-8": ["R9"],
  "AC02-1": ["R6"], "AC02-2": ["R6"], "AC02-3": ["R6"], "AC02-4": ["R8"], "AC02-5": ["R6"],
  "AC02-6": ["R6", "R8"], "AC02-7": ["R6"],
  "AC03-1": ["R11"], "AC03-2": ["R11"], "AC03-3": ["R8", "R11"], "AC03-4": ["R11"],
  "AC03-5": ["R12"], "AC03-6": ["R8", "R11"], "AC03-7": ["R10", "R11"],
  "AC04-1": ["R1", "R2", "R9"], "AC04-2": ["R3", "R17"], "AC04-3": ["R5"], "AC04-4": ["R4"],
  "AC04-5": ["R2", "R17"], "AC04-6": ["R4"], "AC04-7": ["R17"], "AC04-8": ["R2"],
  "AC05-1": ["R10"], "AC05-2": ["R10"], "AC05-3": ["R10"], "AC05-4": ["R10"], "AC05-5": ["R10"],
  "AC05-6": ["R6", "R9"], "AC05-7": ["R1", "R9", "R11"],
  "AC06-1": ["R13"], "AC06-2": ["R13"], "AC06-3": ["R13"], "AC06-4": ["R14"], "AC06-5": ["R13", "R14"],
  "AC07-1": ["R18"], "AC07-2": ["R5"], "AC07-3": ["R1"], "AC07-4": ["R4"], "AC07-5": ["R3"],
  "AC07-6": ["R7"], "AC07-7": ["R1"], "AC07-8": ["R17"], "AC07-9": ["R15", "R16", "R19"],
};

const inspectionAcs = new Set(["AC07-8", "AC07-9"]);

const taskAc = {
  T1: ["AC01-1", "AC01-2", "AC01-3", "AC01-4", "AC01-5", "AC01-6", "AC01-7", "AC01-8"],
  T2: ["AC02-1", "AC02-2", "AC02-3", "AC02-4", "AC02-5", "AC02-6", "AC02-7"],
  T3: ["AC03-1", "AC03-2", "AC03-3", "AC03-4", "AC03-5", "AC03-6", "AC03-7"],
  T4: ["AC04-1", "AC04-2", "AC04-3", "AC04-4", "AC04-5", "AC04-6", "AC04-7", "AC04-8"],
  T5: ["AC05-1", "AC05-2", "AC05-3", "AC05-4", "AC05-5", "AC05-6", "AC05-7", "AC04-8"],
  T6: ["AC06-1", "AC06-2", "AC06-3", "AC06-4", "AC06-5", "AC04-8"],
  T7: ["AC07-1", "AC07-2", "AC07-3", "AC07-4", "AC07-5", "AC07-6", "AC07-7", "AC07-8", "AC07-9", "AC04-8"],
};

const legacyTests = [
  "cmd/mcp-bridge/diagnostic_deps_test.go",
  "cmd/mcp-bridge/embedding_env_durable_test.go",
  "cmd/mcp-bridge/embedding_env_reload_test.go",
  "cmd/mcp-bridge/embedding_env_rotation_test.go",
  "cmd/mcp-bridge/prompts_acceptance_test.go",
  "cmd/mcp-bridge/provenance_test.go",
  "cmd/mcp-bridge/runtime_edge_test.go",
  "cmd/mcp-bridge/serve_edge_test.go",
  "cmd/mcp-bridge/serve_endpoint_rebind_test.go",
  "cmd/mcp-bridge/serve_hybrid_coexistence_test.go",
  "cmd/mcp-bridge/serve_hybrid_dispatch_test.go",
  "cmd/mcp-bridge/serve_hybrid_exposure_test.go",
  "cmd/mcp-bridge/serve_hybrid_metadata_defaults_test.go",
  "cmd/mcp-bridge/serve_hybrid_metadata_exceptions_test.go",
  "cmd/mcp-bridge/serve_hybrid_metadata_source_test.go",
  "cmd/mcp-bridge/serve_hybrid_metadata_test.go",
  "cmd/mcp-bridge/serve_hybrid_progressive_reveal_lifecycle_test.go",
  "cmd/mcp-bridge/serve_hybrid_progressive_reveal_test.go",
  "cmd/mcp-bridge/serve_hybrid_progressive_session_test.go",
  "cmd/mcp-bridge/serve_hybrid_reload_test.go",
  "cmd/mcp-bridge/serve_logging_compatibility_test.go",
  "cmd/mcp-bridge/serve_logging_concurrent_test.go",
  "cmd/mcp-bridge/serve_logging_execution_test.go",
  "cmd/mcp-bridge/serve_logging_failures_test.go",
  "cmd/mcp-bridge/serve_logging_shapes_test.go",
  "cmd/mcp-bridge/serve_logging_streaming_test.go",
  "cmd/mcp-bridge/serve_logging_test.go",
  "cmd/mcp-bridge/serve_oauth_diagnostics_test.go",
  "cmd/mcp-bridge/serve_oauth_test.go",
  "cmd/mcp-bridge/serve_provenance_test.go",
  "cmd/mcp-bridge/serve_test.go",
  "cmd/mcp-bridge/serve_upstream_restart_test.go",
  "internal/observe/edge_test.go",
  "internal/observe/httpdiagnostic_provenance_test.go",
  "internal/observe/httpdiagnostics_test.go",
  "internal/observe/log_http_fields_test.go",
  "internal/observe/log_manifest_test.go",
  "internal/observe/log_test.go",
  "internal/observe/session_slot_diag_test.go",
];

const serviceTests = [
  "internal/service/inbound_test.go",
  "internal/service/inbound_ownership_test.go",
  "internal/service/inbound_logging_fault_test.go",
  "internal/service/session_diag_test.go",
  "internal/service/tools_test.go",
  "internal/service/service_edge_test.go",
];

const configTests = [
  "internal/config/pins_bounds_test.go",
  "internal/config/bounds_edge_test.go",
  "internal/config/aggregate_edge_test.go",
  "internal/config/parser_edges_test.go",
  "internal/config/api_edges_test.go",
  "internal/config/validate_edge_test.go",
  "internal/config/secret_validation_edge_test.go",
  "internal/config/config_test.go",
  "internal/config/diagnostics_test.go",
  "internal/config/command_test.go",
  "internal/config/embedding_env_test.go",
  "internal/config/secret_sequence_edge_test.go",
];

const taskPaths = {
  T1: [
    "internal/observe/envelope.go",
    "internal/observe/log.go",
    "internal/config/config.go",
    "internal/config/diagnostics.go",
    "internal/config/pins_bounds_test.go",
    "cmd/mcp-bridge/diagnostics_envelope.go",
    "cmd/mcp-bridge/main.go",
    "cmd/mcp-bridge/config_envelope_test.go",
    "cmd/mcp-bridge/example_config_test.go",
    "cmd/mcp-bridge/serve_edge_test.go",
    "cmd/mcp-bridge/main_test.go",
    "cmd/mcp-bridge/embedding_env_cli_test.go",
    "cmd/mcp-bridge/embedding_env_process_test.go",
  ],
  T2: [
    "internal/config/config.go",
    "internal/config/diagnostics.go",
    "internal/config/bounds.go",
    "internal/config/tool_metadata.go",
    "cmd/mcp-bridge/diagnostics_envelope.go",
    "cmd/mcp-bridge/diagnostics_envelope_test.go",
    ...configTests,
  ],
  T3: [
    "cmd/mcp-bridge/startup_failure.go",
    "cmd/mcp-bridge/main.go",
    "cmd/mcp-bridge/serve.go",
    "cmd/mcp-bridge/startup_failure_test.go",
    "cmd/mcp-bridge/runtime_edge_test.go",
    "cmd/mcp-bridge/serve_edge_test.go",
  ],
  T4: [
    "internal/observe/log.go",
    "internal/observe/envelope.go",
    "internal/observe/httpdiagnostic_encode.go",
    "internal/observe/httpdiagnostics.go",
    "internal/observe/httpdiagnostic_types.go",
    "internal/observe/httpwriter_test.go",
    "internal/upstream/dedicated_test.go",
    "internal/upstream/dedicated_edge_test.go",
    "internal/upstream/dedicated_wire_edge_test.go",
    "internal/upstream/prompts_edge_test.go",
    "internal/upstream/tool_metadata_test.go",
    "internal/service/tools.go",
    "internal/service/direct_execution.go",
    "internal/upstream/dedicated.go",
    "internal/auth/middleware_test.go",
    "cmd/mcp-bridge/serve.go",
    "cmd/mcp-bridge/wiring.go",
    "cmd/mcp-bridge/manifest_test.go",
    ...serviceTests,
    ...legacyTests,
  ],
  T5: [
    "cmd/mcp-bridge/wiring.go",
    "cmd/mcp-bridge/serve.go",
    "cmd/mcp-bridge/manifest_test.go",
    "cmd/mcp-bridge/serve_edge_test.go",
    "cmd/mcp-bridge/reload_envelope_test.go",
  ],
  T6: [
    "cmd/mcp-bridge/main.go",
    "cmd/mcp-bridge/config_envelope_test.go",
    "cmd/mcp-bridge/example_config_test.go",
    "cmd/mcp-bridge/serve_edge_test.go",
    "cmd/mcp-bridge/main_test.go",
    "cmd/mcp-bridge/embedding_env_cli_test.go",
    "cmd/mcp-bridge/embedding_env_process_test.go",
  ],
  T7: [
    "docs/logging-contract.md",
    "README.md",
    "CHANGELOG.md",
    "cmd/mcp-bridge/logging_contract_test.go",
  ],
};

const blockedBy = { T1: [], T2: ["T1"], T3: ["T1"], T4: ["T1"], T5: ["T2", "T4"], T6: ["T1"], T7: ["T3", "T4", "T5", "T6"] };

const tasks = taskIds.map(id => ({
  id,
  stage: "implementation",
  status: "pending",
  blockedBy: blockedBy[id],
  requires: [],
  effects: { kind: "write", paths: [...taskPaths[id], `.omp/artifacts/receipt_${id}.json`] },
  acceptanceIds: taskAc[id],
  evidenceIds: [],
  agentRoles: ["engineer"],
}));

const multiOwner = { "AC04-8": ["T4", "T5", "T6", "T7"] };

const acceptance = Object.entries(acMap).map(([id, requirementIds]) => ({
  id,
  requirementIds,
  ownerTaskIds: multiOwner[id] ?? [`T${Number(id.slice(2, 4))}`],
  method: inspectionAcs.has(id) ? "inspection" : "test",
  requiredEvidenceKinds: [inspectionAcs.has(id) ? "inspection" : "test"],
}));

const writePaths = [...new Set([...Object.values(taskPaths).flat(), ...taskIds.map(id => `.omp/artifacts/receipt_${id}.json`)])];

const state = {
  schemaVersion: 2,
  feature,
  revision,
  projectRoot: root,
  spec: { path: specPath, sha256: v2Sha(specText), sectionBindings: bindings },
  packet: { digest: packetDigest, branches: [branch], selectedBranchId: null },
  actors: { plannerId: "planner", implementerIds: ["engineer"] },
  requirements,
  authorization: null,
  reviews: null,
  sources,
  decisions: [],
  envelope: {
    id: "env-1",
    root,
    allowedTaskIds: taskIds,
    readPaths: [
      "internal/observe/",
      "internal/config/",
      "internal/service/",
      "internal/upstream/",
      "internal/auth/",
      "cmd/mcp-bridge/",
      ".scratch/structured-logging/",
      "docs/",
      "README.md",
      "go.mod",
      ".omp/artifacts/",
    ],
    writePaths,
    allowedEffects: ["read", "write", "test"],
    forbiddenEffects: ["network", "publish", "deploy", "schedule", "credentials", "infrastructure"],
    decisionPolicy: { technicalDecisionIds: [], ownerCategories: [] },
    retryPolicy: { maxAttempts: 1, resetOn: ["owner-signal", "material-evidence"] },
  },
  tasks,
  capabilities: [],
  acceptance,
  evidence: [],
  blockers: [],
  gate: { markerPath: ".superpowers/sdd/GATE_OPEN", activationEvidenceIds: [] },
};

const shapeErrors = validateShape(state);
if (shapeErrors.length) {
  console.error("shape errors:", JSON.stringify(shapeErrors, null, 2));
  process.exit(1);
}

const planDir = path.join(root, ".omp", "plans", feature);
fs.mkdirSync(planDir, { recursive: true });
const stateFile = path.join(planDir, "execution-state.json");
fs.writeFileSync(stateFile, JSON.stringify(state, null, 2) + "\n", "utf8");
fs.writeFileSync(path.join(root, ".omp", "plans", "execution-state.json"), JSON.stringify(state, null, 2) + "\n", "utf8");
console.log("wrote", stateFile, "and canonical");
console.log("packetDigest", packetDigest);
console.log("specSha", state.spec.sha256);
console.log("writePaths", writePaths.length, "acceptance", acceptance.length);

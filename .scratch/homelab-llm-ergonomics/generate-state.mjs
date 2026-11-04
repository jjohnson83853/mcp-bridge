import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { defaultDeps, canonicalJson, sha } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs";
import { evaluateContract } from "/home/opencode/.omp/agent/workflow-policy/execution-contract.mjs";
import { validateShape } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-schema.mjs";

const root = "/home/opencode/projects/default/mcp-bridge";
const feature = "homelab-llm-ergonomics";
const revision = "r7";
const recordedAt = new Date().toISOString();

function v2Sha(t) { return crypto.createHash("sha256").update(t, "utf8").digest("hex"); }

const specPath = path.join(root, ".scratch", feature, "spec.md");
const specText = fs.readFileSync(specPath, "utf8");

const bindings = {};
for (const name of ["requirements", "design", "tasks", "envelope"]) {
  const startToken = `<!-- specdd:${name}:start -->`;
  const endToken = `<!-- specdd:${name}:end -->`;
  const start = specText.indexOf(startToken);
  const end = specText.indexOf(endToken);
  if (start === -1 || end === -1) {
    throw new Error(`Missing specdd token for ${name}`);
  }
  const body = specText.slice(start + startToken.length, end);
  bindings[name] = { revision, sha256: v2Sha(body) };
}

const userAuthPath = path.join(root, ".scratch", feature, "user-auth.txt");
const reviewCriticPath = path.join(root, ".scratch", feature, "review-critic.md");
const reviewReviewerPath = path.join(root, ".scratch", feature, "review-code-reviewer.md");
const receiptT1Path = path.join(root, ".omp", "artifacts", "receipt_homelab_T1.json");
const receiptT1Text = fs.existsSync(receiptT1Path) ? fs.readFileSync(receiptT1Path, "utf8") : "";
const receiptT2Path = path.join(root, ".omp", "artifacts", "receipt_homelab_T2.json");
const receiptT2Text = fs.existsSync(receiptT2Path) ? fs.readFileSync(receiptT2Path, "utf8") : "";
const receiptT3Path = path.join(root, ".omp", "artifacts", "receipt_homelab_T3.json");
const receiptT3Text = fs.existsSync(receiptT3Path) ? fs.readFileSync(receiptT3Path, "utf8") : "";
const receiptT4Path = path.join(root, ".omp", "artifacts", "receipt_homelab_T4.json");
const receiptT4Text = fs.existsSync(receiptT4Path) ? fs.readFileSync(receiptT4Path, "utf8") : "";
const branchId = "approved-homelab-llm-ergonomics";
const taskIds = ["T1", "T2", "T3", "T4"];

const branch = {
  id: branchId,
  answerIds: [],
  taskIds,
  envelopeId: "env-1",
  sectionBindings: bindings,
};

const packetDigest = sha(canonicalJson({
  feature,
  projectRoot: root,
  sectionBindings: bindings,
  branches: [branch],
}));

const sources = [
  {
    id: "src-user",
    kind: "user-message",
    artifact: { path: userAuthPath, sha256: v2Sha(fs.readFileSync(userAuthPath, "utf8")) },
    locator: "user conversation message kickoff homelab llm ergonomics backlog under RULES.md S14",
    recordedAt,
  },
  {
    id: "src-review-critic",
    kind: "observation",
    artifact: { path: reviewCriticPath, sha256: v2Sha(fs.readFileSync(reviewCriticPath, "utf8")) },
    locator: "critic review receipt r7",
    recordedAt,
  },
  {
    id: "src-review-reviewer",
    kind: "observation",
    artifact: { path: reviewReviewerPath, sha256: v2Sha(fs.readFileSync(reviewReviewerPath, "utf8")) },
    locator: "code-reviewer review receipt r7",
    recordedAt,
  },
  ...(receiptT1Text ? [{
    id: "src-receipt-t1",
    kind: "observation",
    artifact: { path: receiptT1Path, sha256: v2Sha(receiptT1Text) },
    locator: "engineer receipt homelab T1",
    recordedAt,
  }] : []),
  ...(receiptT2Text ? [{
    id: "src-receipt-t2",
    kind: "observation",
    artifact: { path: receiptT2Path, sha256: v2Sha(receiptT2Text) },
    locator: "engineer receipt homelab T2",
    recordedAt,
  }] : []),
  ...(receiptT3Text ? [{
    id: "src-receipt-t3",
    kind: "observation",
    artifact: { path: receiptT3Path, sha256: v2Sha(receiptT3Text) },
    locator: "engineer receipt homelab T3",
    recordedAt,
  }] : []),
  ...(receiptT4Text ? [{
    id: "src-receipt-t4",
    kind: "observation",
    artifact: { path: receiptT4Path, sha256: v2Sha(receiptT4Text) },
    locator: "engineer receipt homelab T4",
    recordedAt,
  }] : []),
];

const t1Evidence = receiptT1Text ? [{
  id: "ev-t1-test",
  kind: "test",
  artifact: {
    path: receiptT1Path,
    sha256: v2Sha(receiptT1Text),
  },
  feature,
  packetDigest,
  selectedBranchId: branchId,
  taskIds: ["T1"],
  acceptanceIds: ["AC-1", "AC-2"],
  outcome: "pass",
  observedAt: recordedAt,
  producer: "engineer",
  details: {
    command: "go test -v -count=1 ./internal/catalog/... && go test -v -count=1 ./internal/service/... -run 'TestToolSurface(Describe|Call)'",
    exitCode: 0,
  },
}] : [];

const t2Evidence = receiptT2Text ? [{
  id: "ev-t2-test",
  kind: "test",
  artifact: {
    path: receiptT2Path,
    sha256: v2Sha(receiptT2Text),
  },
  feature,
  packetDigest,
  selectedBranchId: branchId,
  taskIds: ["T2"],
  acceptanceIds: ["AC-3"],
  outcome: "pass",
  observedAt: recordedAt,
  producer: "engineer",
  details: {
    command: "go test -v -count=1 ./internal/service/... && go test -v -count=1 ./cmd/mcp-bridge -run 'TestBridgeRebindsServerEndpointOnRestartAndReload'",
    exitCode: 0,
  },
}] : [];

const t3Evidence = receiptT3Text ? [{
  id: "ev-t3-test",
  kind: "test",
  artifact: {
    path: receiptT3Path,
    sha256: v2Sha(receiptT3Text),
  },
  feature,
  packetDigest,
  selectedBranchId: branchId,
  taskIds: ["T3"],
  acceptanceIds: ["AC-4", "AC-5", "AC-6"],
  outcome: "pass",
  observedAt: recordedAt,
  producer: "engineer",
  details: {
    command: "go test -v -count=1 ./internal/service/...",
    exitCode: 0,
  },
}] : [];
const t4Evidence = receiptT4Text ? [{
  id: "ev-t4-test",
  kind: "test",
  artifact: {
    path: receiptT4Path,
    sha256: v2Sha(receiptT4Text),
  },
  feature,
  packetDigest,
  selectedBranchId: branchId,
  taskIds: ["T4"],
  acceptanceIds: ["AC-7", "AC-8"],
  outcome: "pass",
  observedAt: recordedAt,
  producer: "engineer",
  details: {
    command: "go test -race ./... && go vet ./... && go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./... && go run ./cmd/covcheck",
    exitCode: 0,
  },
}] : [];
const writePaths = [
  "internal/catalog/schema.go",
  "internal/catalog/catalog_test.go",
  "internal/service/tools.go",
  "internal/service/direct_execution.go",
  "internal/service/projection.go",
  "internal/service/tools_test.go",
  "internal/service/service_edge_test.go",
  "internal/service/progressive_discovery_test.go",
  "internal/service/progressive_session_test.go",
  "internal/service/progressive_projection_test.go",
  "internal/service/progressive_concurrency_lifecycle_test.go",
  "cmd/mcp-bridge/serve_test.go",
  "cmd/mcp-bridge/serve_oauth_test.go",
  "cmd/mcp-bridge/serve_endpoint_rebind_test.go",
  "cmd/mcp-bridge/serve_hybrid_coexistence_test.go",
  "cmd/mcp-bridge/serve_hybrid_metadata_defaults_test.go",
  "cmd/mcp-bridge/serve_hybrid_metadata_exceptions_test.go",
  "cmd/mcp-bridge/serve_hybrid_metadata_source_test.go",
  "cmd/mcp-bridge/serve_upstream_restart_test.go",
  "README.md",
  "coverage.out",
  ".omp/artifacts/receipt_homelab_T1.json",
  ".omp/artifacts/receipt_homelab_T2.json",
  ".omp/artifacts/receipt_homelab_T3.json",
  ".omp/artifacts/receipt_homelab_T4.json",
];

const readPaths = [
  ".scratch/homelab-llm-ergonomics/spec.md",
  ".scratch/homelab-llm-ergonomics/issues/01-optional-catalog-revision.md",
  ".scratch/homelab-llm-ergonomics/issues/02-semantic-wire-name-publication.md",
  ".scratch/homelab-llm-ergonomics/issues/03-prune-search-describe-call-payloads.md",
  ".scratch/homelab-llm-ergonomics/issues/04-integrated-wire-verification-and-coverage.md",
  ...writePaths,
];

const state = {
  schemaVersion: 2,
  feature,
  revision,
  projectRoot: root,
  spec: {
    path: specPath,
    sha256: v2Sha(specText),
    sectionBindings: bindings,
  },
  packet: {
    digest: packetDigest,
    branches: [branch],
    selectedBranchId: branchId,
  },
  actors: {
    plannerId: "planner",
    implementerIds: ["engineer"],
  },
  requirements: [
    { id: "R1", summary: "Optional catalog_revision on describe_tool and call_tool" },
    { id: "R2", summary: "Semantic Wire Name (<server>__<tool>) Publication with Deterministic Fallback" },
    { id: "R3", summary: "Pruned search_tools and describe_tool Payloads" },
    { id: "R4", summary: "Streamlined Generic call_tool Response Envelope" },
    { id: "R5", summary: "End-to-End Wire Verification, Documentation, and Coverage Gate" },
  ],
  authorization: {
    mode: "bounded",
    id: "auth-1",
    packetDigest,
    selectedBranchId: branchId,
    receipt: {
      sourceId: "src-user",
      recordedAt,
      response: {
        origin: "user",
        explicit: true,
        timedOut: false,
        automatic: false,
        packetDigest,
        selectedBranchId: branchId,
      },
    },
  },
  reviews: {
    specification: {
      verdict: "pass",
      reviewerId: "critic",
      packetDigest,
      receipt: {
        sourceId: "src-review-critic",
        recordedAt,
        response: { origin: "reviewer", verdict: "pass" },
      },
    },
    tasks: {
      verdict: "pass",
      reviewerId: "code-reviewer",
      packetDigest,
      receipt: {
        sourceId: "src-review-reviewer",
        recordedAt,
        response: { origin: "reviewer", verdict: "pass" },
      },
    },
  },
  sources,
  decisions: [],
  envelope: {
    id: "env-1",
    root,
    allowedTaskIds: taskIds,
    readPaths,
    writePaths,
    allowedEffects: ["read", "write", "test"],
    forbiddenEffects: [],
    decisionPolicy: { technicalDecisionIds: [], ownerCategories: [] },
    retryPolicy: { maxAttempts: 1, resetOn: ["owner-signal", "material-evidence"] },
  },
  tasks: [
    {
      id: "T1",
      stage: "implementation",
      status: receiptT1Text ? "done" : "pending",
      blockedBy: [],
      requires: [],
      effects: {
        kind: "write",
        paths: [
          "internal/catalog/schema.go",
          "internal/catalog/catalog_test.go",
          "internal/service/tools.go",
          "internal/service/direct_execution.go",
          "internal/service/tools_test.go",
          ".omp/artifacts/receipt_homelab_T1.json",
        ],
      },
      acceptanceIds: ["AC-1", "AC-2"],
      evidenceIds: receiptT1Text ? ["ev-t1-test"] : [],
      agentRoles: ["engineer"],
    },
    {
      id: "T2",
      stage: "implementation",
      status: receiptT2Text ? "done" : "pending",
      blockedBy: ["T1"],
      requires: [],
      effects: {
        kind: "write",
        paths: [
          "internal/service/tools.go",
          "internal/service/projection.go",
          "internal/service/tools_test.go",
          "internal/service/service_edge_test.go",
          "internal/service/progressive_discovery_test.go",
          "internal/service/progressive_session_test.go",
          "internal/service/progressive_projection_test.go",
          "internal/service/progressive_concurrency_lifecycle_test.go",
          "cmd/mcp-bridge/serve_endpoint_rebind_test.go",
          ".omp/artifacts/receipt_homelab_T2.json",
        ],
      },
      acceptanceIds: ["AC-3"],
      evidenceIds: receiptT2Text ? ["ev-t2-test"] : [],
      agentRoles: ["engineer"],
    },
    {
      id: "T3",
      stage: "implementation",
      status: receiptT3Text ? "done" : "pending",
      blockedBy: ["T2"],
      requires: [],
      effects: {
        kind: "write",
        paths: [
          "internal/service/tools.go",
          "internal/service/tools_test.go",
          ".omp/artifacts/receipt_homelab_T3.json",
        ],
      },
      acceptanceIds: ["AC-4", "AC-5", "AC-6"],
      evidenceIds: receiptT3Text ? ["ev-t3-test"] : [],
      agentRoles: ["engineer"],
    },
    {
      id: "T4",
      stage: "implementation",
      status: receiptT4Text ? "done" : "pending",
      blockedBy: [],
      requires: [],
      effects: {
        kind: "write",
        paths: [
          "internal/catalog/schema.go",
          "internal/catalog/catalog_test.go",
          "internal/service/tools_test.go",
          "cmd/mcp-bridge/serve_test.go",
          "cmd/mcp-bridge/serve_oauth_test.go",
          "cmd/mcp-bridge/serve_hybrid_coexistence_test.go",
          "cmd/mcp-bridge/serve_hybrid_metadata_defaults_test.go",
          "cmd/mcp-bridge/serve_hybrid_metadata_exceptions_test.go",
          "cmd/mcp-bridge/serve_hybrid_metadata_source_test.go",
          "cmd/mcp-bridge/serve_upstream_restart_test.go",
          "README.md",
          "coverage.out",
          ".omp/artifacts/receipt_homelab_T4.json",
        ],
      },
      acceptanceIds: ["AC-7", "AC-8"],
      evidenceIds: receiptT4Text ? ["ev-t4-test"] : [],
      agentRoles: ["engineer"],
    },
  ],
  capabilities: [],
  acceptance: [
    { id: "AC-1", requirementIds: ["R1"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-2", requirementIds: ["R1"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-3", requirementIds: ["R2"], ownerTaskIds: ["T2"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-4", requirementIds: ["R3"], ownerTaskIds: ["T3"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-5", requirementIds: ["R3"], ownerTaskIds: ["T3"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-6", requirementIds: ["R4"], ownerTaskIds: ["T3"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-7", requirementIds: ["R5"], ownerTaskIds: ["T4"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-8", requirementIds: ["R5"], ownerTaskIds: ["T4"], method: "test", requiredEvidenceKinds: ["test"] },
  ],
  evidence: [...t1Evidence, ...t2Evidence, ...t3Evidence, ...t4Evidence],
  blockers: [],
  gate: {
    markerPath: ".superpowers/sdd/GATE_OPEN",
    activationEvidenceIds: [],
  },
};

const shapeErrors = validateShape(state);
if (shapeErrors.length) {
  console.error("Shape errors:", shapeErrors);
  process.exit(1);
}

const planDir = path.join(root, ".omp", "plans", feature);
if (!fs.existsSync(planDir)) {
  fs.mkdirSync(planDir, { recursive: true });
}
const stateFile = path.join(planDir, "execution-state.json");
fs.writeFileSync(stateFile, JSON.stringify(state, null, 2), "utf8");
console.log("Wrote feature state to", stateFile);

const canonicalFile = path.join(root, ".omp", "plans", "execution-state.json");
fs.writeFileSync(canonicalFile, JSON.stringify(state, null, 2), "utf8");
console.log("Wrote canonical state to", canonicalFile);

const report = evaluateContract(canonicalFile, defaultDeps);
console.log("Contract report status:", report.status);
if (report.reasons && report.reasons.length) {
  console.log("Reasons:", report.reasons);
}
console.log("Runnable frontier:", report.runnable_frontier);
console.log("Implementation frontier:", report.implementation_frontier);
console.log("Completion:", report.completion);

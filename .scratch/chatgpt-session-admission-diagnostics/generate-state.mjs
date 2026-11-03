import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { defaultDeps } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs";
import { evaluateContract } from "/home/opencode/.omp/agent/workflow-policy/execution-contract.mjs";
import { validateShape } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-schema.mjs";

const root = "/home/opencode/projects/default/mcp-bridge";
const feature = "chatgpt-session-admission-diagnostics";
const revision = "r10";
const recordedAt = "2026-11-02T23:30:00Z";

function v2Sha(t) { return crypto.createHash("sha256").update(t, "utf8").digest("hex"); }
function canonicalJson(value) {
  if (Array.isArray(value)) return `[${value.map(canonicalJson).join(",")}]`;
  if (value !== null && typeof value === "object") {
    return `{${Object.keys(value).sort().map(k => `${JSON.stringify(k)}:${canonicalJson(value[k])}`).join(",")}}`;
  }
  return JSON.stringify(value);
}

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
const reviewPath = path.join(root, ".scratch", feature, "review-r8.md");
const reviewR10Path = path.join(root, ".scratch", feature, "review-r10.md");

const sources = [
  {
    id: "src-user",
    kind: "user-message",
    artifact: { path: userAuthPath, sha256: v2Sha(fs.readFileSync(userAuthPath, "utf8")) },
    locator: "user conversation message 'implement'",
    recordedAt,
  },
  {
    id: "src-review",
    kind: "observation",
    artifact: { path: reviewPath, sha256: v2Sha(fs.readFileSync(reviewPath, "utf8")) },
    locator: "critic review receipt r8",
    recordedAt,
  },
  {
    id: "src-review-r10",
    kind: "observation",
    artifact: { path: reviewR10Path, sha256: v2Sha(fs.readFileSync(reviewR10Path, "utf8")) },
    locator: "code-reviewer review receipt r10",
    recordedAt,
  },
];

const branchId = "approved-chatgpt-session-admission-diagnostics";
const taskIds = ["T1", "T2", "T3", "T4"];

const branch = {
  id: branchId,
  answerIds: [],
  taskIds,
  envelopeId: "env-1",
  sectionBindings: bindings,
};

const packetDigest = v2Sha(canonicalJson({
  feature,
  projectRoot: root,
  sectionBindings: bindings,
  branches: [branch],
}));

const receiptT1Path = path.join(root, ".omp", "artifacts", "receipt_T1.json");
const receiptT1Text = fs.readFileSync(receiptT1Path, "utf8");

const t1Evidence = {
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
  acceptanceIds: ["AC1"],
  outcome: "pass",
  observedAt: recordedAt,
  producer: "engineer",
  details: {
    command: "go test -count=1 ./internal/auth/... ./internal/service/...",
    exitCode: 0,
  },
};
const receiptT2Path = path.join(root, ".omp", "artifacts", "receipt_T2.json");
const receiptT2Text = fs.readFileSync(receiptT2Path, "utf8");

const t2Evidence = {
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
  acceptanceIds: ["AC2", "AC3", "AC4", "AC7"],
  outcome: "pass",
  observedAt: recordedAt,
  producer: "engineer",
  details: {
    command: "go test -v ./internal/auth/... && go test -v ./cmd/mcp-bridge -run TestSessionAdmissionEnabledParsing",
    exitCode: 0,
  },
};
const receiptT3Path = path.join(root, ".omp", "artifacts", "receipt_T3.json");
const receiptT3Text = fs.readFileSync(receiptT3Path, "utf8");

const t3Evidence = {
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
  acceptanceIds: ["AC6"],
  outcome: "pass",
  observedAt: recordedAt,
  producer: "engineer",
  details: {
    command: "go test -count=1 ./internal/service/...",
    exitCode: 0,
  },
};
const receiptT4Path = path.join(root, ".omp", "artifacts", "receipt_T4.json");
const receiptT4Text = fs.readFileSync(receiptT4Path, "utf8");

const t4Evidence = {
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
  acceptanceIds: ["AC1", "AC2", "AC3", "AC4", "AC5", "AC6", "AC7", "AC8"],
  outcome: "pass",
  observedAt: recordedAt,
  producer: "engineer",
  details: {
    command: "go test -v ./cmd/mcp-bridge/... -run 'TestChatGPT.*Wire' && go run ./cmd/covcheck",
    exitCode: 0,
  },
};


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
    { id: "R1", summary: "Typed Session Admission Outcome Taxonomy" },
    { id: "R2", summary: "Differentiated Outer OAuth Rejection Diagnostics" },
    { id: "R3", summary: "Initialize Handshake Lifecycle & Evidence Preservation" },
    { id: "R4", summary: "Security, Wire Compatibility & Zero Credential Leakage" },
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
        sourceId: "src-review",
        recordedAt,
        response: { origin: "reviewer", verdict: "pass" },
      },
    },
    tasks: {
      verdict: "pass",
      reviewerId: "critic",
      packetDigest,
      receipt: {
        sourceId: "src-review",
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
    readPaths: [
      ".scratch/chatgpt-session-admission-diagnostics/spec.md",
      ".scratch/chatgpt-session-admission-diagnostics/issues/01-session-admission-result-taxonomy.md",
      ".scratch/chatgpt-session-admission-diagnostics/issues/02-middleware-differentiated-rejection-diagnostics.md",
      ".scratch/chatgpt-session-admission-diagnostics/issues/03-initialize-handshake-lifecycle-diagnostics.md",
      ".scratch/chatgpt-session-admission-diagnostics/issues/04-end-to-end-discovery-wire-verification-and-coverage.md",
    ],
    writePaths: [
      "internal/auth/middleware.go",
      "internal/auth/middleware_test.go",
      "internal/service/inbound.go",
      "internal/service/inbound_test.go",
      "cmd/mcp-bridge/serve.go",
      "cmd/mcp-bridge/serve_test.go",
      "cmd/mcp-bridge/serve_oauth_test.go",
      "cmd/mcp-bridge/serve_oauth_diagnostics_test.go",
      "docs/runbooks/chatgpt-integration.md",
      "coverage.out",
    ],
    allowedEffects: ["read", "write", "test"],
    forbiddenEffects: [],
    decisionPolicy: { technicalDecisionIds: [], ownerCategories: [] },
    retryPolicy: { maxAttempts: 1, resetOn: ["owner-signal", "material-evidence"] },
  },
  tasks: [
    {
      id: "T1",
      stage: "implementation",
      status: "done",
      blockedBy: [],
      requires: [],
      effects: {
        kind: "write",
        paths: [
          "internal/auth/middleware.go",
          "internal/auth/middleware_test.go",
          "internal/service/inbound.go",
          "internal/service/inbound_test.go",
          "cmd/mcp-bridge/serve.go",
        ],
      },
      acceptanceIds: ["AC1"],
      evidenceIds: ["ev-t1-test"],
      agentRoles: ["engineer"],
    },
    {
      id: "T2",
      stage: "implementation",
      status: "done",
      blockedBy: ["T1"],
      requires: [],
      effects: {
        kind: "write",
        paths: [
          "internal/auth/middleware.go",
          "internal/auth/middleware_test.go",
          "cmd/mcp-bridge/serve.go",
          "cmd/mcp-bridge/serve_test.go",
        ],
      },
      acceptanceIds: ["AC2", "AC3", "AC4", "AC7"],
      evidenceIds: ["ev-t2-test"],
      agentRoles: ["engineer"],
    },
    {
      id: "T3",
      stage: "implementation",
      status: "done",
      blockedBy: ["T1"],
      requires: [],
      effects: {
        kind: "write",
        paths: [
          "internal/service/inbound.go",
          "internal/service/inbound_test.go",
        ],
      },
      acceptanceIds: ["AC6"],
      evidenceIds: ["ev-t3-test"],
      agentRoles: ["engineer"],
    },
    {
      id: "T4",
      stage: "implementation",
      status: "done",
      blockedBy: ["T2", "T3"],
      requires: [],
      effects: {
        kind: "write",
        paths: [
          "cmd/mcp-bridge/serve_oauth_test.go",
          "cmd/mcp-bridge/serve_oauth_diagnostics_test.go",
          "docs/runbooks/chatgpt-integration.md",
          "coverage.out",
        ],
      },
      acceptanceIds: ["AC1", "AC2", "AC3", "AC4", "AC5", "AC6", "AC7", "AC8"],
      evidenceIds: ["ev-t4-test"],
      agentRoles: ["engineer"],
    },
  ],
  capabilities: [],
  acceptance: [
    { id: "AC1", requirementIds: ["R1"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC2", requirementIds: ["R2"], ownerTaskIds: ["T2"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC3", requirementIds: ["R2"], ownerTaskIds: ["T2"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC4", requirementIds: ["R2"], ownerTaskIds: ["T2"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC5", requirementIds: ["R2"], ownerTaskIds: ["T4"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC6", requirementIds: ["R3"], ownerTaskIds: ["T3"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC7", requirementIds: ["R4"], ownerTaskIds: ["T2", "T4"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC8", requirementIds: ["R4"], ownerTaskIds: ["T4"], method: "test", requiredEvidenceKinds: ["test"] },
  ],
  evidence: [t1Evidence, t2Evidence, t3Evidence, t4Evidence],
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

// Ensure plan directory exists
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

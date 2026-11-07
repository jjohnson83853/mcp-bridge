import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { defaultDeps, canonicalJson, sha } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs";
import { evaluateContract } from "/home/opencode/.omp/agent/workflow-policy/execution-contract.mjs";
import { validateShape } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-schema.mjs";

const root = "/home/opencode/projects/default/mcp-bridge";
const feature = "gateway-tool-annotations";
const revision = "r3";
const recordedAt = "2026-11-07T12:00:00Z";

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

const branchId = "approved-gateway-tool-annotations";
const taskIds = ["T1", "T2"];

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
    locator: "user conversation message kickoff gateway tool annotations backlog under RULES.md S14",
    recordedAt,
  },
  {
    id: "src-review-critic",
    kind: "observation",
    artifact: { path: reviewCriticPath, sha256: v2Sha(fs.readFileSync(reviewCriticPath, "utf8")) },
    locator: "critic review receipt r3",
    recordedAt,
  },
  {
    id: "src-review-reviewer",
    kind: "observation",
    artifact: { path: reviewReviewerPath, sha256: v2Sha(fs.readFileSync(reviewReviewerPath, "utf8")) },
    locator: "code-reviewer review receipt r3",
    recordedAt,
  },
  {
    id: "src-receipt-t1",
    kind: "observation",
    artifact: { path: path.join(root, ".omp", "artifacts", "receipt_T1.json"), sha256: v2Sha(fs.readFileSync(path.join(root, ".omp", "artifacts", "receipt_T1.json"), "utf8")) },
    locator: "engineer receipt T1",
    recordedAt,
  },
  {
    id: "src-receipt-t2",
    kind: "observation",
    artifact: { path: path.join(root, ".omp", "artifacts", "receipt_T2.json"), sha256: v2Sha(fs.readFileSync(path.join(root, ".omp", "artifacts", "receipt_T2.json"), "utf8")) },
    locator: "engineer receipt T2",
    recordedAt,
  },
];

const t2Evidence = [
  {
    id: "ev-t2-test",
    kind: "test",
    artifact: {
      path: path.join(root, ".omp", "artifacts", "receipt_T2.json"),
      sha256: v2Sha(fs.readFileSync(path.join(root, ".omp", "artifacts", "receipt_T2.json"), "utf8")),
    },
    feature,
    packetDigest,
    selectedBranchId: branchId,
    taskIds: ["T2"],
    acceptanceIds: ["AC-4", "AC-5", "AC-6"],
    outcome: "pass",
    observedAt: recordedAt,
    producer: "engineer",
    details: {
      command: "go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./... && go run ./cmd/covcheck && go test -race ./...",
      exitCode: 0,
    },
  },
];

const t1Evidence = [
  {
    id: "ev-t1-test",
    kind: "test",
    artifact: {
      path: path.join(root, ".omp", "artifacts", "receipt_T1.json"),
      sha256: v2Sha(fs.readFileSync(path.join(root, ".omp", "artifacts", "receipt_T1.json"), "utf8")),
    },
    feature,
    packetDigest,
    selectedBranchId: branchId,
    taskIds: ["T1"],
    acceptanceIds: ["AC-1", "AC-2", "AC-3", "AC-5"],
    outcome: "pass",
    observedAt: recordedAt,
    producer: "engineer",
    details: {
      command: "go test -v ./cmd/mcp-bridge -run TestServeHybridMetadata && go test ./...",
      exitCode: 0,
    },
  },
];

const dummy = [
];

const writePaths = [
  "internal/service/tools.go",
  "cmd/mcp-bridge/serve_hybrid_metadata_test.go",
  "cmd/mcp-bridge/serve_hybrid_metadata_exceptions_test.go",
  "README.md",
  "CHANGELOG.md",
  "coverage.out",
  ".omp/artifacts/receipt_T1.json",
  ".omp/artifacts/receipt_T2.json",
];

const readPaths = [
  "spec.md",
  "internal/service/",
  "cmd/mcp-bridge/",
  "README.md",
  "CHANGELOG.md",
  "coverage.out",
  ".omp/artifacts/",
];

const envelope = {
  id: "env-1",
  root,
  allowedTaskIds: taskIds,
  readPaths,
  writePaths,
  allowedEffects: ["read", "write", "test"],
  forbiddenEffects: ["network", "publish", "deploy", "schedule", "credentials", "infrastructure"],
  decisionPolicy: {
    technicalDecisionIds: [],
    ownerCategories: [],
  },
  retryPolicy: {
    maxAttempts: 1,
    resetOn: ["owner-signal", "material-evidence"],
  },
};

const tasks = [
  {
    id: "T1",
    stage: "implementation",
    status: "done",
    blockedBy: [],
    requires: [],
    effects: {
      kind: "write",
      paths: [
        "internal/service/tools.go",
        "cmd/mcp-bridge/serve_hybrid_metadata_test.go",
        "README.md",
        "CHANGELOG.md",
        ".omp/artifacts/receipt_T1.json",
      ],
    },
    acceptanceIds: ["AC-1", "AC-2", "AC-3", "AC-5"],
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
        "cmd/mcp-bridge/serve_hybrid_metadata_exceptions_test.go",
        "coverage.out",
        ".omp/artifacts/receipt_T2.json",
      ],
    },
    acceptanceIds: ["AC-4", "AC-6"],
    evidenceIds: ["ev-t2-test"],
    agentRoles: ["engineer"],
  },
];

const acceptance = [
  { id: "AC-1", requirementIds: ["R1", "R3"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
  { id: "AC-2", requirementIds: ["R2", "R3"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
  { id: "AC-3", requirementIds: ["R4"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
  { id: "AC-4", requirementIds: ["R5"], ownerTaskIds: ["T2"], method: "test", requiredEvidenceKinds: ["test"] },
  { id: "AC-5", requirementIds: ["R6"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
  { id: "AC-6", requirementIds: ["R1", "R2", "R4", "R5"], ownerTaskIds: ["T2"], method: "test", requiredEvidenceKinds: ["test"] },
];

const requirements = [
  { id: "R1", summary: "search_tools explicitly emits readOnlyHint: true, destructiveHint: false, idempotentHint: true, openWorldHint: false" },
  { id: "R2", summary: "describe_tool explicitly emits readOnlyHint: true, destructiveHint: false, idempotentHint: true, openWorldHint: false" },
  { id: "R3", summary: "Discovery tools have non-contradictory annotations" },
  { id: "R4", summary: "call_tool retains conservative classification unchanged" },
  { id: "R5", summary: "Upstream and revealed tools pass through without heuristic injection" },
  { id: "R6", summary: "Operator documentation matches wire contract" },
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
  requirements,
  authorization: {
    mode: "milestone",
    id: "auth-milestone",
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
        milestone: true,
      },
    },
  },
  reviews: {
    specification: {
      verdict: "pass",
      reviewerId: "code-reviewer",
      packetDigest,
      receipt: {
        sourceId: "src-review-reviewer",
        recordedAt,
        response: { origin: "reviewer", verdict: "pass" },
      },
    },
    tasks: {
      verdict: "pass",
      reviewerId: "critic",
      packetDigest,
      receipt: {
        sourceId: "src-review-critic",
        recordedAt,
        response: { origin: "reviewer", verdict: "pass" },
      },
    },
  },
  sources,
  decisions: [],
  envelope,
  tasks,
  capabilities: [],
  acceptance,
  evidence: [...t1Evidence, ...t2Evidence],
  blockers: [],
  gate: {
    markerPath: ".superpowers/sdd/GATE_OPEN",
    activationEvidenceIds: [],
  },
};

const targetDir = path.join(root, ".omp", "plans", feature);
fs.mkdirSync(targetDir, { recursive: true });
const targetStateFile = path.join(targetDir, "execution-state.json");
fs.writeFileSync(targetStateFile, JSON.stringify(state, null, 2), "utf8");

const report = evaluateContract(targetStateFile, defaultDeps);
console.log("Evaluation of target state:", JSON.stringify(report, null, 2));

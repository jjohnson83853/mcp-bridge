import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { defaultDeps, canonicalJson, sha } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs";
import { evaluateContract } from "/home/opencode/.omp/agent/workflow-policy/execution-contract.mjs";
import { validateShape } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-schema.mjs";

const root = "/home/opencode/projects/default/mcp-bridge";
const feature = "openai-tool-schema-compatibility";
const revision = "r3";
const recordedAt = "2026-11-03T10:00:00Z";

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

const sources = [
  {
    id: "src-user",
    kind: "user-message",
    artifact: { path: userAuthPath, sha256: v2Sha(fs.readFileSync(userAuthPath, "utf8")) },
    locator: "user conversation message 'implement'",
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
];

const branchId = "approved-openai-tool-schema-compatibility";
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

const receiptT1Path = path.join(root, ".omp", "artifacts", "receipt_schema_T1.json");
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
  acceptanceIds: ["AC1", "AC2", "AC3", "AC4", "AC5"],
  outcome: "pass",
  observedAt: recordedAt,
  producer: "engineer",
  details: {
    command: "go test -v -count=1 ./internal/catalog/... ./internal/search/... ./internal/service/... -run TestToolSurfaceAdvertisesExactlyItsThreeTools",
    exitCode: 0,
  },
};

const receiptT2Path = path.join(root, ".omp", "artifacts", "receipt_schema_T2.json");
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
  acceptanceIds: ["AC6", "AC7", "AC8"],
  outcome: "pass",
  observedAt: recordedAt,
  producer: "engineer",
  details: {
    command: "go test -race -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./... && go run ./cmd/covcheck",
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
    { id: "R1", summary: "Root Dialect Omission & Standard Tool Manifest Alignment" },
    { id: "R2", summary: "Array Parameter Bounds Normalization & Decoder Enforcement" },
    { id: "R3", summary: "Generic Argument Object Definition" },
    { id: "R4", summary: "Parameter Optionality & String Constraints" },
    { id: "R5", summary: "End-to-End Action Discovery Wire Verification & Coverage Gate" },
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
    readPaths: [
      ".scratch/openai-tool-schema-compatibility/spec.md",
      ".scratch/openai-tool-schema-compatibility/issues/01-tool-input-schema-alignment-and-bounds-normalization.md",
      ".scratch/openai-tool-schema-compatibility/issues/02-end-to-end-action-discovery-wire-verification.md",
      "internal/catalog/schema.go",
      "internal/catalog/catalog_test.go",
      "internal/search/search.go",
      "internal/search/search_test.go",
      "internal/service/tools_test.go",
      "cmd/mcp-bridge/serve_oauth_test.go",
    ],
    writePaths: [
      "internal/catalog/schema.go",
      "internal/catalog/catalog_test.go",
      "internal/search/search.go",
      "internal/search/search_test.go",
      "internal/service/tools_test.go",
      "cmd/mcp-bridge/serve_oauth_test.go",
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
          "internal/catalog/schema.go",
          "internal/catalog/catalog_test.go",
          "internal/search/search.go",
          "internal/search/search_test.go",
          "internal/service/tools_test.go",
        ],
      },
      acceptanceIds: ["AC1", "AC2", "AC3", "AC4", "AC5"],
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
          "cmd/mcp-bridge/serve_oauth_test.go",
          "coverage.out",
        ],
      },
      acceptanceIds: ["AC6", "AC7", "AC8"],
      evidenceIds: ["ev-t2-test"],
      agentRoles: ["engineer"],
    },
  ],
  capabilities: [],
  acceptance: [
    { id: "AC1", requirementIds: ["R1"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC2", requirementIds: ["R2"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC3", requirementIds: ["R3"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC4", requirementIds: ["R4"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC5", requirementIds: ["R2"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC6", requirementIds: ["R5"], ownerTaskIds: ["T2"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC7", requirementIds: ["R5"], ownerTaskIds: ["T2"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC8", requirementIds: ["R5"], ownerTaskIds: ["T2"], method: "test", requiredEvidenceKinds: ["test"] },
  ],
  evidence: [t1Evidence, t2Evidence],
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

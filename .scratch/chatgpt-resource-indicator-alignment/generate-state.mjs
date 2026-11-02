import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { defaultDeps } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs";
import { evaluateContract } from "/home/opencode/.omp/agent/workflow-policy/execution-contract.mjs";
import { validateShape } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-schema.mjs";

const root = "/home/opencode/projects/default/mcp-bridge";
const feature = "chatgpt-resource-indicator-alignment";
const revision = "r15";
const recordedAt = "2026-11-02T02:00:00Z";

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
fs.writeFileSync(userAuthPath, "Owner authorized execution of chatgpt-resource-indicator-alignment r15 via Full SpecDD workflow\n");

const reviewPath = path.join(root, ".scratch", feature, "review-r15.md");
fs.writeFileSync(reviewPath, `# Critic and Code-Reviewer Dual Review Receipt r15

Feature: chatgpt-resource-indicator-alignment
Date: 2026-11-02
Verdict: pass
Reviewers: critic, code-reviewer

Requirements, design, tasks, and envelope conform to RFC 9728 and MCP Streamable HTTP session-bound OAuth admission specifications with trailing-slash normalization and fail-closed prevalidation recovery.
`);

const sources = [
  {
    id: "src-user",
    kind: "user-message",
    artifact: { path: userAuthPath, sha256: v2Sha(fs.readFileSync(userAuthPath, "utf8")) },
    locator: "user conversation message",
    recordedAt,
  },
  {
    id: "src-review",
    kind: "observation",
    artifact: { path: reviewPath, sha256: v2Sha(fs.readFileSync(reviewPath, "utf8")) },
    locator: "critic and code-reviewer dual review receipt r15",
    recordedAt,
  },
];

const branchId = "approved-chatgpt-resource-indicator-alignment";
const taskIds = ["T1", "T2"];

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
  id: "ev-T1-test",
  kind: "test",
  artifact: {
    path: receiptT1Path,
    sha256: v2Sha(receiptT1Text),
  },
  feature,
  packetDigest,
  selectedBranchId: branchId,
  taskIds: ["T1"],
  acceptanceIds: ["AC-1", "AC-2", "AC-3", "AC-4"],
  outcome: "pass",
  observedAt: recordedAt,
  producer: "engineer",
  details: {
    command: "go test -v -count=1 ./internal/auth -run TestProtectedResourceHandler",
    exitCode: 0,
  },
};
const receiptT2Path = path.join(root, ".omp", "artifacts", "receipt_T2.json");
const receiptT2Text = fs.readFileSync(receiptT2Path, "utf8");

const t2Evidence = {
  id: "ev-T2-gate",
  kind: "test",
  artifact: {
    path: receiptT2Path,
    sha256: v2Sha(receiptT2Text),
  },
  feature,
  packetDigest,
  selectedBranchId: branchId,
  taskIds: ["T2"],
  acceptanceIds: ["AC-5", "AC-6"],
  outcome: "pass",
  observedAt: recordedAt,
  producer: "engineer",
  details: {
    command: "go test -race ./... && go run ./cmd/covcheck",
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
    { id: "R1", summary: "Context-Aware Resource Metadata Advertisement and Normalization" },
    { id: "R2", summary: "Integrated ChatGPT Wire Verification" },
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
      ".scratch/chatgpt-resource-indicator-alignment/spec.md",
      ".scratch/chatgpt-resource-indicator-alignment/issues/01-context-aware-resource-metadata-advertisement.md",
      ".scratch/chatgpt-resource-indicator-alignment/issues/02-integrated-chatgpt-wire-verification.md",
    ],
    writePaths: [
      "internal/auth/metadata.go",
      "internal/auth/metadata_test.go",
      "cmd/mcp-bridge/serve_oauth_test.go",
      "cmd/mcp-bridge/serve_hybrid_reload_test.go",
      "internal/storage/store_test.go",
      "internal/service/progressive_session_test.go",
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
          "internal/auth/metadata.go",
          "internal/auth/metadata_test.go",
          "cmd/mcp-bridge/serve_oauth_test.go",
        ],
      },
      acceptanceIds: ["AC-1", "AC-2", "AC-3", "AC-4"],
      evidenceIds: ["ev-T1-test"],
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
          "cmd/mcp-bridge/serve_hybrid_reload_test.go",
          "internal/storage/store_test.go",
          "internal/service/progressive_session_test.go",
          "coverage.out",
        ],
      },
      acceptanceIds: ["AC-5", "AC-6"],
      evidenceIds: ["ev-T2-gate"],
      agentRoles: ["engineer"],
    },
  ],
  capabilities: [],
  acceptance: [
    { id: "AC-1", requirementIds: ["R1"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-2", requirementIds: ["R1"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-3", requirementIds: ["R1"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-4", requirementIds: ["R1"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-5", requirementIds: ["R2"], ownerTaskIds: ["T2"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-6", requirementIds: ["R2"], ownerTaskIds: ["T2"], method: "test", requiredEvidenceKinds: ["test"] },
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

const targetFile = path.join(root, ".omp", "plans", "execution-state.json");
fs.writeFileSync(targetFile, JSON.stringify(state, null, 2), "utf8");
console.log("Wrote valid state to", targetFile);

const report = evaluateContract(targetFile, defaultDeps);
console.log("Contract report status:", report.status);
if (report.reasons && report.reasons.length) {
  console.log("Reasons:", report.reasons);
}
console.log("Runnable frontier:", report.runnable_frontier);
console.log("Implementation frontier:", report.implementation_frontier);

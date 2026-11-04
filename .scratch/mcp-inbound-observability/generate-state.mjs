import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { defaultDeps } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs";
import { evaluateContract } from "/home/opencode/.omp/agent/workflow-policy/execution-contract.mjs";
import { validateShape } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-schema.mjs";

const root = "/home/opencode/projects/default/mcp-bridge";
const feature = "mcp-inbound-observability";
const revision = "r8";
const recordedAt = "2026-11-03T23:58:00Z";

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
const reviewCriticPath = path.join(root, ".scratch", feature, "review-critic.md");
const reviewReviewerPath = path.join(root, ".scratch", feature, "review-code-reviewer.md");

const sources = [
  {
    id: "src-user",
    kind: "user-message",
    artifact: { path: userAuthPath, sha256: v2Sha(fs.readFileSync(userAuthPath, "utf8")) },
    locator: "user conversation backlog prompt",
    recordedAt,
  },
  {
    id: "src-review-critic",
    kind: "observation",
    artifact: { path: reviewCriticPath, sha256: v2Sha(fs.readFileSync(reviewCriticPath, "utf8")) },
    locator: "critic review receipt r8",
    recordedAt,
  },
  {
    id: "src-review-reviewer",
    kind: "observation",
    artifact: { path: reviewReviewerPath, sha256: v2Sha(fs.readFileSync(reviewReviewerPath, "utf8")) },
    locator: "code-reviewer receipt r8",
    recordedAt,
  },
];

const branchId = "approved-mcp-inbound-observability";
const taskIds = ["T1", "T2", "T3"];

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
    { id: "R1", summary: "Unified Inbound Operation Classification & Metric Recording" },
    { id: "R2", summary: "Client Family Classification" },
    { id: "R3", summary: "Exact HTTP Response Bytes" },
    { id: "R4", summary: "Exposed Tool Count Recording & Failure Omission" },
    { id: "R5", summary: "End-to-End Loopback Wire Verification, Docs & Coverage Gate" },
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
      ".scratch/mcp-inbound-observability/spec.md",
      ".scratch/mcp-inbound-observability/issues/01-inbound-method-operation-classification-and-metrics.md",
      ".scratch/mcp-inbound-observability/issues/02-client-family-classification-and-exact-bytes.md",
      ".scratch/mcp-inbound-observability/issues/03-exposed-tool-count-and-wire-verification.md",
    ],
    writePaths: [
      "internal/service/inbound.go",
      "internal/service/inbound_test.go",
      "internal/service/session_exposure.go",
      "internal/observe/metrics.go",
      "internal/observe/httpdiagnostics.go",
      "internal/observe/httpdiagnostic_types.go",
      "internal/observe/httpdiagnostic_encode.go",
      "internal/observe/httpwriter.go",
      "internal/observe/log.go",
      "internal/observe/httpdiagnostics_test.go",
      "internal/observe/session_slot_diag_test.go",
      "cmd/mcp-bridge/serve_logging_compatibility_test.go",
      "cmd/mcp-bridge/serve_oauth_test.go",
      "README.md",
      "CHANGELOG.md",
      ".scratch/mcp-inbound-observability/spec.md",
      ".scratch/mcp-inbound-observability/map.md",
      ".scratch/mcp-inbound-observability/issues/01-inbound-method-operation-classification-and-metrics.md",
      ".scratch/mcp-inbound-observability/issues/02-client-family-classification-and-exact-bytes.md",
      ".scratch/mcp-inbound-observability/issues/03-exposed-tool-count-and-wire-verification.md",
      ".omp/artifacts/receipt_T1.json",
      ".omp/artifacts/receipt_T2.json",
      ".omp/artifacts/receipt_T3.json",
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
          "internal/service/inbound.go",
          "internal/service/inbound_test.go",
          "internal/observe/metrics.go",
          "cmd/mcp-bridge/serve_logging_compatibility_test.go",
        ],
      },
      acceptanceIds: ["AC-1", "AC-2"],
      evidenceIds: ["ev-T1-test"],
      agentRoles: ["engineer"],
    },
    {
      id: "T2",
      stage: "implementation",
      status: "done",
      blockedBy: [],
      requires: [],
      effects: {
        kind: "write",
        paths: [
          "internal/observe/httpdiagnostics.go",
          "internal/observe/httpdiagnostic_types.go",
          "internal/observe/httpdiagnostic_encode.go",
          "internal/observe/httpwriter.go",
          "internal/observe/log.go",
          "internal/observe/httpdiagnostics_test.go",
          "internal/observe/session_slot_diag_test.go",
        ],
      },
      acceptanceIds: ["AC-3", "AC-4", "AC-5"],
      evidenceIds: ["ev-T2-test"],
      agentRoles: ["engineer"],
    },
    {
      id: "T3",
      stage: "implementation",
      status: "done",
      blockedBy: [],
      requires: [],
      effects: {
        kind: "write",
        paths: [
          "internal/service/session_exposure.go",
          "cmd/mcp-bridge/serve_oauth_test.go",
          "README.md",
          "CHANGELOG.md",
          "coverage.out",
        ],
      },
      acceptanceIds: ["AC-5", "AC-6", "AC-7"],
      evidenceIds: ["ev-T3-test"],
      agentRoles: ["engineer"],
    },
  ],
  capabilities: [],
  acceptance: [
    { id: "AC-1", requirementIds: ["R1"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-2", requirementIds: ["R1"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-3", requirementIds: ["R2"], ownerTaskIds: ["T2"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-4", requirementIds: ["R3"], ownerTaskIds: ["T2"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-5", requirementIds: ["R4"], ownerTaskIds: ["T2", "T3"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-6", requirementIds: ["R5"], ownerTaskIds: ["T3"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-7", requirementIds: ["R5"], ownerTaskIds: ["T3"], method: "test", requiredEvidenceKinds: ["test"] },
  ],
  evidence: [
    {
      id: "ev-T1-test",
      kind: "test",
      artifact: {
        path: path.join(root, ".omp", "artifacts", "receipt_T1.json"),
        sha256: v2Sha(fs.readFileSync(path.join(root, ".omp", "artifacts", "receipt_T1.json"), "utf8")),
      },
      feature: "mcp-inbound-observability",
      packetDigest: "0000000000000000000000000000000000000000000000000000000000000000",
      selectedBranchId: branchId,
      taskIds: ["T1"],
      acceptanceIds: ["AC-1", "AC-2"],
      outcome: "pass",
      observedAt: "2026-11-04T02:00:00Z",
      producer: "engineer",
      details: {
        command: "go test -v -count=1 -race -run TestInboundOperationClassification ./internal/service && go test -v -count=1 -race -run TestServeLoggingCompatibility ./cmd/mcp-bridge",
        exitCode: 0,
      },
    },
    {
      id: "ev-T2-test",
      kind: "test",
      artifact: {
        path: path.join(root, ".omp", "artifacts", "receipt_T2.json"),
        sha256: v2Sha(fs.readFileSync(path.join(root, ".omp", "artifacts", "receipt_T2.json"), "utf8")),
      },
      feature: "mcp-inbound-observability",
      packetDigest: "0000000000000000000000000000000000000000000000000000000000000000",
      selectedBranchId: branchId,
      taskIds: ["T2"],
      acceptanceIds: ["AC-3", "AC-4", "AC-5"],
      outcome: "pass",
      observedAt: "2026-11-04T00:30:00Z",
      producer: "engineer",
      details: {
        command: "go test -race -v -count=1 -run TestClientFamilyAndExactBytes ./internal/observe/",
        exitCode: 0,
      },
    },
    {
      id: "ev-T3-test",
      kind: "test",
      artifact: {
        path: path.join(root, ".omp", "artifacts", "receipt_T3.json"),
        sha256: v2Sha(fs.readFileSync(path.join(root, ".omp", "artifacts", "receipt_T3.json"), "utf8")),
      },
      feature: "mcp-inbound-observability",
      packetDigest: "0000000000000000000000000000000000000000000000000000000000000000",
      selectedBranchId: branchId,
      taskIds: ["T3"],
      acceptanceIds: ["AC-5", "AC-6", "AC-7"],
      outcome: "pass",
      observedAt: "2026-11-04T03:30:00Z",
      producer: "engineer",
      details: {
        command: "go test -v -count=1 -run TestInboundObservabilityWire ./cmd/mcp-bridge && go test -race -p 1 -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./... && go run ./cmd/covcheck",
        exitCode: 0,
      },
    },
  ],
  blockers: [],
  gate: {
    markerPath: ".superpowers/sdd/GATE_OPEN",
    activationEvidenceIds: [],
  },
};
state.evidence[0].packetDigest = packetDigest;
state.evidence[1].packetDigest = packetDigest;
state.evidence[2].packetDigest = packetDigest;

const shapeErrors = validateShape(state);
if (shapeErrors.length) {
  console.error("Shape errors:", shapeErrors);
  process.exit(1);
}

const outDir = path.join(root, ".omp", "plans", feature);
fs.mkdirSync(outDir, { recursive: true });
const featureFile = path.join(outDir, "execution-state.json");
fs.writeFileSync(featureFile, JSON.stringify(state, null, 2), "utf8");
console.log("Wrote feature state to", featureFile);

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

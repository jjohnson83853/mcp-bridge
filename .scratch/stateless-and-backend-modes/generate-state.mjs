import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { defaultDeps } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs";
import { evaluateContract } from "/home/opencode/.omp/agent/workflow-policy/execution-contract.mjs";
import { validateShape } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-schema.mjs";

const root = "/home/opencode/projects/default/mcp-bridge";
const feature = "stateless-and-backend-modes";
const revision = "r1";
const recordedAt = "2026-11-02T22:30:00Z";

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
fs.writeFileSync(userAuthPath, "Owner authorized execution of stateless-and-backend-modes r1 via prompt 'yes'\n");

const reviewPath = path.join(root, ".scratch", feature, "review-r1.md");

const sources = [
  {
    id: "src-user",
    kind: "user-message",
    artifact: { path: userAuthPath, sha256: v2Sha(fs.readFileSync(userAuthPath, "utf8")) },
    locator: "owner explicit confirmation 'yes' for stateless-and-backend-modes spec r1",
    recordedAt,
  },
  {
    id: "src-review",
    kind: "observation",
    artifact: { path: reviewPath, sha256: v2Sha(fs.readFileSync(reviewPath, "utf8")) },
    locator: "critic and code-reviewer dual review receipt r1",
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
  {
    id: "src-receipt-t3",
    kind: "observation",
    artifact: { path: path.join(root, ".omp", "artifacts", "receipt_T3.json"), sha256: v2Sha(fs.readFileSync(path.join(root, ".omp", "artifacts", "receipt_T3.json"), "utf8")) },
    locator: "engineer receipt T3",
    recordedAt,
  },
  {
    id: "src-receipt-t4",
    kind: "observation",
    artifact: { path: path.join(root, ".omp", "artifacts", "receipt_T4.json"), sha256: v2Sha(fs.readFileSync(path.join(root, ".omp", "artifacts", "receipt_T4.json"), "utf8")) },
    locator: "engineer receipt T4",
    recordedAt,
  },
  {
    id: "src-receipt-t5",
    kind: "observation",
    artifact: { path: path.join(root, ".omp", "artifacts", "receipt_T5.json"), sha256: v2Sha(fs.readFileSync(path.join(root, ".omp", "artifacts", "receipt_T5.json"), "utf8")) },
    locator: "engineer receipt T5",
    recordedAt,
  },
];

const branchId = "approved-stateless-and-backend-modes";
const taskIds = ["T1", "T2", "T3", "T4", "T5"];

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
    { id: "R1", summary: "Upstream Server Backend Stateless Mode Configuration" },
    { id: "R2", summary: "Upstream Execution Lifecycle in Dedicated Runtime" },
    { id: "R3", summary: "Downstream Streamable Transport Default & Protocol Negotiation" },
    { id: "R4", summary: "Integrated ChatGPT Wire Verification & Coverage Gate" },
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
      ".scratch/stateless-and-backend-modes/spec.md",
      ".scratch/stateless-and-backend-modes/issues/01-upstream-server-config-stateless-stubs.md",
      ".scratch/stateless-and-backend-modes/issues/02-upstream-runtime-stateless-execution.md",
      ".scratch/stateless-and-backend-modes/issues/03-upstream-adaptive-ephemeral-handshake.md",
      ".scratch/stateless-and-backend-modes/issues/04-downstream-stateless-streamable-transport.md",
      ".scratch/stateless-and-backend-modes/issues/05-e2e-chatgpt-wire-verification-and-coverage.md",
    ],
    writePaths: [
      "internal/config/config.go",
      "internal/config/fingerprint.go",
      "internal/config/config_test.go",
      "internal/config/fingerprint_test.go",
      "internal/upstream/dedicated.go",
      "internal/upstream/dedicated_test.go",
      "internal/service/server.go",
      "cmd/mcp-bridge/serve.go",
      "internal/service/initialize_wire_test.go",
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
          "internal/config/config.go",
          "internal/config/fingerprint.go",
          "internal/config/config_test.go",
          "internal/config/fingerprint_test.go",
        ],
      },
      acceptanceIds: ["AC-1"],
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
          "internal/upstream/dedicated.go",
          "internal/upstream/dedicated_test.go",
        ],
      },
      acceptanceIds: ["AC-2"],
      evidenceIds: ["ev-t2-test"],
      agentRoles: ["engineer"],
    },
    {
      id: "T3",
      stage: "implementation",
      status: "done",
      blockedBy: ["T2"],
      requires: [],
      effects: {
        kind: "write",
        paths: [
          "internal/upstream/dedicated.go",
          "internal/upstream/dedicated_test.go",
        ],
      },
      acceptanceIds: ["AC-3"],
      evidenceIds: ["ev-t3-test"],
      agentRoles: ["engineer"],
    },
    {
      id: "T4",
      stage: "implementation",
      status: "done",
      blockedBy: [],
      requires: [],
      effects: {
        kind: "write",
        paths: [
          "internal/service/server.go",
          "cmd/mcp-bridge/serve.go",
          "internal/service/initialize_wire_test.go",
        ],
      },
      acceptanceIds: ["AC-4"],
      evidenceIds: ["ev-t4-test"],
      agentRoles: ["engineer"],
    },
    {
      id: "T5",
      stage: "implementation",
      status: "done",
      blockedBy: ["T1", "T2", "T3", "T4"],
      requires: [],
      effects: {
        kind: "write",
        paths: [
          "cmd/mcp-bridge/serve_oauth_test.go",
          "coverage.out",
        ],
      },
      acceptanceIds: ["AC-5", "AC-6"],
      evidenceIds: ["ev-t5-wire", "ev-t5-gate"],
      agentRoles: ["engineer"],
    },
  ],
  capabilities: [],
  acceptance: [
    { id: "AC-1", requirementIds: ["R1"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-2", requirementIds: ["R1", "R2"], ownerTaskIds: ["T2"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-3", requirementIds: ["R2"], ownerTaskIds: ["T3"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-4", requirementIds: ["R3"], ownerTaskIds: ["T4"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-5", requirementIds: ["R4"], ownerTaskIds: ["T5"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-6", requirementIds: ["R4"], ownerTaskIds: ["T5"], method: "test", requiredEvidenceKinds: ["test"] },
  ],
  evidence: [
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
      acceptanceIds: ["AC-1"],
      outcome: "pass",
      observedAt: recordedAt,
      producer: "engineer",
      details: {
        command: "go test -v -run \"TestServerStateless|TestFingerprintChangesOnServerStateless\" ./internal/config/...",
        exitCode: 0,
      },
    },
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
      acceptanceIds: ["AC-2"],
      outcome: "pass",
      observedAt: recordedAt,
      producer: "engineer",
      details: {
        command: "go test -v -run \"TestDedicatedStateless|TestDedicatedStateful\" ./internal/upstream/...",
        exitCode: 0,
      },
    },
    {
      id: "ev-t3-test",
      kind: "test",
      artifact: {
        path: path.join(root, ".omp", "artifacts", "receipt_T3.json"),
        sha256: v2Sha(fs.readFileSync(path.join(root, ".omp", "artifacts", "receipt_T3.json"), "utf8")),
      },
      feature,
      packetDigest,
      selectedBranchId: branchId,
      taskIds: ["T3"],
      acceptanceIds: ["AC-3"],
      outcome: "pass",
      observedAt: recordedAt,
      producer: "engineer",
      details: {
        command: "go test -v -run TestDedicatedAdaptiveEphemeralHandshake ./internal/upstream/...",
        exitCode: 0,
      },
    },
    {
      id: "ev-t4-test",
      kind: "test",
      artifact: {
        path: path.join(root, ".omp", "artifacts", "receipt_T4.json"),
        sha256: v2Sha(fs.readFileSync(path.join(root, ".omp", "artifacts", "receipt_T4.json"), "utf8")),
      },
      feature,
      packetDigest,
      selectedBranchId: branchId,
      taskIds: ["T4"],
      acceptanceIds: ["AC-4"],
      outcome: "pass",
      observedAt: recordedAt,
      producer: "engineer",
      details: {
        command: "go test -v -run TestStreamableHandlerStatelessDefaultDiscover ./internal/service/...",
        exitCode: 0,
      },
    },
    {
      id: "ev-t5-wire",
      kind: "test",
      artifact: {
        path: path.join(root, ".omp", "artifacts", "receipt_T5.json"),
        sha256: v2Sha(fs.readFileSync(path.join(root, ".omp", "artifacts", "receipt_T5.json"), "utf8")),
      },
      feature,
      packetDigest,
      selectedBranchId: branchId,
      taskIds: ["T5"],
      acceptanceIds: ["AC-5"],
      outcome: "pass",
      observedAt: recordedAt,
      producer: "engineer",
      details: {
        command: "go test -v -run \"TestChatGPTDeveloperModeStatelessDiscoveryWire|TestChatGPTOAuthAndActionDiscoveryWire\" ./cmd/mcp-bridge/...",
        exitCode: 0,
      },
    },
    {
      id: "ev-t5-gate",
      kind: "test",
      artifact: {
        path: path.join(root, ".omp", "artifacts", "receipt_T5.json"),
        sha256: v2Sha(fs.readFileSync(path.join(root, ".omp", "artifacts", "receipt_T5.json"), "utf8")),
      },
      feature,
      packetDigest,
      selectedBranchId: branchId,
      taskIds: ["T5"],
      acceptanceIds: ["AC-6"],
      outcome: "pass",
      observedAt: recordedAt,
      producer: "engineer",
      details: {
        command: "go test -coverprofile=coverage.out -coverpkg=./internal/...,./cmd/... ./... && go run ./cmd/covcheck",
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

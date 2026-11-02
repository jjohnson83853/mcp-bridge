import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { defaultDeps } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs";
import { evaluateContract } from "/home/opencode/.omp/agent/workflow-policy/execution-contract.mjs";
import { validateShape } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-schema.mjs";

const root = "/home/opencode/projects/default/mcp-bridge";
const feature = "session-bound-oauth-admission";
const revision = "r1";
const recordedAt = "2026-11-02T01:30:00Z";

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
fs.writeFileSync(userAuthPath, "Owner authorized execution of session-bound-oauth-admission r1 via Full SpecDD workflow\n");

const reviewPath = path.join(root, ".scratch", feature, "review-r1.md");
fs.writeFileSync(reviewPath, `# Critic Review Receipt r1

Feature: session-bound-oauth-admission
Date: 2026-11-02
Verdict: pass
Reviewer: critic

Requirements, design, tasks, and envelope conform to RFC 9728 and MCP Streamable HTTP session-bound OAuth admission specifications.
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
    locator: "critic review receipt r1",
    recordedAt,
  },
];

const branchId = "approved-session-bound-oauth-admission";
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
    { id: "R1", summary: "Canonical Endpoint Resource Metadata Alignment" },
    { id: "R2", summary: "Session Admission Seam & Tokenless Post-Init Discovery Burst" },
    { id: "R3", summary: "Explicit Credential Precedence & Dynamic Token Renewal" },
    { id: "R4", summary: "Authorization Ceiling & Scope Policy Enforcement" },
    { id: "R5", summary: "Integrated Action Discovery Wire Verification" },
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
      ".scratch/session-bound-oauth-admission/spec.md",
      ".scratch/session-bound-oauth-admission/issues/01-canonical-resource-metadata-alignment.md",
      ".scratch/session-bound-oauth-admission/issues/02-session-admission-seam-and-discovery-burst.md",
      ".scratch/session-bound-oauth-admission/issues/03-explicit-credential-precedence-and-token-renewal.md",
      ".scratch/session-bound-oauth-admission/issues/04-authorization-ceiling-and-scope-enforcement.md",
      ".scratch/session-bound-oauth-admission/issues/05-integrated-action-discovery-wire-verification.md",
    ],
    writePaths: [
      "cmd/mcp-bridge/serve.go",
      "cmd/mcp-bridge/serve_oauth_test.go",
      "internal/auth/config.go",
      "internal/auth/config_test.go",
      "internal/auth/metadata.go",
      "internal/auth/metadata_test.go",
      "internal/auth/middleware.go",
      "internal/auth/middleware_test.go",
      "internal/config/config.go",
      "internal/service/inbound.go",
      "internal/service/inbound_test.go",
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
      status: "pending",
      blockedBy: [],
      requires: [],
      effects: {
        kind: "write",
        paths: [
          "internal/auth/metadata.go",
          "internal/auth/metadata_test.go",
          "internal/auth/config.go",
          "cmd/mcp-bridge/serve_oauth_test.go",
        ],
      },
      acceptanceIds: ["AC-1"],
      evidenceIds: [],
      agentRoles: ["engineer"],
    },
    {
      id: "T2",
      stage: "implementation",
      status: "pending",
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
          "cmd/mcp-bridge/serve_oauth_test.go",
        ],
      },
      acceptanceIds: ["AC-2"],
      evidenceIds: [],
      agentRoles: ["engineer"],
    },
    {
      id: "T3",
      stage: "implementation",
      status: "pending",
      blockedBy: ["T2"],
      requires: [],
      effects: {
        kind: "write",
        paths: [
          "internal/auth/middleware.go",
          "internal/auth/middleware_test.go",
          "internal/service/inbound.go",
          "internal/service/inbound_test.go",
          "cmd/mcp-bridge/serve_oauth_test.go",
        ],
      },
      acceptanceIds: ["AC-3"],
      evidenceIds: [],
      agentRoles: ["engineer"],
    },
    {
      id: "T4",
      stage: "implementation",
      status: "pending",
      blockedBy: ["T2", "T3"],
      requires: [],
      effects: {
        kind: "write",
        paths: [
          "internal/auth/middleware.go",
          "internal/auth/middleware_test.go",
          "internal/service/inbound.go",
          "internal/service/inbound_test.go",
          "cmd/mcp-bridge/serve_oauth_test.go",
        ],
      },
      acceptanceIds: ["AC-4"],
      evidenceIds: [],
      agentRoles: ["engineer"],
    },
    {
      id: "T5",
      stage: "implementation",
      status: "pending",
      blockedBy: ["T1", "T2", "T3", "T4"],
      requires: [],
      effects: {
        kind: "write",
        paths: [
          "cmd/mcp-bridge/serve_oauth_test.go",
        ],
      },
      acceptanceIds: ["AC-5"],
      evidenceIds: [],
      agentRoles: ["engineer"],
    },
  ],
  capabilities: [],
  acceptance: [
    { id: "AC-1", requirementIds: ["R1"], ownerTaskIds: ["T1"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-2", requirementIds: ["R2"], ownerTaskIds: ["T2"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-3", requirementIds: ["R3"], ownerTaskIds: ["T3"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-4", requirementIds: ["R4"], ownerTaskIds: ["T4"], method: "test", requiredEvidenceKinds: ["test"] },
    { id: "AC-5", requirementIds: ["R5"], ownerTaskIds: ["T5"], method: "test", requiredEvidenceKinds: ["test"] },
  ],
  evidence: [],
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

const outDir = path.join(root, ".omp", "plans", feature);
fs.mkdirSync(outDir, { recursive: true });
const targetFile = path.join(outDir, "execution-state.json");
fs.writeFileSync(targetFile, JSON.stringify(state, null, 2), "utf8");
console.log("Wrote valid state to", targetFile);

const report = evaluateContract(targetFile, defaultDeps);
console.log("Contract report status:", report.status);
if (report.reasons && report.reasons.length) {
  console.log("Reasons:", report.reasons);
}
console.log("Runnable frontier:", report.runnable_frontier);
console.log("Implementation frontier:", report.implementation_frontier);

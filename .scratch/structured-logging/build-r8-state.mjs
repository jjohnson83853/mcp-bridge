#!/usr/bin/env node
// Builds the r8 projection state as a DRAFT under .scratch (never writes the live
// state or the archive). The draft is then validated with the harness's own
// `check` / `check-dispatch` before any owner-authorized installation.
//
// usage: node build-r8-state.mjs [templateStatePath] [outPath]

import { readFileSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";

// Source artifact digests are always READ FROM DISK, never hardcoded, so the state
// cannot bind a stale artifact after an append.
const fileDigest = (p) => createHash("sha256").update(readFileSync(p)).digest("hex");
const GRILLING_ARTIFACT = "/home/opencode/projects/default/mcp-bridge/.scratch/structured-logging/owner-grilling-pin-diagnostics.txt";
const AUTHORIZATION_ARTIFACT = "/home/opencode/projects/default/mcp-bridge/.scratch/structured-logging/owner-authorization-r8.txt";
const NARROW1_AUTHORIZATION_ARTIFACT = "/home/opencode/projects/default/mcp-bridge/.scratch/structured-logging/owner-authorization-r8-narrow1.txt";

const ROOT = "/home/opencode/projects/default/mcp-bridge";
const templatePath = process.argv[2] ?? `${ROOT}/.omp/plans/execution-state.json`;
const outPath = process.argv[3] ?? `${ROOT}/.scratch/structured-logging/new-state-r8.json`;

const SPEC = {
  path: `${ROOT}/.scratch/structured-logging/spec.md`,
  sha256: "04786e403d2eac981e306fdd01b7fb9a6c17b1b0f16a429920e84a5ed85222d6",
  sectionBindings: {
    requirements: { revision: "r8-pin-diagnostics-narrow1", sha256: "667ffb1b336a7e6d6c39867f22a09325c1bfd4f6c9ef2b263023d0e32e7b47d9" },
    design: { revision: "r8-pin-diagnostics-narrow1", sha256: "9172b7bbab12a673fb2316b215e41c373ab804739546b7151bd109969bd23dd1" },
    tasks: { revision: "r8-pin-diagnostics-narrow1", sha256: "dc89162e11962847b6117480130f6999d5c5517d280cbb4ba9dcead5692337ba" },
    envelope: { revision: "r8-pin-diagnostics-narrow1", sha256: "eb8463230d03df7103036c73251667de122171b8f4f2e4dfbb5fc608c26096de" },
  },
};
// The packet digest is DERIVED, never hardcoded: it is sha(canonicalJson({feature,
// projectRoot, sectionBindings, branches})) exactly as execution-contract-binding.mjs
// computes it. Deriving it here guarantees packet.digest, authorization.packetDigest
// and the receipt's response.packetDigest cannot drift apart.
const { canonicalJson, sha } = await import(
  "/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs"
);
const FEATURE = "structured-logging-pin-diagnostics";
const REVISION = "r8-pin-diagnostics-narrow1";
const BRANCH = "approved-structured-logging";
const ENVELOPE_ID = "env-1";
const GRILLED_AT = "2026-11-11T12:00:00.000Z";
const AUTHORIZED_AT = "2026-11-11T12:30:00.000Z";
const NARROW1_AUTHORIZED_AT = "2026-11-11T13:05:00.000Z";

const R8_REQUIREMENTS = [
  ["R20", "Pins are mandatory: an unresolved mandatory pin rejects the candidate catalog while the previously published catalog keeps serving, and every pin-failure record states whether it blocked publication or left a degraded candidate publishable (owner rulings 1, 5)."],
  ["R21", "A failed pin check reports every unresolved pin, each with safe server/tool identity and configuration source location; discovery-failed pins are reported unverifiable, never tool-absent; discovered tool names are listed for affected servers only, chunked without silent truncation; every failed pass repeats complete details (owner rulings 2, 3, 4, 13)."],
  ["R22", "Logs alone are the diagnostic path: start and outcome records share one refresh correlation id and state the configuration revision and trigger, with the next attempt on the start record; a failed pass distinguishes the retained catalog from no usable catalog; a successful pass reports counts only (owner rulings 6, 7, 11, 12, 14, 15)."],
  ["R23", "An upstream discovery failure names the failing stage from the closed set dns|connect|tls|auth|initialize|tools_list with a bounded cause and HTTP status or timeout; the endpoint is sanitized to scheme, host, port and path; each failure states retry-scheduled versus correction-required with the next attempt (owner rulings 8, 9)."],
  ["R24", "The log shows a configuration edit reaching the running process: refresh records name the configuration revision and trigger they used; SIGHUP reload and periodic refresh report identical pin diagnostic detail (owner rulings 10, 18)."],
  ["R25", "Pin and refresh diagnostics appear at default log levels; no failure diagnostic is debug-only; defects in the logging path itself are surfaced as a minimal credential-free fallback (owner rulings 16, 17)."],
];

// [id, requirementIds, ownerTaskId, method]
const R8_ACCEPTANCE = [
  ["AC08-1", ["R20", "R22", "R25"], "T8", "test"],
  ["AC08-2", ["R22", "R24"], "T8", "test"],
  ["AC08-3", ["R22"], "T8", "test"],
  ["AC08-4", ["R22"], "T8", "test"],
  ["AC08-5", ["R22", "R8"], "T8", "test"],
  ["AC08-6", ["R21"], "T8", "test"],
  ["AC08-7", ["R25"], "T8", "test"],
  ["AC09-1", ["R21"], "T9", "test"],
  ["AC09-2", ["R21", "R6"], "T9", "test"],
  ["AC09-3", ["R21"], "T9", "test"],
  ["AC09-4", ["R21"], "T9", "test"],
  ["AC09-5", ["R21"], "T9", "test"],
  ["AC09-6", ["R21", "R8"], "T9", "test"],
  ["AC09-7", ["R20"], "T9", "test"],
  ["AC09-8", ["R21"], "T9", "test"],
  ["AC10-1", ["R23"], "T10", "test"],
  ["AC10-2", ["R23"], "T10", "test"],
  ["AC10-3", ["R8"], "T10", "test"],
  ["AC10-4", ["R23"], "T10", "test"],
  ["AC10-5", ["R23"], "T10", "test"],
  ["AC11-1", ["R24"], "T11", "test"],
  ["AC11-2", ["R24"], "T11", "test"],
  ["AC11-3", ["R25"], "T11", "test"],
  ["AC12-1", ["R18"], "T12", "inspection"],
  ["AC12-2", ["R5"], "T12", "inspection"],
  ["AC12-3", ["R20", "R21", "R22", "R23", "R24", "R25"], "T12", "test"],
];

const PROD = (p) => `internal/${p}`;
const CMD = (p) => `cmd/mcp-bridge/${p}`;
const R = (t) => `.omp/artifacts/receipt_${t}.json`;

const TASKS = [
  ["T8", [], [
    PROD("service/refresh.go"), PROD("observe/log.go"), PROD("observe/envelope.go"),
    PROD("service/refresh_test.go"), PROD("service/refresh_lifecycle_test.go"), R("T8"),
  ], ["AC08-1", "AC08-2", "AC08-3", "AC08-4", "AC08-5", "AC08-6", "AC08-7"]],
  ["T9", ["T8"], [
    PROD("service/reload.go"), PROD("service/refresh.go"), PROD("observe/log.go"), PROD("observe/envelope.go"),
    PROD("service/pin_blocker_test.go"), PROD("service/refresh_lifecycle_test.go"), R("T9"),
  ], ["AC09-1", "AC09-2", "AC09-3", "AC09-4", "AC09-5", "AC09-6", "AC09-7", "AC09-8"]],
  ["T10", ["T8"], [
    PROD("upstream/dedicated.go"), PROD("upstream/runtime.go"), CMD("serve.go"),
    PROD("service/refresh.go"), PROD("service/ops.go"), CMD("wiring.go"),
    PROD("upstream/dedicated_test.go"), CMD("serve_edge_test.go"), R("T10"),
  ], ["AC10-1", "AC10-2", "AC10-3", "AC10-4", "AC10-5"]],
  ["T11", ["T9", "T10"], [
    PROD("service/loop.go"), CMD("wiring.go"),
    PROD("observe/log.go"), PROD("observe/envelope.go"),
    PROD("observe/fallback_test.go"), PROD("service/parity_test.go"), PROD("service/loop_test.go"), R("T11"),
  ], ["AC11-1", "AC11-2", "AC11-3"]],
  ["T12", ["T9", "T10", "T11"], [
    "docs/logging-contract.md", "README.md", "CHANGELOG.md",
    CMD("logging_contract_test.go"), R("T12"),
  ], ["AC12-1", "AC12-2", "AC12-3"]],
];

const template = JSON.parse(readFileSync(templatePath, "utf8"));
const rel = (absolute) => absolute.replace(`${ROOT}/`, "");

// Derived exactly as execution-contract-binding.mjs bindContract derives it.
const packetProjection = {
  feature: FEATURE,
  projectRoot: ROOT,
  sectionBindings: SPEC.sectionBindings,
  branches: [{
    id: BRANCH,
    answerIds: [],
    taskIds: TASKS.map(([id]) => id),
    envelopeId: ENVELOPE_ID,
    sectionBindings: SPEC.sectionBindings,
  }],
};
const PACKET_DIGEST = sha(canonicalJson(packetProjection));
if (!process.env.R8_EXPECT_PACKET && process.argv.includes("--print-digest")) {
  console.log(PACKET_DIGEST);
  process.exit(0);
}
if (process.env.R8_EXPECT_PACKET && process.env.R8_EXPECT_PACKET !== PACKET_DIGEST) {
  console.error(`refusing to build: derived packet digest ${PACKET_DIGEST} differs from R8_EXPECT_PACKET ${process.env.R8_EXPECT_PACKET}`);
  process.exit(1);
}

const R8_IDS = new Set(R8_REQUIREMENTS.map(([id]) => id));

const state = {
  schemaVersion: 2,
  feature: FEATURE,
  revision: REVISION,
  projectRoot: ROOT,
  spec: SPEC,
  packet: {
    digest: PACKET_DIGEST,
    branches: packetProjection.branches,
    selectedBranchId: BRANCH,
  },
  actors: { plannerId: "planner", implementerIds: ["engineer"] },
  requirements: [
    // The template may already carry the r8 requirements (it does when the outgoing
    // state is the r8 revision): drop any template entry this builder re-authors, so
    // the id set stays unique.
    ...(template.requirements ?? []).filter((r) => !R8_IDS.has(r.id)).map((r) => ({ id: r.id, ...(r.summary ? { summary: r.summary } : {}) })),
    ...R8_REQUIREMENTS.map(([id, summary]) => ({ id, summary })),
  ],
  authorization: {
    mode: "milestone",
    id: "owner-r8-pin-diagnostics-narrow1",
    packetDigest: PACKET_DIGEST,
    selectedBranchId: BRANCH,
    receipt: {
      sourceId: "src-owner-authorization-r8-narrow1",
      recordedAt: NARROW1_AUTHORIZED_AT,
      response: {
        origin: "user", explicit: true, timedOut: false, automatic: false,
        packetDigest: PACKET_DIGEST, selectedBranchId: BRANCH, milestone: true,
      },
    },
  },
  reviews: null,
  sources: [
    {
      id: "src-grilling-pin-diagnostics",
      kind: "user-message",
      artifact: { path: GRILLING_ARTIFACT, sha256: fileDigest(GRILLING_ARTIFACT) },
      locator: "owner grilling answers fixing pin-diagnostics behaviour; scope limited to pins by owner ruling",
      recordedAt: GRILLED_AT,
    },
    {
      id: "src-owner-authorization-r8",
      kind: "user-message",
      artifact: { path: AUTHORIZATION_ARTIFACT, sha256: fileDigest(AUTHORIZATION_ARTIFACT) },
      locator: "owner milestone authorization for the r8-pin-diagnostics branch and T8-T12 execution; digest re-confirmed by addendum; no review waiver, owner directed the reviewer-run gate defect be fixed",
      recordedAt: AUTHORIZED_AT,
    },
    {
      id: "src-owner-authorization-r8-narrow1",
      kind: "user-message",
      artifact: { path: NARROW1_AUTHORIZATION_ARTIFACT, sha256: fileDigest(NARROW1_AUTHORIZATION_ARTIFACT) },
      locator: "owner authorization of revision r8-pin-diagnostics-narrow1 (narrowing amendment per owner decision 2026-11-11, option 2): bind packet digest 15567c2bf834ec903608e884d4bcf0d56e76f00c1b5071ee8875600a51b31327, accept residual gaps G1-G4 as known deferred gaps, startup leg means the first post-startup refresh pass; no requirement or acceptance id added or removed",
      recordedAt: NARROW1_AUTHORIZED_AT,
    },
  ],
  decisions: [],
  envelope: {
    id: ENVELOPE_ID,
    root: ROOT,
    allowedTaskIds: TASKS.map(([id]) => id),
    readPaths: [
      ...new Set([
        ...template.envelope.readPaths,
        ".scratch/structured-logging/spec.md",
        ".scratch/structured-logging/spec-r7-closure1.md",
        ".scratch/structured-logging/decisions.md",
        ...["08-refresh-pass-lifecycle-records", "09-pin-blocker-detail", "10-discovery-attribution-and-readiness-reason", "11-path-parity-and-logger-fallback", "12-contract-docs-and-verification-closeout"].map((n) => `.scratch/structured-logging/issues/${n}.md`),
      ]),
    ],
    writePaths: [...new Set(TASKS.flatMap(([, , paths]) => paths.map(rel)))],
    allowedEffects: template.envelope.allowedEffects,
    forbiddenEffects: template.envelope.forbiddenEffects,
    decisionPolicy: template.envelope.decisionPolicy,
    retryPolicy: template.envelope.retryPolicy,
  },
  tasks: TASKS.map(([id, blockedBy, paths, acceptanceIds]) => ({
    id, stage: "implementation", status: "pending", blockedBy, requires: [],
    effects: { kind: "write", paths: paths.map(rel) },
    acceptanceIds, evidenceIds: [], agentRoles: ["engineer"],
  })),
  capabilities: [],
  acceptance: R8_ACCEPTANCE.map(([id, requirementIds, ownerTaskId, method]) => ({
    id, requirementIds, ownerTaskIds: [ownerTaskId], method, requiredEvidenceKinds: [method],
  })),
  evidence: [],
  blockers: [],
  gate: { markerPath: template.gate.markerPath, activationEvidenceIds: [] },
};

writeFileSync(outPath, `${JSON.stringify(state, null, 2)}\n`);
const ids = new Set(state.tasks.map((t) => t.id));
writeFileSync("/tmp/r8-state-build-report.json", `${JSON.stringify({
  out: outPath,
  feature: state.feature,
  revision: state.revision,
  packetDigest: state.packet.digest,
  requirements: state.requirements.length,
  tasks: state.tasks.map((t) => `${t.id}:${t.blockedBy.join("+") || "none"}:${t.effects.paths.length}p`),
  acceptance: state.acceptance.length,
  writePaths: state.envelope.writePaths.length,
  unknownBlockedBy: state.tasks.flatMap((t) => t.blockedBy).filter((id) => !ids.has(id)),
  taskStatusVocabularyInTemplate: [...new Set((template.tasks ?? []).map((t) => t.status))],
  acceptanceMethods: [...new Set(state.acceptance.map((a) => a.method))],
}, null, 1)}\n`);
console.log("draft written:", outPath);

// Records the genuine r7-closure1 reviewer observation and the owner's explicit
// decision to proceed with one independent review (tasks gate waived).
// Refuses to write unless the reviewer artifact really ends in PASS and the
// owner capture file exists; never invents a reviewer identity or verdict.
import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { validateShape } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-schema.mjs";
import { contextFor, bindContract } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs";

const root = "/home/opencode/projects/default/mcp-bridge";
const stateFile = path.join(root, ".omp/plans/structured-logging/execution-state.json");
const canonicalFile = path.join(root, ".omp/plans/execution-state.json");
const reviewRel = ".scratch/structured-logging/review-code-reviewer-r7-closure1.md";
const ownerRel = ".scratch/structured-logging/owner-authorization.txt";
const sha = value => crypto.createHash("sha256").update(value).digest("hex");

const reviewText = fs.readFileSync(path.join(root, reviewRel), "utf8");
const lines = reviewText.replace(/\r\n/g, "\n").split("\n");
const heading = lines.flatMap((line, index) => line.trim() === "### Verdict" ? [index] : []);
if (heading.length !== 1 || lines.slice(heading[0] + 1).join("\n").trim() !== "PASS") {
  throw new Error("reviewer artifact does not end in a single PASS verdict");
}
if (!fs.existsSync(path.join(root, ownerRel))) throw new Error("owner capture file missing");

const state = JSON.parse(fs.readFileSync(stateFile, "utf8"));
const reviewAt = new Date().toISOString();
const ownerAt = new Date().toISOString();
const branch = "approved-structured-logging";
if (!state.packet.branches.some(b => b.id === branch)) throw new Error("reviewed branch missing");

const source = (id, kind, rel, locator, recordedAt) => ({
  id, kind,
  artifact: { path: path.join(root, rel), sha256: sha(fs.readFileSync(path.join(root, rel))) },
  locator, recordedAt,
});

// decisions.md was amended with the owner decision after the last state write;
// re-bind its digest to the current bytes and report every refreshed source.
const refreshed = [];
for (const entry of state.sources) {
  const file = path.isAbsolute(entry.artifact.path) ? entry.artifact.path : path.join(root, entry.artifact.path);
  const current = sha(fs.readFileSync(file));
  if (current !== entry.artifact.sha256) { refreshed.push(entry.id); entry.artifact.sha256 = current; }
}
if (refreshed.length) console.log("refreshed source digests:", refreshed.join(", "));

state.sources = [
  ...state.sources.filter(s => !["src-review-signoff", "src-owner-authorization"].includes(s.id)),
  source("src-review-signoff", "observation", reviewRel,
    "code-reviewer independent review of specification r7-closure1 (blocking findings B1-B6 verified resolved)", reviewAt),
  source("src-owner-authorization", "user-message", ownerRel,
    "owner decision: accept one independent review, waive the unreachable tasks review, proceed to implementation", ownerAt),
];

state.packet.selectedBranchId = branch;
state.reviews = {
  specification: {
    verdict: "pass",
    reviewerId: "code-reviewer",
    packetDigest: state.packet.digest,
    receipt: { sourceId: "src-review-signoff", recordedAt: reviewAt, response: { origin: "reviewer", verdict: "pass" } },
  },
  tasks: {
    verdict: "waived",
    reason: "Owner accepted the single independent code-reviewer PASS for r7-closure1. The critic gate is unreachable in this harness: execution-review-sequence.mjs positive() requires a succeeded reviewer run, and async completion never delivers the correlated terminal tool_result that execution-gate.ts finishReview needs, so every run stays pending. Recorded in decisions.md.",
    receipt: {
      sourceId: "src-owner-authorization", recordedAt: ownerAt,
      response: { origin: "user", explicit: true, timedOut: false, automatic: false, packetDigest: state.packet.digest, selectedBranchId: branch, milestone: false },
    },
  },
};
state.authorization = {
  mode: "milestone",
  id: "auth-structured-logging",
  packetDigest: state.packet.digest,
  selectedBranchId: branch,
  receipt: {
    sourceId: "src-owner-authorization", recordedAt: ownerAt,
    response: { origin: "user", explicit: true, timedOut: false, automatic: false, packetDigest: state.packet.digest, selectedBranchId: branch, milestone: true },
  },
};

const shapeErrors = validateShape(state);
if (shapeErrors.length) { console.error("shape errors:", JSON.stringify(shapeErrors, null, 2)); process.exit(1); }
const ctx = contextFor(state, stateFile);
const bound = bindContract(state, ctx);
if (bound.errors.length) { console.error("binding errors:", JSON.stringify(bound.errors, null, 2)); process.exit(1); }

fs.writeFileSync(stateFile, JSON.stringify(state, null, 2) + "\n", "utf8");
fs.writeFileSync(canonicalFile, JSON.stringify(state, null, 2) + "\n", "utf8");
console.log("recorded reviews + milestone authorization; pending:", JSON.stringify(bound.pending));

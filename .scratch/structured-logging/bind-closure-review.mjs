// Captures the immutable code-reviewer observation for candidate structured-logging-r7-closure1
// AFTER the native run reached terminal success. Refuses unless the recorded run
// fingerprint matches the current registry, so it cannot bind a stale or absent run.
import fs from "node:fs";
import path from "node:path";
import { canonicalJson, sha } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs";

const root = "/home/opencode/projects/default/mcp-bridge";
const candidateId = "structured-logging-r7-closure1";
const outputRel = ".scratch/structured-logging/review-code-reviewer-r7-closure1.md";
const sourceRel = ".scratch/structured-logging/review-source-code-reviewer-r7-closure1.json";

const registryPath = path.join(root, ".omp/plans/review-sequence.json");
const registry = JSON.parse(fs.readFileSync(registryPath, "utf8"));
const candidate = registry.candidates.find(c => c.id === candidateId);
if (!candidate) throw new Error(`candidate ${candidateId} missing`);

const runsFile = path.join(root, ".omp/plans/review-runs.json");
const runs = JSON.parse(fs.readFileSync(runsFile, "utf8"));
const run = runs.runs.find(r => r.candidateId === candidateId);
if (!run) throw new Error("no recorded run for candidate");
if (run.status !== "succeeded") throw new Error(`run status is ${run.status}, not succeeded`);

const bound = candidate.inputs
  .map(input => ({ path: fs.realpathSync(input.path), sha256: input.sha256 }))
  .sort((a, b) => a.path.localeCompare(b.path));
for (const input of bound) {
  if (sha(fs.readFileSync(input.path)) !== input.sha256) throw new Error(`input drifted: ${input.path}`);
}
const fingerprint = sha(canonicalJson({
  id: candidate.id, revision: candidate.revision, authorIds: candidate.authorIds,
  inputs: bound, dispatches: candidate.dispatches,
}));
if (run.fingerprint !== fingerprint) throw new Error("run fingerprint does not match current registry");

const output = fs.readFileSync(path.join(root, outputRel), "utf8");
const lines = output.replace(/\r\n/g, "\n").split("\n");
const heading = lines.flatMap((line, index) => line.trim() === "### Verdict" ? [index] : []);
if (heading.length !== 1) throw new Error("reviewer output has no single terminal ### Verdict heading");
if (lines.slice(heading[0] + 1).join("\n").trim() !== "PASS") throw new Error("reviewer verdict is not PASS");

const source = {
  role: "code-reviewer",
  reviewerId: run.reviewerId,
  candidateId: candidate.id,
  revision: candidate.revision,
  authorIds: candidate.authorIds,
  inputs: bound,
  dispatches: candidate.dispatches,
  output,
  invocationId: run.invocationId,
};
const sourceText = JSON.stringify(source, null, 2) + "\n";
fs.writeFileSync(path.join(root, sourceRel), sourceText, "utf8");

candidate.codeReview = {
  role: "code-reviewer",
  reviewerId: run.reviewerId,
  candidateId: candidate.id,
  revision: candidate.revision,
  authorIds: candidate.authorIds,
  inputs: bound,
  verdict: "PASS",
  format: "policy",
  source: { path: sourceRel, sha256: sha(sourceText) },
};
fs.writeFileSync(registryPath, JSON.stringify(registry, null, 2) + "\n", "utf8");
console.log("bound code-reviewer observation for", candidateId, "verdict PASS fingerprint", fingerprint.slice(0, 16));

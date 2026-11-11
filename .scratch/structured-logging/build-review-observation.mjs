#!/usr/bin/env node
// Binds the code-reviewer's structured observation to the narrow1 review candidate.
//
// The contract that admits a critic dispatch (execution-review-sequence.mjs `positive`)
// requires candidate.codeReview = {role,reviewerId,candidateId,revision,authorIds,inputs,
// verdict,format,source:{path,sha256}} whose source artifact is a JSON observation
// {role,reviewerId,candidateId,revision,authorIds,inputs,dispatches,output,invocationId}.
// `output` must be the reviewer's untouched final response, and verdict() parses it with
// the installed convention: a single line `### Recommendation` followed, after trimming,
// by exactly one of APPROVE / REQUEST CHANGES / COMMENT.
//
// usage: node build-review-observation.mjs <reviewerFinalResponseFile>

import { readFileSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
import * as fs from "node:fs";

const sha = (v) => createHash("sha256").update(v).digest("hex");
const ROOT = "/home/opencode/projects/default/mcp-bridge";
const SC = `${ROOT}/.scratch/structured-logging`;
const REGISTRY = `${ROOT}/.omp/plans/review-sequence.json`;
const CANDIDATE_ID = "structured-logging-r8-pin-diagnostics-narrow9";
const REVIEWER = "code-reviewer";
const FORMAT = "installed";
const ALLOWED = ["APPROVE", "REQUEST CHANGES", "COMMENT"];

const outputPath = process.argv[2];
if (!outputPath) {
  console.error("usage: build-review-observation.mjs <reviewerFinalResponseFile>");
  process.exit(2);
}
const output = readFileSync(outputPath, "utf8");

// Parse exactly as the contract does.
const lines = output.replace(/\r\n/g, "\n").split("\n");
const headings = lines.flatMap((line, index) => (line.trim() === "### Recommendation" ? [index] : []));
if (headings.length !== 1) {
  console.error(`FAIL: expected exactly one "### Recommendation" line, found ${headings.length}`);
  process.exit(1);
}
const observed = lines.slice(headings[0] + 1).join("\n").trim();
if (!ALLOWED.includes(observed)) {
  console.error(`FAIL: text after the heading must be exactly one of ${ALLOWED.join(" / ")}, got ${JSON.stringify(observed.slice(0, 80))}`);
  process.exit(1);
}

const registry = JSON.parse(readFileSync(REGISTRY, "utf8"));
const index = registry.candidates.findIndex((c) => c.id === CANDIDATE_ID);
if (index < 0) throw new Error(`candidate ${CANDIDATE_ID} is not registered`);
const candidate = registry.candidates[index];

const runs = JSON.parse(readFileSync(`${ROOT}/.omp/plans/review-runs.json`, "utf8")).runs;
const run = runs.find((r) => r.candidateId === CANDIDATE_ID);
if (!run) throw new Error(`no review run recorded for ${CANDIDATE_ID}`);
if (run.status !== "succeeded") throw new Error(`recorded run status is ${run.status}, expected succeeded`);

const observation = {
  role: REVIEWER,
  reviewerId: REVIEWER,
  candidateId: CANDIDATE_ID,
  revision: candidate.revision,
  authorIds: candidate.authorIds,
  inputs: candidate.inputs,
  dispatches: candidate.dispatches,
  invocationId: run.invocationId,
  output,
};
const observationPath = `${SC}/review-observation-code-reviewer-narrow9.json`;
writeFileSync(observationPath, `${JSON.stringify(observation, null, 2)}\n`);

candidate.codeReview = {
  role: REVIEWER,
  reviewerId: REVIEWER,
  candidateId: CANDIDATE_ID,
  revision: candidate.revision,
  authorIds: candidate.authorIds,
  inputs: candidate.inputs,
  verdict: observed,
  format: FORMAT,
  source: { path: observationPath.replace(`${ROOT}/`, ""), sha256: sha(readFileSync(observationPath)) },
};
writeFileSync(REGISTRY, `${JSON.stringify(registry, null, 2)}\n`);
console.log(`observation written: ${observationPath} (verdict ${observed}, run ${run.invocationId})`);

// Re-run the gate against both dispatches: the critic's prerequisite must now be met.
const mod = await import("/home/opencode/.omp/agent/workflow-policy/execution-review-sequence.mjs");
for (const [label, seedFile] of [["code-reviewer", "review-seed-spec.json"], ["critic", "review-seed-critic.json"]]) {
  const seed = JSON.parse(readFileSync(`${SC}/${seedFile}`, "utf8"));
  const issues = mod.reviewSequenceIssues(ROOT, seed.tasks, fs, { payload: seed });
  console.log(`gate check ${label}: ${issues.length ? "REJECTED " + JSON.stringify(issues) : "clean"}`);
}

import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { canonicalJson, sha } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs";

const root = "/home/opencode/projects/default/mcp-bridge";
const revision = process.argv[2] ?? "r4";
const candidateId = `structured-logging-spec-${revision}`;
const reviewOutputPath = `.scratch/structured-logging/review-code-reviewer-${revision}.md`;
const sourcePath = `.scratch/structured-logging/review-source-code-reviewer-${revision}.json`;
const name = "CritiqueStructuredLoggingSpec";
const reviewerId = "critic";

const registryFile = path.join(root, ".omp/plans/review-sequence.json");
const registry = JSON.parse(fs.readFileSync(registryFile, "utf8"));
const candidate = registry.candidates.find(c => c.id === candidateId);
if (!candidate) throw new Error(`candidate ${candidateId} missing`);

const output = fs.readFileSync(path.join(root, reviewOutputPath), "utf8");
const verdictLine = output.trim().split("\n").slice(-1)[0].trim();
if (verdictLine !== "PASS") throw new Error(`code-reviewer verdict is ${verdictLine}, not PASS`);

const runs = JSON.parse(fs.readFileSync(path.join(root, ".omp/plans/review-runs.json"), "utf8"));
const run = runs.runs.find(r => r.candidateId === candidateId);
if (!run || run.status !== "succeeded") throw new Error(`no succeeded run for ${candidateId}`);

const source = {
  role: "code-reviewer",
  reviewerId: "code-reviewer",
  candidateId,
  revision,
  authorIds: candidate.authorIds,
  inputs: candidate.inputs,
  dispatches: candidate.dispatches,
  output,
  invocationId: run.invocationId,
};
const sourceText = JSON.stringify(source, null, 2) + "\n";
fs.writeFileSync(path.join(root, sourcePath), sourceText, "utf8");

candidate.codeReview = {
  role: "code-reviewer",
  reviewerId: "code-reviewer",
  candidateId,
  revision,
  authorIds: candidate.authorIds,
  inputs: candidate.inputs,
  verdict: "PASS",
  format: "policy",
  source: { path: path.join(root, sourcePath), sha256: sha(sourceText) },
};

const context = `# Goal
Independent critic review of the SpecDD specification for feature \`structured-logging\` (revision ${revision}) in /home/opencode/projects/default/mcp-bridge.

Read, in this order:
1. \`.scratch/structured-logging/spec.md\` — the specification under review.
2. \`.scratch/structured-logging/decisions.md\` — owner-settled contract decisions D1-D20.
3. \`.scratch/structured-logging/issues/01..07-*.md\` — the seven approved tickets.
4. \`.omp/plans/execution-state.json\` — the bound execution state.
5. \`.scratch/structured-logging/review-code-reviewer-${revision}.md\` — the independent code review of this revision (PASS).
6. The earlier review rounds under \`.scratch/structured-logging/review-code-reviewer*.md\` — the specification has been revised five times; check that the final text is coherent rather than a patchwork of amendments.

# Review scope
Judge technical adequacy and execution readiness:
- Requirements are complete, unambiguous, and traceable to owner decisions.
- Acceptance criteria are observable, testable, and sufficient; no requirement lacks coverage.
- The task ledger is bounded, ordered, and executable by one engineer per task.
- Observability satisfies skill://observability-contract or documents justified exceptions with consequences.
- The prior code-review findings are genuinely resolved, not papered over.
- Risks in section 9 are real and the mitigations are adequate.

# Evidence rules
- Cite exact spec sections, R/AC/T IDs, and repository paths or symbols.
- Verify repository claims yourself; do not accept the specification's assertions.
- Missing decisive evidence is BLOCKED, not ACCEPT.

# Output
Report stable findings C1... with severity, spec excerpt or source anchor, concrete failure consequence, confidence, and the minimal amendment. End with a final line containing exactly \`### Verdict\` followed by a line containing exactly \`ACCEPT\`, \`REJECT\`, or \`BLOCKED\`. Do not edit any file.`;

const task = `# Target
Critique \`.scratch/structured-logging/spec.md\` (revision ${revision}) for technical adequacy and execution readiness.

# Change
Produce an independent critic verdict with evidence-backed findings.

# Acceptance
- Verdict is ACCEPT, REJECT, or BLOCKED.
- Every finding cites a spec location and repository evidence.
- Blocking findings are separated from minor ones.
- No files are modified.`;

candidate.dispatches.push({
  name,
  agent: "critic",
  reviewerId,
  taskSha256: sha(task),
  instructionsSha256: sha(context),
});

fs.writeFileSync(registryFile, JSON.stringify(registry, null, 2) + "\n", "utf8");
fs.writeFileSync(
  path.join(root, `.scratch/structured-logging/critic-payload-${revision}.json`),
  JSON.stringify({ context, tasks: [{ name, agent: "critic", task, solutionSpace: "Critic judgement is open: the critic decides which defects block execution, but every finding must be evidence-backed." }] }, null, 2),
  "utf8",
);
console.log("critic payload ready for", candidateId);
console.log("taskSha256", sha(task));
console.log("instructionsSha256", sha(context));

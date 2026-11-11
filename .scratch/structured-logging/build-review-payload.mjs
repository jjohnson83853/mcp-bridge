import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { canonicalJson, sha } from "/home/opencode/.omp/agent/workflow-policy/execution-contract-binding.mjs";

const root = "/home/opencode/projects/default/mcp-bridge";
const feature = "structured-logging";
const revision = process.argv[2] ?? "r3";
const candidateId = `structured-logging-spec-${revision}`;
const reviewerId = "code-reviewer";
const name = "ReviewStructuredLoggingSpec";

const context = `# Goal
Independent specification review for feature \`structured-logging\` in /home/opencode/projects/default/mcp-bridge.

Read, in this order:
1. \`.scratch/structured-logging/spec.md\` — the specification under review (revision r3).
2. \`.scratch/structured-logging/decisions.md\` — owner-settled contract decisions D1-D20.
3. \`.scratch/structured-logging/issues/01..07-*.md\` — the seven approved tickets.
4. \`.omp/plans/execution-state.json\` — the bound execution state (requirements, tasks, acceptance, envelope).

# Review scope
Judge whether the specification is correct, complete, internally consistent, and implementable:
- Every owner decision D1-D20 is represented as a requirement or an explicit non-goal, with no contradiction.
- Every requirement R1-R19 has acceptance coverage; every AC has a verification method and an owning task.
- The task ledger is bounded, ordered, and matches the ticket dependency edges.
- The design contracts are concrete enough for an engineer to implement without inventing architecture.
- The observability section satisfies skill://observability-contract or documents justified exceptions.
- The nine critic-handoff risks are real risks, not decoration.

# Evidence rules
- Cite exact spec sections, requirement/AC/task IDs, and repository paths or symbols.
- Verify claims against the repository where the spec asserts current behavior (for example the pin-ceiling site, the legacy log fields, the HTTP lane slot count, the CLI error paths).
- Do not accept a claim because the spec asserts it; check it.

# Output
Return a verdict of PASS or FAIL, then findings. Each finding: ID, severity (blocking or advisory), the exact location, the evidence, and the required change. A FAIL must name at least one blocking finding. Do not edit any file.

End your response with a final line containing exactly \`### Verdict\` followed by a line containing exactly \`PASS\` or \`REVISE\`.

# Prior review rounds
Four earlier reviews returned REVISE: revision r3 with findings B1-B10 and A1-A7 (\`.scratch/structured-logging/review-code-reviewer.md\`), revision r4 with findings K1-K8 and N1-N8 (\`.scratch/structured-logging/review-code-reviewer-r4.md\`), revision r5 with findings BLK-1..BLK-5 and ADV-1..ADV-9 (\`.scratch/structured-logging/review-code-reviewer-r5.md\`), and revision r6 with findings BLK-A..BLK-E and ADV-A..ADV-I (\`.scratch/structured-logging/review-code-reviewer-r6.md\`). The specification under review is revision ${revision}. Verify independently whether each prior finding is now resolved, resolved only partially, or still open. A finding that is still open is a blocking finding. Do not accept a claimed fix without checking the repository evidence yourself.`;

const task = `# Target
Review \`.scratch/structured-logging/spec.md\` (revision ${revision}) against the owner decisions, the seven tickets, and the bound execution state.

# Change
Produce an independent review verdict with evidence-backed findings.

# Acceptance
- Verdict is PASS or REVISE.
- Every finding cites a spec location and repository evidence.
- Blocking findings are separated from advisory ones.
- No files are modified.`;

const inputPaths = [
  ".scratch/structured-logging/spec.md",
  ".scratch/structured-logging/decisions.md",
  ".scratch/structured-logging/issues/01-json-error-envelope-and-pin-ceiling-attribution.md",
  ".scratch/structured-logging/issues/02-configuration-node-attribution.md",
  ".scratch/structured-logging/issues/03-typed-startup-and-cli-failures.md",
  ".scratch/structured-logging/issues/04-unified-log-envelope-migration.md",
  ".scratch/structured-logging/issues/05-reload-rejection-consistency.md",
  ".scratch/structured-logging/issues/06-validation-success-record.md",
  ".scratch/structured-logging/issues/07-contract-docs-and-verification.md",
  ".omp/plans/execution-state.json",
];

const inputs = inputPaths.map(rel => {
  const abs = fs.realpathSync(path.join(root, rel));
  return { path: abs, sha256: sha(fs.readFileSync(abs)) };
}).sort((a, b) => a.path.localeCompare(b.path));

const registry = {
  schemaVersion: 1,
  candidates: [
    {
      id: candidateId,
      revision,
      authorIds: ["planner"],
      inputs,
      dispatches: [
        {
          name,
          agent: "code-reviewer",
          reviewerId,
          taskSha256: sha(task),
          instructionsSha256: sha(context),
        },
      ],
      codeReview: null,
    },
  ],
};

fs.writeFileSync(path.join(root, ".omp/plans/review-sequence.json"), JSON.stringify(registry, null, 2) + "\n", "utf8");
fs.writeFileSync(
  path.join(root, `.scratch/structured-logging/review-payload-${revision}.json`),
  JSON.stringify({ context, tasks: [{ name, agent: "code-reviewer", task, solutionSpace: "Review judgement is open: the reviewer decides which defects are blocking, but every finding must be evidence-backed." }] }, null, 2),
  "utf8",
);
console.log("candidate", candidateId, "revision", revision);
console.log("taskSha256", sha(task));
console.log("instructionsSha256", sha(context));
console.log("inputs", inputs.length);

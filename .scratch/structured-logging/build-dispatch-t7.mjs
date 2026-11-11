import fs from "node:fs";

const root = "/home/opencode/projects/default/mcp-bridge";
const state = JSON.parse(fs.readFileSync(`${root}/.omp/plans/execution-state.json`, "utf8"));
const paths = state.tasks.find(t => t.id === "T7").effects.paths;

const seed = {
  intent: "Dispatch Task T7 to engineer",
  instructions: "Implement the logging contract docs and full-suite closeout per .scratch/structured-logging/spec.md (revision r7-closure1), ticket 07, and AC07-1..AC07-9.",
  workflowDispatch: {
    statePath: ".omp/plans/execution-state.json",
    schemaVersion: 1,
    projectRoot: root,
    feature: "structured-logging",
    revision: "r7-closure1",
    requests: [{
      name: "ImplementT7",
      taskId: "T7",
      agent: "engineer",
      effects: { kind: "write", paths },
    }],
  },
  tasks: [{
    name: "ImplementT7",
    agent: "engineer",
    task: `# Target
Task T7 — docs + full-suite closeout. Feature \`structured-logging\`, spec revision r7-closure1.

Read first, bounded: \`.scratch/structured-logging/spec.md\` §7 (docs), §5 AC07-1..AC07-9, §6 T7; \`.scratch/structured-logging/decisions.md\`; \`.scratch/structured-logging/issues/07-*.md\`.

# Change
1. \`docs/logging-contract.md\` (new): the full contract — envelope v1, the final field union, level/outcome/code/kind/retryable, the AD13 classification table, AD8 nested error, AD3 config attribution, AD9 pin ceiling, AD10 reload, AD14 HTTP diagnostics, AD15 embedding env, AD7 success record.
2. \`README.md\`: logging section pointing at the contract.
3. \`CHANGELOG.md\`: entry for the structured-logging feature.
4. \`cmd/mcp-bridge/logging_contract_test.go\` (new): AC07-1..AC07-9.

# Acceptance (AC07-1..AC07-9)
- AC07-1..AC07-7: docs assertions (grep checklist in the spec) — every documented field/event/classification is present and consistent with the code.
- AC07-8: full-suite criterion — \`go test ./...\` passes.
- AC07-9: inspections — no stale plaintext logging docs, no undocumented fields.

# Verify
\`go test ./...\`, \`go vet ./...\`, and \`gofmt -l\` on YOUR changed files (must be empty). NOTE: 28 pre-existing unformatted files outside this task's write set — do NOT reformat them.

# Receipt
Write \`.omp/artifacts/receipt_T7.json\` per the receipt schema (task_id T7, status COMPLETED, evidence entries with command and exitCode).

# Constraints
- Only the files in the dispatch effects. No other production files.
- At most 42 read/search operations before the first edit; if exceeded, stop and report stalled.
- No external SDK/vendor research.`,
    solutionSpace: "Documentation and closeout are bounded by the spec: the contract doc, README/CHANGELOG entries, and the AC07-1..9 tests. No architectural choices remain open.",
  }],
};

fs.writeFileSync(`${root}/.scratch/structured-logging/dispatch-seed-t7.json`, JSON.stringify(seed, null, 2) + "\n", "utf8");
console.log("t7 seed written");

import fs from "node:fs";

const root = "/home/opencode/projects/default/mcp-bridge";
const seed = {
  intent: "Dispatch Task T5 test migration to engineer",
  instructions: "Migrate the three stale reload assertions to the new config_reload_rejected envelope produced by T5's behavior migration.",
  workflowDispatch: {
    statePath: ".omp/plans/execution-state.json",
    schemaVersion: 1,
    projectRoot: root,
    feature: "structured-logging",
    revision: "r7-closure1",
    requests: [{
      name: "ImplementT5Tests",
      taskId: "T5",
      agent: "engineer",
      effects: {
        kind: "write",
        paths: [
          "cmd/mcp-bridge/embedding_env_reload_test.go",
          "cmd/mcp-bridge/embedding_env_rotation_test.go",
          "cmd/mcp-bridge/serve_test.go",
        ],
      },
    }],
  },
  tasks: [{
    name: "ImplementT5Tests",
    agent: "engineer",
    task: `# Target
Task T5 — migrate three stale reload assertions to the new envelope. Your behavior migration emits \`config_reload_rejected\` via the shared renderer; these files still assert the OLD reload event and \`error.code "invalid_config"\`. Exact findings (verified by the T5 engineer):

1. \`cmd/mcp-bridge/embedding_env_reload_test.go:101,111-112\` — filters for \`event=reload\`, then requires \`error.code=invalid_config\`. New record: \`event=config_reload_rejected\`, operation \`reload\`; shared renderer emits \`restart_required\` for immutable changed embedding values and \`reference_unresolved\` for missing/empty refs.
2. \`cmd/mcp-bridge/embedding_env_rotation_test.go:86,95\` — waits for \`event=reload\`, then requires \`error.code=invalid_config\`. Rotation hits the restart-only guard; new record: \`config_reload_rejected\` / \`restart_required\` / operation \`reload\`.
3. \`cmd/mcp-bridge/serve_test.go:744,748-750\` — waits for \`reload\`, then for nested \`error.code=invalid_config\`. Invalid \`servers[0].endpoint\` now emits \`config_reload_rejected\` / \`url_invalid\` / operation \`reload\`.

# Change
Update ONLY those three files: replace the stale \`event=reload\` / \`error.code "invalid_config"\` expectations with the new event, operation, and the specific code named above (plus the nested AD8 error shape and \`config.*\` attribution where the test asserts it). Keep each test's intent (reload rejection visible, no duplicate records, active config still served).

# Verify
\`go test ./cmd/mcp-bridge/...\` — must now pass (the only prior failure was \`TestConfigDiagnosticFieldManifest\`, already fixed in-scope).

# Receipt
Update \`.omp/artifacts/receipt_T5.json\` (task_id T5, status COMPLETED, evidence entries with command and exitCode).

# Constraints
- Only the three files above. No other production or test files.
- At most 42 read/search operations before the first edit; if exceeded, stop and report stalled.`,
    solutionSpace: "Three bounded assertion migrations from the old reload event/code to the new config_reload_rejected envelope with the named codes. No architectural choices remain open.",
  }],
};

fs.writeFileSync(`${root}/.scratch/structured-logging/dispatch-seed-t5-tests.json`, JSON.stringify(seed, null, 2) + "\n", "utf8");
console.log("t5 tests seed written");

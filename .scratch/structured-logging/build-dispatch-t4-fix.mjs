import fs from "node:fs";

const root = "/home/opencode/projects/default/mcp-bridge";
const state = JSON.parse(fs.readFileSync(`${root}/.omp/plans/execution-state.json`, "utf8"));
const paths = state.tasks.find(t => t.id === "T4").effects.paths;

const seed = {
  intent: "Dispatch Task T4 fix to engineer",
  instructions: "Finish the T4 unified log envelope migration: two scoped test failures remain. Build and vet are already green. Fix only the two named failures within the T4 effects.",
  workflowDispatch: {
    statePath: ".omp/plans/execution-state.json",
    schemaVersion: 1,
    projectRoot: root,
    feature: "structured-logging",
    revision: "r7-closure1",
    requests: [{ name: "ImplementT4Fix", taskId: "T4", agent: "engineer", effects: { kind: "write", paths } }],
  },
  tasks: [{
    name: "ImplementT4Fix",
    agent: "engineer",
    task: `# Target
Task T4 — finish the unified log envelope migration. Two scoped test failures remain; \`go build ./...\` and \`go vet ./...\` are already green.

# Current state (verified this session)
- \`go build ./...\` exit 0; \`go vet ./...\` exit 0.
- \`go test ./internal/upstream/... ./internal/auth/...\` pass.
- Two failures remain:

1. \`internal/observe\` — \`TestHTTPDiagnosticEmitsExactClosedLaneShape\`: the HTTP lane emits 55 fields, wants 56. The ONLY missing slot is \`process_id\` (the record has \`process_state: "unavailable"\` but no \`process_id\`). The test's \`wantKeys\` = \`slotKeys\` minus \`error\`, \`exposed_tools\`, \`effective_policy_revision\`, \`clock_skew_seconds\`. Reconcile the encoder and the test so the closed lane shape is exactly the expected set (either always emit \`process_id\` per the AD2 map, or correct the test's fixture/expectation — decide from the spec's AD2/ADV-6 boundary rule).

2. \`internal/service\` — \`TestInboundUnauthorizedLogClassifiesAuthFailure\`: the unauthorized audit record emits \`failure_stage: "inner_credential"\`, \`failure_reason: "static_invalid"\`, and no \`error\` object; the test requires the record to contain \`client_auth_failed\` (and must NOT contain \`backend_auth_failed\`). The inbound rejection at \`internal/service/inbound.go:466\` and \`:688\` sets \`legacyRejection{"client_auth", "client_auth_failed"}\`; the migration is mis-mapping it. Fix the classification so the unauthorized path emits \`client_auth_failed\`.

# Change
Fix only these two issues within the T4 effects. Do not re-migrate anything else.

# Verify (all must pass)
\`go build ./... && go vet ./...\`, \`go test ./internal/observe/... ./internal/service/... ./internal/upstream/... ./internal/auth/...\`.

# Receipt
Write \`.omp/artifacts/receipt_T4.json\` per the receipt schema (task_id T4, status COMPLETED, evidence entries with command and exitCode).

# Constraints
- Only the files in the dispatch effects. No other production files.
- At most 42 read/search operations before the first edit; if exceeded, stop and report stalled.
- No external SDK/vendor research.`,
    solutionSpace: "Two bounded fixes: the HTTP lane's missing process_id slot and the inbound unauthorized client_auth_failed classification. No architectural choices remain open.",
  }],
};

fs.writeFileSync(`${root}/.scratch/structured-logging/dispatch-seed-t4-fix.json`, JSON.stringify(seed, null, 2) + "\n", "utf8");
console.log("t4 fix seed written");

import fs from "node:fs";

const root = "/home/opencode/projects/default/mcp-bridge";
const state = JSON.parse(fs.readFileSync(`${root}/.omp/plans/execution-state.json`, "utf8"));
const paths = state.tasks.find(t => t.id === "T4").effects.paths;

const seed = {
  intent: "Dispatch Task T4 cmd-migration fix to engineer",
  instructions: "Fix the two classes of cmd/mcp-bridge failures blocking T4's package suite: a K3 production regression and test capture mismatches. A read-only root-cause analysis is provided.",
  workflowDispatch: {
    statePath: ".omp/plans/execution-state.json",
    schemaVersion: 1,
    projectRoot: root,
    feature: "structured-logging",
    revision: "r7-closure1",
    requests: [{ name: "ImplementT4CmdFix", taskId: "T4", agent: "engineer", effects: { kind: "write", paths } }],
  },
  tasks: [{
    name: "ImplementT4CmdFix",
    agent: "engineer",
    task: `# Target
Task T4 — complete the cmd/mcp-bridge test migration. A read-only debugger root-caused all remaining failures into TWO classes. Use these findings directly (do not re-derive).

# Class 1 — K3 PRODUCTION regression (fix the code)
The T4Fix round regressed the K3 exception:
- \`internal/observe/httpdiagnostics.go:437-438\` (in \`observeEntry\`) attaches a nested \`error\` for \`client_auth_failed\`.
- \`internal/observe/httpdiagnostic_encode.go:83\` allows that HTTP error code.
The spec's K3 exception requires the 401 client-auth failure to carry NO nested error (asserted by \`cmd/mcp-bridge/serve_logging_test.go\` ClientAuthentication). Remove the auth special-case in \`observeEntry\` and the corresponding allowance in \`httpdiagnostic_encode.go\`; keep the encoder/routing otherwise. Then migrate \`TestInboundUnauthorizedLogClassifiesAuthFailure\` (\`internal/service/service_edge_test.go\`) — it currently passes only because of the regression; after the fix, assert the NEW representation (\`stage: "client_auth"\` present, \`backend_auth_failed\` absent) instead of requiring the literal \`client_auth_failed\`.

# Class 2 — TEST capture mismatches (fix the tests)
\`serve.go:50\` now uses \`NewLogger(stdout, stderr)\` and \`internal/observe/log.go\` routes warn/error→stderr, info/debug→stdout (CORRECT per AC04-4 — do NOT undo routing). The affected harnesses separate the streams and read only stdout. Fix the TESTS to capture both streams:
- \`cmd/mcp-bridge/serve_logging_test.go:250-256\` (\`startHTTPLoggingBridgeWithWriter\`): combine stderr into the captured writer for custom (non-\`*syncWriter\`) writers too.
- \`cmd/mcp-bridge/serve_logging_failures_test.go:356\`: combine/expose stderr.
- \`cmd/mcp-bridge/serve_logging_streaming_test.go:168\`: capture stderr.
- \`cmd/mcp-bridge/serve_oauth_diagnostics_test.go:67\`: match both streams (harness separates them at \`serve_oauth_test.go:159-162\`).
- \`cmd/mcp-bridge/serve_upstream_restart_test.go:80-82\` and \`:255\`: read the injected writer, not process \`os.Stdout\` (the event is unchanged, info → stdout).
- \`cmd/mcp-bridge/embedding_env_rotation_test.go:86\`: poll stderr (warn/reload); \`:94\`: assert nested \`error.code\` and \`config.property_path\`, not flat code/location.

Do NOT change \`manifest_test.go\` (T5 owns its manifest migration).

# Verify (all must pass now)
- \`go build ./... && go vet ./...\`
- \`go test ./cmd/mcp-bridge/...\`
- \`go test ./internal/observe/... ./internal/service/... ./internal/upstream/... ./internal/auth/...\`

# Receipt
Write \`.omp/artifacts/receipt_T4.json\` (task_id T4, status COMPLETED, evidence entries with command and exitCode).

# Constraints
- Only the files in the dispatch effects. No T5 behavior/renderer-loop change; no legacy field reintroduction; no new vocabulary keys.
- At most 42 read/search operations before the first edit; if exceeded, stop and report stalled.
- No external SDK/vendor research.`,
    solutionSpace: "Two bounded classes: revert the K3 regression in observeEntry/encoder (production) and make the affected cmd test harnesses capture the stderr stream the correct AC04-4 routing now uses. No architectural choices remain open.",
  }],
};

fs.writeFileSync(`${root}/.scratch/structured-logging/dispatch-seed-t4-cmdfix.json`, JSON.stringify(seed, null, 2) + "\n", "utf8");
console.log("t4 cmd-fix seed written");

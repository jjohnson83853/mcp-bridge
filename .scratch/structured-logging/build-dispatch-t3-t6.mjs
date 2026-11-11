import fs from "node:fs";

const root = "/home/opencode/projects/default/mcp-bridge";
const state = JSON.parse(fs.readFileSync(`${root}/.omp/plans/execution-state.json`, "utf8"));
const pathsFor = id => state.tasks.find(t => t.id === id).effects.paths;

const common = {
  statePath: ".omp/plans/execution-state.json",
  schemaVersion: 1,
  projectRoot: root,
  feature: "structured-logging",
  revision: "r7-closure1",
};

const t3 = {
  intent: "Dispatch Task T3 to engineer",
  instructions: "Implement typed startup and CLI failures per .scratch/structured-logging/spec.md (revision r7-closure1), ticket 03, AD5/AD6, and AC03-1..AC03-7.",
  workflowDispatch: {
    ...common,
    requests: [{ name: "ImplementT3", taskId: "T3", agent: "engineer", effects: { kind: "write", paths: pathsFor("T3") } }],
  },
  tasks: [{
    name: "ImplementT3",
    agent: "engineer",
    task: `# Target
Task T3 — Typed startup and CLI failures. Feature \`structured-logging\`, spec revision r7-closure1.

Read first, bounded: \`.scratch/structured-logging/spec.md\` §4 AD5 (r7: level/outcome fields; \`startup_publication_failed\` row; \`load_config\` for the config branch), AD6; §5 AC03-1..AC03-7; §6 T3; \`.scratch/structured-logging/decisions.md\`; \`.scratch/structured-logging/issues/03-*.md\`.

# Change
1. \`cmd/mcp-bridge/startup_failure.go\` (new): typed \`startupFailure\` value with explicit \`level\` and \`outcome\` fields (constant per row) per AD5.
2. \`cmd/mcp-bridge/main.go\`: flag handling per AD6 — \`flags.SetOutput(io.Discard)\` before parse; NO custom \`Usage\`; in the \`flag.ErrHelp\` branch ONLY temporarily switch output to stderr, print the unchanged default usage, restore \`io.Discard\`, exit 2 human-readable no JSON; every other parse error emits exactly one \`cli_usage_failed\` envelope (\`error.code "cli_usage"\`, kind \`usage\`, retryable false, static remediation, operation \`cli\`) on stderr, exit 1.
3. \`cmd/mcp-bridge/serve.go\`: startup failure sites (:81-127, :236, :240/:245) and \`loop.Start\` error path (:262) emit typed records per AD5; startup publication failure (\`errors.Is(err, service.ErrPublicationFailed)\`) → event \`startup_publication_failed\`, operation \`startup\`, level \`error\`, outcome \`failure\`, code \`publication_failed\`, kind \`internal\`, retryable true, static remediation — exactly once.
4. Tests: \`cmd/mcp-bridge/startup_failure_test.go\` (new), \`cmd/mcp-bridge/runtime_edge_test.go\`, \`cmd/mcp-bridge/serve_edge_test.go\` (\`TestServeStartupFailures\` → \`errors.As\`; ADDS the startup-publication single-record assertion, ADV-D).

# Acceptance (AC03-1..AC03-7)
- AC03-1..AC03-6: each startup failure class emits one JSON error record per the AD5 table — distinct stable \`error.code\`/\`error.kind\`, event \`startup_failed\`, operation \`startup\`, LEVEL \`error\` and OUTCOME \`failure\` asserted; the table includes the r7 \`startup_publication_failed\` row (level error, outcome failure, code \`publication_failed\`, kind \`internal\`, retryable true), asserted once with no second record from any other producer; CLI errors typed JSON exit 1 operation \`cli\`, no usage text; \`--help\` unchanged.
- AC03-7: invalid document through \`serve\` → exactly one \`config_validation_failed\` envelope, exit 1, operation \`load_config\`, identical attribution fields to \`--check-config\` (which also pins \`load_config\`), no plaintext.

# RED/GREEN
RED first: typed records per class incl. \`startup_publication_failed\` exactly once — expected failure: plaintext path, exit 2, usage text, reload-named publication event. Then implement to GREEN.

# Verify
\`go test ./cmd/mcp-bridge -run 'Flag|Startup|Usage|Help|RunContext|ServeStartup'\`, then \`go test ./cmd/mcp-bridge/...\`.

# Receipt
Write \`.omp/artifacts/receipt_T3.json\` per the receipt schema (task_id T3, status COMPLETED, evidence entries with command and exitCode).

# Constraints
- Only the files in the dispatch effects. No other production files.
- Prohibited: no usage text on parse errors; no \`--help\` change; no config-branch changes beyond B6; no wiring/operatorLog changes (T5).
- At most 42 read/search operations before the first edit; if exceeded, stop and report stalled.
- No external SDK/vendor research.`,
    solutionSpace: "Implementation is bounded by the spec: AD5 typed startupFailure rows, AD6 flag handling, and the AC03-1..7 tests. No architectural choices remain open.",
  }],
};

const t6 = {
  intent: "Dispatch Task T6 to engineer",
  instructions: "Implement the validation success record per .scratch/structured-logging/spec.md (revision r7-closure1), ticket 06, AD7/AD8, ADV-C, and AC06-1..AC06-5.",
  workflowDispatch: {
    ...common,
    requests: [{ name: "ImplementT6", taskId: "T6", agent: "engineer", effects: { kind: "write", paths: pathsFor("T6") } }],
  },
  tasks: [{
    name: "ImplementT6",
    agent: "engineer",
    task: `# Target
Task T6 — Validation success record. Feature \`structured-logging\`, spec revision r7-closure1.

Read first, bounded: \`.scratch/structured-logging/spec.md\` §4 AD7/AD8, ADV-C (level \`info\`, outcome \`success\`); §5 AC06-1..AC06-5; §6 T6; \`.scratch/structured-logging/decisions.md\`; \`.scratch/structured-logging/issues/06-*.md\`.

# Change
1. \`cmd/mcp-bridge/main.go\`: \`checkConfig\` success path builds the success envelope DIRECTLY via the envelope builder — event \`config_validation_succeeded\`, operation \`check_config\`, level \`info\`, outcome \`success\`, no \`error\` object, \`eventMessage\` message ("configuration valid"), no configuration contents; stdout, exit 0.
2. \`cmd/mcp-bridge/config_envelope_test.go\`: success-record assertions.
3. \`cmd/mcp-bridge/example_config_test.go\` (:67 migrates).
4. \`cmd/mcp-bridge/serve_edge_test.go\` (:42 migrates).
5. \`cmd/mcp-bridge/main_test.go\`: \`TestCheckConfigPrintsReferenceNamesWithoutValues\` :21-43 migrates — the success stdout is the JSON envelope and no longer carries the \`cfg.String()\` reference names; the no-secret-leak assertions are preserved against the envelope.
6. \`cmd/mcp-bridge/embedding_env_cli_test.go\`, \`cmd/mcp-bridge/embedding_env_process_test.go\`: success-path assertions as needed.

# Acceptance (AC06-1..AC06-5)
- AC06-1..AC06-5: valid document → exactly one \`config_validation_succeeded\` record, stdout, exit 0, LEVEL \`info\`, OUTCOME \`success\` asserted, non-empty \`message\`, operation \`check_config\`, no \`error\` object, no config values; invalid → failure record stderr exit 1 (\`load_config\`).

# RED/GREEN
RED first: success-record test — expected failure: plaintext \`configuration valid: ...\`. Then implement to GREEN.

# Verify
\`go test ./cmd/mcp-bridge -run CheckConfig\`, \`go test ./cmd/mcp-bridge/...\`.

# Receipt
Write \`.omp/artifacts/receipt_T6.json\` per the receipt schema (task_id T6, status COMPLETED, evidence entries with command and exitCode).

# Constraints
- Only the files in the dispatch effects. No other production files.
- Prohibited: serializing configuration content; Diagnostics renderer for the success record.
- At most 42 read/search operations before the first edit; if exceeded, stop and report stalled.
- No external SDK/vendor research.`,
    solutionSpace: "Implementation is bounded by the spec: direct-builder success envelope, level info/outcome success, and the AC06-1..5 tests. No architectural choices remain open.",
  }],
};

fs.writeFileSync(`${root}/.scratch/structured-logging/dispatch-seed-t3.json`, JSON.stringify(t3, null, 2) + "\n", "utf8");
fs.writeFileSync(`${root}/.scratch/structured-logging/dispatch-seed-t6.json`, JSON.stringify(t6, null, 2) + "\n", "utf8");
console.log("seeds written: t3, t6");

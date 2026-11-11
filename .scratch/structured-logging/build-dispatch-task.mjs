// Builds the engineer dispatch seed for one r8 task, from the INSTALLED r8 state.
//   node .scratch/structured-logging/build-dispatch-task.mjs T9
//   node .scratch/structured-logging/build-dispatch-task.mjs T9 .scratch/structured-logging/new-state-r8.json
// Effect paths are copied verbatim from that task's targetPaths, so a seed cannot drift
// from the installed envelope. Dispatch STRICTLY SERIAL in ticket order:
//   T8 -> T9 -> T10 -> T11 -> T12
// Then: node workflow-policy/execution-state.mjs prepare-dispatch <seed> <state>
import { readFileSync, writeFileSync, existsSync } from "node:fs";

const ROOT = "/home/opencode/projects/default/mcp-bridge";
const SC = `${ROOT}/.scratch/structured-logging`;
const FEATURE = "structured-logging-pin-diagnostics";
const REVISION = "r8-pin-diagnostics-narrow4";

const HEADER = `Feature \`${FEATURE}\`, revision \`${REVISION}\`.`;
const RULE = `# Constraints
- Only the files in this dispatch's effects. No other production files.
- At most 42 read/search operations before the first edit; if exceeded, stop and report stalled.
- No external SDK/vendor research.`;

const PROMPTS = {
  T8: {
    ticket: "08",
    title: "Refresh pass lifecycle records",
    read: "section 2 (R20/R22/R25), section 4 (AD16/AD19/AD20), section 5 (AC08-1..AC08-7), section 6 T8; rulings 6, 7, 11-15",
    change: `1. \`internal/service/refresh.go\`: the pass outcome is discarded at \`report(assembly, err)\` and \`runPass()\` emits no lifecycle records. Capture that error and emit AD16 records per pass: one start record and exactly one outcome record sharing a \`refresh_id\`, each carrying \`config_revision\`, \`trigger\` and \`next_attempt_ms\` (AC08-2).
2. Failure: exactly one \`catalog_refresh_failed\` at warn or error with debug disabled (AC08-1, AC08-7) stating \`catalog_state\` as retained (\`retained_generation\`, \`catalog_tools\`) or \`none_available\` (AC08-3). No field carries raw error text (R8). Each failed pass repeats its complete diagnostic, undeduplicated and untruncated (AC08-6).
3. Success: counts only - \`servers_discovered\`, \`tools_discovered\`, \`tools_published\`, \`pins_resolved\`, \`pins_total\` (AC08-4) and NO individual server or tool name anywhere (AC08-5).
4. \`internal/observe/log.go\`: add the AD19 events owned here (\`catalog_refresh_started\`, \`catalog_refresh_completed\`, \`catalog_refresh_failed\`), the \`refresh\` operation value, and the whitelist entries. \`internal/observe/envelope.go\`: extend \`EnvelopeFieldNames()\` with the AD20 keys this task owns.`,
    red: "Author `internal/service/refresh_lifecycle_test.go` first: a pass whose candidate is rejected must emit started+failed records sharing `refresh_id`. Run it, capture the failing output as RED evidence.",
    verify: "`go test ./internal/service -run Refresh`, `go vet ./...`, `gofmt -l` over the changed files (must be empty).",
    extra: "- No metric changes; no `internal/service/reload.go` or `cmd/mcp-bridge/wiring.go` changes; no `schema_version` change.\n- If a required field cannot be sourced without raw error text (R8), report BLOCKED with evidence rather than emitting it.",
  },
  T9: {
    ticket: "09",
    title: "Pin blocker detail",
    read: "section 2 (R21, R8), section 4 (AD18), section 5 (AC09-1..AC09-8), section 6 T9; rulings 2-5 and 13",
    change: `1. \`internal/service/reload.go\`: the pin identity at the existing wrap site (defect site reload.go:562-564) is text-only. Carry structured pin identity ALONGSIDE the existing wrap - do not replace the wrap text.
2. \`internal/service/refresh.go\`: collect every pin blocker in the pass, not just the first.
3. \`internal/service/refresh.go\` / \`internal/observe/log.go\`: emit \`pin_blocker_detail\` per pin with source location (\`property_path\`, line, column equivalent) and the structured pin identity; \`internal/observe/envelope.go\`: add the \`pin_blocker_detail\` AD20 keys.
4. Every field ends up in the record - no raw error text anywhere (R8).`,
    red: "Author `internal/service/pin_blocker_test.go` first: with several unresolved pins, ALL pins are reported with source locations. Run it, capture the failing output as RED evidence - today only the first, text-only pin is described.",
    verify: "`go test ./internal/service -run 'Pin|Blocker|Refresh'`, `go vet ./...`, `gofmt -l` over the changed files.",
    extra: "- No raw error text in any field (R8).\n- Do not change the r7 configuration-diagnostic renderer or its tests.",
  },
  T10: {
    ticket: "10",
    title: "Discovery attribution and readiness reason",
    read: "section 2 (R23), section 4 (AD17), section 5 (AC10-1..AC10-5), section 6 T10; rulings 8 and 9",
    change: `1. Locate the discovery site named in the ledger from the evidence - expected \`internal/upstream/dedicated.go\`. If the site is not there, STOP and report BLOCKED with the evidence; do not guess a file.
2. Emit \`upstream_discovery_failed\` with stage, status and endpoint attribution for a per-endpoint discovery failure. The endpoint sanitizer emits scheme/host/port/path ONLY.
3. \`internal/service/refresh.go\`: carry retryable and next-attempt information.
4. \`internal/service/ops.go\`: populate the finite \`Reason\` classes (currently empty).
5. \`cmd/mcp-bridge/wiring.go\`: \`publishObserver\` must update readiness on REJECTION too, not only on publish. Add readiness assertions to \`cmd/mcp-bridge/serve_edge_test.go\`.`,
    red: "Author the upstream discovery test at the located site first: a discovery failure asserts stage/status/endpoint attribution. It must fail before the change. Capture the RED output.",
    verify: "`go test ./internal/service ./internal/upstream ./cmd/mcp-bridge -run 'Discovery|Readiness|Refresh'`, `go vet ./...`, `gofmt -l` over the changed files.",
    extra: "- No credentials, headers, query strings or bodies in any field; the endpoint sanitizer emits scheme/host/port/path only.\n- No metric changes.",
  },
  T11: {
    ticket: "11",
    title: "Path parity and logger fallback",
    read: "section 2 (R21, R24, R25), section 4 (AD21), section 5 (AC11-1..AC11-3), section 6 T11; rulings 10 and 16-18",
    change: `1. \`internal/service/loop.go\`: attribute reload revision and trigger from the real sources (\`loop.go\` watch, Refresher tick, Trigger).
2. \`cmd/mcp-bridge/serve.go\`: the startup path must produce the SAME pin detail as the reload path for the same pinned-catalog rejection.
3. \`cmd/mcp-bridge/wiring.go\`: add \`config_revision\`/\`trigger\` to \`config_reload_rejected\`, and emit \`config_reload_applied\`.
4. \`internal/observe/log.go\` / \`envelope.go\`: add the two remaining AD19 events, the \`logging\` operation value, and the single-attempt fallback path that surfaces \`logging_write_failed\` when encoding fails. Add \`internal/service/parity_test.go\` and extend \`internal/service/loop_test.go\`, plus the \`internal/observe\` fallback test (locate the existing one; create \`internal/observe/fallback_test.go\` only if none exists).`,
    red: "Author `internal/service/parity_test.go` first: drive the SAME pinned-catalog rejection through the startup and reload paths and assert equal detail - expected failure, because the startup path lacks the detail. Then the fallback test: an encode failure must surface `logging_write_failed` - expected failure, single attempt, no recursion. Capture both RED outputs.",
    verify: "`go test ./internal/service ./internal/observe ./cmd/mcp-bridge -run 'Parity|Reload|Fallback'`, `go vet ./...`, `gofmt -l` over the changed files.",
    extra: "- Do not alter r7 record assertions: additive fields only (AD21).\n- The fallback makes exactly one attempt; no recursion.",
  },
  T12: {
    ticket: "12",
    title: "Contract docs and verification closeout",
    read: "section 7 (observability), section 5 (AC12-1..AC12-3), section 6 T12; the AD20 rulings per event and the final vocabulary",
    change: `1. \`docs/logging-contract.md\`: document the new events and fields, and the optional-versus-\`schema_version\` ruling per AD20.
2. \`README.md\` and \`CHANGELOG.md\`: the new events/fields.
3. \`cmd/mcp-bridge/logging_contract_test.go\`: doc-drift test for the new events, query-key tests, and full-suite consolidation covering the AC08..AC12 producers.`,
    red: "Author the doc-drift test for the new events first; it must fail before the docs are written. Capture the RED output.",
    verify: "`go test ./...` (full-suite gate), `go vet ./...`, `gofmt -l` over the changed files (must be empty).",
    extra: "- No documented claim beyond records that are actually emitted.\n- No induced-failure review gate (owner ruling 19).",
  },
};

const taskId = process.argv[2];
if (!PROMPTS[taskId]) {
  console.error(`usage: build-dispatch-task.mjs <${Object.keys(PROMPTS).join("|")}> [statePath]`);
  process.exit(2);
}
const meta = PROMPTS[taskId];
const statePath = process.argv[3] ?? `${ROOT}/.omp/plans/execution-state.json`;
const fallback = `${SC}/new-state-r8.json`;
const stateFile = existsSync(statePath) && JSON.parse(readFileSync(statePath, "utf8")).feature === FEATURE ? statePath : fallback;
const state = JSON.parse(readFileSync(stateFile, "utf8"));
if (state.feature !== FEATURE) throw new Error(`no ${FEATURE} state at ${statePath} or ${fallback}`);

const t = state.tasks.find((x) => x.id === taskId);
if (!t) throw new Error(`${taskId} is not present in ${stateFile}`);
const paths = t.effects.paths.map((p) => (p.startsWith(ROOT) ? p.slice(ROOT.length + 1) : p));

const task = `# Target
Task ${taskId} - ${meta.title}. ${HEADER} Ticket ${meta.ticket}.

Read first, bounded: \`.scratch/structured-logging/spec.md\` ${meta.read}; \`.scratch/structured-logging/issues/${meta.ticket}-*.md\`.

# Change
${meta.change}

# Test first
${meta.red}

# Verify
${meta.verify}

# Receipt
Write \`.omp/artifacts/receipt_${taskId}.json\` with task_id ${taskId}, status, and evidence entries carrying command and exitCode.

${RULE}
${meta.extra}`;

const instructions = `Implement ${meta.title} end to end per .scratch/structured-logging/spec.md (revision ${REVISION}), ticket ${meta.ticket}, and the recorded decisions.`;
const workflowDispatch = {
  // The payload always names the real live state; --state-path only redirects the
  // prepare-dispatch SEED so this can be preflighted against an uninstalled draft.
  statePath: ".omp/plans/execution-state.json",
  schemaVersion: 1,
  projectRoot: ROOT,
  feature: FEATURE,
  revision: REVISION,
  requests: [{ name: `Implement${taskId}`, taskId, agent: "engineer", effects: { kind: "write", paths } }],
};
const tasks = [
  {
    name: `Implement${taskId}`,
    agent: "engineer",
    task,
    solutionSpace: `Implementation is bounded by the spec: ${meta.title}, its acceptance criteria and its listed RED/GREEN tests. No architectural choices remain open.`,
  },
];

// buildDispatchPayload(seed) takes the FLAT seed {intent, instructions, workflowDispatch,
// tasks} (execution-state.mjs:56, :717). The task tool itself is called with the payload form.
const seedOverride = (process.argv.find((a) => a.startsWith("--state-path=")) ?? "").split("=")[1];
const seed = {
  intent: `Dispatch Task ${taskId} to engineer`,
  instructions,
  workflowDispatch: seedOverride ? { ...workflowDispatch, statePath: seedOverride } : workflowDispatch,
  tasks,
};
const payload = { i: seed.intent, context: JSON.stringify({ instructions, workflowDispatch }), tasks };

const stem = taskId.toLowerCase();
writeFileSync(`${SC}/dispatch-seed-${stem}.json`, `${JSON.stringify(seed, null, 2)}\n`);
writeFileSync(`${SC}/dispatch-payload-${stem}.json`, `${JSON.stringify(payload, null, 2)}\n`);
console.log(`seed    (prepare-dispatch): dispatch-seed-${stem}.json`);
console.log(`payload (task tool)       : dispatch-payload-${stem}.json`);
console.log(`${taskId} deps   : ${t.blockedBy.length ? t.blockedBy.join("+") : "none (entry point)"}  | stage: ${t.stage}`);
console.log(`${taskId} effects: ${paths.length} paths`);
console.log(`acceptance: ${t.acceptanceIds.join(",")}`);
console.log(`state read: ${stateFile}`);

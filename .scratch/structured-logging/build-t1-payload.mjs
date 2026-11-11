import fs from "node:fs";
import path from "node:path";
import { buildDispatchPayload, validateDispatchPayload } from "/home/opencode/.omp/agent/workflow-policy/execution-gate.ts";

const root = "/home/opencode/projects/default/mcp-bridge";
const statePath = ".omp/plans/execution-state.json";
const state = JSON.parse(fs.readFileSync(path.join(root, statePath), "utf8"));
const task = state.tasks.find(t => t.id === "T1");
if (!task) throw new Error("T1 missing from state");

const instructions = `Implement task T1 of feature structured-logging per .scratch/structured-logging/spec.md (revision ${state.revision}). Read the specification sections 2, 4 (AD1, AD3, AD8, AD9), 5 (AC01-1..AC01-8) and 6 (T1) before editing. Follow the repository's existing conventions. Do not touch files outside the declared effect paths.`;

const taskText = `# Target
Task T1 — JSON error envelope and pin-ceiling attribution, end to end.

# Files (exact; nothing else may be written)
- internal/observe/envelope.go (new)
- internal/observe/log.go (minimal: envelope used by failure emission; full logger migration is T4)
- internal/config/config.go (move the pin ceiling into the ordered per-pin loop)
- internal/config/diagnostics.go (add RelatedLine, RelatedColumn, ActualCount, MaxCount, FirstExcessIndex)
- internal/config/pins_bounds_test.go
- cmd/mcp-bridge/diagnostics_envelope.go (new)
- cmd/mcp-bridge/main.go (checkConfig failure path)
- cmd/mcp-bridge/config_envelope_test.go (new)
- cmd/mcp-bridge/example_config_test.go (failure-path assertions only)
- cmd/mcp-bridge/serve_edge_test.go (check-config assertions only)
- .omp/artifacts/receipt_T1.json

# Change
1. Add the versioned envelope in internal/observe: schema_version 1, level, service "mcp-bridge", event, message, operation, outcome, nested error {code, kind, retryable, remediation}, and event-specific objects. No timestamp, environment, version, or subsystem. One JSON object per line, one write, no retry, no plaintext fallback. debug/info to stdout, warn/error to stderr.
2. Add the shared configuration renderer in cmd/mcp-bridge that maps a *config.Diagnostics to one envelope: first record only, config.path, config.property_path, one-based line/column, safe identifiers, related location, counts, and the static reason as message.
3. Move the pin-ceiling check into the ordered per-pin loop so entries 0..63 validate first; at index 64 raise pin_ceiling with property_path "pins[64]", that element's line/column from the YAML root node, actual_count, max_count 64, first_excess_index 64, and pin_server/pin_tool only when that entry passes the same validation as the earlier entries.
4. Make the --check-config failure path emit exactly one JSON error envelope on stderr and exit 1; remove the plaintext "invalid configuration:" output.
5. Migrate the named tests to assert the new behaviour.

# Acceptance
- AC01-1: 65 pins produce exactly one JSON line on stderr and exit 1.
- AC01-2: the record carries property_path "pins[64]", its one-based line/column, actual_count 65, max_count 64, first_excess_index 64.
- AC01-3: safe pin_server/pin_tool of the first excess entry only.
- AC01-4: no time/timestamp/environment/version/subsystem and no plaintext "mcp-bridge:" prefix.
- AC01-5: 64 pins still start normally with no error record.
- AC01-6: the emitted line parses as one JSON object; stdout is empty on this failure.
- AC01-7: a malformed or hostile pins[64] identity is omitted entirely; positions and counts remain.
- AC01-8: a malformed pins[0] with 65 total entries reports the pins[0] class, not pin_ceiling, in exactly one record.

# Verification (run and record exact commands and exit codes)
- go test ./internal/config -run 'Pin|Ceiling'
- go test ./cmd/mcp-bridge -run 'CheckConfig|ExampleConfig'
- go vet ./...

# Rules
- RED first: extend the named tests to express the acceptance criteria and observe them fail for the current plaintext/ordering behaviour, then implement to GREEN.
- Do not modify the HTTP diagnostic lane, do not delete the legacy logger, do not implement the success record or other rejection classes.
- Write the receipt to .omp/artifacts/receipt_T1.json with the exact commands, exit codes, and a summary of the diff.
- If a required fact is missing or a file outside the list must change, stop and report BLOCKED with the exact reason; do not expand scope.`;

const seed = {
  intent: "Dispatch task T1 to engineer",
  instructions,
  workflowDispatch: {
    statePath,
    schemaVersion: 1,
    projectRoot: root,
    feature: state.feature,
    revision: state.revision,
    requests: [
      {
        name: "ImplementT1",
        taskId: "T1",
        agent: "engineer",
        effects: { kind: "write", paths: task.effects.paths },
      },
    ],
  },
  tasks: [
    {
      name: "ImplementT1",
      agent: "engineer",
      task: taskText,
      solutionSpace: "One implementation path: the specification fixes the envelope shape, the renderer, the ceiling ordering and the acceptance tests. Local Go details only.",
    },
  ],
};

const built = buildDispatchPayload(seed);
if (!built.ok) {
  console.error("buildDispatchPayload failed:", JSON.stringify(built, null, 2));
  process.exit(1);
}
const validated = validateDispatchPayload(built.payload);
console.log("validateDispatchPayload:", JSON.stringify(validated));
if (!validated.ok) process.exit(1);
fs.writeFileSync(path.join(root, ".scratch/structured-logging/t1-payload.json"), JSON.stringify(built.payload, null, 2), "utf8");
fs.writeFileSync(path.join(root, ".scratch/structured-logging/t1-request.json"), JSON.stringify(seed.workflowDispatch, null, 2), "utf8");
console.log("wrote t1-payload.json and t1-request.json");

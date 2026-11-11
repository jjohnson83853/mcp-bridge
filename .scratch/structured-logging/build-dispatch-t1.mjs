import fs from "node:fs";

const seed = {
  intent: "Dispatch Task T1 to engineer",
  instructions: "Implement the envelope core and pin-ceiling attribution end to end per .scratch/structured-logging/spec.md (revision r7-closure1), ticket 01, and decisions D1-D10/D15.",
  workflowDispatch: {
    statePath: ".omp/plans/execution-state.json",
    schemaVersion: 1,
    projectRoot: "/home/opencode/projects/default/mcp-bridge",
    feature: "structured-logging",
    revision: "r7-closure1",
    requests: [
      {
        name: "ImplementT1",
        taskId: "T1",
        agent: "engineer",
        effects: {
          kind: "write",
          paths: [
            "internal/observe/envelope.go",
            "internal/observe/log.go",
            "internal/config/config.go",
            "internal/config/diagnostics.go",
            "internal/config/pins_bounds_test.go",
            "cmd/mcp-bridge/diagnostics_envelope.go",
            "cmd/mcp-bridge/main.go",
            "cmd/mcp-bridge/config_envelope_test.go",
            "cmd/mcp-bridge/example_config_test.go",
            "cmd/mcp-bridge/serve_edge_test.go",
            "cmd/mcp-bridge/main_test.go",
            "cmd/mcp-bridge/embedding_env_cli_test.go",
            "cmd/mcp-bridge/embedding_env_process_test.go",
            ".omp/artifacts/receipt_T1.json"
          ]
        }
      }
    ]
  },
  tasks: [
    {
      name: "ImplementT1",
      agent: "engineer",
      task: `# Target
Task T1 — Envelope core + pin-ceiling attribution end to end. Feature \`structured-logging\`, spec revision r7-closure1.

Read first, bounded: \`.scratch/structured-logging/spec.md\` §2 (envelope), §4 (AD3/AD9/AD14), §5 AC01-1..AC01-8, §6 T1; \`.scratch/structured-logging/decisions.md\` D1-D10/D15; \`.scratch/structured-logging/issues/01-json-error-envelope-and-pin-ceiling-attribution.md\`.

# Change
1. \`internal/observe/envelope.go\` (new): envelope v1 record builder plus \`EnvelopeFieldNames() []string\` returning the final r7 union (AD8) — exact sorted union, no duplicates.
2. \`internal/observe/log.go\` (minimal): JSON-per-line writer used by the config failure path.
3. \`internal/config/config.go\`: move the pin ceiling check into the ordered pin loop (currently config.go:210-216) so the failing element index is known; emit \`property_path "pins[64]"\`, one-based line/column of that element, \`actual_count\`, \`max_count\`, \`first_excess_index\`.
4. \`internal/config/diagnostics.go\`: AD3 fields (path, property_path, line, column, server, pin_server, pin_tool, ref_kind, ref_name, related.*).
5. \`cmd/mcp-bridge/diagnostics_envelope.go\` (new): shared renderer for config diagnostics.
6. \`cmd/mcp-bridge/main.go\`: \`--check-config\` failure emits exactly one JSON line on stderr, exit 1, event \`config_validation_failed\`, operation \`load_config\`; stdout empty.
7. Tests: \`internal/config/pins_bounds_test.go\`, \`cmd/mcp-bridge/config_envelope_test.go\` (new), \`cmd/mcp-bridge/example_config_test.go\`, \`cmd/mcp-bridge/serve_edge_test.go\`, \`cmd/mcp-bridge/main_test.go\`, \`cmd/mcp-bridge/embedding_env_cli_test.go\`, \`cmd/mcp-bridge/embedding_env_process_test.go\` — only the assertions named for T1.

# Acceptance (AC01-1..AC01-8)
- AC01-1: 65-pin document → exactly one JSON line on stderr, exit 1; event \`config_validation_failed\`, operation \`load_config\`.
- AC01-2: record has \`property_path "pins[64]"\`, one-based line/column of that element, \`actual_count 65\`, \`max_count 64\`, \`first_excess_index 64\`.
- AC01-3: safe \`pin_server\`/\`pin_tool\` of the first excess entry only.
- AC01-4: no \`time\`/\`timestamp\`/\`environment\`/\`version\`/\`subsystem\`, no plaintext \`mcp-bridge:\` prefix.
- AC01-5: 64 pins start normally, no error record.
- AC01-6: emitted line parses as one JSON object; stdout empty on this failure.
- AC01-7: malformed/attacker-controlled \`pins[64]\` identity → \`pin_server\`/\`pin_tool\` omitted, no hostile bytes serialized.
- AC01-8: malformed \`pins[0]\` + 65 entries → pins[0] class (\`pin_fields\` at index 0), NOT \`pin_ceiling\`; exactly one record.

# RED/GREEN
RED first: add the failing tests for AC01-1..AC01-8, run them, capture the failure output. Then implement to GREEN.

# Verify
\`go test ./internal/config/... ./cmd/mcp-bridge/...\`, \`go vet ./...\`, \`gofmt -l .\` (must be empty).

# Receipt
Write \`.omp/artifacts/receipt_T1.json\` per the receipt schema (task_id T1, status COMPLETED, evidence entries with command and exitCode).

# Constraints
- Only the files in the dispatch effects. No other production files.
- At most 42 read/search operations before the first edit; if exceeded, stop and report stalled.
- No external SDK/vendor research.`,
      solutionSpace: "Implementation is bounded by the spec: envelope v1 builder, ordered pin-loop ceiling attribution, and the AC01-1..8 tests. No architectural choices remain open."
    }
  ]
};

fs.writeFileSync(".scratch/structured-logging/dispatch-seed-t1.json", JSON.stringify(seed, null, 2) + "\n", "utf8");
console.log("seed written");

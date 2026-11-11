import fs from "node:fs";

const root = "/home/opencode/projects/default/mcp-bridge";
const state = JSON.parse(fs.readFileSync(`${root}/.omp/plans/execution-state.json`, "utf8"));
const paths = state.tasks.find(t => t.id === "T5").effects.paths;

const seed = {
  intent: "Dispatch Task T5 to engineer",
  instructions: "Implement reload rejection consistency and lifecycle records per .scratch/structured-logging/spec.md (revision r7-closure1), ticket 05, AD4/AD7/AD10/AD13, and AC05-1..AC05-7.",
  workflowDispatch: {
    statePath: ".omp/plans/execution-state.json",
    schemaVersion: 1,
    projectRoot: root,
    feature: "structured-logging",
    revision: "r7-closure1",
    requests: [{ name: "ImplementT5", taskId: "T5", agent: "engineer", effects: { kind: "write", paths } }],
  },
  tasks: [{
    name: "ImplementT5",
    agent: "engineer",
    task: `# Target
Task T5 — Reload rejection consistency + lifecycle records. Feature \`structured-logging\`, spec revision r7-closure1.

Read first, bounded: \`.scratch/structured-logging/spec.md\` §4 AD4/AD7/AD10, AD13 (r7 classification table); §5 AC05-1..AC05-7; §6 T5; \`.scratch/structured-logging/decisions.md\`; \`.scratch/structured-logging/issues/05-*.md\`.

# Change
1. \`cmd/mcp-bridge/wiring.go\`: BEHAVIOR migration — single \`config_reload_rejected\` envelope via the shared renderer; publication branch = single \`reload_publication_failed\` emitter; AD13 snapshot/prompt/generic/fallback classification; manifest deletion.
2. \`cmd/mcp-bridge/serve.go\` if needed (prompt closure :129 routing).
3. \`cmd/mcp-bridge/manifest_test.go\`: manifest assertions replaced by envelope assertions.
4. \`cmd/mcp-bridge/serve_edge_test.go\`: :195-240 behavior migration + full-path exactly-one publication count.
5. \`cmd/mcp-bridge/reload_envelope_test.go\` (new; also carries AC05-7).

# Acceptance (AC05-1..AC05-7)
- AC05-1..AC05-4: invalid reload candidate → one warn envelope, \`config_reload_rejected\`, operation \`reload\`, active config keeps serving, candidate not published, startup of the same document stays error/exit 1.
- AC05-5: the publication-failure path emits EXACTLY ONE \`reload_publication_failed\` record — asserted BOTH by the direct \`operatorLog\` test (nested \`error.code "internal_error"\`, kind \`internal\`, operation \`publish\`, static remediation/message, no secret from the underlying error) AND by a FULL-PATH test driving a publication failure through \`Loop.Reload\`/\`Refresher\` with the serve wiring and counting exactly one record (tools.go emitter deleted; precedent \`TestRefreshPublicationFailureClassificationThroughLoop\`).
- AC05-6: multi-record Diagnostics render exactly ONE envelope (first document-order record + \`diagnostics_incomplete\`/\`remaining_stages\`).
- AC05-7: each AD13 lifecycle branch emits its typed envelope — \`startup_snapshot_rejected\` (warn/startup/degraded/\`snapshot_unavailable\`/resource/retryable true), \`prompt_publication_failed\` (warn/publish/failure/\`internal_error\`/internal/false), \`reload_rejected\` (warn/reload/failure/\`internal_error\`/internal/false) — each with pinned level/outcome/code/kind/retryable/static remediation/static message, NO raw error text from the wrapped error, and NO legacy fallback record; the verified call-site inventory (loop.go:146, loop.go:201-204, refresh.go:212-215, serve.go:129) is asserted complete.

# RED/GREEN
RED first: one warn record per AC05-1..5 + AC05-7 typed lifecycle records + the full-path duplicate count — expected failure: duplicate \`reload_publication_failed\`, flat manifest, unspecified fallback records. Then implement to GREEN.

# Verify
\`go test ./cmd/mcp-bridge -run 'Reload|Manifest|Operator|Snapshot|Prompt'\`, \`go test ./cmd/mcp-bridge/...\`.

# Receipt
Write \`.omp/artifacts/receipt_T5.json\` per the receipt schema (task_id T5, status COMPLETED, evidence entries with command and exitCode).

# Constraints
- Only the files in the dispatch effects. No other production files.
- Prohibited: no \`internal/service\`/\`internal/upstream\` edits; no new vocabulary keys beyond AD13's events (no new Fields keys).
- At most 42 read/search operations before the first edit; if exceeded, stop and report stalled.
- No external SDK/vendor research.`,
    solutionSpace: "Implementation is bounded by the spec: single reload-rejection/publication envelopes, AD13 lifecycle classification, and the AC05-1..7 tests. No architectural choices remain open.",
  }],
};

fs.writeFileSync(`${root}/.scratch/structured-logging/dispatch-seed-t5.json`, JSON.stringify(seed, null, 2) + "\n", "utf8");
console.log("t5 seed written");

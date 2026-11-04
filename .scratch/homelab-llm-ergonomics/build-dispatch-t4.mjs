import fs from "node:fs";
import { buildDispatchPayload, validateDispatchPayload } from "/home/opencode/.omp/agent/workflow-policy/execution-gate.ts";

const paths = [
  "internal/catalog/schema.go",
  "internal/catalog/catalog_test.go",
  "internal/service/tools_test.go",
  "cmd/mcp-bridge/serve_test.go",
  "cmd/mcp-bridge/serve_oauth_test.go",
  "cmd/mcp-bridge/serve_hybrid_coexistence_test.go",
  "cmd/mcp-bridge/serve_hybrid_metadata_defaults_test.go",
  "cmd/mcp-bridge/serve_hybrid_metadata_exceptions_test.go",
  "cmd/mcp-bridge/serve_hybrid_metadata_source_test.go",
  "cmd/mcp-bridge/serve_upstream_restart_test.go",
  "README.md",
  "coverage.out",
  ".omp/artifacts/receipt_homelab_T4.json"
];

const seed = {
  intent: "Surgical repair of tools_test.go race and verification",
  instructions: "Implement surgical gate repairs and full repository verification according to .scratch/homelab-llm-ergonomics/spec.md and ticket 04.",
  workflowDispatch: {
    statePath: ".omp/plans/execution-state.json",
    schemaVersion: 1,
    projectRoot: "/home/opencode/projects/default/mcp-bridge",
    feature: "homelab-llm-ergonomics",
    revision: "r7",
    requests: [
      {
        name: "ImplementT4",
        taskId: "T4",
        agent: "engineer",
        effects: {
          kind: "write",
          paths: paths
        }
      }
    ]
  },
  tasks: [
    {
      name: "ImplementT4",
      agent: "engineer",
      task: `# Target: Task T4 - Synchronization Repair in tools_test.go and Full Verification Gate
Apply the debugger-isolated fix in internal/service/tools_test.go to eliminate the race where assertToolOperationLogs checks logs.String() before the server goroutine emits deferred http_complete:

1. In internal/service/tools_test.go:
- Read lines 345-395.
- Update assertToolOperationLogs signature from:
    func assertToolOperationLogs(t *testing.T, rawLogs, operation, outcome, reason, upstreamState string, admission bool)
  to:
    func assertToolOperationLogs(t *testing.T, logs *syncBuffer, operation, outcome, reason, upstreamState string, admission bool)
- At the top of assertToolOperationLogs immediately after t.Helper():
    var rawLogs string
    waitFor(t, "request-correlated http_complete", func() bool {
        rawLogs = logs.String()
        requestID := ""
        for _, record := range decodeToolLogRecords(t, rawLogs) {
            if record["event"] == "mcp_complete" {
                requestID, _ = record["request_id"].(string)
            }
            if record["event"] == "http_complete" && requestID != "" && record["request_id"] == requestID {
                return true
            }
        }
        return false
    })
- Update all call sites of assertToolOperationLogs (lines ~377, ~386, ~440, ~452) to pass &logs instead of logs.String().

2. Verification & Receipt:
- Run: go test -v -count=1 ./internal/service -run 'TestToolSurface(Describe|Call)CatalogRevisionAdmission$'
- Run: go test -race ./...
- Run: go vet ./...
- Run: go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./... && go run ./cmd/covcheck
- Write .omp/artifacts/receipt_homelab_T4.json with status: "COMPLETED", recording all command outcomes.
`,
      solutionSpace: "Update assertToolOperationLogs in tools_test.go to wait for deferred http_complete using waitFor, then execute full race, vet, and coverage verification"
    }
  ]
};

const result = buildDispatchPayload(seed);
if (!result.ok) {
  console.error("buildDispatchPayload failed:", result);
  process.exit(1);
}
console.log("buildDispatchPayload OK!");
const validation = validateDispatchPayload(result.payload);
console.log("validateDispatchPayload:", validation);
fs.writeFileSync(".scratch/homelab-llm-ergonomics/built-payload-t4.json", JSON.stringify(result.payload, null, 2), "utf8");
const { statePath, ...reqOnly } = seed.workflowDispatch;
fs.writeFileSync(".scratch/homelab-llm-ergonomics/dispatch-request-t4.json", JSON.stringify(reqOnly, null, 2), "utf8");

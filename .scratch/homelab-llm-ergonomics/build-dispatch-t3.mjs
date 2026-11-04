import fs from "node:fs";
import { buildDispatchPayload, validateDispatchPayload } from "/home/opencode/.omp/agent/workflow-policy/execution-gate.ts";

const paths = [
  "internal/service/tools.go",
  "internal/service/tools_test.go",
  ".omp/artifacts/receipt_homelab_T3.json"
];

const seed = {
  intent: "Dispatch Task T3 to engineer",
  instructions: "Prune search, describe, and call payloads according to .scratch/homelab-llm-ergonomics/spec.md and ticket 03.",
  workflowDispatch: {
    statePath: ".omp/plans/execution-state.json",
    schemaVersion: 1,
    projectRoot: "/home/opencode/projects/default/mcp-bridge",
    feature: "homelab-llm-ergonomics",
    revision: "r7",
    requests: [
      {
        name: "ImplementT3",
        taskId: "T3",
        agent: "engineer",
        effects: {
          kind: "write",
          paths
        }
      }
    ]
  },
  tasks: [
    {
      name: "ImplementT3",
      agent: "engineer",
      task: `# Target: Task T3 — Prune Search, Describe, and Call Payloads
Reference Ticket: .scratch/homelab-llm-ergonomics/issues/03-prune-search-describe-call-payloads.md
Specification Reference: .scratch/homelab-llm-ergonomics/spec.md (R3, AC-4, AC-5, AC-6, Section 3.4, 4/T3)

# Exact Files and Changes:
1. internal/service/tools.go:
   - In searchPayload(response search.Response) (lines ~890-940):
     - Remove the top-level "backends" field from the payload map. (Retain catalog_revision, mode, degraded_reasons, results).
     - In each result item map:
       - Omit "last_success_at", "catalog_status", and "catalog_revision".
       - Retain: "tool_name", "server", "upstream_name", "description", "score", "pinned", and "availability".
   - In describeTool(ctx context.Context, req *mcp.CallToolRequest) (lines ~500-580):
     - In the payload map:
       - Omit "last_success_at" and "catalog_status".
       - Retain: "server", "catalog_revision", "tool_name", "upstream_name", "description", "pinned", "availability", "validation_status", "input_schema".
       - Preserve "output_schema" and metadata fields when present.
   - In callTool(ctx context.Context, req *mcp.CallToolRequest) (lines ~820-845):
     - In the outer payload map:
       - Omit "catalog_revision", "catalog_status", and "availability".
       - Retain: "server", "tool_name", "request_id", and "upstream_result".

2. internal/service/tools_test.go:
   - Add/update unit test assertions for searchPayload, describeTool, and callTool verifying that pruned fields are omitted and required fields are retained.
   - Update any existing tests expecting "backends", "last_success_at", or "catalog_status" in tool responses.

3. Write terminal receipt:
   Create .omp/artifacts/receipt_homelab_T3.json recording task outcome, executed verification commands, and test pass evidence.

# Verification Commands to Run:
go test -v -count=1 ./internal/service/...
`,
      solutionSpace: "Prune redundant SRE telemetry from searchPayload, describeTool, and callTool response structures and update service tests"
    }
  ]
};

const built = buildDispatchPayload(seed);
console.log("buildDispatchPayload OK!");
const validated = validateDispatchPayload(built.payload);
console.log("validateDispatchPayload:", validated);

fs.writeFileSync(".scratch/homelab-llm-ergonomics/built-payload-t3.json", JSON.stringify(built.payload, null, 2), "utf8");
const { statePath, ...reqOnly } = seed.workflowDispatch;
fs.writeFileSync(".scratch/homelab-llm-ergonomics/dispatch-request-t3.json", JSON.stringify(reqOnly, null, 2), "utf8");

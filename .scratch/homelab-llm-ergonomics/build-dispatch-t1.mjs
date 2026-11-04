import fs from "node:fs";
import { buildDispatchPayload, validateDispatchPayload } from "/home/opencode/.omp/agent/workflow-policy/execution-gate.ts";

const seed = {
  intent: "Dispatch Task T1 to engineer",
  instructions: "Implement optional catalog_revision admission and describe correlation according to .scratch/homelab-llm-ergonomics/spec.md and ticket 01.",
  workflowDispatch: {
    statePath: ".omp/plans/execution-state.json",
    schemaVersion: 1,
    projectRoot: "/home/opencode/projects/default/mcp-bridge",
    feature: "homelab-llm-ergonomics",
    revision: "r7",
    requests: [
      {
        name: "ImplementT1",
        taskId: "T1",
        agent: "engineer",
        effects: {
          kind: "write",
          paths: [
            "internal/catalog/schema.go",
            "internal/catalog/catalog_test.go",
            "internal/service/tools.go",
            "internal/service/direct_execution.go",
            "internal/service/tools_test.go",
            ".omp/artifacts/receipt_homelab_T1.json"
          ]
        }
      }
    ]
  },
  tasks: [
    {
      name: "ImplementT1",
      agent: "engineer",
      task: `# Target: Task T1 — Optional catalog_revision Admission and Describe Correlation
Reference Ticket: .scratch/homelab-llm-ergonomics/issues/01-optional-catalog-revision.md
Specification Reference: .scratch/homelab-llm-ergonomics/spec.md (R1, AC-1, AC-2, Section 3.1, 3.2, 4/T1)

# Exact Files and Changes:
1. internal/catalog/schema.go:
   - Line 18: Update CatalogRevisionPattern = "^$|^[a-f0-9]{64}$".
   - DescribeToolInputSchema() (lines ~50-65): Update "required" from []any{"tool_name", "catalog_revision"} to []any{"tool_name"}.
   - CallToolInputSchema() (lines ~70-85): Update "required" from []any{"tool_name", "catalog_revision", "arguments"} to []any{"tool_name", "arguments"}.
   - Ensure "catalog_revision" property retains pattern: CatalogRevisionPattern.

2. internal/catalog/catalog_test.go:
   - Add unit test coverage verifying catalog_revision is absent from "required" in both schemas.
   - Verify that empty string "" matches CatalogRevisionPattern, valid 64-character lowercase hex string matches, and malformed strings (e.g. "not-hex", "1234", uppercase) fail.

3. internal/service/tools.go:
   - Add helper isValidCatalogRevision(rev string) bool checking len(rev) == 64 and validHex(rev).
   - Standardize admission logging helper:
     func (t *ToolSurface) logAdmissionFailure(ctx context.Context, requestID, operationID, corrState string, start time.Time, operation string, reason string)
     which contributes operation evidence via t.contributeOperation(requestID, operationID, "not_attempted") and emits an mcp_complete warning log entry with Operation: operation, stage: "tool_admission", outcome: "error", reason: reason, and mcp_method: "tools_call".
   - In describeTool(ctx context.Context, req *mcp.CallToolRequest):
     - Update signature to accept ctx context.Context (not ignored _).
     - Extract server-owned request identity and correlation state from carrier header:
       carrier := req.Extra.Header.Get("X-Mcp-Bridge-Internal-Request-Id")
       if len(carrier) == 32 && validHex(carrier) { requestID = carrier; corrState = "valid" }
       operationID := observe.CorrelationID("")
       start := time.Now()
     - If wire.CatalogRevision == "": default wire.CatalogRevision = assembly.CatalogRevision().
     - Else:
       - If !isValidCatalogRevision(wire.CatalogRevision):
         call t.logAdmissionFailure(ctx, requestID, operationID, corrState, start, "describe_tool", "invalid_arguments")
         and return errorResult(search.CodeInvalidArguments, "catalog_revision is not a valid 64-character hexadecimal digest", core.ExecutionNotStarted), nil.
       - If wire.CatalogRevision != assembly.CatalogRevision():
         call t.logAdmissionFailure(ctx, requestID, operationID, corrState, start, "describe_tool", "catalog_changed")
         and return errorResult(codeCatalogChanged, "the catalog revision changed", core.ExecutionNotStarted), nil.
     - On successful describe:
       - Call t.contributeOperation(requestID, operationID, "not_attempted").
       - If t.logger != nil: emit mcp_complete with request_id: requestID, operation_id: operationID, operation: "describe_tool", outcome: "success", reason: "none", correlation_state: corrState.
   - In callTool(ctx context.Context, req *mcp.CallToolRequest):
     - If wire.CatalogRevision == "": default wire.CatalogRevision = assembly.CatalogRevision().
     - Else:
       - If !isValidCatalogRevision(wire.CatalogRevision):
         call t.logAdmissionFailure(ctx, requestID, operationID, corrState, start, "call_tool", "invalid_arguments")
         and return errorResult(search.CodeInvalidArguments, "catalog_revision is not a valid 64-character hexadecimal digest", core.ExecutionNotStarted), nil.
       - If wire.CatalogRevision != assembly.CatalogRevision():
         call t.logAdmissionFailure(ctx, requestID, operationID, corrState, start, "call_tool", "catalog_changed")
         and return errorResult(codeCatalogChanged, "the catalog revision changed", core.ExecutionNotStarted), nil.

4. internal/service/direct_execution.go:
   - Lines 34 and 39: Update calls to e.surface.logAdmissionFailure(ctx, e.requestID, e.operationID, e.correlation, e.requestStart, "call_tool", "invalid_arguments").

5. internal/service/tools_test.go:
   - Update and add comprehensive tests for describe_tool and call_tool:
     - Omitted catalog_revision succeeds and uses live revision.
     - Empty string catalog_revision succeeds and uses live revision.
     - Valid matching 64-hex revision succeeds.
     - Valid stale 64-hex revision fails with catalog_changed and emits mcp_complete with reason: "catalog_changed".
     - Malformed revision (e.g. non-hex string) fails with invalid_arguments and emits mcp_complete with reason: "invalid_arguments".
     - Verify describe_tool logs operation: "describe_tool" and contributes upstream_state: "not_attempted".

6. Write terminal receipt:
   Create .omp/artifacts/receipt_homelab_T1.json recording task outcome, executed verification commands, and test pass evidence.

# Verification Commands to Run:
go test -v -count=1 ./internal/catalog/...
go test -v -count=1 ./internal/service/... -run 'TestToolSurface(Describe|Call)'
`,
      solutionSpace: "Implement optional catalog_revision admission, format validation, describe requestID extraction, and standardized admission logging"
    }
  ]
};

const built = buildDispatchPayload(seed);
if (!built.ok) {
  console.error("buildDispatchPayload failed:", built);
  process.exit(1);
}
console.log("buildDispatchPayload OK!");
const validated = validateDispatchPayload(built.payload);
console.log("validateDispatchPayload:", validated);

fs.writeFileSync(".scratch/homelab-llm-ergonomics/built-payload-t1.json", JSON.stringify(built.payload, null, 2), "utf8");
fs.writeFileSync(".scratch/homelab-llm-ergonomics/dispatch-request-t1.json", JSON.stringify(seed.workflowDispatch, null, 2), "utf8");

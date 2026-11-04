import fs from "node:fs";
import { buildDispatchPayload, validateDispatchPayload } from "/home/opencode/.omp/agent/workflow-policy/execution-gate.ts";

const paths = [
  "internal/service/tools.go",
  "internal/service/projection.go",
  "internal/service/tools_test.go",
  "internal/service/service_edge_test.go",
  "internal/service/progressive_discovery_test.go",
  "internal/service/progressive_session_test.go",
  "internal/service/progressive_projection_test.go",
  "internal/service/progressive_concurrency_lifecycle_test.go",
  "cmd/mcp-bridge/serve_endpoint_rebind_test.go",
  ".omp/artifacts/receipt_homelab_T2.json"
];
const seed = {
  intent: "Dispatch Task T2 to engineer",
  instructions: "Implement semantic wire name publication and dynamic projection according to .scratch/homelab-llm-ergonomics/spec.md and ticket 02.",
  workflowDispatch: {
    statePath: ".omp/plans/execution-state.json",
    schemaVersion: 1,
    projectRoot: "/home/opencode/projects/default/mcp-bridge",
    feature: "homelab-llm-ergonomics",
    revision: "r7",
    requests: [
      {
        name: "ImplementT2",
        taskId: "T2",
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
      name: "ImplementT2",
      agent: "engineer",
      task: `# Target: Task T2 — Semantic Wire Name Publication and Dynamic Projection
Reference Ticket: .scratch/homelab-llm-ergonomics/issues/02-semantic-wire-name-publication.md
Specification Reference: .scratch/homelab-llm-ergonomics/spec.md (R2, AC-3, Section 3.3, 4/T2)

# Exact Files and Changes:
1. internal/service/tools.go:
   - Add helper function:
     func effectiveWireName(serverID, rawName string) string
     which computes config.WireToolName(serverID, rawName).
     If len(wire) <= 64 and wire contains only valid identifier characters (e.g. letters, digits, hyphens, underscores), return wire.
     Otherwise, return core.PublicName(serverID, rawName).
   - In searchPayload(response search.Response):
     Iterate results and set toolName := effectiveWireName(result.Server, result.UpstreamName) instead of core.PublicName.
   - In resolveBinding(catalog *core.Bridge, toolName string):
     Ensure dual resolution: already supports resolving via config.ParseWireToolName or core.PublicName. Verify both resolve to the underlying binding.

2. internal/service/projection.go:
   - In preparePinProjection(owner projectionOwner, previous *ToolProjection, assembly *Assembly, catalogBindings map[logicalToolTarget]struct{}):
     In the unpinned dynamic interest loop (lines ~320-335), when constructing projectionToolSpec for unpinned tools, use:
     name: effectiveWireName(interest.serverID, interest.rawName)
     instead of core.PublicName(interest.serverID, interest.rawName).

3. Regression & Unit Tests:
   - internal/service/tools_test.go:
     Add assertions verifying tools in search results use <server>__<tool> wire names when <= 64 characters, and fall back to t_<base32> when > 64 chars or invalid chars.
   - internal/service/progressive_discovery_test.go, progressive_session_test.go, progressive_projection_test.go, progressive_concurrency_lifecycle_test.go:
     Update expected tool names for dynamic tools from legacy t_<base32> to wire names <server>__<tool> (e.g. alpha__read).
   - cmd/mcp-bridge/serve_endpoint_rebind_test.go:
     Update awaitDiscovery and callCurrentTool calls to expect wire names for discovered tools.

4. Surgical repair isolated by debugger:
   - In internal/service/service_edge_test.go:476:
     Replace 'catalog_revision: "old"' with a 64-hex stale revision strings.Repeat("b", 64) so that it tests stale catalog revision change rather than malformed hex rejection.

5. Write terminal receipt:
   Create .omp/artifacts/receipt_homelab_T2.json recording task outcome, executed verification commands, and test pass evidence.
# Verification Commands to Run:
go test -v -count=1 ./internal/service/...
go test -v -count=1 ./cmd/mcp-bridge -run 'TestBridgeRebindsServerEndpointOnRestartAndReload'
`,
      solutionSpace: "Implement effectiveWireName with 64-char bound and deterministic hash fallback, update searchPayload and preparePinProjection, and update service/rebind test suites"
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

fs.writeFileSync(".scratch/homelab-llm-ergonomics/built-payload-t2.json", JSON.stringify(built.payload, null, 2), "utf8");
const { statePath, ...reqOnly } = seed.workflowDispatch;
fs.writeFileSync(".scratch/homelab-llm-ergonomics/dispatch-request-t2.json", JSON.stringify(reqOnly, null, 2), "utf8");

# Progressive metadata reveal — SDK registry evidence r4

## Scope

Bounded read-only supplement to `.scratch/progressive-metadata-reveal/spec.md` (`cursor-choice-reconciliation-2`). No source, test, workflow-state or approval changes. No Gemini/client research or runtime probe. Graphify skipped because the worktree is dirty.

The SDK module-cache source was unavailable through the project-scoped codebase searcher, so the fallback was limited to LSP-located Go SDK v1.8.0 symbols and exact reads under `/home/opencode/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.8.0/mcp/`.

## Verified facts

- `mcp/server.go:315-363`: `Server.AddTool` validates and constructs a `serverTool`, then invokes `changeAndNotify` for that one registry mutation. `featureSet.add` replaces a tool with the same name. The SDK source itself notes batching as a TODO (`:358-362`).
- `mcp/server.go:613-615`: `Server.RemoveTools` invokes a separate `changeAndNotify` call. Each `changeAndNotify` acquires/releases `Server.mu` (`:747-765`). There is no public batch API in the inspected methods.
- `mcp/server.go:979-995`: one `tools/list` result is constructed under `Server.mu`; the result appends existing `*Tool` pointers. This lock does not span multiple registry API calls or multiple pagination requests.
- Notification scheduling is asynchronous/debounced (`mcp/server.go:739-765`); notification delivery gathers sessions under the SDK mutex and sends after unlocking (`:775-803`). A delay is not a commit barrier.
- `Server.AddReceivingMiddleware` is a public wrapper seam (`mcp/server.go:1851-1864`). It can wrap receiving-method dispatch, but the existence of this API alone does not establish a safe application-wide publication transaction.

## Consequence (inference)

The SDK serializes each individual add/remove/list operation, not a batch of several mutations. Without an additional bridge barrier shared by every relevant list/call reader and publication writer, a concurrent list can observe a state between those operations. A bridge-owned barrier could potentially hide intermediate registry states from participating requests, but these reads do not prove coverage of all tools/list paths, notification ordering, runtime acceptance, route admission, pagination generations, or session ownership.

Therefore B2 remains open; no infeasibility verdict is established. The new evidence narrows it: the proposed design cannot rely on SDK-level batch atomicity, `Server.mu` alone, or notification debounce. B4 remains a separate per-session owner/lifecycle question. No tests, builds, race probes or wire probes were run.
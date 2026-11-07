# Specification: Gateway Built-in Discovery Tool Annotations

Status: ready-for-agent

## Problem Statement

When client agents (such as Gemini CLI or other orchestrators with client-side execution safety gates) connect to `mcp-bridge`, invoking bridge-owned discovery tools (`search_tools` and `describe_tool`) may trigger interactive confirmation prompts or warnings.

In the current implementation:
- `search_tools` and `describe_tool` omit `destructiveHint` from their `mcp.ToolAnnotations` declaration. Because the Go MCP SDK applies `omitempty` to `*bool` fields, `destructiveHint` is completely omitted from the serialized JSON payload.
- MCP client implementations that default missing `destructiveHint` fields to `true` (per MCP schema specification convention) observe an apparent contradiction (`readOnlyHint: true`, `destructiveHint: true`), causing automated execution gates to halt for human confirmation on read-only catalog discovery calls.
- In addition, both `search_tools` and `describe_tool` currently set `openWorldHint: true` despite executing entirely against the bridge's local, in-memory catalog cache and schemas.

## Solution

Explicitly configure accurate, non-conflicting advisory annotations on the bridge's built-in discovery tools (`search_tools` and `describe_tool`):
- `readOnlyHint`: `true`
- `destructiveHint`: `false`
- `idempotentHint`: `true`
- `openWorldHint`: `false`

The generic gateway wrapper `call_tool` retains its conservative fallback classification (`destructiveHint: true`, `readOnlyHint: false`/unspecified, `idempotentHint: false`, `openWorldHint: true`), because it can multiplex arbitrary upstream mutations. Upstream and dynamically revealed tools continue to preserve their source annotations and operator-configured overrides without synthetic normalization or hostname-based inference.

<!-- specdd:requirements:start -->
## User Stories

1. As an autonomous AI agent, I want `search_tools` to explicitly declare `destructiveHint: false` and `openWorldHint: false`, so that client-side safety gates do not infer destructive execution when querying the tool catalog.
2. As an autonomous AI agent, I want `describe_tool` to explicitly declare `destructiveHint: false` and `openWorldHint: false`, so that inspecting an indexed tool's schema does not halt orchestration for confirmation prompts.
3. As a developer integrating `mcp-bridge` with strict MCP clients, I want discovery tool annotations to be non-contradictory (`readOnlyHint: true` accompanied by `destructiveHint: false`), so that schema validators and execution planners accept discovery calls as safe.
4. As an operator of `mcp-bridge`, I want `call_tool` to remain classified as potentially destructive and open-world, so that generic, unpinned tool invocations are not treated as universally safe.
5. As an operator configuring server metadata overrides, I want upstream tools to retain their exact source annotations and configured overrides, so that the bridge does not alter upstream tool definitions or invent assumptions about closed environments.
6. As a security auditor, I want metadata annotations to remain strictly advisory hints, so that bridge execution admission, authentication, and lease controls are never bypassed or relaxed based on annotation hints.
<!-- specdd:requirements:end -->
<!-- specdd:design:start -->
## Implementation Decisions

1. **Built-in Tool Annotation Definitions:**
   Update the static tool registrations for `search_tools` and `describe_tool` in the service registration layer so that their advisory annotations explicitly include:
   - `ReadOnlyHint: true`
   - `DestructiveHint: boolPtr(false)`
   - `IdempotentHint: true`
   - `OpenWorldHint: boolPtr(false)`

2. **No Heuristic Annotation Rewriting:**
   The bridge will not synthesize `destructiveHint: false` or `openWorldHint: false` for upstream or dynamically exposed tools based on endpoint URLs, hostnames, or `readOnlyHint` state. Upstream tools continue to pass through normalized source metadata and explicit operator configuration overrides via the existing layered metadata system (`source -> server_default -> tool`).

3. **Preserve `call_tool` Classification:**
   The generic `call_tool` wrapper remains unchanged with conservative advisory hints (`destructiveHint: true`, `idempotentHint: false`, `openWorldHint: true`).

4. **Documentation Alignment:**
   Update documentation tables and test fixtures that assert the advisory hint presence on `search_tools` and `describe_tool` to reflect the explicit `destructiveHint: false` and `openWorldHint: false` contract.
<!-- specdd:design:end -->
<!-- specdd:tasks:start -->
## Testing Decisions

1. **Good Test Standard:**
   Tests must exercise external HTTP JSON-RPC behavior on the real `/mcp` endpoint (`tools/list` and `tools/call`). They must assert wire-level JSON properties rather than unexported Go struct internals.

2. **Wire-Level Acceptance Seam:**
   - Extend `cmd/mcp-bridge/serve_hybrid_metadata_test.go` and related test suites to assert that `tools/list` returns the updated explicit hints for `search_tools` and `describe_tool` (`readOnlyHint: true`, `destructiveHint: false`, `idempotentHint: true`, `openWorldHint: false`).
   - Verify that `call_tool` retains its conservative hint payload.
   - Verify that upstream tool metadata and override layers remain unaffected.

3. **Regression Safety:**
   Run the complete Go test suite (`go test ./...`) to ensure that existing progressive reveal, session projection, and metadata provenance invariants pass with exit code 0.
<!-- specdd:tasks:end -->
<!-- specdd:envelope:start -->
## Out of Scope

- Client-side execution gate implementations (e.g., changes to Gemini CLI or third-party client prompt policies).
- Automatic mutation or inference of upstream tool annotations based on IP address, private domain names, or schema inspection.
- Changes to the JSON-RPC wire protocol, MCP SDK data structures, or session authentication/lease models.
- Guaranteeing that third-party client gates will suppress interactive prompts, as prompt policies are client-governed.

## Further Notes

Metadata annotations (`readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`) are strictly advisory hints defined by the MCP specification. They do not grant execution authority or bypass any server-side validation or authorization rules.
<!-- specdd:envelope:end -->

# Gateway Tool Annotations

Effort map for the gateway built-in discovery tool annotation alignment.

## Spec
- [.scratch/gateway-tool-annotations/spec.md](spec.md)

## Tickets
1. [issues/01-explicit-annotations-built-in-discovery-tools.md](issues/01-explicit-annotations-built-in-discovery-tools.md) — Status: `ready-for-agent`
2. [issues/02-regression-and-invariant-verification.md](issues/02-regression-and-invariant-verification.md) — Status: `ready-for-agent` (Blocked by: 01)

## Decisions-so-far
- Built-ins (`search_tools`, `describe_tool`): explicitly `readOnlyHint: true`, `destructiveHint: false`, `idempotentHint: true`, `openWorldHint: false`.
- Upstream tools: preserve source annotations and operator-configured overrides without heuristic rewriting.
- Generic wrapper `call_tool`: preserve conservative fallback classification (`destructiveHint: true`, `idempotentHint: false`, `openWorldHint: true`).

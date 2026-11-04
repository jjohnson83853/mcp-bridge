## Code Review Summary

**Scope:** r3 specification and ticket strategy—not an implementation sign-off.
**Documents reviewed:** 2, with supporting source tracing.
**Total issues:** 0 — CRITICAL: 0 / HIGH: 0 / MEDIUM: 0 / LOW: 0.

### Stage 1 — Specification Compliance

1. **Seam validity: PASS.**
   `cmd/mcp-bridge/wiring.go:175` is the correct construction boundary. `ServerStateless()` returns `true` when omitted and preserves explicit `false` (`internal/config/config.go:47–51`). `NewDedicated` stores `DedicatedOptions.Stateless` directly, and discovery/tool execution consume it (`internal/upstream/dedicated.go:54,100–113,224–257`). The proposed line activates existing behavior rather than introducing another session-management mechanism.

2. **Deterministic restart oracle: PASS as a design.**
   `spec.md:29–36` replaces both the server and its session-owning handler while preserving the endpoint. The `"before"`/`"after"` assertions prove execution against the new generation; execution counts guard against duplicate dispatch. This matches the existing dedicated-runtime fixture (`internal/upstream/dedicated_test.go:250–318`).

3. **Explicit stateful preservation: PASS.**
   Parsed YAML with `stateless: false`, two successful calls, one initialization, and two executions (`spec.md:37–39`) tests the configuration-to-runtime boundary—not merely the constructor. Explicit `false` retains the existing `sessionFor` reuse path.

### Stage 2 — Concurrency, Edge Cases, and Coverage

- The handler snapshot under `RLock`, followed by unlocking **before** `ServeHTTP`, correctly avoids blocking swaps behind long-lived HTTP/SSE requests (`spec.md:31`).
- Implementation should retain the referenced fixture’s **stateful upstream handler**, atomic counters, and generation-local state captured by callbacks. A stateless peer could mask RED.
- Await the first response before swapping; validate a non-null success result and absence of a JSON-RPC error, not only a default-valued `isError`.
- Spec and ticket use the same full-suite race/coverage command. `cmd/covcheck/main.go:23,59–62` enforces aggregate source-line coverage **≥90.0%**. Focused test success alone cannot satisfy AC4.

**VERDICT: pass**

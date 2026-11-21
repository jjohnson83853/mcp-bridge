# 01 — Breaker Regains Reachable Close Path via Discovery Probe

**What to build:**
Allow a latched open circuit breaker on an upstream server to recover when the upstream service returns to health, without requiring a process restart. When a server's breaker has tripped open after reaching its failure threshold and its configured open duration (`openDuration`) has elapsed, the non-executing discovery/refresh leg (`Dedicated.Discover`) claims the single half-open probe via `client.BeginProbe()`. If discovery succeeds, the probe callback closes the breaker and resets the failure counter, admitting subsequent tool calls. If discovery fails, the breaker remains open and resets its backoff timer. Closed breakers and standard tool executions remain unchanged. Tool calls never act as execution probes (zero replay of unadmitted executions). State the boundary regarding stateful sessions (`stateless: false` cached session eviction remains tracked under backlog ticket 2).

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] AC-1: `Dedicated.Discover` checks if the upstream breaker is open and `openDuration` has passed, claiming the half-open probe via `BeginProbe()` before querying tools.
- [ ] AC-2: A successful discovery probe executes the completion callback with `true`, closing the breaker (`openedAt.IsZero()`, `halfOpen == false`, `failures == 0`).
- [ ] AC-3: A failed discovery probe executes the completion callback with `false`, keeping the breaker open and updating `openedAt` to the current time.
- [ ] AC-4: Regular tool calls (`AdmitCall` / `CallTool`) remain strictly non-probing; they never execute or replay tool calls while the breaker is open or half-open.
- [ ] AC-5: In a full service lifecycle test, after 3 transport failures latch the breaker and `openDuration` elapses, a subsequent catalog refresh pass (`Refresher.runPass()`) successfully probes the upstream, closes the breaker, and allows subsequent tool calls to succeed without a process restart.
- [ ] AC-6: When `stateless: true` (the production default), the discovery probe creates a clean ephemeral connection and does not rely on a potentially poisoned session.
- [ ] AC-7: Full repository test suite passes clean (`go test -race -p 1 ./...`) and aggregate coverage gate meets >= 90.0%.

# 05: Diagnose streaming, timeouts, cancellations, and disconnects

**What to build:** Operators can tell a long-lived HTTP stream from slow MCP/backend execution and distinguish a deadline, explicit cancellation, observed disconnect or unknown failure. Completion visibility does not depend on closing the transport first.

**Blocked by:** 03 — Distinguish protocol, tool-admission, and backend failures.

Status: complete

Type: task

Source: Logging observability specification, revision LOG-OBS-r4 (supersedes the original r1 source reference; approved ticket scope and dependencies unchanged). Coverage: AC10, AC13; relevant AC4, AC8, AC14–AC22 and AC24. This ticket builds on 03's independent outcome and typed failure classification. Ticket publication approval does not authorize implementation; applicable project gates still apply.

- [ ] A decoded MCP operation and its actual backend dispatch each emit their own terminal duration/completion promptly, while the enclosing HTTP stream may remain open. Releasing the stream later produces its separate HTTP lifetime record. Boundary records may arrive out of order; do not buffer them or delay response delivery to manufacture ordering.
- [ ] Explicit deadline expiration maps to deadline_exceeded, explicit protocol cancellation to request_cancelled, and an observed transport close/write failure to client_disconnected only where the transport evidence supports it. Bare context cancellation is not proof of client disconnection. Unknown causes retain an honest finite unknown/cancellation classification at the observed stage.
- [ ] A committed HTTP 200 followed by failed delivery retains observed status 200 with incomplete transport outcome; it does not rewrite the earlier RPC/tool/backend facts. Missing response evidence remains unavailable rather than an invented status or result. Rejections still have no backend attempt, and no terminal event is duplicated within a boundary.
- [ ] Preserve actual SDK session/context lifetimes, flushing, required writer interfaces, notification behavior and correlation parents. A stream carrying another operation cannot reattribute completion using whichever client ID was last observed. Authentication, networking, cancellation, timeout, retry and public-response policies are unchanged.
- [ ] The existing real HTTP/MCP harness and local fake upstreams use independent operation-finished and stream-release barriers. Captured logs prove operation/backend completion before HTTP completion and the longer HTTP lifetime after release. Use ordering and generous duration bounds, not exact milliseconds or sleeps as proof.
- [ ] Separate public timeout, explicit protocol cancellation, observed socket close/write failure and unexplained cancellation fixtures prove the finite classifications and unchanged public behavior. Do not force client_disconnected where the SDK exposes only cancellation. Synthetic secrets in cancellation details, error text, headers and in-flight arguments/results never appear in logs.
- [ ] Update operator documentation/changelog with stream-versus-operation timing, incomplete delivery and cancellation/disconnection interpretation. Retain deterministic RED/GREEN evidence and a real local streaming smoke exercise, run relevant full-suite/race checks with at least 90% full-suite source-line coverage, and complete applicable code/Go/security review. Existing metrics, diagnostics and logging-writer failure behavior remain unchanged; no payload buffering or unbounded correlation registry is added.

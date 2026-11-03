# Feature Map: ChatGPT Session Admission Observability & Diagnostic Transparency (`chatgpt-session-admission-diagnostics`)

- **Effort:** `chatgpt-session-admission-diagnostics`
- **Owner:** Coordinator
- **Status:** ready-for-agent
- **Revision:** r10

## Summary
Eliminate diagnostic opacity during ChatGPT action discovery failures by replacing generic `failure_reason: "credentials_missing"` and `session_state: "not_evaluated"` with typed, differentiated session admission outcomes (`session_absent`, `session_fingerprint_mismatch`, `jwt_expired`, `auth_mode_unavailable`), verifying handshake lifecycle evidence, preserving the fixed 64-slot schema, asserting zero credential leakage across 401/403/404 surfaces, and ensuring deterministic composed wire verification.

## Tickets
1. `issues/01-session-admission-result-taxonomy.md` (T1) — Typed Session Admission Seam & Outcome Classification. Boundary `!now.Before(deadline)` verified before/at/after. Preserves `SessionClaims`. Blocked by: None.
2. `issues/02-middleware-differentiated-rejection-diagnostics.md` (T2) — Differentiated Session Rejections, Pseudonyms & Pre-Auth Method Peeking. On-the-fly `session_ref` for unknown IDs, 256-byte payload peek into `operation`, generic 401 preservation. Blocked by: T1.
3. `issues/03-initialize-handshake-lifecycle-diagnostics.md` (T3) — Initialize Handshake Lifecycle Evidence & Stateless Representation. Single terminal completion record, stateless `session_binding: "unavailable"`. Blocked by: T1.
4. `issues/04-end-to-end-discovery-wire-verification-and-coverage.md` (T4) — Composed Wire Verification, Zero Leakage Across 401/403/404 & Triage Runbook. Validates post-OAuth discovery, non-zero 403/404 surfaces, zero credential leakage, `>= 90.0%` coverage gate, and runbook documentation. Blocked by: T2, T3.

## Decisions
- `D1`: Wire secrecy: HTTP responses remain strictly generic 401s (`"unauthorized\n"` and standard `WWW-Authenticate` challenge). No session state or failure reasons leak to the client.
- `D2`: Zero-schema-edit mapping: reuses existing allowlisted enums under `failure_stage: "session"` (`session_absent`, `session_fingerprint_mismatch`, `jwt_expired`, `auth_mode_unavailable`).
- `D3`: Correlation without leakage: salted one-way 8-character pseudonym (`session_ref`, e.g. `s_...`) logged for all requests (including on-the-fly generation for unknown IDs).
- `D4`: Pre-auth transport peeking: bounded 256-byte peek into `decision.Operation` (slot 20) with `io.MultiReader` rewind (`"sse_listen"`, `"http_probe"`, or JSON-RPC method).
- `D5`: The expiry condition is settled intentionally as `!h.now().Before(rec.authDeadline)`, rejecting requests at or after the exact deadline, with explicit before/at/after unit testing.
- `D6`: Composed wire verification in `cmd/mcp-bridge/serve_oauth_test.go` verifies physical stdout JSON records, non-zero 401/403/404 surfaces, and zero token/hash/private-key leakage.
- `D7`: Zero-cost operational runbook section added to `docs/runbooks/chatgpt-integration.md` with copy-paste LogQL queries (`failure_stage="session"`, `session_ref="s_..."`).

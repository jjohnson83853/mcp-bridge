# 02 — Differentiated Session Rejections, Pseudonyms & Pre-Auth Method Peeking

**What to build:**
When a client sends a request carrying an `Mcp-Session-Id` header without an `Authorization` header, the authentication middleware classifies the failure under `failure_stage: "session"` and logs the exact root cause (`session_absent`, `session_fingerprint_mismatch`, `jwt_expired`, or `auth_mode_unavailable` when admission is disabled). Computes an on-the-fly salted `session_ref` for unknown session IDs so multi-request bursts from dead sessions correlate in logs without leaking raw credentials. Peeks up to 256 bytes of payload to record the JSON-RPC method (or `"sse_listen"` / `"http_probe"`) in the `operation` diagnostic slot without corrupting stream consumption. Preserves byte-exact generic HTTP 401 `"unauthorized\n"` response bodies and standard `WWW-Authenticate` challenges.

**Blocked by:** 01 — Typed Session Admission Seam & Outcome Classification

**Status:** closed

- [x] Requests with unknown session ID emit `failure_stage: "session"`, `failure_reason: "session_absent"`, `session_header: "present"`, `session_state: "absent"`.
- [x] Requests with unauthenticated session ID emit `failure_stage: "session"`, `failure_reason: "session_fingerprint_mismatch"`, `session_header: "present"`, `session_state: "existing"`.
- [x] Requests with expired session ID emit `failure_stage: "session"`, `failure_reason: "jwt_expired"`, `session_header: "present"`, `session_state: "existing"`.
- [x] Requests with session ID when admission is disabled emit `failure_stage: "session"`, `failure_reason: "auth_mode_unavailable"`, `session_header: "present"`, `session_state: "existing"`.
- [x] Requests with session ID when running in stateless mode emit `failure_stage: "session"`, `failure_reason: "session_absent"`, `session_header: "present"`, `session_state: "unavailable"`.
- [x] Unknown session IDs produce a consistent, salted 8-character `session_ref` (e.g. `s_...`) across identical requests.
- [x] Payload peeking extracts the JSON-RPC `"method"` into `decision.Operation` without corrupting request body consumption for downstream handlers; falls back to `"sse_listen"` for GET and `"http_probe"` for empty POSTs.
- [x] All rejected tokenless requests return verbatim `"unauthorized\n"` and standard `WWW-Authenticate` challenge headers on the HTTP wire.
- [x] Unit tests in `internal/auth/middleware_test.go` and `cmd/mcp-bridge/serve_test.go` assert these behaviors, verifying pre-fix RED and post-fix GREEN.

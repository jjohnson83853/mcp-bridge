# 04 — Composed Wire Verification, Zero Leakage Across 401/403/404 & Triage Runbook

**What to build:**
Composed loopback wire tests in `cmd/mcp-bridge/serve_oauth_test.go` validating the complete post-OAuth discovery flow against the real HTTP server. Exercises stateful handshakes, tokenless discovery admission, differentiated 401 rejections, 403 scope rejections, 404 route rejections, and zero credential leakage across rejected bodies, headers, and captured stdout diagnostic JSON records. Enforces aggregate `>= 90.0%` test coverage and publishes the triage runbook with copy-paste LogQL queries in `docs/runbooks/chatgpt-integration.md`.

**Blocked by:** 02 — Differentiated Session Rejections, Pseudonyms & Pre-Auth Method Peeking, 03 — Initialize Handshake Lifecycle Evidence & Stateless Representation

**Status:** closed

- [x] Wire test verifies probe without session ID returns HTTP 401 with `failure_stage: "outer_oauth"`, `failure_reason: "credentials_missing"`, `session_header: "absent"`.
- [x] Wire test verifies valid OAuth `initialize` handshake returns HTTP 200 with `Mcp-Session-Id` and `session_state: "created"`.
- [x] Wire test verifies follow-up tokenless discovery burst returns HTTP 200 with `session_header: "present"`, `session_state: "existing"`, `binding_relation: "match"`.
- [x] Wire test verifies unknown session ID returns HTTP 401 with `failure_stage: "session"`, `failure_reason: "session_absent"`.
- [x] Wire test verifies unauthenticated session ID returns HTTP 401 with `failure_stage: "session"`, `failure_reason: "session_fingerprint_mismatch"`.
- [x] Wire test verifies expired session ID returns HTTP 401 with `failure_stage: "session"`, `failure_reason: "jwt_expired"`.
- [x] Wire test exercises concrete 403 (insufficient scope) returning `"forbidden: insufficient scope\n"` and 404 (authenticated nonexistent route) returning 404, asserting nonzero counts for each status.
- [x] Wire assertions scan all rejected bodies, headers, and stdout JSON records, confirming 0 occurrences of fixture bearer tokens, raw session IDs, fixture RSA private keys, and SHA-256 hashes.
- [x] Full workspace test suite passes under `go test -race ./...` and meets aggregate line coverage `>= 90.0%` (`cmd/covcheck`).
- [x] Operational runbook section published in `docs/runbooks/chatgpt-integration.md` with copy-paste LogQL queries for session triage and correlation.

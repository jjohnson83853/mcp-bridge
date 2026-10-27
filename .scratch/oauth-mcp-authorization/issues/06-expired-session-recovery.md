# 06: Recover expired or unknown MCP sessions without an authentication loop

Status: needs-triage
Type: task
Classification: confirmed MCP protocol defect; linkage to the reported OMP incident is not yet proven.

## What to build

After successfully authenticating a request, return the standard recoverable HTTP 404 for a missing or expired MCP session. Any compliant client can then initialize a fresh session instead of treating the stale session as a credential failure and requiring a manual reconnect. Preserve authentication and isolation of existing sessions.

## Evidence

The current admission code gives both a missing/reaped session and an existing session's identity mismatch the same HTTP 401. Idle expiry removes session records. The MCP session-management contract requires 404 after session termination and client reinitialization on that response. Existing focused session checks pass for current behavior, not the corrected protocol outcome.

See [diagnosis and verification evidence](../authentication-recovery-evidence.md). This may explain reconnect-only recovery, but incident logs do not distinguish the failed branch.

## Acceptance criteria

- [ ] A caller with valid credentials using a session that was idle-expired, explicitly removed or lost receives HTTP 404, without executing an upstream operation.
- [ ] The client can respond with a new InitializeRequest without a stale session header and resume read-only tools/list calls without manual credential re-entry.
- [ ] Invalid or expired credentials still receive the appropriate authentication failure before session recovery; insufficient scope still receives 403.
- [ ] An existing session belonging to a different authorized identity remains inaccessible. Do not turn all fingerprint mismatches into successful access or a blanket recovery bypass.
- [ ] Static-token session isolation and concurrent independent clients remain protected.
- [ ] Deterministic regression coverage exercises the real composed HTTP/auth/session boundary with a controlled clock, proves the current wrong status before the change, and passes after it. Cover idle expiry, unknown session, successful reinitialization and rejected credentials.
- [ ] Behavior is protocol-based for every client, with no OMP/Gemini user-agent special cases, forced container restart, disabled authentication or inflated session lifetime.

## Blocked by

None (ticket dependencies). Triage and applicable implementation/review gates remain required.

## Comments

2026-10-26: Published from debugger diagnosis. User reports that reconnecting OMP alone restores access temporarily and Gemini does not show the problem. No implementation or authorization-policy change has been made.

# ChatGPT MCP Bridge Integration & Troubleshooting Runbook

## Overview
The `mcp-bridge` supports session-bound OAuth admission when `MCP_SESSION_ADMISSION_ENABLED=true`.
ChatGPT completes OAuth during the initial `initialize` handshake, obtaining an `Mcp-Session-Id`. Subsequent discovery requests (`tools/list`, `notifications/initialized`) carry that session header without repeating the Bearer token.

## Triage LogQL Queries for Loki (datasource: Loki / UID P8E80F9AEF21F6940)

### 1. Unknown Session Bursts (session_absent)
Occurs when follow-up requests arrive with an unknown or evicted `Mcp-Session-Id`:
```logql
{source_subsystem="mcp-bridge"} |= `"failure_reason":"session_absent"`
```

### 2. Expired Session Bursts (jwt_expired)
Occurs when follow-up requests arrive after the authenticated session's 8-hour admission deadline has elapsed:
```logql
{source_subsystem="mcp-bridge"} |= `"failure_reason":"jwt_expired"`
```

### 3. Insufficient OAuth Scope (scope_insufficient)
Occurs when the OAuth token lacks the required scope (e.g., `mcp:connect`):
```logql
{source_subsystem="mcp-bridge"} |= `"failure_reason":"scope_insufficient"`
```

### 4. Unauthenticated Session Rejection (session_fingerprint_mismatch)
Occurs when a tokenless request carries an `Mcp-Session-Id` that was not created under an authenticated OAuth session:
```logql
{source_subsystem="mcp-bridge"} |= `"failure_reason":"session_fingerprint_mismatch"`
```

### 5. Session Correlation via Pseudonym Reference
Link a session handshake and subsequent requests without exposing raw session identifiers:
```logql
{source_subsystem="mcp-bridge"} |= `"session_ref":"<session_ref>"`
```

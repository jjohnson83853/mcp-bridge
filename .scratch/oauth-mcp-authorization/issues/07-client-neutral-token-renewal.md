# 07: Support secure token renewal without trapping MCP clients in session-authentication failures

Status: needs-triage
Type: research
Classification: confirmed bearer-fingerprint behavior; suspected OAuth interoperability defect, not a proven OMP refresh failure.

## What to build

Establish and implement a secure, client-neutral OAuth renewal/session-recovery contract. A client presenting a newly valid access token for the same approved authorization context must be able to continue or recover its MCP session without repeated manual reconnects. Do not remove the protections that prevent a different identity from using an existing session.

## Evidence

Sessions currently bind to the exact bearer-token bytes. A renewed access token with different bytes therefore fails the existing session check even if OAuth validation accepts it. Static-token fingerprint isolation is intentional, tested, and part of the previously approved design. Reconnecting OMP replaces connection state and may replace both bearer and session; the live incident does not prove whether refresh was missed, completed, or unrelated. Gemini's different observed behavior is not proof of cause.

See [diagnosis and verification evidence](../authentication-recovery-evidence.md).

## Acceptance criteria

- [ ] First reproduce the mechanism at the composed HTTP boundary: initialize with synthetic valid token A, then use token B with different bytes but the same proposed authorized context on the existing session. Record the observed status without involving a real issuer or real credentials.
- [ ] Record a reviewed identity/recovery policy distinguishing static tokens from OAuth. Consider issuer, principal, resource/audience, authorized client and current authorization context; do not bind blindly to subject alone or silently weaken previously approved affinity.
- [ ] Under the approved policy, valid same-context renewal either continues the session or follows a standard automatic reinitialization path. It does not produce a persistent 401/reconnect loop.
- [ ] Validate credentials and current authorization on every request. Invalid/expired tokens remain rejected, missing required scope remains 403, and a changed issuer/principal/client/resource or incompatible authorization context cannot inherit another session's access.
- [ ] Existing static-token isolation remains intact; two clients' independent sessions do not become interchangeable.
- [ ] Client-neutral wire regressions cover renewal while retaining a session, renewal with a new session, stale-session recovery, invalid renewal and concurrent independent clients. No branching on client name or user agent.
- [ ] If actual OMP/Gemini smoke is safely available, record product/version and read-only recovery behavior. Otherwise mark it NOT RUN; do not substitute a claim that all real clients were tested.
- [ ] Determine the reported incident's failing branch using sanitized request-path/challenge and renewal/session-change evidence if obtainable. Do not claim OMP's refresh implementation is broken from reconnect recovery alone.
- [ ] Any approved implementation uses test-first behavior evidence and security review; no trust bypass, secret logging, lifetime-extension workaround or container restart dependency.

## Blocked by

None (ticket dependencies). Secure policy selection, independent review and applicable approvals are prerequisites to implementation; this research ticket is not an approved source change. Session-expiry status correction is tracked independently in ticket 06.

## Comments

2026-10-26: User explicitly requires that a token-refresh solution protect other clients as well as OMP. This is a generic OAuth/MCP contract investigation, not an OMP-specific patch. The local executable reports omp/18.12.6; the active process build and Gemini version are not independently established.

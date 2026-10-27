# 08: Keep authentication metadata challenges reachable for path-bearing resource URLs

Status: needs-triage
Type: task
Classification: code-supported conditional routing defect; deployed configuration applicability and incident linkage are unverified.

## What to build

Ensure that the protected-resource metadata URL advertised during authentication is publicly reachable and describes the configured resource, including a resource URL whose path ends in an MCP endpoint. Clients must not follow the challenge into another authentication failure instead of discovering how to authenticate.

## Evidence

Challenge construction appends the well-known metadata path to the configured resource URL. Serving composition registers the public metadata handler only at the root well-known path. For a path-bearing resource URL these paths differ, and the advertised path falls through to normal inbound admission. The deployment's effective resource URL was not captured; this finding is not asserted as the cause of OMP's intermittent failures.

See [source-backed diagnosis and limitations](../authentication-recovery-evidence.md).

## Acceptance criteria

- [ ] Configure synthetic origin-only and path-bearing resource URLs; request the protected MCP endpoint without credentials and capture its complete authentication challenge.
- [ ] Following the exact advertised resource-metadata URL without credentials returns HTTP 200 metadata describing the configured resource, not another 401 or a route failure.
- [ ] Metadata resource identity and authorization-server information remain accurate, including existing trailing-slash normalization.
- [ ] The protected MCP endpoint still requires valid authorization. Public metadata routing must not expose MCP tool calls or bypass session/authentication checks.
- [ ] Prove the current path mismatch and corrected behavior through the real composed HTTP routing/auth boundary using local fixtures, with no real issuer or credentials.
- [ ] Behavior is compatible with any client following the advertised challenge; do not special-case OMP or Gemini.

## Blocked by

None (ticket dependencies). Confirm supported configuration assumptions during triage and satisfy applicable implementation/review gates before source changes.

## Comments

2026-10-26: Found during recurring OMP reconnect diagnosis. No live metadata URL probe or production-configuration claim was made; no code or configuration has been changed.

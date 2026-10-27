# Recurring OMP authentication/session failure — diagnosis

Recorded 2026-10-26. Investigation only: no application, test, credential, deployment or authorization-policy changes. Debugger: `RecurringAuthDebugger`.

## Conclusion and confidence

**Confirmed protocol defect:** the bridge returns HTTP 401 for an authenticated request whose MCP session is missing or idle-expired, rather than the HTTP 404 that enables standard MCP session reinitialization. This can affect any compliant stateful HTTP client, not only OMP.

**Confirmed behavior, suspected OAuth interoperability defect:** session identity is the SHA-256 fingerprint of the exact bearer bytes. A different valid access token therefore cannot reuse that session, even if it represents the same authorized identity. Static-token isolation is intentional and tested; the existing OAuth task map explicitly records prior approval of bearer-fingerprint binding. A secure renewal policy needs review, not an unapproved switch to subject-only binding.

**The cause of these particular OMP failures is not proven.** Reconnecting OMP is the only observed restorative action. Reconnect can replace credentials and session state, so it cannot by itself distinguish missed token refresh, token rotation, or a stale session. There is no evidence that a container restart is required. Gemini reportedly does not exhibit the problem; its actual product/version and renewal/session behavior were not captured.

## Observed evidence

- Mounted `tool/search_tools` and `tool/call_tool` repeatedly returned transport-stage HTTP 401, `WWW-Authenticate: Bearer`, with no bridge tool-result envelope. Representative traces: `b7f7c64ff2f58c98d91af8c9599161f4` and `3897b49d046fdb112948ec13fcec3e59`. If this is the complete challenge, it matches inner admission more closely than the outer OAuth challenge; transport rendering completeness is not independently established.
- After the user's latest reconnect, the parent successfully called search_tools for codebase-searcher; backend fresh/available at `2026-10-26T14:43:29.963673444Z`. Subsequent debugger source calls and parent observability calls succeeded. No failure was forced against the live authenticated connection.
- Loki datasource `P8E80F9AEF21F6940`, exact-service query `{service_name="vector-logs"} | json | service="mcp-bridge"`, UTC range 12:45–14:45 on 2026-10-26, limit 8: eight returned rows had HTTP 401. Latest returned row: `2026-10-26T14:43:18.705134Z`. This is a bounded sample, not an incident-wide count. Normalized records retain status and the fixed completion message, but not the admission reason/stage needed to distinguish rejected credentials from session rejection. Structured metadata only exposed detected_level. No raw credentials, session IDs or payloads were output or retained here.
- The runtime log image tag is `registry.localdomain:5000/mcp-bridge:latest`; this does not identify the deployed source revision.
- `omp --version` returned `omp/18.12.6`. This identifies the local executable, not independently the active process build or Gemini version.

## Source-backed mechanisms

Debugger used bounded MCP source retrieval and LSP; source calls succeeded. Graphify skipped because no verified graph path and planning changes make clean-tree gating unavailable. No post-denial local-source fallback was used in this investigation.

1. `internal/service/inbound.go:330-343`: session validation combines existence and bearer-fingerprint equality; both absent session and existing-session identity mismatch receive HTTP 401, bare Bearer challenge and session_auth_failed.
2. `internal/service/inbound.go:424-436`: idle reaping deletes session records. A subsequent authenticated request therefore enters the absent-session branch above.
3. `internal/service/inbound.go:603-620`: validated OAuth claims can pass credential admission, but session identity still derives from bearer bytes. Passing OAuth validation does not preserve the old session after token-byte rotation.
4. `internal/service/inbound_test.go:101-145`: static token-two is rejected while token-one continues. This is an isolation contract, not evidence that OAuth renewal should be rejected.
5. `internal/service/inbound_bounds_test.go:182-202`: existing idle-expiry mechanism coverage. Its current expected status is not proof of MCP protocol correctness.
6. `internal/auth/middleware.go:99-102,121-148,162-163`: outer OAuth rejection emits realm/resource_metadata; JWT issuer/time/audience validation and scope enforcement precede claim propagation. Insufficient scope returns 403. JWT rejection is a distinct possible cause from stale-session admission.
7. `internal/auth/jwks.go:38-49`: JWKS cache has a 15-minute minimum refresh interval; no stale-key incident was established.
8. `internal/auth/middleware.go:31-32` and `cmd/mcp-bridge/serve.go:150,162-164`: challenge construction appends the metadata path to the resource URL, while the server registers the metadata handler only at the root well-known path. For a path-bearing resource URL, these paths disagree. Whether the deployed configuration uses such a resource URL is unknown; no credential-free live metadata URL was available for a probe.

Primary transport requirement: [MCP 2025-11-25, Session Management](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports#session-management): a terminated session must receive 404; a client receiving 404 for its session must send a fresh InitializeRequest. The bridge must authenticate the caller before applying session recovery and must retain cross-identity isolation.

## Executed verification

- Live post-reconnect mounted search_tools: PASS; this exercises the actual connection that previously failed.
- `go test ./internal/service -run '^(TestSessionIdleExpiry|TestSessionIdleWithinWindow|TestSessionBoundToTokenFingerprint)$' -count=1`: PASS, exit 0, package 0.008s. Existing current-behavior checks only; not an OAuth renewal regression, a fix, or proof of the live incident cause.
- `omp --version`: PASS, `omp/18.12.6`.
- New regression tests, full suite/race/vet/coverage, synthetic renewed-JWT wire matrix, and actual OMP/Gemini differential recovery: NOT RUN. No implementation was requested in this diagnosis slice.

## Ranked hypotheses and distinguishing predictions

1. **Stale/lost/idle-expired MCP session:** after successful credential validation, the request has no admitted session. A branch-specific session-admission record or controlled idle-expiry wire repro distinguishes it; recovery404 should trigger client reinitialization without reauthentication.
2. **Valid access-token rotation with retained session:** token renewal succeeds but bearer bytes change while the session is retained. Two synthetic valid JWTs with the same approved authorization context should reproduce the fingerprint mismatch without contacting a real issuer. This is not evidence that OMP failed to refresh.
3. **OMP retains expired credentials or cannot refresh:** outer OAuth validation rejects the request. A complete sanitized challenge and renewal outcome would distinguish this from inner session rejection. Client internals were not inspected.
4. **Route/challenge discovery mismatch:** a client reaches an unwrapped path or follows a path-bearing resource's advertised metadata URL. Actual request path plus a credential-free metadata fetch distinguishes this. Deployment relevance remains unknown.

No token expiry interval, issuer outage, JWKS rotation event, container failure, or client-refresh implementation bug is asserted from elapsed time alone.

## Cross-client acceptance boundaries

Use the existing loopback HTTP/auth/session seam with ephemeral keys, synthetic access tokens and controlled clocks. Cover a client retaining a session across valid token renewal, a client starting a fresh session, expiry/loss and reinitialization, and concurrent independent clients. Authenticate every request. Invalid/expired tokens remain 401; insufficient scope remains 403; different issuer/principal/client/audience or otherwise incompatible authorization context must not gain access to an existing session. Preserve static-token isolation. No OMP/Gemini user-agent special cases, disabled authentication, global trust, lifetime extension or container-restart workaround.

A future implementation must select and review the secure OAuth identity/reinitialization contract before changing fingerprint affinity. The incident still needs actual request path, complete sanitized challenge, deployed revision, and credential-renewal/session-change booleans or branch-specific admission evidence. Do not collect bearer strings or session identifiers.

## Published follow-ups

- 06: expired/unknown session recovery status — confirmed protocol defect.
- 07: client-neutral OAuth renewal/session recovery — confirmed behavior, suspected interoperability defect; incident causality open.
- 08: path-bearing resource metadata challenge — code-supported conditional defect; deployed configuration applicability unknown.

These are triage entries, not implementation authorization. Existing requirements/design approvals for the metadata feature are unchanged.

## Self-evaluation

Summary: 3.8/5 across five axes; evidence-backed defects delivered, incident root cause remains open.

- Accuracy 4/5: source/protocol discrepancy and observed 401s are anchored; deployed revision and branch correlation remain unknown. Improve with sanitized admission evidence.
- Completeness 3/5: debugger diagnosis and cross-client ticket criteria are supplied; the exact OMP failure branch and token-renewal behavior could not be established. Improve with a synthetic renewed-JWT wire repro and safe incident correlation, not a guessed fix.
- Clarity 4/5: confirmed behavior, protocol defect and incident inference are separated; several interacting auth/session branches remain. Keep the user-facing result focused on that distinction.
- Actionability 4/5: independent tickets and safe acceptance checks exist; renewal changes still require a secure identity decision.
- Conciseness 4/5: detailed source/verification evidence is kept here rather than repeated in every ticket; user-facing explanation should stay short.

Critical issues (axes <=2): none. Top improvement: identify the failing admission branch and securely reproduce renewal across independent client profiles. Self-check: the user should agree that this supports generic fixes, but not a claim that OMP token refresh has been proved faulty. Verdict: deliver the triage findings with that limitation, not an implementation/fix claim.

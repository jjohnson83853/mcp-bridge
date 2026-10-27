# OAuth 2.1 MCP Authorization (RFC 9728) — Task Frontier

Canonical specification: [OAuth 2.1 MCP Authorization Specification](spec.md).

Original tickets 01–05 are resolved. Recovery follow-ups 06–08 await triage; the diagnostics amendment below is a draft, not an executable implementation frontier.

| Ticket | Blocked by | Current state |
| --- | --- | --- |
| [01 — Add JWX dependency and internal auth package](issues/01-jwx-dependency-and-auth-package.md) | None | resolved |
| [02 — Protected resource metadata handler](issues/02-protected-resource-metadata-handler.md) | 01 | resolved |
| [03 — HTTP authentication middleware and claim validation](issues/03-http-auth-middleware-and-claim-validation.md) | 01 | resolved |
| [04 — Inbound transport context integration and session binding](issues/04-inbound-transport-context-integration.md) | 03 | resolved |
| [05 — Server composition wiring and HTTP wire test suite](issues/05-server-composition-wiring-and-wire-tests.md) | 02, 04 | resolved |

## Decisions so far

- **Grilling Round 1**: User approved dual-mode fallback, `github.com/lestrrat-go/jwx/v2` engine, outer `http.ServeMux` routing seam, environment variable configuration with defaults, and Bearer token SHA-256 session binding.
- **Grilling Round 2**: User approved fast failure on malformed/expired JWTs with 401 challenge, context-driven claims bypass in `InboundHandler`, non-blocking JWKS initialization, dynamic `authorization_servers` metadata, and RFC 6750 `error="insufficient_scope"` header on 403 Forbidden.
- **Grilling Round 3**: User approved 60-second clock skew tolerance, typed `Claims` context model, trailing slash audience normalization, multi-format scope parsing (space-delimited string & array), and in-process mock JWKS testing with zero external network calls.
- **Testing Seam**: The highest seam—loopback HTTP requests against `inboundServer.Handler` using an ephemeral mock JWKS server—is the primary testing seam for observable wire verification.

- Ticket 01 resolved: `github.com/lestrrat-go/jwx/v2` added to `go.mod`, `internal/auth/config.go` and `internal/auth/claims.go` implemented and verified with 100% test coverage.
- Ticket 02 resolved: `internal/auth/metadata.go` implemented and verified in `internal/auth/metadata_test.go` with 100% test coverage.
- Ticket 03 resolved: `internal/auth/jwks.go` and `internal/auth/middleware.go` implemented and verified with 99% test coverage.
- Ticket 04 resolved: `internal/service/inbound.go` updated to recognize `auth.ClaimsFromContext` while retaining session affinity.
- Ticket 05 resolved: Outer multiplexer wired in `cmd/mcp-bridge/serve.go`, end-to-end wire tests verified in `cmd/mcp-bridge/serve_oauth_test.go`, and full-suite coverage measured at 91.8% (gate >=90% passed).

## Authentication recovery follow-ups — 2026-10-26

The original implementation tickets above remain resolved. These new findings await triage; they do not change the approved security contract or authorize implementation.

| Ticket | Blocked by | Triage state | Evidence classification |
| --- | --- | --- | --- |
| [06 — Recover expired or unknown MCP sessions](issues/06-expired-session-recovery.md) | None | needs-triage | Confirmed protocol defect: authenticated stale sessions receive 401 rather than recovery 404. |
| [07 — Client-neutral OAuth token renewal and session recovery](issues/07-client-neutral-token-renewal.md) | None; secure policy review required | needs-triage | Bearer-byte affinity confirmed; OAuth interoperability and live-incident cause need investigation. |
| [08 — Reachable metadata challenges for path-bearing resources](issues/08-path-bearing-resource-metadata.md) | None | needs-triage | Conditional routing mismatch; deployment applicability unverified. |

[Diagnosis and executed verification](authentication-recovery-evidence.md): debugger review, successful post-reconnect discovery, bounded bridge 401 log sample, local OMP executable version and focused current-session checks. Reconnecting OMP is the only observed restorative action; no container fault or missed-refresh cause is claimed. Cross-client regression requirements are included in the tickets; no OMP/Gemini special case or authentication bypass is proposed.

## Diagnostic evidence amendment — 2026-10-26

The user requested enough diagnostic information to investigate the recurring authentication failure, then confirmed the testing seams and requested template-based tracker publication. The focused canonical specification is now [OAUTH-DIAG-r4 — OAuth/session diagnostics](../oauth-session-diagnostics/spec.md), preserving requirements R-D1–R-D9, acceptance criteria AC-D1–AC-D10 and prerequisite/task ledger T-D0–T-D6. The r3 amendment in the original OAuth spec is retained as historical planning evidence.

The amendment distinguishes credential/scope rejection, absent sessions, existing-session binding mismatch, routing and observed downstream failures. It requires safe request correlation, process/build/policy provenance, and preservation through the actual collection/normalization path into queried Loki. It does not change authentication, session affinity, response status or challenge policy; the proposed public correlation contract requires separate review and approval.

**Tracker status: ready-for-agent; implementation BLOCKED.** The label records the requested publication, not an implementation gate. Existing logger/composition/public-ID contracts, collector preservation, deployed identity and client evidence remain explicitly unresolved. Local JSON output cannot satisfy deployed Loki acceptance. Only the testing seam is confirmed; affected requirements/design/tasks checkpoints remain unapproved. Corrective issues 06–08 remain separate. No engineer dispatch, gate change or implementation is recorded.

[Independent reviews and approval impact](diagnostics-review.md): critic and security publication reviews PASS for r4, with no blocking findings. Prior r3 planning and r2 security verdicts remain historical evidence. Evidence and user-approval blockers remain; no implementation or live-validation result is implied.
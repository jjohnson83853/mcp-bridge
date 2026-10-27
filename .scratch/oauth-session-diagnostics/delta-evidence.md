# Diagnostics delta evidence — ticket 01

Evidence scope: source/API facts and research comparison, not implemented/runtime/external acceptance. Historical review and approval receipts are retained in [review.md](review.md); current execution state is authoritative only in [execution-state.json](execution-state.json).

## Deployed source baseline

Executed in this repository: `git status --porcelain=v1 --branch`, `git rev-parse HEAD`, and `git ls-remote --symref origin HEAD`.

- Remote default branch: main.
- Remote GitHub HEAD: `e621622397298bb4eb44a9b0f5ff1a00d6f16300`.
- Local HEAD: `e621622397298bb4eb44a9b0f5ff1a00d6f16300`.
- User-confirmed rule: current GitHub HEAD is deployed. No older-image hypothesis or additional proof of that relationship is needed.
- Status showed modified/untracked planning artifacts only; no application/test-source changes were reported. The new diagnostics spec/tickets are local-only and are not deployed behavior.

## Actual source-access result

One approved bridge call attempted codebase-searcher read_range for project `mcp-bridge`, `internal/observe/log.go`, lines 1–75 (max_lines 75).

Result: HTTP transport, receive stage, http_status failure, 401 unauthorized, retryable=no, WWW-Authenticate Bearer. Trace: `ac90350991cb72c00a5f08a201f0432c`. No source content returned. No retry, local-source fallback, Git-object source extraction, LSP bypass or Graphify call followed. Graphify is also ineligible on the dirty worktree.

This proves the connector rejected the read. It does not identify token expiry, refresh failure, session mismatch, or a container fault as the cause. After the user's reconnection, the first exact bounded retry succeeded (bridge response request ID `8e5608b0704f2a1bc5267bbba23db59d`), and authenticated source collection continued. The earlier access failure is historical, not a current research blocker.

## Existing logging: retained baseline, not new verification

The logging checkpoint record documents all six original tickets complete, with HTTP admission diagnostics, correlated MCP/backend execution, protocol/tool/backend failure classification, response shapes, streaming/cancellation and concurrent/reload isolation. It records suite/race PASS, 92.0% source-line coverage and passing code/security reviews. Contradictory stale map prose does not reopen that work.

The historical suite/race/coverage results are not verification of new work. Current interfaces at the recorded HEAD have now been collected in the [source contract](source-contract.md). The fresh composed HTTP smoke below verifies baseline wire/log behavior only.

## Source-checked delta inventory

Current source anchors, signatures, enums and compatibility constraints are in the source contract. This matrix separates existing implementation from the missing delta; proposed design/budget resolutions still require planner and independent review.

| Requirement | Existing evidence to reuse | Remaining comparison / owner ticket |
| --- | --- | --- |
| R-D1 | serve.go:131–164 wraps inbound with OAuth; inbound.go:139–274 owns inner terminal logging. | Outer rejection and separate metadata/health bypass inner owner. Add one outer owner with inner contribution/direct fallback; planner resolves writer/panic preservation in 01, implementation 02. |
| R-D2 | middleware.go:99–163 has actual OAuth branches; inbound.go:317–343 merges absent/mismatch; tools.go:209–315 already reports real downstream failures. | Add typed authoritative auth/session contributions, not rebuilt backend instrumentation. JWT subtype API review in 01; 02/03/04 own missing contributions. |
| R-D3 | inbound.go:143–145,285–287,362–371 separates owned internal ID from caller-controlled public X-Request-ID. | Public ID is ineligible as trusted identity without changing approved semantics. Planner selects exact owned-ID/header contract in 01 before 02; candidate header still unapproved. |
| R-D4 | log.go:63–75 fixes environment/version to unspecified; existing request/operation/backend chain remains. | Process incarnation, truthful build/policy and route/challenge diagnostics are missing. HEAD/deployed relationship resolved; 04 adds only missing context. Runtime/collector evidence remains external 05. |
| R-D5 | inbound.go:42–45,326–343,424–436 has exact-bearer comparison, live records and reaping. | Missing absent-versus-mismatch distinction and independent live-session pseudonym; record replacement must preserve new reference. OAuth helper does not send a session header, so retained/fresh A/B controls remain new work in 03. |
| R-D6 | log.go already closes/project fields, redacts recursively, validates IDs and avoids same-event sink retries. | New keys collide with schema/session/header redaction; no total encoded byte cap exists. Planner must reconcile physical compatibility and prove proposed core/full budgets in 01; local privacy/failure tests 02–04, actual normalized bytes 05. |
| R-D7 | Existing logging stops at stdout; historical Loki sample lacks distinguishing fields. | Actual capture/normalizer map and origin-to-Loki equality are external 05. Field absence alone does not establish where loss occurred. |
| R-D8 | Existing real composed HTTP/JWKS/captured-log seam confirmed; fresh focused baseline smoke PASS. Exact final commands and seven current metric collectors recorded. | New diagnostic matrix, entropy seams, lifecycle and unchanged scrape proof remain 02–04/final05. Historical coverage is not rerun coverage. |
| R-D9 | Existing finite fields and correlated downstream completion support safe local interpretation. | Incremental reason/query/visibility guidance and bounded evidence handoff remain; 05 proves real Loki procedure. Named external owners and actual client renewal are UNKNOWN. |

AC-D1–AC-D10 remain open for new diagnostics. The reviewed r2 proposal supplies the complete prerequisite/owner/verification map and corrected physical/owner/writer/serialization contracts. Source facts, proposal review and baseline PASS do not sign off new implementation, runtime/header/schema/budget enforcement, session controls or external AC-D3/D5/D6/D8. No criterion is waived or narrowed.

## Exact next actions and boundaries

1. DONE: user reconnection restored authenticated source retrieval; current logger, composition, admission/session, downstream logging, metrics and test contracts were collected through bounded project-scoped MCP/LSP access.
2. CURRENT NEXT GATE: finish reachable ticket01 research and identify exact remaining local evidence before engineer dispatch. Three explicit user checkpoints and independent requirements/design/task reviews are recorded in review.md; no remaining approval is invented. A new tool-catalog401 was observed during correction and not retried/bypassed; recovery/current authenticated source access remain unproven.
3. Identify collector/proxy/runtime evidence responsibilities for ticket 05. Their external access, real-client refresh evidence and historical incident causality do not gate independently resolvable local research or code.

Executed after reconnection: `go test ./cmd/mcp-bridge -run '^(TestServeOAuthWire|TestServeLoggingExecution|TestServeLoggingFailures|TestServeLoggingConcurrent)$' -count=1` — PASS, package 0.397s. Existing cases start the real composed loopback HTTP bridge with local dependencies; this is baseline behavior, not new-diagnostics or actual renewal proof. Pure-data proposed-schema arithmetic also PASS: core2048/optional200/full2248 bytes including LF; this is not runtime serializer/Loki proof. Exact installed verifier-clock API docs were checked. Full suite/race/vet/coverage, deployed collector probes and real OMP/Gemini smoke were not run. No application/test/config/gate changes, builds, commits or pushes were performed. Ticket01 remains unresolved for the precise review/reconciliation prerequisites in review.md.

## Bounded handoff and operator limits

- [Verified authentication/observer/session seams](auth-seam-evidence.md) and [planner proposal](planner-resolution.md): delivered research; specific independent review findings and current next gate live only in review.md. The user's TCP/session/token-lifetime hypothesis is recorded without claiming actual client behavior or incident cause.

- [Current source contract](source-contract.md): exact source anchors/signatures, existing fields, seven metric collectors and focused/final commands. Reuse completed logging; do not establish a second stream.
- Earlier 401 followed by successful reconnect proves access failure and recovery, not expiry, renewal, session mismatch, container failure or the point of collector loss.
- External roles needed for ticket05: stdout/capture/collector owner, normalized Loki/query/retention/access owner, client/proxy response-visibility capture owner, runtime/effective-policy evidence owner. Named contacts and authorized evidence access remain UNKNOWN. These do not block independent local diagnostics.
- Keep queries initially within two minutes, at most ten minutes when justified, fifty rows per query and two hundred per bundle. Export allowlisted classifications/opaque approved correlation only; minimize session pseudonyms. Never export credentials, claims/identity, raw session values/digests, arbitrary headers/paths/queries/payloads/errors or raw production logs.
- Local serialization tests cannot prove collector preservation. Shared wrapper traces cannot establish exact bridge joins; hidden-header/concurrent association stays ambiguous without safe boundary evidence.

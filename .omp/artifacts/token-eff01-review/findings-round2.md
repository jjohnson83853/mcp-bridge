# Ticket 01 spec — review round 2 findings ledger

Candidate revision r2 (`ticket01-bounded-results-v2`), verdict REQUEST CHANGES (bound).
Observation: `.omp/artifacts/token-eff01-review/observation-code-reviewer-r2-3efa69023401.json`.
Reviewer: `code-reviewer`. Review budget (S15): 2 of 5 used; 3 remaining.

## Verified closures (do not regress)

F1 (owner resolution + `directRequestOwner` for `call_tool`), F2 (upstream code extraction and page `code`), F4 (T3 test roots), F6 (docs-only T6), F8 (owner-namespaced store), F9 (atomic create/admission), F10 (periodic sweep), F12 (excluded wrapper constant), F14 (paging vs clipping discriminator), F15 (validated ceilings).

Reviewer caution on K1: T1's oracle must marshal through the SDK's own `MarshalJSON`, not hand-count fields, or `_meta`/`resultType` escape the measurement.

## Open findings

| ID | Class | Sev | Anchor | Defect |
| --- | --- | --- | --- | --- |
| R-01 | statement-contradiction | HIGH | §4.2 unretainable branch | An unretainable upstream `isError` result is given `code:"result_too_large"` with no second field, so R10's requirement that the upstream's own top-level code stay visible is false; AC10 has no unretainable-error fixture. |
| R-02 | statement-contradiction | MEDIUM | §4.5 eviction | §4.5 adds a third `result_too_large` cause (aggregate cap with insufficient own handles) that R1's exhaustive exception list and AC13 omit; R1 is false on a reachable, unoracled branch. State its precedence against `max_retained_result_bytes`. |
| R-03 | statement-contradiction | MEDIUM | §4.3 | Claiming pattern/`minimum` violations are rejected by schema validation is false for built-ins: the SDK delegates argument validation to the caller (`go-sdk mcp/server.go:306-308`) and handlers only `decodeStrict` (`internal/service/tools.go:1074-1081`), which enforces neither. `invalid_arguments` therefore has no producer and the malformed-vs-unrecognized split is unimplementable. Require handler-side shape validation before any store access, plus an AC. |
| R-04 | r-ac-coverage-gap | MEDIUM | §4.3 T3 | T3 omits `internal/service/session_exposure.go`, so the `requiresExposure` registration that §4.3 relies on has no owning task and no AC; the default branch (`session_exposure.go:60-83`) leaves the tool not exposure-relevant. |
| R-05 | ac-not-falsifiable | MEDIUM | §6 README claims | T6's checks are positive greps only, so they pass while README still says "three tools" or "complete upstream result"; the changed-claim inventory also omits `README.md:322-325` (discovery/describe response shape). |
| R-06 | statement-contradiction | LOW | §4.6 | No rule exists for retained handles across a catalog reload (handle key carries no revision; only restart and session retirement are addressed), yet §4.6 asserts catalog-revision admission is not bypassed. |
| R-07 | r-ac-coverage-gap | LOW | §5 AC12 | AC12's oracle is the ≥90% full-suite gate, but no task runs `go test ./...` or the coverage check. |
| R-08 | ac-not-falsifiable | LOW | §4.5 AC14 | "No handle equals a content or session digest" cannot fail for any CSPRNG output; the testable property is uniqueness. |
| R-09 | ac-not-falsifiable | LOW | §4.2 R11 | R11/AC11 reference a "full-content request" this ticket forbids and no fixture defines; reword as forward compatibility or drop. |

## Adequacy recorded (keep)

K2 no nesting; K3 crypto handle plus epoch namespace; K4 bounded store with creating-session-only eviction; K5 error pass-through apart from R-01; K6 fourth tool without schema change or `full`; K7 seam after `commitDiscovery` with verified counts; K8 UTF-8 slices; K9 acyclic ledger; K10 four README claims identified apart from R-05.

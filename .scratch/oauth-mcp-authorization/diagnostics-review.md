# Diagnostic specification review record

2026-10-26. Canonical artifact: [spec.md](spec.md), **OAUTH-DIAG-r3**. Requested scope: update the plan with sufficient safe evidence for future authentication/session diagnosis, not implementation.

**Verdict: planning draft PASS; implementation BLOCKED.** No affected requirements/design/tasks user checkpoint is recorded as approved. Historical OAuth and metadata approvals do not authorize this amendment.

## Independent reviews

- `AuthDiagnosticCritic`: r1 planning PASS with C1–C8 minor clarifications; r2 PASS with those resolved and two residual wording/traceability nits; r3 targeted PASS confirms both resolved at spec lines 264, 275 and 281. No new contradiction found in the final delta.
- `AuthDiagnosticPrivacy` (`security-reviewer`): r1 PASS, no Critical/High findings; four LOW findings. r2 targeted PASS confirms all four resolved, with no new privacy/integrity contradiction. r3 changes only conditional correlation wording and suite-wide prerequisite traceability; no new security contract.
- C1–C8 disposition: external acceptance blockers/unknown owners; prerequisite coverage; sole completion ownership; diagnostic versus effective policy revision; lifecycle regression; existing public-ID compatibility gate; full-suite PASS/coverage retained; core-byte retention. See spec lines 351–368 for the detailed disposition.
- Security findings preserved: session refs are stable pseudonyms and minimized in shared bundles; required diagnostic core cannot be displaced by oversized extras; exported operation records need complete-byte allowlist/sentinel scrutiny; distinguish origin absence, transit stripping, CORS invisibility and wrapper hiding. Resolved in r2, not waived.

## Verification and limits

Planner saved r3. Coordinator observed the r3 marker, conditional AC-D1 wording and suite-wide AC-D1–AC-D10 prerequisite map; independent critic verified the final two changes. Reviews used saved planning/evidence artifacts only, not source or live deployment validation.

All new behavioral tests, full-suite/coverage checks, real producer-to-Loki traversal and OMP/Gemini smoke: **NOT RUN**. No application/test/config/gate changes, image builds, infrastructure edits, commits or pushes.

U-D1–U-D6 remain unresolved: source schema/composition/public-ID/error mapping, collector preservation/access/retention/failure visibility, deployed build/policy identity, header visibility, client renewal evidence and complete branch/config applicability. External AC-D3/D5/D6/D8 portions cannot pass on local JSON output alone. Actual refresh behavior and historical incident cause remain UNKNOWN; corrective issues 06–08 stay separate.

Planner reported one edit-capability discovery call returning HTTP 401, no results/source returned, no retry or bypass. It informed no r3 technical fact and is not evidence of this incident's cause.

Self-evaluation (judgment, 1–5): accuracy/completeness/clarity/actionability/conciseness each 4. Limit: feasibility remains evidence-blocked.

## Standalone tracker publication — OAUTH-DIAG-r4

Canonical diagnostics specification: [OAuth/session diagnostics](../oauth-session-diagnostics/spec.md). Published with `Status: ready-for-agent` using the seven requested template sections and 32 numbered actor/feature/benefit stories. Historical OAuth technical content and the r3 amendment remain retained; cross-references point to r4.

User selected **“Use the proposed seams”**: composed HTTP listener with captured structured logs, plus actual deployed collector/normalizer/Loki preservation as separate acceptance. This confirms the test boundary only; requirements/design/tasks checkpoints remain UNAPPROVED.

`AuthDiagnosticCritic`: publication PASS, no blocking findings; no dropped acceptance or contradiction identified. `AuthDiagnosticPrivacy`: security-equivalence PASS, no blocking findings; r3 security meaning, unknowns and approval boundaries preserved. Source/collector/client probes and application tests NOT RUN.

Coordinator structure checks verified the seven headings, 32 sequential stories, label and complete stable identifiers. An initial read hit the documented size limit; remaining lines were retrieved. An overbroad path check matched a finite enum sequence, not a source path; manual inspection and critic review confirmed the distinction. Neither was a specification defect or application-test failure.

U-D1–U-D6 and external AC blockers remain. No implementation permission, gate change, code/config change, independent planning commit or push. This publication does not establish actual token refresh or the incident's cause.

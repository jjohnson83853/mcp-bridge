# Ticket 01 research review and next gate

## Evidence snapshot before retrospective maintenance

Local source/API research is collected at deployed/local GitHub HEAD `e621622397298bb4eb44a9b0f5ff1a00d6f16300`. Original incident cause and OMP/Gemini renewal/session behavior remain UNKNOWN. This is not a source-change, implementation approval or deployed-diagnostics claim.

Historical review and checkpoint receipts below remain evidence for OAUTH-DIAG-r6 and task amendment OAUTH-DIAG-r6-tasks-a1. Use [execution-state.json](execution-state.json) for current approvals, blockers and execution readiness; this document is not a second mutable status ledger.

## Independent critic findings

Historical r1 source: completed ReviewDiagnosticResolution report (read-only packet review plus bounded logger/inbound spot-checks; no tests/probes). C1–C4 were corrected in r2 and passed targeted re-review; C5 remains the canonical amendment/checkpoint requirement. Original findings are preserved:

- C1 MAJOR, severity preservation: current HTTP completion can remain info for tool-reported error under HTTP200; proposed warn is a required new diagnostic behavior, not existing preservation. Correct wording/source attribution and targeted RED/GREEN requirement while preserving actual admission/delivery exceptions.
- C2 MAJOR, capability construction: selected six-capability/64-mask intent needs a concrete maintainable facade construction and parameterized presence/absence test strategy, including native MaxBytesReader signaling and FlushError observation. Do not hand an unspecified combinatorial implementation to an engineer.
- C3 MAJOR, ownership adaptation: specify private marker/detection/state and exact inner observer/finalizer/metrics-defer adaptation; direct fallback and duplicate-completion RED case must be concrete.
- C4 MINOR, merge precedence: clarify trusted typed fields versus same-name redacted placeholders. Resolve together with security M1.
- C5 required canonical amendments: null status, complete method vocabulary, separate diagnostic provenance/duration keys, severity exceptions/change, generic JWT/unknown SDK limits, capability restoration, selected response header and composed seams need reviewed canonical reconciliation and affected checkpoints. Literal r4 is not implementation-ready.

Critic confirmed complete R/AC/T mapping, preserved old metrics, truthful generic/unavailable limits, proposed-ID justification and arithmetic receipt scope. Actual exhaustive JWT/SDK behavior and runtime/collector proof remain open. No historical cause or client implementation was established.

## Independent security findings

Historical r1 source: completed ReviewDiagnosticPrivacy report; packet/doc review only, no source retrieval or runtime checks. No proven current input-to-sink vulnerability, critical or high finding was asserted. M1/L1 corrections passed independent targeted r2 review by ConfirmTypedProjectionFix:

- M1 MEDIUM, typed-key collision: final HTTP projection must unconditionally delete every enumerated typed-owned key from the legacy/redacted output before inserting validated typed values. Absent/invalid/unavailable values must never fall back to stale Entry.Fields/base content. Cover request_id/operation_id/process_id/build_state/auth_mode and redacted diag_schema/session_* collisions, including entropy-unavailable negatives. Fixed keys, strict enums/types/ASCII and unchanged generic redaction remain required.
- L1 LOW, mechanism wording: session_binding=bearer_sha256 names the mechanism only; no credential fingerprint is emitted. Session diagnostic references remain independent random128-bit values.

Security confirmed bounded/lifetime-minimized refs, no new lookup capability, preserved wire auth/status/challenges, safe fixed schema intent and supplied upper-bound arithmetic. Implemented serializer/max-valid fixtures, writer/entropy behavior, actual normalized/exported bytes, live response visibility and dependency taxonomy remain unverified.

## Reviewed r2 correction disposition

- C1: explicitly distinguishes current HTTP info from new tool-error-under200 warn; targeted RED/GREEN and known severity exceptions retained.
- C2: concrete offline-generated finite facade/static-switch construction, bounded files, one embedded owner allocation, no runtime generation/reflection/callback allocation, independent supported/absent/delegation tests, FlushError and native MaxBytesReader handling.
- C3: private context/state/carrier/claimant lease and exact inner observer/finalizer removal, contribution/direct fallback and metrics-defer order; normal/panic finalization and original pre-finalizer window are explicit.
- C4/M1: unconditional clear/delete of every owned schema key before validated insertion; no stale Fields/base/redacted fallback when value is absent, invalid, unavailable or omitted. Generic redaction unchanged.
- L1: bearer_sha256 is only a mechanism label, never a credential digest.

Reviewer receipts: ReviewDiagnosticResolution targeted changed-section r2 review PASS; ConfirmTypedProjectionFix M1/L1 text comparison PASS,33.1s. Neither ran source/runtime/tests or authorized implementation. Schema/evaluator bounds unchanged; prior parent arithmetic PASS2048/200/2248 bytes remains proposed-map proof only. Exhaustive dependency/SDK claims, runtime/max-valid fixtures and actual normalized/exported/Loki acceptance remain unverified, not waived.

## Quality assessment

EvaluateDiagnosticResearch, document-only review: accuracy4, completeness5, clarity3, actionability4, conciseness4; overall4.0/5. Its two required corrections were applied: superseded access/progress blockers were changed to historical wording; completed fact collection, reviewed proposal and remaining canonical/user gates now have this single disposition. No new score is claimed and no substantive inaccurate implementation-completion claim was found.

## Ordered next gate

1. DONE: planner r2 corrected C1–C4/M1/L1 without changing schema bounds; independent targeted critic/security review PASS. R-D/AC-D/T-D IDs, actual unknowns and external acceptance remain intact.
2. **CURRENT NEXT GATE:** finish reachable ticket01 research evidence and identify exact remaining authoritative local facts before engineer dispatch. All three user checkpoints and independent requirements/design/task reviews are complete in their recorded scopes. Ticket02 remains blocked by unresolved01; actual machine-gate prerequisites must be checked before dispatch. The separately observed tool-catalog401 recovery/current authenticated source access remain unproven. No live-probe or infrastructure/auth-policy change was approved.
3. Only after the applicable gates: implementation02; independent03/04; main TDD/regression/full-suite source-line coverage>=90% and local behavioral proof. Unverified exhaustive JWT/SDK claims remain affected blockers; no guessed classifications. Ticket05 separately needs authorized actual capture/collector/Loki/header/runtime evidence. No local substitute or weakened AC.

The user's TCP/session/token hypothesis is recorded in auth-seam-evidence.md. Existing synthetic controls distinguish stale-A outer rejection from valid-B/retained-A-session mismatch and fresh-B success; neither proves actual client renewal behavior. No source/config/infrastructure change, build, commit or push performed.

## Review-agent failure and contained recovery

The first security follow-up wake failed with provider `invalid_prompt` / generation stopped with an incomplete tool call; no answer was produced. This is not a new MCP401 or evidence about token refresh. Exact upstream cause is UNKNOWN. A fresh narrow read-only M1/L1 review recovered successfully (ConfirmTypedProjectionFix PASS). An attempted per-task model override was rejected by argument validation because model overrides are seance-only; that rejected call did not execute. One corrected fresh dispatch omitted the unsupported field; no model/global configuration change occurred. No repeated wake, source bypass or production mutation was used.

## Canonical r5 reconciliation

User “continue” authorized the previously stated reconciliation, not requirements approval. Planner updated the existing canonical spec to OAUTH-DIAG-r5; coordinator updated only the five existing ticket contract links to that revision, preserving their dependencies/acceptance. Before mutation, Git status showed main tracking origin/main and existing planning-only changes.

Executed publication check PASS: seven exact to-spec top-level headings;32 sequential actor/feature/benefit stories; complete unchanged R-D1–D9,AC-D1–D10,T-D0–D6,U-D1–D6 sets; all three checkpoints UNAPPROVED; Implementation Decisions contains no source filenames/code fences. The71.7KB document was read in bounded pages after the50KB read footer; an initial path-validator false positive on ordinary enum/prose slashes was corrected. Neither was a specification defect. No application tests/build/lint/runtime/external probe or Git publish occurred for this planning reconciliation.

## Canonical r6 checkpoint review

Planner folded four MINOR r5 clarifications into r6: explicitly widened HTTP-log timing, exact available/unavailable correlation state, complete observability acceptance references, and unsupported FlushError behavior/tests. Architecture/schema/bounds and all32 stories/R/AC/T/U IDs were unchanged. ReviewDiagnosticResolution returned REQUIREMENTS ACCEPT and SPECIFICATION/DESIGN ACCEPT after targeted saved-r6 inspection; no remaining defect found in that scope. No runtime/source/test/build/external probe occurred. This is the current specification verdict, not the later saved-task-ledger verdict or any user confirmation.

The r6 requirements checkpoint is now APPROVED by the explicit user selection recorded below. Required scope remains: one existing completion stream including outer OAuth, authoritative auth/scope/session/downstream distinctions, safe owned correlation, independent refs/provenance, strict privacy/byte limits, actual collection preservation, deterministic local/full-suite source-line coverage>=90%, and bounded operator evidence. The feature does not change auth/session-binding/recovery semantics or prove historical/client renewal behavior. No design/task/implementation approval is inferred.

## Explicit r6 requirements checkpoint

Question: “Approve OAUTH-DIAG-r6 requirements R-D1–R-D9 and acceptance criteria AC-D1–AC-D10? This is requirements approval only; design, tasks and implementation remain unapproved.”

User selection: **“Approve requirements”**. Requirements checkpoint APPROVED for that exact revision and scope. Design/tasks remain UNAPPROVED; separate saved-task-ledger review PENDING; implementation NOT GRANTED. No technical contract or acceptance criterion changed.

## Explicit r6 design checkpoint

Question: “Approve OAUTH-DIAG-r6 Implementation Decisions and Testing Decisions, including the Mcp-Diagnostic-Id response header, widened HTTP-log timing, severity change and proposed test seams? This approves design only; tasks and implementation remain unapproved.”

User selection: **“Approve design”**. Design checkpoint APPROVED for that exact revision and scope, separately from requirements approval. Tasks remain UNAPPROVED; separate saved-task-ledger review PENDING; implementation NOT GRANTED. No technical contract or acceptance criterion changed.

## Separate r6 saved-task-ledger review

ReviewDiagnosticTaskLedger: **REJECT**, document-only, 2m25s. No source/runtime/network/test/build action. R/AC coverage, independent03/04, external unwaivable acceptance, privacy, unchanged auth/metrics and deployed=HEAD rule passed review.

- **C1 MAJOR:** ticket01:20/23/24 mixes research closure with implementation-only writer/budget/new-seam runtime proof while02 requires01 resolved. Separate existing capability facts, analytical budget and selected interfaces in01 from mandatory runtime RED/GREEN proof in02–04. Preserve AC-D1/D5 and all evidence; never close by assertion.
- **C2 MINOR:** explicitly delegate widened duration_ms tests/docs to02 and final operator rehearsal05.
- **C3 MINOR:** coverage lists omit02 R-D5 and03 R-D4 despite owning their ACs.
- **C4 MINOR:** make dispatch-blocked execution clear without inventing a sixth triage label; tracker labels do not override dependencies/approval gates.
- **C5 MINOR:** reconcile coordinator-owned final full-suite/coverage gate versus repeated per-ticket full-suite wording and T-D6 ownership; preserve >=90% full-suite source-line coverage and every local/release gate.

Planner correction must preserve approved requirements/design, all AC-D1–D10, IDs and genuine dependencies. This verdict grants no code permission and does not resolve ticket01 or its remaining authoritative branch facts.

## Operational observation during task correction

Planner reported a nonretryable **HTTP 401** from mounted bridge tool-catalog search, wrapper trace `ad37647200f6f0fd35463bb934903a48`; no source content, retry or denied-route bypass. The trace is not established as a bridge-owned diagnostic ID. This observation does not establish expiry, refresh failure, retained-session mismatch or a container fault.

The planner's local exact-replacement editor was unavailable and its full canonical-document read was truncated. Coordinator authorized a bounded planning-document continuation and proposal-only handoff to the existing external report sink; coordinator will persist planner-authored replacements with native hashline edits. This keeps documentation work reachable without bypassing the authenticated source route. Source/tool-catalog access has not been re-proven.

## Persisted task-only amendment

Planner CorrectDiagnosticTaskLedger delivered OAUTH-DIAG-r6-tasks-a1 in `local://diagnostic-task-correction-report.md`; coordinator applied its exact changed blocks using fresh hashline anchors. Only canonical task-view/revision-checkpoint metadata and the five existing ticket ownership/execution/verification clauses changed. Approved requirements/design and AC/scenario definitions were untouched; IDs, dependency graph and every checkbox state were preserved.

C1 research closure now requires existing source/interface facts, reviewed analytical bound and selected seams; implementation-only writer/budget/new-hook proof remains mandatory in02–04. C2 duration proof/docs/rehearsal is explicitly allocated; C3 missing R coverage added; C4 separate Execution BLOCKED retains existing labels; C5 Main owns the settled-code local full-suite/race/vet/>=90% source-line gate before shippable completion, independent of external05, which reuses valid identical-code receipts and still needs actual external proof. No tests or new acceptance proof were claimed. Fresh independent saved-task re-review requested; amended tasks remain UNAPPROVED.

## Independent amended saved-task verdict

ReviewDiagnosticTaskLedger: **TASK LEDGER ACCEPT, unchanged after targeted evidence completion**, for OAUTH-DIAG-r6-tasks-a1. C1–C5 resolved. Six additional grouped raw planning reads closed all initially named truncation/dependency limits; explicit03/04 independence and05 dependencies verified. No required runtime, privacy, >=90% source-line, pre-release or external acceptance proof waived. Remaining generic Contract wording is MINOR/nonblocking with no gate effect; no further correction required.

Verification was document-only. No source/SDK, MCP retry, runtime/network, test/build/lint/formatter, Git/config/gate action. User TASKS approval remains required; research01 is not resolved, external evidence/owners remain unknown, and this review does not establish current authenticated source access or historical incident cause.

## Explicit amended-task checkpoint

Question: “Approve OAUTH-DIAG-r6-tasks-a1 (T-D0–T-D6 and tickets 01–05), including this granularity and blocking graph? Approval permits sequenced engineer work only after its research/dependency and machine-gate prerequisites are satisfied; it does not authorize live probes or infrastructure/auth-policy changes.”

User selection: **“Approve tasks”**. Third checkpoint APPROVED for that exact amendment, T-D0–T-D6 and five-ticket graph, separately from unchanged r6 requirements/design. Sequenced engineer execution remains conditional on actual prerequisites; unchecked research criteria, external acceptance and current access uncertainty are not waived. No GATE_OPEN, implementation dispatch, source/test change or Git publish occurred.

## Research frontier and actual gate artifacts

Ticket01 now has11/12 research criteria checked with criterion-specific existing artifact/source anchors; only authoritative branch/JWT/SDK inventory remains unchecked. Checked research mapping/arithmetic/interface selections do not close any future runtime or external AC. Isolated synthetic installed-API probes are collecting only those remaining local facts; they do not retry the bridge or infer the historical cause.

Gate inventory: `.superpowers/sdd/GATE_OPEN` exists with earlier feature authorizations, but an exact `OAUTH-DIAG` search returned no match. It has not been modified or reused as diagnostic authorization. The expected project `.omp/extensions/sdd-gate.ts` was absent in the scoped file lookup; no `.superpowers/sdd/**/progress.md` matched. Actual extension loading/hard dispatch enforcement is therefore **not verified**; file policy alone is not proof. Missing enforcement does not waive recorded approvals, research prerequisites or the prohibition on denied-route bypass. No global configuration was inspected or changed.

## Current process stop and remaining evidence boundary

The user challenged excessive planning/review. No further planning or critic round is scheduled: requirements, design and amended tasks are approved. Ticket01 research reconciliation is complete as an evidence comparison, not ticket resolution:11/12 criteria are checked; authoritative bridge-embedded SDK cause/typed-observation and unexercised parser mapping remain explicitly incomplete in criterion22. Existing facts must not be restarted.

Both isolated local probes reported final exit0; independent Go review APPROVE-WITH-QUALIFICATIONS. Retained JSON and auth-seam-evidence.md now distinguish actual predicates/status/error-object observations from unsupported exact-code, taxonomy/order and protocol-conformance claims. Disposable probe sources were removed. No bridge application/test code, auth policy or deployed behavior changed.

Actual gate/access limits are recorded above: earlier-feature marker only, diagnostic authorization not added, project gate-extension presence/loading not verified, catalog401 observed without retry/bypass and authenticated source recovery unproven. These are not missing user approvals. The next implementation frontier is02 after actual prerequisite/access resolution; no new feature design or broad audit is required. No commit/push or infrastructure action occurred.

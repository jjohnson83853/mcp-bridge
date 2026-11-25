# Ticket 01 review packet — candidate `ticket01-bounded-results-v1`

Mode: specification review (pre-implementation).
Candidate: `.scratch/token-efficient-tool-responses/spec.md`, sha256 `4eed4835a9fc8b688e1dda7f5add5cd85ac56c804db304b268af27c2f540d9a3`, 44287 bytes.

## Reviewed inputs (all read-only)

1. Candidate specification (above).
2. Authoritative intake: `.scratch/token-efficient-tool-responses/issues/01-bounded-results-with-paging.md` — ACs O1–O9.
3. Refinement assessment with R1–R12, mapping, sibling boundaries: `.omp/artifacts/token-efficiency-ticket01-refinement.md`.
4. Feature map and sibling tickets: `.scratch/token-efficient-tool-responses/map.md`, issues 02/05/06.

## Author and reviewers

- Author: `planner` (PagingTicketSpec).
- Reviewers: independent, read-only. Neither may edit the candidate.

## Risk packet (shared by both stages)

| ID | Risk | Class | Affected requirements | Evidence requested |
| --- | --- | --- | --- | --- |
| K1 | The ceiling is measured on the wrong serialization, so a "bounded" response still ships a duplicate copy in `structuredContent`. | byte-accounting | R1, R2, R11 | Names the exact measured form and shows the duplicate paths are counted once, not twice or zero times. |
| K2 | Retrieval nesting: paged previews themselves exceed the ceiling or are recursively paged. | contract | R1, R3 | States preview/pages cannot be re-paged and shows a termination argument. |
| K3 | Handle identity is guessable, content-derived, or bound only to a reusable session ID. | security | R7, R9 | Distinguishes session ID from incarnation; rejection is indistinguishable across unknown/tampered/foreign. |
| K4 | Storage bounds allow cross-session eviction or unbounded growth under many sessions. | resource | R9 | Per-session and aggregate accounting, eviction ordering, and isolation rule. |
| K5 | Error semantics regress: a paged `isError` result is mistaken for a success, or a dispatch failure becomes a paged success. | correctness | R10 | Error visibility in preview and pages; small typed error path unchanged. |
| K6 | Retrieval cannot be expressed because stored input schemas reject unknown parameters (`additionalProperties:false`). | compatibility | R6 | Names the caller-visible surface and why schema validation admits it. |
| K7 | Discovery/describe ceilings break existing discovered-tool exposure or catalog-revision admission. | regression | R11 | Shows invariants preserved and which existing tests would catch a break. |
| K8 | UTF-8 or JSON-escaping corruption at page boundaries. | correctness | R4, R5 | Page-boundary rule and round-trip oracle. |
| K9 | Task ledger cannot be verified: a task has no falsifiable acceptance evidence, or depends on unwritten capability. | planning | R12, T1–T6 | Each task names its proving check and its dependency edges. |
| K10 | Documented public contract change is silent: README still promises the complete upstream result inline. | documentation | R6 | Names the changed claims and the replacement wording requirement. |

## Rulings that are settled, not open

- Delivery-only bounding. Execution is never skipped.
- No `full` projection knob here (ticket 05 owns `full`).
- No default field projections and no deduplication in this ticket.
- Bridge-issued handles only; no client transcript identifiers.
- Session isolation, exposure epoch, catalog-revision checks, and error taxonomy are invariant.

## Output contract

Return `VERDICT: PASS` or `VERDICT: REVISE`, then findings ordered by severity. Each finding: ID, requirement or task affected, one-sentence defect statement, evidence anchor (spec line/section), and the observable consequence. Do not propose implementation code; you may name the invariant the spec must state. Do not soften a REVISE into PASS because the defect looks small.

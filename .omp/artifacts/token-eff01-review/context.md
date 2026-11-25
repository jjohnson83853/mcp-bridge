# Shared reviewer instructions — feature `token-efficient-tool-responses`, ticket 01 spec candidate

Repository: /home/opencode/projects/default/mcp-bridge (Go MCP bridge server). Read-only: never modify any file.

## Candidate under review

`.scratch/token-efficient-tool-responses/spec.md` — review the on-disk bytes; this context does not pin a
revision or digest, because repairs replace the file between rounds. Candidate id
`token-efficient-tool-responses-ticket01-spec`. Read the file's own STATUS section for its revision. Author: `planner`.

Prior rounds are recorded in `.omp/artifacts/token-eff01-review/findings-round1.md` and `findings-round2.md`;
the current revision's ledger is the highest-numbered one present. Verified closures must not regress.

## Read first

1. `.omp/artifacts/token-efficiency-ticket01-review-packet.md` — reviewed inputs, risk packet K1–K10, settled rulings, output contract.
2. The candidate specification.
3. `.scratch/token-efficient-tool-responses/issues/01-bounded-results-with-paging.md` — authoritative intake, ACs O1–O9.
4. `.omp/artifacts/token-efficiency-ticket01-refinement.md` — refined R1–R12, mapping, sibling boundaries.
5. `.scratch/token-efficient-tool-responses/map.md` and sibling issues 02, 05, 06 for boundary overlap.

## Grounding sources (read only what a finding needs)

- `internal/service/tools.go:425-472`, `:526-656`, `:699-910`, `:967-990`, `:1035-1048`
- `internal/service/direct_execution.go:29-64`
- `internal/service/session_exposure.go:100-177`
- `internal/service/projection.go:195-215`, `:367-470`
- `internal/service/cursor.go`
- `internal/catalog/schema.go:74-89` (`additionalProperties:false`)
- `internal/config/bounds.go:80-95`
- `README.md:277-340`, `:455-490`
- `.omp/RULES.md`

## Settled rulings (do not re-litigate)

- Delivery-only bounding; execution is never skipped.
- No `full` projection knob in this ticket (ticket 05 owns `full`).
- No default field projections, no deduplication, no ranking changes.
- Bridge-issued handles only; no client transcript identifiers.
- Session isolation, exposure epoch, catalog-revision admission, and the existing error taxonomy are invariants.

## Constraints

- Judge the specification, not the implementation. A design preference is not a defect; an unverifiable acceptance criterion, an intent hole, an unstated invariant, a security or isolation failure, or a contradiction with the ticket or settled rulings is.
- Do not propose implementation code or line-level edits. Name the invariant or oracle the spec must state.
- Requirements must be falsifiable by observable behavior. If a requirement can only be checked by reading internals, say so.
- Treat O1–O9 and R1–R12 as acceptance sources; a dropped or weakened AC is a finding.

## Output contract

Return `VERDICT: PASS` or `VERDICT: REVISER` as the first line, then findings ordered by severity. Each finding: ID, affected requirement or task, one-sentence defect, spec section anchor, observable consequence. Never upgrade a REVISE to PASS because a defect looks small; never invent findings to appear thorough.

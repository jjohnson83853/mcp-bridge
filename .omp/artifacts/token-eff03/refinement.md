# Refinement — ticket #117

## Assessment

READY — refinement only, not approval.
Source: jjohnson83853/mcp-bridge#117, updated 2026-11-27T15:30:35Z; seven bullets numbered A1–A7 below. #116 is CLOSED; delivery comment identifies main f32e8e06e15f483cca6488c2d50fe8435f3f052d. No declared blocker.

Current behavior (paths relative to internal/service/):
• tools.go:479–524: search validates owner/current assembly and response revision, then commits ALL returned targets before delivering compact results.
• tools.go:1030–1118: hits contain only tool_name/description/score; normalized descriptions have a 36-encoded-byte budget. internal/search/search.go:72–84 caps effective default/maximum at 3/4. Ticket 02 stays closed.
• tools.go:580–678: describe returns stored schemas AND commits its target; it currently auto-exposes.
• projection.go:297–346,367–470: commits merge session interests into pin-backed descriptors and reconcile the registry; publisher discovery is a no-op. Surviving interests rebase across publications, not necessarily disappear.
• session_exposure.go:29–164,229–265: middleware lists owner-specific built-ins/descriptors, rejects another owner's projected direct name, and binds cursors to epoch/projection generation.
• inbound.go:481–484,527–535,581–599: strips caller epoch, verifies session credential binding, stamps retained server epoch. tools.go:290–330 additionally validates live owner, accepted generation, registry reference, target and descriptor/schema identity.

Normalized request: Separate search from declaration exposure. Only caller-selected, explicitly requested bounded exposure may add unpinned declarations. Preserve configured pins, direct-call eligibility and ownership/epoch/catalog protections. Document the exposure request and describe behavior; ordinary describe must not auto-expose unless explicitly designed.

Bidirectional equivalence table: original bullet ↔ proposed measurable clause/oracle; each row preserves the same observable obligation, without selecting implementation.
A1 ↔ Compare complete tools/list traversal before/after search: zero additions attributable solely to matches; same no-fanout guarantee.
A2 ↔ Document finite bound N and request contract; selected valid sets within N succeed, above-bound sets deterministically reject; same bounded explicit selection.
A3 ↔ Publisher and session lists retain configured pin names/descriptors as before; same pin compatibility.
A4 ↔ Never-pinned/never-explicitly-exposed direct calls retain existing rejection behavior and dispatch no upstream call; same eligibility boundary.
A5 ↔ Expose in session A: chosen declarations appear there; session B's projection remains unchanged; same isolation.
A6 ↔ Change catalog revision: stale projection/revision admissions remain invalid, owner/epoch checks remain enforced; same invalidation, not a new requirement to erase surviving interests.
A7 ↔ Automated tests cover A1, explicit success, bound rejection, pin preservation and A5; same five required cases.

Planner decisions: new built-in vs existing-method parameter vs describe_tool option; exact schema, finite N and whether per-request/cumulative; duplicates/empty/unknown names, mixed-set atomicity and deterministic errors; revision binding, repeat exposure, publisher semantics. Explicitly decide describe default/opt-in exposure—current unconditional mutation cannot remain accidental.

Sizing/risks: One coherent projection transaction and admission boundary; all bullets measurable, no irreducible scope. Risks: shared registry leaking cross-session access, owner retirement/reused epochs, reload races and stale cursors. progressive_discovery_test.go:244–355 currently expects searches to expose tools. Preserve compact search/ranking, generic call_tool and retained-result paging; exclude sibling tickets. Source inspection only; no edits or checks run.

## Handoff

Use assessment as #117 planner input. Resolve exposure mechanism/bound and describe semantics before formal AC/design. Preserve A1–A7; do not reopen #116. No implementation or approval supplied.

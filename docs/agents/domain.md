# Domain Docs

## Layout

Single-context: root `CONTEXT.md` and root `docs/adr/`.

## Before exploration

- Read `CONTEXT.md` when present.
- Read ADRs relevant to the area being explored.
- If `CONTEXT-MAP.md` exists later, follow its pointers to relevant
  context-specific context documents and ADRs.
- When domain documents are absent, proceed silently. Do not create
  placeholder context documents or ADRs; domain-modeling creates them as needed.

## Vocabulary and decisions

Use context-document terminology in specifications, issues, tests, and proposals.
If a needed concept is missing, reconsider the terminology or note the
gap for domain-modeling.

Surface conflicts with existing ADRs explicitly rather than silently
overriding their decisions.

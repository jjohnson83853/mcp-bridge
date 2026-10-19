# Domain Docs

## Layout

Single-context: root `GLOSSARY.md` and root `docs/adr/`.

## Before exploration

- Read `GLOSSARY.md` when present.
- Read ADRs relevant to the area being explored.
- If `GLOSSARY-MAP.md` exists later, follow its pointers to relevant
  context-specific glossaries and ADRs.
- When domain documents are absent, proceed silently. Do not create
  placeholder glossaries or ADRs; domain-modeling creates them as needed.

## Vocabulary and decisions

Use glossary terminology in specifications, issues, tests, and proposals.
If a needed concept is missing, reconsider the terminology or note the
gap for domain-modeling.

Surface conflicts with existing ADRs explicitly rather than silently
overriding their decisions.

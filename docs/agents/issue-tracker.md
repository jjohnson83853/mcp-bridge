# Issue tracker: Local Markdown

Issues and specs live as Markdown files in this repository, not GitHub Issues.

## Conventions

- One feature per directory: `.scratch/<feature-slug>/`.
- Feature spec: `.scratch/<feature-slug>/spec.md`.
- Implementation tickets: `.scratch/<feature-slug>/issues/<NN>-<slug>.md`,
  numbered from 01, one file per ticket.
- Record triage state as a `Status:` line near the top of each spec or ticket.
  Use the vocabulary in `triage-labels.md`.
- Append discussion under `## Comments`.

## Skill operations

- Publish: create the appropriate spec or ticket file.
- Fetch: read the referenced file.
- Apply a triage label: update its `Status:` line.

## Wayfinding

- Map: `.scratch/<effort>/map.md`.
- Child tickets use the numbered issue-file convention above.
- Record ticket kind in `Type:`: research, prototype, grilling, or task.
- Record dependencies as `Blocked by: NN, NN`.
- A ticket is unblocked when all listed dependencies are resolved.
- For coordinated work, current approvals/access/dependency blockers live in
  `.omp/plans/execution-state.json`, as defined by project policy and
  `skill://specdd-coordinator`. Specs, maps and tickets link there instead of
  repeating mutable execution summaries. Historical observations remain evidence.
- `Status:` and resolved answers still describe tracker lifecycle; the execution
  record cannot resolve a ticket, fabricate approval or waive acceptance.
- Claim an unblocked ticket by setting `Status: claimed` before working.
- Resolve by appending `## Answer`, setting `Status: resolved`, and adding
  an answer pointer to the map's Decisions-so-far section.
- Choose the first open, unblocked, unclaimed child ticket by number.
- `claimed` and `resolved` are wayfinding lifecycle states, not triage labels.

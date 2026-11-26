# Issue tracker: GitHub Issues

Issues and specs live as GitHub issues in this repository. The local Markdown
tracker under `.scratch/` was migrated on 2026-11-26 and is retired as a
tracker: `.scratch/MIGRATION.md` maps every file to its issue, and where a file
and an issue disagree, the issue wins.

## Conventions

- One feature per milestone (`<feature-slug>`); the migration source directory
  naming is preserved there.
- Feature spec: one parent issue labelled `type:spec`, carrying the
  specification and a checklist of its tickets.
- Implementation tickets: one issue per ticket labelled `type:ticket`, titled
  `[<feature-slug>] <NN> — <title>`.
- Triage state is a `status:*` label using the vocabulary in
  `triage-labels.md`; the original `Status:` line survives in the issue body.
- Discussion belongs in issue comments.

## Skill operations

- Publish: `gh issue create` with the `type:spec` or `type:ticket` label and the
  feature milestone.
- Fetch: `gh issue view <number> --json body,labels,milestone`.
- Apply a triage label: `gh issue edit <number> --add-label status:<value>`.

## Wayfinding

- Map: `.scratch/<effort>/map.md` remains a working note, not the tracker.
- Record ticket kind in the issue body: `Type:` research, prototype, grilling,
  or task.
- Record dependencies as `Blocked by: #N, #N`; the migration rewrote legacy
  `NN` references to issue numbers.
- A ticket is unblocked when all listed blocking issues are closed.
- For coordinated work, current approvals/access/dependency blockers live in
  `.omp/plans/execution-state.json`, as defined by project policy and
  `skill://specdd-coordinator`. Issues link there instead of repeating mutable
  execution summaries. Historical observations remain evidence.
- Issue state and resolved answers describe tracker lifecycle; the execution
  record cannot resolve an issue, fabricate approval or waive acceptance.
- Claim an unblocked ticket by adding `status:claimed` before working.
- Resolve by appending the answer as a comment, adding `status:resolved`, and
  closing the issue; add an answer pointer to the map's Decisions-so-far
  section when a map exists.
- Choose the oldest open, unblocked, unclaimed ticket issue by number.
- `status:claimed` and `status:resolved` are wayfinding lifecycle states, not
  triage labels.

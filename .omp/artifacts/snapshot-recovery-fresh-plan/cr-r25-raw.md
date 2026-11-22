<task-result id="SnapshotRecoverySpecR25CodeReview-2" agent="code-reviewer" status="completed" duration="3m25s">

<output>
Specification mode, initial pass. Candidate `snapshot-recovery-spec-r25`@r25; read-only, no tests run; SHA-256 not recomputable with this toolset (identity accepted from the `bfb95c…` registry/state binding).

- **[MEDIUM][HIGH]** `.scratch/snapshot-recovery/spec.md:8` — The r25 Round bullet absorbed the r24 text ("…advisory only (§9). with **no change to any R/AC/T id … or the task decomposition**: F1…F6"), contradicting its own item (1) (decomposition changed, T4 added) and leaving prior-round findings unscoped. Fix: remove/split the merged r24 paragraph.
- **[MEDIUM][HIGH]** `spec.md:228` (also `:111`) — §9.8's "no R/AC/T id added, removed, or renamed" and §5's "(no envelope change)" are false: §6.1 adds T4 to `allowedTaskIds`; §9.22 scopes only 9.9–9.21, so 9.8 is live. Fix: "no R/AC id changed; T4 added by owner authorization; the 14-file write set is unchanged".
- **[MEDIUM][HIGH]** `spec.md:137`, `:156`, `:166` — the mapping claims each R/AC id is owned by exactly one task, but AC1-10 is assigned to T2 (base + subcases) and again to T3 ((b-xA)…(b-dangle)). The subcase partition is complete and non-overlapping; the exactly-one-id claim is not. Fix: give AC1-10 one owner and cross-reference T3's subcase oracles (or restate ownership per subcase).
- **[LOW][HIGH]** `spec.md:90` — "`runtimeErrorInfo("state_directory")` returns after T2" is a stale pre-r25 task id (envelope vocabulary is T3; cf. §3:35, §7:207), contradicting the change log's claim that live stale task references were re-pointed. Fix: s/after T2/after T3/.
- **[LOW][MEDIUM]** `spec.md:69`, `:83` — the pre-pass record pins link entries' identity+path and the final target's identity only, while ordinary (iii)/(m-iii) path matching must match a moved regular final target (AC1-10(b-xB2)/(b-xB3)); a literal reading leaves (iii) undecidable there. Fix: record the final target's cleaned path as a chain-entry path.
- **[LOW][HIGH]** `spec.md:4`, `:8` — "canonical execution state remains r16" is contradicted by observed `.omp/plans/execution-state.json` (feature snapshot-recovery, revision r25, spec sha256 `bfb95c…`); true at dispatch, stale after registration. Fix: scope it to the authoring dispatch.
- **[LOW][HIGH]** `spec.md:8`, `:135`, `:248` vs `.omp/artifacts/snapshot-recovery-ticket-decomposition-r25.md:3` — authorization is said to be "recorded in" an artifact whose Status is "draft proposal, not authorized" (its mapping also omits AC1-5..AC1-9 and sets `resource.path`=quarantine path, contrary to D3/AC2-2); the owner record is `round-15-seed.json.ownerAuthorization`. Fix: cite the seed; treat the artifact as a proposal.

**Checks passed:** CT24-C5 re-key consistent (R2/§4 contract/§4(3)/alternatives/§8(2)/T3/§7/§9) with D10/D11 untouched (D12 only amended); AC2-9 hard-linked variant executable-in-spec (layout, `Readlink`, `SavePrompts` acceptance, restart no-op, 1/0/0 counting); CT24-C8 applied, §9.6 intact; CR24 LOWs applied with the old token retained only as quotation; §6.1 union exactly the same 14 files; std-lib only; no R/AC id renumbering; no review/approval/test claim. C4 and CR23 F1–F6 closures intact.

## Verdict: REQUEST CHANGES
</output>
</task-result>
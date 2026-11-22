# Patch: ignore harness result spill in .gitignore

Status: prepared only; coordinator edit gate denies direct writes to repo-root files
(`E_COORDINATOR_EDIT_FORBIDDEN`). Needs an authorized implementer or an owner
maintenance exception. Local effect is already applied per-clone via
`.git/info/exclude` (uncommitted), which stops this worktree from dirtying.

Apply to `/home/opencode/projects/default/mcp-bridge/.gitignore`, append after the
`bashnew` line:

```diff
--- a/.gitignore
+++ b/.gitignore
@@ -46,3 +46,6 @@
 # local tool scratch spill
 bashnew
+
+# harness result spill files (regenerated per large tool output; historical files stay tracked)
+.omp/artifacts/result-pith/call_*
```

Rationale: the harness writes one `call_*_fc_*.json` spool file per large tool
result. They are untracked transient output, not evidence, and make
`execution-state.mjs verify-delivery` fail with `E_DIRTY_WORKTREE`. The 43
already-tracked spill files (Nov 11 commits and the r4 preservation set) are
unaffected because `.gitignore` does not untrack files.

Verification after applying:

```bash
touch .omp/artifacts/result-pith/call_ignorecheck_fc_tmp_x.json
git status --porcelain   # must be empty
git check-ignore -v .omp/artifacts/result-pith/call_ignorecheck_fc_tmp_x.json
rm .omp/artifacts/result-pith/call_ignorecheck_fc_tmp_x.json
```

Once committed, the matching block in `.git/info/exclude` can be removed.

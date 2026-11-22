# result-spill-ignore — maintenance specification

<!-- specdd:requirements:start -->
## Requirements

- R1: .gitignore MUST ignore harness result-spill files matching .omp/artifacts/result-pith/call_* so that regenerated spill cannot dirty the worktree.
- R2: Already-tracked spill files MUST remain tracked and unmodified; the rule must not untrack or rewrite them.
- R3: The ignore rule MUST be the only production change; no source, test, or canonical-state behavior changes.
<!-- specdd:requirements:end -->

<!-- specdd:design:start -->
## Design

Append to /home/opencode/projects/default/mcp-bridge/.gitignore after the existing bashnew line:

    # harness result spill files (regenerated per large tool output; historical files stay tracked)
    .omp/artifacts/result-pith/call_*

Git ignore rules affect only untracked paths, so the tracked spill files stay in the index and their bytes are untouched. The local .git/info/exclude block becomes redundant after this change and may be removed by the owner.
<!-- specdd:design:end -->

<!-- specdd:tasks:start -->
## Tasks

- ENG1 (engineer, write): append the three-line rule block to .gitignore; verify with the commands in AC1. No other file.
<!-- specdd:tasks:end -->

<!-- specdd:envelope:start -->
## Envelope

- allowedTaskIds: ENG1
- writePaths: .gitignore
- readPaths: .gitignore, .omp/artifacts/gitignore-result-pith.patch.md
- allowedEffects: read, write
- retryPolicy: maxAttempts 1
<!-- specdd:envelope:end -->

# Tool Guard trial (project-scoped)

Status: **prepared, not installed.** The OMP dispatch gate blocks coordinator
edits to project paths; installation needs one owner action.

## What is already in place

| Item | Path |
|---|---|
| `tg` binary (built from tool-guard-core `e58562de3b74`, Go 1.26.11) | `/home/opencode/.local/share/tool-guard/mcp-bridge/bin/tg` |
| Five shipped example policies (Apache-2.0, unmodified) | `/home/opencode/.local/share/tool-guard/mcp-bridge/policies/` |
| Audit chain (created per tool call) | `/home/opencode/.local/share/tool-guard/mcp-bridge/audit/hook-audit-<UTC date>.jsonl` |
| OMP adapter (prepared, verified via `--hook`) | `.omp/artifacts/toolguard-trial/tool-guard.ts` |

No repository file is modified by the trial. The adapter is pinned to
`/home/opencode/projects/default/mcp-bridge` and is inert anywhere else.

## Activate

Copy the adapter to the project hook discovery path, from a plain terminal
(outside the OMP session; the hook protects itself once active):

```sh
install -m 0644 \
  /home/opencode/projects/default/mcp-bridge/.omp/artifacts/toolguard-trial/tool-guard.ts \
  /home/opencode/projects/default/mcp-bridge/.omp/hooks/pre/tool-guard.ts
```

Takes effect on the next OMP session start in this project (ambient hook
discovery loads `.omp/hooks/pre/*.ts`).

## Rollback

From a plain terminal:

```sh
rm /home/opencode/projects/default/mcp-bridge/.omp/hooks/pre/tool-guard.ts
```

Then restart the session. Binary/policies under `~/.local/share/tool-guard/`
can stay for a later retry or be deleted.

## Enforcement mapping

- `allow` -> tool runs
- `deny` -> blocked, reason returned to the model
- `ask` -> main session with UI: operator confirm; print mode/subagents: blocked
- internal error (missing binary/policy, timeout, malformed output) -> blocks
  `bash`/`write`/`edit`/`eval`, allows read-only tools

Tool name mapping: `write`->write, `edit`/`ast_edit`->edit, `bash`->bash,
`eval`->bash with the code text as `command`, read-only tools -> `read`.
Unknown names stay as-is and land in Tool Guard's `shell` group.

## Protected paths (unconditional, before policy evaluation)

- `~/.local/share/tool-guard/mcp-bridge` (binary, policies, audit)
- `<repo>/.omp/hooks/pre/tool-guard.ts`

Consequence: once active, the guard file itself cannot be edited or removed by
any tool call in-session. Use a plain terminal for rollback or edits.

## Known limits (documented by upstream)

- Shell checks are text heuristics, not a shell parser; see the upstream
  `examples/coding-agent-guard/README.md` limits section.
- `eval` mapping catches command-shaped deny rules in code text, not nested
  `sh("...")/Bun.$(...)` strings. Treat `eval` as only partially covered.
- Secret policies match listed vendor key formats only; they are not secret
  scanners.
- A shell redirect to `AGENTS.md` is not covered by the context-files policy;
  only `-protect-path` covers named paths.

## Boundaries

This ADDS a runtime tool firewall. It does **not** replace, disable, weaken, or
substitute for the SpecDD dispatch gates (`workflow-policy/execution-gate.ts`,
canonical execution state, reviews, receipts, retry limits).

## Starter policy (project-specific)

Typical Tool Guard setup = a small policy file with one effect per rule,
an offline call corpus, and `simulate` before enforcement. This trial adds
`policies/mcp_bridge_guards.yaml` (artifact copy under
`.omp/artifacts/toolguard-trial/policies/`):

| Rule | Effect |
|---|---|
| `curl\|sh`, `bash <(curl ...)` | deny |
| direct `.git/` writes (tool or shell) | deny |
| docker/podman/systemctl/kubectl mutations | escalate |
| global toolchain installs | escalate |

Corpus: `.omp/artifacts/toolguard-trial/calls.jsonl` (12 calls).

Install the policy (takes effect on the next tool call; `tg` reloads policies
per invocation, no restart needed):

```sh
install -m 0644 \
  /home/opencode/projects/default/mcp-bridge/.omp/artifacts/toolguard-trial/policies/mcp_bridge_guards.yaml \
  /home/opencode/.local/share/tool-guard/mcp-bridge/policies/
```

Regression check (expected: 4 allowed, 3 escalated, 5 denied):

```sh
~/.local/share/tool-guard/mcp-bridge/bin/tg simulate \
  -policy-dir ~/.local/share/tool-guard/mcp-bridge/policies \
  -calls .omp/artifacts/toolguard-trial/calls.jsonl
```

Add a rule: edit the policy -> `tg lint -policy <file>` -> add a corpus line ->
`tg simulate` -> install. Keep one effect and one citation per rule.

## Verification run (2026-11-20)

- All five policies `tg lint` exit 0.
- Direct decisions: recursive `rm` -> ask; plain `echo` -> allow; write to
  policy dir -> deny; write to hook file -> deny; shell `rm` of hook file ->
  deny; read-only `cat` of hook file -> allow; normal `write` -> allow.
- End-to-end through OMP (`omp -p --hook <artifact>`): `echo TOOLGUARD_ALLOW`
  executed; `rm -rf /tmp/toolguard-canary-nope-12345` blocked with
  `Tool Guard: escalation needs an operator with UI (...manual review...)`.

## Agent skills

### Issue tracker

Before reading or publishing issues/specs, read `docs/agents/issue-tracker.md`.
This repository tracks work as local Markdown under `.scratch/`.

### Triage labels

Before assigning triage state, read `docs/agents/triage-labels.md`.
Use the five default canonical labels.

### Domain docs

Before codebase exploration, read `docs/agents/domain.md`.
This repository uses a single-context domain-doc layout.

## Coordinator and subagent guardrails

### No coordinator implementation
- The coordinator session MUST NOT directly edit application or test source code (`internal/**/*.go`, `cmd/**/*.go`).
- All production code changes, review fixes, and test modifications MUST be delegated to `engineer` or `test-engineer` subagents.
- The coordinator taking over the keyboard to write "trivial fixes", "quick adjustments", or test code is strictly prohibited.

### Subagent stall and incomplete escalation
- If an engineer or test-guide subagent yields `INCOMPLETE`, stalls, or fails verification, the coordinator MUST NOT step in to finish or write the code directly.
- The coordinator MUST immediately stop and report:
  1. The exact duration and tool/reading count of the failed subagent.
  2. Root cause of why the subagent stalled or failed (e.g., unbounded research, ambiguous contract).
  3. The proposed surgical dispatch prompt for user approval before relaunching.

### Implementation dispatches: zero open-ended research
- Implementation prompts for `engineer` agents MUST enforce a strict zero-research boundary: no open-ended reading of external SDKs, vendor code, or codebase exploration.
- Dispatches MUST provide exact file targets, line ranges, concrete struct definitions, and function signatures.
- If an engineer executes more than 12 read/search operations without producing an edit or write, it must be flagged as stalled.

### Test guide wire contract discipline
- When dispatching a `test-engineer` or test guide for integration tests, the prompt MUST specify the exact JSON-RPC wire payload, method name, and endpoint contract (e.g., using `call_tool` envelope rather than direct tool names) to prevent invalid test invocations.

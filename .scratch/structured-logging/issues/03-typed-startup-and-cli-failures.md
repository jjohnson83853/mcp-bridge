# 03 - Typed Startup and CLI Failures

Status: ready-for-agent
Blocked-by: 01

## What to build
Failures that are not configuration validation use the same envelope with a stable code, kind, retryability, and static remediation. Covered: inbound listener bind failure, state directory failure, embedding provider failure, tool surface failure, unknown flag, missing argument, and invalid command invocation.

Each record names the failed operation and the safe resource identity only: listener name and address, configured state path, or upstream server id. Upstream URLs, query strings, headers, payloads, credentials, and wrapped Go error text never appear. `--help` remains intentional human-readable output, not an error record.

## Acceptance Criteria
- [ ] AC-1: Listener, state-directory, embedding-provider, and tool-surface failures each emit one JSON error record with a distinct stable `error.code` and `error.kind`.
- [ ] AC-2: Each record includes `operation`, `outcome: "failure"`, `error.retryable`, and a static `error.remediation`.
- [ ] AC-3: Resource context is limited to typed fields such as listener name/address, state path, or server id; no URL, header, payload, or raw error string appears.
- [ ] AC-4: Unknown flag, missing argument, and invalid command invocation emit typed JSON on stderr and exit 1.
- [ ] AC-5: `--help` output is unchanged and emits no JSON record.
- [ ] AC-6: Tests assert the code, kind, retryability, and absence of forbidden values for each failure.

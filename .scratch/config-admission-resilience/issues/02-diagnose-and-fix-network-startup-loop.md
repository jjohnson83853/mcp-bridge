# 02 — Diagnose and fix the network instance startup crash loop

**What to build:** The second bridge instance (`mcp-bridge-network`, separate config and state mount) reaches `running`, serves traffic, and stays up, with the failing startup stage and cause recorded in the ticket. It currently exits 1 every cycle and Docker restarts it indefinitely; its only diagnostic is the generic `internal_error` line that ticket 01 replaces. Diagnosis must use the live container's actual config and state directory, not a reduced reproduction. The fix may be code or configuration; the service must not be resolved by deletion or by dropping its features (its configured pins and upstream set remain).

**Blocked by:** 01 — the typed cause must exist before the live failure can be identified.

**Status:** ready-for-agent

- [ ] The failing stage and cause are recorded from the live instance, quoting the typed diagnostic produced by ticket 01.
- [ ] The failure is reproduced locally with the instance's actual config and state layout before the fix is applied.
- [ ] After the fix the container reaches `running`, passes its healthcheck, and serves a successful inbound MCP request.
- [ ] RestartCount does not increase across at least ten minutes of observation.
- [ ] A regression test covers the identified failure class; configured upstreams and pins are preserved.

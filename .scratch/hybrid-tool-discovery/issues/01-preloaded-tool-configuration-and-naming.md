# 01: Preloaded tool configuration bounds and wire naming

**What to build:** Operators can specify preloaded tools via `pins` in `config.yaml` with normative bounds enforcement and deterministic wire naming. Deliver configuration parsing, an exact upper-bound pin check (maximum 64 pins), server existence validation, and a canonical `<server_id>__<tool_name>` wire naming helper adhering to the 128-character limit.

**Blocked by:** None (can start immediately).

Status: complete

Type: task
Source: Hybrid tool discovery specification, revision HYBRID-DISCOVERY-r3. Coverage: User Stories 1–6, 26; Implementation Decisions 1, 2, 7; Contract Amendment 1.

- [ ] Enforce a strict upper bound of at most 64 configured pins (`max_preloaded_tools = 64`) during configuration validation, failing closed with a typed diagnostic if exceeded.
- [ ] Implement a canonical wire naming function formatting preloaded tools as `<server_id>__<upstream_tool_name>`.
- [ ] Validate that upstream tool names adhere to `^[a-zA-Z0-9_-]+$` and that the total wire name length does not exceed 128 characters (`len(server_id) + 2 + len(upstream_tool_name) <= 128`), failing closed with typed diagnostics if violated.
- [ ] Provide inverse mapping to extract the owning server ID and raw upstream tool name from a valid namespaced wire name.
- [ ] Update `catalog.CallToolInputSchema()` and `catalog.DescribeToolInputSchema()` tool_name pattern to: `^(t_[a-z2-7]{52}|[a-z][a-z0-9-]{0,31}__[a-zA-Z0-9_-]{1,94})$`.
- [ ] Preserve existing `pins` validation: each entry must contain exactly `server` and `tool` keys, reference a known server from `servers:`, have non-empty strings, and disallow duplicate pins.
- [ ] Verify that `Config.Fingerprint()` continues to track changes to `pins` deterministically.
- [ ] Add unit tests in `internal/config` covering valid pin bounds, boundary edge cases (>64 pins, >128 chars), duplicate pins, and fingerprint stability.

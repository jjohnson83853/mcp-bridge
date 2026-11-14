# Upstream breaker recovery — backlog

Status: needs-triage

Incident: pinned Home Assistant tool (`homeassistant__ha_search`) failed for ~90 minutes with
client-side `CONNECTION_CLOSED` on bridge A (`/mcp-bridge-network`), while grafana pins worked,
bridge B (same image, HA unpinned, called via `search_tools`/`call_tool`) succeeded, and restarting
bridge A fixed it immediately.

Evidence: `.scratch/upstream-breaker-recovery/incident-2026-11-14.md`
(Loki uid `P8E80F9AEF21F6940`, label `source_subsystem=<container>`; collected read-only by
`telemetry-scout` + `debugger`, no configuration read, no replay).

These are **pre-spec** items: evidence and the blocking question, not acceptance criteria. ACs,
requirement text and design are authored by the planner once the owner rules on the blocking
question — writing them here would pre-empt that.

## Proven

| # | Finding | Evidence |
|---|---|---|
| 1 | 14 pinned HA calls, 10:47:44Z–12:14:42Z, each failed at the bridge in 34–46 ms with `error.code=backend_unavailable`, `kind=dependency`, `retryable=true` | `backend_complete server=homeassistant` rows, bridge A |
| 2 | The client request did reach a bridge handler; the bridge emitted HTTP 200 and `mcp_complete stage=backend_dispatch` | `http_complete POST tools_call status=200 bytes=304 upstream_state=transport_failed` at 12:14:42.325Z |
| 3 | HA never went away: container up since 2026-11-12, RestartCount 0, healthy, `POST /mcp 200` from bridge A's IP in every 5-min bucket, no 4xx/5xx/session-termination lines | Portainer inspect + `hass-mcp` access log |
| 4 | HA truncates heavy `ha_search` responses: an 11.49 s success at 10:44:04 coincides with `smart_search … Automation config fetch budget exhausted (10.0s). Fetched 60/206` | `hass-mcp` WARN + `backend_complete duration_ms=11490` |
| 5 | Catalog was **not** frozen: a triggered reload succeeded 11:09:17 (pins 67/67, tools 841) and interval refreshes succeeded every ~300 s through 12:14:17 | `catalog_refresh_completed`, bridge A |
| 6 | The 10:30:30–11:09:00 refresh-failure series was `google-health-mcp` `dns_lookup_failed`, not HA | `catalog_refresh_failed` + `upstream_discovery_failed` rows |
| 7 | Only a new process cleared the failure: 12:19:56, same tool, 364 ms | `backend_complete … outcome=success` |
| 8 | Code: `RecordFailure` opens the per-server breaker at 3 transport failures; `AdmitCall` then rejects while `openedAt` is non-zero; `BeginProbe` is the only reset and has **zero production callers**; `RecordSuccess` is unreachable while the breaker is open | `internal/upstream/runtime.go:203-222,243-245,294-313` |
| 9 | A successful refresh/discovery never resets the breaker — discovery does not pass through `AdmitCall`, and a refresh reuses the same client | findings 5 + 8; `internal/service/reload.go:580-582` |
| 10 | SIGHUP/reload cannot clear it: an unchanged endpoint + credential fingerprint reuses the held `Client`, so the same breaker survives the reload | `internal/service/reload.go:580-582` |
| 11 | No pinned-only stream-termination path exists: upstream failures return an `IsError` JSON-RPC **result** with nil Go error, so they cannot close the downstream stream | `internal/service/direct_execution.go:45-49` |
| 12 | The pin blocker record contradicts itself: 10:34:47 `pin_blocker_detail blocker_class=tool_absent config.pin_tool=homeassistant__ha_search pins[30]` while its own `discovered_tools` chunk contains `ha_search` | pin blocker rows |

## Not proven

- Which side truncated the upstream stream on the failing calls. HA logged `200` and no error, the
  bridge logged `transport_failed` after 34–46 ms; HA logs no `Mcp-Session-Id` or duration, so
  per-call attribution is same-second only. The bridge does not log the 304-byte JSON-RPC error
  body it returned.
- Where the client's `CONNECTION_CLOSED` text comes from. The string exists nowhere in this
  repository; the bridge sent 200 + a JSON-RPC error result, so this is client-side rendering
  `[INFERENCE]` — no bridge-side stream close was observed.
- Whether bridge A's `homeassistant` server is `stateless: false`. The deployed config document is
  not readable read-only; default is `true` (`internal/config/config.go:48-53`). If it is
  `false`, finding 8 is joined by a second permanent wedge: `Dedicated.sessionFor` caches one
  `ClientSession` until `Close()` and never evicts it on transport failure
  (`internal/upstream/dedicated.go:294-309,417-427`).

## Pre-spec tickets

| # | Title | Blocked by | Blocking question |
|---|---|---|---|
| 1 | Breaker must regain a reachable close path | None | Is the half-open attempt driven by the refresh/reload discovery leg (a successful non-executing probe resets the counter, already scheduled every ~300 s), by a lazy admission check once `open_duration` elapses, or both? Both must never replay the failed execution (`ExecutionUnknown`). |
| 2 | Evict the cached upstream session on transport failure | None | Only reachable in `stateless: false` mode today. Does the fix belong in `Dedicated` (drop `session` on a transport-coded failure) or in `ServerClient` (report the failure kind to its owner), and must discovery evict too? |
| 3 | Attribute refresh failures to the server and reason | None | `catalog_refresh_failed` currently records one `error.code` for the whole pass, so a single `google-health-mcp` DNS failure read as an HA outage for 40 minutes. Which fields (`server`, `failure_stage`, `failure_reason`, `http_status`) and does the pin blocker's `tool_absent` classification get corrected in the same record? |
| 4 | Setting that pins every read-only tool | None | Owner story below. One switch expands the pin set from the explicit `pins` list to every catalog tool advertising `readOnlyHint: true`. Which scope does the switch carry (global / per-server opt-in), and does the 128-pin ceiling fail the candidate, degrade, or get replaced by the flag? |

Pick order follows the tracker convention: first open, unblocked, unclaimed by number. These do not
block each other; ticket 1 carries the incident, ticket 2 is the same failure class under a
non-default configuration, ticket 3 is what made the incident look like something else.

## Owner story (2026-11-14): pin every read-only tool

> As the bridge operator, I want one setting that, when enabled, declares every read-only
> upstream tool as a pinned tool, so clients see my fleet's inspection tools directly in their
> tool list instead of me enumerating dozens of `pins` entries by hand and re-editing them every
> time a server adds a tool. Off by default; enabling it is an explicit operator action, and
> the expandable pin set must never turn on implicitly.

Current mechanism this would replace or extend:

- `pins` is an explicit operator list `[{server, tool}]`, resolved to an exact
  `[server_id, raw_name]` binding at assembly build (`core.AddPins`, `internal/core/core.go`).
  A pin whose definition is missing fails the whole call with zero resolved pins; a config with
  more than 128 entries is rejected at parse (`pin_ceiling`,
  `internal/config/config.go:22,242-256`).
- Read-only status exists only as the upstream's own claim: `annotations.readOnlyHint`, stored
  verbatim in `ToolDefinition.Metadata`, validated for shape only, and documented as
  "hints are advisory claims, never authorization" (`internal/core/tool_metadata.go:77-108`).
- Scale today: bridge A holds 67 pins, 19 servers, 841 tools. A global expansion would blow the
  128 ceiling by an order of magnitude, and every pinned name is also repeated in the
  `search_tools` description (`internal/service/tools.go:99-104`).
- Pinned tools are per-generation and per-session projections; a refresh that adds or removes a
  tool changes what the next client generation sees.

Blocking questions (owner ruling required before a planner writes R/AC/design):

1. **Scope shape.** Global `pins.read_only: true`, per-server `servers[].pin_read_only: true`, or
   an allowlist of servers? Global is one switch but makes every noisy server's tool list part of
   every client's prompt.
2. **Trust.** Pinning on an upstream-supplied advisory hint means a server can make its tools
   direct tools by claiming they are read-only. Accept the hint, require a per-server opt-in, or
   require both (server opts in AND hint is true)?
3. **Ceiling.** Does the flag bypass, raise, or collide with `maxPins = 128`? If expansion exceeds
   it, must the candidate be rejected (kills reload), degrade with a recorded reason, or must the
   flag require an explicit ceiling of its own?
4. **Absent hint.** MCP does not default an absent `readOnlyHint` to true; the story assumes
   "absent = not read-only". Confirm, or include unannotated tools under a separate switch.
5. **Churn.** A refresh that discovers a new read-only tool would auto-pin it. Does the client get
   `notifications/tools/list_changed`, and is the expansion ordered deterministically so
   descriptor keys and catalog revisions stay stable across passes?
6. **Explicit pins.** Union with dedupe, and which wins when an operator explicitly pins a tool a
   server later annotates as read-only?
7. **Guidance surface.** With hundreds of pinned names, does the search description still enumerate
   them, or does it switch to a count plus `pins` in metadata?

Alternative worth ruling on rather than silently excluding: keep pins explicit and rely on
`search_tools`/`call_tool` for everything unpinned — that path kept working through the incident
above, at the cost of two round trips per call and no direct-declared tool.

## Operational mitigation until ticket 1 ships

Restart the bridge (`RestartCount` 0 → new process). A SIGHUP reload disables nothing — see
finding 10. Reading `backend_complete` `duration_ms` distinguishes a latched breaker (tens of
milliseconds, `backend_unavailable`) from a genuine upstream outage (real durations, mixed codes).
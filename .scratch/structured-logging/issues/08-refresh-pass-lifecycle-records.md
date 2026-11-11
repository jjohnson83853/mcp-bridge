# 08 - Refresh Pass Lifecycle Records

Status: ready-for-agent
Blocked-by: none

## What to build
Every catalog refresh pass becomes observable at start and at outcome, so a pass that fails after its candidate was built is recorded even though the previous catalog keeps serving. Today such a failure is silent: the pass error exists only as a discarded argument, and the only visible traces are count-based metrics that read as "not yet probed".

Start and outcome records share one correlation ID and state the configuration revision the pass used, the trigger that started it, and the next scheduled attempt. A failed pass states the service impact in the same record: either the previously published catalog is still being served, with its generation and tool count, or the bridge is serving zero upstream tools. A successful pass reports counts only - servers discovered successfully, tools discovered, tools published, mandatory pins resolved out of total - and never enumerates individual servers or tool names.

Severity follows the default contract: completion at info, failure at warn or error. Every failed pass repeats its complete diagnostic; no failure is suppressed as a repeat and no detail is obtainable only at debug level. This ticket is the carrier the later pin tickets attach their detail to.

## Acceptance Criteria
- [ ] AC-1: A refresh pass whose candidate is rejected emits exactly one pass-level failure record (`catalog_refresh_failed`); per-pin `pin_blocker_detail` records are additional and not counted by this oracle — warn when the retained catalog keeps serving, error when serving zero upstream tools.
- [ ] AC-2: Start and outcome records carry a shared correlation ID, the configuration revision, and the trigger; the start record carries the next scheduled attempt.
- [ ] AC-3: A failed pass states whether a previously published catalog is still served, with its generation and tool count, or that no upstream tools are available; it also reports the candidate's server and tool counts (counts only, never per-server or per-tool listings).
- [ ] AC-4: A successful pass reports servers discovered successfully, tools discovered, tools published, and mandatory pins resolved out of total, as separate counts.
- [ ] AC-5: A successful pass lists no individual server or tool name.
- [ ] AC-6: Each failed pass repeats its complete diagnostic; no deduplication and no truncation of detail.
- [ ] AC-7: Every `catalog_refresh_failed` record carries `level` `warn` or `error` exactly as pinned by AC-1 — positively asserted on each emitted record (`warn` with `retained_published`, `error` with `none_available`).

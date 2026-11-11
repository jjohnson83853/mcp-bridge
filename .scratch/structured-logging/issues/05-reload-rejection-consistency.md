# 05 - Reload Rejection Consistency

Status: ready-for-agent
Blocked-by: 02, 04

## What to build
A rejected reloaded candidate uses the same diagnostic contract as startup but different process behaviour. The record is `warn` with `outcome: "failure"` and the nested configuration error, the candidate is discarded, and the active configuration keeps serving. Startup rejection stays `error` with exit 1.

## Acceptance Criteria
- [ ] AC-1: An invalid reloaded candidate emits one `warn` record with the nested configuration error and the same attribution fields as startup.
- [ ] AC-2: The previously active configuration continues serving after the rejection.
- [ ] AC-3: The rejected candidate is not staged or published.
- [ ] AC-4: Startup rejection of the same document remains `error` with exit 1.
- [ ] AC-5: Tests assert severity, stream, retained configuration, and identical attribution between the two paths.

# 11 - Path Parity and Logger Fallback

Status: ready-for-agent
Blocked-by: 09, 10

## What to build
A config-validation pin rejection reports the same core pin diagnostic detail on the SIGHUP reload path (`config_reload_rejected`) and the configuration-validation path (`config_validation_failed`): the `pins[i]` property path with position, plus pin identity only where the diagnostic class carries validated identity; a runtime pin rejection on the reload leg is deferred to backlog ticket 19. No path may be the only way to obtain the useful error, so an operator never needs to send a signal to a running process to learn what a pinned tool rejection means.

In addition, the logging path itself becomes observable: when a record cannot be encoded or the writer rejects it, the process surfaces a minimal credential-free diagnostic where possible, so "nothing failed" and "failure logging is broken" are distinguishable states.

## Acceptance Criteria
- [ ] AC-1: A config-validation pin rejection (the same malformed-pin document) reports the same core detail on the SIGHUP reload path (`config_reload_rejected`) and the configuration-validation path (`config_validation_failed`): the `pins[i]` property path with position, plus pin identity only where the diagnostic class carries validated identity (absent for `pin_fields`/`pin_server_missing`/`pin_tool_missing`); runtime reload parity is deferred to backlog ticket 19.
- [ ] AC-2: Neither surviving path's pin failure is obtainable only through a SIGHUP reload — the configuration-validation record (`config_validation_failed`) alone carries the full AC-1 detail set.
- [ ] AC11-3 (11/AC-3; R25): a failed log encode or write surfaces `logging_write_failed` — minimal, credential-free — instead of failing silently, written as one JSON line to the lane OPPOSITE the one the failed record routed to (the declared D5 exception, AD21: failed stderr warn/error write → warn-level fallback on stdout; failed stdout debug/info write → fallback on stderr). test.

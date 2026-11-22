# 04 — Strict mode, config-loaded startup event, and documented storage base

**What to build:** Operators can choose fail-closed behavior with `MCP_STRICT_CONFIG=true`, which disables the last-known-good fallback entirely. Every successful config load, including a degraded start, emits a `config_loaded` event carrying the config file path and the resolved absolute storage directory, so the on-disk state location is visible at startup. Documentation states one explicit base for a relative `storage.directory` and the example config names it.

This ticket deliberately does not add a new rejection for a storage directory inside the config directory: the currently deployed healthy instance uses that layout, so a new hard failure would turn a documentation gap into an outage. That rejection remains a separate follow-up decision.

**Blocked by:** 03 — fallback semantics decide when `config_loaded` precedes a `config_degraded`.

**Status:** ready-for-agent

- [ ] `MCP_STRICT_CONFIG=true` plus invalid active config exits with the existing startup failure, even when a validated copy exists.
- [ ] The default (unset) keeps ticket 03's fallback behavior.
- [ ] A successful load emits one `config_loaded` with the config path and the resolved absolute storage directory.
- [ ] A degraded start emits `config_degraded` and still reports the resolved storage directory.
- [ ] `configs/config.example.yaml` and operator docs state the resolution base for `storage.directory`.
- [ ] Startup-envelope tests cover strict, default, and degraded paths.

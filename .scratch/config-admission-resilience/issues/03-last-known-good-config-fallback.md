# 03 — Serve a validated last-known-good config instead of crash-looping

**What to build:** When the active config fails validation but a previously validated copy exists, the process starts from that copy and serves, emitting `config_degraded` once with the config path, line/property, and the class of the rejected value, so the operator sees exactly what to fix. The invalid value is never accepted silently, the operator's config file is never overwritten, and the fallback copy never contains resolved secret material. When no validated copy exists, the process still fails closed with the existing startup-failure contract. The fallback must be generated only from a config that already passed validation in a previous successful start.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] Invalid active config with a validated prior copy: process serves and emits one `config_degraded` naming path, line, and property.
- [ ] The invalid value is never used; the served settings are the validated copy's values.
- [ ] The operator's config file is byte-identical after a degraded start.
- [ ] No resolved secret, key, or token is written into the fallback copy or the event.
- [ ] Invalid active config with no validated copy: existing startup failure, exit non-zero, no listener bound.
- [ ] A config that validates is persisted as the new last-known-good without rewriting the operator file.
- [ ] Tests cover degraded start, no-fallback start, persistence, and the no-secret property.

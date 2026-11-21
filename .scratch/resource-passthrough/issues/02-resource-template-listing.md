# 02 — Resource template listing (`resources/templates/list`)

**Type:** task

**What to build:** Parameterized upstream resources are proxied with the same bridge namespace while remaining expandable by clients. A template like `ha://states/{entity_id}` must still present `{entity_id}` as a substitutable variable in its bridge URI, and resolving the bridge URI after substitution must reach the exact upstream URI.

**Blocked by:** 01.

**Status:** ready-for-agent

- [ ] `resources/templates/list` returns the union of upstream templates with bridge-namespaced URIs; RFC 6570 expressions (`{var}`, `{+var}`, `{?q}`) remain intact and substitutable.
- [ ] With the declared variables substituted, the resulting bridge URI decodes to the upstream URI with exactly those substitutions — verified end to end against a template-serving upstream.
- [ ] Template `name`, `title`, `description`, `mimeType`, and annotations are preserved; a template without variables is listed and readable through the same path as a static resource.
- [ ] Template discovery shares the refresh/bounding/persistence behavior of static resources: bounded count, partial failure tolerated, last-known set served after restart.
- [ ] Round-trip tests cover variable-bearing, variable-free, and encoded-literal braces in the upstream path.
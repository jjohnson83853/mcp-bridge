# 02 — Server tags and aliases in configuration and metadata patches

**Type:** task

**What to build:** Operators can attach vocabulary a server's own tool descriptions never contain — `["firewall", "dhcp", "dns", "router", "vlan"]` for a router, `["vm", "lxc", "hypervisor"]` for a virtualization host — so natural-language discovery reaches that server's tools.

**Blocked by:** 01.

**Status:** ready-for-agent

- [ ] Configuration accepts per-server `tags` and `aliases`, validated for count, length, charset, and uniqueness; invalid entries fail configuration validation with the existing diagnostic style rather than being silently dropped.
- [ ] Metadata patches can add or replace a server's tags/aliases without editing configuration, and the patch wins over configuration deterministically.
- [ ] Tags and aliases participate in both BM25 and embedding text; a query matching only a tag ranks that server's tools above unrelated servers.
- [ ] Tags and aliases do not appear in `tools/list`, `describe_tool`, or any client-facing descriptor field that does not already carry operator metadata.
- [ ] Changing tags invalidates only the affected server's vectors; other servers keep theirs.
- [ ] Tests cover config validation, patch override, tag-only retrieval, and scoped vector invalidation.
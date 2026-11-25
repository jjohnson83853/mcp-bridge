# 05 — Default field projections for known list operations

**Type:** task
**Priority:** P1
**Status:** ready-for-agent
**Blocked by:** None — can start immediately.

**What to build:** Selected high-fan-out list tools return a compact default projection instead of their full object graph: container lists return the small identity/state set, and the endpoint/inventory list tools of the configured infrastructure servers return their obvious identity set. Callers opt into full objects explicitly, and callers can request a narrower or different projection. The full result remains retrievable, and this ticket does not create an unbounded escape hatch: unbounded output is still bounded by ticket 01’s ceiling.

**Notes:** Scope is an explicit allowlist keyed by server identity plus tool name, added only after reading each tool’s stored schema. Do not match on descriptive substrings or provider-name heuristics, which misfire on unrelated tools. Start with the two observed cases: the Docker container list and the Portainer endpoint list. Proxmox and OPNsense list tools are included only when their stored schemas have been inspected and their default fields are recorded in the ticket’s design. Defaults must be visible to the caller in the response so a projection cannot masquerade as the upstream’s actual shape.

- [ ] The allowlisted list tools return the documented compact projection by default, and the response states that a projection was applied.
- [ ] An explicit full request returns the upstream’s unprojected objects, still subject to the response ceiling.
- [ ] An explicit field selection overrides the default projection and rejects unknown fields deterministically.
- [ ] Non-allowlisted tools, including similarly named ones on other servers, are unaffected.
- [ ] Each allowlist entry is justified by the tool’s stored schema captured in the design, not by a name pattern.
- [ ] Errors, empty lists, and upstream shape changes degrade safely rather than dropping fields silently.
- [ ] Tests cover: default projection, full opt-in, custom selection, unknown-field rejection, unaffected neighbors, and a shape-change fixture.

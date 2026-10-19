# 05: Dynamic SIGHUP reload and tool list change notifications

**What to build:** The bridge supports dynamic reconfiguration of preloaded tools via SIGHUP reload. Adding, modifying, or removing pins updates the running tool set atomically without downtime, preserving Last-Known-Good (LKG) catalog definitions for existing pins during temporary backend outages, failing closed only on newly introduced invalid pins, and emitting `notifications/tools/list_changed` to connected clients.

**Blocked by:** 04.

**Status:** ready-for-agent

Type: task

Source: Hybrid tool discovery specification, revision HYBRID-DISCOVERY-r2. Coverage: User Stories 22–25; Implementation Decision 6.

- [ ] On SIGHUP reload or catalog refresh, reconcile the candidate generation's pins against the fresh catalog.
- [ ] Preserve Last-Known-Good (LKG) catalog definitions for existing pinned tools if their upstream is temporarily unreachable, marking their health as degraded/unavailable.
- [ ] If a candidate configuration introduces a *newly added pin* that cannot be resolved in the upstream catalog, reject the reload atomically, keeping the active generation running without interruption.
- [ ] On successful reload publication, atomically update the active preloaded tool registry.
- [ ] Broadcast `notifications/tools/list_changed` over active client sessions if supported by the transport.
- [ ] Ensure subsequent `tools/list` and direct `tools/call` invocations immediately reflect the new generation's preloaded tool set.
- [ ] Add integration tests simulating SIGHUP with pin additions, removals, and temporary upstream outages, verifying LKG preservation, atomic cutover, and updated `tools/list` responses.

# 03 — Go runtime and process collectors

**Type:** task

**What to build:** File-descriptor and goroutine pressure become visible before they become an outage. The standard Go and process collectors are registered on the bridge's own registry, so `process_open_fds / process_max_fds` can be alerted before the EMFILE failures previously seen on Docker LXC hosts.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] `go_goroutines`, `go_memstats_alloc_bytes`, `process_open_fds`, `process_max_fds`, and the standard process CPU/RSS series are exposed by the ops listener.
- [ ] Collectors are registered on the bridge's private registry only; the process default registry is not used, so several bridges in one process (tests, embedded operator) never conflict or panic.
- [ ] The collector set adds no labels that could grow with server, tool, or client input.
- [ ] Tests assert `process_open_fds` and `go_goroutines` appear in the exposition and that two independent instances can be constructed in one process.
# 01 — Per-server request duration histograms

**Type:** task

**What to build:** Operators can separate bridge overhead from upstream slowness. Every inbound operation and upstream call is timed into a histogram labelled by server, operation, and outcome, so `histogram_quantile(0.95, …)` answers "which upstream is slow" without log archaeology.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] `bridge_request_duration_seconds{server, operation, outcome}` is exported with buckets `0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60`.
- [ ] `server` uses the existing bounded server set (aggregate label where no server is attributable); `operation` uses the existing closed inbound operation vocabulary and grows only for operations the bridge actually serves; `outcome` uses the existing closed success/failure vocabulary.
- [ ] For upstream-backed operations the observation measures the MCP operation lifetime, not the enclosing HTTP lifetime; a client-cancelled or timed-out call is still observed once, with a failure outcome.
- [ ] Out-of-vocabulary labels are refused without creating a series, matching the existing metric guards.
- [ ] Tests assert the exposition shape, that a fast validation reject and a slow upstream land in different buckets, and label refusal.
# 09 — Durable snapshot write counters

**Type:** task

**What to build:** The last-known-good catalog file is the bridge's recovery asset, and its durability failures are silent today. Publish attempts and published size become visible, so a disk-full or permission failure that would cost state on restart is alertable before the restart.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] `bridge_storage_snapshot_writes_total{outcome}` is exported with `outcome` in `success` and `failure`, counting every atomic publish attempt including partial write, sync, rename, and lock-contention failures.
- [ ] `bridge_storage_snapshot_bytes` reports the size of the most recently and successfully published document; a failed attempt leaves it unchanged.
- [ ] Existing snapshot behavior is not altered: instrumentation must not turn a tolerated write failure into a request failure, and must not add retries.
- [ ] Neither metric carries a path, server, or free-text label.
- [ ] Tests cover successful publish updating bytes, a permission or disk failure incrementing `failure` while bytes stay unchanged, and lock contention being classified as failure.
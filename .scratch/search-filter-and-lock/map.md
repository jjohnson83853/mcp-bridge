# Search filter panic and storage lock recovery — task frontier

Canonical specification: [Search filter panic and storage lock recovery](spec.md).

| Ticket | Blocked by | Current state |
| --- | --- | --- |
| [01 — Search server filter index mapping](issues/01-search-server-filter-index-mapping.md) | None | resolved |
| [02 — Storage flock crash recovery](issues/02-storage-flock-crash-recovery.md) | None | resolved |

## Decisions so far
- Implement via TDD with `engineer` subagent.
- Ticket 01 resolved: `internal/search/search.go` maps global indices to eligible slice indices, dropping non-eligible documents and preventing out-of-bounds slice indexing panics in `fuse()`.
- Ticket 02 resolved: `internal/storage/store.go` uses `syscall.Flock` on `.writer.lock`, allowing automatic recovery over stale lock files after process crashes while enforcing concurrent writer exclusivity.
- Verified with full test suite passing across all 11 packages and source-line coverage at 91.6% (exceeding the >=90% gate).

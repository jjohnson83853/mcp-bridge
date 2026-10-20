# 02: Storage flock crash recovery

**What to build:** In `internal/storage/store.go`, replace `os.O_CREATE|os.O_EXCL` with advisory file locking (`syscall.Flock` with `LOCK_EX | LOCK_NB`) on `.writer.lock`. This allows restarted processes to acquire the lock even if a previous process crashed without running `Close()`, while still preventing multiple concurrent live writers.

**Blocked by:** None.

**Status:** resolved

Type: task

Source: `.scratch/search-filter-and-lock/spec.md`.

- [x] In `internal/storage/store.go`, update `Open` to open `.writer.lock` with `os.O_CREATE | os.O_RDWR` and call `syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)`. Return `ErrWriterLocked` when `flock` returns `EWOULDBLOCK` or `EAGAIN`.
- [x] In `internal/storage/store.go`, update `Close` to release the lock with `syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)`, close the file, and delete the lock file.
- [x] In `internal/storage/store_test.go`, add a test verifying that when a process terminates/closes its file descriptor without deleting the file, a subsequent `storage.Open` succeeds without `ErrWriterLocked`.
- [x] Verify `go test ./internal/storage` passes.

## Answer

Implemented in `internal/storage/store.go`. `Open` now uses `os.O_CREATE | os.O_RDWR` and acquires an advisory lock via `syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)`. When a process terminates, the kernel drops the lock descriptor, allowing clean restart recovery over stale `.writer.lock` files while still enforcing single-writer exclusivity for concurrent live processes.

Verified by `TestOpenRecoversStaleLockfileWhenProcessNotAlive` and `TestOpenCreatesPrivateLayoutAndExclusiveWriter` in `internal/storage/store_test.go`. Full package tests pass.

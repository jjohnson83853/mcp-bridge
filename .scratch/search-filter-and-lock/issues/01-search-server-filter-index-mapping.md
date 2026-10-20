# 01: Search server filter index mapping

**What to build:** In `internal/search/search.go`, translate global document indices returned by `s.index.rank()` into indices of the `eligible` slice when `request.Servers` is filtered, dropping documents from non-eligible servers. In `internal/search/search_test.go`, add a reproducer test with multi-server corpus where total documents > eligible documents and assert that search with `Servers` filter never panics and returns only tools from the specified server.

**Blocked by:** None.

**Status:** resolved

Type: task

Source: `.scratch/search-filter-and-lock/spec.md`.

- [x] Write a reproducer test in `internal/search/search_test.go` where total documents > eligible documents and `Servers` filter is passed, confirming RED (panic: index out of range).
- [x] In `internal/search/search.go`:
  - When constructing `eligible`, construct `globalToEligible := make([]int, len(s.snapshot.Documents))` initialized to `-1`.
  - Filter `lexical := s.index.rank(...)` through `globalToEligible`, appending only mapped entries (`globalToEligible[idx] != -1`).
- [x] Confirm GREEN on the reproducer test.
- [x] Verify `go test ./internal/search` passes.

## Answer

Implemented in `internal/search/search.go`. `eligible` now constructs and returns `globalToEligible []int` mapping global snapshot indices to eligible slice indices. `Searcher.Search` translates `rawLexical` via `globalToEligible`, filtering out non-eligible documents and ensuring all indices passed to `fuse()` are bounded by `len(eligible)`.

Verified by `TestServerFilterPanicWhenCorpusLargerThanEligible` in `internal/search/search_test.go`. Full package tests pass.

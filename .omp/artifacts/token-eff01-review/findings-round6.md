# Ticket 01 spec — review round 6 findings ledger

Candidate revision r6 (`ticket01-bounded-results-v6`), verdict REQUEST CHANGES (bound).
Observation: `.omp/artifacts/token-eff01-review/observation-code-reviewer-r6-d7a35b93717d.json`.
Review budget (owner allocations): default five rounds were consumed; the owner reset to 5 and then raised the allocation to **6**. Consumed under the current allocation: **1 of 6**, 5 remaining.

## Verified adequate (do not regress)

Measured form and duplicate-once accounting; no recursive paging; crypto-random handles with `{sessionID,epoch}` binding and indistinguishable `invalid_handle`; shape-before-store and range-after-lookup; per-session plus aggregate bounds with FIFO and no cross-session eviction; error class across preview and pages with small dispatch failures; `additionalProperties:false` schema, description tokens, README contract, no `full` collision; discovery seam after `commitDiscovery`; T1–T7 edges with per-task oracles otherwise coherent.

Round-5 closures M1 (feasibility validation exists), M2 (scope stated), L1, L2, L3 hold as *directions*; the findings below refine them rather than reopening them.

## Open findings

| ID | Class | Sev | Anchor | Defect |
| --- | --- | --- | --- | --- |
| F1 | statement-contradiction | HIGH | §4.4, §4.5, AC1/AC3/AC13 | The feasibility constants bound structured metadata only: they omit the content-block wrapper, the `isError` field, and any worst-case serialization of the first content byte. A config the stated validation accepts (metadata = ceiling − 1) with a `"` boundary byte yields no non-empty prefix. Bound the complete serialized preview or page minus content, plus a single-byte worst-case reserve. |
| F2 | lifecycle-hazard | MEDIUM | §4.5, §4.4, AC4 | Pages are sized against "the configured delivery ceiling", but handles survive a supported reload that changes `delivery_ceiling_bytes`. The reload either shifts page boundaries, breaking R4's identical-bytes promise, or leaves pages above the new ceiling. Pin the ceiling at retention time or invalidate handles on change. |
| F3 | ac-not-falsifiable | MEDIUM | AC8, T2/T3 | AC8 assigns the caller-visible `result_expired`/`result_evicted` returns and their log events to T2, whose scope forbids handler wiring, and omits the retrieval handler owned by T3. |
| F4 | ac-not-falsifiable | MEDIUM | AC12, T7 | `go test ./... -coverprofile` neither runs the repository's 90% gate (`go run ./cmd/covcheck`) nor uses `-coverpkg=./... -covermode=atomic` as `.github/workflows/verify.yml:48,51` does. |
| F5 | ac-not-falsifiable | MEDIUM | §4.5, T2 | The periodic TTL sweep has no oracle and no lifecycle owner in T2's write roots; a per-request sweep cannot reclaim entries for a live idle session, and a goroutine in `inbound.go` or `result_handle.go` has no start/stop contract. |
| F6 | statement-contradiction | MEDIUM | §11 OD2, §10 | `8 MiB (= transport default)` is not headroom, and "`too_large` unreachable under default config" is false: the measured form duplicates content and structured forms plus escaping, so a near-8 MiB upstream result serializes above `max_retained_result_bytes`. |
| F7 | ac-not-falsifiable | MEDIUM | AC10 | R10's retainable-branch truncation marker has no oracle: AC10 asserts only the byte-exact code, and AC16 covers the unretainable branch. |
| F8 | statement-contradiction | MEDIUM | T6, `README.md:467` | "Executions return the upstream result unaltered inside a provenance envelope" becomes false over-ceiling but is absent from the changed-claims list and T6's negative greps. |
| F9 | statement-contradiction | LOW | §4.2 | `upstream_code` is specified as `<string|null>` and also as "otherwise omitted"; the serialized form and byte-accounting fixture differ. |

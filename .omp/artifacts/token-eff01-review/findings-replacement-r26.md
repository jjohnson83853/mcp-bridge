VERDICT: REVISE

**Mode:** Specification review (candidate `r26-replacement`, on-disk bytes). **Scope:** requirements R1–R16, ACs, tasks, §6–§8.

### Findings

**[MEDIUM] F1 — Terminal sizing oracle cites the wrong rule.**
Section: §4 AC11 vs §2 R10.
Defect: AC11 requires each terminal's `Content` to be "the longest prefix that fits under **R5's** pinned sizing rule", but R10 defines terminal sizing separately and explicitly states "(R5's rule covers preview/page envelopes only)." The two envelopes carry different metadata (preview: `paging_required`/`handle`; terminal: `retention`/`execution_state`), so the longest-prefix literal differs.
Consequence: a test written from AC11 may derive the wrong literal length for the golden prefix, failing a correct implementation or accepting a wrong one — the independent-byte-oracle invariant is not unambiguous for terminals.
Fix: AC11 must cite R10's terminal sizing rule (the same greedy pass with R10's terminal metadata at R11(g)'s pinned maximum widths), and should state the literal is derived from R10, not R5.

**[LOW–MEDIUM] F2 — Retirement's handling of live markers is contradictory.**
Section: §2 R6 (charge disposition and "only R16's marker cap evicts a marker"; "R16's marker-cap eviction is the sole exception") vs §2 R7, §4 AC38, AC43.
Defect: R6 says retirement "releases every record it held (entries, their reservations and **live markers**)", and AC43 asserts that delta, while R6's admission paragraph says only the marker cap evicts a marker and R7/AC38 define retirement infeasibility using only "expired/evicted records" then FIFO entries — a phrase never defined to include live markers.
Consequence: whether a retirement commit may reclaim its namespace's live-marker charges before testing whether the tombstone fits is unspecified; AC38's admission outcome and AC43's "exact aggregate delta" can differ between implementations.
Fix: scope "only R16's marker cap evicts a marker" to non-retirement admission, and state in R7/AC38 that the retirement commit releases the namespace's live markers (or explicitly excludes them) before the tombstone-fit test; name that released set in AC38.

**[LOW] F3 — Exact clearing instant is unpinned.**
Section: §2 R7 ("first sweep whose clock **exceeds** its creation time plus `tombstone_ttl`"), R16 ("clearing deadline — the expiry/eviction instant plus `tombstone_ttl`"; "cleared at `tombstone_ttl`"; "only after that lifetime ends"), R4, AC5.
Defect: the boundary at `transition + tombstone_ttl` is stated in three ways ("exceeds", "deadline … at", "after … ends"). Entry expiry is inclusive at `handle_ttl` (AC5 "from `handle_ttl` onward"), but marker/tombstone clearing is not pinned at its endpoint.
Consequence: at exactly the deadline, the returned code (`result_evicted` vs `invalid_handle`) and the 256-byte aggregate charge are unspecified; a clock-pinned oracle can pass two different behaviours.
Fix: state one half-open rule (e.g., live while `now < transition + ttl`; cleared, charge released, and `invalid_handle` at `now ≥ transition + ttl`) and pin it in AC5/AC18/AC39.

### Positive observations
- The six carried invariants are present and each has an explicit oracle: independent golden literals (R1/R2, AC1/AC2/AC6/AC37/AC40), marker/tombstone lifetime and accounting (R7/R16, AC5/AC8/AC18/AC39/AC43), pinned `upstream_code` (R8/R9, AC9/AC10/AC29), closed label registries with producers (R12, AC13/AC44), per-terminal bounds (R10/R14, AC11/AC26), and ceiling/env-feasibility validation (R11, AC12/AC20–AC23/AC33–AC35/AC41/AC42).
- Cross-references are mostly single-sourced (`§3` defers to R5 for sizing; ACs name concrete test functions and task owners), and task write roots are exact-file scoped with staging dependencies.

### Recommendation
REQUEST CHANGES
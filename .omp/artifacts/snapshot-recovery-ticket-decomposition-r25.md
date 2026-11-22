# snapshot-recovery — ticket decomposition proposal (4 tickets)

**Status:** draft proposal, not authorized. Re-decomposing the approved T1–T3 task set changes the approved envelope
(`env-1.allowedTaskIds`), so it requires explicit owner authorization before any planner/spec change.
Existing R/AC ids are preserved by mapping; no new R/AC/T ids are invented here.

**Why:** rounds 12–14 fixed fixture-level counterexamples while the same failure family (moved/quarantined
reconciliation identity, protection membership, persistence after reconciliation) kept reappearing. Each ticket
below owns one invariant end to end, plus one cumulative counterexample suite shared across tickets and one
end-to-end gate.

**Shared contract (non-ticket, one owner):**
- identity of a configured link = resolved containing-directory context + entry identity (D11), never bare inode;
- moved-entry record = original configured path + pre-move entry identity (D12);
- ordering, retention and preservation rules are stated once and referenced, never restated per ticket;
- every ticket's tests consume one cumulative fixtures file so a counterexample cannot be lost between tickets.

---

## Ticket SR-1 — Quarantine core: classify, preserve bytes, warn, continue

**Type:** task | **Status:** draft | **Blocked by:** shared contract registration
**Owns:** R6, R7, R9 (classification/reason vocabulary, quarantine naming + retention bound, one warn record);
AC1-1..AC1-4; startup-side effect in AC2-1..AC2-3.

**Required behavior:** unreadable/rejected `snapshot.json` or `prompts.json` is recoverable startup state:
atomic rename to a bounded quarantine name in the same directory, bytes preserved, one typed warn record
(`startup_snapshot_rejected`, finite `failure_reason`, `resource.path` = quarantine path), continue with no
restored catalog, publish fresh generation. Never delete/truncate/overwrite first. Retention bounded (5 newest).
Unwritable directory, non-file at the path, or rename failure stays fatal as `state_directory`.

**Out of scope:** symlink chains, reconciliation, retention interaction with protected families.

---

## Ticket SR-2 — Retention and protection: what the quarantine may delete

**Type:** task | **Status:** draft | **Blocked by:** SR-1
**Owns:** R5, D10, D11; AC1-10(base), (b-realias), (b-revisit), (b-cycle), (b-hardalias), (b-hardalias2),
(b-hardcycle), (b-budget).

**Required behavior:** family bounds (5 newest per family), protection membership by entry identity + cleaned
path, deterministic eviction order, backing-byte preservation when a quarantine link or protected entry depends
on the evicted file. Expansion of a configured symlink chain terminates on finite resolution, on the 4097th splice
(either of the two 4096 bounds), or on a true ancestor cycle — each with its own classification, and a finite
alias (including hard-linked symlinks in different directories) must never be classified as a cycle.

**Out of scope:** deciding whether to reconcile a dangling configured link; startup sequencing.

---

## Ticket SR-3 — Configured-link reconciliation: ordinary and mutual loops

**Type:** task | **Status:** draft | **Blocked by:** SR-1 (records), SR-2 (retention after reconciliation)
**Owns:** R2, R8, D12; AC1-10(b-x), (b-xA2), (b-dangle); AC2-9 (all subcases: orphaned-link, mutual pair,
both-moved, self-moved, hard-linked mutual variant, non-mutual carve-out).

**Required behavior:** at most one reconciliation per run; eligibility requires the configured entry to still be a
live `lstat` symlink whose `Stat` is `ErrNotExist` AND it is not in the moved-entry set keyed by
(original configured path, pre-move identity); partner-versus-self is decided from recorded chain-entry paths,
never inode alone; reserve+rename discipline, link bytes preserved (`Readlink` asserted), one `orphaned_link`
record, deterministic warning order, R3-fatal on rename failure, second run a no-op, retention pass after
reconciliation.

**Explicitly preserved limitation:** the originally-dangling NON-mutual carve-out remains OPEN and must be pinned
by its own AC2-9 subcase, not silently passed.

**Out of scope:** startup admission ordering; whether fresh `prompts.json` is accepted (SR-4 owns that oracle).

---

## Ticket SR-4 — Startup integration and the end-to-end gate

**Type:** task | **Status:** draft | **Blocked by:** SR-1, SR-2, SR-3
**Owns:** R1, R3, R4; AC2-1..AC2-8; the cross-ticket end-to-end suite.

**Required behavior:** the whole startup path proves the sequence: open → recover (quarantine/reconcile) →
admission → publish → prompt persistence → restart no-op. Includes the mutual-pair restart oracle (fresh valid
`snapshot.json`, fresh regular `prompts.json` accepted by direct `SavePrompts`; refusal FAILs), the both-moved
and self-moved no-fire cases, exactly-once counting per run, and the single fatal classification for
unrecoverable cases. This ticket owns the integration test that would have caught the r22→r24 regression family.

**Acceptance gate:** all four tickets' suites plus the cumulative fixtures file pass in one run; no fixture may be
removed, renamed or weakened when its defect is claimed closed. Closure claims must cite the surviving fixture.

---

## Notes for the owner / planner

- This proposal keeps the 14-file envelope unchanged; it changes task boundaries and test ownership only.
- Existing T1/T2/T3 content maps as: T1 → SR-1+SR-2, T2 → SR-3, T3 → SR-4.
- Applying it requires owner authorization (task-id set change) and a planner-owned specification update, then fresh
  independent reviews. Nothing here authorizes implementation, a waiver, or resumption of a blocked review.

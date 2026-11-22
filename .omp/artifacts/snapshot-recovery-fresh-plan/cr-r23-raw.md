# Code Review — snapshot-recovery spec r23 (specification mode; no implementation)

**Findings**

1. **HIGH** `[statement-contradiction]` `.scratch/snapshot-recovery/spec.md:83` (§4(3)), confidence HIGH — the reconciliation predicate reads "in both classes, whose entry now `Stat`s `fs.ErrNotExist` **or is absent**". An absent configured entry satisfies it for the ORDINARY class too, so AC1-10(b-xA2) (`:122`: snapshot link moved in pass 1, its target `prompts.json` moved in pass 2) spuriously reconciles — a third `Recovery`/fatal rename — contradicting D12 (iv) (`:69`) and b-xA2's "no reconciliation fires … nothing is left to reconcile". Fix: delete "or is absent"; require the entry to still exist as a symlink post-pass.

2. **MEDIUM** `[lifecycle-hazard]` `:69` D12 (m-iv), confidence HIGH — "or the entry is absent" lets the mutual detection reconcile an entry this run itself quarantined: `snapshot.json→prompts.json` with `prompts.json→prompts.json` makes both sides `Stat` ELOOP, both move; (m-iii) passes via the partner's move, (m-iv) via absence ⇒ reserve succeeds, rename of a missing source fails ⇒ R3 fatal (exit 1), and two reconciliations break D12's own at-most-one argument. R2 (`:16`) likewise presumes a present link ("link bytes preserved"). Fix: apply the ordinary (iii) guard — configured entry not in the moved-entry set.

3. **LOW** `[verification]` `:131` AC2-9 mutual subcase, confidence HIGH — the reconciled quarantine's preserved link text is never asserted (no `Readlink`→`snapshot.json`), and the mutual class's post-reconciliation retention is unseeded (only ordinary b-xB-res). Fix: add the Readlink assertion plus one seeded variant.

4. **LOW** `[precision]` `:68` D11, confidence MEDIUM — "resolved containing-directory context" is never pinned to path string vs directory entry identity (`:122` uses a path). Path spelling sends a true cycle through a directory alias to the budget class instead of the cycle class (harmless, unoracled). Fix: define it as the containing directory's `lstat` identity, path as fallback.

5. **LOW** `[statement-precision]` `:113` AC1-1(d), confidence MEDIUM — cites only (m-iii) for the mutual no-fire; a pre-pass dangling chain fails (m-ii), the deciding condition. Fix: cite (m-ii)+(m-iii).

6. **LOW** `[editorial]` `:131,:164,:167,:191` — "non-mutal" typo (5×). Fix: "non-mutual".

**Closure**

- **C4: VERIFIED CLOSED.** Traced (b-hardalias) `a`→`d/l`→`d/d/l`→`q5`: context+identity keys splice finitely, `q5` recorded (entry identity+path), protected, `q4` evicted, read-through resolves, link moves. (b-hardalias2) three same-inode links in three containing dirs traverse finitely; (b-hardcycle) `x2` shares `x`'s inode *and* directory ⇒ same key ⇒ true cycle stops as `read_error` with ordinary eviction. Key is sound (same inode+context+target text ⇒ identical continuation); popped/different-context revisits continue; membership unchanged; both 4096 bounds retained.
- **CR22 LOWs: all three closed** — (b-budget) `k4097`→`q5` makes the trip observable; either-bound wording; R9 rescoped and names the originally-dangling limitation.
- **C5: PARTIAL** (findings 1–2). The clean mutual pair traces correctly end-to-end: pass-1 `read_error`, pass-2 dangling no-op, own-family rename with `orphaned_link`, order [snapshot, prompts], R3 discipline, retention pass, second-run no-op; AC2-9 discriminates (unreconciled ⇒ catalog bytes ⇒ `SavePrompts` refusal + second-run record). Owner direction is recorded in `round-12-seed.json`; §9.11/§8(2) keep retention-unlink and the NON-mutual carve-out OPEN — no false claim.
- **Regression OK:** no id/envelope change (14-file union = 4+6+4); std-lib only (`os.Link`); `store.go:104-106`,`:122-126` anchors correct; `publishFile` renames over the symlink, so "fresh regular `prompts.json`" is mechanically right; §9.7's stale-mirror account accurate; no review/approval claim.

**Limits:** no hashing tool — supplied SHA-256 identity unverified; no tests/lint run by me (spec-lint's 20 findings are pre-existing mirror-drift/shared-oracle).

## Verdict: REQUEST CHANGES
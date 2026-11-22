## Code Review Summary

**Mode:** specification (initial), `snapshot-recovery-spec-r19@r19`; no implementation. Reviewed: spec, ticket mirror, logging contract, 8 source anchors.
**Limits:** no hash/shell tool → supplied SHA-256s are unverified byte-wise; no tests exist or ran; spec-lint not run. The ticket mirror is stale as stated (line 7 `r7`, line 11 `SNAPREC-SPEC-r3`) and was not treated as contract.

### Closure (verified against current source)
- **F1 CLOSED** — entry-identity candidacy plus per-element lexical fallback agree across R2:16, R5:19, D10:64, §4:79, §8:198, T1:135; candidate comparison never uses `Stat`. AC1-10(b-eloop):118 seeds c4/c5 as the oldest, asserts both survive and c3 (not c5) is the sole eviction; r18's Stat candidacy would instead evict c5. Cycle termination by revisited identity is coherent with `lstat`, which never loops.
- **F2 CLOSED** — D11:65 (also R5:19/§4:79/§8:198/T1:135) resolves each relative target against `filepath.Dir` of the link just read. (b-sub)/(b-sub2):118 resolve `<state>/sub/../snapshot.corrupt-q5.json` to q5, preserve q5 bytes, evict q4, leave `sub/l1`/`alt/l2` intact, and assert chain read-through.
- **L1** design closed (entry identity; four-slot bound and >4 carve-out; AC counts 5/5/5/5 and 6 agree). **L2** (b-xA2)/(b-xB2):118 pin the owning pass, 2-vs-1 `Recovery` counts, quarantine bytes, and absent configured path. **L3** (b-mix):118 + §8(2):198 + §9.11:214. **L4** (b-vac):118 Modes 1–2.
- **Regression OK** — no R/AC/T id change; §6.1 remains 14 files = T1(4)+T2(6)+T3(4); `go.mod` unchanged (std-lib `Lstat`/`SameFile`/`Dir`, test `Mkfifo`); Load/Save/LoadPrompts/SavePrompts/ensureReplaceable preserved (R1:15, D7, T1 prohibited); C6/CR15 retained (AC1-3:111, (b-prot):118, AC1-9(a2):117).
- **Anchors verified:** wiring.go:486/:492/:494; startup_failure.go:26/:29/:30; envelope.go:103/:136 (exact equality)/:575; log.go:228, default 275; config_envelope_test.go:166/:169 (129); field array 668–669 leaves `quarantine_path` correctly sorted; docs alert queries match the claim. T2 needs no out-of-envelope edit: `logging_contract_test.go` pins only its own 23 emitted events, and `WriteJSONLine` bypasses `validErrorInfo`, so the off-arm record is unchanged by adding `snapshot_unavailable`.

### Issues
**L-1 [LOW][statement-precision] spec.md:111** — Confidence HIGH. "A conforming symlink whose `Stat` resolves into a protected target is NOT itself protected" is overbroad: (b-chain):118's `q1` is exactly such a symlink and is protected by its own recorded entry identity. R5:19 correctly says "merely". Fix: add "that is not itself a recorded chain element".
**L-2 [LOW][statement-precision] spec.md:19, :79, :198** — Confidence HIGH. The C2 count formula ("excess of unprotected … above the four surviving slots") yields 0 for (b-chain):118 (3 unprotected, 1 protected, 1 deletion required) and for (b-prot)/(b-sub); the operative rule and every AC count are correct. Fix: ordinary count = pre-existing − 4; mixed count = full unprotected set.
**L-3 [LOW][coverage] spec.md:111; R5:19** — Confidence MEDIUM. L1's non-over-protection behavior and the out-of-directory vacuous mode are declarative only, with no seeded oracle. Fix: seed a pre-existing conforming symlink resolving into a protected target and assert oldest-first eviction.
**L-4 [LOW][editorial] spec.md:16, :40** — Confidence HIGH. R2:16 has a stray backtick after "just read`"; §3:40 cites LoadPrompts branches at `:93-94`/`:96-97`/`:100-102`/`:103-107`, but store.go returns at `:95-96` (decode), `:98-99` (trailing), `:101-102` (Validate), `:104-106` (checksum).

### Positive
(b-eloop)'s oldest-cycle seeding, the per-hop resolution arithmetic, and the (b-mix)/(b-vac) "no diversion, no extra record" oracles are precise and testable; anchor discipline is otherwise exact.

### Recommendation
APPROVE

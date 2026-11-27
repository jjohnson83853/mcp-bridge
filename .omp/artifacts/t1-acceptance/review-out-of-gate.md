# Out-of-gate read-only review — T1 (owner exception, 2026-11-27)

Reviewer: `code-reviewer` (independent, read-only). Scope: the seven T1 files plus spec r40.
Authorization: `.scratch/token-efficient-tool-responses/decisions.md` — "Owner exception: out-of-gate read-only review of T1".

## Result

**No blocking (HIGH) defect.** Implementation conforms to R2 and R11(a)–(k) as written.

| # | Class | Sev | Anchor | Finding | Fix |
|---|---|---|---|---|---|
| 1 | Test adequacy | MEDIUM | AC23, R11(g) | The `terminal` subtest was byte-identical to `preview` (delivery 816, preview 727); terminal-infeasible implies preview-infeasible at the pinned widths, so breaking the terminal computation left the suite green | Assert the helper directly for terminal widths (727 reject, 728 accept) |
| 2 | Spec gap (latent runtime risk) | MEDIUM | R11(g), AC23, AC28 | The pinned maximal envelope omits the outer `CallToolResult.IsError`; a real error result with `IsError:true` serializes ~+14–15 B beyond the checked envelope, so a config admitted at exactly 6 B headroom can exceed the ceiling for error results | Fix spec-side: pin `isError:true` in the maximal envelope or add a fixed +15 B before T3 sizes real previews — planner-owned, NOT applied |
| 3 | Diagnostic attribution | LOW | R11(j), AC42 | (j) evaluated after (c)/(g)/(h), so `max_retained_result_bytes: 4294967296` alone answered `namespace_cap_order`, leaving `entry_cap_bound` reachable only with every other cap raised | Hoist the (j) comparison after the numeric parse loop |
| 4 | Dead code | LOW | R11(g)/(j) | Tautological arm in the `envelope_feasibility` key selection; unreachable `entry_cap_bound` branch in the parse-error path | Delete both |
| 5 | Test gap | LOW | AC22, AC34, AC42 | No `ttl_invalid` case for ≤0 durations; no acceptance boundaries at `max_upstream_code_bytes` 11/10 or `max_retained_result_bytes` 4294967295/4294967296 | Add those subtests |

Findings 1, 3, 4, 5 were dispatched for repair under T1's write roots. Finding 2 is a spec-level pin and remains open for the planner before T3.

## Verified pass

1. Relations (a)–(k) present with exact codes and triggers, including `entry_cap_bound` at > 4294967295 and `upstream_code_marker_bound` = `len("[truncated]")` = 11; eight codes in the closed array; docs block matches.
2. Feasibility uses R11(g) widths and +6: bases 210/298/210 B, +512 ⇒ 722/810/722, +6 ⇒ 728/816/728; the strict negation rejects at 727/815/727. Reviewer recomputed the pinned literals independently.
3. §6 defaults exact; overrides and zero-value `Config` verified.
4. Goldens are literals (136 B and 8197 B straddle 8192); duplicate occurrences all counted; `measureResult` equals wire bytes.
5. No T2+ leak; section-shape and unknown-key rejection preserved; `go build ./...` clean.

## Not verifiable by reading

AC28's runtime half; AC1/AC40 boundary behaviour; T3/T4 producer envelope shapes (not implemented); whether T3's preview sizing compensates for finding 2.

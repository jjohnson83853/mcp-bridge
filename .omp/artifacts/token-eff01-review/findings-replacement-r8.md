VERDICT: REQUEST CHANGES

**Review scope:** specification mode — on-disk `.scratch/token-efficient-tool-responses/spec.md` (`r8-replacement`), intake 01 (O1–O9), refinement R1–R12, sibling tickets 02/05/06, prior ledgers r6/r7, plus read-only source grounding (`session_exposure.go`, `log.go`, `envelope.go`, `httpdiagnostics.go`, `tools.go`, `direct_execution.go`, README anchors). No implementation exists; no command diagnostics were run.

**F1 [HIGH] — R5's per-page delivery ceiling has no acceptance oracle, and JSON re-escaping is unaccounted for.**
Sections: §2 R5, §3 Page/OD1, §2 R11(g), §4 AC6/AC17.
Intake O2 / refinement R3 require every retrieved page to be ≤ the ceiling. R5 states it, but no AC asserts a page's serialized size: AC6 checks concatenation and last-page zeros; AC17 checks rune validity only. Worse, R11(g) validates "maximal page metadata plus ≥4 bytes headroom", not the page value's marshaling; `Content` re-encoding expands `"`, `\`, control characters and `<`/`&` up to 6×, so a page cut at the raw ceiling can serialize well over it undetected.
Fix: add an AC asserting each page's serialized length ≤ the pinned delivery ceiling on an escaping-heavy multi-page fixture, and state in R5/§3 that page cut points are chosen against the serialized page value, not raw slice length.

**F2 [HIGH] — R12/AC13's `operation` "closed existing enumeration" does not exist, so AC13 has no valid enforcement point.**
Sections: §2 R12, §7 Finite labels, §4 AC13, §5 T2.
`operationVocabulary` (`internal/observe/httpdiagnostics.go:660`) is the only closed operation set: it contains `search`/`describe`/`call_tool`/`describe_tool`, but not `search_tools` or `retrieve_result`; `buildEnvelope` (`internal/observe/envelope.go:151`) validates only `Operation != ""`. Live records use `refresh`, `startup`, `reload`, `request`, `prompts_list`. Applying the four-surface set globally would reject those into `logging_write_failed`; leaving it unenforced makes "unlisted value fails the vocabulary test" false. T2's write roots cannot register either choice (`log.go`/`httpdiagnostics.go` absent except for the conditional new-reason route).
Fix: scope the operation enumeration to paging events, name whether it extends `operationVocabulary` or is new (and where), add that file to T2's write roots, and state that existing operations are unaffected.

**F3 [MEDIUM] — `upstream_code` has no defined producer for `search_tools`/`describe_tool`, yet AC19 requires one.**
Sections: §2 R8, §4 AC19.
R8 names producers only for `call_tool` and directly invoked projected tools. Discovery/describe results are bridge-generated with no upstream `is_error`/`code`, but AC19 demands "R8's per-surface `upstream_code` producer" on all four surfaces, leaving its oracle undefined for two.
Fix: state that discovery/describe results are always `is_error:false` with `upstream_code` absent (and assert that in AC19), or name their producer.

**F4 [MEDIUM] — `max_upstream_code_bytes` may be smaller than the pinned marker, making R8 unsatisfiable at a load-valid config.**
Sections: §2 R8, §2 R11(f)/(g), §6.
R8 requires the 11-byte `[truncated]` marker inside the bound, but R11(f) accepts any value >0 and R11(g) computes maximal metadata at that bound. `max_upstream_code_bytes = 4` passes load yet cannot render the marker inside the bound.
Fix: add the typed relation `max_upstream_code_bytes ≥ len("[truncated]")`, or define marker omission/shortening for smaller bounds and make R11(g) use that render.

**F5 [MEDIUM] — AC13's `TestLogLabelProducers` spans T3/T4, but T4 cannot write the test file.**
Sections: §4 AC13, §5 T3, §5 T4.
T3 writes the producer test into its own files (`bounded_delivery_test.go`/`retrieve_result_test.go`); T4's exact write roots are `tools.go`, `direct_execution.go`, `discovery_paging_test.go`, while its RED list says the same `TestLogLabelProducers` must fail there. Go cannot redefine a test function, and T4 may not edit T3's file.
Fix: split into `TestLogLabelProducersCallTool` (T3) and `TestLogLabelProducersDiscovery` (T4), or explicitly add the T3 test file to T4's write roots.

**F6 [MEDIUM] — Aggregate admission versus same-namespace FIFO eviction is unordered, making AC15 non-deterministic.**
Sections: §2 R6, §3 Terminal table, §4 AC15.
R6 permits per-namespace FIFO eviction and requires aggregate totals "after the commit" to stay within `max_bytes_aggregate`. If "after the commit" includes that namespace's own evictions, an aggregate-full result whose namespace can evict would be admitted, contradicting AC15's unconditional terminal expectation.
Fix: pin the order (aggregate check before or after same-namespace eviction) and state AC15's precondition accordingly, e.g. the target namespace has no evictable entries.

**F7 [LOW] — §3 Preview restates the bound on `Content`, contradicting R3's whole-preview measurement.**
Sections: §2 R3, §3 Preview. The design line says `Content` ≤ preview ceiling, while R3 measures the serialized preview value. Fix: state `Content` is a UTF-8-safe prefix and the serialized preview is ≤ the ceiling per R3.

**F8 [LOW] — T5's write-root statement contradicts itself on `serve.go`.**
Section: §5 T5. It declares `serve.go` "not a write root" then grants any proven inventory edit there as "unconditional T5 work". Fix: delete the conditional sentence or list `serve.go` as an exact write root.

**Positive observations.** The independent literal-byte oracles (R2/AC2/AC6), tombstone/marker lifetime with fixed 256 B accounting (R7/R16/AC5/AC8/AC18), pinned `upstream_code`/ceilings (R8/R9/AC10/AC29), exact terminal set with per-terminal bounds (R10/R11(g)/AC11/AC26), ceiling-order/envelope validation (R11/AC12/AC23/AC28), R13 precedence, AC36 namespace isolation, and single-sourced preview/page metadata are present and testable. No requirement was dropped relative to O3–O9 or R1–R12 except the page-ceiling oracle above.

### Recommendation
REQUEST CHANGES
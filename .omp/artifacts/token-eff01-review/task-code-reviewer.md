# Target

Review specification candidate `token-efficient-tool-responses-ticket01-spec` — the file `.scratch/token-efficient-tool-responses/spec.md`. Review its on-disk bytes; it declares its own revision. Read the shared instructions in the batch context first.

# Checks, in priority order

1. Byte accounting: does the spec measure the actual serialized client response, count duplicated content-block and structured forms once, and prove that both the 8192 ceiling and the ≤1536-byte preview are checks of that same form?
2. Retrieval termination: can a preview or a page itself become an over-ceiling payload (nesting)? Does the spec state a termination rule?
3. Handle security: crypto-random, content-independent, bound to session incarnation (session ID plus exposure epoch), indistinguishable rejection for unknown/tampered/foreign/other-incarnation handles. A reused session ID after rehydration must not resurrect handles.
4. Storage bounds and isolation: per-session and aggregate accounting, deterministic eviction, no cross-session eviction, defined concurrency behavior.
5. Error semantics: `isError` visibility in preview and every page; bridge dispatch failures keep their small typed shape; no paged success for a failure.
6. Compatibility: the retrieval surface must pass stored input-schema validation (`additionalProperties:false`) and be documented as a caller-visible contract; no `full` knob collision with ticket 05.
7. Regression: ceilings on discovery/describe must not break exposure, projection epoch, or catalog-revision admission; name the existing behavior that would fail.
8. Task ledger T1–T6: each task independently verifiable, dependency edges coherent, no task requiring an unstated capability or a sibling ticket.
9. Public documentation: which README claims change and which must be preserved.

# Deliver

`VERDICT: PASS` or `VERDICT: REVISE` on the first line, then findings ordered by severity with ID, affected R/T, one-sentence defect, spec section anchor, and observable consequence. Then list what you verified as adequate, so coverage is distinguishable from silence.

# Acceptance

- Every finding is anchored to a spec section and a requirement or task id.
- No implementation code proposals; name the invariant the spec must state instead.
- The worktree is unmodified.

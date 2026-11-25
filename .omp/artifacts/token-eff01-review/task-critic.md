# Target

Gate the acceptance contract of `.scratch/token-efficient-tool-responses/spec.md` (candidate `token-efficient-tool-responses-ticket01-spec`; review its on-disk bytes, which declare their own revision). Read the shared instructions in the batch context first.

# Your special focus

Attack the acceptance contract, not prose quality:

- Can a lazy implementation satisfy each R while violating its intent? Name the cheap false pass and the criterion that must close it.
- Does any R depend on unobservable state, an internal function, or a test that only proves a code path ran?
- Are `is_error`, `total_bytes`, `remaining_bytes`, page indices, and the 8192/8193 boundary unambiguous (inclusive versus exclusive, bytes versus runes, before or after escaping, text blocks versus the whole serialized response)?
- Does the spec silently resolve an owner decision the packet marks open, or contradict a settled ruling (delivery-only, no `full` knob, no projections, no dedup, session isolation and epoch invariants, existing error taxonomy)?
- Can a task in T1–T6 pass without its own falsifiable check?
- Is the handle store safe under many sessions, repeated retrieval, and eviction, with no unbounded growth and no cross-session interference?

# Deliver

`VERDICT: PASS` or `VERDICT: REVISE` on the first line, then:

1. Findings ordered by severity: ID, affected R or T, the cheap false pass or contradiction, spec anchor, and the criterion that would close it.
2. A per-requirement verdict for R1–R12: adequate, weak, or missing, each with one-line evidence.
3. Any requirement that a reviewer could not falsify against the built binary, named precisely.
4. Any silent resolution of an open owner decision, or contradiction with a settled ruling.

# Acceptance

- All of R1–R12 receive an explicit verdict; silence is not adequacy.
- Findings target intent-level holes, not wording preferences.
- No implementation proposals; name the invariant or oracle the spec must add.
- The worktree is unmodified.

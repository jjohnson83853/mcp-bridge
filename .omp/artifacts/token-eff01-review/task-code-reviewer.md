# Target

Review the replacement specification at `.scratch/token-efficient-tool-responses/spec.md` (ticket 01, bounded tool results with recoverable paging). It replaces an 80 KB predecessor that failed 27 review rounds without producing implementation; the owner ordered a concise replacement.

# Review stance

Judge THIS document on its own terms:
- Every requirement falsifiable and non-duplicative?
- Every acceptance criterion names an oracle independent of the implementation it tests?
- Every task owns only the behaviour it introduces, with exact write roots?
- Are the six carried correctness invariants present and testable — independent byte oracle; retired-marker lifetime and accounting; pinned `upstream_code`; finite label enumerations with producers; per-terminal preview bounds; ceiling-order validation?
- Is anything stated twice, or too vague to test?

Do NOT ask for the predecessor's length, its review-history tables, or its per-surface repetition. Do NOT reopen what section 8 (NON-GOALS and OPEN DECISIONS) deliberately scopes out, unless a non-goal hides a correctness hole.

# Output

Verdict: APPROVE or REQUEST CHANGES, then findings with severity, each naming the exact section and the fix. Keep the whole review under 900 words.

## Code Review Summary

**Mode/pass:** specification, initial (r15). **Files Reviewed:** 18. **Total Issues:** 5 (0 CRITICAL/HIGH/MEDIUM; 5 LOW).

**Closure (source-checked):** per-family retention filter, fs.ErrExist advance, R7 carve-out, fatal-first ordering, AC1-6 full shape + fixed message, T2 producer strings, five-reason table, LoadPrompts/SavePrompts negatives, tie-break, empty-string env, R2 stat-error: all closed. C5: R5/D10/§4(3)/AC1-5/AC1-6 agree on select-once, attempt-once, first-failure-ends-pass, zero-attempt listing, permitted excess; second-unlink over-deletion is caught by the exact-one-attempt + named-attempt assertions. Not closed: ticket account (F1), AC1-6 ownership (F2).

**Envelope/ids:** R1–R9, AC1-1..10, AC2-1..9, T1–T3 unchanged vs round-3-spec-r14.md; §6.1 write set identical (14 files) and equals the §6 union; schema_version 1.

### By Severity
- CRITICAL: 0
- HIGH: 0
- MEDIUM: 0
- LOW: 5

### Issues

**[LOW][provenance] Ticket account contradicted by the input**
File: spec.md:33,210 vs issues/01:91 | ID: CR15-01 (reopens r14 §9.7) | Confidence: HIGH
Issue: spec claims the heading is "mirror of spec r14" with differing text; the input reads r15 and already carries the tie-break, C5/D10 and resource.path pins. No AC impact.
Fix: state the observed heading/body; drop the divergence claim.

**[LOW][traceability] T1 still claims AC1-6 wholesale**
File: spec §6 T1 vs AC1-6 Verify, T3 | ID: CR15-02 | Confidence: HIGH
Issue: T1 lists AC1-1..AC1-10, but AC1-6's cmd TestEmitRecoveryRecords half is T3's and unreachable from T1's files; T3's "no dual ownership" is contradicted on paper.
Fix: scope T1 to the storage half; name T3 for the cmd half.

**[LOW][verifiability] Two ranges omit the fact they support**
File: spec T2 inputs/§3: wiring.go:486-491, log.go:224- | ID: CR15-03 | Confidence: HIGH
Issue: the off-arm block is 486-494 and the quoted remediation is at 492; eventMessage starts at 228 (default 275 is inside the cited 270-275).
Fix: 486-494; 227-276.

**[LOW][coverage] snapshot_unavailable ErrorInfo shape unpinned**
File: spec §4/T2 inputs vs AC2-2, T2 RED | ID: CR15-04 | Confidence: HIGH
Issue: §4 requires kind=resource/retryable=true/remediation "verify the durable snapshot and storage permissions, then retry"; AC2-2 checks only error.code and T2 pins only state_directory. validErrorInfo admits exact equality, so a wrong remediation ships green.
Fix: add a four-field snapshot_unavailable shape case in edge_test.go, cited by AC2-2.

**[LOW][coverage] Retention loop tested only with one excess**
File: AC1-3/1-5/1-6/1-8/1-9 (5 seeds), R5/§4(3) | ID: CR15-05 | Confidence: HIGH
Issue: no oracle requires two evictions, so evicting at most one per pass passes all ACs; a backlog could stay above QuarantineKeep (a second unlink is still caught).
Fix: add a 6-seed (2-excess) subcase asserting two evictions and total 5.

### Positive Observations
- Family-scoped parse and cross-family seeds are consistent across R5/§4(3)/AC1-3/1-6/1-8/§8(2).
- R7 carve-out and fatal-first are negative behavioral oracles, not prose; §9.11's owner flag for irreversible deletion is explicit.
- AC1-6 pins resource.path, validErrorInfo equality, the canonical remediation and the non-default message; producer literals match source.

**Limits:** no shell/hash or write tool, so registry section hashes and byte identity of .scratch/spec.md vs round-4-spec-r15.md (same 215 lines/header) are unverified; no go test/spec-lint; no implementation exists; ticket edit timing unavailable (agentview recorded no edit).

### Recommendation
APPROVE
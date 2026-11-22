Format-only continuation of the SAME round-1 r12 draft (SNAPREC-SPEC-r12, DRAFT): status line, R1–R9, all 19 AC ids and text, T1–T3, and decision ids D1–D8 are byte-unchanged — insertions only, no other byte altered.

Inserted marker line numbers in the returned `specification` text (five insertion sites, eight marker lines):
(1) requirements: start L11 (immediately after `## 2. REQUIREMENTS`), end L27 (immediately before `## 3. EVIDENCE AND UNCERTAINTY`);
(2) design: start L60 (after `## 4. DESIGN`), end L93 (immediately before `## 5. ACCEPTANCE CRITERIA`);
(3) tasks: start L122 (immediately before the `- **T1 — storage recovery engine.**` bullet), end L153 (immediately after T3's last sub-bullet);
(4) new `### 6.1 ENVELOPE` heading L155;
(5) envelope: start L156 (after the 6.1 heading), end L166 — content L158–L164 is a mechanical restatement derived strictly from §6: envelope id `env-1`; root `/home/opencode/projects/default/mcp-bridge`; allowedTaskIds `CR1, CT1, T1, T2, T3`; write set = the exact 14-path union of the per-task Expected files (no path added or removed vs §6/owner envelope); allowed effects `read, write, test`; forbidden effects `network, publish, deploy, schedule, credentials, infrastructure`; plus verbatim the one blocker line.

Ordering verified: all markers unique, non-nested, ordered requirements < design < tasks < envelope; envelope region starts (L156) after tasks region ends (L153); regions cover §2/§4/§6-task-block/§6.1 only.

No verification commands run (read-only edit of my own draft); spec-lint and tests remain the coordinator's. No blockers.
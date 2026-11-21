# 03 — `search_tools` filters and result kind

**Type:** task

**What to build:** A single discovery round trip can express what the turn actually needs: read-only inspection only, a server set to exclude, and which kinds of catalog entries to consider — instead of the model filtering results itself.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

- [ ] `search_tools` accepts `read_only_only` (bool), `exclude_servers` (bounded list), and `kind` (one or more of `tool`, `prompt`, `resource`) using the existing pointer-style wire decoding and bounds checks.
- [ ] `read_only_only` includes only entries whose effective annotation `readOnlyHint` is true; entries with missing or unknown metadata are excluded.
- [ ] `exclude_servers` composes with the existing `servers` allowlist; a contradictory request (same server in both) is rejected deterministically with a clear error.
- [ ] Every result entry carries `kind`; all existing fields keep their current shape and meaning, so current clients are unaffected.
- [ ] Filtering is applied before top-k truncation, so a filtered request still returns a full page when eligible entries exist.
- [ ] Tests cover destructive-tool exclusion, excluded-server absence, kind filtering, and contradictory input.
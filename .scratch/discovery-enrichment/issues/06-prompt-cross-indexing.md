# 06 — Cross-index upstream prompts into the same hybrid space

**Type:** task

**What to build:** A query for a workflow ("zfs scrub", "lease template") surfaces the matching upstream prompt alongside tools, so the client can fetch the prepared procedure with `prompts/get` instead of reconstructing it from tool descriptions.

**Blocked by:** 03.

**Status:** ready-for-agent

- [ ] Prompt definitions are indexed with name, description, and argument names/descriptions, and are returned by `search_tools` with `kind: "prompt"` plus owning server and prompt name.
- [ ] Prompt documents and vectors follow the same versioned content identity as tools: a changed prompt re-embeds, a removed prompt loses its vector, and unchanged prompts keep theirs across restarts.
- [ ] Prompt catalog refresh updates the search snapshot exactly like tool refresh; a refresh failure leaves the last-known prompt entries rather than dropping them.
- [ ] Only prompts a client can actually `prompts/get` are returned (disabled or unready servers excluded).
- [ ] Tie-breaking between a prompt and a tool of equal score is deterministic and documented.
- [ ] Tests cover prompt-only matching, vector invalidation on prompt change, and unready-server exclusion.
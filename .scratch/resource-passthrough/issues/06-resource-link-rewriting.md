# 06 — `resource_link` and embedded resource rewriting in tool results

**Type:** task

**What to build:** Upstream tools that return references instead of large inline payloads stay followable: every `resource_link` URI and embedded-resource URI in a `CallToolResult` is rewritten into the bridge namespace, so the model can fetch it with `resources/read`.

**Blocked by:** 01.

**Status:** ready-for-agent

- [ ] Content entries of type `resource_link` have their `uri` rewritten to the bridge form; `name`, `title`, `description`, `mimeType`, `size`, `icons`, and annotations are unchanged.
- [ ] Content entries of type `resource` (embedded content) have their `uri` rewritten while the embedded text/blob payload is untouched.
- [ ] Rewriting covers both `isError == false` and `isError == true` results, and both inline and streamed responses.
- [ ] A URI that no configured server can serve is still returned as a link (rewritten only if it belongs to a known server); it must not turn into an error.
- [ ] Content block types other than these two pass through byte-identical.
- [ ] Tests cover a tool returning a resource link, an embedded resource, an unknown-server URI, and an unrelated content block.
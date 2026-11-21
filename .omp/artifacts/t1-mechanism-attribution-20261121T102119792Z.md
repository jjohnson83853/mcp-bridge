# T1 mechanism attribution (AC01-5)

Observed facts from the recorded wire captures:

- default-stateless: protocol "2026-07-28"; tools/call body contains owner/repo; Mcp-Param-owner and Mcp-Param-repo absent; result isError=true; body-only control succeeded in the same window.
- stateful control: same modern peer, protocol "2026-07-28"; Mcp-Param-owner/repo present with the argument values; body still contains both values; result success; exactly one server/discover and zero initialize.

Attribution (derived, not observed directly): insufficient protocol version is excluded because the same negotiated version carries parameter headers on the stateful control. The default-stateless call's missing mechanism is the bridge-side parameter-header emission/definition-cache path, not the protocol version. The internal cache state was not instrumented; this attribution rests on the paired wire facts above.

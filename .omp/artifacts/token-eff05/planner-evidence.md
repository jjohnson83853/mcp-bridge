# Ticket 119 evidence bundle

Prepared by the coordinator. Facts below are the supplied evidence; planner may read repo paths but has no live MCP/network access.

## Authoritative intake

- GitHub issue jjohnson83853/mcp-bridge#119, body sha256 `caf4384624c8ce814fe3df8a9019c2a410c976e6aaeddf276c5fffefc0e0fd81`.
- Snapshot: `.omp/artifacts/token-eff05/issue-119.json` (body + one comment + labels), `.omp/artifacts/token-eff05/issue-119.md`.
- Refinement (READY): `.omp/artifacts/token-eff05/refinement.md`.
- Parent feature issue: #157, ticket 01 already delivered.

## Stored schemas (captured via describe_tool, not reconstructed from names)

1. `portainer__EndpointList` — `.omp/artifacts/token-eff05/schema-portainer_endpointlist.json` (12529 bytes, valid JSON; server `portainer`, upstream_name `EndpointList`, `readOnlyHint: true`, no `output_schema`). Input schema is a filter set (`name`, `limit`, `endpointIds`, `excludeSnapshots`, ...), `additionalProperties:false`.
2. `portainer__docker_proxy` — `.omp/artifacts/token-eff05/schema-portainer_docker_proxy.json` (2476 bytes; server `portainer`, upstream_name `docker_proxy`, required `environment_id` + `path`, optional `method`, `query_params`, `headers`, `body`, and optional `select` JMESPath expression). Its stored description says: "Proxy a Docker Engine API request through Portainer... The `path` is forwarded as-is to the Docker daemon (e.g. /containers/json, /images/json, /containers/{id}/json). Pass a JMESPath `select` to project just the fields you need — raw Docker payloads are large. Example for /containers/json: `[].{id:Id,name:Names[0],state:State,image:Image}`." `output_schema` is `{result: string}` (x-fastmcp-wrap-result).
3. Not captured as decisive evidence: Proxmox `proxmox__get_containers` (LXC) and OPNsense `opn_list_*`. The ticket makes their inclusion conditional on inspected schemas plus recorded default fields; excluding them is in scope, including them would be optional scope.

## Observed payload evidence (from issue comment 2026-11-27T16:15:49Z)

- Portainer EndpointList full objects include heavy members: `AzureCredentials`, `TLSConfig`, `Snapshots`, `PublicURL`.
- Docker `/containers/json` full objects include heavy members: `HostConfig`, `ImageManifestDescriptor`, `Mounts`, `NetworkSettings`.
- Sizes: `/containers/json` serialized tool result 110237 bytes = 18 pages at the 8192-byte ceiling; EndpointList 25128 bytes.
- A caller-supplied `select: "[].{id:Id,name:Names[0],state:State}"` on the docker proxy reduced 50260 → 9001 characters. This is evidence the projection reduces page count, not an acceptance target.

## Existing implementation anchors

- `internal/service/tools.go`: `callTool` (~line 879) admission, `Dispatch`, result delivery; `searchTools` (~line 502) and `describeTool` (~line 583) also funnel results through `t.boundedCallToolResult`.
- `internal/service/bounded_delivery.go`: `deliverBoundedResult` (line 18) measures the serialized `mcp.CallToolResult` and either passes through or pages; `boundedCallToolResult` (line 250) is the shared seam; `renderResultPreview`/`renderResultPage`/`renderResultTerminal`.
- `internal/catalog/schema.go`: built-in tool registration (`search_tools`, `describe_tool`, `call_tool`, `retrieve_result`) if a bridge-level argument or new built-in surface is designed.
- `internal/config`: bounds validation and typed diagnostics; retained-result bounds already exist.
- Upstream tool identity is available at dispatch time as `binding.ServerID` + raw tool name; arguments are available as the validated `wire.Arguments` / dispatched `args`.

## Constraints supplied by coordinator

- One coherent plan/review cycle; no unnecessary widening (owner preference recorded for this feature).
- Preserve AC1–AC7 semantics exactly; no new numeric performance AC.
- Allowlist entries are explicit identities; no name-pattern matching.
- Projection must not silently drop fields on errors, empty lists, or upstream shape changes (AC6).
- Full result remains retrievable under ticket 01's ceiling; a full opt-in must not create an unbounded escape hatch.
- The `docker_proxy` operation discriminator must not project unrelated Docker API paths.
- Tests must not depend on live Portainer/Docker. Use captured-shape fixtures.
- Observability section is mandatory per role; load `skill://observability-contract` if available to the planner, otherwise state the limit and follow existing repo conventions in `docs/logging-contract.md`.

## Commands/conventions

- Go repo; tests are `go test ./...` focused per package. Existing service tests live next to sources (`internal/service/*_test.go`).
- The spec file must be self-contained but may reference source paths and this evidence bundle.

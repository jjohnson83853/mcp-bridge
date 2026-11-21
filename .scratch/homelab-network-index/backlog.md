# Homelab network topology index — backlog

Status: needs-triage
Type: task

Goal (owner): a token-cheap, auto-refreshing index of the homelab network — topology, container
networks, upstream endpoints, telemetry IDs, and known quirks — written to Second Brain and to a
local skill reference, refreshed by a scheduled job that never invokes an LLM, so future
troubleshooting sessions skip repeated MCP discovery.

This file is **pre-spec evidence and routing**, not acceptance criteria. Requirements, ACs and
design are authored by the planner (`.omp/RULES.md` M07/S03). The continuation prompt in §7 exists
only to make the next dispatch mechanical.

## 1. Why (friction observed in the 2026-11-15 session)

| # | Observed cost/failure | Consequence |
|---|---|---|
| 1 | Every diagnostic session re-runs discovery: `EndpointList`, `StackList`, `docker_proxy /networks`, `/containers/json`, `list_datasources`, `list_loki_label_values`, `opn_scan_config`, Proxmox `get_nodes`/`get_vms`/`get_containers` | dozens of large JSON payloads per incident, most repeated verbatim across sessions |
| 2 | Container↔endpoint IDs must be re-derived from logs each time (`172.28.0.9` = `mcp-bridge-network`, `172.28.0.10` = `mcp-bridge`, Caddy proxmox `b4ad13a6…` vs `6ce05f22…`) | rediscovery cost paid per incident, and stale IDs caused a wrong-container diagnosis |
| 3 | Tool inventory is session-dependent: when the Grafana/Portainer MCP servers were unmounted mid-session, indexed tools vanished and calls failed with "unknown tool" | an index must record which session/mount it was built from and degrade explicitly |
| 4 | Bridge catalog degradation is invisible without querying: `mode: keyword`, `degraded_reasons: ["vectors_absent"]`, `catalog_refresh_failed` every 300 s (`upstream_unavailable`) | tools silently missing (`mcp-rag`) were mistaken for connector/auth faults |
| 5 | Upstream breaker wedge (`.scratch/upstream-breaker-recovery/`) requires a process restart | health facts belong in the index so triage does not re-derive them |

## 2. Proposed index sources (tool names as the owner enumerated)

All calls use the bridge envelope `call_tool {tool_name, arguments}` for indexed upstream tools;
directly declared tools are invoked directly. Verify exact schema with `describe_tool` before the
first call of each — do not assume payload shape.

- **OPNsense** (`opnsense`): `opn_scan_config`; `opn_get_config_section` for `interfaces`, `vlans`,
  `filter`, `nat`, `staticroutes`, `unbound`, `dnsmasq`; `opn_gateway_status`, `opn_ipv6_status`,
  `opn_list_static_routes`, `opn_wireguard_status`, `opn_openvpn_status`,
  `opn_mdns_repeater_status`, `opn_list_dnsmasq_ranges`, `opn_list_dnsmasq_leases`,
  `opn_arp_table`, `opn_list_dns_overrides`, `opn_list_dns_forwards`, `opn_list_dnsbl`,
  `opn_list_firewall_rules`, `opn_list_firewall_aliases`, `opn_list_ddns_accounts`,
  `opn_haproxy_search` (frontends/backends/servers/ACLs/healthchecks).
- **Proxmox** (`proxmox`): `get_nodes`, `get_storage`, `list_bridges` (on `nas`), `get_vms`,
  `get_vm_config`, `get_vm_ip_addresses`, `get_containers`, `get_container_config`,
  `get_container_ip`.
- **Portainer/Docker** (`portainer`): `EndpointList(select="[].{id:Id,name:Name,type:Type,status:Status}")`;
  per endpoint `StackList(select="[].{id:Id,name:Name,endpointId:EndpointId,status:Status}")`;
  `docker_proxy /networks` and `/containers/json` with the projections already used in this session.
  Known: `3` = `Services`, `5` = `docker-lxc`.
- **Grafana** (`grafana`): `list_datasources` (Loki `P8E80F9AEF21F6940`, Prometheus `dfjbim150pvk0d`),
  `list_loki_label_values` for active `source_system`/`source_subsystem`, `search_dashboards`.
- **Smart home / IoT** (`homeassistant`): `ha_get_overview`. (`hubitat`, `mqtt-mcp`,
  `reolink-mcp-bridge`: hub IP + radio summary, broker endpoint, camera IPs — verify tool
  availability first; they were not all reachable this session.)
- **Scheduler / repos / runbooks**: `gh-cron-mcp` `list_jobs`;
  `github` `search_repositories` for infrastructure repos;
  `second-brain` reads of `02_Areas/homelab/homelab-infrastructure.md` and
  `02_Areas/homelab/vector-loki-logging.md`.

## 3. Output targets

1. `/intermediate/thoughts/02_Areas/homelab/network-topology.md` — canonical note, YAML
   frontmatter, `[[wikilinks]]` to related runbooks, deterministic sort order, tables only.
2. `~/.omp/agent/skills/homelab-network/references/topology.md` — hot reference for `omp` sessions.
3. Both must carry a preserved `## Manual Notes & Known Quirks` section that the sync job never
   overwrites.

## 4. Automation shape

1. Deterministic Python sync script (suggested `/srv/mcp-grafana/scripts/sync_network_topology.py`)
   querying OPNsense, Proxmox and Portainer APIs directly — no LLM, no MCP bridge round-trip —
   writing both files atomically, preserving manual sections.
2. Nightly schedule registered/documented via `gh-cron-mcp` (or a systemd timer on `docker-lxc`).
3. `~/.omp/agent/skills/homelab-network/SKILL.md` must instruct: read `references/topology.md`
   first, reuse cached IDs/UIDs, and skip discovery calls (`EndpointList`, `list_datasources`,
   `opn_scan_config`) unless a target is absent from the index.

## 5. Gotchas the implementer must encode (all observed)

1. `MCP_RESOURCE_URL` is both the advertised `resource` and the validated `aud`; it must be
   byte-identical to the connector URL. `internal/auth/middleware.go:76-110` accepts an `aud`
   equal to the configured resource, or equal after dropping a literal `/mcp` on either side;
   `https://tool.bullprog.com/network/mcp` does **not** match `…/mcp`.
2. pocket-id compares the authorize `resource` **exactly**: no prefix match, no trailing-slash
   normalization. `https://tool.bullprog.com/network` does not satisfy `…/network/mcp`.
3. `/authorize` success is **302 → `/interaction`** (sign-in page). Failure is **303** to the
   connector redirect with `error=invalid_request` (resource/client not granted) or
   `error=invalid_scope` (`'mcp:connect' is not available for the requested resource`).
4. A missing **user-delegated API access grant** on the OIDC client produces the same
   `invalid_request` as an unknown resource — the error text does not distinguish them.
5. Federated client credentials (`Credentials` tab) authenticate the *client to pocket-id* with a
   JWT; they are unrelated to access-token `aud` and to MCP resource identity. Do not add them to
   "fix" a 401.
6. Gemini's `resource` comes from the **connector URL**, not from the bridge's
   `/.well-known/oauth-protected-resource`; changing `MCP_RESOURCE_URL` alone does not change what
   the AS sees.
7. Redirect URIs are per connector instance (`…/r/user_bound_custom-mcp-<hash>`): deleting and
   re-adding the connector in Gemini mints a new hash. Callback list vs logout-callback list are
   different fields; pocket-id matches exactly.
8. Deployment is not readable read-only over Portainer (`/proc/1/environ` blocked); never infer the
   deployed env from the repo config. `docker exec` of the container process is restricted by SSH
   policy. Record this so the next agent does not retry blocked probes.
9. Bridge catalog can be stale without being down: `catalog refresh failed … upstream_unavailable`
   every 300 s with retained `catalog_tools=973`, plus `mode: keyword` /
   `degraded_reasons: ["vectors_absent"]` when the embedding provider is unwired
   (`configs/config.mcp-grafana-stack.yaml:51-60` placeholder `fictitious-embedding-model-v1`,
   `internal/search/provider.go` emits `embedded vectors != model dims` and discards them).
10. Two Caddy instances (`caddy-14` proxmox LXC and `docker-windows` service) both serve
    `id.bullprog.com`; inconsistent AS metadata (`502` vs `200`) was traced to the stale LXC
    instance, not to DNS. Topology must record *which hostname is served by which instance*.

## 6. Discrepancies found between documented notes and live state

| # | Documented | Live |
|---|---|---|
| 1 | Asus RT-BE92U AP at `192.168.50.57` | observed `192.168.50.202` (owner's note) — confirm from DHCP leases during the build |
| 2 | implicit assumption that one OIDC client audience serves both bridges | pocket-id had only `https://tool.bullprog.com/mcp` granted to the client; `/network/mcp` API existed but the network grant was `user-delegated: off` |
| 3 | connector config considered static | redirect URI hash changes on every Gemini connector re-creation |

## 7. Continuation prompt (dispatch this, do not re-derive)

> Build the homelab network index described in `.scratch/homelab-network-index/backlog.md`.
>
> 0. Read `docs/agents/domain.md`, then the two Second Brain notes named in §2.
> 1. Inventory: for each source in §2 call `describe_tool` once per tool, then `call_tool` with the
>    projections specified. Record, per source: reachability, `catalog_revision`, and any
>    `degraded_reasons`. If a tool is absent from the session, record it as unavailable — do not
>    substitute a different server.
> 2. Write `network-topology.md` and `references/topology.md` with sorted tables: VLANs/subnets,
>    firewall aliases + rule summary, NAT/port-forwards, DNS (unbound + dnsmasq ranges/leases/
>    overrides/forwards), HAProxy frontends/backends, Proxmox nodes/VMs/CTs with bridges + VLAN tags
>    + IPs, Docker endpoints/stacks/networks/container IPs + published ports, Grafana datasource
>    UIDs + Loki label values + dashboard UIDs, and a `## Manual Notes & Known Quirks` section
>    seeded from §5.
> 3. Record the hostname→Caddy-instance→upstream chain, and which OIDC client/resource each bridge
>    deployment uses.
> 4. Write the sync script (§4.1) with credentials from env refs only; atomic write; preserve the
>    manual section verbatim.
> 5. Register the nightly job and prove one execution end-to-end (exit code, diff of the two files,
>    preserved manual section).
> 6. Update `~/.omp/agent/skills/homelab-network/SKILL.md` per §4.3.
> 7. Report: paths written, discrepancies resolved (with the observed values), script path, schedule,
>    and one evidence line per source showing it was queried live.
>
> Out of scope: changing pocket-id, Caddy, or bridge configuration; anything requiring an LLM at
> refresh time; storing secrets in either Markdown file.

## 8. Open questions for the planner

1. Repo home for the sync script (`mcp-grafana` stack repo vs a dedicated homelab-ops repo) and its
   test/ownership model.
2. Refresh cadence and staleness signalling: does the note carry a `generated_at` + source-status
   block that downstream agents must check before trusting IDs?
3. Whether the index should also record per-deployment auth identity (client id, resource, scopes)
   as a table, given the 2026-11-15 misdiagnoses.
4. How the local skill reference stays in sync on machines other than the one running the job
   (checkout vs symlink vs copy in the job).
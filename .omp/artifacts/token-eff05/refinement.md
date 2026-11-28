# Issue refinement — jjohnson83853/mcp-bridge#119

Status: READY (refinement only; not approval)
Source: GitHub jjohnson83853/mcp-bridge#119; parent #157.
Body sha256: caf4384624c8ce814fe3df8a9019c2a410c976e6aaeddf276c5fffefc0e0fd81 (comment 2026-11-27T16:15:49Z).
Evidence anchors: `.omp/artifacts/token-eff05/issue-119.json`, `issue-119.md`, `schema-portainer_endpointlist.json`, `schema-portainer_docker_proxy.json`.

## Assessment

[INFERENCE] One coherent plan/review cycle is reasonable: two observed operations, shared projection precedence, bounded delivery and seven testable ACs. Coordinator evidence establishes ticket 01 delivered (8192-byte ceiling); relevant seams are `internal/service/{tools.go,bounded_delivery.go}`. Comment measures 110237 bytes/18 pages, not equivalent context consumption; 50260→9001 characters is a separate projection example, not a promised target. Risks: `docker_proxy` also serves unrelated operations; existing `select` uses JMESPath; schemas do not establish list-item shapes. Its `output_schema` wraps a result string, despite intake saying no output schema. These require planner treatment, not broader scope or live re-verification. No irreversible infrastructure action is requested.

## Smallest rewritten request

R1. Provide compact defaults for the observed Portainer EndpointList and Docker container-list operation through `portainer__docker_proxy` at `/containers/json`: small identity/state fields for containers and identity fields for endpoints. Preserve explicit full/custom requests, full-result retrieval and ticket 01's ceiling.
R2. Use explicit server-identity/tool-name allowlisting justified by captured stored schemas, never descriptive substrings/provider heuristics; do not project unrelated proxy operations. Include Proxmox/OPNsense lists only after inspecting their stored schemas and recording default fields in the design. Make projection/defaults visible, not apparent upstream shape. Planner chooses fields and interface.
R3. Explain the benefit as fewer retrieval pages; introduce no numeric performance AC.
W1–W7 retain the original AC1–AC7 verbatim:
- AC1/W1: The allowlisted list tools return the documented compact projection by default, and the response states that a projection was applied.
- AC2/W2: An explicit full request returns the upstream's unprojected objects, still subject to the response ceiling.
- AC3/W3: An explicit field selection overrides the default projection and rejects unknown fields deterministically.
- AC4/W4: Non-allowlisted tools, including similarly named ones on other servers, are unaffected.
- AC5/W5: Each allowlist entry is justified by the tool's stored schema captured in the design, not by a name pattern.
- AC6/W6: Errors, empty lists, and upstream shape changes degrade safely rather than dropping fields silently.
- AC7/W7: Tests cover: default projection, full opt-in, custom selection, unknown-field rejection, unaffected neighbors, and a shape-change fixture.

## Bidirectional equivalence

| Original | Rewritten | Why equivalent |
| --- | --- | --- |
| Build paragraph | R1 | Same outputs, retrieval and ceiling. |
| Notes | R2 | Same conditional scope, exclusions and disclosure. |
| Comment | R3 | Motivation only. |
| AC1 | W1 | Literal identity. |
| AC2 | W2 | Literal identity. |
| AC3 | W3 | Literal identity. |
| AC4 | W4 | Literal identity. |
| AC5 | W5 | Literal identity. |
| AC6 | W6 | Literal identity. |
| AC7 | W7 | Literal identity. |

No weaker/stronger defaults, compatibility, errors, boundaries or tests.

## Unresolved (planner-owned)

- Field lists.
- Request interface.
- Safe shape-change behavior.
- How a multi-operation proxy tool is keyed without projecting unrelated paths.

No decisive dependency is missing.

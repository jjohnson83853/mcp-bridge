#!/usr/bin/env python3
"""Build and install a ticket-119 revision state from /tmp/spec-<rev>.txt.
Usage: build_and_install.py <rev> <prev_sha> <review_source_path> <review_label>
"""
import re, hashlib, json, os, sys

root = '/home/opencode/projects/default/mcp-bridge'
rev, prev, review_src, review_label = sys.argv[1:5]
src = open(f'/tmp/spec-{rev}.txt').read()
titles = ['1. STATUS', '2. REQUIREMENTS', '3. EVIDENCE AND UNCERTAINTY', '4. DESIGN',
          '5. ACCEPTANCE CRITERIA', '6. TASK LEDGER', '7. OBSERVABILITY',
          '8. RELEASE AND RECOVERY', '9. CRITIC HANDOFF', '10. SUPPLIED-FINDING RESOLUTION']
out = ['## ' + l if l in titles else l for l in src.splitlines()]
idx = {l[3:]: i for i, l in enumerate(out) if l.startswith('## ') and l[3:] in titles}
bounds = {'requirements': ('2. REQUIREMENTS', '3. EVIDENCE AND UNCERTAINTY'),
          'design': ('4. DESIGN', '5. ACCEPTANCE CRITERIA'),
          'tasks': ('6. TASK LEDGER', '7. OBSERVABILITY'),
          'envelope': ('7. OBSERVABILITY', '8. RELEASE AND RECOVERY')}
marker = {}
for name, (a, b) in bounds.items():
    marker[idx[a]] = marker.get(idx[a], '') + f'<!-- specdd:{name}:start -->\n'
    marker[idx[b]] = marker.get(idx[b], '') + f'<!-- specdd:{name}:end -->\n'
res = ''
for i, l in enumerate(out):
    if i in marker:
        res += marker[i]
    res += l + '\n'

spec = f'.scratch/token-efficient-tool-responses/ticket05-spec-{rev}.md'
open(spec, 'w').write(res)

def secsha(s, name):
    st, en = f'<!-- specdd:{name}:start -->', f'<!-- specdd:{name}:end -->'
    a, b = s.index(st), s.index(en)
    return hashlib.sha256(s[a + len(st):b].encode()).hexdigest()

sectionBindings = {n: {'revision': rev, 'sha256': secsha(res, n)} for n in bounds}
spec_sha = hashlib.sha256(res.encode()).hexdigest()
feature = f'token-efficient-tool-responses-ticket05-{rev}'
tasks = [
 {'id': 'CR1', 'stage': 'research', 'status': 'pending', 'blockedBy': [], 'requires': [], 'effects': {'kind': 'read', 'paths': [spec]}, 'acceptanceIds': [], 'evidenceIds': [], 'agentRoles': ['code-reviewer']},
 {'id': 'CT1', 'stage': 'research', 'status': 'pending', 'blockedBy': [], 'requires': [], 'effects': {'kind': 'read', 'paths': [spec]}, 'acceptanceIds': [], 'evidenceIds': [], 'agentRoles': ['critic']},
 {'id': 'T1', 'stage': 'implementation', 'status': 'pending', 'blockedBy': [], 'requires': [], 'effects': {'kind': 'write', 'paths': ['internal/service/field_projection.go', 'internal/service/field_projection_test.go']}, 'acceptanceIds': ['AC1', 'AC3', 'AC5', 'AC6'], 'evidenceIds': [], 'agentRoles': ['engineer']},
 {'id': 'T2', 'stage': 'implementation', 'status': 'pending', 'blockedBy': ['T1', 'T3'], 'requires': [], 'effects': {'kind': 'write', 'paths': ['internal/service/tools.go', 'internal/service/direct_execution.go', 'internal/catalog/schema.go', 'internal/service/bounded_delivery.go', 'internal/service/result_handle.go', 'internal/service/field_projection_surface_test.go', 'README.md']}, 'acceptanceIds': ['AC1', 'AC2', 'AC3', 'AC4', 'AC6', 'AC7'], 'evidenceIds': [], 'agentRoles': ['engineer']},
 {'id': 'T3', 'stage': 'implementation', 'status': 'pending', 'blockedBy': ['T1'], 'requires': [], 'effects': {'kind': 'write', 'paths': ['internal/observe/log.go', 'internal/observe/metrics.go', 'internal/observe/envelope.go', 'internal/observe/log_test.go', 'internal/observe/metrics_test.go', 'docs/logging-contract.md']}, 'acceptanceIds': ['AC6', 'AC7'], 'evidenceIds': [], 'agentRoles': ['engineer']},
 {'id': 'T4', 'stage': 'verification', 'status': 'pending', 'blockedBy': ['T2', 'T3'], 'requires': [], 'effects': {'kind': 'read', 'paths': [spec, 'internal/service', 'internal/catalog', 'internal/observe', 'internal/config', 'README.md']}, 'acceptanceIds': ['AC1', 'AC2', 'AC3', 'AC4', 'AC5', 'AC6', 'AC7'], 'evidenceIds': [], 'agentRoles': ['engineer']},
]
branch = {'id': feature + f'-{rev}', 'answerIds': [], 'taskIds': [t['id'] for t in tasks], 'envelopeId': 'env-1', 'sectionBindings': sectionBindings}

def canon(v):
    if isinstance(v, list):
        return '[' + ','.join(canon(x) for x in v) + ']'
    if isinstance(v, dict):
        return '{' + ','.join(json.dumps(k) + ':' + canon(v[k]) for k in sorted(v)) + '}'
    return json.dumps(v, separators=(',', ':'))

packet_digest = hashlib.sha256(canon({'feature': feature, 'projectRoot': root, 'sectionBindings': sectionBindings, 'branches': [branch]}).encode()).hexdigest()
reqs = [
 {'id': 'R1', 'summary': 'Exactly two allowlisted operations: portainer EndpointList and portainer docker_proxy path /containers/json; Proxmox/OPNsense excluded.'},
 {'id': 'R2', 'summary': 'Default projection on both paths with ONE disclosure schema {v,applied,entry,mode,fields,reason?}; fields=[] when nothing applied, entry="" when no match; whole upstream_result re-derived.'},
 {'id': 'R3', 'summary': 'Explicit full via call_tool returns unprojected objects; direct-path opt-out via call_tool projection.full; ceiling preserved.'},
 {'id': 'R4', 'summary': 'Field selection overrides defaults, raw keys, unknown fields reject on non-empty lists, empty list+fields accepted, fields:[] rejected; bounds + runtime-derived budget; full XOR fields.'},
 {'id': 'R5', 'summary': 'Allowlist entries explicit identities justified by captured schemas/payloads; no patterns; non-matching docker_proxy paths labeled not_allowlisted.'},
 {'id': 'R6', 'summary': 'Errors, empty lists, carrier failures, non-empty aggregate all-defaults-missing, non-object items, direct collisions degrade with per-row disclosure channels; forged members never bridge-authoritative.'},
 {'id': 'R7', 'summary': 'Non-allowlisted tools and calls with no projection request byte-identical including page splits on both paths; disclosure metadata only when budget admits; engine decision out-of-band.'},
]
acs = [('AC1', ['R2', 'R5'], ['T1', 'T2']), ('AC2', ['R3'], ['T2']), ('AC3', ['R4'], ['T1', 'T2']),
       ('AC4', ['R7', 'R1'], ['T2']), ('AC5', ['R5', 'R1'], ['T1']), ('AC6', ['R6'], ['T1', 'T2', 'T3']),
       ('AC7', ['R1', 'R2', 'R3', 'R4', 'R5', 'R6'], ['T1', 'T2', 'T3', 'T4'])]
accept = [{'id': i, 'requirementIds': r, 'ownerTaskIds': o, 'method': 'test', 'requiredEvidenceKinds': ['test']} for i, r, o in acs]
write_paths = sorted({p for t in tasks if t['effects']['kind'] == 'write' for p in t['effects']['paths']})

def art(p):
    b = open(os.path.join(root, p), 'rb').read()
    return {'path': os.path.join(root, p), 'sha256': hashlib.sha256(b).hexdigest()}

sources = [
  {'id': 'src-issue-119', 'kind': 'repository', 'artifact': art('.omp/artifacts/token-eff05/issue-119.md'), 'locator': 'GitHub jjohnson83853/mcp-bridge#119 snapshot; body sha256 caf4384624c8ce814fe3df8a9019c2a410c976e6aaeddf276c5fffefc0e0fd81', 'recordedAt': '2026-11-27T22:05:00.000Z'},
  {'id': 'src-refinement-119', 'kind': 'observation', 'artifact': art('.omp/artifacts/token-eff05/refinement.md'), 'locator': 'issue-refiner READY assessment for #119', 'recordedAt': '2026-11-27T22:05:00.000Z'},
  {'id': 'src-schema-endpointlist', 'kind': 'observation', 'artifact': art('.omp/artifacts/token-eff05/schema-portainer_endpointlist.json'), 'locator': 'describe_tool portainer__EndpointList stored schema', 'recordedAt': '2026-11-27T22:05:00.000Z'},
  {'id': 'src-schema-dockerproxy', 'kind': 'observation', 'artifact': art('.omp/artifacts/token-eff05/schema-portainer_docker_proxy.json'), 'locator': 'describe_tool portainer__docker_proxy stored schema', 'recordedAt': '2026-11-27T22:05:00.000Z'},
  {'id': 'src-endpoint-observed', 'kind': 'observation', 'artifact': art('.omp/artifacts/token-eff05/endpointlist-observed-20261128T093237Z.json'), 'locator': 'live read-only portainer__EndpointList call through the bridge; Id/Name/Type observed on all 3 endpoints', 'recordedAt': '2026-11-28T09:32:37.000Z'},
  {'id': 'src-critic-r16', 'kind': 'observation', 'artifact': art('.omp/artifacts/token-eff05/review-ct-r16-source-bcd2aac8-20261128T093310Z.json'), 'locator': 'Critic REVISE observations for r16 (C1, C2)', 'recordedAt': '2026-11-28T09:33:10.000Z'},
]
if os.path.exists(os.path.join(root, review_src)):
    sources.append({'id': f'src-review-{review_label}', 'kind': 'observation', 'artifact': art(review_src), 'locator': f'prior CodeReviewer observations ({review_label})', 'recordedAt': '2026-11-28T00:00:00.000Z'})

state = {'schemaVersion': 2, 'feature': feature, 'revision': rev, 'projectRoot': root,
 'spec': {'path': os.path.join(root, spec), 'sha256': spec_sha, 'sectionBindings': sectionBindings},
 'packet': {'digest': packet_digest, 'branches': [branch], 'selectedBranchId': None},
 'actors': {'plannerId': 'planner', 'implementerIds': ['engineer']},
 'requirements': reqs, 'acceptance': accept, 'tasks': tasks,
 'envelope': {'id': 'env-1', 'root': root, 'allowedTaskIds': [t['id'] for t in tasks],
   'readPaths': [spec, '.omp/artifacts/token-eff05', '.scratch/token-efficient-tool-responses', 'README.md', 'docs', '.omp/RULES.md', 'internal/service', 'internal/catalog', 'internal/config', 'internal/observe', 'cmd/mcp-bridge'],
   'writePaths': write_paths, 'allowedEffects': ['read', 'write', 'test'],
   'forbiddenEffects': ['network', 'publish', 'deploy', 'schedule', 'credentials', 'infrastructure'],
   'decisionPolicy': {'technicalDecisionIds': [], 'ownerCategories': []},
   'retryPolicy': {'maxAttempts': 1, 'resetOn': ['owner-signal', 'material-evidence']}},
 'evidence': [], 'capabilities': [], 'blockers': [], 'decisions': [], 'sources': sources,
 'gate': {'markerPath': os.path.join(root, '.omp/plans', feature, 'marker.json'), 'activationEvidenceIds': []},
 'authorization': None, 'reviews': None}
seed = {'statePath': f'.omp/plans/{feature}/execution-state.json', 'markerPath': f'.omp/plans/{feature}/marker.json', 'state': state}
json.dump(seed, open(f'.omp/artifacts/token-eff05/bootstrap-seed-{rev}.json', 'w'), indent=2)
print('spec', spec_sha)
print('packet', packet_digest)
print('writes', len(write_paths))

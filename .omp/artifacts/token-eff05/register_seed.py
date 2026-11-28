#!/usr/bin/env python3
"""Build candidate request + CR/CT payloads for a ticket-119 revision.
Usage: register_seed.py <rev> <prev_review_source> <prev_review_label> <review_name> <review_focus>
"""
import json, hashlib, os, sys

root = '/home/opencode/projects/default/mcp-bridge'
rev, prev_src, prev_label, review_name, focus = sys.argv[1:6]
spec = f'.scratch/token-efficient-tool-responses/ticket05-spec-{rev}.md'
cand = f'token-efficient-tool-responses-ticket05-spec-{rev}'
spec_sha = hashlib.sha256(open(os.path.join(root, spec), 'rb').read()).hexdigest()
feature = f'token-efficient-tool-responses-ticket05-{rev}'

instructions = f'''# Shared review packet — ticket #119, revision {rev} (specification review)

Repo: {root} (Go MCP bridge). Read-only.
Candidate: `{spec}`, sha256 {spec_sha}, revision {rev}, candidate id {cand}, author planner.
Repair review: {focus}
Prior review observation: `.omp/artifacts/token-eff05/{os.path.basename(prev_src)}` ({prev_label}).

## Review scope
Apply W09E. Verify the claimed closures at their root invariants; preserve earlier closures; check adjacent cases, AC1–AC7 semantics, D-table totality, out-of-band decision threading, write-set arithmetic and ticket-01 compatibility.

Report APPROVE or REQUEST CHANGES with taxonomy-classified findings and anchors.
'''

task = f'''# Target
Review `.scratch/token-efficient-tool-responses/ticket05-spec-{rev}.md` (sha256 {spec_sha}) as the {rev} repair for ticket #119.

# Method
Read the candidate and every cited anchor, including `.omp/artifacts/token-eff05/{os.path.basename(prev_src)}`. Verify the claimed closures; preserve earlier closures; check adjacent cases. Do not implement or edit.

# Output
Return APPROVE or REQUEST CHANGES with stable finding IDs, taxonomy class, severity, exact anchor and fix. No writes or repo-mutating commands.'''

wd = {'statePath': root + '/.omp/plans/execution-state.json', 'schemaVersion': 1, 'projectRoot': root,
      'feature': feature, 'revision': rev,
      'requests': [{'name': review_name, 'taskId': 'CR1', 'agent': 'code-reviewer',
                    'effects': {'kind': 'read', 'paths': [spec]},
                    'targetCandidateId': cand, 'targetRevision': rev}]}
cr = {'i': f'Review ticket 119 specification {rev}',
      'context': json.dumps({'instructions': instructions, 'workflowDispatch': wd}, separators=(',', ':')),
      'tasks': [{'name': review_name, 'agent': 'code-reviewer', 'task': task,
                 'solutionSpace': f'independent specification review of ticket #119 {rev}'}]}
json.dump(cr, open(f'.omp/artifacts/token-eff05/payload-cr-{rev}.json', 'w'), indent=2)

wd2 = dict(wd)
wd2['requests'] = [{'name': review_name.replace('Review', 'Critic'), 'taskId': 'CT1', 'agent': 'critic',
                    'effects': {'kind': 'read', 'paths': [spec]},
                    'targetCandidateId': cand, 'targetRevision': rev}]
ct = {'i': f'Critic review ticket 119 specification {rev}',
      'context': json.dumps({'instructions': instructions, 'workflowDispatch': wd2}, separators=(',', ':')),
      'tasks': [{'name': wd2['requests'][0]['name'], 'agent': 'critic',
                 'task': f'# Target\nIndependently assess `{spec}` (sha256 {spec_sha}) for ticket #119.\n\n# Method\nRead the candidate, prior finding ledger, evidence bundle and sources. Apply your full installed assessment. Do not implement or edit.\n\n# Output\nReturn PASS or REVISE with residual findings and verdict basis. No writes or repo-mutating commands.',
                 'solutionSpace': f'independent critic assessment for ticket #119 {rev}'}]}
json.dump(ct, open(f'.omp/artifacts/token-eff05/payload-ct-{rev}.json', 'w'), indent=2)

files = [spec, '.omp/artifacts/token-eff05/issue-119.md', '.omp/artifacts/token-eff05/refinement.md',
         '.omp/artifacts/token-eff05/planner-evidence.md',
         '.omp/artifacts/token-eff05/schema-portainer_endpointlist.json',
         '.omp/artifacts/token-eff05/schema-portainer_docker_proxy.json', prev_src,
         'internal/service/tools.go', 'internal/service/bounded_delivery.go',
         'internal/service/direct_execution.go', 'internal/service/result_handle.go',
         'internal/service/result_handle_test.go', 'internal/service/bounded_delivery_test.go',
         'internal/catalog/schema.go', 'internal/config/bounds.go', 'internal/observe/metrics.go',
         'internal/observe/envelope.go', 'internal/observe/log.go', 'internal/observe/log_test.go',
         'internal/observe/metrics_test.go', 'docs/logging-contract.md', 'README.md']
inputs = [{'path': os.path.join(root, f), 'sha256': hashlib.sha256(open(os.path.join(root, f), 'rb').read()).hexdigest()} for f in files]
req = {'candidateId': cand, 'revision': rev, 'authorIds': ['planner'], 'inputs': inputs, 'reviewers': [
    {'name': review_name, 'taskId': 'CR1', 'reviewerId': 'code-reviewer', 'payloadPath': f'.omp/artifacts/token-eff05/payload-cr-{rev}.json'},
    {'name': wd2['requests'][0]['name'], 'taskId': 'CT1', 'reviewerId': 'critic', 'payloadPath': f'.omp/artifacts/token-eff05/payload-ct-{rev}.json'}]}
json.dump(req, open(f'.omp/artifacts/token-eff05/candidate-request-{rev}.json', 'w'), indent=2)
json.dump({'intent': cr['i'], 'instructions': instructions, 'workflowDispatch': wd, 'tasks': cr['tasks']},
          open(f'.omp/artifacts/token-eff05/cr-seed-{rev}.json', 'w'), indent=2)
print('prepared', rev, spec_sha)

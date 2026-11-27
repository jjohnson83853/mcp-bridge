# T2 root-cause diagnosis — observe edit failure (r40)

## 1. Root cause of `E_EDIT_REJECTED` near line 357

The engineer attempted a multi-file patch bundle that modified:
- `internal/observe/metrics.go`
- `internal/observe/envelope.go`
- `internal/observe/httpdiagnostics.go`
- `internal/observe/log.go`
- `docs/logging-contract.md`
- `internal/observe/log_test.go`

In `internal/observe/log.go`:
```
PUT <276:
    [12 case statements...]
PUT 278.=278:
+       return ""
```
Line 278 in `internal/observe/log.go` is:
```go
277:    default:
278:        return "mcp-bridge event"
279:    }
280: }
```
Because the patch applied preceding insertions (`PUT <35:`, `PUT >225:`, `PUT <276:`), the buffer line numbers were shifted. Replacing line 278 with `+ return ""` stripped the mandatory `default:` label or opening of the block, or mangled the switch statement, causing the post-patch file to fail Go syntax parsing near line 357 of the expanded buffer. The tool guard checked whether the resulting file would parse as valid Go and rejected the entire multi-file patch atomically.

## 2. Verified replacement blocks for `internal/observe`

### A. `internal/observe/metrics.go`
Add to `var (...)` near line 37:
```go
	retentionCodes = map[string]struct{}{
		"result_capacity_exceeded":           {},
		"result_aggregate_capacity_exceeded": {},
		"result_namespace_retired":           {},
		"result_entropy_unavailable":         {},
		"result_epoch_unresolved":            {},
	}
```
Add fields to `type Metrics struct` near line 125:
```go
	retentionRejections    *prometheus.CounterVec
	resultsPaged           prometheus.Counter
	resultPagesServed      prometheus.Counter
	retentionEvictions     prometheus.Counter
	logOperationViolations prometheus.Counter
```
Instantiate in `NewMetrics` and register in `m.registry.MustRegister`:
- `bridge_retention_rejections_total` (label `retention`)
- `bridge_results_paged_total`
- `bridge_result_pages_served_total`
- `bridge_retention_evictions_total`
- `bridge_log_operation_violations_total`

Add methods:
```go
func (m *Metrics) RecordRetentionRejection(retention string) bool {
	if _, ok := retentionCodes[retention]; !ok {
		return false
	}
	m.retentionRejections.WithLabelValues(retention).Inc()
	return true
}

func (m *Metrics) RecordResultsPaged()         { m.resultsPaged.Inc() }
func (m *Metrics) RecordResultPageServed()     { m.resultPagesServed.Inc() }
func (m *Metrics) RecordRetentionEviction()    { m.retentionEvictions.Inc() }
func (m *Metrics) RecordLogOperationViolation() { m.logOperationViolations.Inc() }
```

### B. `internal/observe/httpdiagnostics.go`
Replace `operationVocabulary` with the exact 31 elements from `log_test.go:424-431`:
```go
var operationVocabulary = []string{
	"unknown", "request", "discover", "initialize", "initialized", "tools_list",
	"tools_call", "call_tool", "resources_list", "resources_read", "ping",
	"sse_stream", "search", "describe", "health", "describe_tool",
	"prompts_list", "prompts_get", "refresh", "prompt_load", "prompt_persist",
	"startup", "handshake", "reload", "cli", "search_tools", "retrieve_result",
	"load_config", "check_config", "publish", "logging",
}
```

### C. `internal/observe/log.go`
In `eventMessage(event string) string`, replace the fallback so unlisted events return `""`:
```go
	case "result_paged":
		return "tool result paged"
	case "result_page_served":
		return "tool result page served"
	case "result_expired":
		return "retained result expired"
	case "result_evicted":
		return "retained result evicted"
	case "invalid_handle":
		return "invalid result handle"
	case "invalid_arguments":
		return "invalid result retrieval arguments"
	case "result_capacity_exceeded":
		return "result retention capacity exceeded"
	case "result_aggregate_capacity_exceeded":
		return "aggregate result retention capacity exceeded"
	case "result_namespace_retired":
		return "result retention namespace retired"
	case "result_entropy_unavailable":
		return "result handle entropy unavailable"
	case "result_epoch_unresolved":
		return "result retention epoch unresolved"
	case "log_vocabulary_violation":
		return "logging vocabulary violation"
	default:
		return ""
```
And enforce violation recording in `Logger.Log` and `WriteJSONLine` per `TestLogLabelEnumeration`.

## 3. Package-private store seam in `internal/service/result_handle.go`
The clean test file `result_handle_test.go` requires:
- `newResultStore(bounds config.RetainedResultBounds, logger *observe.Logger, metrics *observe.Metrics, now func() time.Time, entropy io.Reader) *resultStore`
- `resultNamespace{sessionID, epoch string; publisher bool}`
- `resultOwnerResolver func(resultNamespace) (resultNamespace, bool)`
- `resultCommit{handle string, retention string}`
- `resultLookup{code string}`
- `store.commit(owner resultNamespace, resolver resultOwnerResolver, payload []byte, upstreamCode string, isError bool) resultCommit`
- `store.lookup(handle string, owner resultNamespace, resolver resultOwnerResolver, page int) resultLookup`
- `store.retire(owner resultNamespace)`
- `store.stats() (struct with aggregateBytes, entries, tombstones int)`
- `store.entries map[string]*resultEntry`
- `store.installMarker(entry *resultEntry, code string, deadline time.Time)`
- `store.sweep()`

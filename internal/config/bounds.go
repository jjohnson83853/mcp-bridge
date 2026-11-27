package config

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"
)

// validateWeights checks the optional hybrid-search weights. Every rejection
// is one typed record (spec §3.1D): the rejected key, a finite
// weight_invalid/weights_invalid class, and a static reason — no observed
// number.
func validateWeights(root *yaml.Node, configPath string) *Diagnostics {
	keyword, keywordNode := findScalarWithNode(root, "keyword_weight")
	embedding, embeddingNode := findScalarWithNode(root, "embedding_weight")
	if keyword == "" && embedding == "" {
		return nil
	}
	invalid := func(key, shape string, node *yaml.Node) *Diagnostics {
		return single(Diagnostic{
			Path: configPath, Key: key, Line: nodeLine(node), Column: nodeColumn(node),
			Code: "weight_invalid", Property: "value", Shape: shape,
			Reason: "invalid " + strings.TrimPrefix(key, "search."),
		})
	}
	var k, e float64
	if keyword == "" {
		return invalid("search.keyword_weight", "missing", keywordNode)
	}
	if _, err := fmt.Sscan(keyword, &k); err != nil || math.IsNaN(k) || math.IsInf(k, 0) {
		return invalid("search.keyword_weight", "invalid_numeric", keywordNode)
	}
	if embedding == "" {
		return invalid("search.embedding_weight", "missing", embeddingNode)
	}
	if _, err := fmt.Sscan(embedding, &e); err != nil || math.IsNaN(e) || math.IsInf(e, 0) {
		return invalid("search.embedding_weight", "invalid_numeric", embeddingNode)
	}
	shape := ""
	switch {
	case k <= 0:
		shape = "keyword_nonpositive"
	case e < 0:
		shape = "embedding_negative"
	case math.Abs(k+e-1) > 1e-9:
		shape = "sum_not_one"
	}
	if shape == "" {
		return nil
	}
	key, relatedKey := "search.keyword_weight", "search.embedding_weight"
	ownNode, relatedNode := keywordNode, embeddingNode
	if shape == "embedding_negative" {
		key, relatedKey = relatedKey, key
		ownNode, relatedNode = relatedNode, ownNode
	}
	return single(Diagnostic{
		Path: configPath, Key: key,
		Line: nodeLine(ownNode), Column: nodeColumn(ownNode),
		RelatedKey: relatedKey, RelatedLine: nodeLine(relatedNode), RelatedColumn: nodeColumn(relatedNode),
		Code: "weights_invalid", Property: "weights", Shape: shape,
		Reason: "search weights must be nonnegative, sum to one, and keyword_weight positive",
	})
}

func validateBounds(node *yaml.Node, configPath string) *Diagnostics {
	return validateBoundsAt(node, "", "", configPath)
}

// validateBoundsAt walks the document carrying the dotted path of the mapping
// being inspected and the validated owning server id, so a rejected bound names
// the key an operator must edit (`servers[0].limits.max_concurrent_calls`, not
// a bare `timeout` that could be any of a dozen keys) and the server it belongs
// to (spec §3.1 D:548). Every rejection is one typed record (spec §3.1D): the
// indexed dotted key, a finite bound_range/duration_type/duration_range/
// duration_positive class with the coarse shape, and a static reason — never an
// observed number.
func validateBoundsAt(node *yaml.Node, path, server, configPath string) *Diagnostics {
	bounds := map[string][2]float64{
		"max_top_k": {1, 50}, "default_top_k": {1, 50}, "max_sessions": {1, 4096}, "max_concurrent_calls": {1, 128},
		"max_tools_per_server": {1, 10000}, "max_tools_total": {1, 20000}, "max_pages_per_server": {1, 1000},
		"max_schema_bytes": {1, 262144}, "max_description_bytes": {1, 32768}, "max_snapshot_bytes": {1, 268435456},
		"max_request_bytes": {1, 1048576}, "max_event_bytes": {1, 4194304}, "max_response_bytes": {1, 8388608},
		"dimensions": {1, 4096}, "batch_size": {1, 128}, "max_concurrency": {1, 32}, "query_cache_entries": {0, 4096},
		"rrf_constant": {1, 1e9}, "slots": {1, 128},
	}
	boundRange := func(keyPath, shape string, value *yaml.Node) *Diagnostics {
		return single(Diagnostic{
			Path: configPath, Key: keyPath, Server: server, Line: nodeLine(value), Column: nodeColumn(value),
			Code: "bound_range", Property: "range", Shape: shape,
			Reason: "outside permitted bounds",
		})
	}
	durationType := func(keyPath string, value *yaml.Node) *Diagnostics {
		return single(Diagnostic{
			Path: configPath, Key: keyPath, Server: server, Line: nodeLine(value), Column: nodeColumn(value),
			Code: "duration_type", Property: "type", Shape: shapeOf(value),
			Reason: "must be a duration",
		})
	}
	durationRange := func(keyPath, shape string, value *yaml.Node) *Diagnostics {
		return single(Diagnostic{
			Path: configPath, Key: keyPath, Server: server, Line: nodeLine(value), Column: nodeColumn(value),
			Code: "duration_range", Property: "range", Shape: shape,
			Reason: "outside permitted duration bounds",
		})
	}
	durationPositive := func(keyPath, shape string, value *yaml.Node) *Diagnostics {
		return single(Diagnostic{
			Path: configPath, Key: keyPath, Server: server, Line: nodeLine(value), Column: nodeColumn(value),
			Code: "duration_positive", Property: "duration", Shape: shape,
			Reason: "must be a positive duration",
		})
	}
	if node.Kind == yaml.MappingNode {
		// A server's own mapping owns the validated id above it; every bound
		// inside it is attributed to that id (spec §3.1 D:548). Only an id
		// matching the validated server-ID rule may own the records (H2 rule);
		// an unsafe scalar is omitted, never echoed.
		if isServerElement(path) {
			if id := mappingScalar(node, "id"); id != "" && serverIDPattern.MatchString(id) {
				server = id
			}
		}
		for i := 0; i < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			keyPath := key.Value
			if path != "" {
				keyPath = path + "." + key.Value
			}
			if bound, ok := bounds[key.Value]; ok {
				var number float64
				if err := value.Decode(&number); err != nil {
					return boundRange(keyPath, "invalid_number", value)
				}
				switch {
				case number < bound[0]:
					return boundRange(keyPath, "below_minimum", value)
				case number > bound[1]:
					return boundRange(keyPath, "above_maximum", value)
				}
			}
			// session_idle_timeout is a session lifetime, not a request timeout: the
			// spec's documented default is 10m (.omp/plans/headless-mcp-bridge.md §4.2),
			// so it gets an explicit window instead of the request-path suffix heuristic
			// below. 1s..1h admits that default with headroom for longer-lived idle
			// sessions while rejecting effectively permanent sessions (>1h) and
			// sub-second churning; non-positive/unparseable values still fail closed.
			// The branch is exclusive: the key ends in "_timeout" but is NOT a
			// request-path timeout, so it must never fall through to that rule.
			if key.Value == "session_idle_timeout" {
				if value.Kind != yaml.ScalarNode {
					return durationType(keyPath, value)
				}
				duration, err := time.ParseDuration(value.Value)
				switch {
				case err != nil:
					return durationRange(keyPath, "invalid_duration", value)
				case duration < sessionIdleMinDuration:
					return durationRange(keyPath, "below_minimum", value)
				case duration > sessionIdleMaxDuration:
					return durationRange(keyPath, "above_maximum", value)
				}
			} else if key.Value == "timeout" || strings.HasSuffix(key.Value, "_timeout") {
				if value.Kind != yaml.ScalarNode {
					return durationType(keyPath, value)
				}
				duration, err := time.ParseDuration(value.Value)
				limit := 30 * time.Second
				if key.Value != "timeout" {
					limit = 120 * time.Second
				}
				if strings.Contains(strings.ToLower(key.Value), "search") || strings.Contains(strings.ToLower(key.Value), "embed") {
					limit = 30 * time.Second
				}
				switch {
				case err != nil:
					return durationRange(keyPath, "invalid_duration", value)
				case duration <= 0:
					return durationRange(keyPath, "nonpositive", value)
				case duration > limit:
					return durationRange(keyPath, "above_maximum", value)
				}
			}
			if key.Value == "refresh_interval" || key.Value == "stale_after" || key.Value == "query_cache_ttl" || key.Value == "open_duration" {
				duration, err := time.ParseDuration(value.Value)
				switch {
				case err != nil:
					return durationPositive(keyPath, "invalid_duration", value)
				case duration <= 0:
					return durationPositive(keyPath, "nonpositive", value)
				}
			}
			if diags := validateBoundsAt(value, keyPath, server, configPath); diags != nil {
				return diags
			}
		}
		return nil
	}
	// A sequence element gains its zero-based index so nested bound/duration
	// records carry the complete indexed location (spec §3.1 D:548), together
	// with the validated owner inherited from the enclosing server mapping.
	for index, child := range node.Content {
		childPath := fmt.Sprintf("%s[%d]", path, index)
		if path == "" {
			childPath = fmt.Sprintf("[%d]", index)
		}
		if diags := validateBoundsAt(child, childPath, server, configPath); diags != nil {
			return diags
		}
	}
	return nil
}

const retainedEnvelopeHeadroom uint64 = 6

func defaultRetainedResultBounds() RetainedResultBounds {
	return RetainedResultBounds{
		DeliveryCeilingBytes:   8192,
		PreviewCeilingBytes:    1536,
		MaxRetainedResultBytes: 8388608,
		MaxBytesPerNamespace:   16777216,
		MaxEntriesPerNamespace: 16,
		MaxBytesAggregate:      67108864,
		HandleTTL:              time.Hour,
		TombstoneTTL:           15 * time.Minute,
		MaxUpstreamCodeBytes:   512,
		MaxMarkersPerNamespace: 16,
	}
}

func validateRetainedResults(root *yaml.Node, configPath string, bounds *RetainedResultBounds) *Diagnostics {
	*bounds = defaultRetainedResultBounds()
	node := mappingChild(root, "retained_results")
	if node == nil {
		return nil
	}

	numericBounds := []struct {
		key     string
		value   *uint64
		minimum uint64
	}{
		{"delivery_ceiling_bytes", &bounds.DeliveryCeilingBytes, 1},
		{"preview_ceiling_bytes", &bounds.PreviewCeilingBytes, 1},
		{"max_retained_result_bytes", &bounds.MaxRetainedResultBytes, 0},
		{"max_bytes_per_namespace", &bounds.MaxBytesPerNamespace, 0},
		{"max_entries_per_namespace", &bounds.MaxEntriesPerNamespace, 1},
		{"max_bytes_aggregate", &bounds.MaxBytesAggregate, 0},
		{"max_upstream_code_bytes", &bounds.MaxUpstreamCodeBytes, 1},
		{"max_markers_per_namespace", &bounds.MaxMarkersPerNamespace, 1},
	}
	for _, setting := range numericBounds {
		value := mappingChild(node, setting.key)
		if value == nil {
			continue
		}
		number, err := retainedResultUint(value)
		if err != nil {
			shape := "invalid_number"
			if strings.HasPrefix(value.Value, "-") {
				shape = "below_minimum"
			}
			return retainedResultDiagnostic(configPath, setting.key, value, "bound_range", "range", shape, "outside permitted bounds")
		}
		if number < setting.minimum {
			return retainedResultDiagnostic(configPath, setting.key, value, "bound_range", "range", "below_minimum", "outside permitted bounds")
		}
		*setting.value = number
	}
	if bounds.MaxRetainedResultBytes > 4294967295 {
		return retainedResultDiagnostic(configPath, "max_retained_result_bytes", mappingChild(node, "max_retained_result_bytes"), "entry_cap_bound", "range", "above_maximum", "per-entry cap exceeds the supported bound")
	}

	for _, setting := range []struct {
		key   string
		value *time.Duration
	}{
		{"handle_ttl", &bounds.HandleTTL},
		{"tombstone_ttl", &bounds.TombstoneTTL},
	} {
		value := mappingChild(node, setting.key)
		if value == nil {
			continue
		}
		if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
			return retainedResultDiagnostic(configPath, setting.key, value, "ttl_invalid", "duration", "wrong_type", "retention TTL must be a positive duration")
		}
		duration, err := time.ParseDuration(value.Value)
		if err != nil {
			return retainedResultDiagnostic(configPath, setting.key, value, "ttl_invalid", "duration", "invalid_duration", "retention TTL must be a positive duration")
		}
		if duration <= 0 {
			return retainedResultDiagnostic(configPath, setting.key, value, "ttl_invalid", "duration", "nonpositive", "retention TTL must be a positive duration")
		}
		*setting.value = duration
	}

	if bounds.PreviewCeilingBytes >= bounds.DeliveryCeilingBytes {
		return retainedResultDiagnostic(configPath, "preview_ceiling_bytes", mappingChild(node, "preview_ceiling_bytes"), "ceiling_order", "order", "not_strictly_less", "preview ceiling must be less than delivery ceiling")
	}
	if bounds.MaxRetainedResultBytes <= bounds.DeliveryCeilingBytes {
		return retainedResultDiagnostic(configPath, "max_retained_result_bytes", mappingChild(node, "max_retained_result_bytes"), "entry_cap_order", "order", "not_greater", "per-entry cap must exceed delivery ceiling")
	}
	if bounds.MaxBytesPerNamespace < bounds.MaxRetainedResultBytes {
		return retainedResultDiagnostic(configPath, "max_bytes_per_namespace", mappingChild(node, "max_bytes_per_namespace"), "namespace_cap_order", "order", "below_entry_cap", "namespace byte cap must cover the per-entry cap")
	}
	previewFeasible := retainedResultEnvelopeFits(maximalPreviewMetadata(), bounds.MaxUpstreamCodeBytes, bounds.PreviewCeilingBytes)
	pageFeasible := retainedResultEnvelopeFits(maximalPageMetadata(), bounds.MaxUpstreamCodeBytes, bounds.DeliveryCeilingBytes)
	terminalFeasible := retainedResultEnvelopeFits(maximalTerminalMetadata(), bounds.MaxUpstreamCodeBytes, bounds.PreviewCeilingBytes)
	if !previewFeasible || !pageFeasible || !terminalFeasible {
		key := "delivery_ceiling_bytes"
		if !previewFeasible || !terminalFeasible {
			key = "preview_ceiling_bytes"
		}
		return retainedResultDiagnostic(configPath, key, mappingChild(node, key), "envelope_feasibility", "feasibility", "insufficient_headroom", "maximal result envelope leaves insufficient rune headroom")
	}
	if bounds.MaxBytesAggregate < bounds.MaxRetainedResultBytes {
		return retainedResultDiagnostic(configPath, "max_bytes_aggregate", mappingChild(node, "max_bytes_aggregate"), "aggregate_order", "order", "below_entry_cap", "aggregate byte cap must cover the per-entry cap")
	}
	if bounds.MaxUpstreamCodeBytes < uint64(len("[truncated]")) {
		return retainedResultDiagnostic(configPath, "max_upstream_code_bytes", mappingChild(node, "max_upstream_code_bytes"), "upstream_code_marker_bound", "minimum", "below_minimum", "upstream code limit must fit the truncation marker")
	}
	return nil
}

func retainedResultUint(node *yaml.Node) (uint64, error) {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!int" {
		return 0, fmt.Errorf("not an integer")
	}
	return strconv.ParseUint(node.Value, 0, 64)
}

func retainedResultDiagnostic(configPath, key string, node *yaml.Node, code, property, shape, reason string) *Diagnostics {
	return single(Diagnostic{
		Path: configPath, Key: "retained_results." + key,
		Line: nodeLine(node), Column: nodeColumn(node),
		Code: code, Property: property, Shape: shape, Reason: reason,
	})
}

func retainedResultEnvelopeFits(metadata map[string]any, upstreamCodeBytes, ceiling uint64) bool {
	encoded, err := json.Marshal(&mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: ""}},
		StructuredContent: metadata,
	})
	if err != nil {
		return false
	}
	baseBytes := uint64(len(encoded))
	maxUint64 := ^uint64(0)
	if baseBytes > maxUint64-retainedEnvelopeHeadroom ||
		upstreamCodeBytes > maxUint64-baseBytes-retainedEnvelopeHeadroom {
		return false
	}
	return baseBytes+upstreamCodeBytes+retainedEnvelopeHeadroom <= ceiling
}

func maximalPreviewMetadata() map[string]any {
	return map[string]any{
		"paging_required": true,
		"handle":          strings.Repeat("h", 43),
		"total_bytes":     ^uint64(0),
		"is_error":        false,
		"upstream_code":   "",
	}
}

func maximalPageMetadata() map[string]any {
	return map[string]any{
		"handle":          strings.Repeat("h", 43),
		"page":            ^uint32(0),
		"total_pages":     ^uint32(0),
		"remaining_pages": ^uint32(0),
		"total_bytes":     ^uint64(0),
		"remaining_bytes": ^uint64(0),
		"is_error":        false,
		"upstream_code":   "",
	}
}

func maximalTerminalMetadata() map[string]any {
	return map[string]any{
		"retention":       "result_aggregate_capacity_exceeded",
		"execution_state": "executed",
		"total_bytes":     ^uint64(0),
		"is_error":        false,
		"upstream_code":   "",
	}
}

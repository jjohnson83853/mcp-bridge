package config

import (
	"fmt"
	"math"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// validateWeights checks the optional hybrid-search weights. Every rejection
// is one typed record (spec §3.1D): the rejected key, a finite
// weight_invalid/weights_invalid class, and a static reason — no observed
// number.
func validateWeights(root *yaml.Node, configPath string) *Diagnostics {
	keyword, embedding := findScalar(root, "keyword_weight"), findScalar(root, "embedding_weight")
	if keyword == "" && embedding == "" {
		return nil
	}
	invalid := func(key, shape string) *Diagnostics {
		return single(Diagnostic{
			Path: configPath, Key: key,
			Code: "weight_invalid", Property: "value", Shape: shape,
			Reason: "invalid " + strings.TrimPrefix(key, "search."),
		})
	}
	var k, e float64
	if keyword == "" {
		return invalid("search.keyword_weight", "missing")
	}
	if _, err := fmt.Sscan(keyword, &k); err != nil || math.IsNaN(k) || math.IsInf(k, 0) {
		return invalid("search.keyword_weight", "invalid_numeric")
	}
	if embedding == "" {
		return invalid("search.embedding_weight", "missing")
	}
	if _, err := fmt.Sscan(embedding, &e); err != nil || math.IsNaN(e) || math.IsInf(e, 0) {
		return invalid("search.embedding_weight", "invalid_numeric")
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
	return single(Diagnostic{
		Path: configPath, Key: "search.keyword_weight",
		RelatedKey: "search.embedding_weight",
		Code:       "weights_invalid", Property: "weights", Shape: shape,
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
	boundRange := func(keyPath, shape string) *Diagnostics {
		return single(Diagnostic{
			Path: configPath, Key: keyPath, Server: server,
			Code: "bound_range", Property: "range", Shape: shape,
			Reason: "outside permitted bounds",
		})
	}
	durationType := func(keyPath string, value *yaml.Node) *Diagnostics {
		return single(Diagnostic{
			Path: configPath, Key: keyPath, Server: server,
			Code: "duration_type", Property: "type", Shape: shapeOf(value),
			Reason: "must be a duration",
		})
	}
	durationRange := func(keyPath, shape string) *Diagnostics {
		return single(Diagnostic{
			Path: configPath, Key: keyPath, Server: server,
			Code: "duration_range", Property: "range", Shape: shape,
			Reason: "outside permitted duration bounds",
		})
	}
	durationPositive := func(keyPath, shape string) *Diagnostics {
		return single(Diagnostic{
			Path: configPath, Key: keyPath, Server: server,
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
					return boundRange(keyPath, "invalid_number")
				}
				switch {
				case number < bound[0]:
					return boundRange(keyPath, "below_minimum")
				case number > bound[1]:
					return boundRange(keyPath, "above_maximum")
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
					return durationRange(keyPath, "invalid_duration")
				case duration < sessionIdleMinDuration:
					return durationRange(keyPath, "below_minimum")
				case duration > sessionIdleMaxDuration:
					return durationRange(keyPath, "above_maximum")
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
					return durationRange(keyPath, "invalid_duration")
				case duration <= 0:
					return durationRange(keyPath, "nonpositive")
				case duration > limit:
					return durationRange(keyPath, "above_maximum")
				}
			}
			if key.Value == "refresh_interval" || key.Value == "stale_after" || key.Value == "query_cache_ttl" || key.Value == "open_duration" {
				duration, err := time.ParseDuration(value.Value)
				switch {
				case err != nil:
					return durationPositive(keyPath, "invalid_duration")
				case duration <= 0:
					return durationPositive(keyPath, "nonpositive")
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

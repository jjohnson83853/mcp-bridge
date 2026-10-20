// Package catalog holds the bridge's own MCP tool schemas and the exact
// JSON Schema admission applied to upstream tool arguments (r8 plan §4.4).
package catalog

// Bridge tool names advertised on the single /mcp endpoint (r8 §4.4).
const (
	ToolSearchTools  = "search_tools"
	ToolDescribeTool = "describe_tool"
	ToolCallTool     = "call_tool"
)

// JSON Schema dialect pinned for the bridge's own tools (r8 §4.4).
const Draft202012 = "https://json-schema.org/draft/2020-12/schema"

// Exact §4.4 patterns for the bridge's own parameter values.
const (
	PublicToolNamePattern  = `^(t_[a-z2-7]{52}|[a-z][a-z0-9-]{0,31}__[a-zA-Z0-9_-]{1,94})$`
	CatalogRevisionPattern = "^[a-f0-9]{64}$"
	ServerIDPattern        = "^[a-z][a-z0-9-]{0,31}$"
)

// Bounds fixed by r8 §4.2/§4.4 for the bridge's own parameters.
const (
	QueryMaxLength    = 2048
	TopKDefault       = 10
	TopKMaximum       = 50
	ServerFilterMax   = 32
	MaxServerIdentity = 1024
)

// SearchToolsInputSchema returns the exact search_tools inputSchema (r8 §4.4).
// Effective default/max are narrowed from config at runtime.
func SearchToolsInputSchema() map[string]any {
	return map[string]any{
		"$schema":              Draft202012,
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"query"},
		"properties": map[string]any{
			"query": map[string]any{
				"type":      "string",
				"minLength": 1,
				"maxLength": QueryMaxLength,
			},
			"top_k": map[string]any{
				"type":    "integer",
				"minimum": 1,
				"maximum": TopKMaximum,
				"default": TopKDefault,
			},
			"servers": map[string]any{
				"type":        "array",
				"minItems":    1,
				"maxItems":    ServerFilterMax,
				"uniqueItems": true,
				"items":       map[string]any{"type": "string", "pattern": ServerIDPattern},
			},
		},
	}
}

// DescribeToolInputSchema returns the exact describe_tool inputSchema (r8 §4.4).
func DescribeToolInputSchema() map[string]any {
	return map[string]any{
		"$schema":              Draft202012,
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"tool_name", "catalog_revision"},
		"properties": map[string]any{
			"tool_name":        map[string]any{"type": "string", "pattern": PublicToolNamePattern},
			"catalog_revision": map[string]any{"type": "string", "pattern": CatalogRevisionPattern},
		},
	}
}

// CallToolInputSchema returns the exact call_tool inputSchema (r8 §4.4).
func CallToolInputSchema() map[string]any {
	return map[string]any{
		"$schema":              Draft202012,
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"tool_name", "catalog_revision", "arguments"},
		"properties": map[string]any{
			"tool_name":        map[string]any{"type": "string", "pattern": PublicToolNamePattern},
			"catalog_revision": map[string]any{"type": "string", "pattern": CatalogRevisionPattern},
			"arguments":        map[string]any{"type": "object"},
		},
	}
}

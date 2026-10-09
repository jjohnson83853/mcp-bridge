package catalog_test

import (
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
)

// assertSchema pins the §4.4 root invariants shared by all three bridge tools.
func assertSchema(t *testing.T, name string, schema map[string]any, required ...string) map[string]any {
	t.Helper()
	if schema["$schema"] != catalog.Draft202012 {
		t.Fatalf("%s $schema = %v; want draft 2020-12", name, schema["$schema"])
	}
	if schema["type"] != "object" {
		t.Fatalf("%s type = %v; want object", name, schema["type"])
	}
	if schema["additionalProperties"] != false {
		t.Fatalf("%s additionalProperties = %v; want false", name, schema["additionalProperties"])
	}
	got, ok := schema["required"].([]any)
	if !ok || len(got) != len(required) {
		t.Fatalf("%s required = %v; want %v", name, schema["required"], required)
	}
	for i, want := range required {
		if got[i] != want {
			t.Fatalf("%s required[%d] = %v; want %s", name, i, got[i], want)
		}
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("%s properties = %v; want object", name, schema["properties"])
	}
	return properties
}

func TestSearchToolsSchemaMatchesContract(t *testing.T) {
	properties := assertSchema(t, "search_tools", catalog.SearchToolsInputSchema(), "query")

	query := properties["query"].(map[string]any)
	if query["type"] != "string" || query["minLength"] != 1 || query["maxLength"] != catalog.QueryMaxLength {
		t.Fatalf("search_tools query = %v; want bounded string", query)
	}
	topK := properties["top_k"].(map[string]any)
	if topK["type"] != "integer" || topK["minimum"] != 1 ||
		topK["maximum"] != catalog.TopKMaximum || topK["default"] != catalog.TopKDefault {
		t.Fatalf("search_tools top_k = %v; want bounded integer", topK)
	}
	servers := properties["servers"].(map[string]any)
	if servers["type"] != "array" || servers["minItems"] != 1 ||
		servers["maxItems"] != catalog.ServerFilterMax || servers["uniqueItems"] != true {
		t.Fatalf("search_tools servers = %v; want bounded unique array", servers)
	}
	items := servers["items"].(map[string]any)
	if items["pattern"] != catalog.ServerIDPattern {
		t.Fatalf("search_tools servers items pattern = %v; want %s", items["pattern"], catalog.ServerIDPattern)
	}
	// The removed namespace field must never reappear.
	if _, present := properties["namespace"]; present {
		t.Fatalf("search_tools advertises a namespace field: %v", properties)
	}
}

func TestDescribeToolSchemaMatchesContract(t *testing.T) {
	properties := assertSchema(t, "describe_tool", catalog.DescribeToolInputSchema(), "tool_name", "catalog_revision")

	toolName := properties["tool_name"].(map[string]any)
	if toolName["pattern"] != catalog.PublicToolNamePattern {
		t.Fatalf("describe_tool tool_name pattern = %v; want %s", toolName["pattern"], catalog.PublicToolNamePattern)
	}
	revision := properties["catalog_revision"].(map[string]any)
	if revision["pattern"] != catalog.CatalogRevisionPattern {
		t.Fatalf("describe_tool catalog_revision pattern = %v; want %s", revision["pattern"], catalog.CatalogRevisionPattern)
	}
}

func TestCallToolSchemaMatchesContract(t *testing.T) {
	properties := assertSchema(t, "call_tool", catalog.CallToolInputSchema(), "tool_name", "catalog_revision", "arguments")

	if properties["tool_name"].(map[string]any)["pattern"] != catalog.PublicToolNamePattern {
		t.Fatalf("call_tool tool_name pattern = %v", properties["tool_name"])
	}
	if properties["catalog_revision"].(map[string]any)["pattern"] != catalog.CatalogRevisionPattern {
		t.Fatalf("call_tool catalog_revision pattern = %v", properties["catalog_revision"])
	}
	arguments := properties["arguments"].(map[string]any)
	if arguments["type"] != "object" {
		t.Fatalf("call_tool arguments = %v; want object", arguments)
	}
}

// TestSchemasAreIndependent guards callers from sharing mutable schema state.
func TestSchemasAreIndependent(t *testing.T) {
	first := catalog.SearchToolsInputSchema()
	first["type"] = "mutated"
	properties := first["properties"].(map[string]any)
	properties["query"].(map[string]any)["maxLength"] = 1
	if second := catalog.SearchToolsInputSchema(); second["type"] != "object" ||
		second["properties"].(map[string]any)["query"].(map[string]any)["maxLength"] != catalog.QueryMaxLength {
		t.Fatalf("SearchToolsInputSchema returned shared mutable state: %v", second)
	}
}

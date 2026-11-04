package catalog_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
)

// assertSchema pins the §4.4 root invariants shared by all three bridge tools.
func assertSchema(t *testing.T, name string, schema map[string]any, required ...string) map[string]any {
	t.Helper()
	if _, hasSchema := schema["$schema"]; hasSchema {
		t.Fatalf("%s has unexpected $schema: %v", name, schema["$schema"])
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
	if servers["type"] != "array" || servers["uniqueItems"] != true {
		t.Fatalf("search_tools servers = %v; want unique array", servers)
	}
	if _, hasMin := servers["minItems"]; hasMin {
		t.Fatalf("search_tools servers has unexpected minItems: %v", servers["minItems"])
	}
	if _, hasMax := servers["maxItems"]; hasMax {
		t.Fatalf("search_tools servers has unexpected maxItems: %v", servers["maxItems"])
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
	properties := assertSchema(t, "describe_tool", catalog.DescribeToolInputSchema(), "tool_name")

	toolName := properties["tool_name"].(map[string]any)
	if toolName["pattern"] != catalog.PublicToolNamePattern {
		t.Fatalf("describe_tool tool_name pattern = %v; want %s", toolName["pattern"], catalog.PublicToolNamePattern)
	}
	revision := properties["catalog_revision"].(map[string]any)
	if revision["pattern"] != catalog.ToolInputCatalogRevisionPattern {
		t.Fatalf("describe_tool catalog_revision pattern = %v; want %s", revision["pattern"], catalog.ToolInputCatalogRevisionPattern)
	}
}

func TestCallToolSchemaMatchesContract(t *testing.T) {
	properties := assertSchema(t, "call_tool", catalog.CallToolInputSchema(), "tool_name", "arguments")

	if properties["tool_name"].(map[string]any)["pattern"] != catalog.PublicToolNamePattern {
		t.Fatalf("call_tool tool_name pattern = %v", properties["tool_name"])
	}
	if properties["catalog_revision"].(map[string]any)["pattern"] != catalog.ToolInputCatalogRevisionPattern {
		t.Fatalf("call_tool catalog_revision pattern = %v; want %s", properties["catalog_revision"], catalog.ToolInputCatalogRevisionPattern)
	}
	arguments := properties["arguments"].(map[string]any)
	if arguments["type"] != "object" || arguments["additionalProperties"] != true ||
		arguments["description"] == "" {
		t.Fatalf("call_tool arguments = %v; want extensible described object", arguments)
	}
}
func TestCatalogRevisionPatterns(t *testing.T) {
	strictPattern := regexp.MustCompile(catalog.CatalogRevisionPattern)
	toolInputPattern := regexp.MustCompile(catalog.ToolInputCatalogRevisionPattern)
	digest := strings.Repeat("a", 64)

	if !strictPattern.MatchString(digest) {
		t.Errorf("expected %q to match CatalogRevisionPattern", digest)
	}
	if strictPattern.MatchString("") {
		t.Error("expected empty string to be rejected by CatalogRevisionPattern")
	}
	for _, input := range []string{"not-hex", "1234", strings.Repeat("A", 64)} {
		if strictPattern.MatchString(input) {
			t.Errorf("expected %q to be rejected by CatalogRevisionPattern", input)
		}
	}

	for _, input := range []string{"", digest} {
		if !toolInputPattern.MatchString(input) {
			t.Errorf("expected %q to match ToolInputCatalogRevisionPattern", input)
		}
	}
	for _, input := range []string{"not-hex", "1234", strings.Repeat("A", 64)} {
		if toolInputPattern.MatchString(input) {
			t.Errorf("expected %q to be rejected by ToolInputCatalogRevisionPattern", input)
		}
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

// TestPublicToolNamePattern verifies the exact §4.4 admission pattern for
// tool_name: opaque hashes (t_...) and preloaded wire names (server__tool)
// match, while malformed names are rejected.
func TestPublicToolNamePattern(t *testing.T) {
	pattern := regexp.MustCompile(catalog.PublicToolNamePattern)

	valid := []struct {
		name  string
		input string
	}{
		{"opaque hash", "t_" + strings.Repeat("a", 52)},
		{"wire name", "rag__search_docs"},
	}
	for _, tc := range valid {
		if !pattern.MatchString(tc.input) {
			t.Errorf("%s: expected %q to match PublicToolNamePattern", tc.name, tc.input)
		}
	}

	invalid := []string{
		"",
		"search_tools",
		"rag_search_docs",
		"rag__",
		"__search_docs",
		"Rag__search_docs",
		"rag__search docs",
		strings.Repeat("a", 33) + "__tool",
	}
	for _, input := range invalid {
		if pattern.MatchString(input) {
			t.Errorf("expected %q to be rejected by PublicToolNamePattern", input)
		}
	}
}

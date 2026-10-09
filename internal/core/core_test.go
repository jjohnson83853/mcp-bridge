package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPublicNameCanonical(t *testing.T) {
	got := PublicName("portainer", "list_endpoints")
	if len(got) != 54 {
		t.Fatalf("public name length %d", len(got))
	}
	if strings.ToLower(got) != got || !strings.HasPrefix(got, "t_") {
		t.Fatalf("prefix/case: %s", got)
	}
	// same raw name on different servers → different public names
	other := PublicName("developer", "list_endpoints")
	if other == got {
		t.Fatalf("same public name across servers")
	}
}

// Bindings lists every bound tool in stable public-name order and hands out
// copies, so the tool surface can index the published catalog safely.
func TestBindingsListsEveryBindingInPublicNameOrder(t *testing.T) {
	bridge := NewBridge()
	schema := json.RawMessage(`{"type":"object"}`)
	if err := bridge.Register("alpha", "https://alpha.invalid/mcp", []ToolDefinition{
		{RawName: "write", InputSchema: schema},
		{RawName: "read", InputSchema: schema},
	}); err != nil {
		t.Fatalf("Register alpha: %v", err)
	}
	if err := bridge.Register("beta", "https://beta.invalid/mcp", []ToolDefinition{
		{RawName: "read", InputSchema: schema},
	}); err != nil {
		t.Fatalf("Register beta: %v", err)
	}

	bindings := bridge.Bindings()
	if len(bindings) != 3 {
		t.Fatalf("Bindings returned %d entries; want 3", len(bindings))
	}
	for i := 1; i < len(bindings); i++ {
		previous := PublicName(bindings[i-1].ServerID, bindings[i-1].RawName)
		current := PublicName(bindings[i].ServerID, bindings[i].RawName)
		if previous >= current {
			t.Fatalf("Bindings are not in public-name order: %s then %s", previous, current)
		}
	}

	bindings[0].RawName = "mutated"
	bindings[0].Def.Description = "mutated"
	if _, ok := bridge.Binding(PublicName(bindings[0].ServerID, "mutated")); ok {
		t.Fatal("Bindings exposed the catalog's own binding")
	}
	stored, ok := bridge.Binding(PublicName("alpha", "read"))
	if !ok {
		t.Fatal("Bindings lost alpha/read")
	}
	if stored.Def.InputSchema == nil || stored.Def.Description == "mutated" {
		t.Fatalf("the catalog binding was corrupted through Bindings: %+v", stored)
	}
}

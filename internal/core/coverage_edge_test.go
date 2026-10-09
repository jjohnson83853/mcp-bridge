package core_test

import (
	"context"
	"encoding/json"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"testing"
)

func TestBridgeAdmissionAndAccessorEdges(t *testing.T) {
	b := core.NewBridge()
	if err := b.Register("s", "e", nil); err != nil {
		t.Fatal(err)
	}
	if len(b.Bindings()) != 0 {
		t.Fatal("empty registration")
	}
	if _, ok := b.Binding("missing"); ok {
		t.Fatal("missing binding")
	}
	if err := b.Register("s", "e", []core.ToolDefinition{{RawName: "x"}, {RawName: "x"}}); err == nil {
		t.Fatal("duplicate accepted")
	}
	if len(b.Bindings()) != 0 {
		t.Fatal("failed registration mutated")
	}
	if err := b.Register("s", "e", []core.ToolDefinition{{}}); err == nil {
		t.Fatal("empty raw accepted")
	}
	if err := b.Register("s", "e", []core.ToolDefinition{{RawName: "x"}}); err != nil {
		t.Fatal(err)
	}
	rev := b.Revision()
	pub := core.PublicName("s", "x")
	listed := b.Bindings()
	listed[0].RawName = "mutated"
	binding, _ := b.Binding(pub)
	if binding.RawName != "x" {
		t.Fatal("binding copy contract")
	}
	if err := b.Register("s", "other", nil); err == nil {
		t.Fatal("endpoint rebind")
	}
	if _, err := core.AddPins(b, []core.PinnedTool{{ServerID: "absent", RawName: "tool"}}); err == nil {
		t.Fatal("missing pin accepted")
	}
	if _, err := core.AddPins(b, []core.PinnedTool{{ServerID: "s", RawName: "x"}}); err != nil {
		t.Fatal(err)
	}
	var recorder core.UpstreamRecorder
	if _, e := b.Dispatch(context.Background(), "unknown", rev, nil, &recorder); e == nil || e.Code != "unknown_tool" {
		t.Fatalf("unknown=%+v", e)
	}
	if _, e := b.Dispatch(context.Background(), pub, "stale", nil, &recorder); e == nil || e.Code != "catalog_changed" {
		t.Fatalf("stale=%+v", e)
	}
}
func TestRevisionTracksDescriptionSchemaAndWhitespace(t *testing.T) {
	makeBridge := func(def core.ToolDefinition) *core.Bridge {
		b := core.NewBridge()
		if err := b.Register("s", "e", []core.ToolDefinition{def}); err != nil {
			t.Fatal(err)
		}
		return b
	}
	a := makeBridge(core.ToolDefinition{RawName: "x", Description: "one", InputSchema: json.RawMessage(`{ "type" : "object" }`)})
	b := makeBridge(core.ToolDefinition{RawName: "x", Description: "one", InputSchema: json.RawMessage(`{"type":"object"}`)})
	if a.Revision() != b.Revision() {
		t.Fatal("whitespace changed revision")
	}
	c := makeBridge(core.ToolDefinition{RawName: "x", Description: "two"})
	d := makeBridge(core.ToolDefinition{RawName: "x", InputSchema: json.RawMessage(`{"type":"string"}`)})
	if a.Revision() == c.Revision() || a.Revision() == d.Revision() {
		t.Fatal("content did not change revision")
	}
}

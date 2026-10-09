package core_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"testing"
)

func TestDefinitionDigestCanonicalSchemaAndIndependentEncoding(t *testing.T) {
	a := core.ToolDefinition{RawName: "tool", Description: "run", InputSchema: json.RawMessage(`{ "type" : "object", "properties" : { "x" : { "type" : "string" } } }`)}
	b := core.ToolDefinition{RawName: "tool", Description: "run", InputSchema: json.RawMessage(`{"properties":{"x":{"type":"string"}},"type":"object"}`)}
	got := core.DefinitionDigest("server", "tool", a)
	if got != core.DefinitionDigest("server", "tool", b) {
		t.Fatal("schema whitespace/key order changed digest")
	}
	entry := struct {
		Server      string          `json:"server"`
		Raw         string          `json:"raw"`
		Description string          `json:"description"`
		Input       json.RawMessage `json:"input_schema"`
		Output      json.RawMessage `json:"output_schema"`
	}{"server", "tool", "run", json.RawMessage(`{"properties":{"x":{"type":"string"}},"type":"object"}`), nil}
	encoded, _ := json.Marshal(entry)
	sum := sha256.Sum256(encoded)
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("digest=%s want independently computed %s bytes=%s", got, want, encoded)
	}
}

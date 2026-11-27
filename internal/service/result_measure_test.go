package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMeasurementIndependentOracle(t *testing.T) {
	largePayload := strings.Repeat("x", 2700)
	fixtures := []struct {
		name           string
		result         *mcp.CallToolResult
		wantBytes      string
		wantByteLength int
	}{
		{
			name: "duplicate content below delivery ceiling",
			result: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "dup"},
					&mcp.TextContent{Text: "dup"},
				},
				StructuredContent: map[string]any{"count": 2, "duplicate": "dup"},
				IsError:           true,
			},
			wantBytes:      `{"content":[{"type":"text","text":"dup"},{"type":"text","text":"dup"}],"structuredContent":{"count":2,"duplicate":"dup"},"isError":true}`,
			wantByteLength: 136,
		},
		{
			name: "duplicate content above delivery ceiling",
			result: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: largePayload},
					&mcp.TextContent{Text: largePayload},
				},
				StructuredContent: map[string]any{"copy": largePayload},
			},
			wantBytes: `{"content":[{"type":"text","text":"` +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				`"},{"type":"text","text":"` +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				`"}],"structuredContent":{"copy":"` +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				(`xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx` + `xxxxxxxxxx`) +
				`"}}`,
			wantByteLength: 8197,
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			if len(fixture.wantBytes) != fixture.wantByteLength {
				t.Fatalf("golden fixture length = %d, want literal %d", len(fixture.wantBytes), fixture.wantByteLength)
			}
			marshaled, err := json.Marshal(fixture.result)
			if err != nil {
				t.Fatalf("marshal fixture: %v", err)
			}
			measured, err := measureResult(fixture.result)
			if err != nil {
				t.Fatalf("measure fixture: %v", err)
			}
			if measured != fixture.wantByteLength {
				t.Errorf("measureResult() = %d bytes, want literal %d", measured, fixture.wantByteLength)
			}
			if string(marshaled) != fixture.wantBytes {
				t.Errorf("marshaled result bytes differ from the literal golden fixture")
			}
			if (measured > 8192) != (fixture.wantByteLength > 8192) {
				t.Errorf("measured %d bytes on wrong side of the default 8192-byte delivery ceiling", measured)
			}
		})
	}
}

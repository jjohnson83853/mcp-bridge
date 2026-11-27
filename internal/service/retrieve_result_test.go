package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func retrievalRequest(raw string) *mcp.CallToolRequest {
	return &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(raw)}}
}

func retrieveForOracle(t *testing.T, surface *ToolSurface, arguments string) *mcp.CallToolResult {
	t.Helper()
	result, err := surface.retrieveResult(context.Background(), retrievalRequest(arguments))
	if err != nil {
		t.Fatalf("retrieve_result returned protocol error: %v", err)
	}
	if result == nil {
		t.Fatal("retrieve_result returned nil tool result")
	}
	return result
}

func TestHandleRejectionUniformHandler(t *testing.T) {
	store, owner, resolve, _ := newDeliveryOracle(t)
	other := resultNamespace{sessionID: "other-session", epoch: "1"}
	foreign := deliverBoundedResult(store, deliveryFixture(strings.Repeat("foreign", 200), false, ""), other, func(value resultNamespace) (resultNamespace, bool) { return value, true }, "")
	foreignHandle := structuredResult(foreign)["handle"].(string)
	surface := &ToolSurface{results: store}
	inputs := []string{
		`{"handle":"` + strings.Repeat("A", 43) + `"}`,
		`{"handle":"` + strings.Repeat("A", 43) + `"}`,
		`{"handle":"` + foreignHandle + `"}`,
		`{"handle":"` + strings.Repeat("B", 43) + `"}`,
		`{"handle":"` + strings.Repeat("C", 43) + `"}`,
		`{"handle":"` + strings.Repeat("D", 43) + `"}`,
	}
	var expected []byte
	for _, input := range inputs {
		result := retrieveForOracle(t, surface, input)
		if !result.IsError || structuredResult(result)["code"] != "invalid_handle" {
			t.Fatalf("rejection = %#v / isError=%v", structuredResult(result), result.IsError)
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		if expected == nil {
			expected = encoded
		} else if string(encoded) != string(expected) {
			t.Fatalf("handler rejections differ: %s != %s", encoded, expected)
		}
	}
	_ = owner
	_ = resolve
}

func TestPageBoundsValidated(t *testing.T) {
	store, owner, resolve, _ := newDeliveryOracle(t)
	preview := deliverBoundedResult(store, deliveryFixture(strings.Repeat("pages", 200), false, ""), owner, resolve, "")
	handle := structuredResult(preview)["handle"].(string)
	surface := &ToolSurface{results: store}
	invalid := retrieveForOracle(t, surface, `{"handle":"`+handle+`","page":9999}`)
	if !invalid.IsError || structuredResult(invalid)["code"] != "invalid_arguments" {
		t.Fatalf("out-of-range page result = %#v", structuredResult(invalid))
	}
	if len(invalid.Content) != 1 || len(structuredResult(invalid)) != 4 {
		t.Fatalf("out-of-range rejection has page data: %#v", invalid)
	}
}

func TestRetrieveShapeInvalid(t *testing.T) {
	store, owner, resolve, _ := newDeliveryOracle(t)
	preview := deliverBoundedResult(store, deliveryFixture(strings.Repeat("live", 200), false, ""), owner, resolve, "")
	handle := structuredResult(preview)["handle"].(string)
	surface := &ToolSurface{results: store}
	cases := []string{
		`{"handle":"bad","page":1}`,
		`{"handle":"` + handle + `","page":0}`,
		`{"handle":"` + handle + `","page":1.5}`,
		`{"handle":"` + handle + `","page":"1"}`,
	}
	for _, input := range cases {
		result := retrieveForOracle(t, surface, input)
		if !result.IsError || structuredResult(result)["code"] != "invalid_arguments" {
			t.Errorf("input %s result = %#v", input, structuredResult(result))
		}
	}
}

func TestRetrieveValidationPrecedence(t *testing.T) {
	store, owner, resolve, _ := newDeliveryOracle(t)
	preview := deliverBoundedResult(store, deliveryFixture(strings.Repeat("live", 200), false, ""), owner, resolve, "")
	handle := structuredResult(preview)["handle"].(string)
	surface := &ToolSurface{results: store}
	cases := []struct {
		name string
		args string
		want string
	}{
		{name: "invalid handle before invalid page", args: `{"handle":"bad","page":0}`, want: "invalid_arguments"},
		{name: "unresolved handle before invalid page", args: `{"handle":"` + strings.Repeat("Z", 43) + `","page":0}`, want: "invalid_handle"},
		{name: "owner-bound invalid page", args: `{"handle":"` + handle + `","page":0}`, want: "invalid_arguments"},
		{name: "owner-bound out of bounds", args: `{"handle":"` + handle + `","page":9999}`, want: "invalid_arguments"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := retrieveForOracle(t, surface, tc.args)
			if got := structuredResult(result)["code"]; got != tc.want {
				t.Fatalf("code=%v, want %s", got, tc.want)
			}
		})
	}
}

func TestRetrieveResultDefaultsPage(t *testing.T) {
	store, owner, resolve, _ := newDeliveryOracle(t)
	preview := deliverBoundedResult(store, deliveryFixture(strings.Repeat("page", 200), false, ""), owner, resolve, "")
	handle := structuredResult(preview)["handle"].(string)
	page := retrieveForOracle(t, &ToolSurface{results: store}, `{"handle":"`+handle+`"}`)
	if page.IsError || structuredResult(page)["page"] != 1 {
		t.Fatalf("default retrieval page = %#v", page)
	}
}

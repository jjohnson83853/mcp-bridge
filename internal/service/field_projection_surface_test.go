package service

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"github.com/jjohnson83853/mcp-bridge/internal/config"
)

func TestFieldProjectionSurfaceSchema(t *testing.T) {
	schema := catalog.CallToolInputSchema()
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("call_tool schema has no properties object")
	}
	projection, ok := properties["projection"].(map[string]any)
	if !ok {
		t.Fatal("call_tool schema has no projection object")
	}
	if projection["type"] != "object" || projection["additionalProperties"] != false {
		t.Fatalf("projection schema is not a strict object: %#v", projection)
	}
	members, ok := projection["properties"].(map[string]any)
	if !ok {
		t.Fatalf("projection schema has no properties object: %#v", projection)
	}
	if members["full"].(map[string]any)["type"] != "boolean" {
		t.Fatalf("projection.full is not boolean: %#v", members["full"])
	}
	fields, ok := members["fields"].(map[string]any)
	if !ok || fields["type"] != "array" || fields["minItems"] != 1 || fields["maxItems"] != 16 {
		t.Fatalf("projection.fields does not expose the bounded selection: %#v", members["fields"])
	}
	fieldItems, ok := fields["items"].(map[string]any)
	if !ok || fieldItems["type"] != "string" || fieldItems["maxLength"] != 64 || fieldItems["pattern"] != "^[A-Za-z0-9_]+$" {
		t.Fatalf("projection.fields items do not expose the field-name rule: %#v", fields["items"])
	}
}

const projectionTestBudget = 16*67 + 1

func TestProjectionSurfaceAdmissionLadder(t *testing.T) {
	fields := []any{"Id"}
	valid := map[string]any{"fields": fields}
	fullFields := map[string]any{"full": true, "fields": fields}
	empty := map[string]any{"fields": []any{}}
	duplicate := map[string]any{"fields": []any{"Id", "Id"}}
	tooMany := make([]any, 17)
	for i := range tooMany {
		tooMany[i] = "F" + strings.Repeat("x", i)
	}
	cases := []struct {
		name, server, tool, path string
		args                     map[string]any
		budget                   int
		reason, mode             string
	}{
		{"non-object L1", "portainer", "EndpointList", "", map[string]any{"projection": "bad"}, projectionTestBudget, projectionReasonMalformedRequest, ""},
		{"wrong member type L1", "portainer", "EndpointList", "", map[string]any{"projection": map[string]any{"full": "true", "fields": fields}, "select": "x"}, projectionTestBudget, projectionReasonMalformedRequest, ""},
		{"full true fields L2", "portainer", "EndpointList", "", map[string]any{"projection": fullFields}, projectionTestBudget, projectionReasonMalformedRequest, ""},
		{"select before empty L3", "portainer", "EndpointList", "", map[string]any{"projection": empty, "select": "x"}, projectionTestBudget, projectionReasonSelectConflict, ""},
		{"select before duplicate L3", "portainer", "EndpointList", "", map[string]any{"projection": duplicate, "select": "x"}, projectionTestBudget, projectionReasonSelectConflict, ""},
		{"empty L4", "portainer", "EndpointList", "", map[string]any{"projection": empty}, projectionTestBudget, projectionReasonMalformedRequest, ""},
		{"duplicate L4", "portainer", "EndpointList", "", map[string]any{"projection": duplicate}, projectionTestBudget, projectionReasonMalformedRequest, ""},
		{"too many L4", "portainer", "EndpointList", "", map[string]any{"projection": map[string]any{"fields": tooMany}}, projectionTestBudget, projectionReasonMalformedRequest, ""},
		{"too long L4", "portainer", "EndpointList", "", map[string]any{"projection": map[string]any{"fields": []any{strings.Repeat("A", 65)}}}, projectionTestBudget, projectionReasonMalformedRequest, ""},
		{"bad character L4", "portainer", "EndpointList", "", map[string]any{"projection": map[string]any{"fields": []any{"Mount-Id"}}}, projectionTestBudget, projectionReasonMalformedRequest, ""},
		{"static precedes support", "other", "EndpointList", "", map[string]any{"projection": empty}, projectionTestBudget, projectionReasonMalformedRequest, ""},
		{"unsupported L5", "other", "EndpointList", "", map[string]any{"projection": valid}, projectionTestBudget, projectionReasonNotSupported, ""},
		{"over budget L6", "portainer", "EndpointList", "", map[string]any{"projection": valid}, 0, projectionReasonNotSupported, ""},
		{"false full composite", "portainer", "EndpointList", "", map[string]any{"projection": map[string]any{"full": false, "fields": fields}}, projectionTestBudget, "", projectionModeFields},
		{"false full plus select", "portainer", "EndpointList", "", map[string]any{"projection": map[string]any{"full": false, "fields": fields}, "select": "x"}, projectionTestBudget, projectionReasonSelectConflict, ""},
		{"exact docker path", "portainer", "docker_proxy", "/containers/json", map[string]any{"projection": valid, "path": "/containers/json"}, projectionTestBudget, "", projectionModeFields},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := decideProjection(tc.server, tc.tool, tc.path, tc.args, 0, tc.budget)
			if tc.reason != "" {
				if err == nil || decision.Reason != tc.reason {
					t.Fatalf("decision=%#v err=%v want=%s", decision, err, tc.reason)
				}
				return
			}
			if err != nil || decision.Mode != tc.mode {
				t.Fatalf("decision=%#v err=%v want mode=%s", decision, err, tc.mode)
			}
		})
	}
}

func TestProjectionSurfaceSelectPresenceIsComputedBeforeAdmission(t *testing.T) {
	for _, value := range []any{nil, ""} {
		_, args := projectionRequestArgs(json.RawMessage(`{"fields":["Id"]}`), mustProjectionJSON(t, map[string]any{"select": value}))
		if _, ok := args["select"]; ok {
			t.Fatalf("select %#v should be absent", value)
		}
	}
	projection, args := projectionRequestArgs(json.RawMessage(`{"full":false,"fields":["Id"]}`), json.RawMessage(`{"select":"Id"}`))
	if !projection.SelectPresent || args["select"] != "Id" {
		t.Fatalf("select not captured before admission: %#v %#v", projection, args)
	}
}

func TestProjectionSurfaceEngineShapesAndModeFields(t *testing.T) {
	endpoint := map[string]any{"Id": "ep-1", "Name": "primary", "Type": 1, "Hidden": true}
	projected, decision, err := transformProjection("portainer", "EndpointList", "", map[string]any{}, []any{endpoint}, projectionTestBudget)
	if err != nil || !decision.Applied {
		t.Fatalf("default projection %#v, %v", decision, err)
	}
	if !reflect.DeepEqual(projected, []any{map[string]any{"Id": "ep-1", "Name": "primary", "Type": 1}}) {
		t.Fatalf("default fields = %#v", projected)
	}
	assertDecisionFields(t, decision, []string{"Id", "Name", "Type"})

	empty, emptyDecision, err := transformProjection("portainer", "EndpointList", "", map[string]any{}, []any{}, projectionTestBudget)
	if err != nil || !emptyDecision.Applied || len(empty) != 0 {
		t.Fatalf("empty default %#v %#v %v", empty, emptyDecision, err)
	}
	fieldsArgs := map[string]any{"projection": map[string]any{"fields": []any{"Id", "Name"}}}
	partial, fieldsDecision, err := transformProjection("portainer", "EndpointList", "", fieldsArgs, []any{map[string]any{"Id": "a"}, map[string]any{"Id": "b", "Name": "B"}}, projectionTestBudget)
	if err != nil || !fieldsDecision.Applied {
		t.Fatalf("partial custom fields %#v %v", fieldsDecision, err)
	}
	if !reflect.DeepEqual(partial, []any{map[string]any{"Id": "a"}, map[string]any{"Id": "b", "Name": "B"}}) {
		t.Fatalf("partial fields = %#v", partial)
	}
	assertDecisionFields(t, fieldsDecision, []string{"Id", "Name"})
	emptyFields, emptyFieldsDecision, err := transformProjection("portainer", "EndpointList", "", map[string]any{"projection": map[string]any{"fields": []any{"Unknown"}}}, []any{}, projectionTestBudget)
	if err != nil || !emptyFieldsDecision.Applied || emptyFieldsDecision.Mode != projectionModeFields || len(emptyFields) != 0 {
		t.Fatalf("empty fields %#v %#v %v", emptyFields, emptyFieldsDecision, err)
	}
	_, unknown, err := transformProjection("portainer", "EndpointList", "", map[string]any{"projection": map[string]any{"fields": []any{"Id", "Missing"}}}, []any{map[string]any{"Id": "a"}}, projectionTestBudget)
	if err == nil || unknown.Reason != projectionReasonUnknownFieldSelection {
		t.Fatalf("unknown field decision %#v, %v", unknown, err)
	}

	aggregate, aggregateDecision, err := transformProjection("portainer", "EndpointList", "", map[string]any{}, []any{map[string]any{"Other": "x"}}, projectionTestBudget)
	if err != nil || aggregateDecision.Applied || aggregateDecision.Reason != projectionReasonItemShapeUnexpected || !reflect.DeepEqual(aggregate, []any{map[string]any{"Other": "x"}}) {
		t.Fatalf("aggregate miss %#v %#v %v", aggregate, aggregateDecision, err)
	}
	assertDecisionFields(t, aggregateDecision, []string{"Id", "Name", "Type"})
	mixed := []any{map[string]any{"Id": "a"}, "not-an-object"}
	mixedResult, mixedDecision, err := transformProjection("portainer", "EndpointList", "", map[string]any{}, mixed, projectionTestBudget)
	if err != nil || mixedDecision.Applied || mixedDecision.Reason != projectionReasonItemShapeUnexpected || !reflect.DeepEqual(mixedResult, mixed) {
		t.Fatalf("mixed list %#v %#v %v", mixedResult, mixedDecision, err)
	}

	fullArgs := map[string]any{"projection": map[string]any{"full": true}}
	full, fullDecision, err := transformProjection("portainer", "EndpointList", "", fullArgs, []any{endpoint}, projectionTestBudget)
	if err != nil || fullDecision.Applied || fullDecision.Reason != projectionReasonFullRequested || !reflect.DeepEqual(full, []any{endpoint}) {
		t.Fatalf("full opt-in %#v %#v %v", full, fullDecision, err)
	}
	assertDecisionFields(t, fullDecision, []string{})
	assertDisclosure(t, projectionDisclosure(aggregateDecision), false, "portainer.EndpointList", projectionModeDefault, []string{"Id", "Name", "Type"}, projectionReasonItemShapeUnexpected)
	assertDisclosure(t, projectionDisclosure(fieldsDecision), true, "portainer.EndpointList", projectionModeFields, []string{"Id", "Name"}, "")
}

func mustProjectionJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func fakeProjectionUpstream(t *testing.T, text string, structured any, isError bool, extra map[string]any) (json.RawMessage, mcp.CallToolResult) {
	t.Helper()
	upstream := map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}, "isError": isError}
	if structured != nil {
		upstream["structuredContent"] = structured
	}
	for key, value := range extra {
		upstream[key] = value
	}
	raw := mustProjectionJSON(t, upstream)
	return json.RawMessage(raw), decodeUpstreamResult(raw)
}

func decodeProjectionList(t *testing.T, text string) []any {
	t.Helper()
	value, err := decodeProjectionJSON([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	payload, ok := projectionPayloadFromValue(value)
	if !ok {
		t.Fatalf("not a list payload: %#v", value)
	}
	return payload.items
}

func assertDecisionFields(t *testing.T, d projectionDecision, want []string) {
	t.Helper()
	if !reflect.DeepEqual(d.Fields, want) {
		t.Fatalf("fields=%#v want=%#v", d.Fields, want)
	}
}

func assertExactKeys(t *testing.T, object map[string]any, keys ...string) {
	t.Helper()
	if len(object) != len(keys) {
		t.Fatalf("keys=%#v want=%v", object, keys)
	}
	for _, key := range keys {
		if _, ok := object[key]; !ok {
			t.Fatalf("missing %q in %#v", key, object)
		}
	}
}

func assertDisclosure(t *testing.T, value map[string]any, applied bool, entry, mode string, fields []string, reason string) {
	t.Helper()
	if value["v"] != 1 || value["applied"] != applied || value["entry"] != entry || value["mode"] != mode {
		t.Fatalf("header=%#v", value)
	}
	got, ok := disclosureFields(value["fields"])
	if !ok || !reflect.DeepEqual(got, fields) {
		t.Fatalf("fields=%#v want=%#v", value["fields"], fields)
	}
	gotReason, hasReason := value["reason"].(string)
	if applied && hasReason || !applied && (!hasReason || gotReason != reason) {
		t.Fatalf("reason=%#v want=%q applied=%v", value["reason"], reason, applied)
	}
}

func disclosureFields(value any) ([]string, bool) {
	if fields, ok := value.([]string); ok {
		return fields, true
	}
	values, ok := value.([]any)
	if !ok {
		return nil, false
	}
	fields := make([]string, len(values))
	for i, value := range values {
		fields[i], ok = value.(string)
		if !ok {
			return nil, false
		}
	}
	return fields, true
}

func assertEqualJSONListString(t *testing.T, value string, want []any) {
	t.Helper()
	if got := decodeProjectionList(t, value); !reflect.DeepEqual(got, want) {
		t.Fatalf("list=%#v want=%#v", got, want)
	}
}

func cloneCallToolResult(t *testing.T, result mcp.CallToolResult) mcp.CallToolResult {
	t.Helper()
	var clone mcp.CallToolResult
	if err := json.Unmarshal(mustProjectionJSON(t, result), &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func TestProjectionSurfaceMultiplexerWholeMemberAndWrapper(t *testing.T) {
	items := []any{map[string]any{"Id": "ep-1", "Name": "primary", "Type": 1, "Hidden": true}}
	listJSON := mustProjectionJSON(t, items)
	forged := map[string]any{"v": 1, "applied": true, "entry": "upstream"}
	structured := map[string]any{"result": string(listJSON), "other": map[string]any{"keep": true}, "mcp_bridge_projection": forged}
	raw, upstream := fakeProjectionUpstream(t, string(listJSON), structured, false, map[string]any{"upstream_only": true})
	decision, err := decideProjection("portainer", "EndpointList", "", map[string]any{}, 0, projectionTestBudget)
	if err != nil {
		t.Fatal(err)
	}
	decision, err = transformProjectionResult(&upstream, decision, "portainer", "EndpointList", "", map[string]any{}, projectionTestBudget)
	if err != nil || !decision.Applied {
		t.Fatalf("decision=%#v err=%v", decision, err)
	}
	whole, err := multiplexerUpstreamResult(raw, upstream, decision)
	if err != nil {
		t.Fatal(err)
	}
	wholeObject := whole.(map[string]any)
	if _, ok := wholeObject["upstream_only"]; ok {
		t.Fatalf("applied whole member kept upstream-only key: %#v", wholeObject)
	}
	content := wholeObject["content"].([]any)
	projected := decodeProjectionList(t, content[0].(map[string]any)["text"].(string))
	assertExactKeys(t, projected[0].(map[string]any), "Id", "Name", "Type")
	result := wholeObject["structuredContent"].(map[string]any)
	assertEqualJSONListString(t, result["result"].(string), projected)
	if !reflect.DeepEqual(result["other"], structured["other"]) || !reflect.DeepEqual(result["mcp_bridge_projection"], rawValue(mustProjectionJSON(t, forged))) {
		t.Fatalf("non-payload wrapper members changed: %#v", result)
	}
	assertDisclosure(t, projectionDisclosure(decision), true, "portainer.EndpointList", projectionModeDefault, []string{"Id", "Name", "Type"}, "")
}

func TestProjectionSurfacePassThroughIsDecodedValueIdentical(t *testing.T) {
	items := []any{map[string]any{"Id": "ep-1", "Name": "primary", "Type": 1, "Hidden": true}}
	listJSON := mustProjectionJSON(t, items)
	raw, upstream := fakeProjectionUpstream(t, string(listJSON), nil, false, map[string]any{"upstream_only": true})
	args := map[string]any{"projection": map[string]any{"full": true}}
	decision, err := decideProjection("portainer", "EndpointList", "", args, 0, projectionTestBudget)
	if err != nil {
		t.Fatal(err)
	}
	decision, err = transformProjectionResult(&upstream, decision, "portainer", "EndpointList", "", args, projectionTestBudget)
	if err != nil || decision.Applied {
		t.Fatalf("full decision %#v err=%v", decision, err)
	}
	got, err := multiplexerUpstreamResult(raw, upstream, decision)
	if err != nil || !reflect.DeepEqual(got, rawValue(raw)) {
		t.Fatalf("full passthrough changed: got %#v err=%v", got, err)
	}
	assertDisclosure(t, projectionDisclosure(decision), false, "portainer.EndpointList", projectionModeFull, []string{}, projectionReasonFullRequested)
}

func TestProjectionSurfaceDirectAbsentAndWrapperCarriers(t *testing.T) {
	endpoints := []any{map[string]any{"Id": "ep-1", "Name": "primary", "Type": 1, "Hidden": true}}
	endpointJSON := mustProjectionJSON(t, endpoints)
	_, endpoint := fakeProjectionUpstream(t, string(endpointJSON), nil, false, nil)
	decision := applyDirectProjection(&endpoint, "portainer", "EndpointList", json.RawMessage(`{}`), projectionTestBudget)
	if !decision.Applied {
		t.Fatalf("endpoint direct projection = %#v", decision)
	}
	assertExactKeys(t, decodeProjectionList(t, endpoint.Content[0].(*mcp.TextContent).Text)[0].(map[string]any), "Id", "Name", "Type")
	assertDisclosure(t, endpoint.StructuredContent.(map[string]any), true, "portainer.EndpointList", projectionModeDefault, []string{"Id", "Name", "Type"}, "")
	if len(endpoint.StructuredContent.(map[string]any)) != 5 {
		t.Fatalf("absent carrier not replaced by only disclosure: %#v", endpoint.StructuredContent)
	}

	containers := []any{map[string]any{"Id": "ctr-1", "Names": []any{"/demo"}, "State": "running", "Image": "alpine", "Hidden": true}}
	containerJSON := mustProjectionJSON(t, containers)
	structured := map[string]any{"result": string(containerJSON), "status": "ok", "nested": map[string]any{"keep": true}}
	_, container := fakeProjectionUpstream(t, string(containerJSON), structured, false, nil)
	decision = applyDirectProjection(&container, "portainer", "docker_proxy", json.RawMessage(`{"path":"/containers/json"}`), projectionTestBudget)
	if !decision.Applied || decision.Entry != "portainer.docker_proxy /containers/json" {
		t.Fatalf("docker direct projection = %#v", decision)
	}
	projected := decodeProjectionList(t, container.Content[0].(*mcp.TextContent).Text)
	assertExactKeys(t, projected[0].(map[string]any), "Id", "Name", "State", "Image")
	if projected[0].(map[string]any)["Name"] != "/demo" {
		t.Fatalf("Name not derived from Names[0]: %#v", projected[0])
	}
	result := container.StructuredContent.(map[string]any)
	assertEqualJSONListString(t, result["result"].(string), projected)
	if result["status"] != "ok" || !reflect.DeepEqual(result["nested"], structured["nested"]) {
		t.Fatalf("non-payload SC changed: %#v", result)
	}
	assertDisclosure(t, result[directProjectionMember].(map[string]any), true, "portainer.docker_proxy /containers/json", projectionModeDefault, []string{"Id", "Name", "State", "Image"}, "")
}

func TestProjectionSurfaceDirectOverridesAndCarrierFailures(t *testing.T) {
	items := []any{map[string]any{"Id": "ep-1", "Name": "primary", "Type": 1}}
	listJSON := mustProjectionJSON(t, items)
	forged := map[string]any{"v": 1, "applied": true, "entry": "forged"}
	collidingSC := map[string]any{"mcp_bridge_projection": forged, "other": "unchanged"}
	_, collision := fakeProjectionUpstream(t, string(listJSON), collidingSC, false, nil)
	before := cloneCallToolResult(t, collision)
	decision := applyDirectProjection(&collision, "portainer", "EndpointList", json.RawMessage(`{}`), projectionTestBudget)
	if decision.Applied || decision.Reason != projectionReasonDisclosureKeyCollision || !reflect.DeepEqual(collision, before) {
		t.Fatalf("collision changed result: %#v %#v", decision, collision)
	}

	_, nonObject := fakeProjectionUpstream(t, string(listJSON), []any{"opaque"}, false, nil)
	before = cloneCallToolResult(t, nonObject)
	decision = applyDirectProjection(&nonObject, "portainer", "EndpointList", json.RawMessage(`{}`), projectionTestBudget)
	if decision.Reason != projectionReasonStructuredContentNotMirrorable || !reflect.DeepEqual(nonObject, before) {
		t.Fatalf("non-object SC was not preserved: %#v %#v", decision, nonObject)
	}

	_, selected := fakeProjectionUpstream(t, string(listJSON), []any{"opaque"}, false, nil)
	before = cloneCallToolResult(t, selected)
	decision = applyDirectProjection(&selected, "portainer", "EndpointList", json.RawMessage(`{"select":"Id"}`), projectionTestBudget)
	if decision.Reason != projectionReasonUpstreamSelectPresent || !reflect.DeepEqual(selected, before) {
		t.Fatalf("select did not dominate carrier shape: %#v", decision)
	}
	_, selectedCollision := fakeProjectionUpstream(t, string(listJSON), collidingSC, false, nil)
	decision = applyDirectProjection(&selectedCollision, "portainer", "EndpointList", json.RawMessage(`{"select":"Id"}`), projectionTestBudget)
	if decision.Reason != projectionReasonUpstreamSelectPresent {
		t.Fatalf("select/collision precedence = %#v", decision)
	}

	_, errorCollision := fakeProjectionUpstream(t, "failed", collidingSC, true, nil)
	before = cloneCallToolResult(t, errorCollision)
	decision = applyDirectProjection(&errorCollision, "portainer", "EndpointList", json.RawMessage(`{"select":"Id"}`), projectionTestBudget)
	if projectionDecisionDisclosed(decision) || !reflect.DeepEqual(errorCollision, before) {
		t.Fatalf("direct error disclosed or changed: %#v", decision)
	}

	_, forgedNeighbor := fakeProjectionUpstream(t, string(listJSON), collidingSC, false, nil)
	before = cloneCallToolResult(t, forgedNeighbor)
	decision = applyDirectProjection(&forgedNeighbor, "other", "EndpointList", json.RawMessage(`{}`), projectionTestBudget)
	if projectionDecisionDisclosed(decision) || !reflect.DeepEqual(forgedNeighbor, before) {
		t.Fatalf("non-matching forged member affected result: %#v", decision)
	}
}

func TestProjectionSurfaceCarrierFailureDisclosureFields(t *testing.T) {
	items := []any{map[string]any{"Id": "ep-1", "Name": "primary", "Type": 1}}
	listJSON := mustProjectionJSON(t, items)
	raw, upstream := fakeProjectionUpstream(t, string(listJSON), []any{"opaque"}, false, nil)
	decision, err := decideProjection("portainer", "EndpointList", "", map[string]any{}, 0, projectionTestBudget)
	if err != nil {
		t.Fatal(err)
	}
	decision, err = transformProjectionResult(&upstream, decision, "portainer", "EndpointList", "", map[string]any{}, projectionTestBudget)
	if err != nil || decision.Reason != projectionReasonStructuredContentNotMirrorable {
		t.Fatalf("multiplexer carrier failure %#v %v", decision, err)
	}
	value, err := multiplexerUpstreamResult(raw, upstream, decision)
	if err != nil || !reflect.DeepEqual(value, rawValue(raw)) {
		t.Fatalf("skip changed upstream member: %#v %v", value, err)
	}
	assertDisclosure(t, projectionDisclosure(decision), false, "portainer.EndpointList", projectionModeDefault, []string{"Id", "Name", "Type"}, projectionReasonStructuredContentNotMirrorable)

	args := map[string]any{"projection": map[string]any{"fields": []any{"Id", "Mounts"}}}
	_, fieldsDecision, err := transformProjection("portainer", "EndpointList", "", args, []any{"not-an-object"}, projectionTestBudget)
	if err != nil || fieldsDecision.Reason != projectionReasonItemShapeUnexpected {
		t.Fatalf("fields carrier failure %#v %v", fieldsDecision, err)
	}
	assertDisclosure(t, projectionDisclosure(fieldsDecision), false, "portainer.EndpointList", projectionModeFields, []string{"Id", "Mounts"}, projectionReasonItemShapeUnexpected)
}

func projectionTestBounds(preview, delivery, aggregate uint64) config.RetainedResultBounds {
	return config.RetainedResultBounds{
		DeliveryCeilingBytes: delivery, PreviewCeilingBytes: preview,
		MaxUpstreamCodeBytes: 512, MaxRetainedResultBytes: 1 << 20,
		MaxBytesPerNamespace: 1 << 20, MaxBytesAggregate: aggregate,
		MaxEntriesPerNamespace: 10, HandleTTL: time.Hour, TombstoneTTL: time.Minute,
	}
}

func projectionTestStore(bounds config.RetainedResultBounds, aggregate uint64) *resultStore {
	bounds.MaxBytesAggregate = aggregate
	return newResultStore(bounds, nil, nil, func() time.Time { return time.Unix(100, 0) }, bytes.NewReader(make([]byte, 1024)))
}

func maximalProjectionFields(t *testing.T, budget int) []string {
	t.Helper()
	for count := 16; count > 0; count-- {
		for length := 64; length > 0; length-- {
			fields := make([]string, count)
			for index := range fields {
				fields[index] = string(rune('A'+index)) + strings.Repeat("x", length-1)
			}
			if len(mustProjectionJSON(t, fields)) <= budget {
				return fields
			}
		}
	}
	return nil
}

func TestProjectionSurfaceMaximalRequestHeadroomAndClamp(t *testing.T) {
	bounds := projectionTestBounds(1536, 4096, 1<<20)
	budget := projectionFieldsBudget(bounds)
	if budget < 512 {
		t.Fatalf("default field budget %d is below its documented floor", budget)
	}
	fields := maximalProjectionFields(t, budget)
	encoded := mustProjectionJSON(t, fields)
	if len(fields) == 0 || len(encoded) > budget {
		t.Fatalf("maximal legal fields %#v use %d bytes of %d", fields, len(encoded), budget)
	}
	item := make(map[string]any, len(fields))
	for _, field := range fields {
		item[field] = "v"
	}
	args := map[string]any{"projection": map[string]any{"fields": stringsAsAny(fields)}}
	projected, decision, err := transformProjection("portainer", "EndpointList", "", args, []any{item}, budget)
	if err != nil || !decision.Applied || len(projected) != 1 {
		t.Fatalf("maximal request %#v %#v %v", projected, decision, err)
	}
	entry := &resultEntry{handle: projectionMaximumBudgetHandle, payload: []byte(strings.Repeat("p", 12000)), projection: &decision, previewCeilingBytes: bounds.PreviewCeilingBytes, deliveryCeilingBytes: bounds.DeliveryCeilingBytes, maxUpstreamCodeBytes: bounds.MaxUpstreamCodeBytes}
	preview := renderResultPreview(entry)
	if preview.Content[0].(*mcp.TextContent).Text == "" {
		t.Fatal("maximal request produced an empty preview")
	}
	if _, ok := preview.StructuredContent.(map[string]any)["projection"]; !ok {
		t.Fatal("preview omitted threaded disclosure")
	}
	page := renderResultPage(entry, 1)
	if page.Content[0].(*mcp.TextContent).Text == "" {
		t.Fatal("maximal request produced an empty page")
	}
	if _, ok := page.StructuredContent.(map[string]any)["projection"]; !ok {
		t.Fatal("page omitted threaded disclosure")
	}
	metadata := previewSizingMetadata(entry)
	measured, err := measureResult(&mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{}}, StructuredContent: metadata})
	if err != nil || uint64(measured)+bounds.MaxUpstreamCodeBytes+projectionRetainedEnvelopeHeadroom > bounds.PreviewCeilingBytes {
		t.Fatalf("headroom failed: measured=%d err=%v", measured, err)
	}

	tight := projectionTestBounds(32, 64, 1<<20)
	if projectionFieldsBudget(tight) != 0 || maxProjectionFieldsBudget(-1) != 0 {
		t.Fatal("field budget did not clamp at zero")
	}
	rejected, err := decideProjection("portainer", "EndpointList", "", map[string]any{"projection": map[string]any{"fields": []any{"Id"}}}, 0, projectionFieldsBudget(tight))
	if err == nil || rejected.Reason != projectionReasonNotSupported {
		t.Fatalf("zero-budget request did not reject at L6: %v", err)
	}
}

func TestProjectionSurfaceDefaultFullAndTerminalDisclosure(t *testing.T) {
	bounds := projectionTestBounds(1536, 4096, 1<<20)
	owner := resultNamespace{publisher: true}
	decision := projectionDecision{Applied: true, Entry: "portainer.EndpointList", Mode: projectionModeDefault, Fields: []string{"Id", "Name", "Type"}}
	large := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.Repeat("x", 8000)}}, StructuredContent: map[string]any{"projection": projectionDisclosure(decision)}}
	store := projectionTestStore(bounds, 1<<20)
	preview := deliverBoundedResult(store, large, owner, publisherResultResolver, "", decision)
	metadata := preview.StructuredContent.(map[string]any)
	if metadata["paging_required"] != true {
		t.Fatalf("expected paged preview: %#v", metadata)
	}
	assertDisclosure(t, metadata["projection"].(map[string]any), true, decision.Entry, decision.Mode, decision.Fields, "")
	entry := store.entries[metadata["handle"].(string)]
	if entry == nil {
		t.Fatal("paged result was not retained")
	}
	page := renderResultPage(entry, 1)
	assertDisclosure(t, page.StructuredContent.(map[string]any)["projection"].(map[string]any), true, decision.Entry, decision.Mode, decision.Fields, "")

	terminal := deliverBoundedResult(projectionTestStore(bounds, 1), large, owner, publisherResultResolver, "", decision)
	assertDisclosure(t, terminal.StructuredContent.(map[string]any)["projection"].(map[string]any), true, decision.Entry, decision.Mode, decision.Fields, "")

	fullDecision := projectionDecision{Mode: projectionModeFull, Fields: []string{}, Reason: projectionReasonFullRequested}
	fullPayload := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.Repeat("full", 2000)}}}
	fullStore := projectionTestStore(bounds, 1<<20)
	fullPreview := deliverBoundedResult(fullStore, fullPayload, owner, publisherResultResolver, "", fullDecision)
	fullMetadata := fullPreview.StructuredContent.(map[string]any)
	assertDisclosure(t, fullMetadata["projection"].(map[string]any), false, "", projectionModeFull, []string{}, projectionReasonFullRequested)
	fullEntry := fullStore.entries[fullMetadata["handle"].(string)]
	if fullEntry == nil || !bytes.Equal(fullEntry.payload, mustProjectionJSON(t, fullPayload)) {
		t.Fatal("full opt-in did not retain the unprojected result")
	}

	forged := map[string]any{"applied": true, "entry": "upstream"}
	collisionItems := []any{map[string]any{"Id": "ep-1", "Name": "primary", "Type": 1, "Hidden": strings.Repeat("h", 8000)}}
	collisionJSON := mustProjectionJSON(t, collisionItems)
	collisionSC := map[string]any{"result": string(collisionJSON), "mcp_bridge_projection": forged}
	_, direct := fakeProjectionUpstream(t, string(collisionJSON), collisionSC, false, nil)
	collision := applyDirectProjection(&direct, "portainer", "EndpointList", json.RawMessage(`{}`), projectionTestBudget)
	if collision.Reason != projectionReasonDisclosureKeyCollision || !reflect.DeepEqual(direct.StructuredContent.(map[string]any)[directProjectionMember], forged) {
		t.Fatalf("collision payload changed: %#v %#v", collision, direct.StructuredContent)
	}
	terminalCollision := deliverBoundedResult(projectionTestStore(bounds, 1), &direct, owner, publisherResultResolver, "", collision)
	assertDisclosure(t, terminalCollision.StructuredContent.(map[string]any)["projection"].(map[string]any), false, "portainer.EndpointList", projectionModeDefault, []string{"Id", "Name", "Type"}, projectionReasonDisclosureKeyCollision)
}

func stringsAsAny(values []string) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}

func TestProjectionSurfaceUnaffectedNeighborSizes(t *testing.T) {
	items := []any{map[string]any{"Id": "x", "Name": "y", "Type": 1, "Hidden": true}}
	listJSON := mustProjectionJSON(t, items)
	structured := map[string]any{"mcp_bridge_projection": map[string]any{"applied": true}}
	raw, multiplexer := fakeProjectionUpstream(t, string(listJSON), structured, false, nil)
	before, _ := json.Marshal(rawValue(raw))
	got, err := multiplexerUpstreamResult(raw, multiplexer, projectionDecision{})
	if err != nil || !reflect.DeepEqual(got, rawValue(raw)) {
		t.Fatalf("multiplexer neighbor changed: %#v %v", got, err)
	}
	after, _ := json.Marshal(got)
	if len(before) != len(after) {
		t.Fatalf("multiplexer neighbor size %d != %d", len(before), len(after))
	}

	_, direct := fakeProjectionUpstream(t, string(listJSON), structured, false, nil)
	directBefore := mustProjectionJSON(t, direct)
	directOriginal := cloneCallToolResult(t, direct)
	decision := applyDirectProjection(&direct, "portainer", "docker_proxy", json.RawMessage(`{"path":"/images/json"}`), projectionTestBudget)
	if projectionDecisionDisclosed(decision) || !reflect.DeepEqual(direct, directOriginal) {
		t.Fatalf("direct neighbor changed: %#v", decision)
	}
	directAfter := mustProjectionJSON(t, direct)
	if len(directBefore) != len(directAfter) {
		t.Fatalf("direct neighbor size %d != %d", len(directBefore), len(directAfter))
	}
}

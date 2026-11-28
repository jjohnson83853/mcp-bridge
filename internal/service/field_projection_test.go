package service

import (
	"reflect"
	"testing"
)

const projectionFieldsBudgetDefault = 512

func fieldProjectionForTest(server, tool, path string, args map[string]any, items []any) ([]any, projectionDecision, error) {
	return transformProjection(server, tool, path, args, items, projectionFieldsBudgetDefault)
}

func TestFieldProjectionEndpointDefaults(t *testing.T) {
	items := []any{
		map[string]any{
			"Id":               7,
			"Name":             "endpoint-a",
			"Type":             1,
			"Status":           1,
			"AzureCredentials": map[string]any{"secret": "excluded"},
		},
	}

	got, decision, err := fieldProjectionForTest("portainer", "EndpointList", "", nil, items)
	if err != nil {
		t.Fatalf("transformProjection() error = %v", err)
	}
	if !decision.Applied || decision.Entry != "portainer.EndpointList" || decision.Mode != "default" || !reflect.DeepEqual(decision.Fields, []string{"Id", "Name", "Type"}) {
		t.Fatalf("decision = %#v", decision)
	}
	want := []any{map[string]any{"Id": 7, "Name": "endpoint-a", "Type": 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("projected items = %#v, want %#v", got, want)
	}
}

func TestFieldProjectionContainerDefaultsAndNames(t *testing.T) {
	items := []any{
		map[string]any{"Id": "one", "Names": []any{"/first", "/alias"}, "State": "running", "Image": "alpine", "Labels": map[string]any{"x": "y"}},
		map[string]any{"Id": "two", "State": "stopped", "Image": "busybox", "Name": "must-not-survive"},
		map[string]any{"Id": "three", "Names": []any{}, "State": "created", "Image": "nginx"},
	}

	got, decision, err := fieldProjectionForTest("portainer", "docker_proxy", "/containers/json", nil, items)
	if err != nil {
		t.Fatalf("transformProjection() error = %v", err)
	}
	if !decision.Applied || decision.Entry != "portainer.docker_proxy /containers/json" || decision.Mode != "default" || !reflect.DeepEqual(decision.Fields, []string{"Id", "Name", "State", "Image"}) {
		t.Fatalf("decision = %#v", decision)
	}
	want := []any{
		map[string]any{"Id": "one", "Name": "/first", "State": "running", "Image": "alpine"},
		map[string]any{"Id": "two", "State": "stopped", "Image": "busybox"},
		map[string]any{"Id": "three", "State": "created", "Image": "nginx"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("projected items = %#v, want %#v", got, want)
	}
}

func TestFieldProjectionAllowlistIdentity(t *testing.T) {
	items := []any{map[string]any{"Id": 1, "Name": "unchanged", "Type": 2, "extra": true}}
	cases := []struct {
		name   string
		server string
		tool   string
		path   string
	}{
		{name: "pluralized endpoint", server: "portainer", tool: "EndpointLists"},
		{name: "other server", server: "other", tool: "EndpointList"},
		{name: "other docker path", server: "portainer", tool: "docker_proxy", path: "/images/json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, decision, err := fieldProjectionForTest(tc.server, tc.tool, tc.path, nil, items)
			if err != nil {
				t.Fatalf("transformProjection() error = %v", err)
			}
			if !reflect.DeepEqual(got, items) {
				t.Fatalf("unallowlisted items changed: got %#v, want %#v", got, items)
			}
			if decision.Applied || decision.Entry != "" || decision.Mode != "" || len(decision.Fields) != 0 || decision.Reason != "" {
				t.Fatalf("decision = %#v, want no-match pass-through", decision)
			}
		})
	}
}

func TestFieldProjectionFieldsSelection(t *testing.T) {
	items := []any{
		map[string]any{"Id": 7, "Mounts": []any{"/data"}, "Names": []any{"raw-name"}, "State": "running"},
		map[string]any{"Id": 8, "State": "stopped"},
	}
	args := map[string]any{"fields": []any{"Id", "Mounts"}}

	got, decision, err := fieldProjectionForTest("portainer", "docker_proxy", "/containers/json", args, items)
	if err != nil {
		t.Fatalf("transformProjection() error = %v", err)
	}
	if !decision.Applied || decision.Mode != "fields" || !reflect.DeepEqual(decision.Fields, []string{"Id", "Mounts"}) {
		t.Fatalf("decision = %#v", decision)
	}
	want := []any{map[string]any{"Id": 7, "Mounts": []any{"/data"}}, map[string]any{"Id": 8}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("projected items = %#v, want %#v", got, want)
	}
}

func TestFieldProjectionFieldsRejectUnknownOnlyWhenObserved(t *testing.T) {
	args := map[string]any{"fields": []any{"Mounts"}}
	got, decision, err := fieldProjectionForTest("portainer", "docker_proxy", "/containers/json", args, []any{map[string]any{"Id": 1}})
	if err == nil || got != nil {
		t.Fatalf("transformProjection() = (%#v, %#v, %v), want unknown-field rejection", got, decision, err)
	}
	if decision.Applied || decision.Mode != "fields" || decision.Reason != "unknown_field_selection" || !reflect.DeepEqual(decision.Fields, []string{"Mounts"}) {
		t.Fatalf("decision = %#v", decision)
	}

	got, decision, err = fieldProjectionForTest("portainer", "docker_proxy", "/containers/json", args, []any{})
	if err != nil || !decision.Applied || decision.Mode != "fields" || !reflect.DeepEqual(decision.Fields, []string{"Mounts"}) || len(got) != 0 {
		t.Fatalf("empty-list fields projection = (%#v, %#v, %v), want accepted empty result", got, decision, err)
	}
}

func TestFieldProjectionRequestValidation(t *testing.T) {
	cases := []struct {
		name   string
		args   map[string]any
		reason string
		mode   string
		fields []string
	}{
		{name: "empty fields", args: map[string]any{"fields": []any{}}, reason: "malformed_request", mode: "fields"},
		{name: "full true with fields", args: map[string]any{"full": true, "fields": []any{"Id"}}, reason: "malformed_request", mode: "full"},
		{name: "select conflict", args: map[string]any{"fields": []any{"Id"}, "select": "x"}, reason: "select_conflict", mode: "fields", fields: []string{"Id"}},
		{name: "full false fields applies", args: map[string]any{"full": false, "fields": []any{"Id"}}, mode: "fields", fields: []string{"Id"}},
		{name: "full false fields with select", args: map[string]any{"full": false, "fields": []any{"Id"}, "select": "x"}, reason: "select_conflict", mode: "fields", fields: []string{"Id"}},
		{name: "full alone", args: map[string]any{"full": true}, reason: "full_requested", mode: "full", fields: []string{}},
	}
	items := []any{map[string]any{"Id": 1}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, decision, err := fieldProjectionForTest("portainer", "EndpointList", "", tc.args, items)
			if tc.reason == "" {
				if err != nil || !decision.Applied {
					t.Fatalf("transformProjection() = (%#v, %#v, %v), want applied", got, decision, err)
				}
			} else if tc.reason == "full_requested" {
				if err != nil || decision.Applied || decision.Reason != tc.reason {
					t.Fatalf("transformProjection() = (%#v, %#v, %v), want full-request skip", got, decision, err)
				}
			} else if err == nil || decision.Applied || decision.Reason != tc.reason {
				t.Fatalf("transformProjection() = (%#v, %#v, %v), want rejection reason %q", got, decision, err, tc.reason)
			}
			if decision.Mode != tc.mode || !reflect.DeepEqual(decision.Fields, tc.fields) {
				t.Fatalf("decision mode/fields = %q/%#v, want %q/%#v", decision.Mode, decision.Fields, tc.mode, tc.fields)
			}
		})
	}
}

func TestFieldProjectionBoundsAndTotalLadder(t *testing.T) {
	tooLong := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	tooMany := make([]any, 17)
	for i := range tooMany {
		tooMany[i] = "f"
	}
	cases := []struct {
		name   string
		server string
		tool   string
		path   string
		args   map[string]any
		want   string
	}{
		{name: "L1 non-object projection", server: "portainer", tool: "EndpointList", args: map[string]any{"projection": "bad"}, want: "malformed_request"},
		{name: "L2 before select", server: "portainer", tool: "EndpointList", args: map[string]any{"full": true, "fields": []any{}, "select": "x"}, want: "malformed_request"},
		{name: "L3 before empty fields bounds", server: "portainer", tool: "EndpointList", args: map[string]any{"fields": []any{}, "select": "x"}, want: "select_conflict"},
		{name: "duplicate fields", server: "portainer", tool: "EndpointList", args: map[string]any{"fields": []any{"Id", "Id"}}, want: "malformed_request"},
		{name: "too many fields", server: "portainer", tool: "EndpointList", args: map[string]any{"fields": tooMany}, want: "malformed_request"},
		{name: "field too long", server: "portainer", tool: "EndpointList", args: map[string]any{"fields": []any{tooLong}}, want: "malformed_request"},
		{name: "invalid field character", server: "portainer", tool: "EndpointList", args: map[string]any{"fields": []any{"Id.Name"}}, want: "malformed_request"},
		{name: "L4 before unsupported operation", server: "other", tool: "EndpointList", args: map[string]any{"fields": []any{"Id", "Id"}}, want: "malformed_request"},
		{name: "L5 unsupported operation", server: "other", tool: "EndpointList", args: map[string]any{"fields": []any{"Id"}}, want: "not_supported"},
		{name: "L5 wrong docker path", server: "portainer", tool: "docker_proxy", path: "/images/json", args: map[string]any{"fields": []any{"Id"}}, want: "not_supported"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := decideProjection(tc.server, tc.tool, tc.path, tc.args, 1, projectionFieldsBudgetDefault)
			if err == nil || decision.Applied || decision.Reason != tc.want {
				t.Fatalf("decideProjection() = (%#v, %v), want exactly %q", decision, err, tc.want)
			}
		})
	}
}

func TestFieldProjectionSafeDegradation(t *testing.T) {
	cases := []struct {
		name  string
		items []any
	}{
		{name: "non-object item", items: []any{map[string]any{"Id": 1}, "changed-shape"}},
		{name: "all default fields missing", items: []any{map[string]any{"unexpected": 1}, map[string]any{"other": 2}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, decision, err := fieldProjectionForTest("portainer", "EndpointList", "", nil, tc.items)
			if err != nil {
				t.Fatalf("transformProjection() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.items) {
				t.Fatalf("skip changed upstream items: got %#v, want %#v", got, tc.items)
			}
			if decision.Applied || decision.Mode != "default" || decision.Reason != "item_shape_unexpected" || !reflect.DeepEqual(decision.Fields, []string{"Id", "Name", "Type"}) {
				t.Fatalf("decision = %#v", decision)
			}
		})
	}
}

func TestFieldProjectionDefaultEmptyAndPartialLists(t *testing.T) {
	got, decision, err := fieldProjectionForTest("portainer", "EndpointList", "", nil, []any{})
	if err != nil || !decision.Applied || decision.Mode != "default" || decision.Reason != "" || !reflect.DeepEqual(decision.Fields, []string{"Id", "Name", "Type"}) || len(got) != 0 {
		t.Fatalf("empty default projection = (%#v, %#v, %v)", got, decision, err)
	}

	items := []any{map[string]any{"Id": 1}, map[string]any{"Type": 2, "extra": true}}
	got, decision, err = fieldProjectionForTest("portainer", "EndpointList", "", nil, items)
	if err != nil || !decision.Applied || decision.Reason != "" {
		t.Fatalf("partial default projection = (%#v, %#v, %v), want applied", got, decision, err)
	}
	want := []any{map[string]any{"Id": 1}, map[string]any{"Type": 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partial projected items = %#v, want %#v", got, want)
	}
}

func TestFieldProjectionRuntimeFieldsBudget(t *testing.T) {
	args := map[string]any{"fields": []any{"Id"}}
	cases := []struct {
		name    string
		budget  int
		applied bool
	}{
		{name: "documented default floor", budget: projectionFieldsBudgetDefault, applied: true},
		{name: "exact encoded width", budget: 6, applied: true},
		{name: "one byte too small", budget: 5},
		{name: "zero clamps to zero", budget: 0},
		{name: "negative clamps to zero", budget: -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := decideProjection("portainer", "EndpointList", "", args, 1, tc.budget)
			if tc.applied {
				if err != nil || !decision.Applied || decision.Reason != "" {
					t.Fatalf("decideProjection() = (%#v, %v), want applied", decision, err)
				}
				return
			}
			if err == nil || decision.Applied || decision.Reason != "not_supported" {
				t.Fatalf("decideProjection() = (%#v, %v), want L6 not_supported", decision, err)
			}
		})
	}

	decision, err := decideProjection("portainer", "EndpointList", "", map[string]any{"fields": []any{}}, 1, 0)
	if err == nil || decision.Reason != "malformed_request" {
		t.Fatalf("static bounds with zero budget = (%#v, %v), want L4 malformed_request", decision, err)
	}
}

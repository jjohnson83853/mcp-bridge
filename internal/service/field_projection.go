package service

import (
	"encoding/json"
	"errors"
)

type projectionEntry struct {
	Server, Tool, Path string
	DefaultFields      []string
}

var projectionAllowlist = []projectionEntry{
	{Server: "portainer", Tool: "EndpointList", DefaultFields: []string{"Id", "Name", "Type"}},
	{Server: "portainer", Tool: "docker_proxy", Path: "/containers/json", DefaultFields: []string{"Id", "Name", "State", "Image"}},
}

type projectionDecision struct {
	Applied bool
	Entry   string
	Mode    string
	Fields  []string
	Reason  string
}

const (
	projectionReasonFullRequested                  = "full_requested"
	projectionReasonUpstreamSelectPresent          = "upstream_select_present"
	projectionReasonUpstreamError                  = "upstream_error"
	projectionReasonPayloadUnparseable             = "payload_unparseable"
	projectionReasonPayloadShapeUnexpected         = "payload_shape_unexpected"
	projectionReasonItemShapeUnexpected            = "item_shape_unexpected"
	projectionReasonStructuredContentNotMirrorable = "structured_content_not_mirrorable"
	projectionReasonDisclosureKeyCollision         = "disclosure_key_collision"
	projectionReasonMalformedRequest               = "malformed_request"
	projectionReasonNotSupported                   = "not_supported"
	projectionReasonSelectConflict                 = "select_conflict"
	projectionReasonUnknownFieldSelection          = "unknown_field_selection"

	projectionModeDefault = "default"
	projectionModeFields  = "fields"
	projectionModeFull    = "full"
)

var (
	errProjectionMalformedRequest = errors.New("malformed projection request")
	errProjectionNotSupported     = errors.New("projection fields are not supported for this operation or budget")
	errProjectionSelectConflict   = errors.New("projection fields conflict with upstream select")
	errProjectionUnknownField     = errors.New("projection contains a field not present in the upstream list")
)

func decideProjection(server, tool, path string, args map[string]any, itemCount, fieldsBudget int) (projectionDecision, error) {
	entry, matched := matchProjectionEntry(server, tool, path)
	entryID := ""
	var defaults []string
	if matched {
		entryID, defaults = projectionEntryID(entry), entry.DefaultFields
	}
	options, badProjection, selectPresent := projectionOptions(args)
	if badProjection {
		return rejectedProjection(entryID, "", nil, projectionReasonMalformedRequest)
	}
	_ = itemCount // The transformer uses item cardinality for payload-relative validation.

	full, fullPresent := false, false
	fields, fieldsPresent := []string(nil), false
	if options != nil {
		if raw, present := options["full"]; present {
			fullPresent = true
			var ok bool
			full, ok = raw.(bool)
			if !ok {
				mode := ""
				var attempted []string
				if rawFields, hasFields := options["fields"]; hasFields {
					mode = projectionModeFields
					attempted, _ = parseProjectionFields(rawFields)
				}
				return rejectedProjection(entryID, mode, attempted, projectionReasonMalformedRequest)
			}
		}
		if raw, present := options["fields"]; present {
			fieldsPresent = true
			var ok bool
			fields, ok = parseProjectionFields(raw)
			if !ok {
				mode := projectionModeFields
				if fullPresent && full {
					mode = projectionModeFull
				}
				return rejectedProjection(entryID, mode, nil, projectionReasonMalformedRequest)
			}
		}
	}

	// L1 was checked above. Preserve the one total, first-match-wins order.
	if fullPresent && full && fieldsPresent {
		return rejectedProjection(entryID, projectionModeFull, nil, projectionReasonMalformedRequest)
	}
	if fieldsPresent && selectPresent {
		return rejectedProjection(entryID, projectionModeFields, fields, projectionReasonSelectConflict)
	}
	if fieldsPresent && !validProjectionFields(fields) {
		return rejectedProjection(entryID, projectionModeFields, fields, projectionReasonMalformedRequest)
	}
	if fieldsPresent && !matched {
		return rejectedProjection(entryID, projectionModeFields, fields, projectionReasonNotSupported)
	}
	if fieldsPresent {
		encoded, err := json.Marshal(fields)
		if err != nil || len(encoded) > maxProjectionFieldsBudget(fieldsBudget) {
			return rejectedProjection(entryID, projectionModeFields, fields, projectionReasonNotSupported)
		}
		return projectionDecision{Applied: true, Entry: entryID, Mode: projectionModeFields, Fields: cloneProjectionFields(fields)}, nil
	}
	if fullPresent && full {
		return projectionDecision{Entry: entryID, Mode: projectionModeFull, Fields: []string{}, Reason: projectionReasonFullRequested}, nil
	}
	if !matched {
		return projectionDecision{Fields: []string{}}, nil
	}
	if selectPresent {
		return projectionDecision{Entry: entryID, Mode: projectionModeDefault, Fields: cloneProjectionFields(defaults), Reason: projectionReasonUpstreamSelectPresent}, nil
	}
	return projectionDecision{Applied: true, Entry: entryID, Mode: projectionModeDefault, Fields: cloneProjectionFields(defaults)}, nil
}

func transformProjection(server, tool, path string, args map[string]any, items []any, fieldsBudget int) ([]any, projectionDecision, error) {
	decision, err := decideProjection(server, tool, path, args, len(items), fieldsBudget)
	if err != nil {
		return nil, decision, err
	}
	if !decision.Applied {
		return items, decision, nil
	}
	entry, matched := matchProjectionEntry(server, tool, path)
	if !matched {
		return items, decision, nil
	}

	allDefaultFieldsMissing := len(items) > 0 && decision.Mode == projectionModeDefault
	var fieldsSeen map[string]bool
	if decision.Mode == projectionModeFields {
		fieldsSeen = make(map[string]bool, len(decision.Fields))
		for _, field := range decision.Fields {
			fieldsSeen[field] = false
		}
	}
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			decision.Applied, decision.Reason = false, projectionReasonItemShapeUnexpected
			return items, decision, nil
		}
		if decision.Mode == projectionModeDefault && entry.Tool == "docker_proxy" {
			if _, present := item["Names"]; present {
				if _, _, valid := containerName(item); !valid {
					decision.Applied, decision.Reason = false, projectionReasonPayloadShapeUnexpected
					return items, decision, nil
				}
			}
		}
		if decision.Mode == projectionModeFields {
			for field := range fieldsSeen {
				if _, present := item[field]; present {
					fieldsSeen[field] = true
				}
			}
		} else if allDefaultFieldsMissing && hasProjectedDefaultField(entry, item) {
			allDefaultFieldsMissing = false
		}
	}
	if decision.Mode == projectionModeFields && len(items) > 0 {
		for _, seen := range fieldsSeen {
			if !seen {
				decision.Applied, decision.Reason = false, projectionReasonUnknownFieldSelection
				return nil, decision, errProjectionUnknownField
			}
		}
	} else if allDefaultFieldsMissing {
		decision.Applied, decision.Reason = false, projectionReasonItemShapeUnexpected
		return items, decision, nil
	}

	projected := make([]any, len(items))
	for i, raw := range items {
		item := raw.(map[string]any)
		if decision.Mode == projectionModeFields {
			projected[i] = projectRawFields(item, decision.Fields)
		} else {
			projected[i] = projectDefaultFields(entry, item)
		}
	}
	return projected, decision, nil
}

func projectionOptions(args map[string]any) (map[string]any, bool, bool) {
	if args == nil {
		return nil, false, false
	}
	options := args
	nested := false
	if raw, present := args["projection"]; present {
		nested = true
		if raw == nil {
			options = nil
		} else {
			var ok bool
			options, ok = raw.(map[string]any)
			if !ok {
				return nil, true, projectionSelectPresent(args["select"])
			}
		}
	}
	selectPresent := projectionSelectPresent(args["select"])
	if nested && options != nil {
		if raw, present := options["select"]; present && projectionSelectPresent(raw) {
			selectPresent = true
		}
	}
	return options, false, selectPresent
}

func projectionSelectPresent(value any) bool {
	if value == nil {
		return false
	}
	if selector, ok := value.(string); ok {
		return selector != ""
	}
	return true
}

func parseProjectionFields(value any) ([]string, bool) {
	switch fields := value.(type) {
	case []string:
		return cloneProjectionFields(fields), true
	case []any:
		parsed := make([]string, len(fields))
		for i, field := range fields {
			value, ok := field.(string)
			if !ok {
				return nil, false
			}
			parsed[i] = value
		}
		return parsed, true
	default:
		return nil, false
	}
}

func validProjectionFields(fields []string) bool {
	if len(fields) == 0 || len(fields) > 16 {
		return false
	}
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if len(field) == 0 || len(field) > 64 {
			return false
		}
		for i := 0; i < len(field); i++ {
			c := field[i]
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
				return false
			}
		}
		if _, exists := seen[field]; exists {
			return false
		}
		seen[field] = struct{}{}
	}
	return true
}

func maxProjectionFieldsBudget(budget int) int {
	if budget < 0 {
		return 0
	}
	return budget
}

func rejectedProjection(entry, mode string, fields []string, reason string) (projectionDecision, error) {
	decision := projectionDecision{Entry: entry, Mode: mode, Fields: cloneProjectionFields(fields), Reason: reason}
	return decision, projectionError(reason)
}

func projectionError(reason string) error {
	switch reason {
	case projectionReasonMalformedRequest:
		return errProjectionMalformedRequest
	case projectionReasonSelectConflict:
		return errProjectionSelectConflict
	case projectionReasonNotSupported:
		return errProjectionNotSupported
	case projectionReasonUnknownFieldSelection:
		return errProjectionUnknownField
	default:
		return nil
	}
}

func matchProjectionEntry(server, tool, path string) (projectionEntry, bool) {
	for _, entry := range projectionAllowlist {
		if entry.Server == server && entry.Tool == tool && entry.Path == path {
			return entry, true
		}
	}
	return projectionEntry{}, false
}

func projectionEntryID(entry projectionEntry) string {
	if entry.Tool == "docker_proxy" {
		return "portainer.docker_proxy /containers/json"
	}
	return entry.Server + "." + entry.Tool
}

func cloneProjectionFields(fields []string) []string {
	if fields == nil {
		return nil
	}
	return append([]string(nil), fields...)
}

func hasProjectedDefaultField(entry projectionEntry, item map[string]any) bool {
	for _, field := range entry.DefaultFields {
		if entry.Tool == "docker_proxy" && field == "Name" {
			if _, present, _ := containerName(item); present {
				return true
			}
			continue
		}
		if _, present := item[field]; present {
			return true
		}
	}
	return false
}

func projectDefaultFields(entry projectionEntry, item map[string]any) map[string]any {
	projected := make(map[string]any, len(entry.DefaultFields))
	for _, field := range entry.DefaultFields {
		if entry.Tool == "docker_proxy" && field == "Name" {
			if name, present, _ := containerName(item); present {
				projected[field] = name
			}
			continue
		}
		if value, present := item[field]; present {
			projected[field] = value
		}
	}
	return projected
}

func projectRawFields(item map[string]any, fields []string) map[string]any {
	projected := make(map[string]any, len(fields))
	for _, field := range fields {
		if value, present := item[field]; present {
			projected[field] = value
		}
	}
	return projected
}

func containerName(item map[string]any) (any, bool, bool) {
	raw, present := item["Names"]
	if !present {
		return nil, false, true
	}
	switch names := raw.(type) {
	case []any:
		if len(names) == 0 {
			return nil, false, true
		}
		return names[0], true, true
	case []string:
		if len(names) == 0 {
			return nil, false, true
		}
		return names[0], true, true
	default:
		return nil, false, false
	}
}

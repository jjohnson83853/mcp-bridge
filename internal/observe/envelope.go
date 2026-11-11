package observe

import (
	"math"
	"net/url"
	"strconv"
	"strings"
)

const SchemaVersion = 1

// Envelope is the application-owned version-one JSON record shape.
type Envelope struct {
	SchemaVersion  int           `json:"schema_version"`
	Level          string        `json:"level"`
	Service        string        `json:"service"`
	Event          string        `json:"event"`
	Message        string        `json:"message"`
	Operation      string        `json:"operation"`
	Outcome        string        `json:"outcome"`
	Error          *ErrorInfo    `json:"error,omitempty"`
	Config         *ConfigInfo   `json:"config,omitempty"`
	Resource       *ResourceInfo `json:"resource,omitempty"`
	RequestID      string        `json:"request_id,omitempty"`
	Generation     uint64        `json:"generation,omitempty"`
	DurationMS     int64         `json:"duration_ms,omitempty"`
	Server         string        `json:"server,omitempty"`
	ConfigRevision string        `json:"config_revision,omitempty"`
	Trigger        string        `json:"trigger,omitempty"`
}

// ErrorInfo contains the stable, queryable classification of a failed event.
type ErrorInfo struct {
	Code        string `json:"code"`
	Kind        string `json:"kind"`
	Retryable   bool   `json:"retryable"`
	Remediation string `json:"remediation"`
}

// ConfigInfo contains safe configuration attribution, never document values.
type ConfigInfo struct {
	Path                  string             `json:"path,omitempty"`
	PropertyPath          string             `json:"property_path,omitempty"`
	Line                  int                `json:"line,omitempty"`
	Column                int                `json:"column,omitempty"`
	Server                string             `json:"server,omitempty"`
	PinServer             string             `json:"pin_server,omitempty"`
	PinTool               string             `json:"pin_tool,omitempty"`
	RefKind               string             `json:"ref_kind,omitempty"`
	RefName               string             `json:"ref_name,omitempty"`
	Shape                 string             `json:"shape,omitempty"`
	Related               *RelatedConfigInfo `json:"related,omitempty"`
	ActualCount           int                `json:"actual_count,omitempty"`
	MaxCount              int                `json:"max_count,omitempty"`
	FirstExcessIndex      int                `json:"first_excess_index,omitempty"`
	DiagnosticsIncomplete bool               `json:"diagnostics_incomplete"`
	RemainingStages       string             `json:"remaining_stages,omitempty"`
}

// RelatedConfigInfo locates the earlier or counterpart configuration value.
type RelatedConfigInfo struct {
	PropertyPath string `json:"property_path,omitempty"`
	Line         int    `json:"line,omitempty"`
	Column       int    `json:"column,omitempty"`
	RefKind      string `json:"ref_kind,omitempty"`
	RefName      string `json:"ref_name,omitempty"`
	Server       string `json:"server,omitempty"`
}

// ResourceInfo carries only safe resource identifiers.
type ResourceInfo struct {
	Path     string `json:"path,omitempty"`
	Listener string `json:"listener,omitempty"`
	Address  string `json:"address,omitempty"`
}

// NewEnvelope constructs a record with the pinned service and schema identity.
func NewEnvelope(level, event, message, operation, outcome string) Envelope {
	return Envelope{
		SchemaVersion: SchemaVersion,
		Level:         level,
		Service:       "mcp-bridge",
		Event:         event,
		Message:       message,
		Operation:     operation,
		Outcome:       outcome,
	}
}

// RuntimeErrorInfo maps the closed runtime reason vocabulary to its stable
// queryable error classification. Unknown inputs collapse to the safe fallback.
func RuntimeErrorInfo(reason string) *ErrorInfo {
	if reason == "none" {
		return nil
	}
	info, ok := runtimeErrorInfo(reason)
	if !ok {
		info, _ = runtimeErrorInfo("unknown")
	}
	return &info
}

func runtimeErrorInfo(reason string) (ErrorInfo, bool) {
	info := ErrorInfo{Code: reason}
	switch reason {
	case "none":
		return ErrorInfo{}, false
	case "host_rejected", "origin_rejected", "client_auth_failed", "session_auth_failed",
		"request_too_large", "route_not_found", "unsupported_capability", "unsupported_schema",
		"invalid_arguments", "unknown_tool", "malformed_request", "invalid_request",
		"method_not_found", "client_disconnected":
		info.Kind, info.Remediation = "usage", "correct the request and retry"
	case "request_cancelled":
		info.Kind, info.Retryable, info.Remediation = "usage", true, "retry the request"
	case "session_capacity":
		info.Kind, info.Retryable, info.Remediation = "resource", true, "retry when capacity is available"
	case "backend_unavailable", "backend_auth_failed", "backend_busy", "deadline_exceeded",
		"upstream_protocol_error", "catalog_changed", "upstream_unavailable", "discovery_failed":
		info.Kind, info.Retryable, info.Remediation = "dependency", true, "verify the upstream service and retry"
	case "refresh_failed":
		info.Kind, info.Retryable, info.Remediation = "dependency", true, "verify the upstream dependencies and retry"
	case "tool_reported_error":
		info.Kind, info.Remediation = "dependency", "correct the upstream tool behavior"
	case "client_missing":
		info.Kind, info.Remediation = "configuration", "correct the server configuration or credentials"
	case "invalid_config", "pin_rejected":
		info.Kind, info.Remediation = "configuration", "correct the configuration"
	case "internal_error", "correlation_unavailable", "unknown":
		info.Kind, info.Remediation = "internal", "inspect the bridge logs and configuration"
	default:
		return ErrorInfo{}, false
	}
	return info, true
}

func validErrorInfo(info *ErrorInfo) bool {
	if info == nil {
		return false
	}
	expected, ok := runtimeErrorInfo(info.Code)
	return ok && *info == expected
}

func buildEnvelope(entry Entry) (map[string]any, bool) {
	if !validLevel(entry.Level) || entry.Event == "" || entry.Operation == "" ||
		!validEnvelopeOutcome(entry.Outcome) || entry.Duration < 0 {
		return nil, false
	}
	message := eventMessage(entry.Event)
	if message == "" {
		return nil, false
	}
	if entry.Error != nil && (!validErrorInfo(entry.Error) || !errorOutcome(entry.Outcome)) {
		return nil, false
	}

	record := map[string]any{
		"schema_version": SchemaVersion,
		"level":          entry.Level,
		"service":        "mcp-bridge",
		"event":          entry.Event,
		"message":        message,
		"operation":      entry.Operation,
		"outcome":        entry.Outcome,
	}
	if entry.Error != nil {
		record["error"] = entry.Error
	}
	if validHexID(entry.RequestID) {
		record["request_id"] = entry.RequestID
	}
	if entry.Generation != 0 {
		record["generation"] = entry.Generation
	}
	if entry.Duration != 0 || entry.Event == "upstream_ephemeral_handshake" {
		record["duration_ms"] = entry.Duration.Milliseconds()
	}
	if safeServer(entry.Server) {
		record["server"] = entry.Server
	}
	if !addEnvelopeFields(record, entry.Fields) {
		return nil, false
	}
	return record, true
}

func errorOutcome(outcome string) bool {
	switch outcome {
	case "error", "failure", "timeout", "denied", "busy", "degraded":
		return true
	default:
		return false
	}
}
func validEnvelopeOutcome(outcome string) bool {
	for _, candidate := range outcomeVocabulary {
		if outcome == candidate {
			return true
		}
	}
	return false
}

// SanitizeEndpoint keeps only the configured scheme, authority, and path.
func SanitizeEndpoint(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.User = nil
	parsed.Opaque = ""
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""
	parsed.RawPath = ""
	return parsed.String()
}

func validFailureReason(reason string) bool {
	switch reason {
	case "upstream_unavailable", "discovery_failed", "client_missing",
		"authentication_rejected_during_initialize", "authentication_rejected_during_tools_list",
		"dns_lookup_failed", "connect_failed", "tls_handshake_failed",
		"initialize_failed", "tools_list_failed", "upstream_http_error":
		return true
	}
	for _, prefix := range []string{
		"timeout_context_deadline_exceeded_", "timeout_network_timeout_",
	} {
		if !strings.HasPrefix(reason, prefix) || !strings.HasSuffix(reason, "ms") {
			continue
		}
		milliseconds := strings.TrimSuffix(strings.TrimPrefix(reason, prefix), "ms")
		if len(milliseconds) == 0 || len(milliseconds) > 8 {
			return false
		}
		value, err := strconv.ParseUint(milliseconds, 10, 32)
		return err == nil && value > 0 && value <= 86_400_000
	}
	return false
}

func addEnvelopeFields(record map[string]any, fields map[string]any) bool {
	for key, value := range fields {
		switch key {
		case "count", "slots", "catalog_tools", "upstream_content_count", "final_content_count":
			if !numericField(value) {
				return false
			}
			record[key] = value
		case "next_attempt_ms", "retained_age_ms", "retained_generation", "candidate_servers",
			"candidate_tools", "servers_discovered", "tools_discovered", "tools_published",
			"pins_resolved", "pins_total":
			if !nonNegativeIntegerField(value) {
				return false
			}
			record[key] = value
		case "chunk_count":
			count, ok := value.(int)
			index, indexOK := fields["chunk_index"].(int)
			if !ok || count <= 0 || !indexOK || index < 0 || index >= count {
				return false
			}
			record[key] = count
		case "chunk_index":
			index, ok := value.(int)
			count, countOK := fields["chunk_count"].(int)
			if !ok || index < 0 || !countOK || count <= 0 || index >= count {
				return false
			}
			record[key] = index
		case "discovered_tools":
			names, ok := value.([]string)
			if !ok || len(names) > 64 {
				return false
			}
			for _, name := range names {
				if !validDiscoveredToolName(name) {
					return false
				}
			}
			record[key] = names
		case "upstream_text_presence", "upstream_structured_presence", "final_text_presence", "final_structured_presence":
			presence, ok := value.(string)
			if !ok || (presence != "unknown" && presence != "present" && presence != "absent") {
				return false
			}
			record[key] = presence
		case "upstream_shape", "final_shape":
			shape, ok := value.(map[string]any)
			if !ok {
				return false
			}
			record[key] = cloneShape(shape)
		case "operation_id", "backend_call_id":
			id, ok := value.(string)
			if !ok || !validHexID(id) {
				return false
			}
			record[key] = id
		case "mcp_method", "mcp_message_kind", "tool":
			text, ok := value.(string)
			if !ok {
				return false
			}
			record[key] = text
		case "refresh_id":
			id, ok := value.(string)
			if !ok || !validHexID(id) {
				return false
			}
			record[key] = id
		case "config_revision":
			revision, ok := value.(string)
			if !ok || !validHexDigest(revision) {
				return false
			}
			record[key] = revision
		case "trigger":
			trigger, ok := value.(string)
			if !ok || (trigger != "interval" && trigger != "trigger") {
				return false
			}
			record[key] = trigger
		case "catalog_state":
			state, ok := value.(string)
			if !ok || (state != "retained_published" && state != "none_available") {
				return false
			}
			record[key] = state
		case "publication_impact":
			impact, ok := value.(string)
			if !ok || (impact != "candidate_rejected" && impact != "degraded_published") {
				return false
			}
			record[key] = impact
		case "blocker_class":
			class, ok := value.(string)
			if !ok || (class != "tool_absent" && class != "server_unverified") {
				return false
			}
			record[key] = class
		case "endpoint":
			text, ok := value.(string)
			if !ok {
				return false
			}
			sanitized := SanitizeEndpoint(text)
			if sanitized == "" {
				return false
			}
			record[key] = sanitized
		case "failure_stage":
			stage, ok := value.(string)
			if !ok || (stage != "dns" && stage != "connect" && stage != "tls" &&
				stage != "auth" && stage != "initialize" && stage != "tools_list") {
				return false
			}
			record[key] = stage
		case "failure_reason":
			reason, ok := value.(string)
			if !ok || !validFailureReason(reason) {
				return false
			}
			record[key] = reason
		case "mode":
			mode, ok := value.(string)
			if !ok || (mode != "hybrid" && mode != "hybrid_partial" && mode != "keyword") {
				return false
			}
			record[key] = mode
		case "backend_id":
			id, ok := value.(string)
			if !ok || !safeServer(id) {
				return false
			}
			record[key] = id
		case "protocol_version":
			version, ok := value.(string)
			if !ok || version == "" {
				return false
			}
			record[key] = version
		case "http_status":
			if value == nil {
				record[key] = nil
				continue
			}
			status, ok := value.(int)
			if !ok || status < 100 || status > 599 {
				return false
			}
			record[key] = status
		case "http_method":
			method, ok := value.(string)
			if !ok || !validMethod(method) {
				return false
			}
			record[key] = method
		case "route":
			route, ok := value.(string)
			if !ok || (route != "mcp" && route != "not_found" && route != "other") {
				return false
			}
			record[key] = route
		case "http_outcome":
			outcome, ok := value.(string)
			if !ok || !validHTTPOutcome(outcome) {
				return false
			}
			record[key] = outcome
		case "stage":
			stage, ok := value.(string)
			if !ok || !validLegacyStage(stage) {
				return false
			}
			record[key] = stage
		case "timing_scope":
			scope, ok := value.(string)
			if !ok || (scope != "http" && scope != "operation" && scope != "backend") {
				return false
			}
			record[key] = scope
		case "http_bytes_bucket":
			bucket, ok := value.(string)
			if !ok || !validHTTPBytesBucket(bucket) {
				return false
			}
			record[key] = bucket
		case "correlation_state":
			state, ok := value.(string)
			if !ok || (state != "valid" && state != "unavailable") {
				return false
			}
			record[key] = state
		case "config.path", "config.property_path", "config.server", "config.pin_server",
			"config.pin_tool", "config.ref_kind", "config.ref_name", "config.shape",
			"config.related.property_path", "config.related.ref_kind", "config.related.ref_name",
			"config.related.server", "config.remaining_stages":
			text, ok := value.(string)
			if !ok {
				return false
			}
			if text != "" {
				setDottedField(record, key, text)
			}
		case "config.line", "config.column", "config.related.line", "config.related.column",
			"config.actual_count", "config.max_count", "config.first_excess_index":
			number, ok := value.(int)
			if !ok || number < 0 {
				return false
			}
			if number != 0 {
				setDottedField(record, key, number)
			}
		case "config.diagnostics_incomplete":
			incomplete, ok := value.(bool)
			if !ok {
				return false
			}
			setDottedField(record, key, incomplete)
		case "resource.path", "resource.listener", "resource.address":
			text, ok := value.(string)
			if !ok {
				return false
			}
			if text != "" {
				setDottedField(record, key, text)
			}
		default:
			return false
		}
	}
	return true
}
func validDiscoveredToolName(name string) bool {
	if name == "" {
		return false
	}
	for index := range len(name) {
		char := name[index]
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' || char == '_' || char == '-') {
			return false
		}
	}
	return true
}

func setDottedField(record map[string]any, dotted string, value any) {
	object, field, _ := strings.Cut(dotted, ".")
	nested, ok := record[object].(map[string]any)
	if !ok {
		nested = make(map[string]any)
		record[object] = nested
	}
	if object == "config" {
		related, leaf, found := strings.Cut(field, ".")
		if found && related == "related" {
			relatedFields, ok := nested["related"].(map[string]any)
			if !ok {
				relatedFields = make(map[string]any)
				nested["related"] = relatedFields
			}
			relatedFields[leaf] = value
			return
		}
	}
	nested[field] = value
}

func numericField(value any) bool {
	switch number := value.(type) {
	case int, int64, uint64:
		return true
	case float64:
		return !math.IsNaN(number) && !math.IsInf(number, 0)
	default:
		return false
	}
}
func nonNegativeIntegerField(value any) bool {
	switch number := value.(type) {
	case int:
		return number >= 0
	case int64:
		return number >= 0
	case uint64:
		return true
	default:
		return false
	}
}

func validHexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, digit := range value {
		if !(digit >= '0' && digit <= '9' || digit >= 'a' && digit <= 'f') {
			return false
		}
	}
	return true
}

func cloneShape(shape map[string]any) map[string]any {
	copy := make(map[string]any, len(shape))
	for key, value := range shape {
		if sensitive(key) {
			copy[key] = "[REDACTED]"
		} else {
			copy[key] = cloneShapeValue(value)
		}
	}
	return copy
}

func cloneShapeValue(value any) any {
	switch item := value.(type) {
	case map[string]any:
		return cloneShape(item)
	case []any:
		copy := make([]any, len(item))
		for i, child := range item {
			copy[i] = cloneShapeValue(child)
		}
		return copy
	default:
		return value
	}
}

var envelopeFieldNames = [...]string{
	"auth_mode",
	"backend_call_id",
	"backend_id",
	"binding_relation",
	"blocker_class",
	"build_state",
	"candidate_servers",
	"candidate_tools",
	"catalog_state",
	"catalog_tools",
	"challenge_kind",
	"challenge_target",
	"chunk_count",
	"chunk_index",
	"client_family",
	"clock_skew_seconds",
	"config",
	"config.actual_count",
	"config.column",
	"config.diagnostics_incomplete",
	"config.first_excess_index",
	"config.line",
	"config.max_count",
	"config.path",
	"config.pin_server",
	"config.pin_tool",
	"config.property_path",
	"config.ref_kind",
	"config.ref_name",
	"config.related.column",
	"config.related.line",
	"config.related.property_path",
	"config.related.ref_kind",
	"config.related.ref_name",
	"config.related.server",
	"config.remaining_stages",
	"config.server",
	"config.shape",
	"config_revision",
	"correlation_state",
	"count",
	"credential_state",
	"diag_duration_ms",
	"diag_schema",
	"diagnostic_policy_revision",
	"discovered_tools",
	"duration_capped",
	"duration_ms",
	"effective_policy_revision",
	"effective_policy_revision_state",
	"endpoint",
	"error",
	"error.code",
	"error.kind",
	"error.remediation",
	"error.retryable",
	"error_detail",
	"error_detail_state",
	"event",
	"exposed_tools",
	"failure_reason",
	"failure_stage",
	"final_content_count",
	"final_shape",
	"final_structured_presence",
	"final_text_presence",
	"generation",
	"http_bytes",
	"http_bytes_bucket",
	"http_method",
	"http_outcome",
	"http_status",
	"level",
	"mcp_message_kind",
	"mcp_method",
	"message",
	"metadata_target_registered",
	"mode",
	"next_attempt_ms",
	"oauth_enabled",
	"operation",
	"operation_id",
	"operation_id_state",
	"operation_relation",
	"outcome",
	"path_class",
	"pins_resolved",
	"pins_total",
	"policy_source",
	"process_id",
	"process_state",
	"protocol_version",
	"publication_impact",
	"refresh_id",
	"request_id",
	"resource",
	"resource.address",
	"resource.listener",
	"resource.path",
	"resource_path_mode",
	"retained_age_ms",
	"retained_generation",
	"route",
	"route_outcome",
	"schema_version",
	"scope_required",
	"scope_state",
	"server",
	"servers_discovered",
	"service",
	"session_binding",
	"session_header",
	"session_ref",
	"session_ref_state",
	"session_state",
	"slots",
	"stage",
	"static_configured",
	"status_state",
	"timing_scope",
	"tool",
	"tools_discovered",
	"tools_published",
	"trigger",
	"upstream_content_count",
	"upstream_shape",
	"upstream_state",
	"upstream_structured_presence",
	"upstream_text_presence",
}

// EnvelopeFieldNames returns the complete sorted, deduplicated r8 vocabulary.
func EnvelopeFieldNames() []string {
	return append([]string(nil), envelopeFieldNames[:]...)
}

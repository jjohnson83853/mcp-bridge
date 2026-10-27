package observe

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"time"
)

type Entry struct {
	Time       time.Time
	Level      string
	Event      string
	RequestID  string
	Generation uint64
	Server     string
	Operation  string
	Duration   time.Duration
	Outcome    string
	Reason     string
	Fields     map[string]any
	HTTP       *HTTPDiagnostic
}

type Logger struct {
	mu      sync.Mutex
	output  io.Writer
	encoder *json.Encoder
}

func NewLogger(output io.Writer) *Logger {
	return &Logger{output: output, encoder: json.NewEncoder(output)}
}

func CorrelationID(header string) string {
	if validHexID(header) {
		return header
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(random[:])
}

func (l *Logger) Log(entry Entry) {
	if l == nil || l.encoder == nil {
		return
	}
	if entry.HTTP != nil {
		l.logHTTP(entry)
		return
	}
	l.logLegacyEntry(entry)
}

// logHTTP encodes the fixed-slot record once and attempts exactly one write.
// Validation failures and sink failures are nonfatal: unsafe output is
// suppressed, never retried and never partially written.
func (l *Logger) logHTTP(entry Entry) {
	if entry.Event != "http_complete" {
		// Incompatible non-http_complete use of the typed lane is invalid
		// input; suppress the typed record rather than emit a misleading one.
		return
	}
	encoder := &httpEncoder{}
	if !encoder.build(entry) {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.output.Write(encoder.buf.Bytes())
}

// logLegacyEntry is the untouched legacy record path.
func (l *Logger) logLegacyEntry(entry Entry) {
	// We need to know if the correlation is unavailable to prevent re-minting
	// an identity when the logger is explicitly told correlation failed.
	state, _ := entry.Fields["correlation_state"].(string)
	unavailable := state == "unavailable"

	// Base identity is preserved only if it is strictly valid.
	// It must never be inferred from untrusted caller input or minted here.
	requestID := entry.RequestID
	if !validHexID(requestID) {
		requestID = ""
	}

	value := map[string]any{
		"time":        time.Now().UTC().Format(time.RFC3339Nano),
		"level":       finite(entry.Level, "info", "debug", "info", "warn", "error"),
		"event":       finite(entry.Event, "unknown", "request", "auth", "backend_call", "search", "catalog_refresh", "breaker", "reload", "readiness", "http_complete", "mcp_complete", "backend_complete"),
		"generation":  entry.Generation,
		"operation":   finite(entry.Operation, "unknown", "request", "discover", "initialize", "tools_list", "tools_call", "call_tool", "search", "describe", "health", "describe_tool", "prompts_list", "prompts_get"),
		"duration_ms": entry.Duration.Milliseconds(),
		"outcome":     finiteOutcome(entry.Outcome),
		"reason":      finiteReason(entry.Reason),
		"service":     "mcp-bridge",
		"environment": "unspecified",
		"version":     "unspecified",
	}

	if unavailable {
		// Honest correlation unavailable: we do not invent an ID, we report the state.
		value["correlation_state"] = "unavailable"
		if entry.Event == "http_complete" {
			// Canonical structured error, replacing the flat error_class.
			value["error"] = map[string]any{"class": "correlation_unavailable"}
		}
	} else {
		if entry.Event == "http_complete" {
			value["correlation_state"] = "available"
			// Delivery failures project the canonical nested error class from
			// the already-validated terminal reason; no caller field can
			// bypass this closed mapping.
			if class := deliveryErrorClass(entry.Reason); class != "" {
				value["error"] = map[string]any{"class": class}
			}
		}
	}
	// Preserve a strictly valid owned identity if provided, regardless of
	// correlation state, but never invent one when it is absent or malformed.
	if requestID != "" {
		value["request_id"] = requestID
	} else if !unavailable && entry.Event != "http_complete" {
		if id := CorrelationID(entry.RequestID); id != "" {
			value["request_id"] = id
		}
	}

	if safeServer(entry.Server) {
		value["server"] = entry.Server
	}
	for key, field := range entry.Fields {
		switch key {
		case "count", "slots", "catalog_tools":
			switch field.(type) {
			case int, int64, uint64, float64:
				value[key] = field
			}
		case "upstream_content_count", "final_content_count":
			switch field.(type) {
			case int, int64, float64:
				value[key] = field
			}
		case "upstream_text_presence", "upstream_structured_presence", "final_text_presence", "final_structured_presence":
			if presence, ok := field.(string); ok {
				value[key] = finite(presence, "unknown", "unknown", "present", "absent")
			}
		case "upstream_shape", "final_shape":
			if shape, ok := field.(map[string]any); ok {
				value[key] = shape
			}
		case "operation_id", "backend_call_id":
			// Correlation identities are server-owned 32-hex values; any other
			// shape is dropped rather than emitted.
			if id, ok := field.(string); ok && validHexID(id) {
				value[key] = id
			}
		case "mcp_method", "mcp_message_kind", "tool":
			if text, ok := field.(string); ok {
				value[key] = text
			}
		case "outcome":
			if outcome, ok := field.(string); ok {
				value[key] = finiteOutcome(outcome)
			}
		case "reason":
			if reason, ok := field.(string); ok {
				value[key] = finiteReason(reason)
			}
		case "duration_ms":
			switch field.(type) {
			case int, int64, float64:
				value[key] = field
			}
		case "mode":
			if mode, ok := field.(string); ok {
				value[key] = finite(mode, "unknown", "hybrid", "hybrid_partial", "keyword")
			}
		case "config_path", "location", "line", "column", "code", "property", "shape", "ref_kind", "ref_name",
			"pin_server", "pin_tool", "related_location", "related_ref_kind", "related_ref_name",
			"diagnostic_index", "remaining_stages":
			value[key] = field
		case "diagnostics_incomplete":
			if incomplete, ok := field.(bool); ok {
				value[key] = incomplete
			}
		case "http_status":
			// Status must be a valid integer in the HTTP 100-599 range, or nil
			// if the response status was unobserved.
			if field == nil {
				value[key] = nil // Explicit null
			} else if n, ok := field.(int); ok && n >= 100 && n <= 599 {
				value[key] = n
			}
		case "http_method":
			if method, ok := field.(string); ok {
				value[key] = finite(method, "OTHER", "GET", "POST", "DELETE", "HEAD", "OPTIONS", "PUT", "PATCH", "OTHER")
			}
		case "route":
			if route, ok := field.(string); ok {
				value[key] = finite(route, "not_found", "mcp", "not_found")
			}
		case "http_outcome":
			if outcome, ok := field.(string); ok {
				value[key] = finite(outcome, "unknown", "success", "failure", "incomplete", "unknown")
			}
		case "stage":
			if stage, ok := field.(string); ok {
				value[key] = finite(stage, "unknown", "none", "http_route", "host_admission", "origin_admission", "client_auth", "session_admission", "request_limits", "mcp_decode", "mcp_dispatch", "tool_admission", "backend_dispatch", "backend_response", "response_delivery", "unknown")
			}
		case "timing_scope":
			if scope, ok := field.(string); ok {
				switch scope {
				case "http", "operation", "backend":
					value[key] = scope
				}
			}
		case "http_bytes_bucket":
			if bucket, ok := field.(string); ok {
				value[key] = finite(bucket, "unavailable", "empty", "le_1k", "le_16k", "le_256k", "le_1m", "gt_1m", "unavailable")
			}
		case "correlation_state":
			if str, ok := field.(string); ok && (str == "valid" || str == "unavailable") {
				value[key] = str
			}
		}
	}

	// Fixed event-specific message, emitted after fields to prevent overriding.
	switch entry.Event {
	case "http_complete", "mcp_complete", "backend_complete":
		eventName, _ := value["event"].(string)
		value["message"] = eventMessage(eventName)
	}

	redact(value)
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.encoder.Encode(value); err != nil {
		// The standard json.Encoder caches the writer error in its err field
		// and will refuse to attempt future writes. Recreate the encoder so
		// the next Log call attempts its own record.
		if l.output != nil {
			l.encoder = json.NewEncoder(l.output)
		}
	}
}

// outcomeVocabulary is the closed set of terminal outcomes a record may
// carry; anything else collapses to the leading fallback.
var outcomeVocabulary = []string{"unknown", "success", "failure", "error", "denied", "busy", "timeout", "fallback", "degraded"}

// reasonVocabulary is the closed set of terminal reasons, including the
// tool-admission and tool-reported failure families.
var reasonVocabulary = []string{"none", "unknown", "host_rejected", "origin_rejected", "client_auth_failed", "session_auth_failed", "session_capacity", "request_too_large", "route_not_found", "backend_unavailable", "backend_auth_failed", "backend_busy", "deadline_exceeded", "upstream_protocol_error", "unsupported_capability", "invalid_config", "client_disconnected", "tool_reported_error", "unknown_tool", "invalid_arguments", "catalog_changed", "unsupported_schema", "malformed_request", "invalid_request", "method_not_found", "request_cancelled", "internal_error", "correlation_unavailable"}

func finiteOutcome(value string) string {
	return finite(value, "unknown", outcomeVocabulary...)
}

func finiteReason(value string) string {
	return finite(value, "unknown", reasonVocabulary...)
}

func finite(value string, fallback string, allowed ...string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return fallback
}

// deliveryErrorClass projects an owned http delivery failure reason into the
// canonical nested error class. The mapping is closed: reasons outside it
// carry no error object, and hostile caller fields can never supply one.
func deliveryErrorClass(reason string) string {
	switch reason {
	case "client_disconnected", "unknown":
		return reason
	}
	return ""
}
func safeServer(value string) bool {
	if value == "_bridge" {
		return true
	}
	if len(value) == 0 || len(value) > 32 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, r := range value[1:] {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

func redact(value any) {
	switch item := value.(type) {
	case map[string]any:
		for key, child := range item {
			if sensitive(key) {
				item[key] = "[REDACTED]"
				continue
			}
			redact(child)
		}
	case []any:
		for _, child := range item {
			redact(child)
		}
	}
}

func sensitive(key string) bool {
	key = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", ""), "_", ""))
	for _, marker := range []string{"token", "key", "secret", "password", "authorization", "cookie", "session", "argument", "result", "query", "schema", "description", "payload", "url", "endpoint", "header", "stack"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}

// validHexID reports whether the provided string is a valid 128-bit lowercase hexadecimal identifier.
func validHexID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, r := range id {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

// eventMessage provides a fixed, event-specific message for the logs.
func eventMessage(event string) string {
	switch event {
	case "http_complete":
		return "http request completed"
	}
	return "mcp-bridge event"
}

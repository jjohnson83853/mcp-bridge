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
}

type Logger struct {
	mu      sync.Mutex
	encoder *json.Encoder
}

func NewLogger(output io.Writer) *Logger { return &Logger{encoder: json.NewEncoder(output)} }

func CorrelationID(header string) string {
	if len(header) == 32 {
		if _, err := hex.DecodeString(header); err == nil && strings.ToLower(header) == header {
			return header
		}
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
	value := map[string]any{
		"time":        time.Now().UTC().Format(time.RFC3339Nano),
		"level":       finite(entry.Level, "info", "debug", "info", "warn", "error"),
		"event":       finite(entry.Event, "unknown", "request", "auth", "backend_call", "search", "catalog_refresh", "breaker", "reload", "readiness"),
		"request_id":  CorrelationID(entry.RequestID),
		"generation":  entry.Generation,
		"operation":   finite(entry.Operation, "unknown", "request", "discover", "initialize", "tools_list", "tools_call", "search", "describe", "health"),
		"duration_ms": entry.Duration.Milliseconds(),
		"outcome":     finite(entry.Outcome, "unknown", "success", "failure", "denied", "busy", "timeout", "fallback", "degraded"),
		"reason":      finite(entry.Reason, "unknown", "none", "backend_unavailable", "backend_auth_failed", "backend_busy", "deadline_exceeded", "upstream_protocol_error", "unsupported_capability", "invalid_config"),
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
		}
	}
	redact(value)
	l.mu.Lock()
	defer l.mu.Unlock()
	_ = l.encoder.Encode(value)
}

func finite(value string, fallback string, allowed ...string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return fallback
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

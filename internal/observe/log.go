package observe

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
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
	Error      *ErrorInfo
	Fields     map[string]any
	HTTP       *HTTPDiagnostic
}

type Logger struct {
	mu     sync.Mutex
	stdout io.Writer
	stderr io.Writer
}

var loggingWriteFailedLine = []byte(`{"schema_version":1,"level":"warn","service":"mcp-bridge","event":"logging_write_failed","message":"logging write failed","operation":"logging","outcome":"failure"}` + "\n")

func NewLogger(stdout, stderr io.Writer) *Logger {
	return &Logger{stdout: stdout, stderr: stderr}
}

// WriteJSONLine marshals one application record and attempts one newline-
// terminated write. Encoding or sink failures are returned without retry.
func WriteJSONLine(output io.Writer, record any) error {
	if output == nil {
		return errors.New("observe: nil log output")
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	written, err := output.Write(encoded)
	if err != nil {
		return err
	}
	if written != len(encoded) {
		return io.ErrShortWrite
	}
	return nil
}

// WriteLoggingFailureFallback writes the minimal logging failure record to the
// opposite standard stream from output.
func WriteLoggingFailureFallback(output io.Writer) {
	fallback := os.Stderr
	if output == os.Stderr {
		fallback = os.Stdout
	}
	if fallback != nil {
		_, _ = fallback.Write(loggingWriteFailedLine)
	}
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
	if l == nil {
		return
	}
	if entry.HTTP != nil {
		l.logHTTP(entry)
		return
	}
	record, ok := buildEnvelope(entry)
	if !ok {
		l.logFailure(entry.Level)
		return
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		l.logFailure(entry.Level)
		return
	}
	encoded = append(encoded, '\n')
	l.write(entry.Level, encoded)
}

// logHTTP encodes the fixed-slot record once and attempts exactly one write.
// Encode and sink failures are surfaced once on the opposite severity lane.
func (l *Logger) logHTTP(entry Entry) {
	if entry.Event != "http_complete" {
		return
	}
	encoder := &httpEncoder{}
	if !encoder.build(entry) {
		l.logFailure(entry.Level)
		return
	}
	l.write(entry.Level, encoder.buf.Bytes())
}

func (l *Logger) write(level string, encoded []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	output := l.stdout
	if level == "warn" || level == "error" {
		output = l.stderr
	}
	if output == nil {
		l.writeFailureFallback(level)
		return
	}
	written, err := output.Write(encoded)
	if err != nil || written != len(encoded) {
		l.writeFailureFallback(level)
	}
}

func (l *Logger) logFailure(level string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writeFailureFallback(level)
}

func (l *Logger) writeFailureFallback(level string) {
	output := l.stderr
	if level == "warn" || level == "error" {
		output = l.stdout
	}
	if output != nil {
		_, _ = output.Write(loggingWriteFailedLine)
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

// eventMessage provides the fixed, event-specific message for application logs.
func eventMessage(event string) string {
	switch event {
	case "http_complete":
		return "http request completed"
	case "mcp_complete":
		return "MCP operation completed"
	case "backend_complete":
		return "upstream tool call completed"
	case "upstream_ephemeral_handshake":
		return "upstream protocol handshake observed"
	case "startup_failed":
		return "bridge startup failed"
	case "cli_usage_failed":
		return "invalid command-line usage"
	case "config_validation_failed":
		return "configuration validation failed"
	case "config_validation_succeeded":
		return "configuration valid"
	case "startup_publication_failed":
		return "startup publication failed"
	case "startup_snapshot_rejected":
		return "startup snapshot unavailable"
	case "prompt_publication_failed":
		return "prompt publication failed"
	case "config_reload_applied":
		return "configuration reload applied"
	case "reload_rejected", "config_reload_rejected":
		return "configuration reload rejected"
	case "reload_publication_failed":
		return "configuration reload publication failed"
	case "catalog_refresh_started":
		return "catalog refresh started"
	case "catalog_refresh_completed":
		return "catalog refresh completed"
	case "catalog_refresh_failed":
		return "catalog refresh failed"
	case "upstream_discovery_failed":
		return "upstream discovery failed"
	case "pin_blocker_detail":
		return "mandatory pin blocks catalog refresh"
	case "logging_write_failed":
		return "logging write failed"
	case "internal_warning":
		return "bridge encountered an internal warning"
	case "reload":
		return "configuration reload diagnostic"
	default:
		return "mcp-bridge event"
	}
}

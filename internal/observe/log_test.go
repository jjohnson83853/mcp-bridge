package observe_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

func TestLoggerRedactsSensitiveFieldsAndCreatesCorrelationID(t *testing.T) {
	var output bytes.Buffer
	logger := observe.NewLogger(&output)
	id := observe.CorrelationID("not-a-valid-id")
	if len(id) != 32 {
		t.Fatalf("correlation id length=%d", len(id))
	}
	for _, r := range id {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			t.Fatalf("correlation id=%q", id)
		}
	}
	logger.Log(observe.Entry{Level: "info", Event: "backend_call", RequestID: id, Server: "alpha", Fields: map[string]any{
		"server": "alpha", "operation": "call_tool", "token": "sensitive-token", "arguments": map[string]any{"secret": "payload-secret"}, "endpoint": "https://host.invalid/mcp?api_key=private",
	}})
	text := output.String()
	for _, secret := range []string{"sensitive-token", "payload-secret", "api_key=private"} {
		if strings.Contains(text, secret) {
			t.Fatalf("log leaked %q: %s", secret, text)
		}
	}
	if !strings.Contains(text, `"request_id":"`+id+`"`) || !strings.Contains(text, `"server":"alpha"`) {
		t.Fatalf("missing safe correlation/server fields: %s", text)
	}
}

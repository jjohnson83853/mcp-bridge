package observe

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogRejectsInvalidEnvelopeFields(t *testing.T) {

	var output bytes.Buffer
	NewLogger(&output, &output).Log(Entry{
		Level: "trace-secret", Event: "attacker-event", Operation: "private-op", Outcome: "arbitrary",
		Server: "Bad_Server", Fields: map[string]any{"mode": "attacker-mode", "count": 3, "slots": "not-a-number"},
	})
	if output.Len() != 0 {
		t.Fatalf("invalid envelope was emitted: %s", output.String())
	}
}
func TestMetricsHelpersBoundLabelsAndExposeAcceptedUpdates(t *testing.T) {
	m := NewMetrics([]string{"alpha"})
	if !m.RecordRequest("prompts_list", "success") || m.RecordRequest("", "success") {
		t.Fatal("request metric acceptance contract changed")
	}
	if !m.SetCatalogTools("alpha", 3) || !m.SetCatalogAge("alpha", 2.5) || !m.RecordRefresh("alpha", "success", "none") {
		t.Fatal("known server metric rejected")
	}
	if m.SetCatalogTools("bad_server", 3) || m.SetCatalogAge("bad_server", 2) || m.RecordRefresh("bad_server", "success", "none") {
		t.Fatal("unknown server metric accepted")
	}
	if !m.RecordReload("rejected", "invalid_configuration") || m.RecordReload("secret-outcome", "secret-reason") {
		t.Fatal("reload label vocabulary contract changed")
	}
	if !m.SetReadiness("alpha", Readiness(9)) || m.SetReadiness("bad_server", ReadinessReady) {
		t.Fatal("readiness server acceptance contract changed")
	}
	response := httptest.NewRecorder()
	m.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	text := response.Body.String()
	for _, want := range []string{`bridge_requests_total{operation="prompts_list",outcome="success",server="_bridge"} 1`, `bridge_catalog_tools{server="alpha"} 3`, `bridge_catalog_age_seconds{server="alpha"} 2.5`, `bridge_catalog_refresh_total{outcome="success",reason="none",server="alpha"} 1`, `bridge_config_reload_total{outcome="rejected",reason="invalid_configuration"} 1`, `bridge_readiness{server="alpha"} 9`} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in metrics: %s", want, text)
		}
	}
	for _, secret := range []string{"bad_server", "secret-outcome", "secret-reason"} {
		if strings.Contains(text, secret) {
			t.Errorf("unaccepted metric label leaked %q", secret)
		}
	}
}

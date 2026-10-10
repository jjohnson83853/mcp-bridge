package observe

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// exposition renders one instance's exposition. Each Metrics owns its registry,
// so tests never share collector state and can run in any order.
func exposition(t *testing.T, m *Metrics) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	m.Handler().ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	if recorder.Code != 200 {
		t.Fatalf("metrics handler status = %d", recorder.Code)
	}
	return recorder.Body.String()
}

func containsServer(t *testing.T, exp, server string) bool {
	t.Helper()
	return strings.Contains(exp, `server="`+server+`"`)
}

// TestBoundaryServerIDs pins the finite server-label vocabulary: an id that does
// not match the configuration's own server-ID rule is refused and never reaches
// the exposition, while the 32-character boundary is accepted.
func TestBoundaryServerIDs(t *testing.T) {
	refused := []string{"Gamma", "gamma_x", "1gamma", strings.Repeat("a", 33), ""}
	for _, id := range refused {
		m := NewMetrics(nil)
		if m.SetCatalogTools(id, 1) {
			t.Fatalf("expected SetCatalogTools refusal for %q", id)
		}
		if m.RecordRefresh(id, "updated", "") {
			t.Fatalf("expected RecordRefresh refusal for %q", id)
		}
		if id != "" && containsServer(t, exposition(t, m), id) {
			t.Fatalf("%q must not appear in exposition", id)
		}
	}
	// The mirror must accept exactly what the schema accepts, including a
	// trailing hyphen, and stop at the schema's 32-character boundary.
	m := NewMetrics(nil)
	for _, id := range []string{strings.Repeat("a", 32), "gamma--"} {
		if !m.SetCatalogTools(id, 1) {
			t.Fatalf("expected schema-valid id %q to be accepted", id)
		}
		if !containsServer(t, exposition(t, m), id) {
			t.Fatalf("schema-valid id %q missing from exposition", id)
		}
	}
}

// TestCardinalityRefusal pins the R-18 cardinality bound: every server the
// configuration admits stays observable, one beyond the bound is refused, and
// the reserved aggregate label is never counted against it.
func TestCardinalityRefusal(t *testing.T) {
	m := NewMetrics(nil)
	for i := 0; i < maxServers; i++ {
		id := fmt.Sprintf("srv%04d", i)
		if !m.SetCatalogTools(id, i) {
			t.Fatalf("expected id %q to be accepted", id)
		}
	}
	if m.SetCatalogTools("overflow", 1) {
		t.Fatal("expected refusal beyond maxServers")
	}
	if containsServer(t, exposition(t, m), "overflow") {
		t.Fatal("refused id must not appear in exposition")
	}
	if !m.SetReadiness(bridgeServerID, ReadinessReady) {
		t.Fatal("the aggregate label must stay valid at the bound")
	}
}

// TestAggregateLabelAlwaysValid pins that the reserved aggregate label is
// accepted without a configured server, so inbound and aggregate work is always
// countable.
func TestAggregateLabelAlwaysValid(t *testing.T) {
	m := NewMetrics(nil)
	if !m.SetReadiness(bridgeServerID, ReadinessReady) {
		t.Fatal("_bridge must stay valid")
	}
	if !strings.Contains(exposition(t, m), `bridge_readiness{server="_bridge"} 2`) {
		t.Fatal("_bridge readiness missing from exposition")
	}
}

// TestServerIDsSorted pins the reported set: seeded servers plus the reserved
// aggregate label, sorted, which is what the ops surface and logs enumerate.
func TestServerIDsSorted(t *testing.T) {
	got := NewMetrics([]string{"beta", "alpha"}).ServerIDs()
	want := []string{"_bridge", "alpha", "beta"}
	if len(got) != len(want) {
		t.Fatalf("ServerIDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ServerIDs = %v, want %v", got, want)
		}
	}
}

// TestConcurrentRegistration pins that a reload storm registering many servers
// at once neither races nor loses an accepted id.
func TestConcurrentRegistration(t *testing.T) {
	m := NewMetrics(nil)
	var group sync.WaitGroup
	for i := 0; i < 100; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			id := fmt.Sprintf("srv%04d", i)
			if !m.SetCatalogTools(id, i) {
				t.Errorf("expected id %q to be accepted", id)
			}
		}(i)
	}
	group.Wait()
	if len(m.ServerIDs()) != 101 { // 100 servers plus the aggregate label
		t.Fatalf("ServerIDs = %d ids, want 101", len(m.ServerIDs()))
	}
}

// TestRefreshReasonIsALabel pins the plan's catalog-refresh shape (§4.11): the
// finite outcome and reason travel as labels, and an unconfigured server never
// creates a series.
func TestRefreshReasonIsALabel(t *testing.T) {
	m := NewMetrics([]string{"alpha"})
	if !m.RecordRefresh("alpha", "failed", "upstream_unavailable") {
		t.Fatal("expected a configured server to be recorded")
	}
	exp := exposition(t, m)
	if !strings.Contains(exp, `bridge_catalog_refresh_total{outcome="failed",reason="upstream_unavailable",server="alpha"} 1`) {
		t.Fatalf("refresh label set missing: %s", exp)
	}
	if m.RecordRefresh("Gamma", "updated", "") {
		t.Fatal("expected an id outside the schema rule to be refused")
	}
}

// TestInboundRequestsUseTheAggregateLabel pins the inbound request metric
// (§4.11): inbound work lands on the reserved aggregate label, because a single
// request is not attributable to one upstream server.
func TestInboundRequestsUseTheAggregateLabel(t *testing.T) {
	m := NewMetrics([]string{"alpha"})
	if !m.RecordRequest("request", "success") {
		t.Fatal("expected the aggregate request count to be recorded")
	}
	exp := exposition(t, m)
	if !strings.Contains(exp, `bridge_requests_total{operation="request",outcome="success",server="_bridge"} 1`) {
		t.Fatalf("inbound request metric missing: %s", exp)
	}
	if m.RecordRequest("", "success") || m.RecordRequest("request", "") {
		t.Fatal("an empty operation or outcome must be refused")
	}
}

// TestCatalogAgeAndReadinessArePerServer pins that both gauges carry the finite
// server label and the readiness vocabulary distinguishes unavailable, degraded
// and ready instead of collapsing them.
func TestCatalogAgeAndReadinessArePerServer(t *testing.T) {
	m := NewMetrics([]string{"alpha"})
	if !m.SetCatalogAge("alpha", 12.5) {
		t.Fatal("expected a configured server's catalog age to be set")
	}
	if !strings.Contains(exposition(t, m), `bridge_catalog_age_seconds{server="alpha"} 12.5`) {
		t.Fatal("catalog age missing from exposition")
	}
	states := []struct {
		state Readiness
		value int
	}{
		{ReadinessUnavailable, 0},
		{ReadinessDegraded, 1},
		{ReadinessReady, 2},
	}
	for _, testCase := range states {
		if !m.SetReadiness("alpha", testCase.state) {
			t.Fatalf("expected readiness %d to be accepted", testCase.state)
		}
		want := fmt.Sprintf(`bridge_readiness{server="alpha"} %d`, testCase.value)
		if !strings.Contains(exposition(t, m), want) {
			t.Fatalf("exposition missing %q", want)
		}
	}
	if m.SetCatalogAge("Gamma", 1) {
		t.Fatal("expected an id outside the schema rule to be refused")
	}
	if containsServer(t, exposition(t, m), "Gamma") {
		t.Fatal("a refused id must not create a series")
	}
}

// TestPromptMetricsBoundedLabels pins the prompt observability contract
// (R-27/AC-29): the prompt surface contributes exactly the finite prompt
// operations to the inbound request metric plus one unlabelled prompt count,
// and no server id, prompt name, argument or payload value can reach the
// exposition.
func TestPromptMetricsBoundedLabels(t *testing.T) {
	m := NewMetrics([]string{"alpha"})
	if !m.RecordRequest("prompts_list", "success") || !m.RecordRequest("prompts_get", "failure") {
		t.Fatal("the finite prompt operation vocabulary must be recordable")
	}
	// Every value below is something a caller could try to smuggle into a
	// label: an unknown operation, a public name, a raw upstream name, and the
	// configured server id.
	for _, operation := range []string{"search", "prompts_delete", "p_publicname", "raw-secret-name", "alpha"} {
		if m.RecordRequest(operation, "success") {
			t.Fatalf("out-of-vocabulary operation %q was accepted as a label", operation)
		}
	}
	for _, outcome := range []string{"rejected", "raw-secret-name"} {
		if m.RecordRequest("prompts_list", outcome) {
			t.Fatalf("out-of-vocabulary outcome %q was accepted as a label", outcome)
		}
	}
	if !m.SetCatalogPrompts(0) {
		t.Fatal("an empty published prompt set must be recordable")
	}
	if m.SetCatalogPrompts(-1) {
		t.Fatal("a negative prompt count must be refused")
	}
	if !m.SetCatalogPrompts(2) {
		t.Fatal("a published prompt set must be recordable")
	}

	exp := exposition(t, m)
	for _, want := range []string{
		`bridge_requests_total{operation="prompts_list",outcome="success",server="_bridge"} 1`,
		`bridge_requests_total{operation="prompts_get",outcome="failure",server="_bridge"} 1`,
		`bridge_catalog_prompts 2`,
	} {
		if !strings.Contains(exp, want) {
			t.Fatalf("missing %q in exposition: %s", want, exp)
		}
	}
	if strings.Contains(exp, "bridge_catalog_prompts{") {
		t.Fatalf("the prompt count gauge must carry no labels: %s", exp)
	}
	for _, secret := range []string{"alpha", "p_publicname", "raw-secret-name", "search"} {
		if strings.Contains(exp, secret) {
			t.Fatalf("sensitive value %q reached the exposition: %s", secret, exp)
		}
	}
}

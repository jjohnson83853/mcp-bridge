package service_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
)

func TestOperationsLivenessAndPerServerReadiness(t *testing.T) {
	state := service.NewReadinessState(true, true, true, true)
	state.SetServer("alpha", true, true, "")
	state.SetServer("beta", false, false, "backend_unavailable")
	handler := service.NewOperationsHandler(state, observe.NewMetrics([]string{"alpha", "beta"}).Handler())
	live := httptest.NewRecorder()
	handler.ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/livez", nil))
	if live.Code != http.StatusOK {
		t.Fatalf("livez=%d, want 200", live.Code)
	}
	ready := httptest.NewRecorder()
	handler.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if ready.Code != http.StatusOK || !strings.Contains(ready.Body.String(), "degraded") || !strings.Contains(ready.Body.String(), "beta") {
		t.Fatalf("readyz status=%d body=%q", ready.Code, ready.Body.String())
	}
	state.SetServer("alpha", false, false, "backend_unavailable")
	unready := httptest.NewRecorder()
	handler.ServeHTTP(unready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if unready.Code != http.StatusServiceUnavailable {
		t.Fatalf("zero usable backends ready status=%d", unready.Code)
	}
}

func TestOperationsMetricsRouteAndUnknownPaths(t *testing.T) {
	state := service.NewReadinessState(true, true, true, true)
	handler := service.NewOperationsHandler(state, observe.NewMetrics(nil).Handler())
	metrics := httptest.NewRecorder()
	handler.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if metrics.Code != http.StatusOK || !strings.Contains(metrics.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("metrics status=%d type=%q", metrics.Code, metrics.Header().Get("Content-Type"))
	}
	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/", nil))
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown operations route=%d, want 404", unknown.Code)
	}
}

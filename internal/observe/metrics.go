package observe

import (
	"net/http"
	"regexp"
	"sort"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// serverIDPattern mirrors the config schema's server-ID rule locally:
// lowercase letter start, then lowercase letters/digits/hyphens, 32 chars max.
var serverIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// maxServers bounds the label-cardinality the metrics surface will accept. It
// mirrors the configuration's own admission bound (internal/config rejects a
// document with more than 1024 servers), so every admitted server stays
// observable while a reload storm can never grow the series without bound
// (R-18). The reserved aggregate label is not counted against it.
const maxServers = 1024

// bridgeServerID is the permanently valid aggregate label.
const bridgeServerID = "_bridge"

// reloadOutcomes and reloadReasons bound the reload counter's labels to the
// finite vocabulary the catalog loop reports, so a rejected document's text can
// never become a series.
var (
	reloadOutcomes = map[string]bool{"success": true, "rejected": true}
	reloadReasons  = map[string]bool{"none": true, "invalid_configuration": true, "admission_failed": true}
)

type serverSet struct {
	mu       sync.RWMutex
	accepted map[string]bool
	// count is the number of configured server ids accepted so far.
	count int
}

func newServerSet(seed []string) *serverSet {
	set := &serverSet{accepted: make(map[string]bool)}
	set.accepted[bridgeServerID] = true
	for _, id := range seed {
		set.valid(id)
	}
	return set
}

// valid reports whether id is accepted, registering it lazily if it fits the
// ID rule and the cardinality bound. Concurrency-safe; no lock across calls.
func (s *serverSet) valid(id string) bool {
	if id == bridgeServerID {
		return true
	}
	if !serverIDPattern.MatchString(id) {
		return false
	}
	s.mu.RLock()
	if s.accepted[id] {
		s.mu.RUnlock()
		return true
	}
	s.mu.RUnlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.accepted[id] { // re-check under write lock
		return true
	}
	if s.count >= maxServers {
		return false
	}
	s.accepted[id] = true
	s.count++
	return true
}

// ids returns the accepted set sorted.
func (s *serverSet) ids() []string {
	s.mu.RLock()
	out := make([]string, 0, len(s.accepted))
	for id := range s.accepted {
		out = append(out, id)
	}
	s.mu.RUnlock()
	sort.Strings(out)
	return out
}

// Readiness is the finite readiness vocabulary of plan §4.11: the gauge's value
// is derived from this state, so an unready server is distinguishable from a
// degraded one instead of collapsing both to zero.
type Readiness int

const (
	// ReadinessUnavailable is a server with no usable catalog or no connection.
	ReadinessUnavailable Readiness = iota
	// ReadinessDegraded is a server whose catalog is usable but not complete
	// and fresh.
	ReadinessDegraded
	// ReadinessReady is a server with a complete, fresh, trusted catalog.
	ReadinessReady
)

type Metrics struct {
	servers  *serverSet
	registry *prometheus.Registry

	requestsTotal *prometheus.CounterVec
	catalogTools  *prometheus.GaugeVec
	catalogAge    *prometheus.GaugeVec
	refreshTotal  *prometheus.CounterVec
	reloadTotal   *prometheus.CounterVec
	readiness     *prometheus.GaugeVec
}

// NewMetrics builds the bridge's metric surface. The collectors live in a
// registry private to this instance, so a process with several bridges (tests,
// an embedded operator) never conflicts on a shared default registry.
func NewMetrics(serverIDs []string) *Metrics {
	m := &Metrics{
		servers:  newServerSet(serverIDs),
		registry: prometheus.NewRegistry(),
		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bridge_requests_total",
			Help: "Inbound bridge requests by server, operation, and outcome.",
		}, []string{"server", "operation", "outcome"}),
		catalogTools: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bridge_catalog_tools",
			Help: "Accepted tool count of each server's current catalog.",
		}, []string{"server"}),
		catalogAge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bridge_catalog_age_seconds",
			Help: "Seconds since the server's complete catalog was last refreshed.",
		}, []string{"server"}),
		refreshTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bridge_catalog_refresh_total",
			Help: "Catalog refresh attempts by server, outcome, and reason.",
		}, []string{"server", "outcome", "reason"}),
		reloadTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bridge_config_reload_total",
			Help: "Configuration reload attempts by outcome and reason.",
		}, []string{"outcome", "reason"}),
		readiness: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bridge_readiness",
			Help: "Readiness by server: 0 unavailable, 1 degraded, 2 ready.",
		}, []string{"server"}),
	}
	m.registry.MustRegister(m.requestsTotal, m.catalogTools, m.catalogAge, m.refreshTotal, m.reloadTotal, m.readiness)
	return m
}

// Handler serves this instance's Prometheus exposition for the ops listener.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) ServerIDs() []string {
	return m.servers.ids()
}

// RecordRequest counts one inbound bridge request. The aggregate label is fixed:
// inbound work is never attributable to a single configured server.
func (m *Metrics) RecordRequest(operation, outcome string) bool {
	if operation == "" || outcome == "" {
		return false
	}
	m.requestsTotal.WithLabelValues(bridgeServerID, operation, outcome).Inc()
	return true
}

func (m *Metrics) SetCatalogTools(server string, tools int) bool {
	if !m.servers.valid(server) {
		return false
	}
	m.catalogTools.WithLabelValues(server).Set(float64(tools))
	return true
}

func (m *Metrics) SetCatalogAge(server string, seconds float64) bool {
	if !m.servers.valid(server) {
		return false
	}
	m.catalogAge.WithLabelValues(server).Set(seconds)
	return true
}

func (m *Metrics) RecordRefresh(server, outcome, reason string) bool {
	if !m.servers.valid(server) {
		return false
	}
	m.refreshTotal.WithLabelValues(server, outcome, reason).Inc()
	return true
}

// RecordReload counts one configuration reload attempt. The labels belong to a
// finite vocabulary, so a rejected document never contributes a free-text
// series (R-18).
func (m *Metrics) RecordReload(outcome, reason string) bool {
	if !reloadOutcomes[outcome] || !reloadReasons[reason] {
		return false
	}
	m.reloadTotal.WithLabelValues(outcome, reason).Inc()
	return true
}

func (m *Metrics) SetReadiness(server string, state Readiness) bool {
	if !m.servers.valid(server) {
		return false
	}
	m.readiness.WithLabelValues(server).Set(float64(state))
	return true
}

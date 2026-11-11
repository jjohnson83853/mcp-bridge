package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

// The finite reason vocabulary a failed refresh records for one server
// (plan §4.7/§4.11). Every value is a stable operator-facing token.
const (
	// RefreshReasonUnavailable marks an upstream the caller could not reach.
	RefreshReasonUnavailable = "upstream_unavailable"
	// RefreshReasonDiscoveryFailed marks a reachable upstream whose tool
	// listing could not be produced.
	RefreshReasonDiscoveryFailed = "discovery_failed"
	// RefreshReasonClientMissing marks a server with no reusable working client.
	RefreshReasonClientMissing = "client_missing"
	// RefreshReasonNotProbed marks a catalog restored from disk that this
	// process has not contacted yet, so its completeness is unverified.
	RefreshReasonNotProbed = "not_probed"
)

// RefreshDeps are one refresh pass's injected collaborators. A refresh reuses the
// reload collaborators but, unlike a reload, keeps every server's last known good
// catalog when that server's discovery fails (plan §4.7).
type RefreshDeps struct {
	ReloadDeps
	// Reason classifies one server's discovery failure into the finite reason
	// vocabulary. It defaults to RefreshReasonDiscoveryFailed.
	Reason func(server config.Server, err error) string
	// OnRejected reports a refresh candidate rejected before publication.
	OnRejected func(err error)
}

// Refresh admits one candidate configuration as a periodic refresh rather than a
// reload: a server whose own discovery fails keeps its last known good catalog
// and stale status while the remaining servers advance normally, so one upstream
// outage never rejects the whole pass. Membership and credential changes are
// still the reload path's job; a candidate that needs a new client is rejected.
func (r *Runtime) Refresh(candidate *config.Config, deps RefreshDeps) (*Assembly, error) {
	if candidate == nil {
		return nil, errors.New("service: refresh needs a candidate configuration")
	}
	if deps.Discover == nil || deps.Now == nil {
		return nil, errors.New("service: refresh needs discovery and clock dependencies")
	}
	return r.build(buildInput{
		mode:      buildRefresh,
		previous:  r.current.Load(),
		candidate: candidate,
		deps:      deps.ReloadDeps,
		reason:    deps.Reason,
	})
}

// RefresherOptions configures one refresh driver (plan §4.7).
type RefresherOptions struct {
	// Interval runs one refresh pass per tick. Zero or negative disables
	// interval polling, leaving notification driven passes only.
	Interval time.Duration
	// Metrics reports one server's resolved status per pass. status is a
	// storage status token ("fresh", "stale"); reason is empty when the server
	// advanced and the finite failure reason otherwise.
	Metrics func(serverID, status, reason string)
	// Published observes each generation a pass publishes, so the command layer
	// can report per-server catalog state without reading internals. It returns
	// reconciliation failures after the runtime has accepted that generation.
	Published func(assembly *Assembly) error
	// Logger receives structured lifecycle records. Logf remains the legacy
	// callback for post-accept publication failures.
	Logger *observe.Logger
	Logf   func(format string, args ...any)
}

// Refresher drives periodic and notification triggered refresh passes for one
// runtime. Coalescing is part of its contract: at most one pass runs at a time,
// and a trigger storm during a pass collapses into exactly one pending pass
// (plan §4.7 "one active + one pending").
type Refresher struct {
	runtime   *Runtime
	current   func() *config.Config
	deps      RefreshDeps
	interval  time.Duration
	metrics   func(serverID, status, reason string)
	published func(assembly *Assembly) error
	rejected  func(err error)
	logger    *observe.Logger
	logf      func(format string, args ...any)

	pending chan struct{}
	running atomic.Bool

	mu     sync.Mutex
	queued int
	passes atomic.Int64
}

// NewRefresher returns a refresher over one runtime. current supplies the
// configuration the current published generation was built from, so a pass
// re-resolves exactly the live membership.
func NewRefresher(runtime *Runtime, current func() *config.Config, deps RefreshDeps, options RefresherOptions) *Refresher {
	deps.Reason = normalizedReason(deps.Reason)
	return &Refresher{
		runtime:   runtime,
		current:   current,
		deps:      deps,
		interval:  options.Interval,
		metrics:   options.Metrics,
		published: options.Published,
		rejected:  deps.OnRejected,
		logger:    options.Logger,
		logf:      options.Logf,
		pending:   make(chan struct{}, 1),
	}
}

// normalizedReason guarantees the recorded reason stays inside the finite
// vocabulary even when a caller supplies no classifier.
func normalizedReason(classify func(server config.Server, err error) string) func(config.Server, error) string {
	return func(server config.Server, err error) string {
		reason := ""
		if classify != nil {
			reason = classify(server, err)
		} else {
			var reasoned interface{ RefreshReason() string }
			if errors.As(err, &reasoned) {
				reason = reasoned.RefreshReason()
			}
		}
		switch reason {
		case RefreshReasonUnavailable, RefreshReasonDiscoveryFailed, RefreshReasonClientMissing:
			return reason
		default:
			return RefreshReasonDiscoveryFailed
		}
	}
}

// Trigger requests a refresh pass. It never blocks: a request raised while a pass
// is running or already pending coalesces into the one pending pass, and any
// further requests in the same window are dropped.
func (r *Refresher) Trigger() {
	r.mu.Lock()
	r.queued++
	r.mu.Unlock()
	select {
	case r.pending <- struct{}{}:
	default:
		r.mu.Lock()
		r.queued--
		r.mu.Unlock()
	}
}

// Pending reports whether one pass is queued behind a running pass.
func (r *Refresher) Pending() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.queued > 0
}

// Running reports whether a pass is executing.
func (r *Refresher) Running() bool {
	return r.running.Load()
}

// Passes reports how many passes have completed, which is the observable the
// coalescing contract is stated against.
func (r *Refresher) Passes() int64 {
	return r.passes.Load()
}

// Run drives the refresher until ctx is cancelled. Failures never stop it: it
// keeps the last known good generation and tries again on the next trigger or
// tick. Disabled polling leaves Run waiting on triggers only.
func (r *Refresher) Run(ctx context.Context) {
	ticks := r.ticks()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			r.runPassFor("interval")
		case <-r.pending:
			r.mu.Lock()
			r.queued--
			r.mu.Unlock()
			r.runPassFor("trigger")
		}
	}
}

func (r *Refresher) ticks() <-chan time.Time {
	if r.interval <= 0 {
		return nil
	}
	return time.NewTicker(r.interval).C
}

// runPass executes one notification-triggered refresh pass for callers that do
// not have a loop source available.
func (r *Refresher) runPass() {
	r.runPassFor("trigger")
}

type refreshPassCounts struct {
	servers      int
	tools        int
	pinsResolved int
}

func (r *Refresher) runPassFor(trigger string) {
	r.running.Store(true)
	defer r.running.Store(false)
	cfg := r.current()
	if cfg == nil {
		return
	}

	refreshID := observe.CorrelationID("")
	revision := cfg.Fingerprint()
	r.logLifecycle(observe.Entry{
		Level:     "info",
		Event:     "catalog_refresh_started",
		Operation: "refresh",
		Outcome:   "unknown",
		Fields:    refreshLifecycleFields(refreshID, revision, trigger, r.nextAttemptMillis()),
	})

	counts := refreshPassCounts{}
	deps := r.deps
	var discoveryFailure error
	if discover := deps.Discover; discover != nil {
		deps.Discover = func(server config.Server) ([]core.ToolDefinition, error) {
			definitions, err := discover(server)
			if err == nil {
				counts.servers++
				counts.tools += len(definitions)
				for _, pin := range cfg.Pins {
					if pin["server"] != server.ID {
						continue
					}
					for _, definition := range definitions {
						if definition.RawName == pin["tool"] {
							counts.pinsResolved++
							break
						}
					}
				}
			} else if discoveryFailure == nil {
				discoveryFailure = err
			}
			return definitions, err
		}
	}
	assembly, err := r.runtime.Refresh(cfg, deps)
	r.report(assembly, err)
	switch {
	case err != nil:
		r.logRefreshFailure(cfg, refreshID, revision, trigger, counts, err, "candidate_rejected")
	case discoveryFailure != nil:
		r.logRefreshFailure(cfg, refreshID, revision, trigger, counts, discoveryFailure, "degraded_published")
	default:
		r.logRefreshCompletion(cfg, refreshID, revision, trigger, counts, assembly)
	}
	r.passes.Add(1)
}

func (r *Refresher) nextAttemptMillis() int64 {
	if r.interval <= 0 {
		return 0
	}
	return r.interval.Milliseconds()
}

func refreshLifecycleFields(refreshID, revision, trigger string, nextAttempt int64) map[string]any {
	return map[string]any{
		"refresh_id":      refreshID,
		"config_revision": revision,
		"trigger":         trigger,
		"next_attempt_ms": nextAttempt,
	}
}

func (r *Refresher) logRefreshCompletion(cfg *config.Config, refreshID, revision, trigger string, counts refreshPassCounts, assembly *Assembly) {
	fields := refreshLifecycleFields(refreshID, revision, trigger, 0)
	delete(fields, "next_attempt_ms")
	fields["servers_discovered"] = counts.servers
	fields["tools_discovered"] = counts.tools
	fields["tools_published"] = len(assembly.Catalog().Bindings())
	fields["pins_resolved"] = len(assembly.Pins())
	fields["pins_total"] = len(cfg.Pins)
	r.logLifecycle(observe.Entry{
		Level:     "info",
		Event:     "catalog_refresh_completed",
		Operation: "refresh",
		Outcome:   "success",
		Fields:    fields,
	})
}

func (r *Refresher) logRefreshFailure(cfg *config.Config, refreshID, revision, trigger string, counts refreshPassCounts, err error, impact string) {
	errorInfo := refreshPassError(err)
	fields := refreshLifecycleFields(refreshID, revision, trigger, 0)
	delete(fields, "next_attempt_ms")
	fields["catalog_state"] = "none_available"
	fields["candidate_servers"] = counts.servers
	fields["candidate_tools"] = counts.tools
	fields["publication_impact"] = impact
	fields["pins_resolved"] = counts.pinsResolved
	fields["pins_total"] = len(cfg.Pins)
	level := "error"

	if r.runtime != nil {
		if retained := r.runtime.Current(); retained != nil && retained.Catalog() != nil {
			toolCount := len(retained.Catalog().Bindings())
			if toolCount > 0 {
				fields["catalog_state"] = "retained_published"
				fields["retained_generation"] = retained.Generation()
				fields["catalog_tools"] = toolCount
				fields["retained_age_ms"] = retainedAgeMillis(retained, time.Now())
				level = "warn"
			}
		}
	}
	if errorInfo.Retryable && r.interval > 0 {
		fields["next_attempt_ms"] = r.nextAttemptMillis()
	}
	r.logLifecycle(observe.Entry{
		Level:     level,
		Event:     "catalog_refresh_failed",
		Operation: "refresh",
		Outcome:   "failure",
		Error:     errorInfo,
		Fields:    fields,
	})
	r.logPinBlockerDetails(refreshID, err)
}

func (r *Refresher) logPinBlockerDetails(refreshID string, err error) {
	var pinErr *pinResolutionError
	if !errors.As(err, &pinErr) {
		return
	}
	for _, blocker := range pinErr.blockers {
		fields := map[string]any{
			"refresh_id":           refreshID,
			"config.pin_tool":      blocker.tool,
			"config.property_path": blocker.propertyPath,
			"blocker_class":        blocker.blockerClass,
			"publication_impact":   "candidate_rejected",
		}
		if blocker.failureReason != "" {
			fields["failure_reason"] = blocker.failureReason
		}
		r.logLifecycle(observe.Entry{
			Level:     "warn",
			Event:     "pin_blocker_detail",
			Server:    blocker.server,
			Operation: "refresh",
			Outcome:   "failure",
			Fields:    fields,
		})
	}
	r.logPinToolChunks(refreshID, pinErr)
}

func (r *Refresher) logPinToolChunks(refreshID string, pinErr *pinResolutionError) {
	servers := make([]string, 0, len(pinErr.discoveredTools))
	for serverID := range pinErr.discoveredTools {
		servers = append(servers, serverID)
	}
	sort.Strings(servers)
	for _, serverID := range servers {
		names := pinErr.discoveredTools[serverID]
		chunkCount := (len(names) + 63) / 64
		if chunkCount == 0 {
			chunkCount = 1
		}
		for chunkIndex := range chunkCount {
			start := chunkIndex * 64
			end := start + 64
			if end > len(names) {
				end = len(names)
			}
			r.logLifecycle(observe.Entry{
				Level:     "warn",
				Event:     "pin_blocker_detail",
				Server:    serverID,
				Operation: "refresh",
				Outcome:   "failure",
				Fields: map[string]any{
					"refresh_id":         refreshID,
					"blocker_class":      "tool_absent",
					"discovered_tools":   names[start:end],
					"chunk_index":        chunkIndex,
					"chunk_count":        chunkCount,
					"publication_impact": "candidate_rejected",
				},
			})
		}
	}
}

func refreshPassError(err error) *observe.ErrorInfo {
	var pinErr *pinResolutionError
	if errors.As(err, &pinErr) {
		return observe.RuntimeErrorInfo("pin_rejected")
	}
	var reasoned interface{ RefreshReason() string }
	if errors.As(err, &reasoned) {
		return observe.RuntimeErrorInfo(reasoned.RefreshReason())
	}
	return observe.RuntimeErrorInfo("refresh_failed")
}

func retainedAgeMillis(assembly *Assembly, now time.Time) int64 {
	var oldest time.Time
	for _, serverID := range assembly.ServerIDs() {
		health, ok := assembly.Health(serverID)
		if ok && !health.LastSuccessAt.IsZero() && (oldest.IsZero() || health.LastSuccessAt.Before(oldest)) {
			oldest = health.LastSuccessAt
		}
	}
	if oldest.IsZero() || now.Before(oldest) {
		return 0
	}
	return now.Sub(oldest).Milliseconds()
}

func (r *Refresher) logLifecycle(entry observe.Entry) {
	if r.logger != nil {
		r.logger.Log(entry)
	}
}

// report publishes one pass's per-server outcome to the metrics hook. A failed
// publication callback suppresses per-server metrics because the projection
// was not committed.
func (r *Refresher) report(assembly *Assembly, err error) {
	if err != nil && r.rejected != nil {
		r.rejected(err)
	}
	if err == nil && assembly != nil && r.published != nil {
		if publicationErr := r.published(assembly); publicationErr != nil {
			if r.logf != nil {
				r.logf("refresh publication failed after runtime acceptance: %v", fmt.Errorf("%w: %w", ErrPublicationFailed, publicationErr))
			}
			return
		}
	}
	if r.metrics == nil {
		return
	}
	report := assembly
	if report == nil {
		report = r.runtime.Current()
	}
	if report == nil {
		return
	}
	for _, server := range report.snapshot.Servers {
		if server.CatalogStatus == storage.StatusFresh {
			r.metrics(server.ServerID, storage.StatusFresh, "")
			continue
		}
		r.metrics(server.ServerID, server.CatalogStatus, server.Reason)
	}
}

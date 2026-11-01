package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
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
	// Logf reports a post-accept publication callback failure.
	Logf func(format string, args ...any)
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
		logf:      options.Logf,
		pending:   make(chan struct{}, 1),
	}
}

// normalizedReason guarantees the recorded reason stays inside the finite
// vocabulary even when a caller supplies no classifier.
func normalizedReason(classify func(server config.Server, err error) string) func(config.Server, error) string {
	if classify == nil {
		return func(config.Server, error) string { return RefreshReasonDiscoveryFailed }
	}
	return func(server config.Server, err error) string {
		reason := classify(server, err)
		if reason == "" {
			return RefreshReasonDiscoveryFailed
		}
		return reason
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
			r.runPass()
		case <-r.pending:
			r.mu.Lock()
			r.queued--
			r.mu.Unlock()
			r.runPass()
		}
	}
}

func (r *Refresher) ticks() <-chan time.Time {
	if r.interval <= 0 {
		return nil
	}
	return time.NewTicker(r.interval).C
}

// runPass executes exactly one refresh pass and retires its pending slot after
// the pass completes, so requests raised during it stay pending until the loop
// consumes them.
func (r *Refresher) runPass() {
	r.running.Store(true)
	defer r.running.Store(false)
	cfg := r.current()
	if cfg == nil {
		return
	}
	assembly, err := r.runtime.Refresh(cfg, r.deps)
	r.report(assembly, err)
	r.passes.Add(1)
}

// report publishes one pass's per-server outcome to the metrics hook. A failed
// publication callback suppresses per-server metrics because the projection
// was not committed.
func (r *Refresher) report(assembly *Assembly, err error) {
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

package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

// ErrPublicationFailed marks a publication callback failure after the runtime has accepted a generation.
var ErrPublicationFailed = errors.New("service: post-accept publication failed")

// LoopOptions composes the drivers one operator configuration implies: the
// candidate loader, the per-server collaborators, the catalog interval, and the
// SIGHUP channel an operator's signal handler feeds (plan §4.7/§4.10/§4.12).
type LoopOptions struct {
	// Load parses the configuration as it currently exists on disk. It is
	// called once at startup and once per signal.
	Load func() (*config.Config, error)
	// Durable supplies the validated snapshot of the previous run so startup
	// can serve the catalogs it already has; a nil function admits every
	// server with no catalog. A snapshot the store refuses is reported once
	// with the finite reason startup_snapshot_rejected and is never cached:
	// startup proceeds without cached state (plan §4.9, AC-13).
	Durable func() (*storage.Snapshot, error)
	// Deps are the reload and refresh collaborators.
	Deps RefreshDeps
	// Interval is the catalog refresh interval; zero disables interval polling.
	Interval time.Duration
	// Signals delivers one value per reload request (SIGHUP). A nil channel
	// disables signal-driven reloads.
	Signals <-chan struct{}
	// Metrics reports one server's status per refresh pass.
	Metrics func(serverID, status, reason string)
	// ReloadMetrics reports one reload outcome: "success", or
	// "rejected" with the finite failure reason.
	ReloadMetrics func(outcome, reason string)
	// Published reconciles and observes each accepted generation, including
	// startup admission. It returns post-accept reconciliation failures.
	Published func(assembly *Assembly) error
	// Logf records operator-facing diagnostics without carrying secret values.
	Logf func(format string, args ...any)
	// Logger receives structured lifecycle records for the refresh driver.
	Logger *observe.Logger
}

// Loop owns one process's configuration lifecycle: the startup admission,
// signal-driven atomic reloads, periodic refresh passes, and the drain of the
// published generation at shutdown. It never mutates the published generation
// itself; every change goes through one Runtime publication.
type Loop struct {
	runtime   *Runtime
	options   LoopOptions
	refresher *Refresher

	mu        sync.Mutex
	candidate *config.Config
	reloads   int
}

// NewLoop validates the loop's collaborators.
func NewLoop(runtime *Runtime, options LoopOptions) (*Loop, error) {
	if runtime == nil {
		return nil, errors.New("service: loop needs a runtime")
	}
	if options.Load == nil {
		return nil, errors.New("service: loop needs a configuration loader")
	}
	if options.Deps.Discover == nil || options.Deps.NewClient == nil || options.Deps.Now == nil {
		return nil, errors.New("service: loop needs discovery, client, and clock dependencies")
	}
	loop := &Loop{runtime: runtime, options: options}
	loop.refresher = NewRefresher(runtime, loop.currentConfig, options.Deps, RefresherOptions{
		Interval:  options.Interval,
		Metrics:   options.Metrics,
		Published: options.Published,
		Logf:      options.Logf,
		Logger:    options.Logger,
	})
	return loop, nil
}

// Reloader exposes the refresh driver's observables (running/pending/passes).
func (l *Loop) Refresher() *Refresher { return l.refresher }

// Reloads reports how many candidates this loop admitted since startup.
func (l *Loop) Reloads() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.reloads
}

// Reload admits the configuration the loader currently returns. A candidate that
// cannot be loaded or admitted leaves the published generation, its catalog, its
// clients, and the durable snapshot exactly as they were (AC-17). A post-accept
// publication failure returns an error without rolling back the accepted runtime.
func (l *Loop) Reload() error {
	candidate, err := l.options.Load()
	if err != nil {
		l.reportReload("rejected", "invalid_configuration")
		return fmt.Errorf("service: reload candidate: %w", err)
	}
	assembly, err := l.runtime.Reload(candidate, l.options.Deps.ReloadDeps)
	if err != nil {
		l.reportReload("rejected", "admission_failed")
		return err
	}
	l.mu.Lock()
	l.candidate = candidate
	l.reloads++
	l.mu.Unlock()
	if l.options.Published != nil {
		if err := l.options.Published(assembly); err != nil {
			return fmt.Errorf("%w: reload publication: %w", ErrPublicationFailed, err)
		}
	}
	l.reportReload("success", "")
	return nil
}

// Start admits the configuration on disk as one tolerant local generation and
// then drives independent reloads, refreshes and probes until ctx is cancelled,
// at which point the published generation is drained and Start returns.
//
// Startup never waits for every upstream (plan §4.7): each server publishes
// from the durable snapshot this process can reuse and is advanced by its own
// bounded probe, so an unreachable upstream leaves that server stale or empty
// while the rest of the bridge serves. A configuration that cannot be read or
// admitted is fatal; post-accept reconciliation failure is also fatal but does
// not roll back the accepted runtime generation.
func (l *Loop) Start(ctx context.Context) error {
	candidate, err := l.options.Load()
	if err != nil {
		l.reportReload("rejected", "invalid_configuration")
		return fmt.Errorf("service: startup configuration: %w", err)
	}
	var durable *storage.Snapshot
	if l.options.Durable != nil {
		durable, err = l.options.Durable()
		if err != nil {
			// A snapshot the store refuses is never trusted and never cached
			// (plan §4.9, AC-13): it is reported once with a finite reason and
			// startup proceeds with no cached state, exactly as when the store
			// holds no snapshot at all.
			l.logf("startup_snapshot_rejected: %v", err)
			durable = nil
		}
	}
	l.mu.Lock()
	l.candidate = candidate
	l.mu.Unlock()
	assembly, err := l.runtime.AdmitStartup(candidate, l.options.Deps.ReloadDeps, durable)
	if err != nil {
		l.reportReload("rejected", "admission_failed")
		return err
	}
	if l.options.Published != nil {
		if err := l.options.Published(assembly); err != nil {
			l.runtime.Drain(l.options.Deps.ReloadDeps)
			return fmt.Errorf("%w: startup publication: %w", ErrPublicationFailed, err)
		}
	}
	l.reportReload("success", "")
	// The local generation is already published; probe every server right
	// away instead of waiting for the first interval tick, so a reachable
	// upstream advances without an all-server barrier.
	l.refresher.Trigger()
	ctx, stop := context.WithCancel(ctx)
	defer stop()

	var drivers sync.WaitGroup
	drivers.Add(2)
	go func() {
		defer drivers.Done()
		l.refresher.Run(ctx)
	}()
	go func() {
		defer drivers.Done()
		l.watch(ctx)
	}()

	<-ctx.Done()
	drivers.Wait()
	l.runtime.Drain(l.options.Deps.ReloadDeps)
	return nil
}

// watch turns each signal delivery into exactly one reload attempt. A rejected
// candidate is reported and the loop keeps serving the published generation.
func (l *Loop) watch(ctx context.Context) {
	if l.options.Signals == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-l.options.Signals:
			if err := l.Reload(); err != nil {
				if errors.Is(err, ErrPublicationFailed) {
					l.logf("configuration reload publication failed after runtime acceptance: %v", err)
				} else {
					l.logf("configuration reload rejected: %v", err, l.reloadAttribution())
				}
				continue
			}
			l.logf("configuration reload applied", l.reloadAttribution())
		}
	}
}

// currentConfig is the configuration the published generation was admitted from.
func (l *Loop) currentConfig() *config.Config {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.candidate
}

func (l *Loop) reloadAttribution() map[string]string {
	candidate := l.currentConfig()
	if candidate == nil {
		return nil
	}
	return map[string]string{
		"config_revision": candidate.Fingerprint(),
		"trigger":         "trigger",
	}
}

func (l *Loop) logf(format string, args ...any) {
	if l.options.Logf == nil {
		return
	}
	l.options.Logf(format, args...)
}

func (l *Loop) reportReload(outcome, reason string) {
	if l.options.ReloadMetrics == nil {
		return
	}
	l.options.ReloadMetrics(outcome, reason)
}

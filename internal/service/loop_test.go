package service_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

// loopLog records the loop's operator-facing lines and reload admission
// outcomes, so a rejected candidate can be asserted on without inspecting
// stdout (the loop reports rejections to both surfaces).
type loopLog struct {
	mu       sync.Mutex
	lines    []string
	outcomes [][2]string
}

func (l *loopLog) logf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func (l *loopLog) reloadOutcome(outcome, reason string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.outcomes = append(l.outcomes, [2]string{outcome, reason})
}

// rejections counts the reload attempts the loop reported as rejected.
func (l *loopLog) rejections() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	count := 0
	for _, outcome := range l.outcomes {
		if outcome[0] == "rejected" {
			count++
		}
	}
	return count
}

// rejectionReasons returns the finite reasons reported with a rejected reload.
func (l *loopLog) rejectionReasons() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var reasons []string
	for _, outcome := range l.outcomes {
		if outcome[0] == "rejected" {
			reasons = append(reasons, outcome[1])
		}
	}
	return reasons
}

func (l *loopLog) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.lines)
}

// candidateHolder is the candidate the loop's loader reads, so a test can swap
// it between signals while the loop is running.
type candidateHolder struct {
	mu  sync.Mutex
	cfg *config.Config
}

func newCandidateHolder(cfg *config.Config) *candidateHolder {
	return &candidateHolder{cfg: cfg}
}

func (c *candidateHolder) set(cfg *config.Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg = cfg
}

func (c *candidateHolder) get() *config.Config {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg
}

// loopHarness builds a loop over the reload harness with a candidate the test
// controls between signals.
func loopHarness(t *testing.T, h *reloadHarness, candidate *candidateHolder, interval time.Duration) (*service.Loop, chan struct{}, *loopLog, *atomicError) {
	t.Helper()
	return loopHarnessWith(t, h, candidate, interval, nil)
}

// loopHarnessWith is loopHarness with the durable-snapshot loader under the
// test's control, so startup can be driven with a snapshot the store refuses.
func loopHarnessWith(t *testing.T, h *reloadHarness, candidate *candidateHolder, interval time.Duration, durable func() (*storage.Snapshot, error)) (*service.Loop, chan struct{}, *loopLog, *atomicError) {
	t.Helper()
	signals := make(chan struct{}, 1)
	log := &loopLog{}
	failure := &atomicError{}
	loop, err := service.NewLoop(h.runtime, service.LoopOptions{
		Load: func() (*config.Config, error) {
			if err := failure.get(); err != nil {
				return nil, err
			}
			return candidate.get(), nil
		},
		Durable:       durable,
		Deps:          refreshDeps(h),
		Interval:      interval,
		Signals:       signals,
		Logf:          log.logf,
		ReloadMetrics: log.reloadOutcome,
		Published:     h.recordPublished,
	})
	if err != nil {
		t.Fatalf("new loop: %v", err)
	}
	return loop, signals, log, failure
}

// atomicError hands the candidate loader a failure the test can switch on while
// the loop is running.
type atomicError struct {
	mu  sync.Mutex
	err error
}

func (a *atomicError) set(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.err = err
}

func (a *atomicError) get() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.err
}

// startLoop runs the loop and surfaces an early exit, so a rejected startup
// admission fails the test with its error instead of a bare timeout.
func startLoop(t *testing.T, ctx context.Context, loop *service.Loop) chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- loop.Start(ctx) }()
	select {
	case err := <-done:
		t.Fatalf("run loop exited during startup: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	return done
}

// TestLoopAdmitsTheStartupCandidateAndDrainsOnShutdown pins the run loop's
// lifecycle: the configuration on disk is admitted once at startup, and a
// cancelled context retires the published generation so every dedicated client
// closes exactly once (plan §4.7/§4.10).
func TestLoopAdmitsTheStartupCandidateAndDrainsOnShutdown(t *testing.T) {
	h := newReloadHarness(t)
	reloadEnv(t)
	candidate := newCandidateHolder(parseReloadConfig(t, reloadTwoServers))
	loop, _, _, _ := loopHarness(t, h, candidate, 0)

	ctx, cancel := context.WithCancel(context.Background())
	done := startLoop(t, ctx, loop)

	waitFor(t, "the startup admission", func() bool { return h.runtime.Current() != nil })
	// The first published generation is the tolerant local admission: with no
	// durable snapshot every server starts empty and unavailable, and the
	// immediate probe may already have advanced the published generation.
	first := h.publishedAssemblies()[0]
	if generation := first.Generation(); generation != 1 {
		t.Fatalf("first published generation = %d; want the local admission 1", generation)
	}
	health, ok := first.Health("alpha")
	if !ok {
		t.Fatal("the local admission lost a configured server")
	}
	if health.Status != storage.StatusNone || health.Availability != service.AvailabilityUnavailable {
		t.Fatalf("local alpha health = %+v; want none and unavailable before any probe", health)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run loop: %v", err)
	}
	waitFor(t, "both clients to drain", func() bool { return len(h.closedClients()) == 2 })
	time.Sleep(10 * time.Millisecond)
	if len(h.closedClients()) != 2 {
		t.Fatalf("closed = %d; want each client closed exactly once", len(h.closedClients()))
	}
}

// TestLoopKeepsThePublishedGenerationWhenTheCandidateIsRejected pins the SIGHUP
// barrier (AC-17): a candidate that cannot be admitted leaves the published
// generation, its catalog, its clients, and the durable snapshot untouched.
func TestLoopKeepsThePublishedGenerationWhenTheCandidateIsRejected(t *testing.T) {
	h := newReloadHarness(t)
	reloadEnv(t)
	candidate := newCandidateHolder(parseReloadConfig(t, reloadTwoServers))
	loop, signals, log, failure := loopHarness(t, h, candidate, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startLoop(t, ctx, loop)

	waitFor(t, "the startup admission", func() bool { return h.runtime.Current() != nil })
	waitFor(t, "the startup probe", func() bool { return loop.Refresher().Passes() > 0 })
	published := h.runtime.Current()
	persists := len(h.persisted())
	failure.set(errors.New("yaml: unknown key"))
	signals <- struct{}{}

	waitFor(t, "the rejected reload to be reported", func() bool { return log.count() > 0 })
	if h.runtime.Current() != published {
		t.Fatal("a rejected candidate replaced the published generation")
	}
	if generation := h.runtime.Current().Generation(); generation != published.Generation() {
		t.Fatalf("generation = %d; want the retained %d", generation, published.Generation())
	}
	if len(h.persisted()) != persists {
		t.Fatalf("persists = %d; want the rejected candidate to persist nothing", len(h.persisted()))
	}
	if len(h.closedClients()) != 0 {
		t.Fatalf("closed = %d; want no client churn on a rejected candidate", len(h.closedClients()))
	}
}

// TestLoopAdmitsASignalCandidateAndDrainsTheRemovedServer pins the atomic swap:
// one signal publishes exactly one new generation, a server the candidate
// dropped is drained, and a server it kept keeps its working client.
func TestLoopAdmitsASignalCandidateAndDrainsTheRemovedServer(t *testing.T) {
	h := newReloadHarness(t)
	reloadEnv(t)
	candidate := newCandidateHolder(parseReloadConfig(t, reloadTwoServers))
	loop, signals, _, _ := loopHarness(t, h, candidate, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startLoop(t, ctx, loop)
	waitFor(t, "the startup admission", func() bool { return h.runtime.Current() != nil })
	waitFor(t, "the startup probe", func() bool { return loop.Refresher().Passes() > 0 })
	before := h.runtime.Current().Generation()

	candidate.set(parseReloadConfig(t, reloadAlphaOnly))
	signals <- struct{}{}

	waitFor(t, "the trimmed generation", func() bool {
		current := h.runtime.Current()
		return current != nil && current.Generation() == before+1
	})
	if ids := h.runtime.Current().ServerIDs(); len(ids) != 1 || ids[0] != "alpha" {
		t.Fatalf("servers = %v; want only alpha", ids)
	}
	waitFor(t, "the removed server's client to drain", func() bool { return len(h.closedClients()) == 1 })
	time.Sleep(10 * time.Millisecond)
	closed := h.closedClients()
	if len(closed) != 1 || closed[0].name != "beta" {
		t.Fatalf("closed = %v; want exactly the removed beta client", closed)
	}
}

// TestLoopRefreshesOnItsInterval pins that the configured catalog interval
// drives refresh passes without any signal, so an upstream catalog change is
// observed without an operator action (plan §4.7).
func TestLoopRefreshesOnItsInterval(t *testing.T) {
	h := newReloadHarness(t)
	reloadEnv(t)
	candidate := newCandidateHolder(parseReloadConfig(t, reloadTwoServers))
	loop, _, _, _ := loopHarness(t, h, candidate, 5*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go loop.Start(ctx)
	waitFor(t, "a periodic refresh pass", func() bool { return loop.Refresher().Passes() > 0 })
	if generation := h.runtime.Current().Generation(); generation < 2 {
		t.Fatalf("generation = %d; want the startup generation plus a refresh", generation)
	}
	if len(h.closedClients()) != 0 {
		t.Fatalf("closed = %d; want a refresh to reuse every working client", len(h.closedClients()))
	}
}

// TestLoopReusesClientsAcrossIdenticalReloads pins that repeated signals of an
// unchanged membership publish one generation each without churning clients,
// so an operator's SIGHUP cannot drop live upstream sessions.
func TestLoopReusesClientsAcrossIdenticalReloads(t *testing.T) {
	h := newReloadHarness(t)
	reloadEnv(t)
	candidate := newCandidateHolder(parseReloadConfig(t, reloadTwoServers))
	loop, signals, _, _ := loopHarness(t, h, candidate, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startLoop(t, ctx, loop)
	waitFor(t, "the startup admission", func() bool { return h.runtime.Current() != nil })
	waitFor(t, "the startup probe", func() bool { return loop.Refresher().Passes() > 0 })

	for want := h.runtime.Current().Generation() + 1; ; want++ {
		signals <- struct{}{}
		target := want
		waitFor(t, "the next identical generation", func() bool {
			current := h.runtime.Current()
			return current != nil && current.Generation() == target
		})
		if target == 4 {
			break
		}
	}
	if len(h.closedClients()) != 0 {
		t.Fatalf("closed = %d; want every client reused across identical reloads", len(h.closedClients()))
	}
}

// TestLoopSignalCandidateIsRejectedWhileAServerIsUnreachable pins the asymmetric
// barrier: a reload candidate is strict, so an unreachable server rejects the
// whole candidate and the published generation survives (AC-17).
func TestLoopSignalCandidateIsRejectedWhileAServerIsUnreachable(t *testing.T) {
	h := newReloadHarness(t)
	reloadEnv(t)
	candidate := newCandidateHolder(parseReloadConfig(t, reloadTwoServers))
	loop, signals, log, _ := loopHarness(t, h, candidate, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startLoop(t, ctx, loop)
	waitFor(t, "the startup admission", func() bool { return h.runtime.Current() != nil })
	waitFor(t, "the startup probe", func() bool { return loop.Refresher().Passes() > 0 })
	published := h.runtime.Current()
	retained := published.Generation()

	h.failDiscover["beta"] = errors.New("upstream down")
	signals <- struct{}{}

	waitFor(t, "the rejected reload", func() bool {
		return log.rejections() == 1 && h.runtime.Current() == published
	})
	if reasons := log.rejectionReasons(); len(reasons) != 1 || reasons[0] != "admission_failed" {
		t.Fatalf("rejection reasons = %v; want exactly admission_failed", reasons)
	}
	if generation := h.runtime.Current().Generation(); generation != retained {
		t.Fatalf("generation = %d; want the retained %d", generation, retained)
	}
}

// TestLoopStartsWithoutTheCachedStateOfARejectedDurableSnapshot pins the
// storage contract at the loop boundary (plan §4.9, AC-13): a snapshot the store
// refuses is never trusted and never fatal. It is reported once through the
// operator hook with a finite reason and without counting a reload admission,
// and startup proceeds exactly as with no snapshot at all, so the immediate
// probe repopulates every server.
func TestLoopStartsWithoutTheCachedStateOfARejectedDurableSnapshot(t *testing.T) {
	reloadEnv(t)
	h := newReloadHarness(t)
	candidate := newCandidateHolder(parseReloadConfig(t, reloadTwoServers))
	corrupt := fmt.Errorf("storage: %w: decode: invalid character", storage.ErrCorrupt)
	loop, _, log, _ := loopHarnessWith(t, h, candidate, 0, func() (*storage.Snapshot, error) {
		return nil, corrupt
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := startLoop(t, ctx, loop)
	waitFor(t, "the startup admission", func() bool { return h.runtime.Current() != nil })

	first := h.publishedAssemblies()[0]
	if generation := first.Generation(); generation != 1 {
		t.Fatalf("first published generation = %d; want the local admission 1", generation)
	}
	if rows := first.Snapshot().Tools; len(rows) != 0 {
		t.Fatalf("startup kept %d tool rows of a refused snapshot; want none", len(rows))
	}
	health, ok := first.Health("alpha")
	if !ok {
		t.Fatal("the refused-snapshot admission lost a configured server")
	}
	if health.Status != storage.StatusNone || health.Availability != service.AvailabilityUnavailable {
		t.Fatalf("alpha health = %+v; want none and unavailable before any probe", health)
	}
	if count := log.count(); count != 1 {
		t.Fatalf("operator lines = %d; want exactly the refused snapshot line: %v", count, log.lines)
	}
	if line := log.lines[0]; !strings.Contains(line, "startup_snapshot_rejected") {
		t.Fatalf("operator line %q; want the finite reason startup_snapshot_rejected", line)
	}
	if got := log.rejections(); got != 0 {
		t.Fatalf("reload rejections = %d; want 0: the startup admission still succeeded", got)
	}

	waitFor(t, "the startup probe to repopulate the catalog", func() bool {
		return h.runtime.Current().Generation() > 1
	})
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Start returned %v; want nil on context cancellation", err)
	}
}

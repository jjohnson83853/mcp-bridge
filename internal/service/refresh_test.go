package service_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

// refreshDeps adds the refresh-only classifier to the reload harness so one
// server's outage never rejects the whole pass (plan §4.7).
func refreshDeps(h *reloadHarness) service.RefreshDeps {
	return service.RefreshDeps{
		ReloadDeps: h.deps(),
		Reason:     func(config.Server, error) string { return "upstream_unavailable" },
	}
}

// advancingClock moves the harness clock forward by one minute per call, so a
// pass's attempt and success times are strictly newer than the previous pass's.
func advancingClock(h *reloadHarness) {
	base := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	var step atomic.Int64
	h.now = func() time.Time { return base.Add(time.Duration(step.Add(1)) * time.Minute) }
}

func refreshConfig(t *testing.T, input string) *config.Config {
	t.Helper()
	reloadEnv(t)
	return parseReloadConfig(t, input)
}

func bindingNames(t *testing.T, assembly *service.Assembly) []string {
	t.Helper()
	names := make([]string, 0, len(assembly.Catalog().Bindings()))
	for _, binding := range assembly.Catalog().Bindings() {
		names = append(names, binding.ServerID+"/"+binding.RawName)
	}
	return names
}

// TestRefreshKeepsLastKnownGoodForFailedServer pins the T-07 refresh contract:
// a healthy server publishes its new catalog, a failing server keeps its
// COMPLETE last known good catalog as stale with a finite reason and its
// catalog age, one pointer swap publishes both, and the superseded generation is
// never mutated (plan §4.7).
func TestRefreshKeepsLastKnownGoodForFailedServer(t *testing.T) {
	h := newReloadHarness(t)
	advancingClock(h)
	cfg := refreshConfig(t, reloadTwoServers)

	first, err := h.runtime.Reload(cfg, h.deps())
	if err != nil {
		t.Fatalf("initial reload: %v", err)
	}
	if got := bindingNames(t, first); len(got) != 2 {
		t.Fatalf("initial bindings = %v; want the two configured servers", got)
	}

	h.mu.Lock()
	h.defs["alpha"] = append(h.defs["alpha"], core.ToolDefinition{
		RawName:     "read2",
		Description: "alpha second read",
	})
	h.failDiscover["beta"] = errors.New("beta refused the session")
	h.mu.Unlock()

	next, err := h.runtime.Refresh(cfg, refreshDeps(h))
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if next.Generation() != first.Generation()+1 {
		t.Fatalf("generation = %d; want %d", next.Generation(), first.Generation()+1)
	}
	if next.CatalogRevision() == first.CatalogRevision() {
		t.Fatal("revision did not change after a catalog change")
	}
	if got := bindingNames(t, next); len(got) != 3 {
		t.Fatalf("bindings = %v; want alpha's two tools plus beta's retained tool", got)
	}
	if _, ok := next.Catalog().Binding(core.PublicName("beta", "write")); !ok {
		t.Fatal("the failing server lost its last known good tool")
	}
	if _, ok := first.Catalog().Binding(core.PublicName("alpha", "read2")); ok {
		t.Fatal("the superseded generation was mutated by a refresh")
	}

	alpha, ok := next.Health("alpha")
	if !ok {
		t.Fatal("healthy server has no health row")
	}
	if alpha.Status != storage.StatusFresh || alpha.Reason != "" {
		t.Fatalf("alpha health = %+v; want fresh with no reason", alpha)
	}
	if alpha.Availability != service.AvailabilityAvailable {
		t.Fatalf("alpha availability = %q; want available", alpha.Availability)
	}
	beta, ok := next.Health("beta")
	if !ok {
		t.Fatal("failing server has no health row")
	}
	if beta.Status != storage.StatusStale || beta.Reason != "upstream_unavailable" {
		t.Fatalf("beta health = %+v; want stale with the classified reason", beta)
	}
	if beta.Availability != service.AvailabilityUnavailable {
		t.Fatalf("beta availability = %q; want unavailable (its own probe failed, so calls are refused)", beta.Availability)
	}
	if beta.LastSuccessAt.IsZero() || !beta.LastSuccessAt.Equal(firstSuccessAt(t, h, first, "beta")) {
		t.Fatalf("beta last success = %s; want the previous complete success %s",
			beta.LastSuccessAt, firstSuccessAt(t, h, first, "beta"))
	}
	if stale, ok := next.Health("beta"); ok && !stale.LastSuccessAt.Before(alpha.LastSuccessAt) {
		t.Fatalf("beta last success %s did not age behind alpha's %s", stale.LastSuccessAt, alpha.LastSuccessAt)
	}
}

// firstSuccessAt reads the durable row's last success from a published
// generation.
func firstSuccessAt(t *testing.T, _ *reloadHarness, assembly *service.Assembly, serverID string) time.Time {
	t.Helper()
	for _, row := range assembly.Snapshot().Servers {
		if row.ServerID == serverID {
			return row.LastSuccessAt
		}
	}
	t.Fatalf("snapshot has no row for %q", serverID)
	return time.Time{}
}

// TestRefreshPublishesNothingWhenAConfiguredClientIsMissing pins that a refresh
// never creates or rotates a client: a membership or credential change belongs
// to an atomic reload, so the pass fails closed and the published generation,
// its catalog, and its clients stay exactly as they were (plan §4.10).
func TestRefreshPublishesNothingWhenAConfiguredClientIsMissing(t *testing.T) {
	h := newReloadHarness(t)
	cfg := refreshConfig(t, reloadAlphaOnly)

	first, err := h.runtime.Reload(cfg, h.deps())
	if err != nil {
		t.Fatalf("initial reload: %v", err)
	}
	persists := len(h.persists)

	t.Setenv("RELOAD_ALPHA_ROTATED", "rotated-secret")
	rotated := refreshConfig(t, strings.Replace(reloadAlphaOnly, "RELOAD_ALPHA_SECRET", "RELOAD_ALPHA_ROTATED", 1))

	if _, err := h.runtime.Refresh(rotated, refreshDeps(h)); err == nil {
		t.Fatal("refresh published a generation for rotated credentials")
	}
	if h.runtime.Current() != first {
		t.Fatal("the published generation changed after a failed refresh")
	}
	if len(h.persists) != persists {
		t.Fatalf("persists = %d; want the failed refresh to persist nothing", len(h.persists))
	}
	if h.runtime.Current().CatalogRevision() != first.CatalogRevision() {
		t.Fatal("the published catalog revision changed after a failed refresh")
	}
}

// TestRefreshKeepsRevisionStableForAnUnchangedCatalog pins that a refresh which
// observes identical definitions keeps the same catalog revision, so an
// expected revision never churns without a real catalog change (plan §4.5).
func TestRefreshKeepsRevisionStableForAnUnchangedCatalog(t *testing.T) {
	h := newReloadHarness(t)
	cfg := refreshConfig(t, reloadTwoServers)

	first, err := h.runtime.Reload(cfg, h.deps())
	if err != nil {
		t.Fatalf("initial reload: %v", err)
	}
	next, err := h.runtime.Refresh(cfg, refreshDeps(h))
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if next.CatalogRevision() != first.CatalogRevision() {
		t.Fatalf("revision = %s; want the unchanged %s", next.CatalogRevision(), first.CatalogRevision())
	}
	if next.Generation() != first.Generation()+1 {
		t.Fatalf("generation = %d; want %d", next.Generation(), first.Generation()+1)
	}
}

// TestRefreshPreservesLKGPinsForFailedServer pins the LKG pin contract: when a
// pinned server's discovery is temporarily unreachable, the refresh keeps the
// last-known-good catalog entry for its pinned tool, preserves the binding,
// marks the server stale, and refuses to acquire it so execution fails closed.
func TestRefreshPreservesLKGPinsForFailedServer(t *testing.T) {
	h := newReloadHarness(t)
	advancingClock(h)
	pinnedYAML := strings.Replace(reloadTwoServers, "pins: []", "pins:\n  - {server: beta, tool: write}", 1)
	cfg := refreshConfig(t, pinnedYAML)

	first, err := h.runtime.Reload(cfg, h.deps())
	if err != nil {
		t.Fatalf("initial reload: %v", err)
	}
	if len(first.Pins()) != 1 {
		t.Fatalf("initial pins = %d; want 1", len(first.Pins()))
	}

	h.mu.Lock()
	h.failDiscover["beta"] = errors.New("beta connection refused")
	h.mu.Unlock()

	next, err := h.runtime.Refresh(cfg, refreshDeps(h))
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if len(next.Pins()) != 1 {
		t.Fatalf("next pins = %d; want 1", len(next.Pins()))
	}
	if next.Pins()[0].PinnedTool.RawName != "write" {
		t.Fatalf("next pinned tool = %q; want write", next.Pins()[0].PinnedTool.RawName)
	}
	if next.Pins()[0].Binding == nil {
		t.Fatal("next pin has no binding; want the LKG binding preserved")
	}
	if !next.PinnedNames()[core.PublicName("beta", "write")] {
		t.Fatalf("next pinned names = %v; want to contain beta/write", next.PinnedNames())
	}
	betaHealth, ok := next.Health("beta")
	if !ok {
		t.Fatal("next health has no entry for beta")
	}
	if betaHealth.Status != storage.StatusStale {
		t.Fatalf("beta health = %s; want %s", betaHealth.Status, storage.StatusStale)
	}
	if lease, acquired := next.Acquire("beta"); acquired || lease != nil {
		t.Fatalf("Acquire(beta) = (%v, %t); want a fail-closed nil lease", lease, acquired)
	}
}

// gatedDiscovery blocks the first discovery so a trigger storm can be aimed at
// an in-flight refresh pass.
type gatedDiscovery struct {
	inner   func(config.Server) ([]core.ToolDefinition, error)
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func (g *gatedDiscovery) discover(server config.Server) ([]core.ToolDefinition, error) {
	if g.calls.Add(1) == 1 {
		g.entered <- struct{}{}
		<-g.release
	}
	return g.inner(server)
}

// TestRefresherCoalescesTriggerStormIntoOneActiveAndOnePending pins the §4.7
// notification contract: however many triggers arrive while a pass runs, at most
// one further pass is queued and it runs exactly once.
func TestRefresherCoalescesTriggerStormIntoOneActiveAndOnePending(t *testing.T) {
	h := newReloadHarness(t)
	cfg := refreshConfig(t, reloadAlphaOnly)
	if _, err := h.runtime.Reload(cfg, h.deps()); err != nil {
		t.Fatalf("initial reload: %v", err)
	}

	gate := &gatedDiscovery{
		inner:   refreshDeps(h).Discover,
		entered: make(chan struct{}, 8),
		release: make(chan struct{}),
	}
	deps := refreshDeps(h)
	deps.Discover = gate.discover

	refresher := service.NewRefresher(h.runtime, func() *config.Config { return cfg }, deps,
		service.RefresherOptions{Interval: time.Hour})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		refresher.Run(ctx)
	}()
	defer func() {
		cancel()
		<-done
	}()

	refresher.Trigger()
	<-gate.entered
	waitFor(t, "the first refresh pass to run", func() bool { return refresher.Running() })

	for range 5 {
		refresher.Trigger()
	}
	if !refresher.Pending() {
		t.Fatal("a trigger storm was dropped instead of coalescing into one pending pass")
	}
	if gate.calls.Load() != 1 {
		t.Fatalf("discovery calls = %d while the first pass runs; want 1", gate.calls.Load())
	}

	close(gate.release)
	waitFor(t, "the coalesced pass to finish", func() bool {
		return !refresher.Running() && !refresher.Pending()
	})
	if calls := gate.calls.Load(); calls != 2 {
		t.Fatalf("discovery calls = %d; want exactly 2 (one active plus one coalesced)", calls)
	}
}

// waitFor polls a condition until it holds or the test deadline expires.
func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

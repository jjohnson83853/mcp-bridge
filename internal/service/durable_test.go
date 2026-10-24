package service_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/search"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

const dimensions = 1024

// T-07 cold-start RED gate (r8 plan §4.5/§4.9/§4.10, AC-13/AC-14/AC-15/AC-16):
// a startup admission reconciles against the durable snapshot of the previous
// run, so it serves that run's exact catalog and current vectors, tombstones a
// server the config has since dropped, and invalidates vectors whose model
// identity changed.

const durableStaleDigest = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

// durableServer is one indexed server of the durable snapshot: the binding and
// health row plus the exact tool and embedding rows an indexing pass stores.
type durableServer struct {
	serverID    string
	def         core.ToolDefinition
	fingerprint string
	status      string
	digest      string
}

func alphaDurableServer(digest string) durableServer {
	return durableServer{
		serverID:    "alpha",
		def:         core.ToolDefinition{RawName: "read", Description: "alpha read", InputSchema: json.RawMessage(`{"type":"object"}`)},
		fingerprint: strings.Repeat("a", 64),
		status:      storage.StatusFresh,
		digest:      digest,
	}
}

func betaDurableServer(digest string) durableServer {
	return durableServer{
		serverID:    "beta",
		def:         core.ToolDefinition{RawName: "write", Description: "beta write", InputSchema: json.RawMessage(`{"type":"object"}`)},
		fingerprint: strings.Repeat("b", 64),
		status:      storage.StatusFresh,
		digest:      digest,
	}
}

// durableSnapshot is the validated snapshot of a previous run whose catalog
// and vectors survived the restart (plan §4.9). It starts from a published
// snapshot so every identity is the real one the runtime computed, and keeps
// the endpoint fingerprint of a server the run already configured. The named
// servers' indexed rows are REPLACED the way an indexing pass replaces them, so
// a server that already has rows in the base snapshot is re-indexed rather than
// duplicated.
func durableSnapshot(t *testing.T, base *storage.Snapshot, servers ...durableServer) *storage.Snapshot {
	t.Helper()
	bindings := map[string]storage.Binding{}
	for _, row := range base.Bindings {
		bindings[row.ServerID] = row
	}
	health := map[string]storage.Server{}
	for _, row := range base.Servers {
		health[row.ServerID] = row
	}
	tools := make([]storage.Tool, 0, len(base.Tools)+len(servers))
	embeddings := make([]storage.Embedding, 0, len(base.Embeddings)+len(servers))
	replaced := map[string]bool{}
	for _, server := range servers {
		replaced[server.serverID] = true
	}
	dropped := map[string]bool{}
	for _, row := range base.Tools {
		if replaced[row.ServerID] {
			dropped[storage.ToolIdentity(row.ServerID, row.RawName)] = true
			continue
		}
		tools = append(tools, row)
	}
	for _, row := range base.Embeddings {
		if !dropped[row.ToolIdentity] {
			embeddings = append(embeddings, row)
		}
	}
	order := make([]string, 0, len(base.Bindings)+len(servers))
	for _, row := range base.Bindings {
		order = append(order, row.ServerID)
	}
	for _, server := range servers {
		fingerprint := server.fingerprint
		if row, ok := bindings[server.serverID]; ok {
			fingerprint = row.EndpointFingerprint
		} else {
			order = append(order, server.serverID)
		}
		bindings[server.serverID] = storage.Binding{ServerID: server.serverID, EndpointFingerprint: fingerprint}
		health[server.serverID] = storage.Server{
			ServerID:            server.serverID,
			EndpointFingerprint: fingerprint,
			NegotiatedVersion:   "2025-06-18",
			CatalogStatus:       server.status,
		}
		contentHash := search.ContentHash(server.def.RawName, server.def.Description)
		tools = append(tools, storage.Tool{
			ServerID:             server.serverID,
			RawName:              server.def.RawName,
			PublicName:           core.PublicName(server.serverID, server.def.RawName),
			Description:          server.def.Description,
			DefinitionHash:       strings.Repeat("3", 64),
			EmbeddingContentHash: contentHash,
			InputSchema:          server.def.InputSchema,
		})
		embeddings = append(embeddings, storage.Embedding{
			ToolIdentity:        storage.ToolIdentity(server.serverID, server.def.RawName),
			ContentHash:         contentHash,
			ModelIdentityDigest: server.digest,
			Dimensions:          dimensions,
			FormatVersion:       search.FormatVersion,
			Vector:              durableVector(),
		})
	}
	snapshot := *base
	snapshot.Bindings = make([]storage.Binding, 0, len(order))
	snapshot.Servers = make([]storage.Server, 0, len(order))
	for _, serverID := range order {
		snapshot.Bindings = append(snapshot.Bindings, bindings[serverID])
		snapshot.Servers = append(snapshot.Servers, health[serverID])
	}
	snapshot.Tools = tools
	snapshot.Embeddings = embeddings
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("durable snapshot fixture: %v", err)
	}
	return &snapshot
}

// publishedSnapshot is the durable snapshot a first run persists, so the
// cold-start fixture carries the real config fingerprint and catalog revision.
func publishedSnapshot(t *testing.T) *storage.Snapshot {
	t.Helper()
	reloadEnv(t)
	first := newReloadHarness(t)
	cfg := parseReloadConfig(t, reloadTwoServers)
	if _, err := first.runtime.Reload(cfg, first.deps()); err != nil {
		t.Fatalf("seed Reload: %v", err)
	}
	persisted := first.persisted()
	if len(persisted) == 0 {
		t.Fatal("seed Reload persisted no snapshot")
	}
	return persisted[len(persisted)-1]
}

func snapshotTools(snapshot *storage.Snapshot, serverID string) []storage.Tool {
	var rows []storage.Tool
	for _, row := range snapshot.Tools {
		if row.ServerID == serverID {
			rows = append(rows, row)
		}
	}
	return rows
}

// AC-13: a cold start serves the previous run's exact catalog and its current
// vectors, and reports both servers as restored-but-not-yet-probed.
func TestStartupServesTheDurableCatalogAndKeepsCurrentVectors(t *testing.T) {
	base := publishedSnapshot(t)
	durable := durableSnapshot(t, base,
		alphaDurableServer(strings.Repeat("a", 64)),
		betaDurableServer(strings.Repeat("a", 64)),
	)

	cold := newReloadHarness(t)
	cfg := parseReloadConfig(t, reloadTwoServers)
	assembly, err := cold.runtime.AdmitStartup(cfg, cold.deps(), durable)
	if err != nil {
		t.Fatalf("AdmitStartup: %v", err)
	}

	if got := assembly.CatalogRevision(); got != durable.Envelope.CatalogRevision {
		t.Fatalf("startup catalog revision = %s; want the durable %s", got, durable.Envelope.CatalogRevision)
	}
	for _, serverID := range []string{"alpha", "beta"} {
		rawName := map[string]string{"alpha": "read", "beta": "write"}[serverID]
		if _, ok := assembly.Catalog().Binding(core.PublicName(serverID, rawName)); !ok {
			t.Fatalf("startup catalog does not serve %s/%s", serverID, rawName)
		}
		if got := len(snapshotTools(assembly.Snapshot(), serverID)); got != 1 {
			t.Fatalf("startup snapshot kept %d tool rows for %s; want 1", got, serverID)
		}
		health, ok := assembly.Health(serverID)
		if !ok {
			t.Fatalf("startup snapshot has no health row for %s", serverID)
		}
		if health.Status != storage.StatusStale || health.Reason == "" {
			t.Fatalf("startup health for %s = %q/%q; want a stale reason", serverID, health.Status, health.Reason)
		}
	}
	if got := len(assembly.Snapshot().Embeddings); got != 2 {
		t.Fatalf("startup snapshot kept %d current vectors; want 2", got)
	}
}

// AC-15/AC-16: a server the config has since dropped is purged to unreachable
// and tombstoned at startup, so it never rebinds and its cache never returns.
func TestStartupTombstonesAServerTheConfigDropped(t *testing.T) {
	base := publishedSnapshot(t)
	dropped := durableServer{
		serverID:    "gone",
		def:         core.ToolDefinition{RawName: "read", Description: "gone read", InputSchema: json.RawMessage(`{"type":"object"}`)},
		fingerprint: strings.Repeat("c", 64),
		status:      storage.StatusFresh,
		digest:      strings.Repeat("a", 64),
	}
	durable := durableSnapshot(t, base, alphaDurableServer(strings.Repeat("a", 64)), dropped)

	cold := newReloadHarness(t)
	cfg := parseReloadConfig(t, reloadTwoServers)
	assembly, err := cold.runtime.AdmitStartup(cfg, cold.deps(), durable)
	if err != nil {
		t.Fatalf("AdmitStartup: %v", err)
	}

	for _, serverID := range assembly.ServerIDs() {
		if serverID == "gone" {
			t.Fatal("startup kept a health row for the dropped server")
		}
	}
	if rows := snapshotTools(assembly.Snapshot(), "gone"); len(rows) != 0 {
		t.Fatalf("startup kept %d tool rows of the dropped server; want 0", len(rows))
	}
	if _, ok := assembly.Health("gone"); ok {
		t.Fatal("startup reported health for the dropped server")
	}
	var tombstoned bool
	for _, binding := range assembly.Snapshot().Bindings {
		if binding.ServerID == "gone" && binding.Retired {
			tombstoned = true
		}
	}
	if !tombstoned {
		t.Fatal("startup did not tombstone the dropped server")
	}
	// The tombstone must outlive the restart: a config that puts the server
	// back refuses to rebind it rather than reviving its cache.
	replayed := parseReloadConfig(t, reloadTwoServers)
	if _, err := cold.runtime.Reload(replayed, cold.deps()); err != nil {
		t.Fatalf("Reload after startup: %v", err)
	}
	if _, err := cold.runtime.AdmitStartup(parseReloadConfig(t, reloadTwoServers), cold.deps(), assembly.Snapshot()); err != nil {
		t.Fatalf("second AdmitStartup: %v", err)
	}
}

// AC-14: a model identity change invalidates the durable vectors but never the
// exact definitions, so the restored catalog still serves its tools.
func TestStartupInvalidatesVectorsWhenTheModelIdentityChanged(t *testing.T) {
	base := publishedSnapshot(t)
	durable := durableSnapshot(t, base,
		alphaDurableServer(durableStaleDigest),
		betaDurableServer(durableStaleDigest),
	)

	cold := newReloadHarness(t)
	cfg := parseReloadConfig(t, reloadTwoServers)
	assembly, err := cold.runtime.AdmitStartup(cfg, cold.deps(), durable)
	if err != nil {
		t.Fatalf("AdmitStartup: %v", err)
	}

	if got := len(assembly.Snapshot().Embeddings); got != 0 {
		t.Fatalf("startup kept %d vectors of a changed model identity; want 0", got)
	}
	if got := len(assembly.Snapshot().Tools); got != 2 {
		t.Fatalf("startup kept %d tool rows of a changed model identity; want 2", got)
	}
	if _, ok := assembly.Catalog().Binding(core.PublicName("alpha", "read")); !ok {
		t.Fatal("startup dropped the exact catalog of a changed model identity")
	}
}

// catalogToolCount counts the published catalog bindings of one server.
func catalogToolCount(assembly *service.Assembly, serverID string) int {
	count := 0
	for _, binding := range assembly.Catalog().Bindings() {
		if binding.ServerID == serverID {
			count++
		}
	}
	return count
}

// AC-13 + §4.10 step 2: a startup whose candidate endpoint changed for a
// server never serves that server's stale cache. The endpoint no longer
// matches the durable binding, so alpha is admitted empty with catalog_status
// none and zero cached rows (a fresh catalog_status none for the new endpoint)
// while the unaffected beta keeps its warm tools and vectors, and the durable
// binding of alpha is rebound to the new endpoint fingerprint.
func TestStartupRebindsServerWhenEndpointChanged(t *testing.T) {
	reloadEnv(t)
	base := publishedSnapshot(t)
	durable := durableSnapshot(t, base,
		alphaDurableServer(strings.Repeat("a", 64)),
		betaDurableServer(strings.Repeat("a", 64)),
	)

	cold := newReloadHarness(t)
	cfg := parseReloadConfig(t, strings.Replace(reloadTwoServers,
		"https://alpha.invalid/mcp", "https://alpha-2.invalid/mcp", 1))
	assembly, err := cold.runtime.AdmitStartup(cfg, cold.deps(), durable)
	if err != nil {
		t.Fatalf("AdmitStartup: %v", err)
	}

	alphaHealth, ok := assembly.Health("alpha")
	if !ok {
		t.Fatal("startup snapshot has no health row for alpha")
	}
	if alphaHealth.Status != storage.StatusNone {
		t.Fatalf("alpha health = %q; want %q", alphaHealth.Status, storage.StatusNone)
	}
	if got := catalogToolCount(assembly, "alpha"); got != 0 {
		t.Fatalf("startup catalog serves %d tools for the rebound alpha; want 0", got)
	}

	betaHealth, ok := assembly.Health("beta")
	if !ok {
		t.Fatal("startup snapshot has no health row for beta")
	}
	if betaHealth.Status != storage.StatusStale {
		t.Fatalf("beta health = %q; want %q", betaHealth.Status, storage.StatusStale)
	}
	if got := catalogToolCount(assembly, "beta"); got != 1 {
		t.Fatalf("startup catalog serves %d tools for the unaffected beta; want 1", got)
	}

	bindings := map[string]storage.Binding{}
	for _, binding := range assembly.Snapshot().Bindings {
		bindings[binding.ServerID] = binding
	}
	rebound, ok := bindings["alpha"]
	if !ok || rebound.Retired {
		t.Fatalf("alpha binding = %+v; want a live rebound binding", rebound)
	}
	if rebound.EndpointFingerprint != mustFingerprint("https://alpha-2.invalid/mcp") {
		t.Fatalf("alpha binding fingerprint = %q; want the rebound endpoint fingerprint", rebound.EndpointFingerprint)
	}
	if got := len(snapshotTools(assembly.Snapshot(), "alpha")); got != 0 {
		t.Fatalf("startup kept %d tool rows for the rebound alpha; want 0", got)
	}
	if got := len(snapshotTools(assembly.Snapshot(), "beta")); got != 1 {
		t.Fatalf("startup kept %d tool rows for beta; want 1", got)
	}
	if got := len(assembly.Snapshot().Embeddings); got != 1 {
		t.Fatalf("startup kept %d vectors; want 1 (beta only)", got)
	}
}

// durableVector is the fixture embedding direction. It has the search layer's
// fixed width, so a document that carries it is vectorizable.
func durableVector() []float64 {
	vector := make([]float64, dimensions)
	vector[0] = 1
	return vector
}

// durableProvider is the fixture embedder: every input embeds to the same
// direction as the durable fixture vectors.
type durableProvider struct{}

func (durableProvider) Embed(_ context.Context, inputs []string) ([][]float64, error) {
	vectors := make([][]float64, len(inputs))
	for index := range vectors {
		vectors[index] = durableVector()
	}
	return vectors, nil
}

// The service's own search surface must serve only current durable vectors:
// a vector whose model identity still matches answers hybrid, while a vector
// from a previous model identity is discarded and the request degrades to
// keyword (plan §4.8/§4.9, T-06 RED).
func TestToolSurfaceUsesOnlyCurrentDurableVectors(t *testing.T) {
	t.Setenv("TOOLS_ALPHA_SECRET", "alpha-secret")
	t.Setenv("TOOLS_CLIENT_TOKEN", "client-token")
	base := publishedSnapshot(t)
	model := search.ModelIdentity{Provider: "fixture", Model: "embed-1", Dimensions: dimensions, FormatVersion: search.FormatVersion}

	for _, test := range []struct {
		name       string
		digest     string
		wantMode   string
		wantReason string
	}{
		{name: "current vector is adopted", digest: model.Digest(), wantMode: "hybrid"},
		{name: "stale vector is discarded", digest: durableStaleDigest, wantMode: "keyword", wantReason: "vectors_absent"},
	} {
		t.Run(test.name, func(t *testing.T) {
			durable := durableSnapshot(t, base, alphaDurableServer(test.digest), betaDurableServer(test.digest))
			cfg := parseReloadConfig(t, toolSurfaceConfig)
			h := newAdmittedToolHarness(t, cfg, service.ToolSurfaceConfig{
				Search:   search.Config{MaxQueryLength: 128, TopKDefault: 5, TopKMax: 20},
				Provider: durableProvider{},
				Model:    model,
			}, durable, model.Digest())

			payload := structured(t, h.call(t, "search_tools", map[string]any{"query": "alpha read"}))
			if payload["mode"] != test.wantMode {
				t.Fatalf("search mode = %v; want %s (%v)", payload["mode"], test.wantMode, payload)
			}
			if test.wantReason != "" {
				reasons, _ := payload["degraded_reasons"].([]any)
				if len(reasons) == 0 || reasons[0] != test.wantReason {
					t.Fatalf("degraded_reasons = %v; want [%s]", payload["degraded_reasons"], test.wantReason)
				}
			}
		})
	}
}

package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
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
// and tombstoned at startup; a later configuration can reinstate it without
// reviving its cached catalog.
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
	// The tombstone must outlive the restart: putting the server back into config
	// reinstates its binding but does not revive its cached catalog.
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
				Search:   search.Config{MaxQueryLength: 128, TopKDefault: 3, TopKMax: 4},
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
func tombstonedDurableSnapshot(t *testing.T, serverIDs ...string) *storage.Snapshot {
	t.Helper()
	base := publishedSnapshot(t)
	snapshot := durableSnapshot(t, base, alphaDurableServer(strings.Repeat("a", 64)), betaDurableServer(strings.Repeat("a", 64)))
	retired := make(map[string]bool, len(serverIDs))
	for _, serverID := range serverIDs {
		retired[serverID] = true
	}
	for index := range snapshot.Bindings {
		if retired[snapshot.Bindings[index].ServerID] {
			snapshot.Bindings[index].Retired = true
		}
	}
	servers := snapshot.Servers[:0]
	for _, server := range snapshot.Servers {
		if !retired[server.ServerID] {
			servers = append(servers, server)
		}
	}
	snapshot.Servers = servers
	tools := snapshot.Tools[:0]
	for _, tool := range snapshot.Tools {
		if !retired[tool.ServerID] {
			tools = append(tools, tool)
		}
	}
	snapshot.Tools = tools
	embeddings := snapshot.Embeddings[:0]
	for _, embedding := range snapshot.Embeddings {
		serverID, _, _ := strings.Cut(embedding.ToolIdentity, "\x00")
		if !retired[serverID] {
			embeddings = append(embeddings, embedding)
		}
	}
	snapshot.Embeddings = embeddings
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("tombstoned durable snapshot: %v", err)
	}
	return snapshot
}

func reinstatementPromptSnapshot(t *testing.T, prompts ...storage.Prompt) *storage.PromptSnapshot {
	t.Helper()
	snapshot := &storage.PromptSnapshot{SchemaVersion: storage.PromptSchemaVersion, Prompts: prompts}
	checksum, err := snapshot.ChecksumValue()
	if err != nil {
		t.Fatalf("prompt checksum: %v", err)
	}
	snapshot.Checksum = checksum
	return snapshot
}

func reinstatementPrompt(serverID, rawName string) storage.Prompt {
	return storage.Prompt{
		ServerID: serverID, RawName: rawName,
		PublicName: core.PublicName(serverID, rawName), Title: rawName,
	}
}

func assertNoServerPrompts(t *testing.T, prompts []storage.Prompt, serverID string) {
	t.Helper()
	for _, prompt := range prompts {
		if prompt.ServerID == serverID {
			t.Fatalf("prompt row for %q survived: %+v", serverID, prompt)
		}
	}
}

func decodeServiceLogRecords(t *testing.T, output string) []map[string]any {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(output))
	var records []map[string]any
	for {
		var record map[string]any
		err := decoder.Decode(&record)
		if errors.Is(err, io.EOF) {
			return records
		}
		if err != nil {
			t.Fatalf("decode service log record: %v: %q", err, output)
		}
		records = append(records, record)
	}
}

func TestStartupReinstatesConfiguredRetiredServerUnbound(t *testing.T) {
	reloadEnv(t)
	durable := tombstonedDurableSnapshot(t, "alpha")
	h := newReloadHarness(t)
	deps := h.deps()
	loadedPrompts := reinstatementPromptSnapshot(t,
		reinstatementPrompt("alpha", "old"),
		reinstatementPrompt("beta", "keep"),
	)
	var persistedPrompts *storage.PromptSnapshot
	var persistedSnapshot *storage.Snapshot
	var persistOrder []string
	deps.LoadPrompts = func() (*storage.PromptSnapshot, error) { return loadedPrompts, nil }
	deps.PersistPrompts = func(prompts *storage.PromptSnapshot) error {
		persistOrder = append(persistOrder, "prompts")
		persistedPrompts = prompts
		return nil
	}
	deps.Persist = func(snapshot *storage.Snapshot) error {
		persistOrder = append(persistOrder, "snapshot")
		persistedSnapshot = snapshot
		return nil
	}

	assembly, err := h.runtime.AdmitStartup(parseReloadConfig(t, reloadTwoServers), deps, durable)
	if err != nil {
		t.Fatalf("AdmitStartup: %v", err)
	}
	if _, ok := assembly.Catalog().Binding(core.PublicName("alpha", "read")); ok {
		t.Fatal("startup restored tools for the reinstated server")
	}
	if rows := snapshotTools(assembly.Snapshot(), "alpha"); len(rows) != 0 {
		t.Fatalf("startup restored %d tool rows for alpha", len(rows))
	}
	for _, embedding := range assembly.Snapshot().Embeddings {
		if strings.HasPrefix(embedding.ToolIdentity, "alpha\x00") {
			t.Fatalf("startup restored alpha embedding %q", embedding.ToolIdentity)
		}
	}
	health, ok := assembly.Health("alpha")
	if !ok || health.Status != storage.StatusNone {
		t.Fatalf("reinstated health = %+v, %v; want a non-success health row", health, ok)
	}
	assertNoServerPrompts(t, assembly.Prompts(), "alpha")
	if persistedPrompts == nil {
		t.Fatal("startup did not persist the purged prompt snapshot")
	}
	assertNoServerPrompts(t, persistedPrompts.Prompts, "alpha")
	if persistedSnapshot == nil || len(snapshotTools(persistedSnapshot, "alpha")) != 0 {
		t.Fatalf("persisted snapshot = %+v; want no alpha catalog", persistedSnapshot)
	}
	if !reflect.DeepEqual(persistOrder, []string{"prompts", "snapshot"}) {
		t.Fatalf("persistence order = %v; want prompts before snapshot", persistOrder)
	}
}

func TestStartupReinstatementDropsStalePromptRowsAcrossRestarts(t *testing.T) {
	reloadEnv(t)
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	initialPrompts := reinstatementPromptSnapshot(t,
		reinstatementPrompt("alpha", "stale"),
		reinstatementPrompt("beta", "keep"),
	)
	if err := store.SavePrompts(initialPrompts); err != nil {
		t.Fatalf("seed prompt rows: %v", err)
	}
	durable := tombstonedDurableSnapshot(t, "alpha")
	cfg := parseReloadConfig(t, reloadTwoServers)

	first := newReloadHarness(t)
	firstDeps := first.deps()
	firstDeps.LoadPrompts = store.LoadPrompts
	firstDeps.PersistPrompts = store.SavePrompts
	firstDeps.Persist = store.Save
	firstAssembly, err := first.runtime.AdmitStartup(cfg, firstDeps, durable)
	if err != nil {
		t.Fatalf("first startup: %v", err)
	}
	assertNoServerPrompts(t, firstAssembly.Prompts(), "alpha")
	storedPrompts, err := store.LoadPrompts()
	if err != nil {
		t.Fatalf("load prompts after first startup: %v", err)
	}
	assertNoServerPrompts(t, storedPrompts.Prompts, "alpha")
	storedSnapshot, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot after first startup: %v", err)
	}
	if rows := snapshotTools(storedSnapshot, "alpha"); len(rows) != 0 {
		t.Fatalf("stored snapshot kept %d alpha tools", len(rows))
	}

	second := newReloadHarness(t)
	secondDeps := second.deps()
	secondDeps.LoadPrompts = store.LoadPrompts
	secondDeps.PersistPrompts = store.SavePrompts
	secondDeps.Persist = store.Save
	secondAssembly, err := second.runtime.AdmitStartup(cfg, secondDeps, storedSnapshot)
	if err != nil {
		t.Fatalf("second startup: %v", err)
	}
	assertNoServerPrompts(t, secondAssembly.Prompts(), "alpha")
	storedPrompts, err = store.LoadPrompts()
	if err != nil {
		t.Fatalf("load prompts after second startup: %v", err)
	}
	assertNoServerPrompts(t, storedPrompts.Prompts, "alpha")
	if rows := snapshotTools(secondAssembly.Snapshot(), "alpha"); len(rows) != 0 {
		t.Fatalf("second startup served %d alpha tools", len(rows))
	}
}

func TestStartupReinstatementEmitsExactlyOneWarnAtCommit(t *testing.T) {
	reloadEnv(t)
	h := newReloadHarness(t)
	durable := tombstonedDurableSnapshot(t, "alpha", "beta")
	var output bytes.Buffer
	deps := h.deps()
	deps.Logger = observe.NewLogger(&output, &output)
	deps.LoadPrompts = func() (*storage.PromptSnapshot, error) {
		return reinstatementPromptSnapshot(t), nil
	}
	deps.PersistPrompts = func(*storage.PromptSnapshot) error { return nil }
	deps.Persist = func(*storage.Snapshot) error { return nil }

	if _, err := h.runtime.AdmitStartup(parseReloadConfig(t, reloadTwoServers), deps, durable); err != nil {
		t.Fatalf("AdmitStartup: %v", err)
	}
	records := decodeServiceLogRecords(t, output.String())
	seen := make(map[string]bool)
	for _, record := range records {
		if record["event"] != "binding_reinstated" {
			continue
		}
		serverID, _ := record["server"].(string)
		if seen[serverID] {
			t.Errorf("server %q was emitted more than once: %+v", serverID, record)
		}
		seen[serverID] = true
		if record["level"] != "warn" || record["operation"] != "startup" || record["outcome"] != "degraded" {
			t.Errorf("binding reinstated record = %+v; want warn/startup/degraded", record)
		}
	}
	if len(seen) != 2 || !seen["alpha"] || !seen["beta"] {
		t.Fatalf("reinstated server emissions = %v; want exactly alpha and beta", seen)
	}

	for _, missing := range []string{"Persist", "PersistPrompts", "LoadPrompts"} {
		t.Run("missing "+missing, func(t *testing.T) {
			h := newReloadHarness(t)
			deps := h.deps()
			deps.LoadPrompts = func() (*storage.PromptSnapshot, error) {
				return reinstatementPromptSnapshot(t), nil
			}
			deps.PersistPrompts = func(*storage.PromptSnapshot) error { return nil }
			deps.Persist = func(*storage.Snapshot) error { return nil }
			var logOutput bytes.Buffer
			deps.Logger = observe.NewLogger(&logOutput, &logOutput)
			switch missing {
			case "Persist":
				deps.Persist = nil
			case "PersistPrompts":
				deps.PersistPrompts = nil
			case "LoadPrompts":
				deps.LoadPrompts = nil
			}
			if _, err := h.runtime.AdmitStartup(parseReloadConfig(t, reloadTwoServers), deps, tombstonedDurableSnapshot(t, "alpha")); err == nil {
				t.Fatal("startup without a required dependency succeeded")
			}
			if h.runtime.Current() != nil {
				t.Fatal("failed reinstatement published a generation")
			}
			for _, record := range decodeServiceLogRecords(t, logOutput.String()) {
				if record["event"] == "binding_reinstated" {
					t.Fatalf("failed reinstatement emitted a commit record: %+v", record)
				}
			}
		})
	}

	t.Run("only no snapshot is absorbed", func(t *testing.T) {
		h := newReloadHarness(t)
		deps := h.deps()
		deps.LoadPrompts = func() (*storage.PromptSnapshot, error) { return nil, storage.ErrNoSnapshot }
		deps.PersistPrompts = func(*storage.PromptSnapshot) error { return nil }
		deps.Persist = func(*storage.Snapshot) error { return nil }
		if _, err := h.runtime.AdmitStartup(parseReloadConfig(t, reloadTwoServers), deps, tombstonedDurableSnapshot(t, "alpha")); err != nil {
			t.Fatalf("startup with no prompt snapshot: %v", err)
		}
	})

	t.Run("prompt load error fails closed", func(t *testing.T) {
		h := newReloadHarness(t)
		var logOutput bytes.Buffer
		deps := h.deps()
		deps.Logger = observe.NewLogger(&logOutput, &logOutput)
		deps.LoadPrompts = func() (*storage.PromptSnapshot, error) { return nil, errors.New("load failed") }
		deps.PersistPrompts = func(*storage.PromptSnapshot) error { return nil }
		deps.Persist = func(*storage.Snapshot) error { return nil }
		if _, err := h.runtime.AdmitStartup(parseReloadConfig(t, reloadTwoServers), deps, tombstonedDurableSnapshot(t, "alpha")); err == nil {
			t.Fatal("startup with an unreadable prompt snapshot succeeded")
		}
		if h.runtime.Current() != nil {
			t.Fatal("prompt load failure published a generation")
		}
		records := decodeServiceLogRecords(t, logOutput.String())
		if len(records) != 1 || records[0]["operation"] != "prompt_load" {
			t.Fatalf("prompt load warnings = %+v; want exactly one prompt_load warning", records)
		}
	})

	t.Run("prompt persist error warns once and fails closed", func(t *testing.T) {
		h := newReloadHarness(t)
		var logOutput bytes.Buffer
		deps := h.deps()
		deps.Logger = observe.NewLogger(&logOutput, &logOutput)
		deps.LoadPrompts = func() (*storage.PromptSnapshot, error) {
			return reinstatementPromptSnapshot(t), nil
		}
		deps.PersistPrompts = func(*storage.PromptSnapshot) error { return errors.New("persist failed") }
		snapshotWrites := 0
		deps.Persist = func(*storage.Snapshot) error {
			snapshotWrites++
			return nil
		}
		if _, err := h.runtime.AdmitStartup(parseReloadConfig(t, reloadTwoServers), deps, tombstonedDurableSnapshot(t, "alpha")); err == nil {
			t.Fatal("startup with a failed prompt persist succeeded")
		}
		if h.runtime.Current() != nil || snapshotWrites != 0 {
			t.Fatalf("failed prompt persistence published state: current=%v snapshot writes=%d", h.runtime.Current(), snapshotWrites)
		}
		records := decodeServiceLogRecords(t, logOutput.String())
		if len(records) != 1 || records[0]["event"] != "internal_warning" || records[0]["operation"] != "prompt_persist" {
			t.Fatalf("prompt persist warnings = %+v; want exactly one prompt_persist warning", records)
		}
	})
}

func TestStartupReinstatementPostRenamePersistFailure(t *testing.T) {
	reloadEnv(t)
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.SavePrompts(reinstatementPromptSnapshot(t, reinstatementPrompt("alpha", "stale"))); err != nil {
		t.Fatalf("seed prompt rows: %v", err)
	}
	var output bytes.Buffer
	h := newReloadHarness(t)
	deps := h.deps()
	deps.Logger = observe.NewLogger(&output, &output)
	deps.LoadPrompts = store.LoadPrompts
	deps.PersistPrompts = store.SavePrompts
	deps.Persist = func(snapshot *storage.Snapshot) error {
		if err := store.Save(snapshot); err != nil {
			return err
		}
		return errors.New("post-save uncertainty")
	}
	durable := tombstonedDurableSnapshot(t, "alpha")
	if _, err := h.runtime.AdmitStartup(parseReloadConfig(t, reloadTwoServers), deps, durable); err == nil {
		t.Fatal("post-save error was accepted")
	}
	if h.runtime.Current() != nil {
		t.Fatal("post-save error published a generation")
	}
	onDisk, err := store.Load()
	if err != nil {
		t.Fatalf("load snapshot after post-save error: %v", err)
	}
	var alphaBindingFound bool
	for _, binding := range onDisk.Bindings {
		if binding.ServerID != "alpha" {
			continue
		}
		alphaBindingFound = true
		if binding.Retired {
			t.Fatal("real Store.Save did not persist the active binding")
		}
	}
	if !alphaBindingFound {
		t.Fatal("real Store.Save did not persist the alpha binding")
	}
	storedPrompts, err := store.LoadPrompts()
	if err != nil {
		t.Fatalf("load prompts after post-save error: %v", err)
	}
	assertNoServerPrompts(t, storedPrompts.Prompts, "alpha")
	restartOutput := &bytes.Buffer{}
	restart := newReloadHarness(t)
	restartDeps := restart.deps()
	restartDeps.Logger = observe.NewLogger(restartOutput, restartOutput)
	restartDeps.Persist = func(*storage.Snapshot) error {
		return errors.New("restart admission failed")
	}
	_, restartErr := restart.runtime.AdmitStartup(parseReloadConfig(t, reloadTwoServers), restartDeps, onDisk)
	if restartErr == nil || restartErr.Error() != "service: startup persist: restart admission failed" {
		t.Fatalf("restart admission error = %v; want persisted admission error", restartErr)
	}
	var restartReinstated int
	for _, record := range decodeServiceLogRecords(t, restartOutput.String()) {
		if record["event"] == "binding_reinstated" {
			restartReinstated++
		}
	}
	if restartReinstated != 0 {
		t.Fatalf("restart emitted %d binding_reinstated records; want 0", restartReinstated)
	}
}

func TestStartupPreservesPromptsWithoutReinstatement(t *testing.T) {
	reloadEnv(t)
	h := newReloadHarness(t)
	prompts := reinstatementPromptSnapshot(t, reinstatementPrompt("alpha", "keep"))
	deps := h.deps()
	deps.LoadPrompts = func() (*storage.PromptSnapshot, error) { return prompts, nil }
	deps.PersistPrompts = func(*storage.PromptSnapshot) error {
		t.Fatal("ordinary startup persisted prompts")
		return nil
	}
	assembly, err := h.runtime.AdmitStartup(parseReloadConfig(t, reloadTwoServers), deps, publishedSnapshot(t))
	if err != nil {
		t.Fatalf("AdmitStartup: %v", err)
	}
	got := assembly.Prompts()
	if len(got) != 1 || got[0].ServerID != "alpha" || got[0].RawName != "keep" {
		t.Fatalf("ordinary startup prompts = %+v; want the existing alpha prompt", got)
	}
}

func TestReloadReinstatesDroppedThenReaddedServer(t *testing.T) {
	reloadEnv(t)
	h := newReloadHarness(t)
	initialPrompts := reinstatementPromptSnapshot(t,
		reinstatementPrompt("alpha", "old"),
		reinstatementPrompt("beta", "keep"),
	)
	var promptFile []storage.Prompt
	deps := h.deps()
	deps.LoadPrompts = func() (*storage.PromptSnapshot, error) { return initialPrompts, nil }
	if _, err := h.runtime.AdmitStartup(parseReloadConfig(t, reloadTwoServers), deps, publishedSnapshot(t)); err != nil {
		t.Fatalf("initial startup: %v", err)
	}

	promptPersistFails := true
	var persistedPromptSnapshot *storage.PromptSnapshot
	deps.PersistPrompts = func(prompts *storage.PromptSnapshot) error {
		if promptPersistFails {
			return errors.New("prompt persist failed")
		}
		persistedPromptSnapshot = prompts
		promptFile = append([]storage.Prompt(nil), prompts.Prompts...)
		return nil
	}
	betaOnly := `schema_version: 1
servers:
  - id: beta
    endpoint: https://beta.invalid/mcp
    credentials:
      bearer: {env: RELOAD_BETA_SECRET}
client_auth:
  bearer_tokens:
    - {env: RELOAD_CLIENT_TOKEN}
pins: []
`
	if _, err := h.runtime.Reload(parseReloadConfig(t, betaOnly), deps); err != nil {
		t.Fatalf("drop alpha: %v", err)
	}
	dropped := h.runtime.Current()
	if dropped == nil {
		t.Fatal("drop reload removed the accepted generation")
	}
	var retainedDroppedPrompt bool
	for _, prompt := range dropped.Prompts() {
		if prompt.ServerID == "alpha" {
			retainedDroppedPrompt = true
			break
		}
	}
	if !retainedDroppedPrompt {
		t.Fatal("failed drop prompt persistence did not restore alpha's previous prompt rows")
	}
	var logOutput bytes.Buffer
	deps.Logger = observe.NewLogger(&logOutput, &logOutput)
	beforeFailedAdd := len(h.persisted())
	if _, err := h.runtime.Reload(parseReloadConfig(t, reloadTwoServers), deps); err == nil {
		t.Fatal("re-add with failed prompt persistence succeeded")
	}
	if len(h.persisted()) != beforeFailedAdd {
		t.Fatal("snapshot persistence ran after prompt persistence failed")
	}
	current := h.runtime.Current()
	if current == nil {
		t.Fatal("failed re-add removed the previously accepted generation")
	}
	if _, exists := current.Server("alpha"); exists {
		t.Fatal("failed re-add published alpha")
	}
	var retainedFailedReaddPrompt bool
	for _, prompt := range current.Prompts() {
		if prompt.ServerID == "alpha" {
			retainedFailedReaddPrompt = true
			break
		}
	}
	if !retainedFailedReaddPrompt {
		t.Fatal("failed re-add did not preserve the accepted generation's previous prompt rows")
	}
	promptPersistFails = false
	readded, err := h.runtime.Reload(parseReloadConfig(t, reloadTwoServers), deps)
	if err != nil {
		t.Fatalf("retry re-add: %v", err)
	}
	assertNoServerPrompts(t, readded.Prompts(), "alpha")
	assertNoServerPrompts(t, promptFile, "alpha")
	if persistedPromptSnapshot == nil {
		t.Fatal("successful re-add did not persist a prompt snapshot")
	}
	assertNoServerPrompts(t, persistedPromptSnapshot.Prompts, "alpha")
	records := decodeServiceLogRecords(t, logOutput.String())
	var promptWarnings, reinstatedWarnings int
	for _, record := range records {
		switch record["event"] {
		case "internal_warning":
			if record["operation"] == "prompt_persist" {
				promptWarnings++
			}
		case "binding_reinstated":
			if record["server"] == "alpha" && record["operation"] == "reload" && record["level"] == "warn" && record["outcome"] == "degraded" {
				reinstatedWarnings++
			}
		}
	}
	if promptWarnings != 1 || reinstatedWarnings != 1 {
		t.Fatalf("warning counts prompt_persist=%d binding_reinstated=%d; want one each: %+v", promptWarnings, reinstatedWarnings, records)
	}
}
func TestRefreshNeverEmitsReinstatement(t *testing.T) {
	reloadEnv(t)
	h := newReloadHarness(t)
	durable := tombstonedDurableSnapshot(t, "alpha")

	startupConfig := parseReloadConfig(t, reloadTwoServers)
	for index := range startupConfig.Servers {
		if startupConfig.Servers[index].ID == "alpha" {
			startupConfig.Servers = append(startupConfig.Servers[:index], startupConfig.Servers[index+1:]...)
			break
		}
	}
	if _, err := h.runtime.AdmitStartup(startupConfig, h.deps(), durable); err != nil {
		t.Fatalf("initial startup: %v", err)
	}

	var output bytes.Buffer
	deps := refreshDeps(h)
	deps.Logger = observe.NewLogger(&output, &output)
	deps.PersistPrompts = func(*storage.PromptSnapshot) error { return nil }
	_, err := h.runtime.Refresh(parseReloadConfig(t, reloadTwoServers), deps)
	const want = `service: refresh reinstatement invariant violated for server "alpha"`
	if err == nil || err.Error() != want {
		t.Fatalf("refresh error = %v; want %q", err, want)
	}
	var reinstated int
	for _, record := range decodeServiceLogRecords(t, output.String()) {
		if record["event"] == "binding_reinstated" {
			reinstated++
		}
	}
	if reinstated != 0 {
		t.Fatalf("refresh emitted %d binding_reinstated records; want 0", reinstated)
	}
}

package storage_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	// T-07 membership reconciliation RED gate (r8 plan §4.9/§4.10 steps 2-3):
	// reconcile the current flat config against the durable snapshot — retain
	// matching servers with their catalog and vectors, purge removed servers so
	// their cache is unreachable, tombstone them so they never rebind, rebind a
	// live server whose endpoint identity changed with its cache purged, and stay
	// deterministic across a restart.
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

const (
	developerFingerprint = "9c9c9c9c9c9c9c9c9c9c9c9c9c9c9c9c9c9c9c9c9c9c9c9c9c9c9c9c9c9c9c9c"
	factoryFingerprint   = "7d7d7d7d7d7d7d7d7d7d7d7d7d7d7d7d7d7d7d7d7d7d7d7d7d7d7d7d7d7d7d7d"
	modelDigest          = "1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a"
	otherModelDigest     = "2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b"
	nextRevision         = "5e5e5e5e5e5e5e5e5e5e5e5e5e5e5e5e5e5e5e5e5e5e5e5e5e5e5e5e5e5e5e5e"
)

func nextConfigFingerprint() string { return strings.Repeat("4", 64) }

// twoServerSnapshot is a valid published snapshot holding the two configured
// servers "portainer" and "developer", each with one tool and one vector.
func twoServerSnapshot() *storage.Snapshot {
	snapshot := validSnapshot()
	developerTool := storage.Tool{
		ServerID:             "developer",
		RawName:              "read_file",
		PublicName:           "t_" + strings.Repeat("b", 52),
		Description:          "read one file",
		DefinitionHash:       strings.Repeat("3", 64),
		EmbeddingContentHash: strings.Repeat("2", 64),
		InputSchema:          json.RawMessage(`{"type":"object"}`),
		OutputSchema:         json.RawMessage(`{"type":"object"}`),
	}
	snapshot.Bindings = append(snapshot.Bindings, storage.Binding{
		ServerID:            "developer",
		EndpointFingerprint: developerFingerprint,
	})
	snapshot.Servers = append(snapshot.Servers, storage.Server{
		ServerID:            "developer",
		EndpointFingerprint: developerFingerprint,
		NegotiatedVersion:   "2025-06-18",
		CatalogStatus:       storage.StatusFresh,
	})
	snapshot.Tools = append(snapshot.Tools, developerTool)
	snapshot.Embeddings = append(snapshot.Embeddings, storage.Embedding{
		ToolIdentity:        storage.ToolIdentity("developer", "read_file"),
		ContentHash:         strings.Repeat("2", 64),
		ModelIdentityDigest: modelDigest,
		Dimensions:          8,
		FormatVersion:       "v1",
		Vector:              finiteVector(8, 1),
	})
	snapshot.Embeddings[0].ModelIdentityDigest = modelDigest
	snapshot.Embeddings[0].Dimensions = 8
	snapshot.Embeddings[0].Vector = finiteVector(8, 1)
	if err := snapshot.Validate(); err != nil {
		panic("twoServerSnapshot: " + err.Error())
	}
	return snapshot
}

func reconcileInput(servers ...storage.ConfiguredServer) storage.ReconcileInput {
	return storage.ReconcileInput{
		CreatedAt:               time.Date(2026, 10, 9, 6, 0, 0, 0, time.UTC),
		ConfigPublicFingerprint: nextConfigFingerprint(),
		CatalogRevision:         nextRevision,
		ModelIdentityDigest:     modelDigest,
		Servers:                 servers,
	}
}

// §4.9/§4.10: removed servers are purged to unreachable and tombstoned, and a
// newly configured server enters unbound without touching the survivors.
func TestReconcilePurgesRemovalsAndKeepsConfiguredServers(t *testing.T) {
	previous := twoServerSnapshot()
	original := twoServerSnapshot()

	next, err := storage.Reconcile(previous, reconcileInput(
		storage.ConfiguredServer{ServerID: "developer", EndpointFingerprint: developerFingerprint, NegotiatedVersion: "2025-06-18"},
		storage.ConfiguredServer{ServerID: "factory", EndpointFingerprint: factoryFingerprint, NegotiatedVersion: "2025-06-18"},
	))
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if !reflect.DeepEqual(previous, original) {
		t.Fatal("Reconcile mutated the previous snapshot")
	}
	if err := next.Validate(); err != nil {
		t.Fatalf("reconciled snapshot is invalid: %v", err)
	}
	if next.Envelope.GenerationSequence != previous.Envelope.GenerationSequence+1 {
		t.Fatalf("generation = %d; want %d", next.Envelope.GenerationSequence, previous.Envelope.GenerationSequence+1)
	}
	if next.Envelope.CatalogRevision != nextRevision || next.Envelope.ConfigPublicFingerprint != nextConfigFingerprint() {
		t.Fatal("reconciled envelope did not adopt the new config identity")
	}
	if !next.Envelope.CreatedAt.Equal(reconcileInput().CreatedAt) {
		t.Fatal("reconciled envelope did not adopt the new created_at")
	}

	bindings := map[string]storage.Binding{}
	for _, binding := range next.Bindings {
		bindings[binding.ServerID] = binding
	}
	if binding, ok := bindings["portainer"]; !ok || !binding.Retired {
		t.Fatalf("removed server binding = %+v; want a tombstone", binding)
	}
	if next.BindingAllowed("portainer", fingerprintValue) || next.BindingAllowed("portainer", strings.Repeat("6", 64)) {
		t.Fatal("a removed server was allowed to rebind")
	}
	if binding, ok := bindings["developer"]; !ok || binding.Retired {
		t.Fatalf("configured server binding = %+v; want a live binding", binding)
	}
	if binding, ok := bindings["factory"]; !ok || binding.Retired {
		t.Fatalf("new server binding = %+v; want a first bind", binding)
	}
	if !next.BindingAllowed("factory", factoryFingerprint) {
		t.Fatal("a newly configured server was refused its first bind")
	}

	servers := map[string]storage.Server{}
	for _, server := range next.Servers {
		servers[server.ServerID] = server
	}
	if _, ok := servers["portainer"]; ok {
		t.Fatal("a removed server kept its health row")
	}
	if server, ok := servers["developer"]; !ok || server.CatalogStatus != storage.StatusFresh {
		t.Fatalf("retained server row = %+v; want a fresh row", server)
	}
	if server, ok := servers["factory"]; !ok || server.CatalogStatus != storage.StatusNone {
		t.Fatalf("new server row = %+v; want catalog_status none", server)
	}

	// Removed-server cache is unreachable: no tool or vector survives it.
	for _, tool := range next.Tools {
		if tool.ServerID == "portainer" {
			t.Fatalf("removed server tool %q survived reconcile", tool.PublicName)
		}
	}
	for _, embedding := range next.Embeddings {
		if strings.HasPrefix(embedding.ToolIdentity, "portainer\x00") {
			t.Fatalf("removed server vector %q survived reconcile", embedding.ToolIdentity)
		}
	}
	// The survivor keeps exactly its own tool and vector.
	var survivorTools, survivorVectors []string
	for _, tool := range next.Tools {
		survivorTools = append(survivorTools, tool.PublicName)
	}
	for _, embedding := range next.Embeddings {
		survivorVectors = append(survivorVectors, embedding.ToolIdentity)
	}
	sort.Strings(survivorTools)
	if len(survivorTools) != 1 || survivorTools[0] != "t_"+strings.Repeat("b", 52) {
		t.Fatalf("retained tools = %v; want only the developer tool", survivorTools)
	}
	if len(survivorVectors) != 1 || survivorVectors[0] != storage.ToolIdentity("developer", "read_file") {
		t.Fatalf("retained vectors = %v; want only the developer vector", survivorVectors)
	}
}

// §4.9: a tombstoned server never rebinds, on any endpoint; other servers are
// unaffected by the rejection.
func TestReconcileRejectsTombstonedRebind(t *testing.T) {
	previous := twoServerSnapshot()
	previous.Bindings[0].Retired = true
	previous.Servers = previous.Servers[1:]
	previous.Tools = previous.Tools[1:]
	previous.Embeddings = previous.Embeddings[1:]

	next, err := storage.Reconcile(previous, reconcileInput(
		storage.ConfiguredServer{ServerID: "portainer", EndpointFingerprint: fingerprintValue},
		storage.ConfiguredServer{ServerID: "developer", EndpointFingerprint: developerFingerprint},
	))
	if !errors.Is(err, storage.ErrRebindTombstoned) {
		t.Fatalf("Reconcile = %v; want ErrRebindTombstoned", err)
	}
	if next != nil {
		t.Fatal("a rejected reconcile returned a snapshot")
	}
}

// §4.10 step 2 (revised): an endpoint identity change on a live configured
// server rebinds it — the binding adopts the new fingerprint, its catalog and
// cache are purged, its health row resets, and unaffected servers keep their
// exact state.
func TestReconcileRebindsEndpointIdentityChange(t *testing.T) {
	previous := twoServerSnapshot()
	original := twoServerSnapshot()
	newFingerprint := strings.Repeat("6", 64)

	next, err := storage.Reconcile(previous, reconcileInput(
		storage.ConfiguredServer{ServerID: "portainer", EndpointFingerprint: newFingerprint},
		storage.ConfiguredServer{ServerID: "developer", EndpointFingerprint: developerFingerprint, NegotiatedVersion: "2025-06-18"},
	))
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if !reflect.DeepEqual(previous, original) {
		t.Fatal("Reconcile mutated the previous snapshot")
	}
	if err := next.Validate(); err != nil {
		t.Fatalf("reconciled snapshot is invalid: %v", err)
	}
	if next.Envelope.GenerationSequence != previous.Envelope.GenerationSequence+1 {
		t.Fatalf("generation = %d; want %d", next.Envelope.GenerationSequence, previous.Envelope.GenerationSequence+1)
	}

	// Binding: live, unretired, pointing at the new endpoint only.
	bindings := map[string]storage.Binding{}
	for _, binding := range next.Bindings {
		bindings[binding.ServerID] = binding
	}
	rebound, ok := bindings["portainer"]
	if !ok {
		t.Fatal("rebound server lost its binding")
	}
	if rebound.Retired {
		t.Fatal("rebound binding was tombstoned")
	}
	if rebound.EndpointFingerprint != newFingerprint {
		t.Fatalf("rebound fingerprint = %q; want the new endpoint identity", rebound.EndpointFingerprint)
	}
	if !next.BindingAllowed("portainer", newFingerprint) {
		t.Fatal("rebound server was refused its new endpoint")
	}
	if next.BindingAllowed("portainer", fingerprintValue) {
		t.Fatal("rebound server still accepts its stale endpoint")
	}

	// Health row: reset to catalog_status none with the new fingerprint.
	servers := map[string]storage.Server{}
	for _, server := range next.Servers {
		servers[server.ServerID] = server
	}
	row, ok := servers["portainer"]
	if !ok {
		t.Fatal("rebound server lost its health row")
	}
	if row.CatalogStatus != storage.StatusNone {
		t.Fatalf("rebound row catalog_status = %q; want none", row.CatalogStatus)
	}
	if row.EndpointFingerprint != newFingerprint {
		t.Fatalf("rebound row fingerprint = %q; want the new endpoint identity", row.EndpointFingerprint)
	}
	if !row.LastSuccessAt.IsZero() {
		t.Fatal("rebound row kept a pre-change last success")
	}

	// Cache: the rebound server's tool definitions and vectors are purged.
	for _, tool := range next.Tools {
		if tool.ServerID == "portainer" {
			t.Fatalf("rebound server tool %q survived the endpoint change", tool.PublicName)
		}
	}
	for _, embedding := range next.Embeddings {
		if embedding.ToolIdentity == storage.ToolIdentity("portainer", "list_containers") {
			t.Fatalf("rebound server vector %q survived the endpoint change", embedding.ToolIdentity)
		}
	}

	// Unaffected server: binding, health row, tool, and vector all retained.
	unaffected, ok := bindings["developer"]
	if !ok || unaffected.Retired || unaffected.EndpointFingerprint != developerFingerprint {
		t.Fatalf("unaffected binding = %+v; want an unchanged live binding", unaffected)
	}
	if row, ok := servers["developer"]; !ok || row.CatalogStatus != storage.StatusFresh || row.NegotiatedVersion != "2025-06-18" {
		t.Fatalf("unaffected row = %+v; want a fresh 2025-06-18 row", row)
	}
	if len(next.Tools) != 1 || next.Tools[0].ServerID != "developer" || next.Tools[0].PublicName != "t_"+strings.Repeat("b", 52) {
		t.Fatalf("retained tools = %+v; want only the developer tool", next.Tools)
	}
	if len(next.Embeddings) != 1 || next.Embeddings[0].ToolIdentity != storage.ToolIdentity("developer", "read_file") {
		t.Fatalf("retained vectors = %+v; want only the developer vector", next.Embeddings)
	}
}

// §4.8/§4.9: a model identity change invalidates vectors but never the exact
// tool definitions; an unchanged model identity reuses them.
func TestReconcileInvalidatesVectorsOnModelIdentityChange(t *testing.T) {
	previous := twoServerSnapshot()
	input := reconcileInput(
		storage.ConfiguredServer{ServerID: "portainer", EndpointFingerprint: fingerprintValue},
		storage.ConfiguredServer{ServerID: "developer", EndpointFingerprint: developerFingerprint},
	)

	reused, err := storage.Reconcile(previous, input)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(reused.Embeddings) != 2 || len(reused.Tools) != 2 {
		t.Fatalf("unchanged model identity dropped vectors: %d vectors, %d tools", len(reused.Embeddings), len(reused.Tools))
	}

	input.ModelIdentityDigest = otherModelDigest
	invalidated, err := storage.Reconcile(previous, input)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(invalidated.Embeddings) != 0 {
		t.Fatalf("model identity change kept %d vectors", len(invalidated.Embeddings))
	}
	if len(invalidated.Tools) != 2 {
		t.Fatalf("model identity change dropped tool definitions: %d tools", len(invalidated.Tools))
	}
	for _, tool := range invalidated.Tools {
		if tool.EmbeddingContentHash == "" {
			t.Fatalf("tool %q lost its content identity", tool.PublicName)
		}
	}
	if err := invalidated.Validate(); err != nil {
		t.Fatalf("invalidated snapshot is invalid: %v", err)
	}
}

// §4.9/§4.10: reconciliation is deterministic and stable across a restart, and
// an empty membership shrinks to a valid, fully tombstoned snapshot.
func TestReconcileIsDeterministicAcrossRestart(t *testing.T) {
	previous := twoServerSnapshot()
	input := reconcileInput(
		storage.ConfiguredServer{ServerID: "developer", EndpointFingerprint: developerFingerprint, NegotiatedVersion: "2025-11-25"},
	)
	first, err := storage.Reconcile(previous, input)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	again, err := storage.Reconcile(previous, input)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	firstBlob, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	againBlob, err := json.Marshal(again)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(firstBlob) != string(againBlob) {
		t.Fatalf("reconcile is not deterministic:\n%s\n%s", firstBlob, againBlob)
	}

	// Restart: persist, reload, reconcile again with the same config.
	directory := t.TempDir()
	store := openStore(t, directory)
	if err := store.Save(first); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	reloaded, err := storage.Reconcile(loaded, input)
	if err != nil {
		t.Fatalf("Reconcile after restart: %v", err)
	}
	if reloaded.Envelope.GenerationSequence != first.Envelope.GenerationSequence+1 {
		t.Fatalf("generation after restart = %d; want %d", reloaded.Envelope.GenerationSequence, first.Envelope.GenerationSequence+1)
	}
	if err := reloaded.Validate(); err != nil {
		t.Fatalf("post-restart snapshot is invalid: %v", err)
	}
	if len(reloaded.Bindings) != len(first.Bindings) || len(reloaded.Tools) != len(first.Tools) {
		t.Fatalf("post-restart reconcile changed membership: %d bindings, %d tools", len(reloaded.Bindings), len(reloaded.Tools))
	}
	for _, server := range reloaded.Servers {
		if server.ServerID == "developer" && server.NegotiatedVersion != "2025-11-25" {
			t.Fatalf("negotiated version = %q; want the reconciled version", server.NegotiatedVersion)
		}
	}

	empty, err := storage.Reconcile(previous, reconcileInput())
	if err != nil {
		t.Fatalf("Reconcile empty membership: %v", err)
	}
	if err := empty.Validate(); err != nil {
		t.Fatalf("empty-membership snapshot is invalid: %v", err)
	}
	if len(empty.Tools) != 0 || len(empty.Embeddings) != 0 || len(empty.Servers) != 0 {
		t.Fatalf("empty membership kept state: %d tools, %d vectors, %d servers", len(empty.Tools), len(empty.Embeddings), len(empty.Servers))
	}
	for _, binding := range empty.Bindings {
		if !binding.Retired {
			t.Fatalf("binding %q survived an empty membership", binding.ServerID)
		}
	}
}

// §4.9: reconciled inputs are validated before any state changes.
func TestReconcileRejectsInvalidInput(t *testing.T) {
	previous := twoServerSnapshot()
	cases := []struct {
		name   string
		mutate func(*storage.ReconcileInput)
	}{
		{name: "duplicate server", mutate: func(in *storage.ReconcileInput) {
			in.Servers = append(in.Servers, in.Servers[0])
		}},
		{name: "missing endpoint fingerprint", mutate: func(in *storage.ReconcileInput) {
			in.Servers[0].EndpointFingerprint = ""
		}},
		{name: "invalid server id", mutate: func(in *storage.ReconcileInput) {
			in.Servers[0].ServerID = "Portainer"
		}},
		{name: "invalid revision", mutate: func(in *storage.ReconcileInput) { in.CatalogRevision = "abc" }},
		{name: "missing config fingerprint", mutate: func(in *storage.ReconcileInput) { in.ConfigPublicFingerprint = "" }},
		{name: "zero created_at", mutate: func(in *storage.ReconcileInput) { in.CreatedAt = time.Time{} }},
		{name: "invalid model digest", mutate: func(in *storage.ReconcileInput) { in.ModelIdentityDigest = "zz" }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			input := reconcileInput(storage.ConfiguredServer{ServerID: "developer", EndpointFingerprint: developerFingerprint})
			testCase.mutate(&input)
			if _, err := storage.Reconcile(previous, input); err == nil {
				t.Fatalf("Reconcile accepted %s", testCase.name)
			}
		})
	}
	if _, err := storage.Reconcile(nil, reconcileInput(storage.ConfiguredServer{ServerID: "developer", EndpointFingerprint: developerFingerprint})); err != nil {
		t.Fatalf("cold-start reconcile: %v", err)
	}
}

package service_test

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/search"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

// The indexing/embedding slice (r8 plan §4.8/§4.9; AC-13): every build writes
// this cycle's exact catalog into the durable rows and completes their vectors,
// reusing a stored vector exactly while the embedded text and the model
// identity are unchanged, so a schema-only change never re-embeds (plan §4.8).

// indexingEmbedder is the fixture embedding provider. It answers one vector per
// document, keyed by the document's own content identity, records every pass it
// was asked for, and can be made to fail or to answer malformed rows.
type indexingEmbedder struct {
	digest string

	mu        sync.Mutex
	passes    [][]search.Document
	fail      error
	responses func(documents []search.Document) ([]storage.Embedding, error)
}

func newIndexingEmbedder(digest string) *indexingEmbedder {
	return &indexingEmbedder{digest: digest}
}

func (e *indexingEmbedder) embed(documents []search.Document) ([]storage.Embedding, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.passes = append(e.passes, append([]search.Document(nil), documents...))
	if e.fail != nil {
		return nil, e.fail
	}
	if e.responses != nil {
		return e.responses(documents)
	}
	rows := make([]storage.Embedding, 0, len(documents))
	for _, document := range documents {
		rows = append(rows, storage.Embedding{
			ToolIdentity:        storage.ToolIdentity(document.ServerID, document.RawName),
			ContentHash:         search.ContentHash(document.RawName, document.Description),
			ModelIdentityDigest: e.digest,
			Dimensions:          dimensions,
			FormatVersion:       search.FormatVersion,
			Vector:              durableVector(),
		})
	}
	return rows, nil
}

// documents is every document the embedder was asked for, flattened over passes.
func (e *indexingEmbedder) documents() []search.Document {
	e.mu.Lock()
	defer e.mu.Unlock()
	var all []search.Document
	for _, pass := range e.passes {
		all = append(all, pass...)
	}
	return all
}

func (e *indexingEmbedder) requests() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.passes)
}

func (e *indexingEmbedder) embedder() func([]search.Document) ([]storage.Embedding, error) {
	return func(documents []search.Document) ([]storage.Embedding, error) {
		return e.embed(documents)
	}
}

// reloaded reloads one candidate on the harness with the given embedder and
// returns the snapshot the runtime persisted for it.
func reloaded(t *testing.T, harness *reloadHarness, embed *indexingEmbedder) *storage.Snapshot {
	t.Helper()
	deps := harness.deps()
	if embed != nil {
		deps.Embed = embed.embedder()
	}
	cfg := parseReloadConfig(t, reloadTwoServers)
	if _, err := harness.runtime.Reload(cfg, deps); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	persisted := harness.persisted()
	if len(persisted) == 0 {
		t.Fatal("Reload persisted no snapshot")
	}
	return persisted[len(persisted)-1]
}

func toolRow(t *testing.T, snapshot *storage.Snapshot, serverID, rawName string) storage.Tool {
	t.Helper()
	for _, row := range snapshot.Tools {
		if row.ServerID == serverID && row.RawName == rawName {
			return row
		}
	}
	t.Fatalf("snapshot has no tool row for %s/%s", serverID, rawName)
	return storage.Tool{}
}

func embeddingRow(snapshot *storage.Snapshot, serverID, rawName string) (storage.Embedding, bool) {
	identity := storage.ToolIdentity(serverID, rawName)
	for _, row := range snapshot.Embeddings {
		if row.ToolIdentity == identity {
			return row, true
		}
	}
	return storage.Embedding{}, false
}

// assertVectorsMatchRows is the invariant every published snapshot must hold:
// a stored vector exists only for a row whose current content identity it
// carries, so no vector can ever answer for other text or another model.
func assertVectorsMatchRows(t *testing.T, snapshot *storage.Snapshot) {
	t.Helper()
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("published snapshot is invalid: %v", err)
	}
	for _, row := range snapshot.Embeddings {
		var tool *storage.Tool
		for i := range snapshot.Tools {
			if storage.ToolIdentity(snapshot.Tools[i].ServerID, snapshot.Tools[i].RawName) == row.ToolIdentity {
				tool = &snapshot.Tools[i]
			}
		}
		if tool == nil {
			t.Fatalf("vector for unknown tool identity %q", row.ToolIdentity)
		}
		if row.ContentHash != tool.EmbeddingContentHash {
			t.Fatalf("vector of %q carries content hash %s; the row says %s", row.ToolIdentity, row.ContentHash, tool.EmbeddingContentHash)
		}
	}
}

// AC-13: a reload writes this cycle's exact definitions into the durable rows and
// stores one current vector per tool, so the published catalog and its vectors
// survive a restart instead of living only in memory.
func TestReloadIndexesTheExactCatalogWithCurrentVectors(t *testing.T) {
	reloadEnv(t)
	harness := newReloadHarness(t)
	digest := strings.Repeat("a", 64)
	embedder := newIndexingEmbedder(digest)

	snapshot := reloaded(t, harness, embedder)
	assertVectorsMatchRows(t, snapshot)

	if got := len(snapshot.Tools); got != 2 {
		t.Fatalf("indexed %d tool rows; want 2", got)
	}
	for serverID, rawName := range map[string]string{"alpha": "read", "beta": "write"} {
		def := harness.defs[serverID][0]
		row := toolRow(t, snapshot, serverID, rawName)
		if row.PublicName != core.PublicName(serverID, rawName) {
			t.Fatalf("row %s/%s public name = %q", serverID, rawName, row.PublicName)
		}
		if row.DefinitionHash != core.DefinitionDigest(serverID, rawName, def) {
			t.Fatalf("row %s/%s definition hash is not the exact definition digest", serverID, rawName)
		}
		if row.EmbeddingContentHash != search.ContentHash(rawName, def.Description) {
			t.Fatalf("row %s/%s embedding content hash = %q", serverID, rawName, row.EmbeddingContentHash)
		}
		if string(row.InputSchema) != string(def.InputSchema) {
			t.Fatalf("row %s/%s input schema = %s; want the exact definition %s", serverID, rawName, row.InputSchema, def.InputSchema)
		}
		vector, ok := embeddingRow(snapshot, serverID, rawName)
		if !ok {
			t.Fatalf("no current vector stored for %s/%s", serverID, rawName)
		}
		if vector.ModelIdentityDigest != digest || vector.Dimensions != dimensions {
			t.Fatalf("vector of %s/%s = digest %s dimensions %d", serverID, rawName, vector.ModelIdentityDigest, vector.Dimensions)
		}
	}

	documents := embedder.documents()
	if len(documents) != 2 {
		t.Fatalf("embedded %d documents; want the 2 tools with no current vector", len(documents))
	}
	for _, document := range documents {
		if document.PublicName != core.PublicName(document.ServerID, document.RawName) {
			t.Fatalf("document %s/%s carries public name %q", document.ServerID, document.RawName, document.PublicName)
		}
		if document.Description != harness.defs[document.ServerID][0].Description {
			t.Fatalf("document %s/%s carries description %q", document.ServerID, document.RawName, document.Description)
		}
	}
}

// AC-13: a second pass over an unchanged catalog reuses every stored vector and
// asks the provider for nothing; changing one description re-embeds exactly that
// tool and keeps the other vector.
func TestReloadReusesVectorsAndReembedsOnlyChangedText(t *testing.T) {
	reloadEnv(t)
	harness := newReloadHarness(t)
	first := newIndexingEmbedder(strings.Repeat("a", 64))
	before := reloaded(t, harness, first)
	if len(before.Embeddings) != 2 {
		t.Fatalf("first pass stored %d vectors; want 2", len(before.Embeddings))
	}

	reuse := newIndexingEmbedder(strings.Repeat("a", 64))
	repeated := reloaded(t, harness, reuse)
	if got := reuse.requests(); got != 0 {
		t.Fatalf("an unchanged catalog triggered %d embedding passes; want 0 (%v)", got, reuse.documents())
	}
	if len(repeated.Embeddings) != 2 {
		t.Fatalf("reused %d vectors; want 2", len(repeated.Embeddings))
	}
	alphaBefore, _ := embeddingRow(before, "alpha", "read")
	alphaAfter, _ := embeddingRow(repeated, "alpha", "read")
	if alphaAfter.ContentHash != alphaBefore.ContentHash || len(alphaAfter.Vector) != len(alphaBefore.Vector) {
		t.Fatal("the reused vector of an unchanged tool changed")
	}

	harness.mu.Lock()
	harness.defs["alpha"] = []core.ToolDefinition{{RawName: "read", Description: "alpha read revised", InputSchema: json.RawMessage(`{"type":"object"}`)}}
	harness.mu.Unlock()
	changed := newIndexingEmbedder(strings.Repeat("a", 64))
	after := reloaded(t, harness, changed)

	documents := changed.documents()
	if len(documents) != 1 || documents[0].ServerID != "alpha" || documents[0].RawName != "read" {
		t.Fatalf("re-embedded %v; want only alpha/read", documents)
	}
	reembedded, ok := embeddingRow(after, "alpha", "read")
	if !ok {
		t.Fatal("the re-embedded tool has no current vector")
	}
	if reembedded.ContentHash != search.ContentHash("read", "alpha read revised") {
		t.Fatalf("re-embedded vector content hash = %q", reembedded.ContentHash)
	}
	betaVector, _ := embeddingRow(after, "beta", "write")
	if betaVector.ContentHash != search.ContentHash("write", "beta write") {
		t.Fatal("an unrelated change replaced the beta vector")
	}
	assertVectorsMatchRows(t, after)
}

// AC-13 "schema-only reuse": a definition whose schema changed needs no new
// vector, while the exact definition hash must move.
func TestReloadReusesVectorsAcrossASchemaOnlyChange(t *testing.T) {
	reloadEnv(t)
	harness := newReloadHarness(t)
	_ = reloaded(t, harness, newIndexingEmbedder(strings.Repeat("a", 64)))
	before := harness.persisted()[0]

	harness.mu.Lock()
	harness.defs["alpha"] = []core.ToolDefinition{{RawName: "read", Description: "alpha read",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"x":{"type":"string"}}}`)}}
	harness.mu.Unlock()
	embedder := newIndexingEmbedder(strings.Repeat("a", 64))
	after := reloaded(t, harness, embedder)

	if got := embedder.requests(); got != 0 {
		t.Fatalf("a schema-only change triggered %d embedding passes; want 0 (%v)", got, embedder.documents())
	}
	alphaBefore := toolRow(t, before, "alpha", "read")
	alphaAfter := toolRow(t, after, "alpha", "read")
	if alphaAfter.DefinitionHash == alphaBefore.DefinitionHash {
		t.Fatal("a schema-only change kept the exact definition hash")
	}
	if alphaAfter.EmbeddingContentHash != alphaBefore.EmbeddingContentHash {
		t.Fatal("a schema-only change moved the embedding content identity")
	}
	vectorBefore, _ := embeddingRow(before, "alpha", "read")
	vectorAfter, ok := embeddingRow(after, "alpha", "read")
	if !ok || vectorAfter.ContentHash != vectorBefore.ContentHash {
		t.Fatal("a schema-only change dropped a still-current vector")
	}
	assertVectorsMatchRows(t, after)
}

// AC-13 degradation: a provider failure must not reject the generation and must
// never leave a vector behind for text it no longer covers.
func TestReloadKeepsOnlyReusableVectorsWhenEmbeddingFails(t *testing.T) {
	reloadEnv(t)
	harness := newReloadHarness(t)
	_ = reloaded(t, harness, newIndexingEmbedder(strings.Repeat("a", 64)))

	harness.mu.Lock()
	harness.defs["alpha"] = []core.ToolDefinition{{RawName: "read", Description: "alpha read revised", InputSchema: json.RawMessage(`{"type":"object"}`)}}
	harness.mu.Unlock()
	failing := newIndexingEmbedder(strings.Repeat("a", 64))
	failing.fail = errors.New("provider unavailable")

	after := reloaded(t, harness, failing)
	assertVectorsMatchRows(t, after)

	if _, ok := embeddingRow(after, "alpha", "read"); ok {
		t.Fatal("a failed pass kept a vector for text it no longer covers")
	}
	beta, ok := embeddingRow(after, "beta", "write")
	if !ok || beta.ContentHash != search.ContentHash("write", "beta write") {
		t.Fatal("a failed pass dropped an unrelated current vector")
	}
	if row := toolRow(t, after, "alpha", "read"); row.EmbeddingContentHash != search.ContentHash("read", "alpha read revised") {
		t.Fatalf("alpha row content hash = %q after the failed pass", row.EmbeddingContentHash)
	}
}

// A malformed provider answer is refused whole: no row of the pass is stored, so
// a wrong vector or another model's vector can never be persisted or served.
func TestReloadRefusesAMalformedEmbeddingPass(t *testing.T) {
	reloadEnv(t)
	for _, test := range []struct {
		name      string
		responses func([]search.Document) ([]storage.Embedding, error)
	}{
		{
			name: "content hash of other text",
			responses: func(documents []search.Document) ([]storage.Embedding, error) {
				rows, _ := newIndexingEmbedder(strings.Repeat("a", 64)).embed(documents)
				rows[0].ContentHash = search.ContentHash("read", "text that was never indexed")
				return rows, nil
			},
		},
		{
			name: "another model identity",
			responses: func(documents []search.Document) ([]storage.Embedding, error) {
				rows, _ := newIndexingEmbedder(strings.Repeat("b", 64)).embed(documents)
				return rows, nil
			},
		},
		{
			name: "dimensions that do not match the vector",
			responses: func(documents []search.Document) ([]storage.Embedding, error) {
				rows, _ := newIndexingEmbedder(strings.Repeat("a", 64)).embed(documents)
				rows[0].Dimensions = dimensions - 1
				return rows, nil
			},
		},
		{
			name: "vector for a tool this snapshot does not index",
			responses: func(documents []search.Document) ([]storage.Embedding, error) {
				rows, _ := newIndexingEmbedder(strings.Repeat("a", 64)).embed(documents)
				rows[0].ToolIdentity = storage.ToolIdentity("alpha", "absent")
				return rows, nil
			},
		},
		{
			name: "two widths in one pass",
			responses: func(documents []search.Document) ([]storage.Embedding, error) {
				rows, _ := newIndexingEmbedder(strings.Repeat("a", 64)).embed(documents)
				narrower := rows[0]
				narrower.ToolIdentity = storage.ToolIdentity("beta", "write")
				narrower.ContentHash = search.ContentHash("write", "beta write")
				narrower.Dimensions = dimensions - 1
				narrower.Vector = narrower.Vector[:dimensions-1]
				return append(rows, narrower), nil
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			harness := newReloadHarness(t)
			_ = reloaded(t, harness, newIndexingEmbedder(strings.Repeat("a", 64)))

			harness.mu.Lock()
			harness.defs["alpha"] = []core.ToolDefinition{{RawName: "read", Description: "alpha read revised", InputSchema: json.RawMessage(`{"type":"object"}`)}}
			harness.mu.Unlock()
			malformed := newIndexingEmbedder(strings.Repeat("a", 64))
			malformed.responses = test.responses

			after := reloaded(t, harness, malformed)
			assertVectorsMatchRows(t, after)
			if _, ok := embeddingRow(after, "alpha", "read"); ok {
				t.Fatal("a refused pass stored a vector for the changed tool")
			}
			beta, ok := embeddingRow(after, "beta", "write")
			if !ok {
				t.Fatal("a refused pass dropped a current vector of another tool")
			}
			if beta.ModelIdentityDigest != strings.Repeat("a", 64) || beta.Dimensions != dimensions {
				t.Fatalf("a refused pass corrupted the retained vector: %+v", beta)
			}
		})
	}
}

// Plan §4.5: a cold local admission contacts no upstream, so it never asks the
// embedding provider either - it serves the vectors the durable snapshot holds.
func TestStartupNeverAsksTheEmbedder(t *testing.T) {
	reloadEnv(t)
	base := publishedSnapshot(t)
	if len(base.Tools) != 2 {
		t.Fatalf("the first run indexed %d tool rows; want 2", len(base.Tools))
	}
	if len(base.Embeddings) != 0 {
		t.Fatalf("a run without an embedder stored %d vectors; want 0", len(base.Embeddings))
	}
	durable := durableSnapshot(t, base,
		alphaDurableServer(strings.Repeat("a", 64)),
		betaDurableServer(strings.Repeat("a", 64)),
	)
	embedder := newIndexingEmbedder(strings.Repeat("a", 64))
	deps := service.ReloadDeps{
		NewClient: func(config.Server) (core.Upstream, error) {
			return newFakeClient("startup"), nil
		},
		Now:                 func() time.Time { return time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC) },
		ModelIdentityDigest: strings.Repeat("a", 64),
		Persist:             func(*storage.Snapshot) error { return nil },
		Embed:               embedder.embedder(),
	}
	assembly, err := service.NewRuntime().AdmitStartup(parseReloadConfig(t, reloadTwoServers), deps, durable)
	if err != nil {
		t.Fatalf("AdmitStartup: %v", err)
	}
	if got := embedder.requests(); got != 0 {
		t.Fatalf("a cold startup triggered %d embedding passes; want 0", got)
	}
	if got := len(assembly.Snapshot().Embeddings); got != 2 {
		t.Fatalf("startup kept %d durable vectors; want 2", got)
	}
	assertVectorsMatchRows(t, assembly.Snapshot())
}

package search_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	// T-06 search RED gate (r8 plan §4.8/§4.4): compile-RED against the
	// missing internal/search package. `go test ./internal/search` fails with
	// "no required module provides package .../internal/search" until the
	// searcher lands GREEN.
	"github.com/jjohnson83853/mcp-bridge/internal/search"
)

const dimensions = 1024

// countingProvider is the deterministic embedding double (U-04 is unresolved,
// so no real provider contract is fabricated). Its vectors alternate sign along
// the dimensions, so they are dissimilar to the corpus fixtures' monotone
// vectors: the double never fabricates semantic matches, only lexical ones.
type countingProvider struct {
	calls   int
	batches []int
	fail    bool
}

func (p *countingProvider) Embed(_ context.Context, inputs []string) ([][]float64, error) {
	p.calls++
	p.batches = append(p.batches, len(inputs))
	if p.fail {
		return nil, errors.New("embedding provider unavailable")
	}
	vectors := make([][]float64, len(inputs))
	for i, input := range inputs {
		vector := make([]float64, dimensions)
		for d := range vector {
			magnitude := float64(len(input)+d) / 10
			if d%2 == 0 {
				vector[d] = magnitude
				continue
			}
			vector[d] = -magnitude
		}
		vectors[i] = vector
	}
	return vectors, nil
}

func testConfig() search.Config {
	return search.Config{
		TopKDefault:    10,
		TopKMax:        50,
		KeywordWeight:  0.5,
		VectorWeight:   0.5,
		RRFConstant:    60,
		BM25K1:         1.2,
		BM25B:          0.75,
		CacheEntries:   8,
		CacheTTL:       time.Minute,
		EmbedTimeout:   time.Second,
		MaxQueryLength: 2048,
	}
}

func model() search.ModelIdentity {
	return search.ModelIdentity{
		Provider:      "openai-compatible",
		BaseURL:       "https://embed.example/v1",
		Model:         "text-embedding-test",
		Revision:      "2026-01",
		Dimensions:    dimensions,
		FormatVersion: search.FormatVersion,
	}
}

func corpus() search.Snapshot {
	return search.Snapshot{
		Revision: strings.Repeat("a", 64),
		Model:    model(),
		Documents: []search.Document{
			{
				ServerID:    "portainer",
				RawName:     "list_containers",
				PublicName:  "t_" + strings.Repeat("a", 52),
				Description: "list running containers on the endpoint",
				Vector:      vector(dimensions, 1),
			},
			{
				ServerID:    "developer",
				RawName:     "list_containers",
				PublicName:  "t_" + strings.Repeat("b", 52),
				Description: "list developer containers",
				Vector:      vector(dimensions, 2),
			},
			{
				ServerID:    "developer",
				RawName:     "restart_service",
				PublicName:  "t_" + strings.Repeat("c", 52),
				Description: "restart a compose service",
				Vector:      vector(dimensions, 3),
			},
		},
		Backends: []search.Backend{
			{Server: "portainer", Status: "fresh", Availability: "available", Revision: strings.Repeat("a", 64), LastSuccessAt: time.Unix(1, 0).UTC()},
			{Server: "developer", Status: "stale", Availability: "unavailable", Reason: "connect_failed", Revision: strings.Repeat("a", 64), LastSuccessAt: time.Unix(2, 0).UTC()},
		},
	}
}

func vector(dimensions int, seed float64) []float64 {
	vector := make([]float64, dimensions)
	for i := range vector {
		vector[i] = seed + float64(i)/1000
	}
	return vector
}

func newSearcher(t *testing.T, snapshot search.Snapshot, provider search.Provider, cfg search.Config) *search.Searcher {
	t.Helper()
	searcher, err := search.NewSearcher(snapshot, provider, cfg, search.WithClock(func() time.Time { return time.Unix(100, 0).UTC() }))
	if err != nil {
		t.Fatalf("NewSearcher: %v", err)
	}
	return searcher
}

// omittedfilterfailsdiscoveroneserver: an omitted servers filter must search
// EVERY configured server, not only the first (r8 §4.8, AC-09).
func TestOmittedFilterSearchesEveryConfiguredServer(t *testing.T) {
	provider := &countingProvider{}
	searcher := newSearcher(t, corpus(), provider, testConfig())
	response, err := searcher.Search(context.Background(), search.Request{Query: "list containers"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	servers := map[string]bool{}
	for _, result := range response.Results {
		servers[result.Server] = true
		if len(result.ToolName) != 54 || !strings.HasPrefix(result.ToolName, "t_") {
			t.Fatalf("result tool_name %q is not a stable public name", result.ToolName)
		}
		if result.CatalogRevision != corpus().Revision {
			t.Fatalf("result catalog_revision %q does not match the global revision", result.CatalogRevision)
		}
	}
	if !servers["portainer"] || !servers["developer"] {
		t.Fatalf("omitted filter searched %v; want both configured servers", servers)
	}
	if response.CatalogRevision != corpus().Revision {
		t.Fatalf("response catalog_revision %q", response.CatalogRevision)
	}
}

// filternarrows: an explicit servers filter returns only the selected server
// and rejects servers that are not configured (r8 §4.8, AC-09).
func TestServerFilterNarrowsAndRejectsUnknown(t *testing.T) {
	searcher := newSearcher(t, corpus(), &countingProvider{}, testConfig())
	response, err := searcher.Search(context.Background(), search.Request{Query: "list containers", Servers: []string{"developer"}})
	if err != nil {
		t.Fatalf("filtered search: %v", err)
	}
	if len(response.Results) == 0 {
		t.Fatal("filtered search returned no results")
	}
	for _, result := range response.Results {
		if result.Server != "developer" {
			t.Fatalf("filter returned unselected server %q", result.Server)
		}
	}

	_, err = searcher.Search(context.Background(), search.Request{Query: "list", Servers: []string{"not-configured"}})
	var failures *search.Failure
	if !errors.As(err, &failures) || failures.Code != "invalid_filter" {
		t.Fatalf("unknown server filter error = %v; want invalid_filter", err)
	}
}

// namespacefieldrejected: there is no namespace parameter; a request carrying
// one is invalid-params, never a compatibility selector (r8 §4.3/§4.4).
func TestDecodeRequestRejectsNamespaceAndUnknownFields(t *testing.T) {
	cfg := testConfig()
	if _, err := search.DecodeRequest(json.RawMessage(`{"query":"x","namespace":"portainer"}`), cfg); err == nil {
		t.Fatal("namespace field accepted; want rejection")
	}
	if _, err := search.DecodeRequest(json.RawMessage(`{"query":"x","servers":["developer"]}`), cfg); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
}

// overlimit: bounds are rejected, never clamped, and an empty query can never
// degrade into a full-catalog response (r8 §4.2/§4.8).
func TestBoundsRejectedWithoutClamping(t *testing.T) {
	cfg := testConfig()
	if request, err := search.DecodeRequest(json.RawMessage(`{"query":"x"}`), cfg); err != nil || request.TopK != cfg.TopKDefault {
		t.Fatalf("omitted top_k = (%d, %v); want default %d", request.TopK, err, cfg.TopKDefault)
	}
	for _, raw := range []string{
		`{"query":"x","top_k":51}`,
		`{"query":"x","top_k":0}`,
		`{"query":"x","top_k":-1}`,
		`{"query":""}`,
		`{"query":"   "}`,
		`{"query":"x","servers":[]}`,
		`{"query":"` + strings.Repeat("q", 2049) + `"}`,
	} {
		if _, err := search.DecodeRequest(json.RawMessage(raw), cfg); err == nil {
			t.Fatalf("request %s accepted; want rejection without clamping", raw)
		}
	}

	searcher := newSearcher(t, corpus(), &countingProvider{}, cfg)
	response, err := searcher.Search(context.Background(), search.Request{Query: "containers", TopK: 1})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(response.Results) != 1 {
		t.Fatalf("top_k=1 returned %d results", len(response.Results))
	}
}

// semanticonly: a vector match with no lexical overlap is reachable in hybrid
// mode but not in keyword mode (r8 §4.8).
func TestSemanticOnlyMatchRequiresVectors(t *testing.T) {
	snapshot := corpus()
	snapshot.Documents[2].Description = "unrelated wording entirely"
	searcher := newSearcher(t, snapshot, &countingProvider{}, testConfig())

	hybrid, err := searcher.Search(context.Background(), search.Request{Query: "zzz-quasar"})
	if err != nil {
		t.Fatalf("hybrid search: %v", err)
	}
	if hybrid.Mode != "hybrid" {
		t.Fatalf("mode = %q; want hybrid", hybrid.Mode)
	}
	if len(hybrid.Results) != 0 {
		t.Fatalf("non-matching query returned %d results", len(hybrid.Results))
	}

	vectorOnly := newSearcherWithVectors(t, snapshot)
	matched, err := vectorOnly.Search(context.Background(), search.Request{Query: "zzz-quasar"})
	if err != nil {
		t.Fatalf("vector search: %v", err)
	}
	if len(matched.Results) == 0 {
		t.Fatal("semantic-only fixture produced no vector match; fixture is not exercising embeddings")
	}
}

func newSearcherWithVectors(t *testing.T, snapshot search.Snapshot) *search.Searcher {
	t.Helper()
	// A provider whose vectors point at the third document's vector direction,
	// so the query has a semantic match without any lexical overlap.
	provider := &fixedProvider{vectors: [][]float64{snapshot.Documents[2].Vector}}
	searcher, err := search.NewSearcher(snapshot, provider, testConfig())
	if err != nil {
		t.Fatalf("NewSearcher: %v", err)
	}
	return searcher
}

type fixedProvider struct{ vectors [][]float64 }

func (p *fixedProvider) Embed(context.Context, []string) ([][]float64, error) { return p.vectors, nil }

// providerfailure: a failing provider degrades to keyword mode with a finite
// reason and never exposes the full catalog (r8 §4.8, R-14, AC-12).
func TestProviderFailureDegradesToKeyword(t *testing.T) {
	searcher := newSearcher(t, corpus(), &countingProvider{fail: true}, testConfig())
	response, err := searcher.Search(context.Background(), search.Request{Query: "list containers"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if response.Mode != "keyword" {
		t.Fatalf("mode = %q; want keyword after provider failure", response.Mode)
	}
	if len(response.DegradedReasons) == 0 {
		t.Fatal("keyword fallback reported no degraded reason")
	}
	if len(response.Results) == 0 || len(response.Results) > testConfig().TopKDefault {
		t.Fatalf("keyword fallback returned %d results; want bounded lexical matches", len(response.Results))
	}
	if len(response.Results) >= len(corpus().Documents) {
		t.Fatal("keyword fallback exposed the full catalog")
	}
}

// partialvectors: when only some eligible documents have current vectors the
// response reports hybrid_partial rather than claiming full hybrid (r8 §4.8).
func TestPartialVectorsReportHybridPartial(t *testing.T) {
	snapshot := corpus()
	snapshot.Documents[1].Vector = nil
	searcher := newSearcher(t, snapshot, &countingProvider{}, testConfig())
	response, err := searcher.Search(context.Background(), search.Request{Query: "list containers"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if response.Mode != "hybrid_partial" {
		t.Fatalf("mode = %q; want hybrid_partial", response.Mode)
	}
}

// semanticabsent: when no eligible document carries a usable vector the
// searcher reports keyword mode with the finite vectors-absent reason, never
// fabricates a semantic match, and never asks the provider to embed (r8 §4.8,
// T-06 RED "semanticabsent").
func TestAbsentVectorsNeverMatchSemantically(t *testing.T) {
	for _, tc := range []struct {
		name  string
		spoil func(*search.Document)
	}{
		{name: "missing", spoil: func(document *search.Document) { document.Vector = nil }},
		{name: "nonfinite", spoil: func(document *search.Document) {
			document.Vector = vector(dimensions, 1)
			document.Vector[0] = math.NaN()
		}},
		{name: "allzero", spoil: func(document *search.Document) {
			document.Vector = make([]float64, dimensions)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := corpus()
			for index := range snapshot.Documents {
				tc.spoil(&snapshot.Documents[index])
			}
			// The provider would answer a semantic query, so only the absence
			// of usable document vectors can keep the match out of the results.
			provider := &countingProvider{}
			searcher := newSearcher(t, snapshot, provider, testConfig())

			response, err := searcher.Search(context.Background(), search.Request{Query: "zzz-quasar"})
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			if response.Mode != search.ModeKeyword {
				t.Fatalf("mode = %q; want %q without usable vectors", response.Mode, search.ModeKeyword)
			}
			if !slices.Contains(response.DegradedReasons, "vectors_absent") {
				t.Fatalf("degraded reasons = %v; want vectors_absent", response.DegradedReasons)
			}
			if len(response.Results) != 0 {
				t.Fatalf("results = %d; want no semantic match without usable vectors", len(response.Results))
			}
			if provider.calls != 0 {
				t.Fatalf("provider calls = %d; want none when no document has a usable vector", provider.calls)
			}
		})
	}
}

// A vector of the wrong dimensionality is a corrupt snapshot, not a degraded
// search: constructing the searcher fails closed instead of scoring it.
func TestSearcherRejectsDimensionMismatchedVectors(t *testing.T) {
	snapshot := corpus()
	snapshot.Documents[0].Vector = vector(dimensions-1, 1)
	if _, err := search.NewSearcher(snapshot, &countingProvider{}, testConfig()); err == nil {
		t.Fatal("NewSearcher accepted a vector that does not match the model dimensions")
	}
}

// ranking: RRF combines the keyword and vector rankings with the pinned
// constant, ties break on the stable public name, and results carry
// provenance (r8 §4.8).
func TestRRFRankingTieBreakAndProvenance(t *testing.T) {
	snapshot := search.Snapshot{
		Revision: strings.Repeat("a", 64),
		Model:    model(),
		Documents: []search.Document{
			{
				ServerID: "portainer", RawName: "read_c", PublicName: "t_" + strings.Repeat("b", 52),
				Description: "read one container", Vector: vector(dimensions, 1),
			},
			{
				ServerID: "portainer", RawName: "read_d", PublicName: "t_" + strings.Repeat("a", 52),
				Description: "read one container", Vector: vector(dimensions, 1),
			},
		},
		Backends: []search.Backend{
			{Server: "portainer", Status: "fresh", Availability: "available", LastSuccessAt: time.Unix(7, 0).UTC(), Revision: strings.Repeat("a", 64)},
		},
	}
	cfg := testConfig()
	// The query vector equals the fixture direction, so both documents rank in
	// the vector list; identical text ties in the lexical list. Both lists
	// therefore rank the same document first and the second document second.
	provider := &fixedProvider{vectors: [][]float64{vector(dimensions, 1)}}
	searcher := newSearcher(t, snapshot, provider, cfg)
	response, err := searcher.Search(context.Background(), search.Request{Query: "read one container"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(response.Results) != 2 {
		t.Fatalf("results = %d; want both tied documents", len(response.Results))
	}
	want := []struct {
		name string
		rank float64
	}{
		{name: "t_" + strings.Repeat("a", 52), rank: 1},
		{name: "t_" + strings.Repeat("b", 52), rank: 2},
	}
	for i, expectation := range want {
		result := response.Results[i]
		if result.ToolName != expectation.name {
			t.Fatalf("rank %d = %q; want the stable public name %q", i+1, result.ToolName, expectation.name)
		}
		wantScore := cfg.KeywordWeight/(cfg.RRFConstant+expectation.rank) + cfg.VectorWeight/(cfg.RRFConstant+expectation.rank)
		if math.Abs(result.Score-wantScore) > 1e-9 {
			t.Fatalf("score %v is not the pinned RRF combination (want %v)", result.Score, wantScore)
		}
		if result.Server != "portainer" || result.UpstreamName == "" || result.Description == "" {
			t.Fatalf("result provenance incomplete: %+v", result)
		}
		if result.CatalogStatus != "fresh" || result.Availability != "available" {
			t.Fatalf("result backend provenance missing: %+v", result)
		}
		if result.LastSuccessAt.IsZero() {
			t.Fatalf("result last_success_at missing: %+v", result)
		}
	}
}

// cache: identical requests reuse one cache entry, while a catalog revision or
// model identity change invalidates it (r8 §4.8 cache key).
func TestCacheKeyCoversRevisionAndModel(t *testing.T) {
	provider := &countingProvider{}
	cfg := testConfig()
	searcher := newSearcher(t, corpus(), provider, cfg)
	request := search.Request{Query: "list containers"}
	if _, err := searcher.Search(context.Background(), request); err != nil {
		t.Fatalf("first search: %v", err)
	}
	afterFirst := provider.calls
	if _, err := searcher.Search(context.Background(), request); err != nil {
		t.Fatalf("cached search: %v", err)
	}
	if provider.calls != afterFirst {
		t.Fatalf("cache miss on identical request: provider calls %d -> %d", afterFirst, provider.calls)
	}

	changed := corpus()
	changed.Revision = strings.Repeat("f", 64)
	refreshed := newSearcher(t, changed, provider, cfg)
	if _, err := refreshed.Search(context.Background(), request); err != nil {
		t.Fatalf("revision-changed search: %v", err)
	}
	if provider.calls == afterFirst {
		t.Fatal("catalog revision change reused stale vectors")
	}

	remodeled := corpus()
	remodeled.Model = search.ModelIdentity{
		Provider: "openai-compatible", BaseURL: "https://embed.example/v1",
		Model: "text-embedding-other", Revision: "2026-02",
		Dimensions: dimensions, FormatVersion: search.FormatVersion,
	}
	other := newSearcher(t, remodeled, provider, cfg)
	if _, err := other.Search(context.Background(), request); err != nil {
		t.Fatalf("model-changed search: %v", err)
	}
	if provider.calls == afterFirst+1 {
		t.Fatal("model identity change reused stale vectors")
	}
}

// cachelru: the cache is bounded, so a query evicted by newer entries is
// re-embedded rather than served from a stale entry (r8 §4.8).
func TestCacheIsBounded(t *testing.T) {
	provider := &countingProvider{}
	cfg := testConfig()
	cfg.CacheEntries = 2
	searcher := newSearcher(t, corpus(), provider, cfg)
	for _, query := range []string{"one", "two", "three", "four"} {
		if _, err := searcher.Search(context.Background(), search.Request{Query: query}); err != nil {
			t.Fatalf("search %q: %v", query, err)
		}
	}
	before := provider.calls
	if _, err := searcher.Search(context.Background(), search.Request{Query: "one"}); err != nil {
		t.Fatalf("re-search: %v", err)
	}
	if provider.calls != before+1 {
		t.Fatalf("bounded cache served an evicted entry: provider calls %d -> %d", before, provider.calls)
	}
}

// cachettl: entries expire with the configured TTL (r8 §4.8).
func TestCacheExpiresAtTTL(t *testing.T) {
	provider := &countingProvider{}
	cfg := testConfig()
	now := time.Unix(100, 0).UTC()
	searcher, err := search.NewSearcher(corpus(), provider, cfg, search.WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("NewSearcher: %v", err)
	}
	request := search.Request{Query: "list containers"}
	if _, err := searcher.Search(context.Background(), request); err != nil {
		t.Fatalf("first search: %v", err)
	}
	before := provider.calls
	now = now.Add(cfg.CacheTTL)
	if _, err := searcher.Search(context.Background(), request); err != nil {
		t.Fatalf("expired search: %v", err)
	}
	if provider.calls != before+1 {
		t.Fatalf("expired entry was reused: provider calls %d -> %d", before, provider.calls)
	}
}

// serverfilterpanicwhenlargercorpus: when the corpus is larger than the
// eligible set after a server filter, the lexical rank indices (global) must
// be translated to eligible indices before use, or a raw global index used
// against the smaller eligible slice panics (r8 §4.8).
func TestServerFilterPanicWhenCorpusLargerThanEligible(t *testing.T) {
	// Arrange: 30 documents, 25 on "other" and 5 on "target".
	documents := make([]search.Document, 0, 30)
	for i := range 25 {
		documents = append(documents, search.Document{
			ServerID:    "other",
			RawName:     fmt.Sprintf("other_tool_%d", i),
			PublicName:  fmt.Sprintf("t_other_%d", i),
			Description: "list running containers on the endpoint",
			Vector:      vector(dimensions, float64(i+1)),
		})
	}
	for i := range 5 {
		documents = append(documents, search.Document{
			ServerID:    "target",
			RawName:     fmt.Sprintf("target_tool_%d", i),
			PublicName:  fmt.Sprintf("t_target_%d", i),
			Description: "list running containers on the target server",
			Vector:      vector(dimensions, float64(i+26)),
		})
	}

	snapshot := search.Snapshot{
		Revision:  strings.Repeat("a", 64),
		Model:     model(),
		Documents: documents,
		Backends: []search.Backend{
			{Server: "other", Status: "fresh", Availability: "available", Revision: strings.Repeat("a", 64), LastSuccessAt: time.Unix(1, 0).UTC()},
			{Server: "target", Status: "fresh", Availability: "available", Revision: strings.Repeat("a", 64), LastSuccessAt: time.Unix(2, 0).UTC()},
		},
	}

	searcher := newSearcher(t, snapshot, &countingProvider{}, testConfig())

	// Act: query matches "other" terms but filter to "target" only.
	response, err := searcher.Search(context.Background(), search.Request{
		Query:   "list running containers",
		Servers: []string{"target"},
	})

	// Assert: must not panic; filtered results carry "target" provenance.
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(response.Results) == 0 {
		t.Fatal("expected at least one result")
	}
	for _, result := range response.Results {
		if result.Server != "target" {
			t.Errorf("expected result.Server = %q, got %q", "target", result.Server)
		}
	}
}

// formatter: the content identity is formatter-versioned and changes with the
// indexed text (r8 §4.8).
func TestContentHashFormatterV1(t *testing.T) {
	base := search.ContentHash("read", "read one container")
	if base != search.ContentHash("read", "read one container") {
		t.Fatal("content hash is not stable for identical input")
	}
	if base == search.ContentHash("read", "read another container") {
		t.Fatal("content hash ignores the description")
	}
	if base == search.ContentHash("read_two", "read one container") {
		t.Fatal("content hash ignores the raw name")
	}
	raw := sha256.Sum256([]byte("read\nread one container"))
	if base == hex.EncodeToString(raw[:]) {
		t.Fatal("content hash does not carry the format version")
	}
}

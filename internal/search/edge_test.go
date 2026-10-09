package search

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProviderRejectsInvalidConfigurationAndResponses(t *testing.T) {
	for _, options := range []ProviderOptions{{Provider: ProviderOpenAICompatible, Model: "m", Dimensions: 1, BaseURL: "ftp://example.test"}, {Provider: ProviderOpenAICompatible, Model: "m", Dimensions: 0, BaseURL: "https://example.test"}, {Provider: ProviderOpenAICompatible, Model: "m", Dimensions: MaxProviderDimensions + 1, BaseURL: "https://example.test"}} {
		if _, err := NewProvider(options); err == nil {
			t.Fatalf("accepted invalid options: %+v", options)
		}
	}
	cases := []struct {
		name, body string
		status     int
		limit      int64
	}{{"status", `upstream secret`, 502, 1024}, {"malformed", `{`, 200, 1024}, {"truncated", `{"data":[`, 200, 1024}, {"oversized", strings.Repeat("x", 32), 200, 8}, {"short data", `{"data":[]}`, 200, 1024}, {"zero", `{"data":[{"index":0,"embedding":[0,0]}]}`, 200, 1024}, {"dimension", `{"data":[{"index":0,"embedding":[1]}]}`, 200, 1024}, {"nonfinite", `{"data":[{"index":0,"embedding":[1e999,1]}]}`, 200, 1024}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer s.Close()
			p, err := NewProvider(ProviderOptions{Provider: ProviderOpenAICompatible, BaseURL: s.URL, Model: "m", Dimensions: 2, MaxResponseBytes: tc.limit})
			if err != nil {
				t.Fatal(err)
			}
			_, err = p.Embed(context.Background(), []string{"q"})
			if err == nil {
				t.Fatal("expected response rejection")
			}
			if strings.Contains(err.Error(), "upstream secret") {
				t.Fatalf("provider leaked body: %v", err)
			}
		})
	}
	long := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1,2]},{"index":1,"embedding":[1,2]}]}`))
	}))
	defer long.Close()
	p, _ := NewProvider(ProviderOptions{Provider: ProviderOpenAICompatible, BaseURL: long.URL, Model: "m", Dimensions: 2})
	if _, err := p.Embed(context.Background(), []string{"q"}); err == nil {
		t.Fatal("accepted excess response vectors")
	}
}

func TestDecodeEmbeddingsOrdersAndRejectsBadIndexes(t *testing.T) {
	vectors, err := decodeEmbeddings([]byte(`{"data":[{"index":1,"embedding":[2,1]},{"index":0,"embedding":[1,2]}]}`), 2, 2)
	if err != nil || vectors[0][0] != 1 || vectors[1][0] != 2 {
		t.Fatalf("vectors=%v err=%v", vectors, err)
	}
	for _, payload := range []string{`{"data":[{"index":2,"embedding":[1,2]}]}`, `{"data":[{"index":0,"embedding":[1,2]},{"index":0,"embedding":[2,1]}]}`} {
		if _, err := decodeEmbeddings([]byte(payload), 2, 2); err == nil {
			t.Fatalf("accepted invalid indexes: %s", payload)
		}
	}
}
func TestVectorFiniteAndZeroBoundaries(t *testing.T) {
	if _, ok := cosine(nil, nil); ok {
		t.Fatal("empty vectors matched")
	}
	if _, ok := cosine([]float64{1}, []float64{1, 2}); ok {
		t.Fatal("mismatched vectors matched")
	}
	if _, ok := cosine([]float64{0}, []float64{1}); ok {
		t.Fatal("zero norm matched")
	}
	if _, ok := cosine([]float64{math.NaN()}, []float64{1}); ok {
		t.Fatal("NaN matched")
	}
	if validVector([]float64{0, 0}, 2) || validVector([]float64{math.Inf(1)}, 1) || validVector([]float64{1}, 2) {
		t.Fatal("invalid vector accepted")
	}
}
func TestVectorCacheRefreshExpiryAndEviction(t *testing.T) {
	now := time.Unix(1, 0)
	cache := newVectorCache(1, time.Minute)
	a, b := cacheKey{query: "a"}, cacheKey{query: "b"}
	cache.put(a, []float64{1}, now)
	cache.put(a, []float64{2}, now.Add(time.Second))
	if got, ok := cache.get(a, now.Add(2*time.Second)); !ok || got[0] != 2 {
		t.Fatalf("updated cache entry=%v ok=%v", got, ok)
	}
	cache.put(b, []float64{3}, now.Add(2*time.Second))
	if _, ok := cache.get(a, now.Add(3*time.Second)); ok {
		t.Fatal("LRU entry survived eviction")
	}
	if _, ok := cache.get(b, now.Add(62*time.Second)); ok {
		t.Fatal("expired entry survived")
	}
}
func TestDecodeRequestRejectsInvalidConfig(t *testing.T) {
	if _, err := DecodeRequest(json.RawMessage(`{"query":"x"}`), Config{TopKDefault: 2, TopKMax: 1}); err == nil {
		t.Fatal("invalid config accepted")
	}
}

func TestConfigValidationBranches(t *testing.T) {
	base := Config{TopKDefault: 1, TopKMax: 1, KeywordWeight: 1, VectorWeight: 0, RRFConstant: 1, BM25K1: 1, BM25B: 0.5, CacheEntries: 1, CacheTTL: time.Second, EmbedTimeout: time.Second, MaxQueryLength: 1}
	cases := []Config{func() Config { c := base; c.TopKDefault = 0; return c }(), func() Config { c := base; c.TopKMax = 0; return c }(), func() Config { c := base; c.KeywordWeight = -1; return c }(), func() Config { c := base; c.KeywordWeight = 0; return c }(), func() Config { c := base; c.RRFConstant = 0; return c }(), func() Config { c := base; c.BM25K1 = 0; return c }(), func() Config { c := base; c.BM25B = 2; return c }(), func() Config { c := base; c.CacheEntries = 0; return c }(), func() Config { c := base; c.CacheTTL = 0; return c }(), func() Config { c := base; c.EmbedTimeout = 0; return c }(), func() Config { c := base; c.MaxQueryLength = 0; return c }()}
	for i, c := range cases {
		if err := c.validate(); err == nil {
			t.Errorf("invalid config %d accepted", i)
		}
	}
}
func TestBM25AndFusionEmptyAndStableTies(t *testing.T) {
	if got := newBM25Index(nil).rank([]string{"x"}, 1, 0.5); len(got) != 0 {
		t.Fatalf("empty corpus rank=%v", got)
	}
	docs := []Document{{PublicName: "z"}, {PublicName: "a"}}
	if got := fuse(docs, [][]int{{0, 1}}, []float64{0}, 60); len(got) != 0 {
		t.Fatalf("zero-weight results=%v", got)
	}
	got := fuse(docs, [][]int{{0, 1}}, []float64{1}, 60)
	if len(got) != 2 || docs[got[0].document].PublicName != "z" {
		t.Fatalf("rank order=%v", got)
	}
	tied := fuse(docs, [][]int{{0, 1}, {1, 0}}, []float64{1, 1}, 60)
	if docs[tied[0].document].PublicName != "a" {
		t.Fatalf("tie break=%v", tied)
	}
}
func TestSearcherAdmissionAndDefaultTopK(t *testing.T) {
	snapshot := Snapshot{Revision: "r", Model: ModelIdentity{Dimensions: 1}, Documents: []Document{{ServerID: "s", PublicName: "a", RawName: "alpha", Description: "match"}}, Backends: []Backend{{Server: "s"}}}
	s, err := NewSearcher(snapshot, nil, Config{})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := s.Search(context.Background(), Request{Query: "match"})
	if err != nil || len(resp.Results) != 1 {
		t.Fatalf("default limit response=%+v err=%v", resp, err)
	}
	for _, request := range []Request{{Query: "  ", TopK: 1}, {Query: "x", TopK: -1}, {Query: "x", TopK: DefaultTopKMax + 1}, {Query: "x", TopK: 1, Servers: []string{"missing"}}} {
		if _, err := s.Search(context.Background(), request); err == nil {
			t.Errorf("accepted invalid request: %+v", request)
		}
	}
	if _, err := NewSearcher(Snapshot{}, nil, Config{}); err == nil {
		t.Fatal("accepted missing snapshot revision")
	}
	if _, err := NewSearcher(Snapshot{Revision: "r"}, nil, Config{}); err == nil {
		t.Fatal("accepted missing dimensions")
	}
	if _, err := NewSearcher(Snapshot{Revision: "r", Model: ModelIdentity{Dimensions: 1}, Documents: []Document{{ServerID: "s"}}}, nil, Config{}); err == nil {
		t.Fatal("accepted unnamed document")
	}
}

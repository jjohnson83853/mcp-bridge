package search_test

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	// T-06 RED gate (r8 plan §4.8): the OpenAI-compatible embedding adapter.
	// Unknown provider names are rejected, never assumed; the request shape is
	// POST {model,input} to base+/embeddings with the provider's own key, and
	// every response is validated (count/index/dimensions/finite/nonzero/size)
	// before a vector reaches the searcher.
	"github.com/jjohnson83853/mcp-bridge/internal/search"
)

const providerDimensions = 8

func providerOptions(server *httptest.Server) search.ProviderOptions {
	return search.ProviderOptions{
		Provider:         search.ProviderOpenAICompatible,
		BaseURL:          server.URL,
		Model:            "text-embedding-3-small",
		APIKey:           "sk-provider-own-key",
		Dimensions:       providerDimensions,
		Timeout:          5 * time.Second,
		MaxConcurrency:   2,
		BatchSize:        2,
		MaxResponseBytes: 1 << 20,
		HTTPClient:       server.Client(),
	}
}

func vectorOf(seed float64) []float64 {
	vector := make([]float64, providerDimensions)
	for d := range vector {
		vector[d] = seed + float64(d)
	}
	return vector
}

func writeVectors(w http.ResponseWriter, indices []int, vectors [][]float64) {
	items := make([]map[string]any, 0, len(indices))
	for i, index := range indices {
		items = append(items, map[string]any{"index": index, "embedding": vectors[i]})
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"data": items})
}

// The adapter must send the documented OpenAI-compatible request and preserve
// input order across bounded batches (plan §4.8).
func TestProviderBatchesRequestsWithOwnCredential(t *testing.T) {
	var (
		mu       sync.Mutex
		batches  [][]string
		auth     []string
		models   []string
		paths    []string
		methods  []string
		received int32
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&received, 1)
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		mu.Lock()
		batches = append(batches, request.Input)
		models = append(models, request.Model)
		auth = append(auth, r.Header.Get("Authorization"))
		paths = append(paths, r.URL.Path)
		methods = append(methods, r.Method)
		mu.Unlock()
		indices := make([]int, len(request.Input))
		vectors := make([][]float64, len(request.Input))
		for i := range request.Input {
			indices[i] = i
			vectors[i] = vectorOf(float64(len(request.Input)))
		}
		writeVectors(w, indices, vectors)
	}))
	defer server.Close()

	provider, err := search.NewProvider(providerOptions(server))
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	inputs := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	vectors, err := provider.Embed(context.Background(), inputs)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vectors) != len(inputs) {
		t.Fatalf("got %d vectors for %d inputs", len(vectors), len(inputs))
	}
	for i, vector := range vectors {
		if len(vector) != providerDimensions {
			t.Fatalf("vector %d has %d dimensions", i, len(vector))
		}
	}
	if got := atomic.LoadInt32(&received); got != 3 {
		t.Fatalf("expected 3 bounded batches of 2, got %d requests", got)
	}
	mu.Lock()
	defer mu.Unlock()
	total := 0
	for i, batch := range batches {
		if len(batch) > 2 {
			t.Fatalf("batch %d exceeded the configured bound: %v", i, batch)
		}
		total += len(batch)
		if methods[i] != http.MethodPost {
			t.Fatalf("method = %s, want POST", methods[i])
		}
		if !strings.HasSuffix(paths[i], "/embeddings") {
			t.Fatalf("path = %s, want .../embeddings", paths[i])
		}
		if models[i] != "text-embedding-3-small" {
			t.Fatalf("model = %s", models[i])
		}
		if auth[i] != "Bearer sk-provider-own-key" {
			t.Fatalf("authorization = %q, want the provider's own key", auth[i])
		}
	}
	if total != len(inputs) {
		t.Fatalf("sent %d inputs, want %d", total, len(inputs))
	}
}

// Base URL normalisation: a configured base is joined with /embeddings without
// doubling or dropping the separator.
func TestProviderJoinsBaseURLWithoutDoublingSeparator(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		writeVectors(w, []int{0}, [][]float64{vectorOf(1)})
	}))
	defer server.Close()

	options := providerOptions(server)
	options.BaseURL = server.URL + "/v1/"
	provider, err := search.NewProvider(options)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if _, err := provider.Embed(context.Background(), []string{"alpha"}); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if path != "/v1/embeddings" {
		t.Fatalf("path = %q, want /v1/embeddings", path)
	}
}

// Every response defect must be rejected rather than stored or ranked.
func TestProviderRejectsMalformedResponses(t *testing.T) {
	cases := []struct {
		name    string
		handler func(w http.ResponseWriter, r *http.Request)
	}{
		{"status", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "sk-provider-own-key", http.StatusInternalServerError)
		}},
		{"error body", func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "sk-provider-own-key"}})
		}},
		{"not json", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<html>")) }},
		{"count", func(w http.ResponseWriter, r *http.Request) {
			writeVectors(w, []int{0}, [][]float64{vectorOf(1)})
		}},
		{"duplicate index", func(w http.ResponseWriter, r *http.Request) {
			writeVectors(w, []int{0, 0}, [][]float64{vectorOf(1), vectorOf(2)})
		}},
		{"out of range index", func(w http.ResponseWriter, r *http.Request) {
			writeVectors(w, []int{0, 5}, [][]float64{vectorOf(1), vectorOf(2)})
		}},
		{"dimensions", func(w http.ResponseWriter, r *http.Request) {
			writeVectors(w, []int{0, 1}, [][]float64{vectorOf(1)[:providerDimensions-1], vectorOf(2)})
		}},
		{"not finite", func(w http.ResponseWriter, r *http.Request) {
			broken := vectorOf(1)
			broken[3] = math.Inf(1)
			writeVectors(w, []int{0, 1}, [][]float64{broken, vectorOf(2)})
		}},
		{"zero vector", func(w http.ResponseWriter, r *http.Request) {
			writeVectors(w, []int{0, 1}, [][]float64{make([]float64, providerDimensions), vectorOf(2)})
		}},
		{"oversized", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[`))
			for i := 0; i < 1<<18; i++ {
				_, _ = w.Write([]byte("1,"))
			}
			_, _ = w.Write([]byte("1]}]}"))
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(test.handler))
			defer server.Close()
			options := providerOptions(server)
			options.MaxResponseBytes = 1 << 16
			provider, err := search.NewProvider(options)
			if err != nil {
				t.Fatalf("NewProvider: %v", err)
			}
			vectors, err := provider.Embed(context.Background(), []string{"alpha", "beta"})
			if err == nil {
				t.Fatalf("expected an error, got %d vectors", len(vectors))
			}
			if strings.Contains(err.Error(), "sk-provider-own-key") {
				t.Fatalf("error leaked the provider credential: %v", err)
			}
		})
	}
}

// A failing batch must not yield a partial result set.
func TestProviderFailsWholeBatchOnOneBadResponse(t *testing.T) {
	var received int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&received, 1)
		writeVectors(w, []int{0, 1}, [][]float64{vectorOf(1), vectorOf(2)[:4]})
	}))
	defer server.Close()

	provider, err := search.NewProvider(providerOptions(server))
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	vectors, err := provider.Embed(context.Background(), []string{"alpha", "beta"})
	if err == nil {
		t.Fatalf("expected an error, got %v", vectors)
	}
	if vectors != nil {
		t.Fatalf("expected no partial vectors, got %d", len(vectors))
	}
}

// Concurrency stays inside the configured bound.
func TestProviderBoundsConcurrency(t *testing.T) {
	var (
		inFlight    int32
		maxInFlight int32
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&inFlight, 1)
		for {
			observed := atomic.LoadInt32(&maxInFlight)
			if current <= observed || atomic.CompareAndSwapInt32(&maxInFlight, observed, current) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		writeVectors(w, []int{0}, [][]float64{vectorOf(1)})
	}))
	defer server.Close()

	options := providerOptions(server)
	options.MaxConcurrency = 2
	options.BatchSize = 1
	provider, err := search.NewProvider(options)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if _, err := provider.Embed(context.Background(), []string{"a", "b", "c", "d", "e", "f"}); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if observed := atomic.LoadInt32(&maxInFlight); observed > 2 {
		t.Fatalf("observed %d concurrent requests, want at most 2", observed)
	}
}

// Cancellation and deadlines reach the request.
func TestProviderHonorsContextCancellation(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer func() {
		close(release)
		server.Close()
	}()

	provider, err := search.NewProvider(providerOptions(server))
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := provider.Embed(ctx, []string{"alpha"}); err == nil {
		t.Fatal("expected the cancelled request to fail")
	}
}

// An empty batch is answered without any upstream request.
func TestProviderEmbedsNoInputsWithoutRequest(t *testing.T) {
	var received int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&received, 1)
		writeVectors(w, nil, nil)
	}))
	defer server.Close()

	provider, err := search.NewProvider(providerOptions(server))
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	vectors, err := provider.Embed(context.Background(), nil)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vectors) != 0 {
		t.Fatalf("got %d vectors for no inputs", len(vectors))
	}
	if got := atomic.LoadInt32(&received); got != 0 {
		t.Fatalf("expected no upstream request, got %d", got)
	}
}

// Unknown providers are rejected, never assumed (plan §4.8).
func TestNewProviderRejectsUnknownProvider(t *testing.T) {
	for _, name := range []string{"", "gemini", "anthropic", "openai"} {
		options := providerOptions(httptest.NewServer(http.NotFoundHandler()))
		options.Provider = name
		if _, err := search.NewProvider(options); err == nil {
			t.Fatalf("provider %q was accepted", name)
		}
	}
}

func TestNewProviderRequiresCompleteIdentity(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	cases := []struct {
		name   string
		mutate func(*search.ProviderOptions)
	}{
		{"base url", func(o *search.ProviderOptions) { o.BaseURL = "" }},
		{"non-http base url", func(o *search.ProviderOptions) { o.BaseURL = "ftp://example.test" }},
		{"model", func(o *search.ProviderOptions) { o.Model = "" }},
		{"dimensions", func(o *search.ProviderOptions) { o.Dimensions = 0 }},
		{"dimensions over the ceiling", func(o *search.ProviderOptions) { o.Dimensions = search.MaxProviderDimensions + 1 }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			options := providerOptions(server)
			test.mutate(&options)
			if _, err := search.NewProvider(options); err == nil {
				t.Fatal("expected a rejection")
			}
		})
	}
}

// The adapter satisfies the searcher's Provider contract.
func TestProviderImplementsSearchProvider(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	provider, err := search.NewProvider(providerOptions(server))
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	var _ search.Provider = provider
}

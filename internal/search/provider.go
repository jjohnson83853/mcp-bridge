package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ProviderOpenAICompatible is the only embedding provider this build speaks:
// the OpenAI-compatible embeddings endpoint, POST {model,input} to
// base+/embeddings with the provider's own credential (plan §4.8). Any other
// configured name is rejected rather than mapped onto an assumed contract.
const ProviderOpenAICompatible = "openai-compatible"

// Provider bounds pinned by the plan (§4.2, §4.8).
const (
	DefaultProviderBatchSize    = 64
	DefaultProviderConcurrency  = 4
	DefaultProviderTimeout      = 10 * time.Second
	DefaultMaxEmbedResponseSize = 1 << 20
	// MaxProviderDimensions is the plan's configured embedding-width ceiling.
	MaxProviderDimensions = 4096
)

// ProviderOptions is the operator-selected embedding provider identity. The
// credential is the provider's own key: it is never sent upstream to a tool
// server, never logged, and never included in an error.
type ProviderOptions struct {
	Provider         string
	BaseURL          string
	Model            string
	APIKey           string
	Dimensions       int
	Timeout          time.Duration
	MaxConcurrency   int
	BatchSize        int
	MaxResponseBytes int64
	HTTPClient       *http.Client
}

// NewProvider validates one embedding provider configuration and builds the
// adapter for it. It fails closed on an unknown provider name, an unusable base
// URL, or a dimensionality that does not match the pinned embedding width, so a
// process never runs against a contract it cannot honour.
func NewProvider(options ProviderOptions) (Provider, error) {
	if options.Provider != ProviderOpenAICompatible {
		return nil, fmt.Errorf("search: unsupported embedding provider %q", options.Provider)
	}
	if options.Model == "" {
		return nil, errors.New("search: embedding provider needs a model")
	}
	if options.Dimensions < 1 || options.Dimensions > MaxProviderDimensions {
		return nil, fmt.Errorf("search: embedding dimensions must be within 1..%d", MaxProviderDimensions)
	}
	endpoint, err := embeddingEndpoint(options.BaseURL)
	if err != nil {
		return nil, err
	}
	if options.Timeout <= 0 {
		options.Timeout = DefaultProviderTimeout
	}
	if options.MaxConcurrency <= 0 {
		options.MaxConcurrency = DefaultProviderConcurrency
	}
	if options.BatchSize <= 0 {
		options.BatchSize = DefaultProviderBatchSize
	}
	if options.MaxResponseBytes <= 0 {
		options.MaxResponseBytes = DefaultMaxEmbedResponseSize
	}
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: options.Timeout}
	}
	return &openAIProvider{
		endpoint:    endpoint,
		model:       options.Model,
		apiKey:      options.APIKey,
		client:      client,
		concurrency: options.MaxConcurrency,
		batchSize:   options.BatchSize,
		maxBytes:    options.MaxResponseBytes,
		dimensions:  options.Dimensions,
	}, nil
}

// embeddingEndpoint joins the configured base URL with the embeddings path.
func embeddingEndpoint(base string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" ||
		parsed.RawQuery != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("search: embedding base_url must be an absolute HTTP(S) URL")
	}
	return strings.TrimSuffix(parsed.String(), "/") + "/embeddings", nil
}

type openAIProvider struct {
	endpoint    string
	model       string
	apiKey      string
	client      *http.Client
	concurrency int
	batchSize   int
	maxBytes    int64
	dimensions  int
}

// Embed answers one bounded batch request per batch of inputs, running at most
// the configured number of requests at once. Input order is preserved, and any
// defect in any response fails the whole call: a partially embedded corpus would
// silently rank on a mixture of two models.
func (p *openAIProvider) Embed(ctx context.Context, inputs []string) ([][]float64, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	vectors := make([][]float64, len(inputs))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wait    sync.WaitGroup
		slots   = make(chan struct{}, p.concurrency)
		failure error
		once    sync.Once
	)
	for start := 0; start < len(inputs); start += p.batchSize {
		end := min(start+p.batchSize, len(inputs))
		wait.Add(1)
		go func(start, end int) {
			defer wait.Done()
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-slots }()
			batch, err := p.embedBatch(ctx, inputs[start:end])
			if err != nil {
				once.Do(func() {
					failure = err
					cancel()
				})
				return
			}
			copy(vectors[start:end], batch)
		}(start, end)
	}
	wait.Wait()
	if failure != nil {
		return nil, failure
	}
	if err := ctx.Err(); err != nil && ctx.Err() != context.Canceled {
		return nil, fmt.Errorf("search: embedding request: %w", err)
	}
	return vectors, nil
}

// embedBatch performs exactly one embeddings request and validates every vector
// it returns.
func (p *openAIProvider) embedBatch(ctx context.Context, inputs []string) ([][]float64, error) {
	body, err := json.Marshal(struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}{Model: p.model, Input: inputs})
	if err != nil {
		return nil, fmt.Errorf("search: encode embedding request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("search: build embedding request")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if p.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	response, err := p.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("search: embedding request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<16))
		_ = response.Body.Close()
	}()
	payload, err := io.ReadAll(io.LimitReader(response.Body, p.maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("search: read embedding response: %w", err)
	}
	if int64(len(payload)) > p.maxBytes {
		return nil, errors.New("search: embedding response exceeds the configured bound")
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search: embedding provider returned status %d", response.StatusCode)
	}
	return decodeEmbeddings(payload, len(inputs), p.dimensions)
}

// decodeEmbeddings validates the OpenAI-compatible response envelope. The
// upstream message text is never propagated: only a finite, payload-free reason
// is reported.
func decodeEmbeddings(payload []byte, want, dimensions int) ([][]float64, error) {
	var response struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, errors.New("search: embedding response is not valid JSON")
	}
	if response.Error != nil {
		return nil, errors.New("search: embedding provider reported an error")
	}
	if len(response.Data) != want {
		return nil, fmt.Errorf("search: embedding provider returned %d vectors for %d inputs", len(response.Data), want)
	}
	vectors := make([][]float64, want)
	seen := make([]bool, want)
	for _, item := range response.Data {
		if item.Index < 0 || item.Index >= want || seen[item.Index] {
			return nil, errors.New("search: embedding response index is out of range or repeated")
		}
		if !validVector(item.Embedding, dimensions) {
			return nil, errors.New("search: embedding response contains an unusable vector")
		}
		seen[item.Index] = true
		vectors[item.Index] = item.Embedding
	}
	return vectors, nil
}

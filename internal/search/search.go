// Package search implements the bridge's global hybrid tool search: BM25 over
// the formatter-v1 tool text, exact normalised cosine over current embeddings,
// and reciprocal-rank fusion of the two rankings (plan §4.8).
package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Bounds and defaults pinned by the plan (§4.2, §4.8). The embedding width is
// not one of them: it is the operator's configured model identity, bounded to
// 1..4096 by the configuration layer and carried on every Snapshot, so a
// vector is only ever validated against the width it was produced at.
const (
	// FormatVersion tags the formatter-v1 content identity.
	FormatVersion = "v1"

	DefaultTopK          = 10
	DefaultTopKMax       = 50
	DefaultKeywordWeight = 0.5
	DefaultVectorWeight  = 0.5
	DefaultRRFConstant   = 60.0
	DefaultBM25K1        = 1.2
	DefaultBM25B         = 0.75
	DefaultCacheEntries  = 256
	DefaultCacheTTL      = 5 * time.Minute
	DefaultEmbedTimeout  = 10 * time.Second
	DefaultMaxQueryRunes = 2048
)

// Failure is a safe, payload-free search failure the tool layer maps to an
// invalid-params style error.
type Failure struct {
	Code    string
	Message string
}

func (f *Failure) Error() string { return f.Code + ": " + f.Message }

// Tool-layer error codes.
const (
	CodeInvalidArguments = "invalid_arguments"
	CodeInvalidFilter    = "invalid_filter"
)

// Config is the search admission configuration.
type Config struct {
	TopKDefault    int
	TopKMax        int
	KeywordWeight  float64
	VectorWeight   float64
	RRFConstant    float64
	BM25K1         float64
	BM25B          float64
	CacheEntries   int
	CacheTTL       time.Duration
	EmbedTimeout   time.Duration
	MaxQueryLength int
}

func (c Config) withDefaults() Config {
	if c.TopKDefault == 0 {
		c.TopKDefault = DefaultTopK
	}
	if c.TopKMax == 0 {
		c.TopKMax = DefaultTopKMax
	}
	if c.KeywordWeight == 0 {
		c.KeywordWeight = DefaultKeywordWeight
	}
	if c.VectorWeight == 0 {
		c.VectorWeight = DefaultVectorWeight
	}
	if c.RRFConstant == 0 {
		c.RRFConstant = DefaultRRFConstant
	}
	if c.BM25K1 == 0 {
		c.BM25K1 = DefaultBM25K1
	}
	if c.BM25B == 0 {
		c.BM25B = DefaultBM25B
	}
	if c.CacheEntries == 0 {
		c.CacheEntries = DefaultCacheEntries
	}
	if c.CacheTTL == 0 {
		c.CacheTTL = DefaultCacheTTL
	}
	if c.EmbedTimeout == 0 {
		c.EmbedTimeout = DefaultEmbedTimeout
	}
	if c.MaxQueryLength == 0 {
		c.MaxQueryLength = DefaultMaxQueryRunes
	}
	return c
}

func (c Config) validate() error {
	switch {
	case c.TopKDefault < 1:
		return errors.New("search: TopKDefault must be positive")
	case c.TopKMax < c.TopKDefault:
		return errors.New("search: TopKMax must be at least TopKDefault")
	case c.KeywordWeight < 0 || c.VectorWeight < 0:
		return errors.New("search: weights must be non-negative")
	case c.KeywordWeight+c.VectorWeight <= 0:
		return errors.New("search: at least one weight must be positive")
	case c.RRFConstant <= 0:
		return errors.New("search: RRFConstant must be positive")
	case c.BM25K1 <= 0 || c.BM25B < 0 || c.BM25B > 1:
		return errors.New("search: BM25 parameters out of range")
	case c.CacheEntries < 1:
		return errors.New("search: CacheEntries must be positive")
	case c.CacheTTL <= 0:
		return errors.New("search: CacheTTL must be positive")
	case c.EmbedTimeout <= 0:
		return errors.New("search: EmbedTimeout must be positive")
	case c.MaxQueryLength < 1:
		return errors.New("search: MaxQueryLength must be positive")
	}
	return nil
}

// Request is one search admission: query text, result bound and an optional
// configured-server filter. There is deliberately no namespace selector.
type Request struct {
	Query   string
	TopK    int
	Servers []string
}

// DecodeRequest strictly decodes the search_tools arguments: unknown fields
// (including any legacy namespace selector) are rejected, and every bound is
// rejected rather than clamped.
func DecodeRequest(raw json.RawMessage, cfg Config) (Request, error) {
	cfg = cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		return Request{}, err
	}
	var wire struct {
		Query   *string   `json:"query"`
		TopK    *int      `json:"top_k"`
		Servers *[]string `json:"servers"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return Request{}, invalidArguments("arguments are not a valid search request")
	}
	request := Request{}
	if wire.Query != nil {
		request.Query = *wire.Query
	}
	if strings.TrimSpace(request.Query) == "" {
		return Request{}, invalidArguments("query must not be empty")
	}
	if len([]rune(request.Query)) > cfg.MaxQueryLength {
		return Request{}, invalidArguments("query exceeds the configured length bound")
	}
	request.TopK = cfg.TopKDefault
	if wire.TopK != nil {
		if *wire.TopK < 1 || *wire.TopK > cfg.TopKMax {
			return Request{}, invalidArguments("top_k is outside the configured bounds")
		}
		request.TopK = *wire.TopK
	}
	if wire.Servers != nil {
		if len(*wire.Servers) == 0 {
			request.Servers = nil
		} else if len(*wire.Servers) > 32 {
			return Request{}, invalidArguments("servers exceeds maximum of 32 entries")
		} else {
			request.Servers = append([]string(nil), (*wire.Servers)...)
		}
	}
	return request, nil
}

func invalidArguments(message string) error {
	return &Failure{Code: CodeInvalidArguments, Message: message}
}

// ModelIdentity is the non-secret embedding model identity. Credentials are
// never part of it, so credential rotation alone reuses cached vectors.
type ModelIdentity struct {
	Provider      string
	BaseURL       string
	Model         string
	Revision      string
	Dimensions    int
	FormatVersion string
}

// Digest is the stable, secret-free identity digest used in cache keys and
// embedding records.
func (m ModelIdentity) Digest() string {
	payload := strings.Join([]string{
		m.Provider, m.BaseURL, m.Model, m.Revision,
		strings.TrimSpace(strings.Join([]string{itoa(m.Dimensions), m.FormatVersion}, ":")),
	}, "\n")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 8)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

// ContentHash is the formatter-v1 content identity of one tool: raw name, a
// newline, the description and the format version. The exact definition hash is
// separate (plan §4.8, §4.9).
func ContentHash(rawName, description string) string {
	sum := sha256.Sum256([]byte(FormatVersion + "\n" + rawName + "\n" + description))
	return hex.EncodeToString(sum[:])
}

// Document is one searchable tool with its current embedding vector.
type Document struct {
	ServerID    string
	RawName     string
	PublicName  string
	Description string
	Pinned      bool
	Vector      []float64
}

func (d Document) text() string { return d.RawName + "\n" + d.Description }

// Backend is one configured server's health and provenance.
type Backend struct {
	Server        string
	Status        string
	Availability  string
	Reason        string
	Revision      string
	LastSuccessAt time.Time
}

// Snapshot is the immutable aggregate a search reads: the current revision, the
// live model identity, the indexed tools and the backend health rows.
type Snapshot struct {
	Revision  string
	Model     ModelIdentity
	Documents []Document
	Backends  []Backend
}

// Provider embeds a bounded batch of inputs. Unknown providers are rejected by
// the adapter layer, never assumed (plan §4.8).
type Provider interface {
	Embed(ctx context.Context, inputs []string) ([][]float64, error)
}

// Result is one ranked match with its provenance.
type Result struct {
	ToolName        string
	Server          string
	UpstreamName    string
	Description     string
	Score           float64
	CatalogRevision string
	CatalogStatus   string
	Availability    string
	LastSuccessAt   time.Time
	Pinned          bool
}

// Response is the search_tools structured result.
type Response struct {
	CatalogRevision string
	Mode            string
	DegradedReasons []string
	Results         []Result
	Backends        []Backend
}

// Search modes (plan §4.4). A degraded mode always carries a finite reason.
const (
	ModeHybrid        = "hybrid"
	ModeHybridPartial = "hybrid_partial"
	ModeKeyword       = "keyword"

	reasonEmbedUnavailable = "embed_provider_unavailable"
	reasonVectorsAbsent    = "vectors_absent"
)

// Option customises a Searcher.
type Option func(*Searcher)

// WithClock injects the clock used for cache TTL evaluation.
func WithClock(clock func() time.Time) Option {
	return func(s *Searcher) {
		if clock != nil {
			s.clock = clock
		}
	}
}

// Searcher answers global searches against one immutable snapshot.
type Searcher struct {
	snapshot Snapshot
	provider Provider
	cfg      Config
	clock    func() time.Time

	index   *bm25Index
	cache   *vectorCache
	servers map[string]Backend
}

// NewSearcher validates the snapshot and configuration and builds the index.
func NewSearcher(snapshot Snapshot, provider Provider, cfg Config, opts ...Option) (*Searcher, error) {
	cfg = cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if snapshot.Revision == "" {
		return nil, errors.New("search: snapshot revision is required")
	}
	if snapshot.Model.Dimensions <= 0 {
		return nil, errors.New("search: model dimensions must be positive")
	}
	for _, document := range snapshot.Documents {
		if document.PublicName == "" || document.ServerID == "" {
			return nil, errors.New("search: every document needs a server and public name")
		}
		if document.Vector != nil && len(document.Vector) != snapshot.Model.Dimensions {
			return nil, errors.New("search: document vector does not match the model dimensions")
		}
	}
	searcher := &Searcher{
		snapshot: snapshot,
		provider: provider,
		cfg:      cfg,
		clock:    time.Now,
		index:    newBM25Index(snapshot.Documents),
		cache:    newVectorCache(cfg.CacheEntries, cfg.CacheTTL),
		servers:  make(map[string]Backend, len(snapshot.Backends)),
	}
	for _, backend := range snapshot.Backends {
		searcher.servers[backend.Server] = backend
	}
	for _, option := range opts {
		option(searcher)
	}
	return searcher, nil
}

// Search answers one query. It is deterministic: ties break on the stable
// public name and unmatched documents are omitted.
func (s *Searcher) Search(ctx context.Context, request Request) (Response, error) {
	if err := s.admit(&request); err != nil {
		return Response{}, err
	}
	eligible, backends, globalToEligible := s.eligible(request.Servers)
	response := Response{
		CatalogRevision: s.snapshot.Revision,
		Mode:            ModeHybrid,
		Results:         []Result{},
		Backends:        backends,
	}

	lists := make([][]int, 0, 2)
	weights := make([]float64, 0, 2)

	rawLexical := s.index.rank(tokenize(request.Query), s.cfg.BM25K1, s.cfg.BM25B)
	lexical := make([]int, 0, len(rawLexical))
	for _, g := range rawLexical {
		if e := globalToEligible[g]; e != -1 {
			lexical = append(lexical, e)
		}
	}
	lists = append(lists, lexical)
	weights = append(weights, s.cfg.KeywordWeight)

	vectorizable := 0
	withVector := make([]int, 0, len(eligible))
	for index, document := range eligible {
		if validVector(document.Vector, s.snapshot.Model.Dimensions) {
			vectorizable++
			withVector = append(withVector, index)
		}
	}

	switch {
	case s.provider == nil:
		response.Mode = ModeKeyword
		response.DegradedReasons = append(response.DegradedReasons, reasonEmbedUnavailable)
	case vectorizable == 0:
		response.Mode = ModeKeyword
		response.DegradedReasons = append(response.DegradedReasons, reasonVectorsAbsent)
	default:
		queryVector, err := s.queryVector(ctx, request)
		if err != nil {
			response.Mode = ModeKeyword
			response.DegradedReasons = append(response.DegradedReasons, reasonEmbedUnavailable)
			break
		}
		if vectorizable < len(eligible) {
			response.Mode = ModeHybridPartial
		}
		semantic := s.rankVectors(queryVector, eligible, withVector)
		if len(semantic) > 0 {
			lists = append(lists, semantic)
			weights = append(weights, s.cfg.VectorWeight)
		}
	}

	fused := fuse(eligible, lists, weights, s.cfg.RRFConstant)
	limit := request.TopK
	if limit > len(fused) {
		limit = len(fused)
	}
	for _, match := range fused[:limit] {
		document := eligible[match.document]
		response.Results = append(response.Results, s.result(document, match.score))
	}
	return response, nil
}

func (s *Searcher) admit(request *Request) error {
	if strings.TrimSpace(request.Query) == "" {
		return invalidArguments("query must not be empty")
	}
	if len([]rune(request.Query)) > s.cfg.MaxQueryLength {
		return invalidArguments("query exceeds the configured length bound")
	}
	if request.TopK == 0 {
		request.TopK = s.cfg.TopKDefault
	}
	if request.TopK < 1 || request.TopK > s.cfg.TopKMax {
		return invalidArguments("top_k is outside the configured bounds")
	}
	if request.Servers != nil && len(request.Servers) == 0 {
		return &Failure{Code: CodeInvalidFilter, Message: "servers must not be empty when provided"}
	}
	for _, server := range request.Servers {
		if _, ok := s.servers[server]; !ok {
			return &Failure{Code: CodeInvalidFilter, Message: "servers must name configured servers"}
		}
	}
	return nil
}

// eligible filters the configured server set before any scoring and returns the
// matching backend rows sorted by server, plus a mapping from every global
// document index to its index in the returned eligible slice (-1 when filtered
// out). The lexical rank list is global-indexed, so this map is what keeps a
// filtered corpus from indexing past the eligible slice.
func (s *Searcher) eligible(servers []string) ([]Document, []Backend, []int) {
	globalToEligible := make([]int, len(s.snapshot.Documents))
	if servers == nil {
		for i := range globalToEligible {
			globalToEligible[i] = i
		}
		documents := append([]Document(nil), s.snapshot.Documents...)
		backends := append([]Backend(nil), s.snapshot.Backends...)
		sort.Slice(backends, func(i, j int) bool { return backends[i].Server < backends[j].Server })
		return documents, backends, globalToEligible
	}
	for i := range globalToEligible {
		globalToEligible[i] = -1
	}
	selected := make(map[string]bool, len(servers))
	for _, server := range servers {
		selected[server] = true
	}
	documents := make([]Document, 0, len(s.snapshot.Documents))
	for globalIdx, document := range s.snapshot.Documents {
		if selected[document.ServerID] {
			documents = append(documents, document)
			globalToEligible[globalIdx] = len(documents) - 1
		}
	}
	backends := make([]Backend, 0, len(selected))
	for _, backend := range s.snapshot.Backends {
		if selected[backend.Server] {
			backends = append(backends, backend)
		}
	}
	sort.Slice(backends, func(i, j int) bool { return backends[i].Server < backends[j].Server })
	return documents, backends, globalToEligible
}

func (s *Searcher) queryVector(ctx context.Context, request Request) ([]float64, error) {
	servers := normalizeServers(request.Servers)
	key := cacheKey{
		query:    request.Query,
		servers:  strings.Join(servers, ","),
		revision: s.snapshot.Revision,
		model:    s.snapshot.Model.Digest(),
	}
	if vector, ok := s.cache.get(key, s.clock()); ok {
		return vector, nil
	}
	embedCtx, cancel := context.WithTimeout(ctx, s.cfg.EmbedTimeout)
	defer cancel()
	vectors, err := s.provider.Embed(embedCtx, []string{request.Query})
	if err != nil {
		return nil, err
	}
	if len(vectors) != 1 || !validVector(vectors[0], s.snapshot.Model.Dimensions) {
		return nil, errors.New("search: provider returned an unusable query embedding")
	}
	vector := append([]float64(nil), vectors[0]...)
	s.cache.put(key, vector, s.clock())
	return vector, nil
}

func (s *Searcher) rankVectors(query []float64, documents []Document, candidates []int) []int {
	type scored struct {
		index      int
		similarity float64
	}
	ranked := make([]scored, 0, len(candidates))
	for _, index := range candidates {
		similarity, ok := cosine(query, documents[index].Vector)
		if !ok || similarity <= 0 {
			continue
		}
		ranked = append(ranked, scored{index: index, similarity: similarity})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].similarity != ranked[j].similarity {
			return ranked[i].similarity > ranked[j].similarity
		}
		return documents[ranked[i].index].PublicName < documents[ranked[j].index].PublicName
	})
	order := make([]int, len(ranked))
	for i, entry := range ranked {
		order[i] = entry.index
	}
	return order
}

func (s *Searcher) result(document Document, score float64) Result {
	backend := s.servers[document.ServerID]
	return Result{
		ToolName:        document.PublicName,
		Server:          document.ServerID,
		UpstreamName:    document.RawName,
		Description:     document.Description,
		Score:           score,
		CatalogRevision: s.snapshot.Revision,
		CatalogStatus:   backend.Status,
		Availability:    backend.Availability,
		LastSuccessAt:   backend.LastSuccessAt,
		Pinned:          document.Pinned,
	}
}

// match is one fused document index with its RRF score.
type match struct {
	document int
	score    float64
}

// fuse applies reciprocal-rank fusion: every list contributes
// weight/(constant+rank) for the documents it ranks, and documents absent from
// every list are omitted.
func fuse(documents []Document, lists [][]int, weights []float64, constant float64) []match {
	scores := make([]float64, len(documents))
	for listIndex, list := range lists {
		weight := weights[listIndex]
		if weight == 0 {
			continue
		}
		for rank, document := range list {
			scores[document] += weight / (constant + float64(rank+1))
		}
	}
	fused := make([]match, 0, len(documents))
	for index, score := range scores {
		if score > 0 {
			fused = append(fused, match{document: index, score: score})
		}
	}
	sort.SliceStable(fused, func(i, j int) bool {
		if fused[i].score != fused[j].score {
			return fused[i].score > fused[j].score
		}
		return documents[fused[i].document].PublicName < documents[fused[j].document].PublicName
	})
	return fused
}

// tokenize casefolds and splits on non-alphanumeric runes.
func tokenize(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func normalizeServers(servers []string) []string {
	if len(servers) == 0 {
		return nil
	}
	unique := make(map[string]bool, len(servers))
	for _, server := range servers {
		unique[server] = true
	}
	normalized := make([]string, 0, len(unique))
	for server := range unique {
		normalized = append(normalized, server)
	}
	sort.Strings(normalized)
	return normalized
}

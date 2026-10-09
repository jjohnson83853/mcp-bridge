package config

import (
	"fmt"
	"time"
)

// Command is the run-loop view of one validated configuration document: the
// bind addresses, storage location, catalog schedule, and upstream defaults the
// process needs (plan §4.2/§4.6/§4.7). Every value is required except the
// host/origin lists; a document that omits a required one is rejected by name
// rather than run with a zero value.
type Command struct {
	HTTPListen           string
	OperationsListen     string
	MaxRequestBytes      int64
	RequestsPerSecond    float64
	Burst                int
	MaxSessions          int
	SessionIdle          time.Duration
	OpsRequestsPerSecond float64
	OpsBurst             int

	StorageDirectory string
	MaxSnapshotBytes int64

	RefreshInterval time.Duration
	StaleAfter      time.Duration

	ConnectTimeout      time.Duration
	CallTimeout         time.Duration
	RefreshTimeout      time.Duration
	MaxConcurrentCalls  int
	BreakerThreshold    int
	BreakerOpenDuration time.Duration
	MaxEventBytes       int64
	MaxResponseBytes    int64

	Search    SearchSettings
	Embedding EmbeddingSettings

	// AllowedHosts are the request Host values the inbound listener accepts; an
	// empty list disables the host check (plan §4.5/§4.6).
	AllowedHosts []string
	// AllowedOrigins are the request Origin values the inbound listener accepts;
	// an empty list disables the origin check.
	AllowedOrigins []string
}

// SearchSettings is the hybrid-search configuration the tool surface needs.
type SearchSettings struct {
	DefaultTopK       int
	MaxTopK           int
	Timeout           time.Duration
	QueryCacheEntries int
	QueryCacheTTL     time.Duration
	KeywordWeight     float64
	EmbeddingWeight   float64
	RRFConstant       int
}

// EmbeddingSettings is the operator-selected embedding model identity. The
// credential is deliberately absent: only the client that performs embedding
// requests resolves it.
type EmbeddingSettings struct {
	Provider         string
	Model            string
	ModelRevision    string
	BaseURL          string
	Dimensions       int
	Timeout          time.Duration
	MaxConcurrency   int
	BatchSize        int
	MaxResponseBytes int64
}

// Command resolves the run-loop settings, failing closed on the first missing or
// mistyped value instead of defaulting it.
func (c Config) Command() (Command, error) {
	var settings Command
	var err error
	read := func(section, key string, target any) {
		if err != nil {
			return
		}
		err = readSetting(c, section, key, target)
	}

	read("http", "listen", &settings.HTTPListen)
	read("operations", "listen", &settings.OperationsListen)
	read("http", "max_request_bytes", &settings.MaxRequestBytes)
	read("http", "requests_per_second", &settings.RequestsPerSecond)
	read("http", "burst", &settings.Burst)
	read("http", "max_sessions", &settings.MaxSessions)
	read("http", "session_idle_timeout", &settings.SessionIdle)
	read("operations", "requests_per_second", &settings.OpsRequestsPerSecond)
	read("operations", "burst", &settings.OpsBurst)
	read("storage", "directory", &settings.StorageDirectory)
	read("storage", "max_snapshot_bytes", &settings.MaxSnapshotBytes)
	read("catalog", "refresh_interval", &settings.RefreshInterval)
	read("catalog", "stale_after", &settings.StaleAfter)
	read("upstream_defaults", "connect_timeout", &settings.ConnectTimeout)
	read("upstream_defaults", "call_timeout", &settings.CallTimeout)
	read("upstream_defaults", "refresh_timeout", &settings.RefreshTimeout)
	read("upstream_defaults", "max_concurrent_calls", &settings.MaxConcurrentCalls)
	read("upstream_defaults", "breaker.consecutive_failures", &settings.BreakerThreshold)
	read("upstream_defaults", "breaker.open_duration", &settings.BreakerOpenDuration)
	read("upstream_defaults", "max_event_bytes", &settings.MaxEventBytes)
	read("upstream_defaults", "max_response_bytes", &settings.MaxResponseBytes)
	read("search", "default_top_k", &settings.Search.DefaultTopK)
	read("search", "max_top_k", &settings.Search.MaxTopK)
	read("search", "timeout", &settings.Search.Timeout)
	read("search", "query_cache_entries", &settings.Search.QueryCacheEntries)
	read("search", "query_cache_ttl", &settings.Search.QueryCacheTTL)
	read("search", "keyword_weight", &settings.Search.KeywordWeight)
	read("search", "embedding_weight", &settings.Search.EmbeddingWeight)
	read("search", "rrf_constant", &settings.Search.RRFConstant)
	read("embedding", "provider", &settings.Embedding.Provider)
	read("embedding", "model", &settings.Embedding.Model)
	read("embedding", "model_revision", &settings.Embedding.ModelRevision)
	read("embedding", "base_url", &settings.Embedding.BaseURL)
	read("embedding", "dimensions", &settings.Embedding.Dimensions)
	read("embedding", "timeout", &settings.Embedding.Timeout)
	read("embedding", "max_concurrency", &settings.Embedding.MaxConcurrency)
	read("embedding", "batch_size", &settings.Embedding.BatchSize)
	read("embedding", "max_response_bytes", &settings.Embedding.MaxResponseBytes)
	// The host/origin lists are OPTIONAL: an absent key leaves the slice nil
	// and disables the check (plan §4.5/§4.6), unlike every other value above.
	if err == nil {
		err = readOptionalSequence(c, "http", "allowed_hosts", &settings.AllowedHosts)
	}
	if err == nil {
		err = readOptionalSequence(c, "http", "allowed_origins", &settings.AllowedOrigins)
	}
	if err != nil {
		return Command{}, err
	}
	return settings, nil
}

// readSetting copies one scalar setting into its typed destination. It reports
// the section and key, never the value, so a rejected document cannot leak a
// secret through the error.
func readSetting(c Config, section, key string, target any) error {
	name := section + "." + key
	leaf := c.section(section)
	walk := splitDots(key)
	for _, part := range walk[:len(walk)-1] {
		child, ok := leaf[part].(map[string]any)
		if !ok {
			return fmt.Errorf("configuration is missing %s", name)
		}
		leaf = child
	}
	value, ok := leaf[walk[len(walk)-1]]
	if !ok {
		return fmt.Errorf("configuration is missing %s", name)
	}
	switch destination := target.(type) {
	case *string:
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s must be a string", name)
		}
		*destination = text
	case *int:
		number, ok := integerValue(value)
		if !ok {
			return fmt.Errorf("%s must be an integer", name)
		}
		*destination = int(number)
	case *int64:
		number, ok := integerValue(value)
		if !ok {
			return fmt.Errorf("%s must be an integer", name)
		}
		*destination = number
	case *float64:
		number, ok := numberValue(value)
		if !ok {
			return fmt.Errorf("%s must be a number", name)
		}
		*destination = number
	case *time.Duration:
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s must be a duration string", name)
		}
		duration, err := time.ParseDuration(text)
		if err != nil {
			return fmt.Errorf("%s must be a duration string", name)
		}
		*destination = duration
	default:
		return fmt.Errorf("configuration reader has no destination for %s", name)
	}
	return nil
}

// readOptionalSequence copies an optional sequence of strings into its typed
// destination. Unlike readSetting, an absent key is legal: it leaves the
// destination nil, so the run loop treats the check as disabled (plan
// §4.5/§4.6). A present value must be a sequence of strings and is copied, so
// the returned slice never aliases caller-visible memory. Element and sequence
// validation belong to admission (Parse); these checks only keep the storage
// conversion total.
func readOptionalSequence(c Config, section, key string, destination *[]string) error {
	name := section + "." + key
	value, ok := c.section(section)[key]
	if !ok {
		return nil
	}
	sequence, ok := value.([]any)
	if !ok {
		return fmt.Errorf("%s must be a sequence of strings", name)
	}
	if len(sequence) == 0 {
		*destination = []string{}
		return nil
	}
	entries := make([]string, len(sequence))
	for index, entry := range sequence {
		text, ok := entry.(string)
		if !ok {
			return fmt.Errorf("%s must be a sequence of strings", name)
		}
		entries[index] = text
	}
	*destination = entries
	return nil
}

func (c Config) section(name string) map[string]any {
	switch name {
	case "http":
		return c.HTTP
	case "operations":
		return c.Operations
	case "storage":
		return c.Storage
	case "catalog":
		return c.Catalog
	case "search":
		return c.Search
	case "embedding":
		return c.Embedding
	case "upstream_defaults":
		return c.UpstreamDefaults
	default:
		return nil
	}
}

func integerValue(value any) (int64, bool) {
	switch number := value.(type) {
	case int:
		return int64(number), true
	case int64:
		return number, true
	case uint64:
		return int64(number), true
	case float64:
		if number != float64(int64(number)) {
			return 0, false
		}
		return int64(number), true
	default:
		return 0, false
	}
}

func numberValue(value any) (float64, bool) {
	switch number := value.(type) {
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	case float64:
		return number, true
	default:
		return 0, false
	}
}

func splitDots(key string) []string {
	parts := make([]string, 0, 2)
	start := 0
	for index := 0; index <= len(key); index++ {
		if index == len(key) || key[index] == '.' {
			parts = append(parts, key[start:index])
			start = index + 1
		}
	}
	return parts
}

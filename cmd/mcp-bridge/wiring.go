package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/search"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
	"github.com/jjohnson83853/mcp-bridge/internal/upstream"
)

// configHolder keeps the configuration the loader staged and the one the
// published generation was admitted from. The two differ only while a reload
// is being admitted, so a candidate the runtime rejects can never change how
// the process behaves (plan §4.10, §4.12).
type configHolder struct {
	mu          sync.Mutex
	staged      *config.Config
	stagedAt    config.Command
	published   *config.Config
	publishedAt config.Command
}

func newConfigHolder() *configHolder { return &configHolder{} }

// stage records the candidate a load just resolved. It does not publish it: the
// loop commits the candidate only when the runtime admits it.
func (h *configHolder) stage(candidate *config.Config, settings config.Command) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.staged, h.stagedAt = candidate, settings
}

// commit publishes the staged candidate. The loop calls it once the runtime has
// admitted the candidate, so the edge keeps serving the previous generation
// until then.
func (h *configHolder) commit() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.staged != nil {
		h.published, h.publishedAt = h.staged, h.stagedAt
	}
}

// settings reports the effective settings of the current generation: the
// published one once a generation exists, and the staged candidate before the
// first admission publishes it.
func (h *configHolder) settings() config.Command {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.published != nil {
		return h.publishedAt
	}
	return h.stagedAt
}

// stagedSettings reports the settings of the candidate being admitted.
func (h *configHolder) stagedSettings() config.Command {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stagedAt
}

// publishedSettings reports the settings of the published generation, or nil
// when no generation has been admitted yet.
func (h *configHolder) publishedSettings() *config.Command {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.published == nil {
		return nil
	}
	settings := h.publishedAt
	return &settings
}

// bounds reports the admission bounds of the current generation, so a reload
// that changes them takes effect without restarting the process (plan §4.5).
func (h *configHolder) bounds() service.InboundBounds {
	settings := h.settings()
	return service.InboundBounds{
		MaxRequestBytes: settings.MaxRequestBytes,
		MaxSessions:     settings.MaxSessions,
		SessionIdle:     settings.SessionIdle,
		AllowedHosts:    settings.AllowedHosts,
		AllowedOrigins:  settings.AllowedOrigins,
	}
}

// restartOnlyField reports the first setting this process binds once and that a
// reload therefore cannot change (plan §4.10: an immutable edit is rejected
// rather than silently half-applied). The listeners, the state directory, the
// refresh interval, the operations limiter, and the embedding and search
// identities are all fixed for the lifetime of the process.
func restartOnlyField(published, candidate config.Command) string {
	switch {
	case candidate.HTTPListen != published.HTTPListen:
		return "http.listen"
	case candidate.OperationsListen != published.OperationsListen:
		return "operations.listen"
	case candidate.StorageDirectory != published.StorageDirectory:
		return "storage.directory"
	case candidate.MaxSnapshotBytes != published.MaxSnapshotBytes:
		return "storage.max_snapshot_bytes"
	case candidate.RefreshInterval != published.RefreshInterval:
		return "catalog.refresh_interval"
	case candidate.OpsRequestsPerSecond != published.OpsRequestsPerSecond:
		return "operations.requests_per_second"
	case candidate.OpsBurst != published.OpsBurst:
		return "operations.burst"
	case candidate.Search != published.Search:
		return "search"
	case candidate.Embedding != published.Embedding:
		return "embedding"
	}
	return ""
}

// upstreamRegistry owns the dedicated upstream clients of the published
// generation, keyed by configured server identity (plan §4.3, §4.10).
type upstreamRegistry struct {
	holder *configHolder

	mu      sync.Mutex
	clients map[string]*upstream.Dedicated
}

func newUpstreamRegistry(holder *configHolder) *upstreamRegistry {
	return &upstreamRegistry{holder: holder, clients: make(map[string]*upstream.Dedicated)}
}

// newClient builds one dedicated runtime from the candidate's credentials and
// effective limits. A replaced runtime is closed by the loop through close.
func (r *upstreamRegistry) newClient(server config.Server) (core.Upstream, error) {
	candidate := r.holder.staged
	if candidate == nil {
		return nil, errors.New("bridge: no configuration is staged")
	}
	settings := r.holder.stagedSettings()
	credentials, err := candidate.ServerCredentials(server.ID)
	if err != nil {
		return nil, err
	}
	limits, err := candidate.ServerLimits(server.ID)
	if err != nil {
		return nil, err
	}
	options := upstream.ServerOptions{
		ID:                  server.ID,
		Endpoint:            server.Endpoint,
		BearerToken:         credentials.Bearer,
		Headers:             credentials.Headers,
		ConnectTimeout:      settings.ConnectTimeout,
		CallTimeout:         settings.CallTimeout,
		MaxConcurrentCalls:  settings.MaxConcurrentCalls,
		BreakerThreshold:    settings.BreakerThreshold,
		BreakerOpenDuration: settings.BreakerOpenDuration,
		Now:                 time.Now,
	}
	if limits.MaxConcurrentCalls != nil {
		options.MaxConcurrentCalls = *limits.MaxConcurrentCalls
	}
	if limits.CallTimeout != nil {
		options.CallTimeout = *limits.CallTimeout
	}
	client, err := upstream.NewDedicated(upstream.DedicatedOptions{Server: options, Stateless: server.ServerStateless()})
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.clients[server.ID] = client
	r.mu.Unlock()
	return client, nil
}

// discover probes one server through its dedicated runtime under the
// configured refresh timeout.
func (r *upstreamRegistry) discover(server config.Server) ([]core.ToolDefinition, error) {
	r.mu.Lock()
	client, ok := r.clients[server.ID]
	r.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("bridge: no dedicated client for server %q", server.ID)
	}
	timeout := r.holder.stagedSettings().RefreshTimeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return client.Discover(ctx)
}

// discoverPrompts publishes only opaque names while retaining the upstream
// prompt name as private provenance for get routing.
func (r *upstreamRegistry) discoverPrompts(server config.Server) ([]storage.Prompt, error) {
	r.mu.Lock()
	client, ok := r.clients[server.ID]
	r.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("bridge: no dedicated client for server %q", server.ID)
	}

	timeout := r.holder.stagedSettings().RefreshTimeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	definitions, err := client.DiscoverPrompts(ctx)
	if err != nil {
		return nil, err
	}
	prompts := make([]storage.Prompt, 0, len(definitions))
	publicNames := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		if definition == nil {
			return nil, errors.New("bridge: upstream returned an invalid prompt definition")
		}
		publicName := core.PromptPublicName(server.ID, definition.Name)
		if _, exists := publicNames[publicName]; exists {
			return nil, fmt.Errorf("bridge: ambiguous prompt publication for server %q", server.ID)
		}
		publicNames[publicName] = struct{}{}
		arguments, err := json.Marshal(definition.Arguments)
		if err != nil {
			return nil, fmt.Errorf("bridge: encode prompt definition for server %q: %w", server.ID, err)
		}
		prompts = append(prompts, storage.Prompt{
			ServerID: server.ID, RawName: definition.Name, PublicName: publicName,
			Title: definition.Title, Description: definition.Description, Arguments: arguments,
		})
	}
	return prompts, nil

}

// close releases one dedicated runtime and drops it when it is still current.
func (r *upstreamRegistry) close(client core.Upstream) error {
	dedicated, ok := client.(*upstream.Dedicated)
	if !ok {
		return nil
	}
	r.mu.Lock()
	if current, held := r.clients[dedicated.ID()]; held && current == dedicated {
		delete(r.clients, dedicated.ID())
	}
	r.mu.Unlock()
	return dedicated.Close()
}

// embedder answers the missing-vector pass of a refresh with the configured
// provider. Every returned row carries the same model identity the tool surface
// serves with, so a vector is either reusable by both or by neither (plan §4.8).
func embedder(provider search.Provider, model search.ModelIdentity, settings config.EmbeddingSettings) func([]search.Document) ([]storage.Embedding, error) {
	return func(documents []search.Document) ([]storage.Embedding, error) {
		if len(documents) == 0 {
			return nil, nil
		}
		inputs := make([]string, len(documents))
		for i, document := range documents {
			inputs[i] = document.RawName + "\n" + document.Description
		}
		ctx, cancel := context.WithTimeout(context.Background(), settings.Timeout)
		defer cancel()
		vectors, err := provider.Embed(ctx, inputs)
		if err != nil {
			return nil, err
		}
		if len(vectors) != len(documents) {
			return nil, fmt.Errorf("embedding provider returned %d vectors for %d documents", len(vectors), len(documents))
		}
		rows := make([]storage.Embedding, 0, len(documents))
		for i, document := range documents {
			rows = append(rows, storage.Embedding{
				ToolIdentity:        storage.ToolIdentity(document.ServerID, document.RawName),
				ContentHash:         search.ContentHash(document.RawName, document.Description),
				ModelIdentityDigest: model.Digest(),
				Dimensions:          settings.Dimensions,
				FormatVersion:       search.FormatVersion,
				Vector:              vectors[i],
			})
		}
		return rows, nil
	}
}

// newEmbeddingProvider resolves the operator's embedding document into the
// provider the refresh path and the tool surface both use. The resolved
// credential never reaches a log, a snapshot, or a tool result.
func newEmbeddingProvider(cfg *config.Config, settings config.EmbeddingSettings) (search.ModelIdentity, search.Provider, error) {
	model := search.ModelIdentity{
		Provider:      settings.Provider,
		BaseURL:       settings.BaseURL,
		Model:         settings.Model,
		Revision:      settings.ModelRevision,
		Dimensions:    settings.Dimensions,
		FormatVersion: search.FormatVersion,
	}
	credential, err := cfg.EmbeddingCredential()
	if err != nil {
		return search.ModelIdentity{}, nil, err
	}
	provider, err := search.NewProvider(search.ProviderOptions{
		Provider:         settings.Provider,
		BaseURL:          settings.BaseURL,
		Model:            settings.Model,
		APIKey:           credential,
		Dimensions:       settings.Dimensions,
		Timeout:          settings.Timeout,
		MaxConcurrency:   settings.MaxConcurrency,
		BatchSize:        settings.BatchSize,
		MaxResponseBytes: settings.MaxResponseBytes,
	})
	if err != nil {
		return search.ModelIdentity{}, nil, err
	}
	return model, provider, nil
}

// searchSettings maps the operator's search document onto the search layer's
// configuration; fields the document does not express keep that layer's own
// defaults.
func searchSettings(settings config.SearchSettings) search.Config {
	return search.Config{
		TopKDefault:   settings.DefaultTopK,
		TopKMax:       settings.MaxTopK,
		KeywordWeight: settings.KeywordWeight,
		VectorWeight:  settings.EmbeddingWeight,
		RRFConstant:   float64(settings.RRFConstant),
		CacheEntries:  settings.QueryCacheEntries,
		CacheTTL:      settings.QueryCacheTTL,
		EmbedTimeout:  settings.Timeout,
	}
}

// localChecks tracks the process-local readiness conditions the operations
// endpoint reports (plan §4.12): the store's exclusive writer lock, the storage
// write path, and whether a generation has been published.
type localChecks struct {
	mu              sync.Mutex
	state           *service.ReadinessState
	writerLock      bool
	storageWritable bool
	routerReady     bool
}

func newLocalChecks(state *service.ReadinessState, writerLock bool) *localChecks {
	return &localChecks{state: state, writerLock: writerLock, storageWritable: true}
}

func (c *localChecks) storage(writable bool) { c.update(func() { c.storageWritable = writable }) }

func (c *localChecks) router(ready bool) { c.update(func() { c.routerReady = ready }) }

func (c *localChecks) update(change func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	change()
	c.state.SetLocal(true, c.writerLock, c.storageWritable, c.routerReady)
}

// publishObserver reports one published generation to the readiness state and
// the metric surface, and commits the configuration it was admitted from.
func publishObserver(holder *configHolder, checks *localChecks, state *service.ReadinessState, metrics *observe.Metrics) func(*service.Assembly) {
	return func(assembly *service.Assembly) {
		if assembly == nil {
			return
		}
		holder.commit()
		snapshot := assembly.Snapshot()
		tools := make(map[string]int, len(assembly.ServerIDs()))
		if snapshot != nil {
			for _, row := range snapshot.Tools {
				tools[row.ServerID]++
			}
		}
		now := time.Now()
		for _, serverID := range assembly.ServerIDs() {
			health, ok := assembly.Health(serverID)
			if !ok {
				continue
			}
			complete := health.Status == storage.StatusFresh
			connected := health.Availability == service.AvailabilityAvailable
			state.SetServer(serverID, connected, complete, health.Reason)
			metrics.SetCatalogTools(serverID, tools[serverID])
			if complete && !health.LastSuccessAt.IsZero() {
				metrics.SetCatalogAge(serverID, now.Sub(health.LastSuccessAt).Seconds())
			}
			level := observe.ReadinessUnavailable
			switch {
			case connected && complete:
				level = observe.ReadinessReady
			case complete:
				level = observe.ReadinessDegraded
			}
			metrics.SetReadiness(serverID, level)
		}
		checks.router(true)
	}
}

// operatorLog renders one operator-facing loop line. The log surface is typed
// and finite (plan §4.11), so a rejected candidate is reported by its class: a
// configuration error can quote the document it rejected.
func operatorLog(logger *observe.Logger) func(string, ...any) {
	return func(_ string, args ...any) {
		var diagnostics *config.Diagnostics
		publicationFailed := false
		for _, arg := range args {
			if err, ok := arg.(error); ok {
				if errors.As(err, &diagnostics) {
					break
				}
				if errors.Is(err, service.ErrPublicationFailed) {
					publicationFailed = true
				}
			}
		}
		if diagnostics != nil {
			for i, record := range diagnostics.Records {
				logger.Log(observe.Entry{
					Level:   "warn",
					Event:   "reload",
					Outcome: "failure",
					Reason:  "invalid_config",
					Server:  recordOwnerLabel(record.Server),
					Fields:  diagnosticFields(record, i, diagnostics),
				})
			}
			if diagnostics.Incomplete {
				logger.Log(observe.Entry{
					Level:   "warn",
					Event:   "reload",
					Outcome: "failure",
					Reason:  "invalid_config",
					Server:  bridgeServerLabel,
					Fields: map[string]any{
						"diagnostics_incomplete": true,
					},
				})
			}
			return
		}
		entry := observe.Entry{
			Level:   "warn",
			Event:   "reload",
			Outcome: "failure",
			Reason:  "invalid_config",
			Server:  bridgeServerLabel,
		}
		if publicationFailed {
			entry.Reason = "internal_error"
		}
		logger.Log(entry)
	}
}

// diagnosticFields projects one typed diagnostics record onto the operator
// logger's exact 18-field manifest (spec HMB-CD-v1 §4.5 item 4). It never
// carries a secret value, a preview, a hash, a fingerprint, an exact length,
// an offset, or any raw YAML/URL/OS text: every key mirrors the package's
// metadata-only contract, and no unclassified error argument is ever logged.
func diagnosticFields(record config.Diagnostic, index int, diagnostics *config.Diagnostics) map[string]any {
	fields := make(map[string]any, 18)
	fields["config_path"] = record.Path
	fields["location"] = record.Key
	if record.Line != 0 {
		fields["line"] = record.Line
		fields["column"] = record.Column
	}
	if record.PinServer != "" {
		fields["pin_server"] = record.PinServer
	}
	if record.PinTool != "" {
		fields["pin_tool"] = record.PinTool
	}
	if record.RefKind != "" {
		fields["ref_kind"] = record.RefKind
	}
	if record.RefName != "" {
		fields["ref_name"] = record.RefName
	}
	if record.RelatedKey != "" {
		fields["related_location"] = record.RelatedKey
	}
	if record.RelatedRefKind != "" {
		fields["related_ref_kind"] = record.RelatedRefKind
	}
	if record.RelatedRefName != "" {
		fields["related_ref_name"] = record.RelatedRefName
	}
	fields["code"] = record.Code
	fields["property"] = record.Property
	fields["shape"] = record.Shape
	fields["diagnostic_index"] = index
	if diagnostics != nil && diagnostics.Incomplete {
		fields["diagnostics_incomplete"] = true
	}
	fields["remaining_stages"] = diagnostics.RemainingStages
	return fields
}

// bridgeServerLabel is the aggregate label of work the bridge does for itself
// rather than for one configured server.
const bridgeServerLabel = "_bridge"

// recordOwnerLabel maps a diagnostic record's owning server id onto the
// operator log's top-level `server` field (spec §4.5 item 4): the record's
// validated owner when it passes the existing safe-server rules, and the
// bridge's own aggregate label otherwise. Ownerless records keep _bridge.
func recordOwnerLabel(server string) string {
	if server == "" || !isSafeServerID(server) {
		return bridgeServerLabel
	}
	return server
}

// isSafeServerID mirrors the observe logger's safe-server rule: the finite
// vocabulary of configured server ids (lowercase lead, lowercase/digits/dashes).
func isSafeServerID(value string) bool {
	if len(value) == 0 || len(value) > 32 {
		return false
	}
	if value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, r := range value[1:] {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

// pumpReloads turns each SIGHUP into one reload request. A burst coalesces into
// one pending request, because the loop already collapses a storm into one
// active pass plus one pending pass (plan §4.7).
func pumpReloads(ctx context.Context, hup <-chan os.Signal, reloads chan<- struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-hup:
			select {
			case reloads <- struct{}{}:
			default:
			}
		}
	}
}

// durableLoader supplies the snapshot of the previous run. A store that holds
// no snapshot is a cold start, not a refusal, so startup proceeds with no
// cached state instead of reporting a rejected snapshot.
func durableLoader(store *storage.Store) func() (*storage.Snapshot, error) {
	return func() (*storage.Snapshot, error) {
		snapshot, err := store.Load()
		if errors.Is(err, storage.ErrNoSnapshot) {
			return nil, nil
		}
		return snapshot, err
	}
}

// promptLoader reads the durable prompt snapshot. A missing file is not an
// error: a bridge that has never persisted a prompt simply has none.
func promptLoader(store *storage.Store) func() (*storage.PromptSnapshot, error) {
	return func() (*storage.PromptSnapshot, error) {
		snapshot, err := store.LoadPrompts()
		if errors.Is(err, storage.ErrNoSnapshot) {
			return nil, nil
		}
		return snapshot, err
	}
}

// configuredServerIDs lists the server identities a document configures, so
// every admitted server stays observable from the first pass.
func configuredServerIDs(cfg *config.Config) []string {
	ids := make([]string, 0, len(cfg.Servers))
	for _, server := range cfg.Servers {
		ids = append(ids, server.ID)
	}
	return ids
}

// orNone keeps the metric labels inside their finite vocabulary: a success has
// no failure reason.
func orNone(reason string) string {
	if strings.TrimSpace(reason) == "" {
		return "none"
	}
	return reason
}

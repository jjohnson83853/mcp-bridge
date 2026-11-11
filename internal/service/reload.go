package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/search"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

// ReloadDeps are the injected collaborators of one membership reload. The
// command layer supplies the real discovery, client, and persistence
// implementations; tests supply fakes.
type ReloadDeps struct {
	// Discover reports the exact tool definitions an upstream currently
	// exposes. A failure rejects the whole candidate reload.
	Discover        func(server config.Server) ([]core.ToolDefinition, error)
	DiscoverPrompts func(server config.Server) ([]storage.Prompt, error)
	LoadPrompts     func() (*storage.PromptSnapshot, error)
	PersistPrompts  func(snapshot *storage.PromptSnapshot) error
	// NewClient builds one dedicated upstream client for a server whose
	// identity or credentials are new in this candidate.
	NewClient func(server config.Server) (core.Upstream, error)
	// Persist stores the reconciled candidate snapshot. It runs before the
	// candidate is published, so a failure publishes nothing.
	Persist func(snapshot *storage.Snapshot) error
	// Embed computes the vectors of the tools that have no current one. It is
	// the only dependency that contacts the embedding provider; a nil Embed
	// keeps the reused vectors and leaves the rest absent (plan §4.8/§4.9).
	Embed func(documents []search.Document) ([]storage.Embedding, error)
	// Close releases one dedicated client after its generation and leases
	// have drained.
	Close func(client core.Upstream) error
	// Now stamps the reconciled snapshot.
	Now func() time.Time
	// ModelIdentityDigest is the canonical 64-hex embedding model identity of
	// this run; a change invalidates stored vectors without touching exact
	// definitions.
	ModelIdentityDigest string
}

// ServerRuntime is one server of a published generation: its identity and the
// exact dedicated client every call on that generation must use.
type ServerRuntime struct {
	ID          string
	Endpoint    string
	Credentials string
	Client      core.Upstream

	// available reports whether this generation's own probe of the server
	// succeeded. It is never derived from holding a client, so a cold
	// admission stays unavailable until a complete accepted discovery
	// advances that server (plan §4.5).
	available bool

	tracker *leaseTracker
}

// Assembly is one immutable published generation of the bridge runtime: the
// effective flat membership, the aggregate catalog, the inbound token
// fingerprints, and the reconciled snapshot. It is never mutated after
// publication; a reload builds a successor and swaps one pointer.
type Assembly struct {
	generation  uint64
	fingerprint string
	revision    string
	auth        [][32]byte
	catalog     *core.Bridge
	servers     map[string]ServerRuntime
	pins        []core.ResolvedPin
	snapshot    *storage.Snapshot
	prompts     []storage.Prompt
	// pinMetadata is the per-pin effective metadata view this generation
	// derived from the server defaults, keyed by serverID + "\x00" + rawName.
	// It is immutable after publication and nil when no defaults are configured.
	pinMetadata map[string]pinMetadataEntry
	// toolMetadata is the immutable accepted defaults/exact overlay per server.
	// Dynamic projections apply these patches only after a target is interested.
	toolMetadata map[string]serverToolMetadata
}

// pinMetadataEntry is one pinned tool's effective metadata view and the
// provenance of every winning override field.
type pinMetadataEntry struct {
	Effective      core.ToolMetadata
	OverrideFields []MetadataOverrideField
}

// PinMetadataView returns one pinned tool's effective metadata and the sorted
// winning override paths, or false when the generation derived no effective
// view for that pin (no configured defaults, or a startup generation whose
// pins are still deferred). The returned metadata is the generation's own
// immutable view and must never be mutated by callers.
func (a *Assembly) PinMetadataView(serverID, rawName string) (core.ToolMetadata, []MetadataOverrideField, bool) {
	if a == nil || len(a.pinMetadata) == 0 {
		return core.ToolMetadata{}, nil, false
	}
	entry, ok := a.pinMetadata[serverID+"\x00"+rawName]
	if !ok {
		return core.ToolMetadata{}, nil, false
	}
	return entry.Effective, entry.OverrideFields, true
}

// Generation is the monotonic publication counter of this assembly.
func (a *Assembly) Generation() uint64 { return a.generation }

// ConfigFingerprint is the public configuration identity this generation runs
// (config §4.2).
func (a *Assembly) ConfigFingerprint() string { return a.fingerprint }

// CatalogRevision is the global catalog revision of this generation.
func (a *Assembly) CatalogRevision() string { return a.revision }

// Catalog is the immutable aggregate catalog of this generation.
func (a *Assembly) Catalog() *core.Bridge { return a.catalog }

// ClientTokenHashes are the inbound token fingerprints accepted by this
// generation. A reload never mutates them in place.
func (a *Assembly) ClientTokenHashes() [][32]byte {
	return append([][32]byte(nil), a.auth...)
}

// Snapshot is the reconciled durable snapshot this generation published.
func (a *Assembly) Snapshot() *storage.Snapshot { return a.snapshot }
func (a *Assembly) Prompts() []storage.Prompt   { return append([]storage.Prompt(nil), a.prompts...) }

// ServerIDs lists the configured servers of this generation in stable order.
func (a *Assembly) ServerIDs() []string {
	ids := make([]string, 0, len(a.servers))
	for id := range a.servers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Server resolves one configured server of this generation.
func (a *Assembly) Server(serverID string) (ServerRuntime, bool) {
	server, ok := a.servers[serverID]
	return server, ok
}

// ServerHealth is one published server's catalog status and runtime
// availability (plan §4.5): the status comes from the durable snapshot, the
// availability from whether this generation holds a live client.
type ServerHealth struct {
	Status        string
	Availability  string
	Reason        string
	LastSuccessAt time.Time
}

// Availability values of one published server (plan §4.5). "probing" is never
// published by a generation: a generation either holds a client or it does not.
const (
	AvailabilityAvailable   = "available"
	AvailabilityUnavailable = "unavailable"
)

// Health reports one configured server's status and availability. An unknown
// server reports false.
func (a *Assembly) Health(serverID string) (ServerHealth, bool) {
	server, ok := a.servers[serverID]
	if !ok {
		return ServerHealth{}, false
	}
	health := ServerHealth{Availability: AvailabilityUnavailable}
	if server.available {
		health.Availability = AvailabilityAvailable
	}
	if a.snapshot != nil {
		for _, row := range a.snapshot.Servers {
			if row.ServerID == serverID {
				health.Status = row.CatalogStatus
				health.Reason = row.Reason
				health.LastSuccessAt = row.LastSuccessAt
				break
			}
		}
	}
	return health, true
}

// Pins are the resolved pinned prerequisite tools of this generation, in
// configured order.
func (a *Assembly) Pins() []core.ResolvedPin {
	return append([]core.ResolvedPin(nil), a.pins...)
}

// PinnedNames reports the public names of this generation's resolved pins.
func (a *Assembly) PinnedNames() map[string]bool {
	names := make(map[string]bool, len(a.pins))
	for _, pin := range a.pins {
		if pin.Binding == nil {
			continue
		}
		names[core.PublicName(pin.PinnedTool.ServerID, pin.PinnedTool.RawName)] = true
	}
	return names
}

// Acquire pins this generation for one call to serverID. It fails once the
// generation has been retired, so a caller that raced a reload re-reads the
// pointer instead of reaching a closed client. It also fails while this
// generation's own probe of the server has not succeeded, so an outage can
// never hand out a lease for dispatch (fail-closed, plan §4.5).
func (a *Assembly) Acquire(serverID string) (*Lease, bool) {
	server, ok := a.servers[serverID]
	if !ok {
		return nil, false
	}
	if !server.available {
		return nil, false
	}
	if !server.tracker.acquire() {
		return nil, false
	}
	return &Lease{server: server, tracker: server.tracker}, true
}

// Runtime is the single publication point of the bridge runtime. Every request
// reads one Assembly through it; a reload replaces that pointer once.
type Runtime struct {
	// commitMu serializes durable candidate writes with runtime acceptance.
	// Serving and projection admission use the separate coordinator barrier.
	commitMu        sync.Mutex
	current         atomic.Pointer[Assembly]
	coordinatorOnce sync.Once
	coordinator     *projectionCoordinator
}

// NewRuntime returns an empty runtime with no published generation.
func NewRuntime() *Runtime { return &Runtime{} }

// Current returns the published generation, or nil before the first reload.
func (r *Runtime) Current() *Assembly { return r.current.Load() }

// Acquire pins the published generation for one call to serverID.
func (r *Runtime) Acquire(serverID string) (*Lease, bool) {
	c := r.progressiveCoordinator()
	c.barrier.Lock()
	defer c.barrier.Unlock()
	assembly := r.current.Load()
	if assembly == nil || (c.acceptedKnown && c.acceptedGeneration != assembly.Generation()) {
		return nil, false
	}
	return assembly.Acquire(serverID)
}

// Lease is one in-flight call pinned to the exact server, endpoint, and
// dedicated client of the generation it was admitted on.
type Lease struct {
	server  ServerRuntime
	tracker *leaseTracker
	once    sync.Once
}

// Server returns the pinned server runtime the call must use.
func (l *Lease) Server() ServerRuntime { return l.server }

// Release ends the lease. It is idempotent; the last release of a retired
// generation triggers that generation's client close.
func (l *Lease) Release() { l.once.Do(func() { l.tracker.release() }) }

// leaseTracker drains one server's generation: new leases are refused once it
// is retired, and the client closes exactly once, after every lease admitted
// before retirement has been released.
type leaseTracker struct {
	mu      sync.Mutex
	leases  int
	retired bool
	closed  bool
	onDrain func()
}

func (t *leaseTracker) acquire() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.retired {
		return false
	}
	t.leases++
	return true
}

func (t *leaseTracker) release() {
	t.mu.Lock()
	t.leases--
	drain := t.retired && t.leases == 0 && !t.closed
	var onDrain func()
	if drain {
		t.closed = true
		onDrain = t.onDrain
		t.onDrain = nil
	}
	t.mu.Unlock()
	if onDrain != nil {
		go onDrain()
	}
}

// retire drains the generation: it refuses new leases and closes the client
// once the leases admitted before retirement have been released. A generation
// with no outstanding lease closes immediately.
func (t *leaseTracker) retire(onDrain func()) {
	t.mu.Lock()
	t.retired = true
	immediate := t.leases == 0 && !t.closed
	if immediate {
		t.closed = true
		t.onDrain = nil
	} else {
		t.onDrain = onDrain
	}
	t.mu.Unlock()
	if immediate {
		go onDrain()
	}
}

// buildMode distinguishes the two candidate admissions: a reload validates the
// whole membership and rejects the candidate on any failure, a refresh keeps
// every server's last known good catalog when its own discovery fails.
type buildMode int

const (
	buildReload buildMode = iota
	buildRefresh
	buildStartup
)

// String names the mode for operator-facing error text (plan §4.7/§4.10).
func (m buildMode) String() string {
	switch m {
	case buildRefresh:
		return "refresh"
	case buildStartup:
		return "startup"
	default:
		return "reload"
	}
}

// buildInput is one candidate build of a published generation.
type buildInput struct {
	mode      buildMode
	previous  *Assembly
	candidate *config.Config
	deps      ReloadDeps
	// reason classifies one server's discovery failure into the finite status
	// vocabulary. Refresh only; a reload rejects the candidate instead.
	reason func(server config.Server, err error) string
	// durable is the validated snapshot of the previous run. Startup admits
	// each server from it without contacting any upstream (plan §4.5).
	durable *storage.Snapshot
}

// serverOutcome records how one configured server resolved in a build.
type serverOutcome struct {
	id string
	// failed marks a server whose own discovery failed.
	failed bool
	reason string
	// notProbed marks a cold-admission server this process has not contacted
	// yet; restored says whether a complete catalog came back from disk.
	notProbed bool
	restored  bool
}

type pinBlocker struct {
	server        string
	tool          string
	propertyPath  string
	blockerClass  string
	failureReason string
}

type pinResolutionError struct {
	blockers        []pinBlocker
	discoveredTools map[string][]string
}

func (e *pinResolutionError) Error() string { return "unresolved mandatory pins" }

func resolveCandidatePins(candidate *config.Config, catalog *core.Bridge, definitions map[string][]core.ToolDefinition, outcomes []serverOutcome) ([]core.ResolvedPin, *pinResolutionError) {
	discoveryFailures := make(map[string]string, len(outcomes))
	for _, outcome := range outcomes {
		if outcome.failed {
			discoveryFailures[outcome.id] = safePinDiscoveryCause(outcome.reason)
		}
	}

	resolved := make([]core.ResolvedPin, 0, len(candidate.Pins))
	blockers := make([]pinBlocker, 0)
	serversWithMissingPins := make(map[string]struct{})
	for index, configured := range candidate.Pins {
		serverID, toolName := configured["server"], configured["tool"]
		binding, ok := catalog.Binding(core.PublicName(serverID, toolName))
		if !ok {
			blocker := pinBlocker{
				server:       serverID,
				tool:         toolName,
				propertyPath: fmt.Sprintf("pins[%d]", index),
				blockerClass: "tool_absent",
			}
			if reason, failed := discoveryFailures[serverID]; failed {
				blocker.blockerClass = "server_unverified"
				blocker.failureReason = reason
			} else {
				serversWithMissingPins[serverID] = struct{}{}
			}
			blockers = append(blockers, blocker)
			continue
		}
		bindingCopy := binding
		resolved = append(resolved, core.ResolvedPin{
			PinnedTool: core.PinnedTool{ServerID: serverID, RawName: toolName},
			Binding:    &bindingCopy,
		})
	}
	if len(blockers) == 0 {
		return resolved, nil
	}

	discoveredTools := make(map[string][]string, len(serversWithMissingPins))
	for serverID := range serversWithMissingPins {
		names := make([]string, 0, len(definitions[serverID]))
		for _, definition := range definitions[serverID] {
			if validPinToolName(definition.RawName) {
				names = append(names, definition.RawName)
			}
		}
		sort.Strings(names)
		discoveredTools[serverID] = names
	}
	return nil, &pinResolutionError{blockers: blockers, discoveredTools: discoveredTools}
}

func safePinDiscoveryCause(reason string) string {
	switch reason {
	case RefreshReasonUnavailable, RefreshReasonDiscoveryFailed, RefreshReasonClientMissing:
		return reason
	default:
		return RefreshReasonDiscoveryFailed
	}
}

func validPinToolName(name string) bool {
	if name == "" {
		return false
	}
	for index := range len(name) {
		char := name[index]
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' || char == '_' || char == '-') {
			return false
		}
	}
	return true
}

// Reload validates one candidate membership and, on success, publishes exactly
// one new generation (plan §4.10 steps 1-6). On any failure it returns an error
// and leaves the published generation, its catalog, its inbound token
// fingerprints, and its working clients exactly as they were; clients it
// created for the rejected candidate are closed.
func (r *Runtime) Reload(candidate *config.Config, deps ReloadDeps) (*Assembly, error) {
	if candidate == nil {
		return nil, errors.New("service: reload needs a candidate configuration")
	}
	if deps.Discover == nil || deps.NewClient == nil || deps.Now == nil {
		return nil, errors.New("service: reload needs discovery, client, and clock dependencies")
	}
	return r.build(buildInput{
		mode:      buildReload,
		previous:  r.current.Load(),
		candidate: candidate,
		deps:      deps,
	})
}

// AdmitStartup publishes the local generation this process starts from (plan
// §4.5/§4.7): every configured server gets its own dedicated client built from
// the CURRENT credentials and effective limits, its catalog is reconstructed
// from the durable snapshot, and no upstream is contacted. Availability is
// unavailable and a restored catalog is stale until an independent, bounded
// discovery probe advances that server, so no all-server startup barrier
// exists. A nil or unreadable durable snapshot simply admits every server with
// no catalog.
func (r *Runtime) AdmitStartup(candidate *config.Config, deps ReloadDeps, durable *storage.Snapshot) (*Assembly, error) {
	if candidate == nil {
		return nil, errors.New("service: startup needs a candidate configuration")
	}
	if deps.NewClient == nil || deps.Now == nil {
		return nil, errors.New("service: startup needs client and clock dependencies")
	}
	return r.build(buildInput{
		mode:      buildStartup,
		previous:  r.current.Load(),
		candidate: candidate,
		deps:      deps,
		durable:   durable,
	})
}

// build assembles and, unless any step fails, publishes the successor of the
// current generation. Nothing is published on failure (plan §4.10 step 4).
func (r *Runtime) build(in buildInput) (*Assembly, error) {
	previous := in.previous
	at := in.deps.Now().UTC()
	next := &Assembly{
		generation:   1,
		fingerprint:  in.candidate.Fingerprint(),
		auth:         in.candidate.ClientTokenHashes(),
		catalog:      core.NewBridge(),
		servers:      make(map[string]ServerRuntime, len(in.candidate.Servers)),
		toolMetadata: cloneServerToolMetadata(in.candidate),
	}
	if previous != nil {
		next.generation = previous.generation + 1
	}
	if previous != nil {
		next.prompts = previous.Prompts()
	}
	if in.mode == buildStartup && in.deps.LoadPrompts != nil {
		if loaded, err := in.deps.LoadPrompts(); err == nil && loaded != nil {
			next.prompts = append([]storage.Prompt(nil), loaded.Prompts...)
		}
	}
	next.prompts = promptsForServers(next.prompts, in.candidate.Servers)

	var created []core.Upstream
	published := false
	defer func() {
		if published || in.deps.Close == nil {
			return
		}
		for _, client := range created {
			_ = in.deps.Close(client)
		}
	}()

	configured := make([]storage.ConfiguredServer, 0, len(in.candidate.Servers))
	outcomes := make([]serverOutcome, 0, len(in.candidate.Servers))
	definitions := make(map[string][]core.ToolDefinition, len(in.candidate.Servers))
	for _, server := range in.candidate.Servers {
		credentials, ok := in.candidate.ServerCredentialFingerprint(server.ID)
		if !ok {
			return nil, fmt.Errorf("service: %s cannot identify credentials of server %q", in.mode, server.ID)
		}
		runtime := ServerRuntime{
			ID:          server.ID,
			Endpoint:    server.Endpoint,
			Credentials: credentials,
			tracker:     &leaseTracker{},
		}
		if held, ok := previousServer(previous, server.ID); ok && in.mode != buildStartup &&
			held.Endpoint == server.Endpoint && held.Credentials == credentials {
			runtime.Client = held.Client
		} else {
			if in.mode == buildRefresh {
				// A refresh never creates or rotates a client: membership and
				// credential changes belong to an atomic reload.
				return nil, fmt.Errorf("service: refresh cannot adopt a new client for server %q", server.ID)
			}
			// A reload and a cold startup both build a dedicated client from
			// the CURRENT credentials and effective limits; startup never
			// adopts a live client from a previous process generation.
			client, err := in.deps.NewClient(server)
			if err != nil {
				return nil, fmt.Errorf("service: %s client for server %q: %w", in.mode, server.ID, err)
			}
			runtime.Client = client
			created = append(created, client)
		}
		if server.PromptsDisabled {
			next.prompts = replaceServerPrompts(next.prompts, server.ID, nil)
		} else if in.mode != buildStartup && in.deps.DiscoverPrompts != nil {
			if discovered, err := in.deps.DiscoverPrompts(server); err == nil {
				next.prompts = replaceServerPrompts(next.prompts, server.ID, discovered)
			}
		}
		var defs []core.ToolDefinition
		if in.mode == buildStartup {
			// No upstream is contacted: the local generation serves whatever
			// complete catalog the previous run stored, and a server without
			// one stays empty until a probe succeeds (plan §4.5).
			defs = definitionsFromSnapshot(in.durable, server.ID, server.Endpoint)
			outcomes = append(outcomes, serverOutcome{
				id:        server.ID,
				notProbed: true,
				restored:  len(defs) > 0,
			})
		} else {
			discovered, err := in.deps.Discover(server)
			switch {
			case err == nil:
				defs = discovered
				runtime.available = true
				outcomes = append(outcomes, serverOutcome{id: server.ID})
			case in.mode == buildReload:
				return nil, fmt.Errorf("service: reload discovery for server %q: %w", server.ID, err)
			default:
				// A refresh isolates the outage: the server keeps its complete
				// last known good catalog, or stays empty when this generation
				// never had one, and the pass still advances the others.
				defs = serverDefinitions(previous, server.ID)
				reason := RefreshReasonDiscoveryFailed
				if in.reason != nil {
					reason = in.reason(server, err)
				}
				outcomes = append(outcomes, serverOutcome{id: server.ID, failed: true, reason: reason})
			}
		}
		if err := next.catalog.Register(server.ID, server.Endpoint, defs); err != nil {
			return nil, fmt.Errorf("service: %s catalog for server %q: %w", in.mode, server.ID, err)
		}
		definitions[server.ID] = defs
		next.servers[server.ID] = runtime
		configured = append(configured, storage.ConfiguredServer{
			ServerID:            server.ID,
			EndpointFingerprint: endpointFingerprint(server.Endpoint),
		})
	}
	sort.Slice(next.prompts, func(i, j int) bool {
		if next.prompts[i].ServerID != next.prompts[j].ServerID {
			return next.prompts[i].ServerID < next.prompts[j].ServerID
		}
		return next.prompts[i].RawName < next.prompts[j].RawName
	})
	next.revision = next.catalog.Revision()

	if in.mode == buildStartup {
		// Startup contacts no upstream, so its catalog is only the durable
		// snapshot; a fresh generation defers pin resolution until its first
		// live refresh.
		next.pins = nil
	} else {
		resolvedPins, pinErr := resolveCandidatePins(in.candidate, next.catalog, definitions, outcomes)
		if pinErr != nil {
			return nil, fmt.Errorf("service: %s pins: %w", in.mode, pinErr)
		}
		resolvedPins, pinMetadata, err := applyServerMetadataDefaults(in.candidate, resolvedPins, definitions, in.mode)
		if err != nil {
			return nil, err
		}
		next.pins = resolvedPins
		next.pinMetadata = pinMetadata
	}

	snapshot, err := storage.Reconcile(in.reconcileBase(previous), storage.ReconcileInput{
		CreatedAt:               at,
		ConfigPublicFingerprint: next.fingerprint,
		CatalogRevision:         next.revision,
		ModelIdentityDigest:     in.deps.ModelIdentityDigest,
		Servers:                 configured,
	})
	if err != nil {
		return nil, fmt.Errorf("service: %s reconcile: %w", in.mode, err)
	}
	for _, outcome := range outcomes {
		switch {
		case outcome.failed:
			snapshot, err = storage.StampServerFailure(snapshot, outcome.id, outcome.reason, at)
		case outcome.notProbed && outcome.restored:
			snapshot, err = storage.StampServerFailure(snapshot, outcome.id, RefreshReasonNotProbed, at)
		case outcome.notProbed:
			snapshot, err = storage.StampServerEmpty(snapshot, outcome.id, at)
		default:
			snapshot, err = storage.StampServerSuccess(snapshot, outcome.id, at)
		}
		if err != nil {
			return nil, fmt.Errorf("service: %s stamp server %q: %w", in.mode, outcome.id, err)
		}
	}
	// Index this cycle's exact catalog into the durable rows and complete the
	// vectors it can reuse or recompute (plan §4.8/§4.9, AC-13): the snapshot
	// is where the published catalog and its vectors survive a restart, so a
	// candidate whose rows cannot be admitted is rejected instead of being
	// persisted partially.
	for _, server := range in.candidate.Servers {
		snapshot, err = storage.IndexTools(snapshot, server.ID, indexedToolRows(server.ID, definitions[server.ID]))
		if err != nil {
			return nil, fmt.Errorf("service: %s index server %q: %w", in.mode, server.ID, err)
		}
	}
	if in.mode != buildStartup {
		// A cold local admission contacts no upstream, so it never asks the
		// embedding provider either; the first refresh completes the vectors.
		snapshot = in.fillVectors(snapshot)
	}
	// Keep slow persistence outside the serving barrier, but serialize it with
	// admission so a candidate that lost the expected-generation race is never
	// persisted.
	r.commitMu.Lock()
	defer r.commitMu.Unlock()
	if err := r.validateAcceptance(previous); err != nil {
		return nil, fmt.Errorf("service: %s accept runtime: %w", in.mode, err)
	}
	if in.deps.Persist != nil {
		if err := in.deps.Persist(snapshot); err != nil {
			return nil, fmt.Errorf("service: %s persist: %w", in.mode, err)
		}
	}
	if in.mode != buildStartup && in.deps.PersistPrompts != nil {
		candidatePrompts := &storage.PromptSnapshot{SchemaVersion: storage.PromptSchemaVersion, Prompts: append([]storage.Prompt(nil), next.prompts...)}
		if err := in.deps.PersistPrompts(candidatePrompts); err != nil && previous != nil {
			next.prompts = previous.Prompts()
		}
	}
	next.snapshot = snapshot

	if err := r.accept(next, previous); err != nil {
		return nil, fmt.Errorf("service: %s accept runtime: %w", in.mode, err)
	}
	published = true
	retirePrevious(previous, next, in.deps)
	return next, nil
}

// reconcileBase is the durable snapshot a build reconciles its membership
// against: the in-memory snapshot of the generation it supersedes, or - for a
// startup admission, which supersedes no live generation - the validated
// snapshot of the previous run, so that run's exact catalog, vectors, and
// tombstones survive the restart instead of being dropped (plan §4.5/§4.9,
// AC-13/AC-15/AC-16).
func promptsForServers(prompts []storage.Prompt, servers []config.Server) []storage.Prompt {
	allowed := make(map[string]bool, len(servers))
	for _, server := range servers {
		if !server.PromptsDisabled {
			allowed[server.ID] = true
		}
	}
	filtered := make([]storage.Prompt, 0, len(prompts))
	for _, prompt := range prompts {
		if allowed[prompt.ServerID] {
			filtered = append(filtered, prompt)
		}
	}
	return filtered
}

func replaceServerPrompts(current []storage.Prompt, serverID string, replacement []storage.Prompt) []storage.Prompt {
	updated := make([]storage.Prompt, 0, len(current)+len(replacement))
	for _, prompt := range current {
		if prompt.ServerID != serverID {
			updated = append(updated, prompt)
		}
	}
	for _, prompt := range replacement {
		if prompt.ServerID == serverID {
			updated = append(updated, prompt)
		}
	}
	return updated
}
func (in buildInput) reconcileBase(previous *Assembly) *storage.Snapshot {
	if in.mode == buildStartup && in.durable != nil {
		return in.durable
	}
	return previousSnapshot(previous)
}

// definitionsFromSnapshot reconstructs one server's exact tool definitions from
// the durable snapshot of the previous run, and only while the server's
// endpoint still matches the durable binding: a changed endpoint must not
// serve the cache of the endpoint it left behind, so it admits empty until a
// probe rebuilds its catalog. The four fields a definition is hashed from
// survive the round trip byte-exactly in canonical form, so a restored catalog
// keeps its public names and revision (plan §4.9, AC-13).
func definitionsFromSnapshot(snapshot *storage.Snapshot, serverID string, endpoint string) []core.ToolDefinition {
	if snapshot == nil {
		return nil
	}
	expected := endpointFingerprint(endpoint)
	var binding *storage.Binding
	for index := range snapshot.Bindings {
		if snapshot.Bindings[index].ServerID == serverID {
			binding = &snapshot.Bindings[index]
			break
		}
	}
	if binding == nil || binding.Retired || binding.EndpointFingerprint != expected {
		return nil
	}
	var defs []core.ToolDefinition
	for _, row := range snapshot.Tools {
		if row.ServerID != serverID {
			continue
		}
		def := core.ToolDefinition{
			RawName:      row.RawName,
			Description:  row.Description,
			InputSchema:  row.InputSchema,
			OutputSchema: row.OutputSchema,
		}
		if row.Metadata != nil {
			def.Metadata = row.Metadata.Clone()
		}
		defs = append(defs, def)
	}
	return defs
}

// serverDefinitions returns one server's exact tool definitions from a
// published generation, or nothing when that generation never held the server.
func serverDefinitions(previous *Assembly, serverID string) []core.ToolDefinition {
	if previous == nil || previous.catalog == nil {
		return nil
	}
	var defs []core.ToolDefinition
	for _, binding := range previous.catalog.Bindings() {
		if binding.ServerID == serverID {
			defs = append(defs, binding.Def)
		}
	}
	return defs
}

// Drain retires the published generation, so every dedicated client closes once
// its in-flight leases release. It is the process shutdown path: the runtime
// publishes nothing afterwards.
func (r *Runtime) Drain(deps ReloadDeps) {
	assembly := r.current.Load()
	if assembly == nil {
		return
	}
	retirePrevious(assembly, &Assembly{servers: map[string]ServerRuntime{}}, deps)
}

// retirePrevious drains the generations a publication superseded: every server
// the new generation does not carry forward is retired, and its client closes
// after the last lease admitted before the swap is released.
func retirePrevious(previous, next *Assembly, deps ReloadDeps) {
	if previous == nil {
		return
	}
	for serverID, server := range previous.servers {
		if carried, ok := next.servers[serverID]; ok && carried.Client == server.Client {
			// The client lives on in the new generation: the retired
			// generation still refuses new leases, but drains without
			// closing a client the successor still owns.
			server.tracker.retire(func() {})
			continue
		}
		server.tracker.retire(clientCloser(deps, server.Client))
	}
}

// clientCloser returns the drain callback of one retired client.
func clientCloser(deps ReloadDeps, client core.Upstream) func() {
	return func() {
		if deps.Close != nil {
			_ = deps.Close(client)
		}
	}
}

func previousServer(assembly *Assembly, serverID string) (ServerRuntime, bool) {
	if assembly == nil {
		return ServerRuntime{}, false
	}
	return assembly.Server(serverID)
}

func previousSnapshot(assembly *Assembly) *storage.Snapshot {
	if assembly == nil {
		return nil
	}
	return assembly.snapshot
}

func endpointFingerprint(endpoint string) string {
	sum := sha256.Sum256([]byte(endpoint))
	return hex.EncodeToString(sum[:])
}

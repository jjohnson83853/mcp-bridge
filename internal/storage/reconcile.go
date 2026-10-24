package storage

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Reconciliation failures. A rejected cycle changes nothing (plan §4.10 step 4).
var (
	ErrRebindTombstoned = errors.New("storage: tombstoned server cannot be rebound")
	ErrEndpointChanged  = errors.New("storage: server endpoint identity changed")
)

// ConfiguredServer is one server of the current flat config membership.
type ConfiguredServer struct {
	ServerID            string
	EndpointFingerprint string
	NegotiatedVersion   string
}

// ReconcileInput is one membership reconciliation cycle: the configured
// servers plus the config/model identities of the run that produced it.
type ReconcileInput struct {
	CreatedAt               time.Time
	ConfigPublicFingerprint string
	CatalogRevision         string
	ModelIdentityDigest     string
	Servers                 []ConfiguredServer
}

// Reconcile derives the next durable snapshot for the configured membership
// (plan §4.9/§4.10). Matching servers are retained with their exact catalog;
// a live server whose endpoint identity changed rebinds to the new endpoint
// with its catalog and cache purged; a server that left the config is purged
// so its cache is unreachable and tombstoned so it never rebinds; a newly
// configured server enters unbound; a model identity change invalidates
// vectors but never exact definitions. The previous snapshot is never mutated
// and a rejected cycle publishes nothing.
func Reconcile(previous *Snapshot, input ReconcileInput) (*Snapshot, error) {
	if err := input.validate(); err != nil {
		return nil, err
	}
	if previous != nil {
		if err := previous.Validate(); err != nil {
			return nil, fmt.Errorf("storage: previous snapshot is invalid: %w", err)
		}
	}

	generation := uint64(1)
	var bindings []Binding
	var servers []Server
	var tools []Tool
	var embeddings []Embedding
	if previous != nil {
		generation = previous.Envelope.GenerationSequence + 1
		bindings = previous.Bindings
		servers = previous.Servers
		tools = previous.Tools
		embeddings = previous.Embeddings
	}

	configured := make(map[string]ConfiguredServer, len(input.Servers))
	for _, server := range input.Servers {
		configured[server.ServerID] = server
	}
	known := make(map[string]bool, len(bindings))
	for _, binding := range bindings {
		known[binding.ServerID] = true
	}

	// Bindings: retain matching servers, tombstone departures, admit first binds.
	nextBindings := make([]Binding, 0, len(bindings)+len(input.Servers))
	purged := make(map[string]bool)
	for _, binding := range bindings {
		server, stillConfigured := configured[binding.ServerID]
		switch {
		case stillConfigured && binding.Retired:
			return nil, fmt.Errorf("%w: server %q", ErrRebindTombstoned, binding.ServerID)
		case stillConfigured && binding.EndpointFingerprint != server.EndpointFingerprint:
			purged[binding.ServerID] = true
			nextBindings = append(nextBindings, Binding{
				ServerID:            binding.ServerID,
				EndpointFingerprint: server.EndpointFingerprint,
			})
		case stillConfigured:
			nextBindings = append(nextBindings, binding)
		case binding.Retired:
			nextBindings = append(nextBindings, binding)
		default:
			purged[binding.ServerID] = true
			nextBindings = append(nextBindings, Binding{
				ServerID:            binding.ServerID,
				EndpointFingerprint: binding.EndpointFingerprint,
				Retired:             true,
			})
		}
	}
	for _, server := range input.Servers {
		if known[server.ServerID] {
			continue
		}
		nextBindings = append(nextBindings, Binding{
			ServerID:            server.ServerID,
			EndpointFingerprint: server.EndpointFingerprint,
		})
	}

	// Health rows: only live configured servers keep a row.
	rows := make(map[string]Server, len(servers))
	for _, server := range servers {
		rows[server.ServerID] = server
	}
	nextServers := make([]Server, 0, len(input.Servers))
	for _, configuredServer := range input.Servers {
		row, existing := rows[configuredServer.ServerID]
		if !existing || purged[configuredServer.ServerID] {
			row = Server{ServerID: configuredServer.ServerID, CatalogStatus: StatusNone}
		}
		row.EndpointFingerprint = configuredServer.EndpointFingerprint
		if configuredServer.NegotiatedVersion != "" {
			row.NegotiatedVersion = configuredServer.NegotiatedVersion
		}
		nextServers = append(nextServers, row)
	}

	// Tools of removed servers are unreachable; vectors are current only while
	// the model identity still matches.
	keptIdentities := make(map[string]bool, len(tools))
	nextTools := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		if purged[tool.ServerID] {
			continue
		}
		nextTools = append(nextTools, tool)
		keptIdentities[ToolIdentity(tool.ServerID, tool.RawName)] = true
	}
	nextEmbeddings := make([]Embedding, 0, len(embeddings))
	for _, embedding := range embeddings {
		if !keptIdentities[embedding.ToolIdentity] {
			continue
		}
		if embedding.ModelIdentityDigest != input.ModelIdentityDigest {
			continue
		}
		nextEmbeddings = append(nextEmbeddings, embedding)
	}

	next := &Snapshot{
		Envelope: Envelope{
			SchemaVersion:           SchemaVersion,
			GenerationSequence:      generation,
			CreatedAt:               input.CreatedAt.UTC(),
			ConfigPublicFingerprint: input.ConfigPublicFingerprint,
			CatalogRevision:         input.CatalogRevision,
		},
		Bindings:   nextBindings,
		Servers:    nextServers,
		Tools:      nextTools,
		Embeddings: nextEmbeddings,
	}
	canonicalize(next)
	if err := next.Validate(); err != nil {
		return nil, fmt.Errorf("storage: reconciled snapshot is invalid: %w", err)
	}
	return next, nil
}

// canonicalize orders every collection by its natural key so a reconciled
// snapshot is byte-identical regardless of input ordering.
func canonicalize(snapshot *Snapshot) {
	sort.Slice(snapshot.Bindings, func(i, j int) bool { return snapshot.Bindings[i].ServerID < snapshot.Bindings[j].ServerID })
	sort.Slice(snapshot.Servers, func(i, j int) bool { return snapshot.Servers[i].ServerID < snapshot.Servers[j].ServerID })
	sort.Slice(snapshot.Tools, func(i, j int) bool { return snapshot.Tools[i].PublicName < snapshot.Tools[j].PublicName })
	sort.Slice(snapshot.Embeddings, func(i, j int) bool {
		return snapshot.Embeddings[i].ToolIdentity < snapshot.Embeddings[j].ToolIdentity
	})
}

func (in ReconcileInput) validate() error {
	if in.CreatedAt.IsZero() {
		return errors.New("storage: reconcile needs a created_at")
	}
	if !revisionPattern.MatchString(in.CatalogRevision) {
		return fmt.Errorf("storage: catalog_revision %q is not a canonical revision", in.CatalogRevision)
	}
	if strings.TrimSpace(in.ConfigPublicFingerprint) == "" {
		return errors.New("storage: reconcile needs a config_public_fingerprint")
	}
	if !hexDigest(in.ModelIdentityDigest) {
		return fmt.Errorf("storage: model identity digest %q is not canonical", in.ModelIdentityDigest)
	}
	seen := make(map[string]bool, len(in.Servers))
	for _, server := range in.Servers {
		if !serverIDPattern.MatchString(server.ServerID) {
			return fmt.Errorf("storage: server_id %q violates the server identity pattern", server.ServerID)
		}
		if seen[server.ServerID] {
			return fmt.Errorf("storage: duplicate configured server %q", server.ServerID)
		}
		seen[server.ServerID] = true
		if strings.TrimSpace(server.EndpointFingerprint) == "" {
			return fmt.Errorf("storage: configured server %q has no endpoint fingerprint", server.ServerID)
		}
	}
	return nil
}

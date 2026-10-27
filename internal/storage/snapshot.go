// Package storage persists the bridge's durable catalog snapshot: a flat,
// schema-versioned, checksummed document published atomically (plan §4.9).
package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

// Schema version constants of the flat on-disk snapshot schema. Version 1 is
// the frozen pre-tool-metadata representation (snapshot_v1.go); the writer
// publishes version 2, which adds optional source metadata. The new reader
// strictly verifies both with version-specific checksums; old binaries never
// receive schema 2 (approved design decisions 13-15).
const (
	schemaVersionLegacy = 1
	SchemaVersion       = 2
)

// Catalog status vocabulary (plan §4.7).
const (
	StatusFresh = "fresh"
	StatusStale = "stale"
	StatusNone  = "none"
)

var (
	publicNamePattern = regexp.MustCompile(catalog.PublicToolNamePattern)
	revisionPattern   = regexp.MustCompile(catalog.CatalogRevisionPattern)
	serverIDPattern   = regexp.MustCompile(catalog.ServerIDPattern)
	// reasonPattern is the finite status-reason vocabulary of a stale row: a
	// lowercase token that can never carry a payload (plan §4.11 redaction).
	reasonPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

// Envelope is the immutable publication header. It is embedded in Snapshot, so
// its fields are the snapshot's top-level JSON keys.
type Envelope struct {
	SchemaVersion           int       `json:"schema_version"`
	GenerationSequence      uint64    `json:"generation_sequence"`
	CreatedAt               time.Time `json:"created_at"`
	ConfigPublicFingerprint string    `json:"config_public_fingerprint"`
	CatalogRevision         string    `json:"catalog_revision"`
	SnapshotChecksum        string    `json:"snapshot_checksum"`
}

// Binding records the endpoint identity a server was bound to, or its tombstone.
type Binding struct {
	ServerID            string `json:"server_id"`
	EndpointFingerprint string `json:"endpoint_fingerprint"`
	Retired             bool   `json:"retired"`
}

// Server is one configured server's persisted health row.
type Server struct {
	ServerID            string    `json:"server_id"`
	EndpointFingerprint string    `json:"endpoint_fingerprint"`
	NegotiatedVersion   string    `json:"negotiated_version"`
	LastSuccessAt       time.Time `json:"last_success_at"`
	LastAttemptAt       time.Time `json:"last_attempt_at"`
	CatalogStatus       string    `json:"catalog_status"`
	Reason              string    `json:"reason,omitempty"`
}

// Tool is one indexed tool with its exact definition and content identities.
type Tool struct {
	ServerID             string          `json:"server_id"`
	RawName              string          `json:"rawname"`
	PublicName           string          `json:"publicname"`
	Description          string          `json:"description,omitempty"`
	DefinitionHash       string          `json:"definition_hash"`
	EmbeddingContentHash string          `json:"embedding_content_hash"`
	InputSchema          json.RawMessage `json:"input_schema,omitempty"`
	OutputSchema         json.RawMessage `json:"output_schema,omitempty"`
	// Metadata is the persisted source metadata (schema 2 only). nil is the
	// metadata-absent form and serializes to nothing, so a schema-2 document
	// without metadata keeps the exact schema-1 byte layout of its rows.
	Metadata *core.ToolMetadata `json:"metadata,omitempty"`
}

// Embedding is one non-secret embedding record for a tool content identity.
type Embedding struct {
	ToolIdentity        string    `json:"tool_identity"`
	ContentHash         string    `json:"content_hash"`
	ModelIdentityDigest string    `json:"model_identity_digest"`
	Dimensions          int       `json:"dimensions"`
	FormatVersion       string    `json:"format_version"`
	Vector              []float64 `json:"vector"`
}

// Snapshot is the complete durable state. It carries no endpoint URLs, secret
// references, sessions, arguments, results or query cache (plan §4.9).
type Snapshot struct {
	Envelope
	Bindings   []Binding   `json:"bindings"`
	Servers    []Server    `json:"servers"`
	Tools      []Tool      `json:"tools"`
	Embeddings []Embedding `json:"embeddings"`
}

const PromptSchemaVersion = 1

type Prompt struct {
	ServerID    string          `json:"server_id"`
	RawName     string          `json:"raw_name"`
	PublicName  string          `json:"public_name"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	Arguments   json.RawMessage `json:"arguments,omitempty"`
}

type PromptSnapshot struct {
	SchemaVersion int      `json:"schema_version"`
	Prompts       []Prompt `json:"prompts"`
	Checksum      string   `json:"checksum"`
}

func (p *PromptSnapshot) ChecksumValue() (string, error) {
	canonical := *p
	canonical.Checksum = ""
	encoded, err := json.Marshal(&canonical)
	if err != nil {
		return "", fmt.Errorf("storage: prompt canonical encoding failed: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func (p *PromptSnapshot) Validate() error {
	if p == nil || p.SchemaVersion != PromptSchemaVersion {
		return fmt.Errorf("storage: unsupported prompt schema version")
	}
	seen := make(map[string]bool, len(p.Prompts))
	for _, prompt := range p.Prompts {
		if !serverIDPattern.MatchString(prompt.ServerID) || prompt.RawName == "" || prompt.PublicName == "" {
			return fmt.Errorf("storage: invalid prompt identity")
		}
		if seen[prompt.PublicName] {
			return fmt.Errorf("storage: duplicate prompt public name")
		}
		seen[prompt.PublicName] = true
		if len(prompt.Arguments) > 0 && !json.Valid(prompt.Arguments) {
			return fmt.Errorf("storage: invalid prompt arguments")
		}
	}
	return nil
}

// ToolIdentity is the server-derived tool identity keying embedding records.
func ToolIdentity(serverID, rawName string) string {
	return serverID + "\x00" + rawName
}

// Checksum is the canonical checksum of the snapshot excluding the checksum
// field itself.
func (s *Snapshot) Checksum() (string, error) {
	canonical := *s
	canonical.Envelope.SnapshotChecksum = ""
	encoded, err := json.Marshal(&canonical)
	if err != nil {
		return "", fmt.Errorf("storage: canonical encoding failed: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

// BindingAllowed reports whether a server may (re)use the given endpoint
// identity. Tombstoned servers never rebind; a first bind always may.
func (s *Snapshot) BindingAllowed(serverID, endpointFingerprint string) bool {
	for _, binding := range s.Bindings {
		if binding.ServerID != serverID {
			continue
		}
		if binding.Retired {
			return false
		}
		return binding.EndpointFingerprint == endpointFingerprint
	}
	return true
}

// StampServerSuccess marks one live server row fresh after a complete accepted
// discovery: the attempt and the last-success time both move to at and any
// stale reason is cleared (plan §4.7). The input snapshot is never mutated; the
// returned snapshot shares the untouched slices.
func StampServerSuccess(s *Snapshot, serverID string, at time.Time) (*Snapshot, error) {
	return stampServerRow(s, serverID, at, func(row *Server, at time.Time) {
		row.LastAttemptAt = at
		row.LastSuccessAt = at
		row.CatalogStatus = StatusFresh
		row.Reason = ""
	})
}

// StampServerFailure marks one live server row stale with a finite, payload-free
// reason after a failed refresh or discovery. Only the attempt time advances, so
// the last COMPLETE accepted catalog stays the row's success and its age keeps
// the outage visible (plan §4.7).
func StampServerFailure(s *Snapshot, serverID, reason string, at time.Time) (*Snapshot, error) {
	if !reasonPattern.MatchString(reason) {
		return nil, fmt.Errorf("storage: server %q needs a finite status reason", serverID)
	}
	return stampServerRow(s, serverID, at, func(row *Server, at time.Time) {
		row.LastAttemptAt = at
		row.CatalogStatus = StatusStale
		row.Reason = reason
	})
}

// StampServerEmpty marks one live server row as having no catalog at all after a
// cold local admission in which no upstream was contacted or none answered: the
// status is none, the stale reason clears, and the attempt time advances while
// the last-success time resets because no complete catalog was ever accepted
// (plan §4.5).
func StampServerEmpty(s *Snapshot, serverID string, at time.Time) (*Snapshot, error) {
	return stampServerRow(s, serverID, at, func(row *Server, at time.Time) {
		row.LastAttemptAt = at
		row.LastSuccessAt = time.Time{}
		row.CatalogStatus = StatusNone
		row.Reason = ""
	})
}

// stampServerRow applies one row transition and returns a snapshot whose Servers
// slice is a copy; every other slice is shared, since stamping never touches it.
func stampServerRow(s *Snapshot, serverID string, at time.Time, transition func(*Server, time.Time)) (*Snapshot, error) {
	if s == nil {
		return nil, errors.New("storage: no snapshot to stamp")
	}
	if at.IsZero() {
		return nil, fmt.Errorf("storage: server %q needs a non-zero attempt time", serverID)
	}
	index := -1
	for i, row := range s.Servers {
		if row.ServerID == serverID {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, fmt.Errorf("storage: no live server row %q to stamp", serverID)
	}
	rows := make([]Server, len(s.Servers))
	copy(rows, s.Servers)
	transition(&rows[index], at.UTC())
	stamped := *s
	stamped.Servers = rows
	return &stamped, nil
}

// Validate checks bounds, version, identity, schemas and vectors. A snapshot
// that fails validation is never published and never trusted.
func (s *Snapshot) Validate() error {
	switch s.SchemaVersion {
	case schemaVersionLegacy:
		// A legacy snapshot in canonical memory must be metadata-absent: the
		// frozen schema-1 representation cannot carry metadata.
		for _, tool := range s.Tools {
			if tool.Metadata != nil {
				return fmt.Errorf("storage: schema_version 1 tool %q carries metadata", tool.PublicName)
			}
		}
	case SchemaVersion:
	default:
		return fmt.Errorf("storage: unsupported schema_version %d", s.SchemaVersion)
	}
	if !revisionPattern.MatchString(s.CatalogRevision) {
		return fmt.Errorf("storage: catalog_revision %q is not a canonical revision", s.CatalogRevision)
	}
	if strings.TrimSpace(s.ConfigPublicFingerprint) == "" {
		return fmt.Errorf("storage: config_public_fingerprint is required")
	}
	if s.CreatedAt.IsZero() || s.CreatedAt.Location() != time.UTC {
		return fmt.Errorf("storage: created_at must be a non-zero UTC timestamp")
	}

	bindings := make(map[string]Binding, len(s.Bindings))
	for _, binding := range s.Bindings {
		if !serverIDPattern.MatchString(binding.ServerID) {
			return fmt.Errorf("storage: binding server_id %q violates the server identity pattern", binding.ServerID)
		}
		if _, duplicate := bindings[binding.ServerID]; duplicate {
			return fmt.Errorf("storage: duplicate binding for server %q", binding.ServerID)
		}
		if strings.TrimSpace(binding.EndpointFingerprint) == "" {
			return fmt.Errorf("storage: binding for server %q has no endpoint fingerprint", binding.ServerID)
		}
		bindings[binding.ServerID] = binding
	}

	servers := make(map[string]Server, len(s.Servers))
	for _, server := range s.Servers {
		if !serverIDPattern.MatchString(server.ServerID) {
			return fmt.Errorf("storage: server_id %q violates the server identity pattern", server.ServerID)
		}
		if _, duplicate := servers[server.ServerID]; duplicate {
			return fmt.Errorf("storage: duplicate server row %q", server.ServerID)
		}
		binding, bound := bindings[server.ServerID]
		if !bound {
			return fmt.Errorf("storage: server %q has no binding", server.ServerID)
		}
		if binding.Retired {
			return fmt.Errorf("storage: retired server %q still has a live server row", server.ServerID)
		}
		if binding.EndpointFingerprint != server.EndpointFingerprint {
			return fmt.Errorf("storage: server %q endpoint fingerprint does not match its binding", server.ServerID)
		}
		switch server.CatalogStatus {
		case StatusFresh, StatusStale, StatusNone:
		default:
			return fmt.Errorf("storage: server %q has catalog_status %q", server.ServerID, server.CatalogStatus)
		}
		if server.CatalogStatus == StatusStale && strings.TrimSpace(server.Reason) == "" {
			return fmt.Errorf("storage: stale server %q needs a safe reason", server.ServerID)
		}
		servers[server.ServerID] = server
	}

	type toolKey struct{ server, raw string }
	toolIdentities := make(map[string]bool, len(s.Tools))
	publicNames := make(map[string]bool, len(s.Tools))
	serverTools := make(map[toolKey]bool, len(s.Tools))
	for _, tool := range s.Tools {
		if _, known := servers[tool.ServerID]; !known {
			return fmt.Errorf("storage: tool %q belongs to unknown server %q", tool.PublicName, tool.ServerID)
		}
		if !publicNamePattern.MatchString(tool.PublicName) {
			return fmt.Errorf("storage: public name %q violates the stable naming pattern", tool.PublicName)
		}
		if strings.TrimSpace(tool.RawName) == "" {
			return fmt.Errorf("storage: tool %q has no raw name", tool.PublicName)
		}
		if !hexDigest(tool.DefinitionHash) {
			return fmt.Errorf("storage: tool %q has no exact definition hash", tool.PublicName)
		}
		if !hexDigest(tool.EmbeddingContentHash) {
			return fmt.Errorf("storage: tool %q has no embedding content hash", tool.PublicName)
		}
		if tool.Metadata != nil {
			if err := tool.Metadata.Validate(); err != nil {
				return fmt.Errorf("storage: tool %q metadata: %w", tool.PublicName, err)
			}
		}
		for _, schema := range []struct {
			label string
			body  json.RawMessage
		}{{"input", tool.InputSchema}, {"output", tool.OutputSchema}} {
			if len(schema.body) == 0 {
				continue
			}
			// Structural validity only: a schema the bridge cannot resolve is
			// still searchable and describable and must survive a restart
			// verbatim (plan §4.4, §4.5, AC-10), so the durable catalog may
			// retain it. Resolvability is demanded at dispatch admission.
			if err := catalog.ValidateSchemaStructure(string(schema.body)); err != nil {
				return fmt.Errorf("storage: tool %q %s schema is invalid: %w", tool.PublicName, schema.label, err)
			}
		}
		key := toolKey{server: tool.ServerID, raw: tool.RawName}
		if serverTools[key] {
			return fmt.Errorf("storage: duplicate tool %q on server %q", tool.RawName, tool.ServerID)
		}
		serverTools[key] = true
		if publicNames[tool.PublicName] {
			return fmt.Errorf("storage: duplicate public name %q", tool.PublicName)
		}
		publicNames[tool.PublicName] = true
		toolIdentities[ToolIdentity(tool.ServerID, tool.RawName)] = true
	}

	embeddings := make(map[string]bool, len(s.Embeddings))
	for _, embedding := range s.Embeddings {
		if !toolIdentities[embedding.ToolIdentity] {
			return fmt.Errorf("storage: embedding for unknown tool identity %q", embedding.ToolIdentity)
		}
		if embeddings[embedding.ToolIdentity] {
			return fmt.Errorf("storage: duplicate embedding for tool identity %q", embedding.ToolIdentity)
		}
		embeddings[embedding.ToolIdentity] = true
		if embedding.Dimensions <= 0 || len(embedding.Vector) != embedding.Dimensions {
			return fmt.Errorf("storage: embedding dimensions %d do not match its vector", embedding.Dimensions)
		}
		norm := 0.0
		for _, value := range embedding.Vector {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return fmt.Errorf("storage: embedding vector for %q is not finite", embedding.ToolIdentity)
			}
			norm += value * value
		}
		if norm == 0 {
			return fmt.Errorf("storage: embedding vector for %q is zero", embedding.ToolIdentity)
		}
		if !hexDigest(embedding.ContentHash) || !hexDigest(embedding.ModelIdentityDigest) {
			return fmt.Errorf("storage: embedding for %q is missing its content or model identity digest", embedding.ToolIdentity)
		}
		if strings.TrimSpace(embedding.FormatVersion) == "" {
			return fmt.Errorf("storage: embedding for %q is missing its format version", embedding.ToolIdentity)
		}
	}
	return nil
}

func hexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

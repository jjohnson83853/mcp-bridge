package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// schemaVersionLegacy is the frozen on-disk snapshot schema version 1, the
// complete pre-tool-metadata representation (defined with the writer/reader
// version constants in snapshot.go). Legacy bytes are verified only
// through these frozen types — never through the canonical schema-2 model —
// so a schema-1 checksum keeps its exact historical semantics.

// envelopeV1 is the frozen schema-1 publication header.
type envelopeV1 struct {
	SchemaVersion           int       `json:"schema_version"`
	GenerationSequence      uint64    `json:"generation_sequence"`
	CreatedAt               time.Time `json:"created_at"`
	ConfigPublicFingerprint string    `json:"config_public_fingerprint"`
	CatalogRevision         string    `json:"catalog_revision"`
	SnapshotChecksum        string    `json:"snapshot_checksum"`
}

// bindingV1 is the frozen schema-1 binding row.
type bindingV1 struct {
	ServerID            string `json:"server_id"`
	EndpointFingerprint string `json:"endpoint_fingerprint"`
	Retired             bool   `json:"retired"`
}

// serverV1 is the frozen schema-1 server health row.
type serverV1 struct {
	ServerID            string    `json:"server_id"`
	EndpointFingerprint string    `json:"endpoint_fingerprint"`
	NegotiatedVersion   string    `json:"negotiated_version"`
	LastSuccessAt       time.Time `json:"last_success_at"`
	LastAttemptAt       time.Time `json:"last_attempt_at"`
	CatalogStatus       string    `json:"catalog_status"`
	Reason              string    `json:"reason,omitempty"`
}

// toolV1 is the frozen schema-1 tool row: it has no metadata member by
// definition, so any metadata member in legacy bytes is an unknown field and
// the strict decode rejects it.
type toolV1 struct {
	ServerID             string          `json:"server_id"`
	RawName              string          `json:"rawname"`
	PublicName           string          `json:"publicname"`
	Description          string          `json:"description,omitempty"`
	DefinitionHash       string          `json:"definition_hash"`
	EmbeddingContentHash string          `json:"embedding_content_hash"`
	InputSchema          json.RawMessage `json:"input_schema,omitempty"`
	OutputSchema         json.RawMessage `json:"output_schema,omitempty"`
}

// embeddingV1 is the frozen schema-1 embedding record.
type embeddingV1 struct {
	ToolIdentity        string    `json:"tool_identity"`
	ContentHash         string    `json:"content_hash"`
	ModelIdentityDigest string    `json:"model_identity_digest"`
	Dimensions          int       `json:"dimensions"`
	FormatVersion       string    `json:"format_version"`
	Vector              []float64 `json:"vector"`
}

// snapshotV1 is the frozen schema-1 snapshot document.
type snapshotV1 struct {
	envelopeV1
	Bindings   []bindingV1   `json:"bindings"`
	Servers    []serverV1    `json:"servers"`
	Tools      []toolV1      `json:"tools"`
	Embeddings []embeddingV1 `json:"embeddings"`
}

// checksum reproduces the schema-1 canonical checksum: SHA-256 over the
// marshaled document with the checksum field blanked.
func (s *snapshotV1) checksum() (string, error) {
	canonical := *s
	canonical.SnapshotChecksum = ""
	encoded, err := json.Marshal(&canonical)
	if err != nil {
		return "", fmt.Errorf("storage: schema-1 canonical encoding failed: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

// toCanonical converts a verified schema-1 snapshot into canonical memory with
// metadata absent. Load alone never rewrites the file; the next accepted save
// publishes schema 2.
func (s *snapshotV1) toCanonical() *Snapshot {
	snapshot := &Snapshot{
		Envelope: Envelope{
			SchemaVersion:           s.SchemaVersion,
			GenerationSequence:      s.GenerationSequence,
			CreatedAt:               s.CreatedAt,
			ConfigPublicFingerprint: s.ConfigPublicFingerprint,
			CatalogRevision:         s.CatalogRevision,
			SnapshotChecksum:        s.SnapshotChecksum,
		},
	}
	for _, binding := range s.Bindings {
		snapshot.Bindings = append(snapshot.Bindings, Binding(binding))
	}
	for _, server := range s.Servers {
		snapshot.Servers = append(snapshot.Servers, Server(server))
	}
	for _, tool := range s.Tools {
		snapshot.Tools = append(snapshot.Tools, Tool{
			ServerID:             tool.ServerID,
			RawName:              tool.RawName,
			PublicName:           tool.PublicName,
			Description:          tool.Description,
			DefinitionHash:       tool.DefinitionHash,
			EmbeddingContentHash: tool.EmbeddingContentHash,
			InputSchema:          tool.InputSchema,
			OutputSchema:         tool.OutputSchema,
		})
	}
	for _, embedding := range s.Embeddings {
		snapshot.Embeddings = append(snapshot.Embeddings, Embedding(embedding))
	}
	return snapshot
}

// decodeSnapshotV1 strictly decodes schema-1 bytes: unknown fields and
// trailing data are rejected, and the recorded checksum must match the frozen
// schema-1 canonical checksum.
func decodeSnapshotV1(raw []byte) (*Snapshot, error) {
	snapshot := &snapshotV1{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(snapshot); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(struct{})); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("trailing data after schema-1 snapshot")
	}
	checksum, err := snapshot.checksum()
	if err != nil {
		return nil, err
	}
	if snapshot.SnapshotChecksum != checksum {
		return nil, fmt.Errorf("schema-1 checksum mismatch")
	}
	return snapshot.toCanonical(), nil
}

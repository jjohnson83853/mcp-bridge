package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Store layout constants (plan §4.9).
const (
	SnapshotFile       = "snapshot.json"
	lockFile           = ".writer.lock"
	PromptSnapshotFile = "prompts.json"
	directoryMode      = 0o700
	fileMode           = 0o600
)

// Sentinel errors the bridge maps onto its own failure vocabulary.
var (
	ErrNoSnapshot   = errors.New("storage: no snapshot")
	ErrCorrupt      = errors.New("storage: snapshot is corrupt")
	ErrWriterLocked = errors.New("storage: store already has a writer")
)

// Store owns one private state directory with a single exclusive writer.
type Store struct {
	directory     string
	lock          *os.File
	sequence      uint64
	trusted       bool
	closed        bool
	promptTrusted bool
}

// Open creates the private state directory and takes the exclusive writer lock.
// The lock is an advisory kernel flock on .writer.lock, so a stale lockfile left
// by a dead process is recovered automatically while a live writer is respected.
func Open(directory string) (*Store, error) {
	if err := os.MkdirAll(directory, directoryMode); err != nil {
		return nil, fmt.Errorf("storage: create state directory: %w", err)
	}
	lock, err := os.OpenFile(filepath.Join(directory, lockFile), os.O_CREATE|os.O_RDWR, fileMode)
	if err != nil {
		return nil, fmt.Errorf("storage: take writer lock: %w", err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, fmt.Errorf("%w: %s", ErrWriterLocked, directory)
		}
		return nil, fmt.Errorf("storage: take writer lock: %w", err)
	}
	return &Store{directory: directory, lock: lock}, nil
}

// Close releases the writer lock. It is idempotent.
func (s *Store) Close() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	_ = syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)
	closeErr := s.lock.Close()
	if err := os.Remove(filepath.Join(s.directory, lockFile)); err != nil && !errors.Is(err, os.ErrNotExist) {
		if closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}

// Dir is the private state directory.
func (s *Store) Dir() string { return s.directory }

// Path is the published snapshot path.
func (s *Store) Path() string       { return filepath.Join(s.directory, SnapshotFile) }
func (s *Store) PromptPath() string { return filepath.Join(s.directory, PromptSnapshotFile) }

func (s *Store) LoadPrompts() (*PromptSnapshot, error) {
	raw, err := os.ReadFile(s.PromptPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNoSnapshot
		}
		return nil, fmt.Errorf("storage: read prompt snapshot: %w", err)
	}
	prompts := &PromptSnapshot{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(prompts); err != nil {
		return nil, fmt.Errorf("%w: decode prompt snapshot: %v", ErrCorrupt, err)
	}
	if err := decoder.Decode(new(struct{})); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: trailing prompt snapshot data", ErrCorrupt)
	}
	if err := prompts.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	checksum, err := prompts.ChecksumValue()
	if err != nil || prompts.Checksum != checksum {
		return nil, fmt.Errorf("%w: prompt checksum mismatch", ErrCorrupt)
	}
	s.promptTrusted = true
	return prompts, nil
}

func (s *Store) SavePrompts(prompts *PromptSnapshot) error {
	if s == nil || s.closed {
		return errors.New("storage: store is closed")
	}
	if prompts == nil {
		return errors.New("storage: nil prompt snapshot")
	}
	if err := prompts.Validate(); err != nil {
		return err
	}
	if !s.promptTrusted {
		if _, err := s.LoadPrompts(); err != nil && !errors.Is(err, ErrNoSnapshot) {
			return fmt.Errorf("storage: refusing to overwrite untrusted prompt snapshot: %w", err)
		}
	}
	checksum, err := prompts.ChecksumValue()
	if err != nil {
		return err
	}
	prompts.Checksum = checksum
	encoded, err := json.Marshal(prompts)
	if err != nil {
		return fmt.Errorf("storage: encode prompt snapshot: %w", err)
	}
	if err := s.publishFile(PromptSnapshotFile, encoded); err != nil {
		return err
	}
	s.promptTrusted = true
	return nil
}

// Load reads and fully re-validates the published snapshot. A missing file is
// ErrNoSnapshot; every rejection is ErrCorrupt and leaves the file untouched.
// The decoder is selected by the document's schema version: schema 1 is
// verified through the frozen legacy representation with schema-1 checksum
// semantics, schema 2 through the canonical representation with schema-2
// checksum semantics (approved design decision 13).
func (s *Store) Load() (*Snapshot, error) {
	raw, err := os.ReadFile(s.Path())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNoSnapshot
		}
		return nil, fmt.Errorf("storage: read snapshot: %w", err)
	}
	var probe struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, fmt.Errorf("%w: decode envelope: %v", ErrCorrupt, err)
	}
	var snapshot *Snapshot
	switch probe.SchemaVersion {
	case schemaVersionLegacy:
		snapshot, err = decodeSnapshotV1(raw)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
		}
	case SchemaVersion:
		snapshot = &Snapshot{}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(snapshot); err != nil {
			return nil, fmt.Errorf("%w: decode: %v", ErrCorrupt, err)
		}
		if err := decoder.Decode(new(struct{})); !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%w: trailing data after snapshot", ErrCorrupt)
		}
		checksum, err := snapshot.Checksum()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
		}
		if snapshot.Envelope.SnapshotChecksum != checksum {
			return nil, fmt.Errorf("%w: checksum mismatch", ErrCorrupt)
		}
	default:
		return nil, fmt.Errorf("%w: unsupported schema_version %d", ErrCorrupt, probe.SchemaVersion)
	}
	if err := snapshot.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	s.trusted = true
	return snapshot, nil
}

// Save validates the snapshot, stamps its canonical checksum and publishes it
// atomically. A rejected or failed save never disturbs the previous snapshot,
// and a published snapshot that does not re-validate is never destructively
// overwritten: the save is refused with ErrCorrupt (plan §4.9).
func (s *Store) Save(snapshot *Snapshot) error {
	if s == nil || s.closed {
		return errors.New("storage: store is closed")
	}
	if snapshot == nil {
		return errors.New("storage: nil snapshot")
	}
	// The writer publishes the current schema: a snapshot admitted from
	// legacy state is upgraded to schema 2 by its next accepted save
	// (approved design decision 14). The stamp precedes validation so the
	// published document is validated exactly as written. The stamps apply
	// to an owned copy: the caller's snapshot is never mutated in place.
	owned := *snapshot
	owned.SchemaVersion = SchemaVersion
	if err := owned.Validate(); err != nil {
		return err
	}
	if err := s.ensureReplaceable(); err != nil {
		return err
	}
	checksum, err := owned.Checksum()
	if err != nil {
		return err
	}
	owned.Envelope.SnapshotChecksum = checksum
	encoded, err := json.Marshal(&owned)
	if err != nil {
		return fmt.Errorf("storage: encode snapshot: %w", err)
	}
	return s.publish(encoded)
}

// publish writes a private temp file, fsyncs it, renames it over the published
// path and fsyncs the directory.
func (s *Store) publishFile(name string, encoded []byte) error {
	s.sequence++
	temp := filepath.Join(s.directory, fmt.Sprintf(".%s.%d.tmp", name, s.sequence))
	file, err := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, fileMode)
	if err != nil {
		return fmt.Errorf("storage: create temp %s: %w", name, err)
	}
	if _, err := file.Write(encoded); err != nil {
		file.Close()
		os.Remove(temp)
		return fmt.Errorf("storage: write temp %s: %w", name, err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(temp)
		return fmt.Errorf("storage: sync temp %s: %w", name, err)
	}
	if err := file.Close(); err != nil {
		os.Remove(temp)
		return fmt.Errorf("storage: close temp %s: %w", name, err)
	}
	if err := os.Rename(temp, filepath.Join(s.directory, name)); err != nil {
		os.Remove(temp)
		return fmt.Errorf("storage: publish %s: %w", name, err)
	}
	return syncDirectory(s.directory)
}
func (s *Store) publish(encoded []byte) error {
	s.sequence++
	temp := filepath.Join(s.directory, fmt.Sprintf(".%s.%d.tmp", SnapshotFile, s.sequence))
	file, err := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, fileMode)
	if err != nil {
		return fmt.Errorf("storage: create temp snapshot: %w", err)
	}
	if _, err := file.Write(encoded); err != nil {
		file.Close()
		os.Remove(temp)
		return fmt.Errorf("storage: write temp snapshot: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(temp)
		return fmt.Errorf("storage: sync temp snapshot: %w", err)
	}
	if err := file.Close(); err != nil {
		os.Remove(temp)
		return fmt.Errorf("storage: close temp snapshot: %w", err)
	}
	if err := os.Rename(temp, s.Path()); err != nil {
		os.Remove(temp)
		return fmt.Errorf("storage: publish snapshot: %w", err)
	}
	s.trusted = true
	return syncDirectory(s.directory)
}

// ensureReplaceable reports whether the currently published snapshot may be
// replaced. A snapshot this store already validated, or one it published
// itself, is replaceable; a corrupt file is not, so its bytes survive for an
// operator to inspect (plan §4.9).
func (s *Store) ensureReplaceable() error {
	if s.trusted {
		return nil
	}
	switch _, err := s.Load(); {
	case errors.Is(err, ErrNoSnapshot):
		return nil
	case err == nil:
		return nil
	default:
		return fmt.Errorf("storage: refusing to overwrite untrusted snapshot: %w", err)
	}
}

func syncDirectory(directory string) error {
	handle, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("storage: open state directory: %w", err)
	}
	defer handle.Close()
	if err := handle.Sync(); err != nil {
		return fmt.Errorf("storage: sync state directory: %w", err)
	}
	return nil
}

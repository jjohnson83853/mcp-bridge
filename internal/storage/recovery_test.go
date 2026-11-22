package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func openRecoveryTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	return store
}

func validRecoverySnapshot() *Snapshot {
	tool := Tool{
		ServerID:             "portainer",
		RawName:              "list_containers",
		PublicName:           "t_" + strings.Repeat("a", 52),
		Description:          "list running containers",
		DefinitionHash:       strings.Repeat("d", 64),
		EmbeddingContentHash: strings.Repeat("e", 64),
		InputSchema:          json.RawMessage(`{"type":"object"}`),
		OutputSchema:         json.RawMessage(`{"type":"object"}`),
	}
	return &Snapshot{
		Envelope: Envelope{
			SchemaVersion:           SchemaVersion,
			GenerationSequence:      7,
			CreatedAt:               time.Date(2026, 10, 9, 5, 4, 3, 123456789, time.UTC),
			ConfigPublicFingerprint: strings.Repeat("c", 64),
			CatalogRevision:         strings.Repeat("a", 64),
		},
		Bindings: []Binding{{ServerID: "portainer", EndpointFingerprint: strings.Repeat("b", 64)}},
		Servers: []Server{{
			ServerID:            "portainer",
			EndpointFingerprint: strings.Repeat("b", 64),
			NegotiatedVersion:   "2025-06-18",
			LastSuccessAt:       time.Date(2026, 10, 9, 5, 0, 0, 0, time.UTC),
			LastAttemptAt:       time.Date(2026, 10, 9, 5, 0, 0, 0, time.UTC),
			CatalogStatus:       StatusFresh,
		}},
		Tools: []Tool{tool},
		Embeddings: []Embedding{{
			ToolIdentity:        ToolIdentity("portainer", "list_containers"),
			ContentHash:         strings.Repeat("e", 64),
			ModelIdentityDigest: strings.Repeat("f", 64),
			Dimensions:          1024,
			FormatVersion:       "v1",
			Vector:              recoveryVector(1024),
		}},
	}
}

func recoveryVector(dimensions int) []float64 {
	vector := make([]float64, dimensions)
	for i := range vector {
		vector[i] = 1 + float64(i)/1000
	}
	return vector
}

func validRecoveryPrompts() *PromptSnapshot {
	prompts := &PromptSnapshot{
		SchemaVersion: PromptSchemaVersion,
		Prompts: []Prompt{{
			ServerID:   "portainer",
			RawName:    "greet",
			PublicName: "p_greet",
		}},
	}
	checksum, _ := prompts.ChecksumValue()
	prompts.Checksum = checksum
	return prompts
}

func setRecoveryClock(t *testing.T, at time.Time) {
	t.Helper()
	previous := now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = previous })
}

func recoveryNames(t *testing.T, directory string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool, len(entries))
	for _, entry := range entries {
		names[entry.Name()] = true
	}
	return names
}

func seedRecoveryQuarantine(t *testing.T, directory, family string, at time.Time, content []byte) string {
	t.Helper()
	name := fmt.Sprintf("%s.corrupt-%s.json", family, at.UTC().Format(time.RFC3339Nano))
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func corruptSnapshotBytes(t *testing.T, store *Store) []byte {
	t.Helper()
	if err := store.Save(validRecoverySnapshot()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	corrupt := bytes.Replace(raw, []byte("list running containers"), []byte("changed running containers"), 1)
	if bytes.Equal(corrupt, raw) {
		t.Fatal("valid snapshot fixture did not contain the expected description")
	}
	return corrupt
}

func TestRecoverNoopValidAbsentAndSymlinkStates(t *testing.T) {
	t.Run("valid files", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		if err := store.Save(validRecoverySnapshot()); err != nil {
			t.Fatal(err)
		}
		if err := store.SavePrompts(validRecoveryPrompts()); err != nil {
			t.Fatal(err)
		}
		snapshotBefore, err := os.ReadFile(store.Path())
		if err != nil {
			t.Fatal(err)
		}
		promptsBefore, err := os.ReadFile(store.PromptPath())
		if err != nil {
			t.Fatal(err)
		}
		before := recoveryNames(t, store.Dir())
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 0 || len(result.RetentionFailures) != 0 {
			t.Fatalf("result=%+v; want no recovery or retention failures", result)
		}
		if got, _ := os.ReadFile(store.Path()); !bytes.Equal(got, snapshotBefore) {
			t.Fatal("valid snapshot bytes changed")
		}
		if got, _ := os.ReadFile(store.PromptPath()); !bytes.Equal(got, promptsBefore) {
			t.Fatal("valid prompts bytes changed")
		}
		if after := recoveryNames(t, store.Dir()); !equalRecoveryNames(before, after) {
			t.Fatalf("directory entries changed: before=%v after=%v", before, after)
		}
	})

	t.Run("both paths absent", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		before := recoveryNames(t, store.Dir())
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 0 || !equalRecoveryNames(before, recoveryNames(t, store.Dir())) {
			t.Fatalf("absent paths changed state: result=%+v", result)
		}
	})

	t.Run("valid snapshot through symlink", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		if err := store.Save(validRecoverySnapshot()); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(store.Dir(), "snapshot-target.json")
		if err := os.Rename(store.Path(), target); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Base(target), store.Path()); err != nil {
			t.Fatal(err)
		}
		linkBefore, err := os.Readlink(store.Path())
		if err != nil {
			t.Fatal(err)
		}
		before := recoveryNames(t, store.Dir())
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 0 || len(result.RetentionFailures) != 0 {
			t.Fatalf("result=%+v; want no mutation", result)
		}
		if got, err := os.Readlink(store.Path()); err != nil || got != linkBefore {
			t.Fatalf("snapshot link changed: target=%q err=%v", got, err)
		}
		if _, err := store.Load(); err != nil {
			t.Fatalf("Load through valid symlink: %v", err)
		}
		if after := recoveryNames(t, store.Dir()); !equalRecoveryNames(before, after) {
			t.Fatalf("directory entries changed: before=%v after=%v", before, after)
		}
	})

	t.Run("dangling symlink", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		if err := os.Symlink("missing-target", store.Path()); err != nil {
			t.Fatal(err)
		}
		linkBefore, err := os.Readlink(store.Path())
		if err != nil {
			t.Fatal(err)
		}
		before := recoveryNames(t, store.Dir())
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 0 || len(result.RetentionFailures) != 0 {
			t.Fatalf("result=%+v; want dangling link untouched", result)
		}
		if got, err := os.Readlink(store.Path()); err != nil || got != linkBefore {
			t.Fatalf("dangling link changed: target=%q err=%v", got, err)
		}
		if after := recoveryNames(t, store.Dir()); !equalRecoveryNames(before, after) {
			t.Fatalf("directory entries changed: before=%v after=%v", before, after)
		}
	})
}

func equalRecoveryNames(left, right map[string]bool) bool {
	if len(left) != len(right) {
		return false
	}
	for name := range left {
		if !right[name] {
			return false
		}
	}
	return true
}

func TestRecoverRejectedReasons(t *testing.T) {
	fixed := time.Date(2026, 11, 21, 12, 0, 0, 123456789, time.UTC)
	setRecoveryClock(t, fixed)

	catalogValid := func(t *testing.T) []byte {
		store := openRecoveryTestStore(t)
		return corruptSnapshotBytes(t, store)
	}
	validPrompts := func() *PromptSnapshot { return validRecoveryPrompts() }

	tests := []struct {
		name        string
		family      string
		seed        func(*testing.T, *Store) []byte
		readFailure bool
		wantReason  string
	}{
		{name: "catalog invalid JSON", family: "snapshot", seed: func(*testing.T, *Store) []byte { return []byte("{") }, wantReason: "corrupt_json"},
		{name: "catalog trailing data", family: "snapshot", seed: func(t *testing.T, store *Store) []byte { return append(catalogValid(t), []byte(" trailing")...) }, wantReason: "corrupt_json"},
		{name: "catalog schema two checksum", family: "snapshot", seed: func(t *testing.T, store *Store) []byte { return catalogValid(t) }, wantReason: "checksum_mismatch"},
		{name: "catalog unknown schema", family: "snapshot", seed: func(*testing.T, *Store) []byte { return []byte(`{"schema_version":999}`) }, wantReason: "unsupported_schema"},
		{name: "catalog validation", family: "snapshot", seed: func(t *testing.T, store *Store) []byte {
			raw, err := json.Marshal(validRecoverySnapshot())
			if err != nil {
				t.Fatal(err)
			}
			var snapshot Snapshot
			if err := json.Unmarshal(raw, &snapshot); err != nil {
				t.Fatal(err)
			}
			snapshot.Envelope.ConfigPublicFingerprint = ""
			checksum, err := snapshot.Checksum()
			if err != nil {
				t.Fatal(err)
			}
			snapshot.Envelope.SnapshotChecksum = checksum
			seed, err := json.Marshal(&snapshot)
			if err != nil {
				t.Fatal(err)
			}
			return seed
		}, wantReason: "validation_failure"},
		{name: "catalog schema one decode or checksum", family: "snapshot", seed: func(*testing.T, *Store) []byte { return []byte(`{"schema_version":1}`) }, wantReason: "corrupt_json"},
		{name: "catalog schema one trailing data", family: "snapshot", seed: func(*testing.T, *Store) []byte { return []byte(`{"schema_version":1} trailing`) }, wantReason: "corrupt_json"},
		{name: "catalog read error", family: "snapshot", seed: func(*testing.T, *Store) []byte { return []byte("preserved catalog bytes") }, readFailure: true, wantReason: "read_error"},
		{name: "prompts invalid JSON", family: "prompts", seed: func(*testing.T, *Store) []byte { return []byte("{") }, wantReason: "corrupt_json"},
		{name: "prompts trailing data", family: "prompts", seed: func(t *testing.T, _ *Store) []byte {
			raw, _ := json.Marshal(validPrompts())
			return append(raw, []byte(" trailing")...)
		}, wantReason: "corrupt_json"},
		{name: "prompts validation", family: "prompts", seed: func(t *testing.T, _ *Store) []byte {
			prompts := validPrompts()
			prompts.Prompts = append(prompts.Prompts, prompts.Prompts[0])
			return marshalRecoveryPrompts(t, prompts)
		}, wantReason: "validation_failure"},
		{name: "prompts checksum", family: "prompts", seed: func(t *testing.T, _ *Store) []byte {
			prompts := validPrompts()
			prompts.Checksum = strings.Repeat("0", 64)
			raw, err := json.Marshal(prompts)
			if err != nil {
				t.Fatal(err)
			}
			return raw
		}, wantReason: "checksum_mismatch"},
		{name: "prompts foreign schema validates as validation failure", family: "prompts", seed: func(*testing.T, *Store) []byte { return []byte(`{"schema_version":2,"prompts":[],"checksum":"x"}`) }, wantReason: "validation_failure"},
		{name: "prompts read error", family: "prompts", seed: func(*testing.T, *Store) []byte { return []byte("preserved prompt bytes") }, readFailure: true, wantReason: "read_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openRecoveryTestStore(t)
			path := store.Path()
			if tt.family == "prompts" {
				path = store.PromptPath()
			}
			seed := tt.seed(t, store)
			if err := os.WriteFile(path, seed, 0o600); err != nil {
				t.Fatal(err)
			}
			if tt.readFailure {
				previous := readFile
				readFile = func(string) ([]byte, error) { return nil, errors.New("injected read failure") }
				t.Cleanup(func() { readFile = previous })
			}
			result, err := store.RecoverRejected()
			if err != nil {
				t.Fatalf("RecoverRejected: %v", err)
			}
			if len(result.Recoveries) != 1 {
				t.Fatalf("recoveries=%+v; want exactly one", result.Recoveries)
			}
			recovery := result.Recoveries[0]
			if string(recovery.Family) != tt.family || string(recovery.Reason) != tt.wantReason {
				t.Fatalf("recovery=%+v; want family=%s reason=%s", recovery, tt.family, tt.wantReason)
			}
			if !filepath.IsAbs(recovery.OriginalPath) || !filepath.IsAbs(recovery.QuarantinePath) {
				t.Fatalf("recovery paths must be absolute: %+v", recovery)
			}
			if got, err := os.ReadFile(recovery.QuarantinePath); err != nil || !bytes.Equal(got, seed) {
				t.Fatalf("quarantine bytes=%q err=%v; want original %q", got, err, seed)
			}
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("original path remains: lstat error=%v", err)
			}
		})
	}
}

func marshalRecoveryPrompts(t *testing.T, prompts *PromptSnapshot) []byte {
	t.Helper()
	checksum, err := prompts.ChecksumValue()
	if err != nil {
		t.Fatal(err)
	}
	prompts.Checksum = checksum
	raw, err := json.Marshal(prompts)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestRecoverPromptsFamily(t *testing.T) {
	store := openRecoveryTestStore(t)
	if err := store.Save(validRecoverySnapshot()); err != nil {
		t.Fatal(err)
	}
	snapshotBefore, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	seed := []byte(`{"schema_version":1,"prompts":[`)
	if err := os.WriteFile(store.PromptPath(), seed, 0o600); err != nil {
		t.Fatal(err)
	}
	setRecoveryClock(t, time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC))
	result, err := store.RecoverRejected()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recoveries) != 1 || string(result.Recoveries[0].Family) != "prompts" {
		t.Fatalf("recoveries=%+v; want exactly one prompts recovery", result.Recoveries)
	}
	if got, err := os.ReadFile(result.Recoveries[0].QuarantinePath); err != nil || !bytes.Equal(got, seed) {
		t.Fatalf("quarantined prompts=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(store.Path()); err != nil || !bytes.Equal(got, snapshotBefore) {
		t.Fatalf("snapshot changed: bytes=%q err=%v", got, err)
	}
}

func TestRecoverRetentionBoundTieBreakAndForeignNames(t *testing.T) {
	fixed := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	t.Run("bound and foreign names", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		for age := 1; age <= 5; age++ {
			seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("snapshot-%d", age)))
			seedRecoveryQuarantine(t, store.Dir(), "prompts", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("prompts-%d", age)))
		}
		nonconforming := filepath.Join(store.Dir(), "snapshot.corrupt-not-a-timestamp.json")
		if err := os.WriteFile(nonconforming, []byte("foreign syntax"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(store.Path(), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 1 || len(result.RetentionFailures) != 0 {
			t.Fatalf("result=%+v", result)
		}
		names := recoveryNames(t, store.Dir())
		for age := 1; age <= 4; age++ {
			name := fmt.Sprintf("snapshot.corrupt-%s.json", fixed.Add(-time.Duration(age)*time.Second).Format(time.RFC3339Nano))
			if !names[name] {
				t.Errorf("newer snapshot quarantine missing: %s", name)
			}
		}
		oldest := fmt.Sprintf("snapshot.corrupt-%s.json", fixed.Add(-5*time.Second).Format(time.RFC3339Nano))
		if names[oldest] {
			t.Errorf("oldest snapshot quarantine survived: %s", oldest)
		}
		if !names[filepath.Base(nonconforming)] {
			t.Errorf("nonconforming name was removed")
		}
		for age := 1; age <= 5; age++ {
			foreign := fmt.Sprintf("prompts.corrupt-%s.json", fixed.Add(-time.Duration(age)*time.Second).Format(time.RFC3339Nano))
			if !names[foreign] {
				t.Errorf("foreign family entry changed: %s", foreign)
			}
		}
		runName := filepath.Base(result.Recoveries[0].QuarantinePath)
		if !names[runName] {
			t.Errorf("this run's quarantine missing: %s", runName)
		}
		wantSnapshot := map[string]bool{filepath.Base(nonconforming): true, runName: true}
		for age := 1; age <= 4; age++ {
			wantSnapshot[fmt.Sprintf("snapshot.corrupt-%s.json", fixed.Add(-time.Duration(age)*time.Second).Format(time.RFC3339Nano))] = true
		}
		for name := range names {
			if strings.HasPrefix(name, "snapshot.corrupt-") && !wantSnapshot[name] {
				t.Errorf("unexpected snapshot-family quarantine survived: %s", name)
			}
		}

		for _, backlog := range []int{6, 9} {
			t.Run(fmt.Sprintf("%d-file backlog evicts every excess", backlog), func(t *testing.T) {
				store := openRecoveryTestStore(t)
				setRecoveryClock(t, fixed)
				for age := 1; age <= backlog; age++ {
					seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("snapshot-%d", age)))
				}
				if err := os.WriteFile(store.Path(), []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
				result, err := store.RecoverRejected()
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Recoveries) != 1 || len(result.RetentionFailures) != 0 {
					t.Fatalf("result=%+v", result)
				}
				names := recoveryNames(t, store.Dir())
				for age := 1; age <= 4; age++ {
					name := fmt.Sprintf("snapshot.corrupt-%s.json", fixed.Add(-time.Duration(age)*time.Second).Format(time.RFC3339Nano))
					if !names[name] {
						t.Errorf("retained candidate missing: %s", name)
					}
				}
				for age := 5; age <= backlog; age++ {
					name := fmt.Sprintf("snapshot.corrupt-%s.json", fixed.Add(-time.Duration(age)*time.Second).Format(time.RFC3339Nano))
					if names[name] {
						t.Errorf("excess candidate survived: %s", name)
					}
				}
				if !names[filepath.Base(result.Recoveries[0].QuarantinePath)] {
					t.Errorf("this run's quarantine missing")
				}
			})
		}
	})

	t.Run("equal timestamps use full-name ascending tie-break", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		tied := fixed.Add(-5 * time.Second)
		first := "snapshot.corrupt-" + tied.Format("2006-01-02T15:04:05.000000000Z") + ".json"
		second := "snapshot.corrupt-" + tied.Format("2006-01-02T15:04:05Z") + ".json"
		for _, name := range []string{first, second} {
			if err := os.WriteFile(filepath.Join(store.Dir(), name), []byte(name), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		for age := 1; age <= 3; age++ {
			seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte("newer"))
		}
		if err := os.WriteFile(store.Path(), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.RecoverRejected(); err != nil {
			t.Fatal(err)
		}
		names := recoveryNames(t, store.Dir())
		if names[first] || !names[second] {
			t.Fatalf("tie retention first=%t second=%t; want ascending full name evicted", names[first], names[second])
		}
	})
}

func TestRecoverRetentionProtectsSameAndCrossFamilyTargets(t *testing.T) {
	fixed := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)

	t.Run("same family target", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		var oldest string
		for age := 1; age <= 5; age++ {
			path := seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("target-%d", age)))
			if age == 5 {
				oldest = path
			}
		}
		if err := os.Symlink(filepath.Base(oldest), store.Path()); err != nil {
			t.Fatal(err)
		}
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 1 {
			t.Fatalf("recoveries=%+v", result.Recoveries)
		}
		quarantined, err := os.ReadFile(result.Recoveries[0].QuarantinePath)
		if err != nil || !bytes.Equal(quarantined, []byte("target-5")) {
			t.Fatalf("quarantine link read-through=%q err=%v", quarantined, err)
		}
		if got, err := os.ReadFile(oldest); err != nil || !bytes.Equal(got, []byte("target-5")) {
			t.Fatalf("protected target=%q err=%v", got, err)
		}
		if _, err := os.Stat(filepath.Join(store.Dir(), fmt.Sprintf("snapshot.corrupt-%s.json", fixed.Add(-4*time.Second).Format(time.RFC3339Nano)))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("oldest unprotected candidate was not evicted: %v", err)
		}
	})

	t.Run("cross family target protected before either pass", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		var oldest string
		for age := 1; age <= 5; age++ {
			path := seedRecoveryQuarantine(t, store.Dir(), "prompts", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("prompt-target-%d", age)))
			if age == 5 {
				oldest = path
			}
		}
		if err := os.Symlink(filepath.Base(oldest), store.Path()); err != nil {
			t.Fatal(err)
		}
		promptSeed := []byte("{")
		if err := os.WriteFile(store.PromptPath(), promptSeed, 0o600); err != nil {
			t.Fatal(err)
		}
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 2 {
			t.Fatalf("recoveries=%+v; want both families", result.Recoveries)
		}
		var snapshotQuarantine string
		for _, recovery := range result.Recoveries {
			if string(recovery.Family) == "snapshot" {
				snapshotQuarantine = recovery.QuarantinePath
			}
		}
		if snapshotQuarantine == "" {
			t.Fatalf("missing snapshot recovery: %+v", result.Recoveries)
		}
		if got, err := os.ReadFile(snapshotQuarantine); err != nil || !bytes.Equal(got, []byte("prompt-target-5")) {
			t.Fatalf("quarantine link read-through=%q err=%v", got, err)
		}
		if got, err := os.ReadFile(oldest); err != nil || !bytes.Equal(got, []byte("prompt-target-5")) {
			t.Fatalf("protected prompt target=%q err=%v", got, err)
		}
		if _, err := os.Stat(filepath.Join(store.Dir(), fmt.Sprintf("prompts.corrupt-%s.json", fixed.Add(-4*time.Second).Format(time.RFC3339Nano)))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("prompt retention did not evict next-oldest candidate: %v", err)
		}
	})
}

func TestRecoverRetentionAliasAndCrossFamilyOrdering(t *testing.T) {
	fixed := time.Date(2026, 11, 22, 5, 0, 0, 0, time.UTC)

	t.Run("directory alias target identity is protected", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		var protected, nextOldest string
		for age := 1; age <= 5; age++ {
			path := seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("target-%d", age)))
			if age == 5 {
				protected = path
			}
			if age == 4 {
				nextOldest = path
			}
		}
		realDirectory := filepath.Join(store.Dir(), "real")
		if err := os.Mkdir(realDirectory, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Link(protected, filepath.Join(realDirectory, "target")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("real", filepath.Join(store.Dir(), "aliasdir")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("aliasdir/target", store.Path()); err != nil {
			t.Fatal(err)
		}
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 1 {
			t.Fatalf("recoveries=%+v", result.Recoveries)
		}
		if got, err := os.ReadFile(result.Recoveries[0].QuarantinePath); err != nil || !bytes.Equal(got, []byte("target-5")) {
			t.Fatalf("alias quarantine bytes=%q err=%v", got, err)
		}
		if got, err := os.ReadFile(protected); err != nil || !bytes.Equal(got, []byte("target-5")) {
			t.Fatalf("alias-protected target=%q err=%v", got, err)
		}
		if _, err := os.Stat(nextOldest); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("next-oldest unprotected candidate remains: %v", err)
		}
	})

	t.Run("unconfigured symlink into protected target is not overprotected", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		var protected, nextOldest string
		for age := 1; age <= 5; age++ {
			path := seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("target-%d", age)))
			if age == 5 {
				protected = path
			}
			if age == 4 {
				nextOldest = path
			}
		}
		oldestAlias := filepath.Join(store.Dir(), "snapshot.corrupt-"+fixed.Add(-6*time.Second).Format(time.RFC3339Nano)+".json")
		if err := os.Symlink(filepath.Base(protected), oldestAlias); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Base(protected), store.Path()); err != nil {
			t.Fatal(err)
		}
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 1 {
			t.Fatalf("recoveries=%+v", result.Recoveries)
		}
		if _, err := os.Lstat(oldestAlias); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unprotected alias survived: %v", err)
		}
		if _, err := os.Stat(nextOldest); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("second excess candidate survived: %v", err)
		}
		if got, err := os.ReadFile(protected); err != nil || !bytes.Equal(got, []byte("target-5")) {
			t.Fatalf("protected target=%q err=%v", got, err)
		}
		if got, err := os.ReadFile(result.Recoveries[0].QuarantinePath); err != nil || !bytes.Equal(got, []byte("target-5")) {
			t.Fatalf("quarantine link bytes=%q err=%v", got, err)
		}
	})

	t.Run("prompt link protects snapshot target in first pass", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		var protected, nextOldest string
		for age := 1; age <= 5; age++ {
			path := seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("target-%d", age)))
			if age == 5 {
				protected = path
			}
			if age == 4 {
				nextOldest = path
			}
		}
		if err := os.WriteFile(store.Path(), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Base(protected), store.PromptPath()); err != nil {
			t.Fatal(err)
		}
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 2 || result.Recoveries[0].Family != FamilySnapshot || result.Recoveries[1].Family != FamilyPrompts {
			t.Fatalf("recoveries=%+v; want snapshot then prompts", result.Recoveries)
		}
		if got, err := os.ReadFile(result.Recoveries[1].QuarantinePath); err != nil || !bytes.Equal(got, []byte("target-5")) {
			t.Fatalf("prompts quarantine bytes=%q err=%v", got, err)
		}
		if got, err := os.ReadFile(protected); err != nil || !bytes.Equal(got, []byte("target-5")) {
			t.Fatalf("first-pass protected target=%q err=%v", got, err)
		}
		if _, err := os.Stat(nextOldest); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("first-pass next-oldest candidate remains: %v", err)
		}
	})
}

func TestRecoverSyncFailureIsFatalAfterQuarantine(t *testing.T) {
	store := openRecoveryTestStore(t)
	fixed := time.Date(2026, 11, 22, 6, 0, 0, 0, time.UTC)
	setRecoveryClock(t, fixed)
	original := []byte("{")
	if err := os.WriteFile(store.Path(), original, 0o600); err != nil {
		t.Fatal(err)
	}
	syncErr := errors.New("directory sync failed")
	oldSync := syncDir
	syncDir = func(string) error { return syncErr }
	t.Cleanup(func() { syncDir = oldSync })

	result, err := store.RecoverRejected()
	if !errors.Is(err, ErrSnapshotUnrecoverable) || !errors.Is(err, syncErr) {
		t.Fatalf("error=%v; want fatal directory-sync cause", err)
	}
	if len(result.Recoveries) != 1 {
		t.Fatalf("recoveries=%+v; want completed quarantine", result.Recoveries)
	}
	if got, err := os.ReadFile(result.Recoveries[0].QuarantinePath); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("post-rename quarantine=%q err=%v", got, err)
	}
	if _, err := os.Lstat(store.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("original still exists after successful rename: %v", err)
	}
}

func TestRecoverRetentionFailuresAreNonfatalAndSingleAttempt(t *testing.T) {
	fixed := time.Date(2026, 11, 22, 7, 0, 0, 0, time.UTC)
	t.Run("listing failure", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		original := []byte("{")
		if err := os.WriteFile(store.Path(), original, 0o600); err != nil {
			t.Fatal(err)
		}
		listErr := errors.New("listing failed")
		oldReadDir := readDir
		readDir = func(string) ([]os.DirEntry, error) { return nil, listErr }
		t.Cleanup(func() { readDir = oldReadDir })

		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatalf("retention listing failure became fatal: %v", err)
		}
		if len(result.Recoveries) != 1 || len(result.RetentionFailures) != 1 {
			t.Fatalf("result=%+v", result)
		}
		failure := result.RetentionFailures[0]
		if failure.Family != FamilySnapshot || failure.QuarantinePath != result.Recoveries[0].QuarantinePath {
			t.Fatalf("retention failure=%+v", failure)
		}
		if got, err := os.ReadFile(failure.QuarantinePath); err != nil || !bytes.Equal(got, original) {
			t.Fatalf("quarantine after listing failure=%q err=%v", got, err)
		}
	})

	t.Run("unlink failure gets one oldest-candidate attempt", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		var oldest string
		for age := 1; age <= 5; age++ {
			path := seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("target-%d", age)))
			if age == 5 {
				oldest = path
			}
		}
		original := []byte("{")
		if err := os.WriteFile(store.Path(), original, 0o600); err != nil {
			t.Fatal(err)
		}
		removeErr := errors.New("unlink failed")
		oldRemove := remove
		var attempts []string
		remove = func(path string) error {
			attempts = append(attempts, path)
			return removeErr
		}
		t.Cleanup(func() { remove = oldRemove })

		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatalf("retention unlink failure became fatal: %v", err)
		}
		if len(attempts) != 1 || attempts[0] != oldest {
			t.Fatalf("unlink attempts=%v; want one attempt for %s", attempts, oldest)
		}
		if len(result.RetentionFailures) != 1 || result.RetentionFailures[0] != (RetentionFailure{Family: FamilySnapshot, QuarantinePath: oldest}) {
			t.Fatalf("retention failures=%+v", result.RetentionFailures)
		}
		if got, err := os.ReadFile(oldest); err != nil || !bytes.Equal(got, []byte("target-5")) {
			t.Fatalf("failed unlink changed candidate=%q err=%v", got, err)
		}
		if got, err := os.ReadFile(result.Recoveries[0].QuarantinePath); err != nil || !bytes.Equal(got, original) {
			t.Fatalf("quarantine after unlink failure=%q err=%v", got, err)
		}
	})
}

func TestRecoverNonRegularPathsAreFatalWithoutMutation(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*testing.T, *Store)
	}{
		{name: "directory", setup: func(t *testing.T, store *Store) {
			if err := os.Mkdir(store.Path(), 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "symlink to directory", setup: func(t *testing.T, store *Store) {
			if err := os.Mkdir(filepath.Join(store.Dir(), "target"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("target", store.Path()); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "fifo", setup: func(t *testing.T, store *Store) {
			if err := syscall.Mkfifo(store.Path(), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "symlink to fifo", setup: func(t *testing.T, store *Store) {
			if err := syscall.Mkfifo(filepath.Join(store.Dir(), "target"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("target", store.Path()); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			store := openRecoveryTestStore(t)
			test.setup(t, store)
			var calls []string
			oldRead, oldReserve, oldRename := readFile, reserve, rename
			readFile = func(string) ([]byte, error) {
				calls = append(calls, "read")
				return nil, errors.New("unexpected read")
			}
			reserve = func(string) error {
				calls = append(calls, "reserve")
				return errors.New("unexpected reservation")
			}
			rename = func(string, string) error {
				calls = append(calls, "rename")
				return errors.New("unexpected rename")
			}
			t.Cleanup(func() {
				readFile, reserve, rename = oldRead, oldReserve, oldRename
			})

			result, err := store.RecoverRejected()
			if !errors.Is(err, ErrSnapshotUnrecoverable) {
				t.Fatalf("error=%v; want ErrSnapshotUnrecoverable", err)
			}
			var modeErr *recoveryModeError
			if !errors.As(err, &modeErr) {
				t.Fatalf("error %T does not retain typed mode cause", err)
			}
			if len(calls) != 0 {
				t.Fatalf("non-regular path caused operations %v", calls)
			}
			if len(result.Recoveries) != 0 {
				t.Fatalf("non-regular path was quarantined: %+v", result.Recoveries)
			}
			if _, err := os.Lstat(store.Path()); err != nil {
				t.Fatalf("configured path was mutated: %v", err)
			}
		})
	}
}

func TestRecoverReservationCollisionAdvancesCandidate(t *testing.T) {
	store := openRecoveryTestStore(t)
	fixed := time.Date(2026, 11, 22, 1, 2, 3, 4, time.UTC)
	setRecoveryClock(t, fixed)
	first := filepath.Join(store.Dir(), "snapshot.corrupt-"+fixed.Format(time.RFC3339Nano)+".json")
	if err := os.WriteFile(first, []byte("preexisting"), 0o600); err != nil {
		t.Fatal(err)
	}
	original := []byte("{")
	if err := os.WriteFile(store.Path(), original, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := store.RecoverRejected()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recoveries) != 1 {
		t.Fatalf("recoveries=%+v", result.Recoveries)
	}
	expected := filepath.Join(store.Dir(), "snapshot.corrupt-"+fixed.Add(time.Nanosecond).Format(time.RFC3339Nano)+".json")
	if result.Recoveries[0].QuarantinePath != expected {
		t.Fatalf("quarantine path=%q, want %q", result.Recoveries[0].QuarantinePath, expected)
	}
	if got, err := os.ReadFile(first); err != nil || !bytes.Equal(got, []byte("preexisting")) {
		t.Fatalf("colliding candidate changed to %q, err=%v", got, err)
	}
	if got, err := os.ReadFile(expected); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("reserved candidate bytes=%q, err=%v", got, err)
	}
}

func TestRecoverReservationFailuresAreFatal(t *testing.T) {
	t.Run("non-existence error", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		original := []byte("{")
		if err := os.WriteFile(store.Path(), original, 0o600); err != nil {
			t.Fatal(err)
		}
		reserveErr := errors.New("reservation failed")
		oldReserve := reserve
		reserve = func(string) error { return reserveErr }
		t.Cleanup(func() { reserve = oldReserve })
		result, err := store.RecoverRejected()
		if !errors.Is(err, ErrSnapshotUnrecoverable) || !errors.Is(err, reserveErr) {
			t.Fatalf("error=%v; want fatal reservation cause", err)
		}
		if len(result.Recoveries) != 0 {
			t.Fatalf("failed reservation produced recovery: %+v", result.Recoveries)
		}
		if got, err := os.ReadFile(store.Path()); err != nil || !bytes.Equal(got, original) {
			t.Fatalf("original after failed reservation=%q, err=%v", got, err)
		}
	})

	t.Run("candidate exhaustion", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		if err := os.WriteFile(store.Path(), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		attempts := 0
		oldReserve := reserve
		reserve = func(string) error {
			attempts++
			return os.ErrExist
		}
		t.Cleanup(func() { reserve = oldReserve })
		result, err := store.RecoverRejected()
		if !errors.Is(err, ErrSnapshotUnrecoverable) {
			t.Fatalf("error=%v; want ErrSnapshotUnrecoverable", err)
		}
		if attempts != 4096 {
			t.Fatalf("reservation attempts=%d; want 4096", attempts)
		}
		if len(result.Recoveries) != 0 {
			t.Fatalf("exhaustion produced recovery: %+v", result.Recoveries)
		}
	})

	t.Run("rename failure removes reservation", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		original := []byte("{")
		if err := os.WriteFile(store.Path(), original, 0o600); err != nil {
			t.Fatal(err)
		}
		renameErr := errors.New("rename failed")
		oldRename := rename
		rename = func(string, string) error { return renameErr }
		t.Cleanup(func() { rename = oldRename })
		result, err := store.RecoverRejected()
		if !errors.Is(err, ErrSnapshotUnrecoverable) || !errors.Is(err, renameErr) {
			t.Fatalf("error=%v; want fatal rename cause", err)
		}
		if len(result.Recoveries) != 0 {
			t.Fatalf("failed rename produced recovery: %+v", result.Recoveries)
		}
		if got, err := os.ReadFile(store.Path()); err != nil || !bytes.Equal(got, original) {
			t.Fatalf("original after failed rename=%q, err=%v", got, err)
		}
		names := recoveryNames(t, store.Dir())
		for name := range names {
			if strings.HasPrefix(name, "snapshot.corrupt-") {
				t.Fatalf("failed rename left reservation %q", name)
			}
		}
	})
}

func TestRecoverOrphanedLinkReconciliation(t *testing.T) {
	t.Run("ordinary link to rejected target", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		if err := store.Save(validRecoverySnapshot()); err != nil {
			t.Fatal(err)
		}
		snapshotBytes, err := os.ReadFile(store.Path())
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(store.Path(), store.PromptPath()); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(PromptSnapshotFile, store.Path()); err != nil {
			t.Fatal(err)
		}
		setRecoveryClock(t, time.Date(2026, 11, 22, 2, 0, 0, 0, time.UTC))
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 2 {
			t.Fatalf("recoveries=%+v; want target plus orphaned link", result.Recoveries)
		}
		if result.Recoveries[0].Family != FamilyPrompts || result.Recoveries[0].Reason != ReasonCorruptJSON {
			t.Fatalf("target recovery=%+v", result.Recoveries[0])
		}
		if result.Recoveries[1].Family != FamilySnapshot || result.Recoveries[1].Reason != ReasonOrphanedLink {
			t.Fatalf("link recovery=%+v", result.Recoveries[1])
		}
		if got, err := os.ReadFile(result.Recoveries[0].QuarantinePath); err != nil || !bytes.Equal(got, snapshotBytes) {
			t.Fatalf("target bytes=%d err=%v", len(got), err)
		}
		if got, err := os.Readlink(result.Recoveries[1].QuarantinePath); err != nil || got != PromptSnapshotFile {
			t.Fatalf("link target=%q err=%v", got, err)
		}
	})

	t.Run("mutual loop partner", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		if err := os.Symlink(PromptSnapshotFile, store.Path()); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(SnapshotFile, store.PromptPath()); err != nil {
			t.Fatal(err)
		}
		setRecoveryClock(t, time.Date(2026, 11, 22, 3, 0, 0, 0, time.UTC))
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 2 {
			t.Fatalf("recoveries=%+v; want loop entry plus partner", result.Recoveries)
		}
		if result.Recoveries[0].Family != FamilySnapshot || result.Recoveries[0].Reason != ReasonReadError {
			t.Fatalf("first loop recovery=%+v", result.Recoveries[0])
		}
		if result.Recoveries[1].Family != FamilyPrompts || result.Recoveries[1].Reason != ReasonOrphanedLink {
			t.Fatalf("partner recovery=%+v", result.Recoveries[1])
		}
		if got, err := os.Readlink(result.Recoveries[1].QuarantinePath); err != nil || got != SnapshotFile {
			t.Fatalf("partner target=%q err=%v", got, err)
		}
	})

	t.Run("self loop moves only in its own family pass", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		if err := os.Symlink(SnapshotFile, store.Path()); err != nil {
			t.Fatal(err)
		}
		setRecoveryClock(t, time.Date(2026, 11, 22, 3, 1, 0, 0, time.UTC))
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 1 || result.Recoveries[0].Family != FamilySnapshot || result.Recoveries[0].Reason != ReasonReadError {
			t.Fatalf("self-loop recoveries=%+v; want one pass-produced read_error", result.Recoveries)
		}
	})

	t.Run("both self loops move without reconciliation", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		if err := os.Symlink(SnapshotFile, store.Path()); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(PromptSnapshotFile, store.PromptPath()); err != nil {
			t.Fatal(err)
		}
		setRecoveryClock(t, time.Date(2026, 11, 22, 3, 2, 0, 0, time.UTC))
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 2 || result.Recoveries[0].Reason != ReasonReadError || result.Recoveries[1].Reason != ReasonReadError {
			t.Fatalf("both-loop recoveries=%+v; want two pass-produced read_error entries", result.Recoveries)
		}
	})

	t.Run("hard-linked loop partner is matched by path and identity", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		if err := os.Symlink(SnapshotFile, store.Path()); err != nil {
			t.Fatal(err)
		}
		if err := os.Link(store.Path(), store.PromptPath()); err != nil {
			t.Fatal(err)
		}
		snapshotInfo, err := os.Lstat(store.Path())
		if err != nil {
			t.Fatal(err)
		}
		promptsInfo, err := os.Lstat(store.PromptPath())
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(snapshotInfo, promptsInfo) {
			t.Fatal("test setup did not create hard-linked symlink entries")
		}
		setRecoveryClock(t, time.Date(2026, 11, 22, 3, 3, 0, 0, time.UTC))
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 2 || result.Recoveries[0].Family != FamilySnapshot || result.Recoveries[0].Reason != ReasonReadError || result.Recoveries[1].Family != FamilyPrompts || result.Recoveries[1].Reason != ReasonOrphanedLink {
			t.Fatalf("hard-linked loop recoveries=%+v; want snapshot read_error then prompts orphaned_link", result.Recoveries)
		}
		if got, err := os.Readlink(result.Recoveries[1].QuarantinePath); err != nil || got != SnapshotFile {
			t.Fatalf("hard-linked partner target=%q err=%v", got, err)
		}
	})

	t.Run("prompt link to moved configured snapshot is reconciled", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		original := []byte("{")
		if err := os.WriteFile(store.Path(), original, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(SnapshotFile, store.PromptPath()); err != nil {
			t.Fatal(err)
		}
		setRecoveryClock(t, time.Date(2026, 11, 22, 3, 4, 0, 0, time.UTC))
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 2 || result.Recoveries[0].Family != FamilySnapshot || result.Recoveries[0].Reason != ReasonCorruptJSON || result.Recoveries[1].Family != FamilyPrompts || result.Recoveries[1].Reason != ReasonOrphanedLink {
			t.Fatalf("configured-link recoveries=%+v; want snapshot corrupt_json then prompts orphaned_link", result.Recoveries)
		}
		if got, err := os.ReadFile(result.Recoveries[0].QuarantinePath); err != nil || !bytes.Equal(got, original) {
			t.Fatalf("snapshot quarantine=%q err=%v", got, err)
		}
		if got, err := os.Readlink(result.Recoveries[1].QuarantinePath); err != nil || got != SnapshotFile {
			t.Fatalf("prompts link target=%q err=%v", got, err)
		}
		if _, err := os.Lstat(store.PromptPath()); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("original prompts link remains: %v", err)
		}
	})
}

func TestRecoverComponentWiseChainProtectsRetentionTarget(t *testing.T) {
	store := openRecoveryTestStore(t)
	fixed := time.Date(2026, 11, 22, 4, 0, 0, 0, time.UTC)
	setRecoveryClock(t, fixed)
	var protected, nextOldest string
	for age := 1; age <= 5; age++ {
		path := seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("target-%d", age)))
		if age == 5 {
			protected = path
		}
		if age == 4 {
			nextOldest = path
		}
	}
	if err := os.Mkdir(filepath.Join(store.Dir(), "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(store.Dir(), "deep"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../deep", filepath.Join(store.Dir(), "sub", "alias")); err != nil {
		t.Fatal(err)
	}
	target := "sub/alias/../" + filepath.Base(protected)
	if err := os.Symlink(target, store.Path()); err != nil {
		t.Fatal(err)
	}
	result, err := store.RecoverRejected()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recoveries) != 1 {
		t.Fatalf("recoveries=%+v", result.Recoveries)
	}
	if got, err := os.ReadFile(protected); err != nil || !bytes.Equal(got, []byte("target-5")) {
		t.Fatalf("component-resolved target=%q err=%v", got, err)
	}
	if _, err := os.Stat(nextOldest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("next-oldest unprotected candidate remains: %v", err)
	}
}

func TestRecoverFiniteSymlinkRevisitProtectsTarget(t *testing.T) {
	store := openRecoveryTestStore(t)
	fixed := time.Date(2026, 11, 22, 4, 45, 0, 0, time.UTC)
	setRecoveryClock(t, fixed)
	var protected, nextOldest string
	for age := 1; age <= 5; age++ {
		path := seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("target-%d", age)))
		if age == 5 {
			protected = path
		}
		if age == 4 {
			nextOldest = path
		}
	}
	if err := os.Mkdir(filepath.Join(store.Dir(), "deep"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("deep", filepath.Join(store.Dir(), "alias")); err != nil {
		t.Fatal(err)
	}
	target := "alias/../alias/../" + filepath.Base(protected)
	if err := os.Symlink(target, store.Path()); err != nil {
		t.Fatal(err)
	}
	result, err := store.RecoverRejected()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recoveries) != 1 {
		t.Fatalf("recoveries=%+v", result.Recoveries)
	}
	if got, err := os.ReadFile(result.Recoveries[0].QuarantinePath); err != nil || !bytes.Equal(got, []byte("target-5")) {
		t.Fatalf("finite revisit quarantine=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(protected); err != nil || !bytes.Equal(got, []byte("target-5")) {
		t.Fatalf("finite revisit target=%q err=%v", got, err)
	}
	if _, err := os.Stat(nextOldest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("next-oldest unprotected candidate remains: %v", err)
	}
}

func TestRecoverHardlinkedSymlinkAliasProtectsTarget(t *testing.T) {
	store := openRecoveryTestStore(t)
	fixed := time.Date(2026, 11, 22, 4, 30, 0, 0, time.UTC)
	setRecoveryClock(t, fixed)
	var protected, nextOldest string
	for age := 1; age <= 5; age++ {
		path := seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("target-%d", age)))
		if age == 5 {
			protected = path
		}
		if age == 4 {
			nextOldest = path
		}
	}
	directory := filepath.Join(store.Dir(), "d")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(directory, "d"), 0o700); err != nil {
		t.Fatal(err)
	}
	firstLink := filepath.Join(store.Dir(), "a")
	if err := os.Symlink("d/l", firstLink); err != nil {
		t.Fatal(err)
	}
	secondLink := filepath.Join(directory, "l")
	if err := os.Link(firstLink, secondLink); err != nil {
		t.Fatal(err)
	}
	firstInfo, err := os.Lstat(firstLink)
	if err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Lstat(secondLink)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(firstInfo, secondInfo) {
		t.Fatal("test setup did not hard-link symlink entries")
	}
	if err := os.Symlink("../../"+filepath.Base(protected), filepath.Join(directory, "d", "l")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a", store.Path()); err != nil {
		t.Fatal(err)
	}

	result, err := store.RecoverRejected()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recoveries) != 1 {
		t.Fatalf("recoveries=%+v", result.Recoveries)
	}
	if got, err := os.ReadFile(result.Recoveries[0].QuarantinePath); err != nil || !bytes.Equal(got, []byte("target-5")) {
		t.Fatalf("hard-linked alias quarantine=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(protected); err != nil || !bytes.Equal(got, []byte("target-5")) {
		t.Fatalf("hard-linked alias target=%q err=%v", got, err)
	}
	if _, err := os.Stat(nextOldest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("next-oldest unprotected candidate remains: %v", err)
	}
}

func TestRecoverConfiguredChainExpansionBoundProtectsOnlyVisitedEntries(t *testing.T) {
	store := openRecoveryTestStore(t)
	fixed := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
	setRecoveryClock(t, fixed)
	var seeds [6]string
	for age := 1; age <= 5; age++ {
		seeds[age] = seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("seed-%d", age)))
	}

	const symlinkCount = 4097
	for index := range symlinkCount {
		name := fmt.Sprintf("link-%04d", index)
		target := fmt.Sprintf("link-%04d", index+1)
		if index == symlinkCount-1 {
			target = filepath.Base(seeds[5])
		}
		if err := os.Symlink(target, filepath.Join(store.Dir(), name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("link-0000", store.Path()); err != nil {
		t.Fatal(err)
	}
	protected := newRecoveryProtection()
	record := resolveConfiguredChain(store.Dir(), store.Path(), protected)
	if record.termination != chainBudget && record.termination != chainDepth {
		t.Fatalf("expansion bound termination=%d; want budget/depth, not loop", record.termination)
	}

	result, err := store.RecoverRejected()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recoveries) != 1 || result.Recoveries[0].Reason != ReasonReadError {
		t.Fatalf("recoveries=%+v; want one read-error recovery", result.Recoveries)
	}
	if _, err := os.Stat(seeds[5]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unreached oldest seed survived; chain may have completed: %v", err)
	}
	for age := 1; age <= 4; age++ {
		if _, err := os.Stat(seeds[age]); err != nil {
			t.Errorf("younger seed %s was unexpectedly evicted: %v", seeds[age], err)
		}
	}
	for index := range symlinkCount {
		path := filepath.Join(store.Dir(), fmt.Sprintf("link-%04d", index))
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("resolver link %s was mutated: %v", path, err)
			break
		}
	}
}
func TestRecoverDoesNotRetainWithoutCreatingQuarantine(t *testing.T) {
	store := openRecoveryTestStore(t)
	fixed := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	setRecoveryClock(t, fixed)
	for age := 1; age <= 7; age++ {
		seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte("preserve"))
	}
	if err := store.Save(validRecoverySnapshot()); err != nil {
		t.Fatal(err)
	}
	before := recoveryNames(t, store.Dir())
	result, err := store.RecoverRejected()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recoveries) != 0 || len(result.RetentionFailures) != 0 {
		t.Fatalf("result=%+v; want no quarantine and no retention", result)
	}
	if after := recoveryNames(t, store.Dir()); !equalRecoveryNames(before, after) {
		t.Fatalf("zero-recovery path ran retention: before=%v after=%v", before, after)
	}
}
func TestRecoverPromptsRetentionBoundAndFamilyIsolation(t *testing.T) {
	fixed := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	store := openRecoveryTestStore(t)
	setRecoveryClock(t, fixed)

	seeded := make(map[string][]byte)
	for age := 1; age <= 5; age++ {
		for _, family := range []string{"prompts", "snapshot"} {
			content := []byte(fmt.Sprintf("%s-%d", family, age))
			path := seedRecoveryQuarantine(t, store.Dir(), family, fixed.Add(-time.Duration(age)*time.Second), content)
			seeded[filepath.Base(path)] = content
		}
	}
	for name, content := range map[string][]byte{
		"prompts.corrupt-not-a-timestamp.json": []byte("nonconforming"),
		"prompts.corrupt-2026-11-21T12:00:00+02:00.json": []byte("offset"),
	} {
		if err := os.WriteFile(filepath.Join(store.Dir(), name), content, 0o600); err != nil {
			t.Fatal(err)
		}
		seeded[name] = content
	}
	original := []byte("{")
	if err := os.WriteFile(store.PromptPath(), original, 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := store.RecoverRejected()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recoveries) != 1 || result.Recoveries[0].Family != FamilyPrompts || len(result.RetentionFailures) != 0 {
		t.Fatalf("result=%+v; want one prompt recovery and no retention failure", result)
	}
	if got, err := os.ReadFile(result.Recoveries[0].QuarantinePath); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("prompt quarantine=%q err=%v", got, err)
	}

	names := recoveryNames(t, store.Dir())
	runName := filepath.Base(result.Recoveries[0].QuarantinePath)
	if !names[runName] {
		t.Fatalf("this run's prompt quarantine is missing: %s", runName)
	}
	for age := 1; age <= 4; age++ {
		name := fmt.Sprintf("prompts.corrupt-%s.json", fixed.Add(-time.Duration(age)*time.Second).Format(time.RFC3339Nano))
		if !names[name] {
			t.Errorf("newer prompt candidate missing: %s", name)
		}
	}
	oldest := fmt.Sprintf("prompts.corrupt-%s.json", fixed.Add(-5*time.Second).Format(time.RFC3339Nano))
	if names[oldest] {
		t.Errorf("oldest prompt candidate survived: %s", oldest)
	}

	conformingPrompts := 0
	for name := range names {
		if _, ok := quarantineTimestamp(FamilyPrompts, name); ok {
			conformingPrompts++
		}
	}
	if conformingPrompts != QuarantineKeep {
		t.Errorf("conforming prompt files=%d; want %d", conformingPrompts, QuarantineKeep)
	}
	for name, want := range seeded {
		if name == oldest {
			continue
		}
		got, err := os.ReadFile(filepath.Join(store.Dir(), name))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("seed %s changed: got=%q err=%v want=%q", name, got, err, want)
		}
	}
}

func TestRecoverRetentionFailuresForBothFamilies(t *testing.T) {
	fixed := time.Date(2026, 11, 23, 13, 0, 0, 0, time.UTC)
	store := openRecoveryTestStore(t)
	setRecoveryClock(t, fixed)

	oldest := make(map[RecoveryFamily]string)
	seeded := make(map[string][]byte)
	for age := 1; age <= 5; age++ {
		for _, family := range []RecoveryFamily{FamilySnapshot, FamilyPrompts} {
			content := []byte(fmt.Sprintf("%s-%d", family, age))
			path := seedRecoveryQuarantine(t, store.Dir(), string(family), fixed.Add(-time.Duration(age)*time.Second), content)
			seeded[path] = content
			if age == 5 {
				oldest[family] = path
			}
		}
	}
	snapshotBytes, promptsBytes := []byte("{"), []byte("[")
	if err := os.WriteFile(store.Path(), snapshotBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.PromptPath(), promptsBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	removeErr := errors.New("injected unlink failure")
	oldRemove := remove
	var attempts []string
	remove = func(path string) error {
		attempts = append(attempts, path)
		return removeErr
	}
	t.Cleanup(func() { remove = oldRemove })

	result, err := store.RecoverRejected()
	if err != nil {
		t.Fatalf("retention failures became fatal: %v", err)
	}
	if len(result.Recoveries) != 2 || len(result.RetentionFailures) != 2 {
		t.Fatalf("result=%+v; want one recovery and failure per family", result)
	}
	wantAttempts := []string{oldest[FamilySnapshot], oldest[FamilyPrompts]}
	if !reflect.DeepEqual(attempts, wantAttempts) {
		t.Fatalf("unlink attempts=%v; want exactly %v", attempts, wantAttempts)
	}
	wantFailures := map[RecoveryFamily]string{
		FamilySnapshot: oldest[FamilySnapshot],
		FamilyPrompts:  oldest[FamilyPrompts],
	}
	for _, failure := range result.RetentionFailures {
		if want, ok := wantFailures[failure.Family]; !ok || failure.QuarantinePath != want {
			t.Errorf("unexpected retention failure: %+v", failure)
		}
		delete(wantFailures, failure.Family)
	}
	if len(wantFailures) != 0 {
		t.Errorf("missing family retention failures: %v", wantFailures)
	}
	for path, want := range seeded {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("seed %s changed: got=%q err=%v want=%q", path, got, err, want)
		}
	}
	for _, recovery := range result.Recoveries {
		want := snapshotBytes
		if recovery.Family == FamilyPrompts {
			want = promptsBytes
		}
		got, err := os.ReadFile(recovery.QuarantinePath)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s quarantine changed: got=%q err=%v want=%q", recovery.Family, got, err, want)
		}
	}
}

func TestRecoverRetentionFutureDatedNamesStayBounded(t *testing.T) {
	fixed := time.Date(2026, 11, 23, 14, 0, 0, 0, time.UTC)
	store := openRecoveryTestStore(t)
	setRecoveryClock(t, fixed)
	for age := 1; age <= 5; age++ {
		seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(time.Duration(age)*time.Nanosecond), []byte(fmt.Sprintf("future-%d", age)))
	}
	original := []byte("{")
	if err := os.WriteFile(store.Path(), original, 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := store.RecoverRejected()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recoveries) != 1 || len(result.RetentionFailures) != 0 {
		t.Fatalf("result=%+v; want one recovery and no retention failure", result)
	}
	runPath := result.Recoveries[0].QuarantinePath
	if got, err := os.ReadFile(runPath); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("this run's quarantine=%q err=%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(store.Dir(), "snapshot.corrupt-"+fixed.Add(time.Nanosecond).Format(time.RFC3339Nano)+".json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("oldest future-dated candidate was not evicted: %v", err)
	}
	for age := 2; age <= 5; age++ {
		path := filepath.Join(store.Dir(), "snapshot.corrupt-"+fixed.Add(time.Duration(age)*time.Nanosecond).Format(time.RFC3339Nano)+".json")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("newer future-dated candidate missing: %s: %v", path, err)
		}
	}
	names := recoveryNames(t, store.Dir())
	conforming := 0
	for name := range names {
		if _, ok := quarantineTimestamp(FamilySnapshot, name); ok {
			conforming++
		}
	}
	if conforming != QuarantineKeep {
		t.Errorf("conforming snapshot files=%d; want %d", conforming, QuarantineKeep)
	}
}
func TestRecoverRetentionListingFailurePreservesBacklog(t *testing.T) {
	fixed := time.Date(2026, 11, 23, 15, 0, 0, 0, time.UTC)
	store := openRecoveryTestStore(t)
	setRecoveryClock(t, fixed)
	seeded := make(map[string][]byte)
	for age := 1; age <= 5; age++ {
		content := []byte(fmt.Sprintf("snapshot-%d", age))
		path := seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), content)
		seeded[path] = content
	}
	original := []byte("{")
	if err := os.WriteFile(store.Path(), original, 0o600); err != nil {
		t.Fatal(err)
	}
	listErr := errors.New("injected listing failure")
	oldReadDir, oldRemove := readDir, remove
	listingAttempts, unlinkAttempts := 0, 0
	readDir = func(string) ([]os.DirEntry, error) {
		listingAttempts++
		return nil, listErr
	}
	remove = func(string) error {
		unlinkAttempts++
		return errors.New("unlink must not run after listing failure")
	}
	t.Cleanup(func() {
		readDir, remove = oldReadDir, oldRemove
	})

	result, err := store.RecoverRejected()
	if err != nil {
		t.Fatalf("listing failure became fatal: %v", err)
	}
	if listingAttempts != 1 || unlinkAttempts != 0 {
		t.Fatalf("listing attempts=%d unlink attempts=%d; want 1 and 0", listingAttempts, unlinkAttempts)
	}
	if len(result.Recoveries) != 1 || len(result.RetentionFailures) != 1 {
		t.Fatalf("result=%+v; want one recovery and one retention failure", result)
	}
	if got, want := result.RetentionFailures[0], (RetentionFailure{Family: FamilySnapshot, QuarantinePath: result.Recoveries[0].QuarantinePath}); got != want {
		t.Fatalf("retention failure=%+v; want %+v", got, want)
	}
	seeded[result.Recoveries[0].QuarantinePath] = original
	for path, want := range seeded {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("file %s changed: got=%q err=%v want=%q", path, got, err, want)
		}
	}
	conforming := 0
	for name := range recoveryNames(t, store.Dir()) {
		if _, ok := quarantineTimestamp(FamilySnapshot, name); ok {
			conforming++
		}
	}
	if conforming != 6 {
		t.Errorf("retained conforming snapshot files=%d; want six after listing failure", conforming)
	}
}

func TestRecoverRetentionPromptTieBreakUsesName(t *testing.T) {
	fixed := time.Date(2026, 11, 23, 16, 0, 0, 0, time.UTC)
	store := openRecoveryTestStore(t)
	setRecoveryClock(t, fixed)
	tie := fixed.Add(-5 * time.Second)
	first := "prompts.corrupt-" + tie.Format("2006-01-02T15:04:05.000000000Z") + ".json"
	second := "prompts.corrupt-" + tie.Format("2006-01-02T15:04:05Z") + ".json"
	for _, name := range []string{first, second} {
		if err := os.WriteFile(filepath.Join(store.Dir(), name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for age := 2; age <= 4; age++ {
		seedRecoveryQuarantine(t, store.Dir(), "prompts", fixed.Add(-time.Duration(age)*time.Second), []byte("newer"))
	}
	if err := os.WriteFile(store.PromptPath(), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := store.RecoverRejected()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recoveries) != 1 || result.Recoveries[0].Family != FamilyPrompts {
		t.Fatalf("recoveries=%+v; want one prompts recovery", result.Recoveries)
	}
	names := recoveryNames(t, store.Dir())
	if names[first] || !names[second] {
		t.Fatalf("tied prompt names first=%t second=%t; want full-name ascending candidate evicted", names[first], names[second])
	}
}

func TestRecoverRetentionProtectsTwoHopBackingChain(t *testing.T) {
	fixed := time.Date(2026, 11, 23, 17, 0, 0, 0, time.UTC)
	store := openRecoveryTestStore(t)
	setRecoveryClock(t, fixed)
	paths := make([]string, 6)
	for age := 1; age <= 5; age++ {
		content := []byte(fmt.Sprintf("seed-%d", age))
		if age == 5 {
			content = []byte("{")
		}
		paths[age] = seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), content)
	}
	if err := os.Remove(paths[1]); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(paths[2]), paths[1]); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(paths[2]); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(paths[5]), paths[2]); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(paths[1]), store.Path()); err != nil {
		t.Fatal(err)
	}

	result, err := store.RecoverRejected()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recoveries) != 1 || result.Recoveries[0].Reason != ReasonCorruptJSON {
		t.Fatalf("recoveries=%+v; want one corrupt-json link recovery", result.Recoveries)
	}
	for _, path := range []string{paths[1], paths[2], paths[5]} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("backing chain entry %s was removed: %v", path, err)
		}
	}
	if _, err := os.Stat(paths[4]); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("oldest unprotected candidate was not removed: %v", err)
	}
	if got, err := os.ReadFile(paths[5]); err != nil || !bytes.Equal(got, []byte("{")) {
		t.Errorf("backing target=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(result.Recoveries[0].QuarantinePath); err != nil || !bytes.Equal(got, []byte("{")) {
		t.Errorf("quarantine read-through=%q err=%v", got, err)
	}
}

func TestRecoverRetentionProtectsOldestELOOPChainEntries(t *testing.T) {
	fixed := time.Date(2026, 11, 23, 18, 0, 0, 0, time.UTC)
	store := openRecoveryTestStore(t)
	setRecoveryClock(t, fixed)
	paths := make([]string, 6)
	for age := 1; age <= 5; age++ {
		paths[age] = seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("seed-%d", age)))
	}
	for _, path := range []string{paths[4], paths[5]} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Base(paths[5]), paths[4]); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(paths[4]), paths[5]); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(paths[4]), store.Path()); err != nil {
		t.Fatal(err)
	}

	result, err := store.RecoverRejected()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recoveries) != 1 || result.Recoveries[0].Reason != ReasonReadError {
		t.Fatalf("recoveries=%+v; want one read-error recovery", result.Recoveries)
	}
	for _, path := range []string{paths[4], paths[5]} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("cycle entry %s was removed: %v", path, err)
		}
	}
	if _, err := os.Stat(paths[3]); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("oldest unprotected candidate q3 was not evicted: %v", err)
	}
	if _, err := os.Lstat(paths[2]); err != nil {
		t.Errorf("unexpected younger candidate eviction: %v", err)
	}
}

func TestRecoverRetentionMixedProtectionDeletesAllUnprotected(t *testing.T) {
	fixed := time.Date(2026, 11, 23, 19, 0, 0, 0, time.UTC)
	store := openRecoveryTestStore(t)
	setRecoveryClock(t, fixed)
	paths := make([]string, 7)
	for age := 1; age <= 6; age++ {
		content := []byte(fmt.Sprintf("seed-%d", age))
		if age == 5 {
			content = []byte("{")
		}
		paths[age] = seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), content)
	}
	for age := 1; age <= 4; age++ {
		if err := os.Remove(paths[age]); err != nil {
			t.Fatal(err)
		}
		target := filepath.Base(paths[age+1])
		if err := os.Symlink(target, paths[age]); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Base(paths[1]), store.Path()); err != nil {
		t.Fatal(err)
	}

	result, err := store.RecoverRejected()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recoveries) != 1 || result.Recoveries[0].Reason != ReasonCorruptJSON {
		t.Fatalf("recoveries=%+v; want one corrupt-json recovery", result.Recoveries)
	}
	if _, err := os.Stat(paths[6]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unprotected oldest candidate survived: %v", err)
	}
	for age := 1; age <= 5; age++ {
		if _, err := os.Lstat(paths[age]); err != nil {
			t.Errorf("protected chain entry s%d was removed: %v", age, err)
		}
	}
	if got := len(recoveryNames(t, store.Dir())) - 1; got != 6 {
		t.Errorf("retained quarantine files=%d; want six including the protected carve-out", got)
	}
	if got, err := os.ReadFile(result.Recoveries[0].QuarantinePath); err != nil || !bytes.Equal(got, []byte("{")) {
		t.Errorf("quarantine read-through=%q err=%v", got, err)
	}
}

func TestRecoverRetentionVacuousAndDanglingTargets(t *testing.T) {
	fixed := time.Date(2026, 11, 23, 20, 0, 0, 0, time.UTC)
	t.Run("nonconforming final target does not protect seeds", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		for age := 1; age <= 5; age++ {
			seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("seed-%d", age)))
		}
		keeper := filepath.Join(store.Dir(), "keeper.json")
		if err := os.WriteFile(keeper, []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Base(keeper), store.Path()); err != nil {
			t.Fatal(err)
		}
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 1 || result.Recoveries[0].Reason != ReasonCorruptJSON {
			t.Fatalf("recoveries=%+v; want one corrupt-json recovery", result.Recoveries)
		}
		if _, err := os.Stat(filepath.Join(store.Dir(), "snapshot.corrupt-"+fixed.Add(-5*time.Second).Format(time.RFC3339Nano)+".json")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("vacuously protected oldest seed was not pruned: %v", err)
		}
		if got, err := os.ReadFile(keeper); err != nil || !bytes.Equal(got, []byte("{")) {
			t.Errorf("nonconforming target=%q err=%v", got, err)
		}
	})

	t.Run("dangling configured link has no retention pass", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		for age := 1; age <= 5; age++ {
			seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("seed-%d", age)))
		}
		if err := os.Symlink("missing.json", store.PromptPath()); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(store.Path(), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 1 || result.Recoveries[0].Family != FamilySnapshot || len(result.RetentionFailures) != 0 {
			t.Fatalf("result=%+v; want only snapshot recovery", result)
		}
		if _, err := os.Readlink(store.PromptPath()); err != nil {
			t.Errorf("pre-existing dangling link changed: %v", err)
		}
		oldest := filepath.Join(store.Dir(), "snapshot.corrupt-"+fixed.Add(-5*time.Second).Format(time.RFC3339Nano)+".json")
		if _, err := os.Stat(oldest); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("snapshot retention did not evict oldest seed: %v", err)
		}
	})

	t.Run("two-component traversal escapes without recovery", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		var seeds []string
		for age := 1; age <= 5; age++ {
			seeds = append(seeds, seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("seed-%d", age))))
		}
		for _, directory := range []string{"sub", "deep"} {
			if err := os.Mkdir(filepath.Join(store.Dir(), directory), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Symlink("../deep", filepath.Join(store.Dir(), "sub", "alias")); err != nil {
			t.Fatal(err)
		}
		target := strings.Join([]string{"sub", "alias", "..", "..", filepath.Base(seeds[4])}, "/")
		if err := os.Symlink(target, store.Path()); err != nil {
			t.Fatal(err)
		}
		before := recoveryNames(t, store.Dir())
		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 0 || len(result.RetentionFailures) != 0 {
			t.Fatalf("result=%+v; escaped dangling link must be untouched", result)
		}
		if after := recoveryNames(t, store.Dir()); !equalRecoveryNames(before, after) {
			t.Fatalf("escaped chain changed directory: before=%v after=%v", before, after)
		}
	})
}

func TestRecoverRetentionAlternatingSubdirectoryHops(t *testing.T) {
	fixed := time.Date(2026, 11, 23, 21, 0, 0, 0, time.UTC)
	store := openRecoveryTestStore(t)
	setRecoveryClock(t, fixed)
	var paths [6]string
	for age := 1; age <= 5; age++ {
		content := []byte(fmt.Sprintf("seed-%d", age))
		if age == 5 {
			content = []byte("{")
		}
		paths[age] = seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), content)
	}
	for _, name := range []string{"sub", "alt"} {
		if err := os.Mkdir(filepath.Join(store.Dir(), name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("../alt/l2", filepath.Join(store.Dir(), "sub", "l1")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../"+filepath.Base(paths[5]), filepath.Join(store.Dir(), "alt", "l2")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("sub/l1", store.Path()); err != nil {
		t.Fatal(err)
	}

	result, err := store.RecoverRejected()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recoveries) != 1 || result.Recoveries[0].Reason != ReasonCorruptJSON {
		t.Fatalf("recoveries=%+v; want one corrupt-json recovery", result.Recoveries)
	}
	for _, path := range []string{paths[5], filepath.Join(store.Dir(), "sub", "l1"), filepath.Join(store.Dir(), "alt", "l2")} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("chain element %s was removed: %v", path, err)
		}
	}
	if _, err := os.Stat(paths[4]); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("oldest unprotected candidate was not removed: %v", err)
	}
	if got, err := os.ReadFile(result.Recoveries[0].QuarantinePath); err != nil || !bytes.Equal(got, []byte("{")) {
		t.Errorf("quarantine read-through=%q err=%v", got, err)
	}
}

func TestRecoverRetentionHardlinkedSymlinkExpansionKeys(t *testing.T) {
	fixed := time.Date(2026, 11, 23, 22, 0, 0, 0, time.UTC)
	t.Run("same symlink inode in three containing directories", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		var paths [6]string
		for age := 1; age <= 5; age++ {
			content := []byte(fmt.Sprintf("seed-%d", age))
			if age == 5 {
				content = []byte("{")
			}
			paths[age] = seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), content)
		}
		dirs := []string{"d", filepath.Join("d", "d"), filepath.Join("d", "d", "d")}
		for _, dir := range dirs {
			if err := os.Mkdir(filepath.Join(store.Dir(), dir), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		first := filepath.Join(store.Dir(), "a")
		if err := os.Symlink("d/l", first); err != nil {
			t.Fatal(err)
		}
		for _, dir := range dirs[:2] {
			if err := os.Link(first, filepath.Join(store.Dir(), dir, "l")); err != nil {
				t.Fatal(err)
			}
		}
		last := filepath.Join(store.Dir(), dirs[2], "l")
		if err := os.Symlink("../../../"+filepath.Base(paths[5]), last); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("a", store.Path()); err != nil {
			t.Fatal(err)
		}

		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 1 || result.Recoveries[0].Reason != ReasonCorruptJSON {
			t.Fatalf("recoveries=%+v; want one corrupt-json recovery", result.Recoveries)
		}
		if _, err := os.Stat(paths[4]); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("next-oldest unprotected candidate was not removed: %v", err)
		}
		if got, err := os.ReadFile(paths[5]); err != nil || !bytes.Equal(got, []byte("{")) {
			t.Errorf("backing target=%q err=%v", got, err)
		}
		if got, err := os.ReadFile(result.Recoveries[0].QuarantinePath); err != nil || !bytes.Equal(got, []byte("{")) {
			t.Errorf("quarantine read-through=%q err=%v", got, err)
		}
	})

	t.Run("hard-linked ancestor key remains a true cycle", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		var paths [6]string
		for age := 1; age <= 5; age++ {
			paths[age] = seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("seed-%d", age)))
		}
		x := filepath.Join(store.Dir(), "x")
		y := filepath.Join(store.Dir(), "y")
		x2 := filepath.Join(store.Dir(), "x2")
		if err := os.Symlink("y", x); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("x2", y); err != nil {
			t.Fatal(err)
		}
		if err := os.Link(x, x2); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("x", store.Path()); err != nil {
			t.Fatal(err)
		}

		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 1 || result.Recoveries[0].Reason != ReasonReadError {
			t.Fatalf("recoveries=%+v; want one read-error recovery", result.Recoveries)
		}
		for _, path := range []string{x, y, x2} {
			if _, err := os.Lstat(path); err != nil {
				t.Errorf("cycle link %s was removed: %v", path, err)
			}
		}
		if _, err := os.Stat(paths[5]); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("oldest unprotected candidate was not evicted: %v", err)
		}
	})
}

func TestRecoverRetentionMidPathNotDirectory(t *testing.T) {
	fixed := time.Date(2026, 11, 23, 23, 0, 0, 0, time.UTC)
	store := openRecoveryTestStore(t)
	setRecoveryClock(t, fixed)
	var paths [6]string
	for age := 1; age <= 5; age++ {
		paths[age] = seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("seed-%d", age)))
	}
	keeper := filepath.Join(store.Dir(), "keeper.json")
	if err := os.WriteFile(keeper, []byte("keeper"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("keeper.json/x", store.Path()); err != nil {
		t.Fatal(err)
	}

	result, err := store.RecoverRejected()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Recoveries) != 1 || result.Recoveries[0].Reason != ReasonReadError {
		t.Fatalf("recoveries=%+v; want one read-error recovery", result.Recoveries)
	}
	if got, err := os.ReadFile(keeper); err != nil || !bytes.Equal(got, []byte("keeper")) {
		t.Errorf("mid-path regular entry=%q err=%v", got, err)
	}
	if _, err := os.Stat(paths[5]); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("oldest unprotected candidate was not evicted: %v", err)
	}
}

func TestRecoverRetentionDistinguishesCycleFromFiniteRevisit(t *testing.T) {
	fixed := time.Date(2026, 11, 24, 1, 0, 0, 0, time.UTC)
	t.Run("directory symlink ancestor cycle is harmless", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		var oldest string
		for age := 1; age <= 5; age++ {
			path := seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("seed-%d", age)))
			if age == 5 {
				oldest = path
			}
		}
		sub := filepath.Join(store.Dir(), "sub")
		if err := os.Mkdir(sub, 0o700); err != nil {
			t.Fatal(err)
		}
		alias := filepath.Join(store.Dir(), "alias")
		back := filepath.Join(sub, "back")
		if err := os.Symlink("sub/back", alias); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("../alias", back); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("alias", store.Path()); err != nil {
			t.Fatal(err)
		}

		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 1 || result.Recoveries[0].Reason != ReasonReadError {
			t.Fatalf("recoveries=%+v; want one read-error recovery", result.Recoveries)
		}
		for _, path := range []string{alias, back} {
			if _, err := os.Lstat(path); err != nil {
				t.Errorf("cycle link %s changed: %v", path, err)
			}
		}
		if _, err := os.Stat(oldest); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("oldest unprotected file was not evicted: %v", err)
		}
	})

	t.Run("three-fold finite directory revisit reaches backing target", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		var oldest, nextOldest string
		for age := 1; age <= 5; age++ {
			content := []byte(fmt.Sprintf("seed-%d", age))
			if age == 5 {
				content = []byte("{")
			}
			path := seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), content)
			if age == 5 {
				oldest = path
			}
			if age == 4 {
				nextOldest = path
			}
		}
		if err := os.Mkdir(filepath.Join(store.Dir(), "deep"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("deep", filepath.Join(store.Dir(), "alias")); err != nil {
			t.Fatal(err)
		}
		target := "alias/../alias/../alias/../" + filepath.Base(oldest)
		if err := os.Symlink(target, store.Path()); err != nil {
			t.Fatal(err)
		}

		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 1 || result.Recoveries[0].Reason != ReasonCorruptJSON {
			t.Fatalf("recoveries=%+v; want one corrupt-json recovery", result.Recoveries)
		}
		if _, err := os.Stat(oldest); err != nil {
			t.Errorf("finite revisit backing target was not protected: %v", err)
		}
		if _, err := os.Stat(nextOldest); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("next-oldest unprotected candidate was not removed: %v", err)
		}
		if got, err := os.ReadFile(result.Recoveries[0].QuarantinePath); err != nil || !bytes.Equal(got, []byte("{")) {
			t.Errorf("quarantine read-through=%q err=%v", got, err)
		}
	})
}
func TestRecoverConfiguredLinkReconciliationFixtures(t *testing.T) {
	fixed := time.Date(2026, 11, 24, 2, 0, 0, 0, time.UTC)

	t.Run("b-xA protects a prompts target while both families reject", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		var target, nextOldest string
		for age := 1; age <= 5; age++ {
			path := seedRecoveryQuarantine(t, store.Dir(), "prompts", fixed.Add(-time.Duration(age)*time.Second), []byte("{"))
			if age == 5 {
				target = path
			}
			if age == 4 {
				nextOldest = path
			}
		}
		if err := os.Symlink(filepath.Base(target), store.Path()); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(store.PromptPath(), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}

		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 2 || result.Recoveries[0].Family != FamilySnapshot || result.Recoveries[0].Reason != ReasonCorruptJSON || result.Recoveries[1].Family != FamilyPrompts || result.Recoveries[1].Reason != ReasonCorruptJSON {
			t.Fatalf("recoveries=%+v; want snapshot then prompts corruption", result.Recoveries)
		}
		if got, err := os.Readlink(result.Recoveries[0].QuarantinePath); err != nil || got != filepath.Base(target) {
			t.Fatalf("snapshot quarantine link target=%q err=%v", got, err)
		}
		if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, []byte("{")) {
			t.Fatalf("protected prompts target=%q err=%v", got, err)
		}
		if _, err := os.Stat(nextOldest); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("oldest unprotected prompts seed was not evicted: %v", err)
		}
		if got := countRecoveryFamily(t, store.Dir(), "prompts"); got != QuarantineKeep {
			t.Fatalf("prompts quarantine count=%d; want %d", got, QuarantineKeep)
		}
	})

	t.Run("b-xB protects a snapshot target before the snapshot retention pass", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		var target, nextOldest string
		for age := 1; age <= 5; age++ {
			path := seedRecoveryQuarantine(t, store.Dir(), "snapshot", fixed.Add(-time.Duration(age)*time.Second), []byte("{"))
			if age == 5 {
				target = path
			}
			if age == 4 {
				nextOldest = path
			}
		}
		if err := os.WriteFile(store.Path(), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Base(target), store.PromptPath()); err != nil {
			t.Fatal(err)
		}

		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 2 || result.Recoveries[0].Family != FamilySnapshot || result.Recoveries[0].Reason != ReasonCorruptJSON || result.Recoveries[1].Family != FamilyPrompts || result.Recoveries[1].Reason != ReasonCorruptJSON {
			t.Fatalf("recoveries=%+v; want snapshot then prompts rejection", result.Recoveries)
		}
		if got, err := os.Readlink(result.Recoveries[1].QuarantinePath); err != nil || got != filepath.Base(target) {
			t.Fatalf("prompts quarantine link target=%q err=%v", got, err)
		}
		if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, []byte("{")) {
			t.Fatalf("protected snapshot target=%q err=%v", got, err)
		}
		if _, err := os.Stat(nextOldest); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("first-pass retention did not evict next-oldest seed: %v", err)
		}
	})

	t.Run("b-xA2 does not reconcile a link moved by its own pass", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		if err := os.WriteFile(store.PromptPath(), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(PromptSnapshotFile, store.Path()); err != nil {
			t.Fatal(err)
		}

		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 2 || result.Recoveries[0].Family != FamilySnapshot || result.Recoveries[0].Reason != ReasonCorruptJSON || result.Recoveries[1].Family != FamilyPrompts || result.Recoveries[1].Reason != ReasonCorruptJSON {
			t.Fatalf("recoveries=%+v; want only the two pass-produced corruptions", result.Recoveries)
		}
		if got, err := os.Readlink(result.Recoveries[0].QuarantinePath); err != nil || got != PromptSnapshotFile {
			t.Fatalf("snapshot quarantine target=%q err=%v", got, err)
		}
		for _, path := range []string{store.Path(), store.PromptPath()} {
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("configured path %q remains: %v", path, err)
			}
		}
	})

	t.Run("b-xB2 reconciles a moved snapshot target after both passes", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		if err := os.WriteFile(store.Path(), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(SnapshotFile, store.PromptPath()); err != nil {
			t.Fatal(err)
		}

		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 2 || result.Recoveries[0].Family != FamilySnapshot || result.Recoveries[0].Reason != ReasonCorruptJSON || result.Recoveries[1].Family != FamilyPrompts || result.Recoveries[1].Reason != ReasonOrphanedLink {
			t.Fatalf("recoveries=%+v; want snapshot corruption then prompts orphan", result.Recoveries)
		}
		if got, err := os.Readlink(result.Recoveries[1].QuarantinePath); err != nil || got != SnapshotFile {
			t.Fatalf("reconciled prompts link target=%q err=%v", got, err)
		}
		for _, path := range []string{store.Path(), store.PromptPath()} {
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("configured path %q remains: %v", path, err)
			}
		}
	})

	t.Run("b-xB3 reconciles a snapshot link after the prompts pass", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		if err := store.Save(validRecoverySnapshot()); err != nil {
			t.Fatal(err)
		}
		snapshotBytes, err := os.ReadFile(store.Path())
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(store.Path()); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(store.PromptPath(), snapshotBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(PromptSnapshotFile, store.Path()); err != nil {
			t.Fatal(err)
		}

		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 2 || result.Recoveries[0].Family != FamilyPrompts || result.Recoveries[0].Reason != ReasonCorruptJSON || result.Recoveries[1].Family != FamilySnapshot || result.Recoveries[1].Reason != ReasonOrphanedLink {
			t.Fatalf("recoveries=%+v; want prompts corruption then snapshot orphan", result.Recoveries)
		}
		if got, err := os.Readlink(result.Recoveries[1].QuarantinePath); err != nil || got != PromptSnapshotFile {
			t.Fatalf("reconciled snapshot link target=%q err=%v", got, err)
		}
		if got, err := os.ReadFile(result.Recoveries[0].QuarantinePath); err != nil || !bytes.Equal(got, snapshotBytes) {
			t.Fatalf("prompts quarantine=%d bytes err=%v", len(got), err)
		}
	})

	t.Run("b-xB-res retains the reconciled prompts link within the family bound", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		var oldest string
		for age := 1; age <= 5; age++ {
			path := seedRecoveryQuarantine(t, store.Dir(), "prompts", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("prompt-%d", age)))
			if age == 5 {
				oldest = path
			}
		}
		if err := os.WriteFile(store.Path(), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(SnapshotFile, store.PromptPath()); err != nil {
			t.Fatal(err)
		}

		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 2 || result.Recoveries[0].Family != FamilySnapshot || result.Recoveries[0].Reason != ReasonCorruptJSON || result.Recoveries[1].Family != FamilyPrompts || result.Recoveries[1].Reason != ReasonOrphanedLink {
			t.Fatalf("recoveries=%+v; want snapshot corruption then prompts orphan", result.Recoveries)
		}
		if _, err := os.Stat(oldest); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("oldest prompts seed survived reconciliation retention: %v", err)
		}
		if got := countRecoveryFamily(t, store.Dir(), "prompts"); got != QuarantineKeep {
			t.Fatalf("prompts quarantine count=%d; want %d", got, QuarantineKeep)
		}
	})

	t.Run("b-dangle preserves the non-mutual originally dangling limitation", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		if err := os.WriteFile(store.Path(), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("gone.json", store.PromptPath()); err != nil {
			t.Fatal(err)
		}

		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 1 || result.Recoveries[0].Family != FamilySnapshot || result.Recoveries[0].Reason != ReasonCorruptJSON {
			t.Fatalf("recoveries=%+v; want only the snapshot rejection", result.Recoveries)
		}
		if got, err := os.Readlink(store.PromptPath()); err != nil || got != "gone.json" {
			t.Fatalf("original dangling prompts link target=%q err=%v", got, err)
		}
		if got := countRecoveryFamily(t, store.Dir(), "prompts"); got != 0 {
			t.Fatalf("prompts quarantine count=%d; want 0 for the documented limitation", got)
		}
	})

	t.Run("mutual both-moved loop pair produces no orphan reconciliation", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		if err := os.Symlink(PromptSnapshotFile, store.Path()); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(PromptSnapshotFile, store.PromptPath()); err != nil {
			t.Fatal(err)
		}

		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 2 || result.Recoveries[0].Family != FamilySnapshot || result.Recoveries[0].Reason != ReasonReadError || result.Recoveries[1].Family != FamilyPrompts || result.Recoveries[1].Reason != ReasonReadError {
			t.Fatalf("recoveries=%+v; want two pass-produced read_error records", result.Recoveries)
		}
		for _, path := range []string{store.Path(), store.PromptPath()} {
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("configured path %q remains: %v", path, err)
			}
		}
	})

	t.Run("mutual reconciliation applies prompts retention", func(t *testing.T) {
		store := openRecoveryTestStore(t)
		setRecoveryClock(t, fixed)
		var oldest string
		for age := 1; age <= 5; age++ {
			path := seedRecoveryQuarantine(t, store.Dir(), "prompts", fixed.Add(-time.Duration(age)*time.Second), []byte(fmt.Sprintf("prompt-%d", age)))
			if age == 5 {
				oldest = path
			}
		}
		if err := os.Symlink(PromptSnapshotFile, store.Path()); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(SnapshotFile, store.PromptPath()); err != nil {
			t.Fatal(err)
		}

		result, err := store.RecoverRejected()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Recoveries) != 2 || result.Recoveries[0].Family != FamilySnapshot || result.Recoveries[0].Reason != ReasonReadError || result.Recoveries[1].Family != FamilyPrompts || result.Recoveries[1].Reason != ReasonOrphanedLink {
			t.Fatalf("recoveries=%+v; want snapshot read_error then prompts orphan", result.Recoveries)
		}
		if got, err := os.Readlink(result.Recoveries[1].QuarantinePath); err != nil || got != SnapshotFile {
			t.Fatalf("reconciled mutual link target=%q err=%v", got, err)
		}
		if _, err := os.Stat(oldest); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("oldest prompts seed survived mutual retention: %v", err)
		}
		if got := countRecoveryFamily(t, store.Dir(), "prompts"); got != QuarantineKeep {
			t.Fatalf("prompts quarantine count=%d; want %d", got, QuarantineKeep)
		}
	})
}

func countRecoveryFamily(t *testing.T, directory, family string) int {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	prefix := family + ".corrupt-"
	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), ".json") {
			count++
		}
	}
	return count
}

func recoveryFamilyPath(store *Store, family RecoveryFamily) string {
	if family == FamilySnapshot {
		return store.Path()
	}
	return store.PromptPath()
}

func seedRejectedRecoveryFamily(t *testing.T, store *Store, family RecoveryFamily, original []byte) (string, string, []byte) {
	t.Helper()
	path := recoveryFamilyPath(store, family)
	otherPath := store.Path()
	if family == FamilySnapshot {
		if err := store.SavePrompts(validRecoveryPrompts()); err != nil {
			t.Fatal(err)
		}
		otherPath = store.PromptPath()
	} else if err := store.Save(validRecoverySnapshot()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	otherBytes, err := os.ReadFile(otherPath)
	if err != nil {
		t.Fatal(err)
	}
	return path, otherPath, otherBytes
}

func TestRecoverAC28SeamsBothFamilies(t *testing.T) {
	families := []RecoveryFamily{FamilySnapshot, FamilyPrompts}

	for _, family := range families {
		t.Run(string(family)+"/post-rename-sync", func(t *testing.T) {
			store := openRecoveryTestStore(t)
			setRecoveryClock(t, time.Date(2026, 11, 25, 1, 2, 3, 4, time.UTC))
			original := []byte("rejected " + string(family))
			path, otherPath, otherBytes := seedRejectedRecoveryFamily(t, store, family, original)
			injected := errors.New("injected directory sync failure")
			oldSync := syncDir
			syncDir = func(string) error { return injected }
			t.Cleanup(func() { syncDir = oldSync })

			result, err := store.RecoverRejected()
			if !errors.Is(err, ErrSnapshotUnrecoverable) || !errors.Is(err, injected) {
				t.Fatalf("sync failure=%v; want fatal sentinel and injected cause", err)
			}
			if len(result.Recoveries) != 1 || result.Recoveries[0].Family != family {
				t.Fatalf("sync failure recoveries=%+v; want completed %s quarantine", result.Recoveries, family)
			}
			quarantine := result.Recoveries[0].QuarantinePath
			if !strings.HasPrefix(filepath.Base(quarantine), string(family)+".corrupt-") {
				t.Fatalf("sync failure quarantine=%q; want %s family", quarantine, family)
			}
			if got, err := os.ReadFile(quarantine); err != nil || !bytes.Equal(got, original) {
				t.Fatalf("post-rename sync quarantine=%q err=%v", got, err)
			}
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("original %s path remains after rename: %v", family, err)
			}
			if got, err := os.ReadFile(otherPath); err != nil || !bytes.Equal(got, otherBytes) {
				t.Fatalf("other family changed after %s sync failure: %q err=%v", family, got, err)
			}
		})

		t.Run(string(family)+"/rename", func(t *testing.T) {
			store := openRecoveryTestStore(t)
			original := []byte("rejected " + string(family))
			path, otherPath, otherBytes := seedRejectedRecoveryFamily(t, store, family, original)
			injected := errors.New("injected quarantine rename failure")
			oldRename := rename
			rename = func(string, string) error { return injected }
			t.Cleanup(func() { rename = oldRename })

			result, err := store.RecoverRejected()
			if !errors.Is(err, ErrSnapshotUnrecoverable) || !errors.Is(err, injected) {
				t.Fatalf("rename failure=%v; want fatal sentinel and injected cause", err)
			}
			if len(result.Recoveries) != 0 {
				t.Fatalf("rename failure reported completed quarantine: %+v", result.Recoveries)
			}
			if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, original) {
				t.Fatalf("rename failure changed original %s bytes=%q err=%v", family, got, err)
			}
			if got := countRecoveryFamily(t, store.Dir(), string(family)); got != 0 {
				t.Fatalf("rename failure left %d %s quarantine entries", got, family)
			}
			if got, err := os.ReadFile(otherPath); err != nil || !bytes.Equal(got, otherBytes) {
				t.Fatalf("other family changed after %s rename failure: %q err=%v", family, got, err)
			}
		})

		t.Run(string(family)+"/reservation-exist-advances", func(t *testing.T) {
			store := openRecoveryTestStore(t)
			fixed := time.Date(2026, 11, 25, 2, 3, 4, 5, time.UTC)
			setRecoveryClock(t, fixed)
			original := []byte("rejected " + string(family))
			path, otherPath, otherBytes := seedRejectedRecoveryFamily(t, store, family, original)
			oldReserve := reserve
			attempts := 0
			reserve = func(candidate string) error {
				attempts++
				if attempts == 1 {
					return os.ErrExist
				}
				return reserveQuarantineFile(candidate)
			}
			t.Cleanup(func() { reserve = oldReserve })

			result, err := store.RecoverRejected()
			if err != nil {
				t.Fatalf("expected reservation collision to advance: %v", err)
			}
			if attempts != 2 || len(result.Recoveries) != 1 || result.Recoveries[0].Family != family {
				t.Fatalf("reservation attempts=%d recoveries=%+v", attempts, result.Recoveries)
			}
			if got, err := os.ReadFile(result.Recoveries[0].QuarantinePath); err != nil || !bytes.Equal(got, original) {
				t.Fatalf("reserved %s bytes=%q err=%v", family, got, err)
			}
			if got, err := os.ReadFile(otherPath); err != nil || !bytes.Equal(got, otherBytes) {
				t.Fatalf("other family changed after %s reservation collision: %q err=%v", family, got, err)
			}
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("original %s path remains: %v", family, err)
			}
		})

		t.Run(string(family)+"/reservation-error", func(t *testing.T) {
			store := openRecoveryTestStore(t)
			original := []byte("rejected " + string(family))
			path, otherPath, otherBytes := seedRejectedRecoveryFamily(t, store, family, original)
			injected := errors.New("injected reservation failure")
			oldReserve := reserve
			reserve = func(string) error { return injected }
			t.Cleanup(func() { reserve = oldReserve })

			result, err := store.RecoverRejected()
			if !errors.Is(err, ErrSnapshotUnrecoverable) || !errors.Is(err, injected) {
				t.Fatalf("reservation failure=%v; want fatal sentinel and injected cause", err)
			}
			if len(result.Recoveries) != 0 {
				t.Fatalf("reservation failure reported recovery: %+v", result.Recoveries)
			}
			if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, original) {
				t.Fatalf("reservation failure changed original %s bytes=%q err=%v", family, got, err)
			}
			if got := countRecoveryFamily(t, store.Dir(), string(family)); got != 0 {
				t.Fatalf("reservation failure left %d %s quarantine entries", got, family)
			}
			if got, err := os.ReadFile(otherPath); err != nil || !bytes.Equal(got, otherBytes) {
				t.Fatalf("other family changed after %s reservation failure: %q err=%v", family, got, err)
			}
		})

		t.Run(string(family)+"/reservation-exhaustion", func(t *testing.T) {
			store := openRecoveryTestStore(t)
			fixed := time.Date(2026, 11, 25, 3, 4, 5, 6, time.UTC)
			setRecoveryClock(t, fixed)
			original := []byte("rejected " + string(family))
			path, otherPath, otherBytes := seedRejectedRecoveryFamily(t, store, family, original)
			for i := range 4096 {
				seedRecoveryQuarantine(t, store.Dir(), string(family), fixed.Add(time.Duration(i)), []byte("reserved"))
			}

			result, err := store.RecoverRejected()
			if !errors.Is(err, ErrSnapshotUnrecoverable) {
				t.Fatalf("reservation exhaustion=%v; want ErrSnapshotUnrecoverable", err)
			}
			if len(result.Recoveries) != 0 {
				t.Fatalf("reservation exhaustion reported recovery: %+v", result.Recoveries)
			}
			if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, original) {
				t.Fatalf("reservation exhaustion changed original %s bytes=%q err=%v", family, got, err)
			}
			if got := countRecoveryFamily(t, store.Dir(), string(family)); got != 4096 {
				t.Fatalf("reservation exhaustion changed %s count=%d; want 4096", family, got)
			}
			if got, err := os.ReadFile(otherPath); err != nil || !bytes.Equal(got, otherBytes) {
				t.Fatalf("other family changed after %s reservation exhaustion: %q err=%v", family, got, err)
			}
		})

		t.Run(string(family)+"/non-regular-no-open", func(t *testing.T) {
			store := openRecoveryTestStore(t)
			fifo := recoveryFamilyPath(store, family)
			var otherPath string
			if family == FamilySnapshot {
				if err := store.SavePrompts(validRecoveryPrompts()); err != nil {
					t.Fatal(err)
				}
				otherPath = store.PromptPath()
			} else {
				if err := store.Save(validRecoverySnapshot()); err != nil {
					t.Fatal(err)
				}
				otherPath = store.Path()
			}
			otherBytes, err := os.ReadFile(otherPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := syscall.Mkfifo(fifo, 0o600); err != nil {
				t.Fatal(err)
			}

			type recoveryResult struct {
				result *RecoveryResult
				err    error
			}
			done := make(chan recoveryResult, 1)
			go func() {
				result, err := store.RecoverRejected()
				done <- recoveryResult{result: result, err: err}
			}()
			select {
			case outcome := <-done:
				if !errors.Is(outcome.err, ErrSnapshotUnrecoverable) {
					t.Fatalf("non-regular %s error=%v; want ErrSnapshotUnrecoverable", family, outcome.err)
				}
				var modeErr *recoveryModeError
				if !errors.As(outcome.err, &modeErr) {
					t.Fatalf("non-regular %s error lost typed mode cause: %v", family, outcome.err)
				}
				if len(outcome.result.Recoveries) != 0 || len(outcome.result.RetentionFailures) != 0 {
					t.Fatalf("non-regular %s result=%+v; want no recovery or retention result", family, outcome.result)
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("non-regular %s recovery blocked; FIFO may have been opened", family)
			}
			info, err := os.Lstat(fifo)
			if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
				t.Fatalf("%s FIFO was changed: info=%v err=%v", family, info, err)
			}
			if got := countRecoveryFamily(t, store.Dir(), string(family)); got != 0 {
				t.Fatalf("non-regular %s created %d quarantine entries", family, got)
			}
			if got, err := os.ReadFile(otherPath); err != nil || !bytes.Equal(got, otherBytes) {
				t.Fatalf("other family changed after non-regular %s: %q err=%v", family, got, err)
			}
		})
	}
}

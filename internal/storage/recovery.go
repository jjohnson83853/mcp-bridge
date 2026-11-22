package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

var ErrSnapshotUnrecoverable = errors.New("storage: snapshot state is unrecoverable")

const QuarantineKeep = 5

// RecoveryFamily identifies one independently retained state file family.
type RecoveryFamily string

const (
	FamilySnapshot RecoveryFamily = "snapshot"
	FamilyPrompts  RecoveryFamily = "prompts"
)

// Reason is the finite cause vocabulary for quarantined state.
type Reason string

const (
	ReasonCorruptJSON       Reason = "corrupt_json"
	ReasonChecksumMismatch  Reason = "checksum_mismatch"
	ReasonUnsupportedSchema Reason = "unsupported_schema"
	ReasonValidationFailure Reason = "validation_failure"
	ReasonReadError         Reason = "read_error"
	ReasonOrphanedLink      Reason = "orphaned_link"
)

type Recovery struct {
	Family         RecoveryFamily
	Reason         Reason
	OriginalPath   string
	QuarantinePath string
}

type RetentionFailure struct {
	Family         RecoveryFamily
	QuarantinePath string
}

type RecoveryResult struct {
	Recoveries        []Recovery
	RetentionFailures []RetentionFailure
}

var (
	stat     = os.Stat
	readFile = os.ReadFile
	reserve  = reserveQuarantineFile
	rename   = os.Rename
	remove   = os.Remove
	readDir  = os.ReadDir
	syncDir  = syncDirectory
	now      = time.Now
)

func reserveQuarantineFile(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, fileMode)
	if err != nil {
		return err
	}
	return file.Close()
}

func (s *Store) RecoverRejected() (*RecoveryResult, error) {
	result := &RecoveryResult{}
	if s == nil || s.closed {
		return result, errors.New("storage: recover rejected state: store is closed")
	}
	stateDir, err := filepath.Abs(s.directory)
	if err != nil {
		return result, unrecoverable(fmt.Errorf("storage: resolve state directory: %w", err))
	}
	stateDir = filepath.Clean(stateDir)
	paths := [2]string{
		filepath.Join(stateDir, SnapshotFile),
		filepath.Join(stateDir, PromptSnapshotFile),
	}
	families := [2]RecoveryFamily{FamilySnapshot, FamilyPrompts}

	protected := newRecoveryProtection()
	prepass := [2]chainRecord{
		resolveConfiguredChain(stateDir, paths[0], protected),
		resolveConfiguredChain(stateDir, paths[1], protected),
	}
	moved := make([]movedEntry, 0, 2)
	created := [2]string{}

	for familyIndex, family := range families {
		recovery, entryInfo, rejected, err := s.recoverFamily(stateDir, family, paths[familyIndex])
		if rejected {
			result.Recoveries = append(result.Recoveries, recovery)
			created[familyIndex] = recovery.QuarantinePath
			moved = append(moved, movedEntry{originalPath: recovery.OriginalPath, info: entryInfo})
		}
		if err != nil {
			return result, err
		}
		if rejected {
			retainFamily(stateDir, family, created[familyIndex], protected, result)
		}
	}

	reconciled := false
	for familyIndex, record := range prepass {
		if reconciled || !record.link || !recordOrphaned(record, moved) {
			continue
		}
		path := paths[familyIndex]
		entry, err := os.Lstat(path)
		if err != nil || entry.Mode()&os.ModeSymlink == 0 || isMovedEntry(path, entry, moved) {
			continue
		}
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			continue
		}
		recovery, entryInfo, createdRecovery, err := quarantineEntry(stateDir, families[familyIndex], path, ReasonOrphanedLink)
		if createdRecovery {
			result.Recoveries = append(result.Recoveries, recovery)
			created[familyIndex] = recovery.QuarantinePath
			moved = append(moved, movedEntry{originalPath: recovery.OriginalPath, info: entryInfo})
			reconciled = true
		}
		if err != nil {
			return result, err
		}
		if createdRecovery {
			retainFamily(stateDir, families[familyIndex], created[familyIndex], protected, result)
		}
	}
	return result, nil
}

func (s *Store) recoverFamily(stateDir string, family RecoveryFamily, path string) (Recovery, os.FileInfo, bool, error) {
	info, err := stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Recovery{}, nil, false, nil
		}
		return quarantineEntry(stateDir, family, path, ReasonReadError)
	}
	if info == nil {
		return quarantineEntry(stateDir, family, path, ReasonReadError)
	}
	if info.IsDir() || !info.Mode().IsRegular() {
		return Recovery{}, nil, false, unrecoverable(&recoveryModeError{family: family, mode: info.Mode()})
	}

	raw, err := readFile(path)
	if err != nil {
		return quarantineEntry(stateDir, family, path, ReasonReadError)
	}
	var reason Reason
	switch family {
	case FamilySnapshot:
		_, reason, err = decodeSnapshot(raw)
	case FamilyPrompts:
		_, reason, err = decodePromptSnapshot(raw)
	default:
		return Recovery{}, nil, false, unrecoverable(fmt.Errorf("storage: unknown recovery family %q", family))
	}
	if err == nil {
		return Recovery{}, nil, false, nil
	}
	return quarantineEntry(stateDir, family, path, reason)
}

func quarantineEntry(stateDir string, family RecoveryFamily, path string, reason Reason) (Recovery, os.FileInfo, bool, error) {
	original, err := filepath.Abs(path)
	if err != nil {
		return Recovery{}, nil, false, unrecoverable(fmt.Errorf("storage: resolve rejected path: %w", err))
	}
	original = filepath.Clean(original)
	entryInfo, _ := os.Lstat(original)
	createdAt := now().UTC()
	var quarantinePath string
	reserved := false
	for offset := range 4096 {
		candidateTime := createdAt.Add(time.Duration(offset) * time.Nanosecond)
		name := fmt.Sprintf("%s.corrupt-%s.json", family, candidateTime.Format(time.RFC3339Nano))
		candidate := filepath.Join(stateDir, name)
		if err := reserve(candidate); err != nil {
			if errors.Is(err, fs.ErrExist) {
				continue
			}
			return Recovery{}, nil, false, unrecoverable(fmt.Errorf("storage: reserve quarantine path: %w", err))
		}
		quarantinePath = candidate
		reserved = true
		break
	}
	if !reserved {
		return Recovery{}, nil, false, unrecoverable(fmt.Errorf("storage: quarantine name reservation exhausted after 4096 candidates: %w", fs.ErrExist))
	}
	if err := rename(original, quarantinePath); err != nil {
		cleanupErr := remove(quarantinePath)
		cause := fmt.Errorf("storage: rename rejected state: %w", err)
		if cleanupErr != nil && !errors.Is(cleanupErr, fs.ErrNotExist) {
			cause = errors.Join(cause, fmt.Errorf("storage: remove unused quarantine reservation: %w", cleanupErr))
		}
		return Recovery{}, nil, false, unrecoverable(cause)
	}
	recovery := Recovery{
		Family:         family,
		Reason:         reason,
		OriginalPath:   original,
		QuarantinePath: quarantinePath,
	}
	if err := syncDir(stateDir); err != nil {
		return recovery, entryInfo, true, unrecoverable(fmt.Errorf("storage: sync state directory after quarantine: %w", err))
	}
	return recovery, entryInfo, true, nil
}

func unrecoverable(cause error) error {
	return fmt.Errorf("%w: %w", ErrSnapshotUnrecoverable, cause)
}

type recoveryModeError struct {
	family RecoveryFamily
	mode   os.FileMode
}

func (e *recoveryModeError) Error() string {
	if e.mode.IsDir() {
		return fmt.Sprintf("storage: %s state path is a directory", e.family)
	}
	return fmt.Sprintf("storage: %s state path has non-regular mode %s", e.family, e.mode)
}

type quarantineCandidate struct {
	path      string
	name      string
	timestamp time.Time
	info      os.FileInfo
}

func retainFamily(stateDir string, family RecoveryFamily, createdPath string, protected *recoveryProtection, result *RecoveryResult) {
	if createdPath == "" {
		return
	}
	createdName := filepath.Base(createdPath)
	entries, err := readDir(stateDir)
	if err != nil {
		result.RetentionFailures = append(result.RetentionFailures, RetentionFailure{
			Family:         family,
			QuarantinePath: createdPath,
		})
		return
	}
	candidates := make([]quarantineCandidate, 0, len(entries))
	allPreexisting := 0
	for _, entry := range entries {
		name := entry.Name()
		timestamp, ok := quarantineTimestamp(family, name)
		if !ok || name == createdName {
			continue
		}
		path := filepath.Join(stateDir, name)
		info, err := os.Lstat(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			result.RetentionFailures = append(result.RetentionFailures, RetentionFailure{Family: family, QuarantinePath: path})
			return
		}
		if info.IsDir() || !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		allPreexisting++
		candidates = append(candidates, quarantineCandidate{path: path, name: name, timestamp: timestamp, info: info})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].timestamp.Equal(candidates[j].timestamp) {
			return candidates[i].name < candidates[j].name
		}
		return candidates[i].timestamp.Before(candidates[j].timestamp)
	})
	toDelete := allPreexisting - (QuarantineKeep - 1)
	if toDelete <= 0 {
		return
	}
	deleted := 0
	for _, candidate := range candidates {
		if deleted >= toDelete {
			break
		}
		if protected.contains(candidate.path, candidate.info) {
			continue
		}
		if err := remove(candidate.path); err != nil {
			result.RetentionFailures = append(result.RetentionFailures, RetentionFailure{Family: family, QuarantinePath: candidate.path})
			return
		}
		deleted++
	}
}

func quarantineTimestamp(family RecoveryFamily, name string) (time.Time, bool) {
	prefix := string(family) + ".corrupt-"
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".json") {
		return time.Time{}, false
	}
	timestampText := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".json")
	if !strings.HasSuffix(timestampText, "Z") {
		return time.Time{}, false
	}
	timestamp, err := time.Parse(time.RFC3339Nano, timestampText)
	if err != nil || timestamp.Location() != time.UTC {
		return time.Time{}, false
	}
	return timestamp, true
}

type recoveryProtection struct {
	identities []os.FileInfo
	paths      map[string]struct{}
}

func newRecoveryProtection() *recoveryProtection {
	return &recoveryProtection{paths: make(map[string]struct{})}
}

func (p *recoveryProtection) add(path string, info os.FileInfo) {
	clean, err := filepath.Abs(path)
	if err == nil {
		p.paths[filepath.Clean(clean)] = struct{}{}
	}
	if info != nil {
		for _, existing := range p.identities {
			if os.SameFile(existing, info) {
				return
			}
		}
		p.identities = append(p.identities, info)
	}
}

func (p *recoveryProtection) contains(path string, info os.FileInfo) bool {
	clean, err := filepath.Abs(path)
	if err == nil {
		if _, ok := p.paths[filepath.Clean(clean)]; ok {
			return true
		}
	}
	if info != nil {
		for _, identity := range p.identities {
			if os.SameFile(identity, info) {
				return true
			}
		}
	}
	return false
}

type chainEnd uint8

const (
	chainUnknown chainEnd = iota
	chainRegular
	chainDangling
	chainEscape
	chainDirectory
	chainLoop
	chainStatError
	chainOther
	chainNotDirectory
	chainBudget
	chainDepth
)

type chainEntry struct {
	path string
	info os.FileInfo
}

type chainRecord struct {
	path         string
	link         bool
	entries      []chainEntry
	termination  chainEnd
	finalRegular bool
}

func resolveConfiguredChain(stateDir, configuredPath string, protected *recoveryProtection) chainRecord {
	path := filepath.Clean(configuredPath)
	record := chainRecord{path: path}
	info, err := os.Lstat(path)
	if err != nil {
		protected.add(path, nil)
		if errors.Is(err, fs.ErrNotExist) {
			record.termination = chainDangling
		} else {
			record.termination = chainStatError
		}
		return record
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return record
	}
	record.link = true
	addChainEntry(&record, protected, path, info)
	target, err := os.Readlink(path)
	if err != nil {
		record.termination = chainStatError
		return record
	}
	resolved := resolveLinkTarget(stateDir, path, target, &record, protected)
	record.termination = resolved.termination
	record.finalRegular = resolved.termination == chainRegular
	return record
}

func addChainEntry(record *chainRecord, protected *recoveryProtection, path string, info os.FileInfo) {
	clean := filepath.Clean(path)
	protected.add(clean, info)
	for _, existing := range record.entries {
		if existing.path == clean && sameFileInfo(existing.info, info) {
			return
		}
	}
	record.entries = append(record.entries, chainEntry{path: clean, info: info})
}

func sameFileInfo(left, right os.FileInfo) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return os.SameFile(left, right)
}

type resolvedTarget struct {
	path        string
	info        os.FileInfo
	termination chainEnd
}

type resolutionFrame struct {
	current     string
	currentInfo os.FileInfo
	components  []string
	next        int
	key         *expansionKey
}

type expansionKey struct {
	entry         os.FileInfo
	directory     os.FileInfo
	path          string
	directoryPath string
}

func resolveLinkTarget(stateDir, linkPath, target string, record *chainRecord, protected *recoveryProtection) resolvedTarget {
	base := filepath.Dir(linkPath)
	if filepath.IsAbs(target) {
		base = string(filepath.Separator)
	}
	base = filepath.Clean(base)
	baseInfo, err := os.Lstat(base)
	if err != nil {
		protected.add(base, nil)
		return resolvedTarget{path: base, termination: chainStatError}
	}
	frames := []resolutionFrame{{
		current:     base,
		currentInfo: baseInfo,
		components:  strings.Split(filepath.ToSlash(target), "/"),
	}}
	active := make([]expansionKey, 0, 8)
	splices := 0

	finish := func(frame resolutionFrame) resolvedTarget {
		addChainEntry(record, protected, frame.current, frame.currentInfo)
		if frame.currentInfo == nil {
			info, err := os.Lstat(frame.current)
			if err != nil {
				protected.add(frame.current, nil)
				return resolvedTarget{path: frame.current, termination: chainStatError}
			}
			frame.currentInfo = info
			addChainEntry(record, protected, frame.current, info)
		}
		if frame.currentInfo.IsDir() {
			return resolvedTarget{path: frame.current, info: frame.currentInfo, termination: chainDirectory}
		}
		if frame.currentInfo.Mode().IsRegular() {
			return resolvedTarget{path: frame.current, info: frame.currentInfo, termination: chainRegular}
		}
		return resolvedTarget{path: frame.current, info: frame.currentInfo, termination: chainOther}
	}

	for len(frames) != 0 {
		index := len(frames) - 1
		frame := &frames[index]
		if frame.next >= len(frame.components) {
			completed := *frame
			frames = frames[:index]
			resolved := finish(completed)
			if completed.key != nil {
				active = active[:len(active)-1]
			}
			if len(frames) == 0 {
				return resolved
			}
			if resolved.termination != chainDirectory && frames[len(frames)-1].next < len(frames[len(frames)-1].components) {
				return resolvedTarget{path: resolved.path, info: resolved.info, termination: chainNotDirectory}
			}
			parent := &frames[len(frames)-1]
			parent.current = resolved.path
			parent.currentInfo = resolved.info
			continue
		}

		component := frame.components[frame.next]
		frame.next++
		if component == "" || component == "." {
			continue
		}
		if component == ".." {
			parent := filepath.Dir(frame.current)
			if parent == frame.current {
				frame.current = parent
				frame.currentInfo, _ = os.Lstat(parent)
				continue
			}
			if pathWithin(stateDir, frame.current) && !pathWithin(stateDir, parent) {
				protected.add(parent, nil)
				return resolvedTarget{path: parent, termination: chainEscape}
			}
			frame.current = parent
			frame.currentInfo, err = os.Lstat(parent)
			if err != nil {
				protected.add(parent, nil)
				return resolvedTarget{path: parent, termination: chainStatError}
			}
			addChainEntry(record, protected, parent, frame.currentInfo)
			continue
		}

		candidate := filepath.Clean(filepath.Join(frame.current, filepath.FromSlash(component)))
		info, err := os.Lstat(candidate)
		if err != nil {
			protected.add(candidate, nil)
			if errors.Is(err, syscall.ENOTDIR) {
				return resolvedTarget{path: candidate, termination: chainNotDirectory}
			}
			if errors.Is(err, fs.ErrNotExist) {
				return resolvedTarget{path: candidate, termination: chainDangling}
			}
			return resolvedTarget{path: candidate, termination: chainStatError}
		}
		addChainEntry(record, protected, candidate, info)
		frame.current = candidate
		frame.currentInfo = info
		if info.Mode()&os.ModeSymlink == 0 {
			if !info.IsDir() && frame.next < len(frame.components) {
				return resolvedTarget{path: candidate, info: info, termination: chainNotDirectory}
			}
			continue
		}

		targetText, err := os.Readlink(candidate)
		if err != nil {
			return resolvedTarget{path: candidate, info: info, termination: chainStatError}
		}
		directoryPath := filepath.Dir(candidate)
		directoryInfo, _ := os.Lstat(directoryPath)
		key := expansionKey{entry: info, directory: directoryInfo, path: candidate, directoryPath: filepath.Clean(directoryPath)}
		if expansionKeyActive(active, key) {
			return resolvedTarget{path: candidate, info: info, termination: chainLoop}
		}
		if len(active) >= 4096 {
			return resolvedTarget{path: candidate, info: info, termination: chainDepth}
		}
		if splices >= 4096 {
			return resolvedTarget{path: candidate, info: info, termination: chainBudget}
		}
		splices++
		active = append(active, key)
		targetBase := directoryPath
		if filepath.IsAbs(targetText) {
			targetBase = string(filepath.Separator)
		}
		targetBase = filepath.Clean(targetBase)
		targetBaseInfo, err := os.Lstat(targetBase)
		if err != nil {
			protected.add(targetBase, nil)
			return resolvedTarget{path: targetBase, termination: chainStatError}
		}
		frames = append(frames, resolutionFrame{
			current:     targetBase,
			currentInfo: targetBaseInfo,
			components:  strings.Split(filepath.ToSlash(targetText), "/"),
			key:         &key,
		})
	}
	return resolvedTarget{termination: chainUnknown}
}

func expansionKeyActive(active []expansionKey, candidate expansionKey) bool {
	for _, key := range active {
		if key.entry == nil || candidate.entry == nil || !os.SameFile(key.entry, candidate.entry) {
			continue
		}
		if key.directory != nil && candidate.directory != nil {
			if os.SameFile(key.directory, candidate.directory) {
				return true
			}
			continue
		}
		if key.path == candidate.path && key.directoryPath == candidate.directoryPath {
			return true
		}
	}
	return false
}

func pathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

type movedEntry struct {
	originalPath string
	info         os.FileInfo
}

func isMovedEntry(path string, info os.FileInfo, moved []movedEntry) bool {
	clean := filepath.Clean(path)
	for _, entry := range moved {
		if clean == entry.originalPath && sameFileInfo(info, entry.info) {
			return true
		}
	}
	return false
}

func recordOrphaned(record chainRecord, moved []movedEntry) bool {
	if !record.link {
		return false
	}
	ordinary := record.finalRegular && hasMovedChainElement(record, moved)
	mutual := (record.termination == chainLoop || record.termination == chainStatError) && hasMovedChainElement(record, moved)
	return ordinary || mutual
}

func hasMovedChainElement(record chainRecord, moved []movedEntry) bool {
	for _, chain := range record.entries {
		if chain.path == record.path {
			continue
		}
		for _, entry := range moved {
			if chain.path == entry.originalPath && sameFileInfo(chain.info, entry.info) {
				return true
			}
		}
	}
	return false
}

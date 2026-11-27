package service

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

const resultRecordCharge = 256

type resultNamespace struct {
	sessionID string
	epoch     string
	publisher bool
}

type resultOwnerResolver func(resultNamespace) (resultNamespace, bool)

type resultCommit struct {
	handle    string
	retention string
}

type resultLookup struct {
	code      string
	rejection *mcp.CallToolResult
	entry     *resultEntry
	page      int
}

type resultEntry struct {
	handle               string
	owner                resultNamespace
	payload              []byte
	upstreamCode         string
	isError              bool
	createdAt            time.Time
	expiresAt            time.Time
	markerTTL            time.Duration
	deliveryCeilingBytes uint64
	previewCeilingBytes  uint64
	maxUpstreamCodeBytes uint64
	live                 bool
	reservation          bool
}

type resultMarker struct {
	handle     string
	owner      resultNamespace
	code       string
	transition time.Time
	deadline   time.Time
}

type resultTombstone struct {
	createdAt time.Time
	deadline  time.Time
}

type resultNamespaceState struct {
	entries      []*resultEntry
	markers      []*resultMarker
	tombstone    *resultTombstone
	payloadBytes uint64
}

type resultStore struct {
	mu             sync.Mutex
	bounds         config.RetainedResultBounds
	logger         *observe.Logger
	metrics        *observe.Metrics
	now            func() time.Time
	entropy        io.Reader
	entries        map[string]*resultEntry
	markers        map[string]*resultMarker
	namespaces     map[resultNamespace]*resultNamespaceState
	aggregateBytes uint64
}

type resultStoreStats struct {
	aggregateBytes        uint64
	entries               int
	tombstones            int
	namespacePayloadBytes uint64
	markers               int
}

func newResultStore(bounds config.RetainedResultBounds, logger *observe.Logger, metrics *observe.Metrics, now func() time.Time, entropy io.Reader) *resultStore {
	if now == nil {
		now = time.Now
	}
	if entropy == nil {
		entropy = rand.Reader
	}
	return &resultStore{
		bounds:     bounds,
		logger:     logger,
		metrics:    metrics,
		now:        now,
		entropy:    entropy,
		entries:    make(map[string]*resultEntry),
		markers:    make(map[string]*resultMarker),
		namespaces: make(map[resultNamespace]*resultNamespaceState),
	}
}

func (s *resultStore) commit(owner resultNamespace, resolver resultOwnerResolver, payload []byte, upstreamCode string, isError bool) resultCommit {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.sweepLocked(now)
	if uint64(len(payload)) > s.bounds.MaxRetainedResultBytes {
		return s.rejectCommit("result_capacity_exceeded")
	}
	resolved, ok := resolveResultOwner(owner, resolver)
	if !ok || resolved != owner || (owner.sessionID == "" && !owner.publisher) {
		return s.rejectCommit("result_namespace_retired")
	}
	state := s.namespaces[owner]
	if state != nil && state.tombstone != nil {
		return s.rejectCommit("result_namespace_retired")
	}

	payloadBytes := uint64(len(payload))
	for state != nil && (uint64(len(state.entries)) >= s.bounds.MaxEntriesPerNamespace ||
		state.payloadBytes+payloadBytes > s.bounds.MaxBytesPerNamespace) {
		s.evictOldestEntryLocked(state, now)
		state = s.namespaces[owner]
	}
	for state != nil && exceedsAggregate(s.aggregateBytes, payloadBytes+resultRecordCharge, s.bounds.MaxBytesAggregate) && len(state.entries) != 0 {
		s.evictOldestEntryLocked(state, now)
		state = s.namespaces[owner]
	}
	if exceedsAggregate(s.aggregateBytes, payloadBytes+resultRecordCharge, s.bounds.MaxBytesAggregate) {
		return s.rejectCommit("result_aggregate_capacity_exceeded")
	}

	handle, ok := s.newHandleLocked()
	if !ok {
		return s.rejectCommit("result_entropy_unavailable")
	}
	if owner.sessionID != "" && owner.epoch == "" {
		return s.rejectCommit("result_epoch_unresolved")
	}

	entry := &resultEntry{
		handle:               handle,
		owner:                owner,
		payload:              append([]byte(nil), payload...),
		upstreamCode:         boundedUpstreamCode(upstreamCode, s.bounds.MaxUpstreamCodeBytes),
		isError:              isError,
		createdAt:            now,
		expiresAt:            now.Add(s.bounds.HandleTTL),
		markerTTL:            s.bounds.TombstoneTTL,
		deliveryCeilingBytes: s.bounds.DeliveryCeilingBytes,
		previewCeilingBytes:  s.bounds.PreviewCeilingBytes,
		maxUpstreamCodeBytes: s.bounds.MaxUpstreamCodeBytes,
		live:                 true,
		reservation:          true,
	}
	state = s.namespaces[owner]
	if state == nil {
		state = &resultNamespaceState{}
		s.namespaces[owner] = state
	}
	state.entries = append(state.entries, entry)
	state.payloadBytes += payloadBytes
	s.entries[handle] = entry
	s.aggregateBytes += payloadBytes + resultRecordCharge
	if s.metrics != nil {
		s.metrics.RecordResultsPaged()
	}
	return resultCommit{handle: handle}
}

func (s *resultStore) lookup(handle string, owner resultNamespace, resolver resultOwnerResolver, page int) resultLookup {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !validResultHandle(handle) {
		return invalidArgumentsLookup()
	}
	resolved, ok := resolveResultOwner(owner, resolver)
	if !ok || resolved != owner {
		return invalidHandleLookup()
	}
	now := s.now()
	s.sweepLocked(now)
	state := s.namespaces[resolved]
	if state != nil && state.tombstone != nil {
		return invalidHandleLookup()
	}
	if entry := s.entries[handle]; entry != nil {
		if entry.owner != resolved {
			return invalidHandleLookup()
		}
		if page < 1 {
			return invalidArgumentsLookup()
		}
		return resultLookup{entry: entry, page: page}
	}
	if marker := s.markers[handle]; marker != nil && marker.owner == resolved {
		return resultLookup{code: marker.code}
	}
	return invalidHandleLookup()
}

func (s *resultStore) retire(owner resultNamespace) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.sweepLocked(now)
	state := s.namespaces[owner]
	if state == nil || state.tombstone != nil || (len(state.entries) == 0 && len(state.markers) == 0) {
		return
	}
	for _, entry := range state.entries {
		delete(s.entries, entry.handle)
		if entry.live {
			state.payloadBytes -= uint64(len(entry.payload))
			entry.live = false
		}
		if entry.reservation {
			s.aggregateBytes -= resultRecordCharge
			entry.reservation = false
		}
		s.aggregateBytes -= uint64(len(entry.payload))
		entry.payload = nil
	}
	for _, marker := range state.markers {
		delete(s.markers, marker.handle)
		s.aggregateBytes -= resultRecordCharge
	}
	state.entries = nil
	state.markers = nil
	state.payloadBytes = 0
	state.tombstone = &resultTombstone{createdAt: now, deadline: now.Add(s.bounds.TombstoneTTL)}
	s.aggregateBytes += resultRecordCharge
}

func (s *resultStore) stats() resultStoreStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked(s.now())
	stats := resultStoreStats{aggregateBytes: s.aggregateBytes}
	for _, state := range s.namespaces {
		stats.entries += len(state.entries)
		stats.markers += len(state.markers)
		stats.namespacePayloadBytes += state.payloadBytes
		if state.tombstone != nil {
			stats.tombstones++
		}
	}
	return stats
}

func (s *resultStore) installMarker(entry *resultEntry, code string, transition time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.installMarkerLocked(entry, code, transition)
}

func (s *resultStore) sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked(s.now())
}

func (s *resultStore) rejectCommit(code string) resultCommit {
	if s.metrics != nil {
		s.metrics.RecordRetentionRejection(code)
	}
	return resultCommit{retention: code}
}

func (s *resultStore) newHandleLocked() (string, bool) {
	var random [32]byte
	for range 8 {
		if _, err := io.ReadFull(s.entropy, random[:]); err != nil {
			return "", false
		}
		handle := base64.RawURLEncoding.EncodeToString(random[:])
		if _, exists := s.entries[handle]; !exists {
			if _, exists := s.markers[handle]; !exists {
				return handle, true
			}
		}
	}
	return "", false
}

func (s *resultStore) evictOldestEntryLocked(state *resultNamespaceState, now time.Time) {
	if len(state.entries) == 0 {
		return
	}
	entry := state.entries[0]
	s.removeEntryLocked(state, 0, entry, "result_evicted", now)
	if s.metrics != nil {
		s.metrics.RecordRetentionEviction()
	}
}

func (s *resultStore) removeEntryLocked(state *resultNamespaceState, index int, entry *resultEntry, markerCode string, transition time.Time) {
	copy(state.entries[index:], state.entries[index+1:])
	state.entries[len(state.entries)-1] = nil
	state.entries = state.entries[:len(state.entries)-1]
	delete(s.entries, entry.handle)
	if entry.live {
		state.payloadBytes -= uint64(len(entry.payload))
		entry.live = false
		s.aggregateBytes -= uint64(len(entry.payload))
	}
	if markerCode == "" {
		if entry.reservation {
			s.aggregateBytes -= resultRecordCharge
			entry.reservation = false
		}
	} else {
		s.installMarkerLocked(entry, markerCode, transition)
	}
	entry.payload = nil
	s.pruneNamespaceLocked(entry.owner, state)
}

func (s *resultStore) installMarkerLocked(entry *resultEntry, code string, transition time.Time) bool {
	if entry == nil || !entry.reservation || entry.live || (code != "result_expired" && code != "result_evicted") {
		return false
	}
	deadline := transition.Add(entry.markerTTL)
	if !s.now().Before(deadline) || s.bounds.MaxMarkersPerNamespace == 0 {
		s.aggregateBytes -= resultRecordCharge
		entry.reservation = false
		return false
	}
	state := s.namespaces[entry.owner]
	if state == nil {
		state = &resultNamespaceState{}
		s.namespaces[entry.owner] = state
	}
	for uint64(len(state.markers)) >= s.bounds.MaxMarkersPerNamespace {
		s.removeMarkerLocked(state, 0)
	}
	marker := &resultMarker{handle: entry.handle, owner: entry.owner, code: code, transition: transition, deadline: deadline}
	state.markers = append(state.markers, marker)
	s.markers[marker.handle] = marker
	entry.reservation = false
	return true
}

func (s *resultStore) removeMarkerLocked(state *resultNamespaceState, index int) {
	marker := state.markers[index]
	copy(state.markers[index:], state.markers[index+1:])
	state.markers[len(state.markers)-1] = nil
	state.markers = state.markers[:len(state.markers)-1]
	delete(s.markers, marker.handle)
	s.aggregateBytes -= resultRecordCharge
}

func (s *resultStore) sweepLocked(now time.Time) {
	for owner, state := range s.namespaces {
		for i := 0; i < len(state.entries); {
			entry := state.entries[i]
			if !now.Before(entry.expiresAt) {
				s.removeEntryLocked(state, i, entry, "result_expired", entry.expiresAt)
				continue
			}
			i++
		}
		for i := 0; i < len(state.markers); {
			if !now.Before(state.markers[i].deadline) {
				s.removeMarkerLocked(state, i)
				continue
			}
			i++
		}
		if state.tombstone != nil && !now.Before(state.tombstone.deadline) {
			state.tombstone = nil
			s.aggregateBytes -= resultRecordCharge
		}
		s.pruneNamespaceLocked(owner, state)
	}
}

func exceedsAggregate(current, additional, limit uint64) bool {
	return current > limit || additional > limit-current
}

func (s *resultStore) pruneNamespaceLocked(owner resultNamespace, state *resultNamespaceState) {
	if len(state.entries) == 0 && len(state.markers) == 0 && state.tombstone == nil {
		delete(s.namespaces, owner)
	}
}

func resolveResultOwner(owner resultNamespace, resolver resultOwnerResolver) (resultNamespace, bool) {
	if resolver == nil {
		return resultNamespace{}, false
	}
	return resolver(owner)
}

func validResultHandle(handle string) bool {
	if len(handle) != 43 {
		return false
	}
	for i := 0; i < len(handle); i++ {
		c := handle[i]
		if !(c >= 'A' && c <= 'Z') && !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

func invalidHandleLookup() resultLookup {
	return resultLookup{
		code:      "invalid_handle",
		rejection: errorResult("invalid_handle", "result handle is invalid", "unknown"),
	}
}

func invalidArgumentsLookup() resultLookup {
	return resultLookup{
		code:      "invalid_arguments",
		rejection: errorResult("invalid_arguments", "result retrieval arguments are invalid", "unknown"),
	}
}

func boundedUpstreamCode(value string, maxBytes uint64) string {
	if escapedJSONLength(value) <= maxBytes {
		return value
	}
	const marker = "[truncated]"
	markerBytes := uint64(len(marker))
	limit := maxBytes - markerBytes
	prefixBytes := 0
	for index, r := range value {
		width := escapedJSONRuneLength(r)
		if uint64(prefixBytes+width) > limit {
			break
		}
		prefixBytes = index + utf8.RuneLen(r)
	}
	return value[:prefixBytes] + marker
}

func escapedJSONLength(value string) uint64 {
	var length uint64
	for _, r := range value {
		length += uint64(escapedJSONRuneLength(r))
	}
	return length
}

func escapedJSONRuneLength(r rune) int {
	switch r {
	case '"', '\\', '\b', '\f', '\n', '\r', '\t':
		return 2
	case '<', '>', '&', '\u2028', '\u2029':
		return 6
	default:
		if r < 0x20 {
			return 6
		}
		return utf8.RuneLen(r)
	}
}

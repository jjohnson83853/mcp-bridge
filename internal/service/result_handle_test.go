package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

type sequenceResultEntropy struct{ draw uint64 }

func (r *sequenceResultEntropy) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r.draw>>uint((i%8)*8)) ^ byte(i*31)
	}
	r.draw++
	return len(p), nil
}

type failedResultEntropy struct{}

func (failedResultEntropy) Read([]byte) (int, error) { return 0, errors.New("entropy unavailable") }

func resultStoreBounds() config.RetainedResultBounds {
	return config.RetainedResultBounds{
		DeliveryCeilingBytes:   512,
		PreviewCeilingBytes:    128,
		MaxRetainedResultBytes: 256,
		MaxBytesPerNamespace:   4096,
		MaxEntriesPerNamespace: 4,
		MaxBytesAggregate:      8192,
		HandleTTL:              time.Hour,
		TombstoneTTL:           15 * time.Minute,
		MaxUpstreamCodeBytes:   32,
		MaxMarkersPerNamespace: 4,
	}
}

func newTestResultStore(bounds config.RetainedResultBounds, clock *time.Time, entropy io.Reader) *resultStore {
	return newResultStore(bounds, observe.NewLogger(io.Discard, io.Discard), observe.NewMetrics(nil), func() time.Time { return *clock }, entropy)
}

func sessionResultNamespace(session, epoch string) resultNamespace {
	return resultNamespace{sessionID: session, epoch: epoch}
}

func publisherResultNamespace() resultNamespace { return resultNamespace{publisher: true} }

func ownerIs(owner resultNamespace) resultOwnerResolver {
	return func(resultNamespace) (resultNamespace, bool) { return owner, true }
}

func commitResult(t *testing.T, store *resultStore, owner resultNamespace, resolver resultOwnerResolver, payload string) resultCommit {
	t.Helper()
	result := store.commit(owner, resolver, []byte(payload), "", false)
	if result.retention != "" {
		t.Fatalf("commit failed with %q", result.retention)
	}
	if result.handle == "" {
		t.Fatal("admitted result has no handle")
	}
	return result
}

func invalidHandleBytes(t *testing.T, result resultLookup) []byte {
	t.Helper()
	if result.code != "invalid_handle" || result.rejection == nil {
		t.Fatalf("lookup = %#v; want shared invalid_handle rejection", result)
	}
	encoded, err := json.Marshal(result.rejection)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestHandleOwnerEpochBinding(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	bounds := resultStoreBounds()
	bounds.MaxEntriesPerNamespace = 1001
	bounds.MaxBytesPerNamespace = 1 << 20
	bounds.MaxBytesAggregate = 2 << 20
	store := newTestResultStore(bounds, &now, &sequenceResultEntropy{})
	owner := sessionResultNamespace("session-a", "epoch-1")
	seen := make(map[string]struct{}, 1000)
	for i := range 1000 {
		committed := commitResult(t, store, owner, ownerIs(owner), `{"same":true}`)
		if len(committed.handle) != 43 {
			t.Fatalf("handle length = %d; want 43", len(committed.handle))
		}
		for _, char := range committed.handle {
			if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_", char) {
				t.Fatalf("handle contains non-base64url character %q", char)
			}
		}
		if _, duplicate := seen[committed.handle]; duplicate {
			t.Fatalf("duplicate handle %q", committed.handle)
		}
		seen[committed.handle] = struct{}{}
		if i == 0 {
			if got := store.lookup(committed.handle, owner, ownerIs(owner), 1); got.code != "" {
				t.Fatalf("owner lookup code = %q", got.code)
			}
			foreign := sessionResultNamespace("session-b", "epoch-1")
			if got := store.lookup(committed.handle, foreign, ownerIs(foreign), 1); got.code != "invalid_handle" {
				t.Fatalf("cross-namespace lookup = %q; want invalid_handle", got.code)
			}
			bumped := sessionResultNamespace("session-a", "epoch-2")
			if got := store.lookup(committed.handle, owner, func(resultNamespace) (resultNamespace, bool) { return bumped, true }, 1); got.code != "invalid_handle" {
				t.Fatalf("post-epoch-bump lookup = %q; want invalid_handle", got.code)
			}
		}
	}
	failed := newTestResultStore(bounds, &now, failedResultEntropy{})
	if got := failed.commit(owner, ownerIs(owner), []byte(`{"same":true}`), "", false); got.retention != "result_entropy_unavailable" || got.handle != "" {
		t.Fatalf("entropy failure = %#v; want result_entropy_unavailable without a handle", got)
	}
}

func TestHandleExpiry(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	store := newTestResultStore(resultStoreBounds(), &now, &sequenceResultEntropy{})
	owner := sessionResultNamespace("session-a", "epoch-1")
	committed := commitResult(t, store, owner, ownerIs(owner), strings.Repeat("x", 80))
	now = now.Add(time.Hour)
	if got := store.lookup(committed.handle, owner, ownerIs(owner), 1); got.code != "result_expired" {
		t.Fatalf("at handle TTL lookup = %q; want result_expired", got.code)
	}
	now = now.Add(14 * time.Minute)
	if got := store.lookup(committed.handle, owner, ownerIs(owner), 1); got.code != "result_expired" {
		t.Fatalf("before marker deadline lookup = %q; want result_expired", got.code)
	}
	now = now.Add(time.Minute)
	if got := store.lookup(committed.handle, owner, ownerIs(owner), 1); got.code != "invalid_handle" {
		t.Fatalf("at marker deadline lookup = %q; want invalid_handle", got.code)
	}
}

func TestRetentionBounded(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	bounds := resultStoreBounds()
	bounds.MaxEntriesPerNamespace = 2
	bounds.MaxBytesPerNamespace = 200
	bounds.MaxRetainedResultBytes = 100
	bounds.MaxBytesAggregate = 2048
	store := newTestResultStore(bounds, &now, &sequenceResultEntropy{})
	owner := publisherResultNamespace()
	first := commitResult(t, store, owner, ownerIs(owner), strings.Repeat("a", 60))
	second := commitResult(t, store, owner, ownerIs(owner), strings.Repeat("b", 60))
	third := commitResult(t, store, owner, ownerIs(owner), strings.Repeat("c", 60))
	if got := store.lookup(first.handle, owner, ownerIs(owner), 1); got.code != "result_evicted" {
		t.Fatalf("FIFO-oldest lookup = %q; want result_evicted", got.code)
	}
	if got := store.lookup(second.handle, owner, ownerIs(owner), 1); got.code != "" {
		t.Fatalf("second entry lookup = %q; want retained", got.code)
	}
	if got := store.lookup(third.handle, owner, ownerIs(owner), 1); got.code != "" {
		t.Fatalf("third entry lookup = %q; want retained", got.code)
	}
	tooLarge := store.commit(owner, ownerIs(owner), []byte(strings.Repeat("z", 101)), "", false)
	if tooLarge.retention != "result_capacity_exceeded" || tooLarge.handle != "" {
		t.Fatalf("over-cap commit = %#v; want result_capacity_exceeded", tooLarge)
	}
	stats := store.stats()
	if stats.entries != 2 || stats.namespacePayloadBytes > bounds.MaxBytesPerNamespace || stats.aggregateBytes > bounds.MaxBytesAggregate {
		t.Fatalf("retained stats exceed configured bounds: %#v", stats)
	}
	byteBounds := resultStoreBounds()
	byteBounds.MaxEntriesPerNamespace = 4
	byteBounds.MaxBytesPerNamespace = 100
	byteBounds.MaxRetainedResultBytes = 100
	byteStore := newTestResultStore(byteBounds, &now, &sequenceResultEntropy{})
	byteFirst := commitResult(t, byteStore, owner, ownerIs(owner), strings.Repeat("d", 40))
	byteSecond := commitResult(t, byteStore, owner, ownerIs(owner), strings.Repeat("e", 40))
	byteThird := commitResult(t, byteStore, owner, ownerIs(owner), strings.Repeat("f", 40))
	if got := byteStore.lookup(byteFirst.handle, owner, ownerIs(owner), 1); got.code != "result_evicted" {
		t.Fatalf("byte-bound FIFO-oldest lookup = %q; want result_evicted", got.code)
	}
	if got := byteStore.lookup(byteSecond.handle, owner, ownerIs(owner), 1); got.code != "" {
		t.Fatalf("byte-bound second lookup = %q; want retained", got.code)
	}
	if got := byteStore.lookup(byteThird.handle, owner, ownerIs(owner), 1); got.code != "" {
		t.Fatalf("byte-bound third lookup = %q; want retained", got.code)
	}
	if got := byteStore.stats(); got.entries != 2 || got.namespacePayloadBytes > byteBounds.MaxBytesPerNamespace {
		t.Fatalf("byte-bound stats = %#v", got)
	}
}

func TestRetiredNamespaceTombstone(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	store := newTestResultStore(resultStoreBounds(), &now, &sequenceResultEntropy{})
	owner := sessionResultNamespace("session-a", "epoch-1")
	committed := commitResult(t, store, owner, ownerIs(owner), strings.Repeat("x", 64))
	before := store.stats().aggregateBytes
	store.retire(owner)
	retired := store.stats()
	if retired.entries != 0 || retired.tombstones != 1 || retired.aggregateBytes != before-64-256+256 {
		t.Fatalf("retirement state = %#v; want one funded tombstone", retired)
	}
	absentOwner := func(resultNamespace) (resultNamespace, bool) { return resultNamespace{}, false }
	if got := store.lookup(committed.handle, owner, absentOwner, 1); got.code != "invalid_handle" {
		t.Fatalf("retired handle lookup = %q; want invalid_handle", got.code)
	}
	late := store.commit(owner, absentOwner, []byte(strings.Repeat("n", 64)), "", false)
	if late.retention != "result_namespace_retired" || late.handle != "" || store.stats().aggregateBytes != retired.aggregateBytes {
		t.Fatalf("post-retirement commit = %#v, stats=%#v", late, store.stats())
	}
	now = now.Add(15 * time.Minute)
	if got := store.stats().aggregateBytes; got != retired.aggregateBytes-256 {
		t.Fatalf("tombstone charge at half-open deadline = %d; want %d", got, retired.aggregateBytes-256)
	}
}

func TestHandleRejectionUniformStore(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	store := newTestResultStore(resultStoreBounds(), &now, &sequenceResultEntropy{})
	owner := sessionResultNamespace("session-a", "epoch-1")
	committed := commitResult(t, store, owner, ownerIs(owner), strings.Repeat("x", 64))
	foreign := sessionResultNamespace("session-b", "epoch-1")
	retired := sessionResultNamespace("session-c", "epoch-1")
	retiredHandle := commitResult(t, store, retired, ownerIs(retired), strings.Repeat("y", 64))
	store.retire(retired)
	bumped := sessionResultNamespace("session-a", "epoch-2")
	absent := func(resultNamespace) (resultNamespace, bool) { return resultNamespace{}, false }
	cases := []resultLookup{
		store.lookup(strings.Repeat("A", 43), owner, ownerIs(owner), 1),
		store.lookup(strings.Repeat("B", 43), owner, ownerIs(owner), 1),
		store.lookup(committed.handle, foreign, ownerIs(foreign), 1),
		store.lookup(committed.handle, owner, func(resultNamespace) (resultNamespace, bool) { return bumped, true }, 1),
		store.lookup(retiredHandle.handle, retired, absent, 1),
		store.lookup(committed.handle, resultNamespace{}, absent, 1),
	}
	want := invalidHandleBytes(t, cases[0])
	for i, result := range cases[1:] {
		if got := invalidHandleBytes(t, result); !bytes.Equal(got, want) {
			t.Errorf("rejection %d differs: %s != %s", i+1, got, want)
		}
	}
}

func TestEvictedHandleRetrieval(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	bounds := resultStoreBounds()
	bounds.MaxEntriesPerNamespace = 1
	store := newTestResultStore(bounds, &now, &sequenceResultEntropy{})
	owner := publisherResultNamespace()
	first := commitResult(t, store, owner, ownerIs(owner), strings.Repeat("a", 64))
	_ = commitResult(t, store, owner, ownerIs(owner), strings.Repeat("b", 64))
	if got := store.lookup(first.handle, owner, ownerIs(owner), 1); got.code != "result_evicted" {
		t.Fatalf("live eviction marker lookup = %q; want result_evicted", got.code)
	}
	now = now.Add(15 * time.Minute)
	if got := store.lookup(first.handle, owner, ownerIs(owner), 1); got.code != "invalid_handle" {
		t.Fatalf("cleared eviction marker lookup = %q; want invalid_handle", got.code)
	}
}

func TestEvictionNamespaceIsolation(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	bounds := resultStoreBounds()
	bounds.MaxEntriesPerNamespace = 1
	store := newTestResultStore(bounds, &now, &sequenceResultEntropy{})
	namespaceA := sessionResultNamespace("session-a", "epoch-1")
	namespaceB := sessionResultNamespace("session-b", "epoch-1")
	a := commitResult(t, store, namespaceA, ownerIs(namespaceA), strings.Repeat("a", 64))
	b := commitResult(t, store, namespaceB, ownerIs(namespaceB), strings.Repeat("b", 64))
	_ = commitResult(t, store, namespaceA, ownerIs(namespaceA), strings.Repeat("c", 64))
	if got := store.lookup(a.handle, namespaceA, ownerIs(namespaceA), 1); got.code != "result_evicted" {
		t.Fatalf("namespace A oldest handle = %q; want result_evicted", got.code)
	}
	if got := store.lookup(b.handle, namespaceB, ownerIs(namespaceB), 1); got.code != "" {
		t.Fatalf("namespace B handle = %q; want retained", got.code)
	}
}

func TestRetirementAdmissionUnconditional(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	bounds := resultStoreBounds()
	bounds.MaxRetainedResultBytes = 256
	bounds.MaxBytesPerNamespace = 256
	bounds.MaxBytesAggregate = 600
	store := newTestResultStore(bounds, &now, &sequenceResultEntropy{})
	retiring := sessionResultNamespace("session-a", "epoch-1")
	other := sessionResultNamespace("session-b", "epoch-1")
	_ = commitResult(t, store, retiring, ownerIs(retiring), strings.Repeat("a", 64))
	_ = commitResult(t, store, other, ownerIs(other), strings.Repeat("b", 24))
	before := store.stats().aggregateBytes
	if before != bounds.MaxBytesAggregate {
		t.Fatalf("aggregate before retirement = %d; want saturated %d", before, bounds.MaxBytesAggregate)
	}
	store.retire(retiring)
	after := store.stats()
	if after.aggregateBytes > before || after.tombstones != 1 || after.entries != 1 {
		t.Fatalf("retirement was not funded by own releases: before=%d after=%#v", before, after)
	}
	absentOwner := func(resultNamespace) (resultNamespace, bool) { return resultNamespace{}, false }
	late := store.commit(retiring, absentOwner, []byte(strings.Repeat("c", 64)), "", false)
	if late.retention != "result_namespace_retired" || late.handle != "" || store.stats().aggregateBytes != after.aggregateBytes {
		t.Fatalf("late commit after retirement = %#v; stats=%#v", late, store.stats())
	}
	zeroEffect := sessionResultNamespace("session-empty", "epoch-1")
	beforeEmpty := store.stats()
	store.retire(zeroEffect)
	if got := store.stats(); got != beforeEmpty {
		t.Fatalf("retiring a namespace with no records changed store: before=%#v after=%#v", beforeEmpty, got)
	}
}

func TestMarkerAdmissionBound(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	transition := now
	bounds := resultStoreBounds()
	bounds.MaxEntriesPerNamespace = 1
	bounds.MaxMarkersPerNamespace = 1
	store := newTestResultStore(bounds, &now, &sequenceResultEntropy{})
	owner := publisherResultNamespace()

	first := commitResult(t, store, owner, ownerIs(owner), strings.Repeat("a", 64))
	now = now.Add(time.Minute)
	second := commitResult(t, store, owner, ownerIs(owner), strings.Repeat("b", 64))
	before := store.stats()
	third := commitResult(t, store, owner, ownerIs(owner), strings.Repeat("c", 64))
	after := store.stats()

	if got, want := after.aggregateBytes, before.aggregateBytes; got != want {
		t.Fatalf("marker-cap swap aggregate delta = %d; want 0 (reservation converts, evicted marker released, new entry reserves)", got-before.aggregateBytes)
	}
	if after.entries != 1 || after.tombstones != 0 || after.markers != 1 {
		t.Fatalf("marker commit moved an entry/tombstone charge: before=%#v after=%#v", before, after)
	}
	if got := store.lookup(first.handle, owner, ownerIs(owner), 1); got.code != "invalid_handle" {
		t.Fatalf("cap-evicted oldest marker lookup = %q; want invalid_handle", got.code)
	}
	if got := store.lookup(second.handle, owner, ownerIs(owner), 1); got.code != "result_evicted" {
		t.Fatalf("newest marker lookup = %q; want result_evicted", got.code)
	}

	entry := store.entries[third.handle]
	now = transition.Add(bounds.TombstoneTTL)
	beforeBornDead := store.stats()
	store.installMarker(entry, "result_evicted", transition)
	if got := store.stats(); got != beforeBornDead {
		t.Fatalf("born-dead marker changed retained state: before=%#v after=%#v", beforeBornDead, got)
	}
	if got := store.lookup(second.handle, owner, ownerIs(owner), 1); got.code != "result_evicted" {
		t.Fatalf("born-dead marker changed live marker result = %q; want result_evicted", got.code)
	}
	if got := store.lookup(third.handle, owner, ownerIs(owner), 1); got.code != "" {
		t.Fatalf("born-dead marker changed live entry result = %q; want retained", got.code)
	}
	store.sweep()
	if got := store.stats(); got != beforeBornDead {
		t.Fatalf("sweep changed retained state: before=%#v after=%#v", beforeBornDead, got)
	}
	if got := store.lookup(second.handle, owner, ownerIs(owner), 1); got.code != "result_evicted" {
		t.Fatalf("sweep changed logical result = %q; want result_evicted", got.code)
	}
}

package storage_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

// statusSnapshot builds one reconciled snapshot with two configured servers and
// no successful discovery yet: both rows start as "none" (plan §4.7).
func statusSnapshot(t *testing.T, at time.Time) *storage.Snapshot {
	t.Helper()
	snapshot, err := storage.Reconcile(nil, storage.ReconcileInput{
		CreatedAt:               at,
		ConfigPublicFingerprint: "f00d",
		CatalogRevision:         strings.Repeat("a", 64),
		ModelIdentityDigest:     strings.Repeat("b", 64),
		Servers: []storage.ConfiguredServer{
			{ServerID: "alpha", EndpointFingerprint: strings.Repeat("1", 64)},
			{ServerID: "beta", EndpointFingerprint: strings.Repeat("2", 64)},
		},
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	return snapshot
}

func statusRow(t *testing.T, snapshot *storage.Snapshot, serverID string) storage.Server {
	t.Helper()
	for _, row := range snapshot.Servers {
		if row.ServerID == serverID {
			return row
		}
	}
	t.Fatalf("snapshot has no server row %q", serverID)
	return storage.Server{}
}

// TestStampServerSuccessMarksRowFreshWithoutMutatingInput pins that a complete
// accepted discovery moves one row to fresh with its attempt and success time,
// and that the input snapshot is left exactly as it was (plan §4.7/§4.9).
func TestStampServerSuccessMarksRowFreshWithoutMutatingInput(t *testing.T) {
	created := time.Date(2026, 10, 9, 6, 0, 0, 0, time.UTC)
	discovered := created.Add(90 * time.Second)
	snapshot := statusSnapshot(t, created)

	stamped, err := storage.StampServerSuccess(snapshot, "alpha", discovered)
	if err != nil {
		t.Fatalf("stamp server success: %v", err)
	}
	row := statusRow(t, stamped, "alpha")
	if row.CatalogStatus != storage.StatusFresh {
		t.Fatalf("status = %q; want %q", row.CatalogStatus, storage.StatusFresh)
	}
	if !row.LastSuccessAt.Equal(discovered) || !row.LastAttemptAt.Equal(discovered) {
		t.Fatalf("times = success %s attempt %s; want both %s", row.LastSuccessAt, row.LastAttemptAt, discovered)
	}
	if row.Reason != "" {
		t.Fatalf("fresh row carries reason %q; want none", row.Reason)
	}
	if err := stamped.Validate(); err != nil {
		t.Fatalf("stamped snapshot is invalid: %v", err)
	}

	if before := statusRow(t, snapshot, "alpha"); before.CatalogStatus != storage.StatusNone || !before.LastSuccessAt.IsZero() {
		t.Fatalf("input snapshot row changed: %+v", before)
	}
	if _, err := storage.StampServerSuccess(snapshot, "beta", discovered); err != nil {
		t.Fatalf("stamp second server: %v", err)
	}
	if after := statusRow(t, snapshot, "beta"); after.CatalogStatus != storage.StatusNone {
		t.Fatalf("input snapshot row beta changed: %+v", after)
	}
}

// TestStampServerFailurePreservesLastSuccessAndCatalogAge pins that a failed
// refresh retains the last COMPLETE accepted catalog as stale with a finite
// reason, advancing only the attempt time (plan §4.7 reliability/LKG).
func TestStampServerFailurePreservesLastSuccessAndCatalogAge(t *testing.T) {
	created := time.Date(2026, 10, 9, 6, 0, 0, 0, time.UTC)
	succeeded := created.Add(time.Minute)
	failed := created.Add(5 * time.Minute)
	snapshot := statusSnapshot(t, created)

	fresh, err := storage.StampServerSuccess(snapshot, "alpha", succeeded)
	if err != nil {
		t.Fatalf("stamp server success: %v", err)
	}
	stale, err := storage.StampServerFailure(fresh, "alpha", "upstream_unavailable", failed)
	if err != nil {
		t.Fatalf("stamp server failure: %v", err)
	}

	row := statusRow(t, stale, "alpha")
	if row.CatalogStatus != storage.StatusStale {
		t.Fatalf("status = %q; want %q", row.CatalogStatus, storage.StatusStale)
	}
	if row.Reason != "upstream_unavailable" {
		t.Fatalf("reason = %q; want upstream_unavailable", row.Reason)
	}
	if !row.LastAttemptAt.Equal(failed) {
		t.Fatalf("last attempt = %s; want %s", row.LastAttemptAt, failed)
	}
	if !row.LastSuccessAt.Equal(succeeded) {
		t.Fatalf("last success = %s; want %s (catalog age must survive a failure)", row.LastSuccessAt, succeeded)
	}
	if err := stale.Validate(); err != nil {
		t.Fatalf("stale snapshot is invalid: %v", err)
	}
	if err := statusSnapshot(t, created).Validate(); err != nil {
		t.Fatalf("control snapshot is invalid: %v", err)
	}
}

// TestStampRejectsUnknownServerAndUnsafeReason pins fail-closed stamping: a
// misrouted refresh never creates a row, and the persisted reason stays a
// finite vocabulary token that cannot carry a payload (plan §4.11 redaction).
func TestStampRejectsUnknownServerAndUnsafeReason(t *testing.T) {
	created := time.Date(2026, 10, 9, 6, 0, 0, 0, time.UTC)
	now := created.Add(time.Minute)
	snapshot := statusSnapshot(t, created)

	if _, err := storage.StampServerSuccess(snapshot, "gamma", now); err == nil {
		t.Fatal("stamping an unknown server succeeded")
	}
	if len(snapshot.Servers) != 2 {
		t.Fatalf("row count = %d; want the configured two rows only", len(snapshot.Servers))
	}
	for _, unsafe := range []string{
		"",
		"  ",
		"Upstream Unavailable",
		"https://alpha.invalid/mcp",
		"timeout after 15s",
		"unsupported\nreason",
		strings.Repeat("x", 65),
	} {
		if _, err := storage.StampServerFailure(snapshot, "alpha", unsafe, now); err == nil {
			t.Fatalf("unsafe reason %q was accepted", unsafe)
		}
	}
	if row := statusRow(t, snapshot, "alpha"); row.CatalogStatus != storage.StatusNone {
		t.Fatalf("rejected stamp changed the row: %+v", row)
	}
	if _, err := storage.StampServerSuccess(snapshot, "alpha", time.Time{}); err == nil {
		t.Fatal("a zero attempt time was accepted")
	}
	if _, err := storage.StampServerSuccess(snapshot, "alpha", now.In(time.FixedZone("plus2", 7200))); err != nil {
		t.Fatalf("non-UTC attempt time rejected: %v", err)
	}
}

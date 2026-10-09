package storage_test

import (
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
	"strings"
	"testing"
	"time"
)

func TestStampServerEmptyClearsSuccessWithoutMutatingInput(t *testing.T) {
	original := validSnapshot()
	at := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	next, err := storage.StampServerEmpty(original, "portainer", at)
	if err != nil {
		t.Fatal(err)
	}
	if !next.Servers[0].LastAttemptAt.Equal(at) || !next.Servers[0].LastSuccessAt.IsZero() || next.Servers[0].CatalogStatus != storage.StatusNone || next.Servers[0].Reason != "" {
		t.Fatalf("empty status row=%+v", next.Servers[0])
	}
	if original.Servers[0].CatalogStatus != storage.StatusFresh {
		t.Fatal("input row mutated")
	}
	if _, err := storage.StampServerEmpty(nil, "portainer", at); err == nil {
		t.Fatal("nil snapshot accepted")
	}
	if _, err := storage.StampServerEmpty(original, "absent", at); err == nil {
		t.Fatal("missing server accepted")
	}
}

func TestReconcileRejectsInvalidPreviousSnapshot(t *testing.T) {
	previous := validSnapshot()
	previous.CatalogRevision = strings.Repeat("z", 64)
	input := storage.ReconcileInput{CreatedAt: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), ConfigPublicFingerprint: strings.Repeat("c", 64), CatalogRevision: strings.Repeat("d", 64), ModelIdentityDigest: strings.Repeat("e", 64), Servers: []storage.ConfiguredServer{{ServerID: "portainer", EndpointFingerprint: strings.Repeat("b", 64)}}}
	if _, err := storage.Reconcile(previous, input); err == nil {
		t.Fatal("invalid previous snapshot accepted")
	}
}

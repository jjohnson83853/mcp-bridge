package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

func main() {
	dir, err := os.MkdirTemp("", "sdprobe")
	if err != nil {
		panic(err)
	}
	store, err := storage.Open(dir)
	if err != nil {
		panic(err)
	}
	tool := storage.Tool{
		ServerID:             "alpha",
		RawName:              "read",
		PublicName:           "t_" + strings.Repeat("a", 52),
		Description:          "stale",
		DefinitionHash:       strings.Repeat("d", 64),
		EmbeddingContentHash: strings.Repeat("e", 64),
		InputSchema:          json.RawMessage(`{"type":"object"}`),
	}
	snap, err := storage.Reconcile(nil, storage.ReconcileInput{
		CreatedAt:               time.Date(2026, 10, 9, 5, 4, 3, 0, time.UTC),
		ConfigPublicFingerprint: strings.Repeat("b", 64),
		CatalogRevision:         strings.Repeat("a", 64),
		ModelIdentityDigest:     strings.Repeat("c", 64),
		Servers:                 []storage.ConfiguredServer{{ServerID: "alpha", EndpointFingerprint: strings.Repeat("b", 64)}},
	})
	if err != nil {
		panic(err)
	}
	snap, err = storage.IndexTools(snap, "alpha", []storage.Tool{tool})
	if err != nil {
		panic(err)
	}
	if err := store.Save(snap); err != nil {
		panic(err)
	}
	_ = store.Close()

	raw, err := os.ReadFile(filepath.Join(dir, storage.SnapshotFile))
	if err != nil {
		panic(err)
	}
	var probe struct {
		Servers []map[string]any `json:"servers"`
	}
	_ = json.Unmarshal(raw, &probe)
	fmt.Println("servers rows:", probe.Servers)
	fmt.Println("dir:", dir)

	// second phase: open, stamp stale, save
	store2, err := storage.Open(dir)
	if err != nil {
		panic(err)
	}
	loaded, err := store2.Load()
	if err != nil {
		panic(err)
	}
	stamped, err := storage.StampServerFailure(loaded, "alpha", "discovery_failed", time.Date(2026, 10, 9, 5, 5, 0, 0, time.UTC))
	if err != nil {
		panic(err)
	}
	if err := store2.Save(stamped); err != nil {
		fmt.Println("save stamped err:", err)
	}
	_ = store2.Close()
	raw2, _ := os.ReadFile(filepath.Join(dir, storage.SnapshotFile))
	fmt.Println("phase2:", string(raw2)[:200])
}

package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

func TestRunFlagParseErrorWritesUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--not-a-flag"}, &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || !bytes.Contains(stderr.Bytes(), []byte("Usage of mcp-bridge:")) || !bytes.Contains(stderr.Bytes(), []byte("flag provided but not defined")) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestDurableLoaderColdStartAndCorruptSnapshot(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	load := durableLoader(store)
	snapshot, err := load()
	if err != nil || snapshot != nil {
		t.Fatalf("cold load=(%v,%v)", snapshot, err)
	}
	if err := os.WriteFile(store.Path(), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := load(); err == nil || !bytes.Contains([]byte(err.Error()), []byte("corrupt")) {
		t.Fatalf("corrupt load error=%v", err)
	}
}

func TestPumpReloadsForwardsSignalAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hup := make(chan os.Signal, 1)
	reloads := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() { pumpReloads(ctx, hup, reloads); close(done) }()
	hup <- syscall.SIGHUP
	select {
	case <-reloads:
	case <-time.After(time.Second):
		t.Fatal("reload not delivered")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pump did not stop")
	}
}

func TestServeListenerReturnsUnexpectedListenerError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := serveListener(&http.Server{}, listener); err == nil {
		t.Fatal("closed listener was classified as clean shutdown")
	}
}

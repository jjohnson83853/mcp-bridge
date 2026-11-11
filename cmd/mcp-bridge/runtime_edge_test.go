package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

func TestRunFlagParseErrorsAreTypedRecords(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown flag", args: []string{"--not-a-flag"}},
		{name: "missing argument", args: []string{"--config"}},
		{name: "invalid invocation", args: []string{"unexpected"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runContext(context.Background(), test.args, &stdout, &stderr)
			if code != 1 || stdout.Len() != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			lines := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
			if len(lines) != 1 {
				t.Fatalf("stderr lines=%d, want one JSON record: %q", len(lines), stderr.String())
			}
			record := decodeEnvelopeLine(t, lines[0]+"\n")
			errorInfo, ok := record["error"].(map[string]any)
			if record["event"] != "cli_usage_failed" || record["operation"] != "cli" ||
				record["level"] != "error" || record["outcome"] != "failure" ||
				!ok || errorInfo["code"] != "cli_usage" || errorInfo["kind"] != "usage" ||
				errorInfo["retryable"] != false || errorInfo["remediation"] == "" {
				t.Fatalf("CLI error envelope=%#v", record)
			}
			if strings.Contains(stderr.String(), "Usage of mcp-bridge:") ||
				strings.Contains(stderr.String(), "flag provided but not defined") ||
				strings.Contains(stderr.String(), "flag needs an argument") {
				t.Fatalf("stderr contains usage text or parser prose: %q", stderr.String())
			}
		})
	}
}

func TestRunHelpPrintsDefaultUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--help"}, &stdout, &stderr); code != 2 {
		t.Fatalf("help exit=%d, want 2", code)
	}
	if stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "Usage of mcp-bridge:\n") ||
		!strings.Contains(stderr.String(), "-check-config") ||
		!strings.Contains(stderr.String(), "-config string") {
		t.Fatalf("help stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), `"schema_version"`) {
		t.Fatalf("help unexpectedly emitted JSON: %q", stderr.String())
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

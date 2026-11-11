package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
)

type reloadLogCall struct {
	format string
	args   []any
}

func TestPinDiagnosticParityWithCheckConfig(t *testing.T) {
	reloadEnv(t)
	cases := []struct {
		name     string
		pins     string
		code     string
		identity bool
	}{
		{
			name: "pin fields",
			pins: "pins:\n  - {server: alpha}\n",
			code: "pin_fields",
		},
		{
			name: "missing server",
			pins: "pins:\n  - {server: '', tool: valid_tool}\n",
			code: "pin_server_missing",
		},
		{
			name: "missing tool",
			pins: "pins:\n  - {server: alpha, tool: ''}\n",
			code: "pin_tool_missing",
		},
		{
			name:     "unknown server identity",
			pins:     "pins:\n  - {server: beta, tool: valid_tool}\n",
			code:     "pin_server_unknown",
			identity: true,
		},
		{
			name:     "duplicate pin identity",
			pins:     "pins:\n  - {server: alpha, tool: valid_tool}\n  - {server: alpha, tool: valid_tool}\n",
			code:     "pin_duplicate",
			identity: true,
		},
	}

	h := newReloadHarness(t)
	active := newCandidateHolder(parseReloadConfig(t, reloadAlphaOnly))
	var pathMu sync.RWMutex
	var reloadPath string
	signals := make(chan struct{}, 1)
	logCalls := make(chan reloadLogCall, len(cases)+1)
	loop, err := service.NewLoop(h.runtime, service.LoopOptions{
		Load: func() (*config.Config, error) {
			pathMu.RLock()
			path := reloadPath
			pathMu.RUnlock()
			if path != "" {
				return config.ParseFile(path)
			}
			return active.get(), nil
		},
		Deps:     refreshDeps(h),
		Interval: time.Hour,
		Signals:  signals,
		Logf: func(format string, args ...any) {
			logCalls <- reloadLogCall{format: format, args: append([]any(nil), args...)}
		},
		Published: func(assembly *service.Assembly) error {
			h.recordPublished(assembly)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("new loop: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := startLoop(t, ctx, loop)
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("loop shutdown: %v", err)
		}
	})
	waitFor(t, "initial reload admission", func() bool { return h.runtime.Current() != nil })

	select {
	case <-logCalls:
		t.Fatal("successful initial admission unexpectedly logged a reload outcome")
	default:
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			document := strings.Replace(reloadAlphaOnly, "pins: []\n", tc.pins, 1)
			if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			pathMu.Lock()
			reloadPath = path
			pathMu.Unlock()
			_, validationErr := config.ParseFile(path)
			var wantDiagnostics *config.Diagnostics
			if !errors.As(validationErr, &wantDiagnostics) || wantDiagnostics == nil {
				t.Fatalf("ParseFile error = %v; want config diagnostics", validationErr)
			}
			if got := wantDiagnostics.Records[0].Code; got != tc.code {
				t.Fatalf("diagnostic code = %q; want %q", got, tc.code)
			}

			checkRecord := runCheckConfigProcess(t, path)
			if checkRecord["event"] != "config_validation_failed" || checkRecord["operation"] != "load_config" {
				t.Fatalf("check-config record = %#v; want config_validation_failed/load_config", checkRecord)
			}
			checkConfig, ok := checkRecord["config"].(map[string]any)
			if !ok {
				t.Fatalf("check-config config object = %#v", checkRecord["config"])
			}

			signals <- struct{}{}
			var reloadCall reloadLogCall
			select {
			case reloadCall = <-logCalls:
			case <-time.After(5 * time.Second):
				t.Fatal("SIGHUP reload path did not report its rejection")
			}
			if reloadCall.format != "configuration reload rejected: %v" || len(reloadCall.args) < 2 {
				t.Fatalf("reload log call = %#v; want rejected diagnostic and attribution", reloadCall)
			}
			reloadErr, ok := reloadCall.args[0].(error)
			if !ok {
				t.Fatalf("reload first argument = %#v; want diagnostic error", reloadCall.args[0])
			}
			var reloadDiagnostics *config.Diagnostics
			if !errors.As(reloadErr, &reloadDiagnostics) || reloadDiagnostics == nil {
				t.Fatalf("reload error = %v; want the same config diagnostics", reloadErr)
			}
			metadata, ok := reloadCall.args[1].(map[string]string)
			if !ok {
				t.Fatalf("reload attribution = %#v; want revision and trigger", reloadCall.args[1])
			}
			if metadata["config_revision"] != active.get().Fingerprint() || metadata["trigger"] != "trigger" {
				t.Fatalf("reload attribution = %#v; want active revision %q and trigger", metadata, active.get().Fingerprint())
			}

			diagnostic := reloadDiagnostics.Records[0]
			for key, got := range map[string]any{
				"property_path": diagnostic.Key,
				"line":          float64(diagnostic.Line),
				"column":        float64(diagnostic.Column),
			} {
				if checkConfig[key] != got {
					t.Errorf("%s differs: reload=%v check-config=%v", key, got, checkConfig[key])
				}
			}
			for _, key := range []string{"pin_server", "pin_tool"} {
				got := checkConfig[key]
				if tc.identity && got == nil {
					t.Errorf("check-config omitted validated %s identity", key)
				}
				if !tc.identity && got != nil {
					t.Errorf("check-config included %s for %s: %v", key, tc.code, got)
				}
			}
		})
	}
}

func runCheckConfigProcess(t *testing.T, path string) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "run", "../../cmd/mcp-bridge", "--check-config", "--config", path)
	var stdout, stderr strings.Builder
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("check-config command error = %v, stdout=%q stderr=%q; want exit 1", err, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("check-config stdout = %q; want empty", stdout.String())
	}
	line := strings.SplitN(strings.TrimSpace(stderr.String()), "\n", 2)[0]
	var record map[string]any
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		t.Fatalf("decode check-config record %q: %v", line, err)
	}
	return record
}

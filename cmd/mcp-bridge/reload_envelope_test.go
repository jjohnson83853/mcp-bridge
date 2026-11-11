package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
)

func TestConfigReloadRejectionMatchesStartupAttribution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("schema_version: 9\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, parseErr := config.ParseFile(path)
	if parseErr == nil {
		t.Fatal("invalid startup/reload document parsed successfully")
	}

	var stdout, startupStderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--config", path}, &stdout, &startupStderr); code != 1 {
		t.Fatalf("startup exit = %d, want 1; stderr=%q", code, startupStderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("startup stdout = %q, want empty", stdout.String())
	}
	startup := decodeEnvelopeLine(t, startupStderr.String())
	if startup["event"] != "config_validation_failed" || startup["operation"] != "load_config" || startup["level"] != "error" {
		t.Fatalf("startup classification = %#v, want error config_validation_failed/load_config", startup)
	}

	var reloadStderr bytes.Buffer
	operatorLog(&reloadStderr)("configuration reload rejected: %v", parseErr)
	reload := decodeEnvelopeLine(t, reloadStderr.String())
	if reload["event"] != "config_reload_rejected" || reload["operation"] != "reload" || reload["level"] != "warn" || reload["outcome"] != "failure" {
		t.Fatalf("reload classification = %#v, want warn config_reload_rejected/reload/failure", reload)
	}
	for _, key := range []string{"message", "config", "error"} {
		if !reflect.DeepEqual(reload[key], startup[key]) {
			t.Errorf("reload %s = %#v; startup %s = %#v", key, reload[key], key, startup[key])
		}
	}
}

func TestOperatorLogCoversVerifiedLifecycleCallSites(t *testing.T) {
	const secret = "lifecycle-error-secret"
	publicationErr := fmt.Errorf("%w: %s", service.ErrPublicationFailed, secret)
	cases := []struct {
		name, format, level, event, operation, outcome, message string
		code, kind, remediation                              string
		retryable                                             bool
		args                                                  []any
	}{
		{
			name: "internal/service/loop.go:146 startup snapshot rejection",
			format: "startup_snapshot_rejected: %v", level: "warn", event: "startup_snapshot_rejected",
			operation: "startup", outcome: "degraded", message: "startup snapshot unavailable",
			code: "snapshot_unavailable", kind: "resource", retryable: true,
			remediation: "verify the durable snapshot and storage permissions, then retry",
			args: []any{errors.New(secret)},
		},
		{
			name: "internal/service/loop.go:202 reload publication failure",
			format: "configuration reload publication failed after runtime acceptance: %v", level: "warn",
			event: "reload_publication_failed", operation: "publish", outcome: "failure",
			message: "configuration reload publication failed", code: "internal_error", kind: "internal",
			remediation: "inspect the bridge logs and configuration", args: []any{publicationErr},
		},
		{
			name: "internal/service/loop.go:204 non-publication reload rejection",
			format: "configuration reload rejected: %v", level: "warn", event: "reload_rejected",
			operation: "reload", outcome: "failure", message: "configuration reload rejected",
			code: "internal_error", kind: "internal", remediation: "inspect the bridge logs and configuration",
			args: []any{errors.New(secret)},
		},
		{
			name: "internal/service/refresh.go:214 refresh publication failure",
			format: "refresh publication failed after runtime acceptance: %v", level: "warn",
			event: "reload_publication_failed", operation: "publish", outcome: "failure",
			message: "configuration reload publication failed", code: "internal_error", kind: "internal",
			remediation: "inspect the bridge logs and configuration", args: []any{publicationErr},
		},
		{
			name: "cmd/mcp-bridge/serve.go:129 prompt publication rejection",
			format: "prompt publication rejected: %s", level: "warn", event: "prompt_publication_failed",
			operation: "publish", outcome: "failure", message: "prompt publication failed",
			code: "internal_error", kind: "internal", remediation: "inspect the bridge logs and configuration",
			args: []any{"prompt publication rejected: " + secret},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			operatorLog(&logs)(tc.format, tc.args...)
			record := decodeEnvelopeLine(t, logs.String())
			assertOperatorEnvelope(t, record, tc.level, tc.event, tc.operation, tc.outcome, tc.message, tc.code, tc.kind, tc.retryable, tc.remediation)
			if strings.Contains(logs.String(), secret) {
				t.Fatalf("lifecycle log leaked wrapped error text: %q", logs.String())
			}
		})
	}
}

func assertOperatorEnvelope(t *testing.T, record map[string]any, level, event, operation, outcome, message, code, kind string, retryable bool, remediation string) {
	t.Helper()
	for key, want := range map[string]any{
		"level": level, "event": event, "operation": operation, "outcome": outcome, "message": message,
	} {
		if got := record[key]; got != want {
			t.Errorf("%s = %v; want %v", key, got, want)
		}
	}
	errorInfo, ok := record["error"].(map[string]any)
	if !ok {
		t.Fatalf("error = %#v, want nested object", record["error"])
	}
	for key, want := range map[string]any{
		"code": code, "kind": kind, "retryable": retryable, "remediation": remediation,
	} {
		if got := errorInfo[key]; got != want {
			t.Errorf("error.%s = %v; want %v", key, got, want)
		}
	}
}

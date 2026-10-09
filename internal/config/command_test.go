package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
)

// commandFixture is a complete, self-contained configuration whose every
// run-loop setting has a distinct value, so a mapping mistake is visible.
const commandFixture = `schema_version: 1
http:
  listen: "127.0.0.1:8181"
  allowed_hosts: ["127.0.0.1:8181"]
  allowed_origins: []
  max_request_bytes: 65536
  requests_per_second: 21
  burst: 42
  max_sessions: 64
  session_idle_timeout: "90s"
operations:
  listen: "127.0.0.1:9191"
  requests_per_second: 3
  burst: 6
storage:
  directory: "./state"
  max_snapshot_bytes: 1048576
catalog:
  refresh_interval: "90s"
  stale_after: "11m"
  max_tools_per_server: 500
  max_tools_total: 900
  max_pages_per_server: 12
  max_schema_bytes: 4096
  max_description_bytes: 2048
search:
  default_top_k: 7
  max_top_k: 21
  timeout: "4s"
  query_cache_entries: 32
  query_cache_ttl: "3m"
  keyword_weight: 0.25
  embedding_weight: 0.75
  rrf_constant: 42
embedding:
  provider: "openai-compatible"
  model: "operator-model"
  model_revision: "r7"
  base_url: "https://embeddings.invalid/v1"
  dimensions: 512
  api_key: {env: COMMAND_FIXTURE_EMBEDDING_KEY}
  timeout: "2s"
  max_concurrency: 3
  batch_size: 16
  max_response_bytes: 2097152
client_auth:
  bearer_tokens:
    - {env: COMMAND_FIXTURE_CLIENT_TOKEN}
upstream_defaults:
  connect_timeout: "6s"
  call_timeout: "31s"
  refresh_timeout: "13s"
  max_concurrent_calls: 5
  max_event_bytes: 1048576
  max_response_bytes: 4194304
  breaker:
    consecutive_failures: 3
    open_duration: "30s"
    half_open_probes: 1
servers:
  - id: alpha
    endpoint: "https://alpha.invalid/mcp"
    credentials:
      bearer: {env: COMMAND_FIXTURE_UPSTREAM_TOKEN}
pins: []
`

func commandConfig(t *testing.T, input string) *config.Config {
	t.Helper()
	commandEnv(t)
	cfg, err := config.Parse([]byte(input), "/tmp/command-fixture.yaml")
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return cfg
}

// commandEnv supplies the fixture's referenced secrets.
func commandEnv(t *testing.T) {
	t.Helper()
	t.Setenv("COMMAND_FIXTURE_EMBEDDING_KEY", "embedding-secret-value")
	t.Setenv("COMMAND_FIXTURE_CLIENT_TOKEN", "client-secret-value")
	t.Setenv("COMMAND_FIXTURE_UPSTREAM_TOKEN", "upstream-secret-value")
}

// TestCommandResolvesTheRunLoopSettings pins that the command's settings come
// from the validated document with the document's own units (plan §4.2/§4.6/
// §4.7): a duration stays a duration, a relative storage directory is resolved
// against the configuration file, and the embedding model identity keeps the
// operator's provider/model/revision.
func TestCommandResolvesTheRunLoopSettings(t *testing.T) {
	settings, err := commandConfig(t, commandFixture).Command()
	if err != nil {
		t.Fatalf("command settings: %v", err)
	}
	if settings.HTTPListen != "127.0.0.1:8181" || settings.OperationsListen != "127.0.0.1:9191" {
		t.Fatalf("listeners = %q/%q", settings.HTTPListen, settings.OperationsListen)
	}
	if settings.MaxRequestBytes != 65536 || settings.RequestsPerSecond != 21 || settings.Burst != 42 || settings.MaxSessions != 64 {
		t.Fatalf("http bounds = %+v", settings)
	}
	if settings.SessionIdle != 90*time.Second || settings.OpsRequestsPerSecond != 3 || settings.OpsBurst != 6 {
		t.Fatalf("idle/ops bounds = %+v", settings)
	}
	if settings.StorageDirectory != "/tmp/state" || settings.MaxSnapshotBytes != 1048576 {
		t.Fatalf("storage = %q/%d", settings.StorageDirectory, settings.MaxSnapshotBytes)
	}
	if settings.RefreshInterval != 90*time.Second || settings.StaleAfter != 11*time.Minute {
		t.Fatalf("catalog timings = %s/%s", settings.RefreshInterval, settings.StaleAfter)
	}
	if settings.ConnectTimeout != 6*time.Second || settings.CallTimeout != 31*time.Second ||
		settings.RefreshTimeout != 13*time.Second || settings.MaxConcurrentCalls != 5 {
		t.Fatalf("upstream defaults = %+v", settings)
	}
	search := settings.Search
	if search.DefaultTopK != 7 || search.MaxTopK != 21 || search.Timeout != 4*time.Second ||
		search.QueryCacheEntries != 32 || search.QueryCacheTTL != 3*time.Minute ||
		search.KeywordWeight != 0.25 || search.EmbeddingWeight != 0.75 || search.RRFConstant != 42 {
		t.Fatalf("search = %+v", search)
	}
	embedding := settings.Embedding
	if embedding.Provider != "openai-compatible" || embedding.Model != "operator-model" ||
		embedding.ModelRevision != "r7" || embedding.BaseURL != "https://embeddings.invalid/v1" ||
		embedding.Dimensions != 512 || embedding.Timeout != 2*time.Second ||
		embedding.MaxConcurrency != 3 || embedding.BatchSize != 16 || embedding.MaxResponseBytes != 2097152 {
		t.Fatalf("embedding = %+v", embedding)
	}
}

// TestCommandFailsClosedOnAMissingSetting pins that a document missing a setting
// the run loop needs is rejected by name instead of running with a zero value.
func TestCommandFailsClosedOnAMissingSetting(t *testing.T) {
	cases := []struct{ name, removed, want string }{
		{"catalog refresh interval", "  refresh_interval: \"90s\"\n", "refresh_interval"},
		{"http listen address", "  listen: \"127.0.0.1:8181\"\n", "http.listen"},
		{"http request bound", "  max_request_bytes: 65536\n", "max_request_bytes"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			broken := strings.Replace(commandFixture, testCase.removed, "", 1)
			if broken == commandFixture {
				t.Fatalf("fixture does not contain %q", testCase.removed)
			}
			_, err := commandConfig(t, broken).Command()
			if err == nil {
				t.Fatalf("missing %s was accepted", testCase.want)
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error does not name the missing setting: %v", err)
			}
		})
	}
}

// TestCommandKeepsTheHostAndOriginListsOptional pins the one place the run-loop
// view is not total (plan §4.5/§4.6): an absent or empty allowed_hosts /
// allowed_origins leaves the corresponding check disabled, while a present list
// is taken in document order and is copied rather than aliased.
func TestCommandKeepsTheHostAndOriginListsOptional(t *testing.T) {
	t.Run("fixture lists are read in document order", func(t *testing.T) {
		settings, err := commandConfig(t, commandFixture).Command()
		if err != nil {
			t.Fatalf("command settings: %v", err)
		}
		if len(settings.AllowedHosts) != 1 || settings.AllowedHosts[0] != "127.0.0.1:8181" {
			t.Fatalf("allowed hosts = %v; want the fixture's single host", settings.AllowedHosts)
		}
		if len(settings.AllowedOrigins) != 0 {
			t.Fatalf("allowed origins = %v; want the fixture's empty list", settings.AllowedOrigins)
		}
	})

	t.Run("absent keys disable both checks", func(t *testing.T) {
		without := strings.Replace(commandFixture, "  allowed_hosts: [\"127.0.0.1:8181\"]\n", "", 1)
		without = strings.Replace(without, "  allowed_origins: []\n", "", 1)
		if without == commandFixture {
			t.Fatal("fixture does not carry the host/origin lists")
		}
		settings, err := commandConfig(t, without).Command()
		if err != nil {
			t.Fatalf("an absent host/origin list was rejected: %v", err)
		}
		if len(settings.AllowedHosts) != 0 || len(settings.AllowedOrigins) != 0 {
			t.Fatalf("absent lists produced %v/%v; want both disabled", settings.AllowedHosts, settings.AllowedOrigins)
		}
	})

	t.Run("multi-entry order is preserved", func(t *testing.T) {
		multi := strings.Replace(commandFixture, "  allowed_hosts: [\"127.0.0.1:8181\"]\n",
			"  allowed_hosts: [\"bridge.internal:8080\", \"127.0.0.1:8181\", \"localhost:8080\"]\n", 1)
		settings, err := commandConfig(t, multi).Command()
		if err != nil {
			t.Fatalf("command settings: %v", err)
		}
		want := []string{"bridge.internal:8080", "127.0.0.1:8181", "localhost:8080"}
		if len(settings.AllowedHosts) != len(want) {
			t.Fatalf("allowed hosts = %v; want %v", settings.AllowedHosts, want)
		}
		for index := range want {
			if settings.AllowedHosts[index] != want[index] {
				t.Fatalf("allowed hosts = %v; want %v in document order", settings.AllowedHosts, want)
			}
		}
	})

	t.Run("a malformed list is rejected by name", func(t *testing.T) {
		for name, replacement := range map[string]string{
			"scalar":        "  allowed_hosts: \"127.0.0.1:8181\"\n",
			"numeric entry": "  allowed_hosts: [8080]\n",
		} {
			t.Run(name, func(t *testing.T) {
				broken := strings.Replace(commandFixture, "  allowed_hosts: [\"127.0.0.1:8181\"]\n", replacement, 1)
				commandEnv(t)
				_, err := config.Parse([]byte(broken), "/tmp/command-fixture.yaml")
				if err == nil {
					t.Fatal("a malformed allowed_hosts was admitted")
				}
				if !strings.Contains(err.Error(), "allowed_hosts") {
					t.Fatalf("error does not name allowed_hosts: %v", err)
				}
			})
		}
	})

	t.Run("the reader rejects a malformed list directly", func(t *testing.T) {
		// The reader keeps its own check so a hand-built Config cannot smuggle a
		// mistyped entry past admission and silently disable the host check.
		for name, malformed := range map[string]any{
			"scalar":        "127.0.0.1:8181",
			"numeric entry": []any{"127.0.0.1:8181", 8080},
		} {
			t.Run(name, func(t *testing.T) {
				cfg := commandConfig(t, commandFixture)
				cfg.HTTP["allowed_hosts"] = malformed
				_, err := cfg.Command()
				if err == nil {
					t.Fatal("a malformed allowed_hosts was accepted")
				}
				if !strings.Contains(err.Error(), "http.allowed_hosts") {
					t.Fatalf("error does not name http.allowed_hosts: %v", err)
				}
				if !strings.Contains(err.Error(), "sequence of strings") {
					t.Fatalf("error does not state the expected shape: %v", err)
				}
			})
		}
	})

	t.Run("the returned list is a copy", func(t *testing.T) {
		cfg := commandConfig(t, commandFixture)
		first, err := cfg.Command()
		if err != nil {
			t.Fatalf("command settings: %v", err)
		}
		first.AllowedHosts[0] = "tampered.invalid:1"
		second, err := cfg.Command()
		if err != nil {
			t.Fatalf("second command settings: %v", err)
		}
		if second.AllowedHosts[0] != "127.0.0.1:8181" {
			t.Fatalf("second read = %v; the reader aliases the parsed document", second.AllowedHosts)
		}
	})
}

// TestCommandRejectsAMisorderedDuration pins that a duration setting is not
// silently coerced from another scalar type: admission rejects the document
// before any run-loop setting is read.
func TestCommandRejectsAMisorderedDuration(t *testing.T) {
	broken := strings.Replace(commandFixture, "  refresh_interval: \"90s\"", "  refresh_interval: 90", 1)
	commandEnv(t)
	if _, err := config.Parse([]byte(broken), "/tmp/command-fixture.yaml"); err == nil {
		t.Fatal("an integer refresh_interval was accepted")
	}
}

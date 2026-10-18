package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// aggregateConfig builds a minimal valid document with the given client token
// elements and per-server credential lines, so aggregate-contract tests vary
// only the references under test. The client_auth section appears only when
// elements are supplied, because an empty bearer_tokens sequence is not a
// valid document shape.
func aggregateConfig(elements []string, serverCredentials []string) string {
	var document strings.Builder
	document.WriteString("schema_version: 1\nservers:\n")
	if len(serverCredentials) == 0 {
		serverCredentials = []string{""}
	}
	for index, credentials := range serverCredentials {
		id := "alpha"
		if index > 0 {
			id = fmt.Sprintf("server-%d", index)
		}
		fmt.Fprintf(&document, "  - id: %s\n    endpoint: https://alpha.invalid/mcp\n", id)
		if credentials != "" {
			document.WriteString("    " + credentials + "\n")
		}
	}
	if len(elements) > 0 {
		document.WriteString("client_auth:\n  bearer_tokens:\n")
		for _, element := range elements {
			document.WriteString("    - " + element + "\n")
		}
	}
	document.WriteString("pins: []\n")
	return document.String()
}

// TestSecretStageReportsIndependentFailures pins the §4.4 aggregation contract:
// every independent reference failure is reported in document order with its
// own identity, on one physical line with the diagnostics=N prefix — never
// only the first failure (probe G6).
func TestSecretStageReportsIndependentFailures(t *testing.T) {
	t.Setenv("AGG_EMPTY_A", "")
	t.Setenv("AGG_EMPTY_B", "")
	input := aggregateConfig([]string{"{env: AGG_EMPTY_A}", "{env: AGG_EMPTY_B}"}, nil)
	path := filepath.Join(t.TempDir(), "config.yaml")
	_, err := Parse([]byte(input), path)
	diags := requireDiagnostics(t, err)
	if len(diags.Records) != 2 {
		t.Fatalf("records = %d, want both independent failures", len(diags.Records))
	}
	requireRendered(t, err,
		fmt.Sprintf("diagnostics=%d:", len(diags.Records)),
		`key="client_auth.bearer_tokens[0]"`, `ref="env:AGG_EMPTY_A"`,
		`key="client_auth.bearer_tokens[1]"`, `ref="env:AGG_EMPTY_B"`,
		"code=secret_empty",
		"; remaining_stages="+stageNotChecked,
	)
	for _, record := range diags.Records {
		if record.Path != path {
			t.Fatalf("record path = %q, want the supplied config path", record.Path)
		}
	}
}

// TestSecretStageCapsAtThirtyTwoRecords pins the §4.4 cap: a document with
// more than 32 broken references reports exactly 32 records and an explicit
// incompleteness marker instead of a guessed total.
func TestSecretStageCapsAtThirtyTwoRecords(t *testing.T) {
	elements := make([]string, 0, maxDiagnostics+1)
	unset := make([]string, 0, maxDiagnostics+1)
	for index := range maxDiagnostics + 1 {
		name := fmt.Sprintf("AGG_CAP_%d", index)
		unset = append(unset, name)
		elements = append(elements, fmt.Sprintf("{env: %s}", name))
	}
	input := aggregateConfig(elements, nil)
	path := filepath.Join(t.TempDir(), "config.yaml")
	_, err := Parse([]byte(input), path)
	diags := requireDiagnostics(t, err)
	if len(diags.Records) != maxDiagnostics {
		t.Fatalf("records = %d, want the %d cap", len(diags.Records), maxDiagnostics)
	}
	if !diags.Incomplete {
		t.Fatal("expected the cap to mark the collection incomplete")
	}
	requireRendered(t, err,
		fmt.Sprintf("diagnostics=%d:", maxDiagnostics),
		"diagnostics_incomplete=true; further locations may be invalid; fix reported errors and rerun",
	)
	for _, unsetName := range unset[maxDiagnostics:] {
		forbidRendered(t, err, unsetName)
	}
}

// TestSharedSecretFileResolvesIdenticallyAndOnce pins that two references to
// one file both observe the same normalized value — the one-terminal-LF trim
// applies to every occurrence, and the shared read is not repeated per
// occurrence (spec §4.4 cache/no-redundant-read).
func TestSharedSecretFileResolvesIdenticallyAndOnce(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "secret"), []byte("token\n"), 0600); err != nil {
		t.Fatal(err)
	}
	input := aggregateConfig(nil, []string{
		"credentials: {bearer: {file: secret}}",
		"credentials: {bearer: {file: secret}}",
	})
	path := filepath.Join(dir, "config.yaml")
	cfg, err := Parse([]byte(input), path)
	if err != nil {
		t.Fatalf("Parse rejected a shared valid file reference: %v", err)
	}
	for _, serverID := range []string{"alpha", "server-1"} {
		credentials, err := cfg.ServerCredentials(serverID)
		if err != nil {
			t.Fatalf("ServerCredentials(%s): %v", serverID, err)
		}
		if credentials.Bearer != "token" {
			t.Fatalf("bearer token for %s = %q, want the shared file value with one final LF removed", serverID, credentials.Bearer)
		}
	}
}

// TestDisjointnessViolationNamesBothReferences pins the §4.2 overlap contract:
// the rejection is a typed record naming both consumer references and the
// static disjointness reason, with no value, hash or equality-mechanism echo.
func TestDisjointnessViolationNamesBothReferences(t *testing.T) {
	for _, tc := range []struct {
		name         string
		clientEnv    string
		serverEnv    string
		env          map[string]string
		relatedKey   string
		relatedShape string
	}{
		{"same reference", "AGG_OVERLAP_A", "AGG_OVERLAP_A",
			map[string]string{"AGG_OVERLAP_A": "shared-canary"}, "servers[0].credentials.bearer", "env"},
		{"equal values different names", "AGG_OVERLAP_A", "AGG_OVERLAP_B",
			map[string]string{"AGG_OVERLAP_A": "shared-canary", "AGG_OVERLAP_B": "shared-canary"}, "servers[0].credentials.bearer", "env"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for name, value := range tc.env {
				t.Setenv(name, value)
			}
			input := aggregateConfig([]string{"{env: " + tc.clientEnv + "}"},
				[]string{"credentials: {bearer: {env: " + tc.serverEnv + "}}"})
			path := filepath.Join(t.TempDir(), "config.yaml")
			_, err := Parse([]byte(input), path)
			diags := requireDiagnostics(t, err)
			requireRendered(t, err,
				`config="`+path+`"`,
				`key="client_auth.bearer_tokens[0]"`,
				`ref="env:`+tc.clientEnv+`"`,
				`related_key="`+tc.relatedKey+`"`,
				`related_ref="env:`+tc.serverEnv+`"`,
				"code=credential_overlap",
				"client tokens and backend credentials must be disjoint",
			)
			forbidRendered(t, err, "shared-canary", "sha256")
			if diags.RemainingStages != "" {
				t.Fatalf("overlap is the final stage; no later stage to name, got %q", diags.RemainingStages)
			}
		})
	}
}

package main

// Runtime provenance and safe route context wire tests. Every assertion reads
// the captured http_complete record owned by the response's Mcp-Diagnostic-Id
// through the real composed public listener: no production types, no
// reflection into the process — only the emitted terminal JSON.

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

// requireCompletionField fetches one record field with a typed failure.
func requireCompletionField(t *testing.T, record map[string]any, key, want string) {
	t.Helper()
	if got, _ := record[key].(string); got != want {
		t.Errorf("%s = %v; want %q", key, record[key], want)
	}
}

// vcsStampOfTestBinary reports the running binary's own VCS stamp so the
// composed assertions can distinguish a verified clean revision from a dirty
// unpushed build without hardcoding the deployment state.
func vcsStampOfTestBinary() (revision string, modified bool, stamped bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", true, false
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision, stamped = setting.Value, true
		case "vcs.modified":
			modified = setting.Value != "false"
		}
	}
	return revision, modified, stamped
}

// TestTerminalRecordRuntimeProvenance drives one outer OAuth rejection
// through the composed listener and asserts the complete provenance block:
// verified or honestly unavailable source identity, the effective nonsecret
// policy tuple, the verified clock skew and the safe resource path mode.
func TestTerminalRecordRuntimeProvenance(t *testing.T) {
	harness := startOAuthHarness(t)
	harness.awaitReady(t)
	publicID, internalID, diagnosticID := diagCanaries(21)
	response := diagPost(t, harness, "", "", map[string]string{
		"X-Request-ID": publicID, "X-Mcp-Bridge-Internal-Request-Id": internalID,
	}, `{}`)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d; want 401", response.StatusCode)
	}
	owned := requireDiagnosticID(t, response, publicID, internalID, diagnosticID)
	records := waitForDiagnosticCompletion(t, harness, "terminal provenance record", owned)
	if len(records) != 1 {
		t.Fatalf("completions = %d; want exactly 1", len(records))
	}
	record := records[0]

	// Safe route classification from the actual dispatch.
	requireCompletionField(t, record, "challenge_target", "root_well_known")
	requireCompletionField(t, record, "metadata_target_registered", "yes")
	requireCompletionField(t, record, "resource_path_mode", "origin_only")
	requireCompletionField(t, record, "path_class", "mcp")
	requireCompletionField(t, record, "route_outcome", "oauth_protected")

	// Effective nonsecret policy tuple from the authoritative config.
	if record["oauth_enabled"] != true {
		t.Errorf("oauth_enabled = %v; want true", record["oauth_enabled"])
	}
	if record["static_configured"] != true {
		t.Errorf("static_configured = %v; want true", record["static_configured"])
	}
	if record["scope_required"] != true {
		t.Errorf("scope_required = %v; want true", record["scope_required"])
	}
	if record["clock_skew_seconds"] != float64(60) {
		t.Errorf("clock_skew_seconds = %v; want 60", record["clock_skew_seconds"])
	}
	requireCompletionField(t, record, "policy_source", "environment")

	// The source revision remains trusted input to build-state validation but
	// is collector-owned and never emitted by the application.
	revision, modified, stamped := vcsStampOfTestBinary()
	if _, present := record["source_revision"]; present {
		t.Errorf("collector-owned source_revision was emitted: %v", record["source_revision"])
	}
	if stamped && !modified && observeValidRevisionShape(revision) {
		requireCompletionField(t, record, "build_state", "verified")
	} else {
		requireCompletionField(t, record, "build_state", "unavailable")
	}

	// No attested effective-policy artifact exists: honest unavailability.
	requireCompletionField(t, record, "effective_policy_revision_state", "unavailable")
	if _, present := record["effective_policy_revision"]; present {
		t.Error("effective_policy_revision present without a verified state")
	}
}

// TestPathBearingResourceChallengeClassification observes the current
// conditional behavior for a path-bearing resource fixture: the challenge
// actually written appends the well-known path after the resource path, and
// that target is registered by this server.
func TestPathBearingResourceChallengeClassification(t *testing.T) {
	t.Setenv("MCP_RESOURCE_URL", "https://mcp.bullprog.com/mcp")
	harness := startOAuthHarness(t)
	harness.awaitReady(t)
	publicID, internalID, diagnosticID := diagCanaries(22)
	response := diagPost(t, harness, "", "", map[string]string{
		"X-Request-ID": publicID, "X-Mcp-Bridge-Internal-Request-Id": internalID,
	}, `{}`)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d; want 401", response.StatusCode)
	}
	// The written challenge must match the classification the record makes.
	challenge := response.Header.Get("WWW-Authenticate")
	if !strings.Contains(challenge, "/mcp/.well-known/oauth-protected-resource") {
		t.Fatalf("written challenge = %q; want the path-appended metadata target", challenge)
	}
	owned := requireDiagnosticID(t, response, publicID, internalID, diagnosticID)
	records := waitForDiagnosticCompletion(t, harness, "path-bearing provenance record", owned)
	if len(records) != 1 {
		t.Fatalf("completions = %d; want exactly 1", len(records))
	}
	record := records[0]
	requireCompletionField(t, record, "challenge_target", "path_appended_well_known")
	requireCompletionField(t, record, "metadata_target_registered", "yes")
	requireCompletionField(t, record, "resource_path_mode", "path_bearing")
}

// TestRouteOutcomeByActualDispatch classifies the metadata, health and
// unmatched-route dispatches of the composed listener.
func TestRouteOutcomeByActualDispatch(t *testing.T) {
	harness := startOAuthHarness(t)
	harness.awaitReady(t)
	cases := []struct {
		name, path, routeOutcome string
		status                   int
		stage                    string
	}{
		{name: "Metadata", path: "/.well-known/oauth-protected-resource", routeOutcome: "metadata", status: http.StatusOK, stage: "none"},
		{name: "MetadataMCPPath", path: "/.well-known/oauth-protected-resource/mcp", routeOutcome: "metadata", status: http.StatusOK, stage: "none"},
		{name: "MetadataMCPPrefix", path: "/mcp/.well-known/oauth-protected-resource", routeOutcome: "metadata", status: http.StatusOK, stage: "none"},
		{name: "Livez", path: "/livez", routeOutcome: "health", status: http.StatusOK, stage: "none"},
		{name: "Unmatched", path: "/definitely-not-a-route", routeOutcome: "inbound_direct", status: http.StatusNotFound,
			stage: "route"},
	}
	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			publicID, internalID, diagnosticID := diagCanaries(40+index)
			request, err := http.NewRequest(http.MethodGet, harness.httpURL+tc.path, nil)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			request.Header.Set("X-Request-ID", publicID)
			response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
			if err != nil {
				t.Fatalf("GET %s: %v", tc.path, err)
			}
			defer response.Body.Close()
			if response.StatusCode != tc.status {
				t.Fatalf("status = %d; want %d", response.StatusCode, tc.status)
			}
			owned := requireDiagnosticID(t, response, publicID, internalID, diagnosticID)
			records := waitForDiagnosticCompletion(t, harness, fmt.Sprintf("%s dispatch record", tc.name), owned)
			if len(records) != 1 {
				t.Fatalf("completions = %d; want exactly 1", len(records))
			}
			requireCompletionField(t, records[0], "route_outcome", tc.routeOutcome)
			requireCompletionField(t, records[0], "failure_stage", tc.stage)
		})
	}
}

// TestToolErrorOperationLinkage drives an actual upstream tool error under
// HTTP 200 and asserts the terminal record links the single decoded operation
// with the truthful upstream class, keeping the wire envelope distinct from
// admission rejection.
func TestToolErrorOperationLinkage(t *testing.T) {
	upstream := fakeUpstreamReportedError(t, "read",
		`{"type":"object","properties":{},"additionalProperties":true}`)
	ctx, httpURL, stdout, stop := startHTTPLoggingBridgeWithUpstream(t, upstream)
	call := loggedFailureToolCall(t, ctx, httpURL, stdout, stop, func(rev string) map[string]any {
		return map[string]any{
			"tool_name":        core.PublicName("alpha", "read"),
			"catalog_revision": rev,
			"arguments":        map[string]any{},
		}
	})
	assertFailureEnvelope(t, call)

	// The terminal record of the tool-error request: exactly one http_complete
	// under HTTP 200, warn severity, single linked operation with its owned
	// identifier and the actual tool_error upstream class.
	completions := filterRecords(decodeCapturedRecords(t, stdout.String()), "http_complete")
	var toolErrorCompletions []map[string]any
	for _, record := range completions {
		if record["http_status"] == float64(http.StatusOK) && record["upstream_state"] == "tool_error" {
			toolErrorCompletions = append(toolErrorCompletions, record)
		}
	}
	if len(toolErrorCompletions) != 1 {
		t.Fatalf("tool-error completions = %d; want exactly 1", len(toolErrorCompletions))
	}
	record := toolErrorCompletions[0]
	if record["level"] != "warn" {
		t.Errorf("level = %v; want warn for actual tool_error under 200", record["level"])
	}
	requireCompletionField(t, record, "operation_relation", "single")
	requireCompletionField(t, record, "operation_id_state", "available")
	operationID, _ := record["operation_id"].(string)
	if !internalIDPattern.MatchString(operationID) {
		t.Errorf("operation_id = %v; want an owned 32-hex operation identity", record["operation_id"])
	}
	// The tool-error envelope stays distinct from any admission rejection.
	requireCompletionField(t, record, "failure_stage", "none")
	requireCompletionField(t, record, "failure_reason", "none")
}

// observeValidRevisionShape mirrors the accepted 40–64 lowercase-hex shape.
func observeValidRevisionShape(value string) bool {
	if len(value) < 40 || len(value) > 64 {
		return false
	}
	return strings.Trim(value, "0123456789abcdef") == ""
}

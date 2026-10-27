package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/auth"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

// sourceRevisionSample is a synthetic 64-hex immutable source revision.
const sourceRevisionSample = "e621622397298bb4eb44a9b0f5ff1a00d6f16300aa11bb22cc33dd44ee55ff66"

func TestSourceRevisionFromBuildDistinguishesVerifiedFromUnpushed(t *testing.T) {
	t.Cleanup(func() { readBuildProvenance = debug.ReadBuildInfo })
	t.Run("VerifiedCleanStamp", func(t *testing.T) {
		readBuildProvenance = func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: sourceRevisionSample},
				{Key: "vcs.modified", Value: "false"},
			}}, true
		}
		revision, ok := sourceRevisionFromBuild()
		if !ok || revision != sourceRevisionSample {
			t.Fatalf("source revision = %q, %v; want %q, true", revision, ok, sourceRevisionSample)
		}
	})
	t.Run("UnpushedLocalBuildClaimsNothing", func(t *testing.T) {
		readBuildProvenance = func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: sourceRevisionSample},
				{Key: "vcs.modified", Value: "true"},
			}}, true
		}
		revision, ok := sourceRevisionFromBuild()
		if ok {
			t.Fatalf("dirty build claimed revision %q as deployable source", revision)
		}
	})
	t.Run("MissingStampStaysUnavailable", func(t *testing.T) {
		readBuildProvenance = func() (*debug.BuildInfo, bool) { return nil, false }
		if _, ok := sourceRevisionFromBuild(); ok {
			t.Fatal("missing build stamp produced a source revision")
		}
	})
	t.Run("HashShapedTagIsNotProvenance", func(t *testing.T) {
		readBuildProvenance = func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: strings.Repeat("Z", 64)},
			}}, true
		}
		if _, ok := sourceRevisionFromBuild(); ok {
			t.Fatal("non-lowercase-hex value was accepted as a source revision")
		}
	})
}

func TestBuildDiagnosticProvenanceFromAuthoritativeInterfaces(t *testing.T) {
	t.Cleanup(func() { readBuildProvenance = debug.ReadBuildInfo })
	t.Cleanup(func() { lookupEnv = origLookupEnvForProvenance })
	readBuildProvenance = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: sourceRevisionSample},
			{Key: "vcs.modified", Value: "false"},
		}}, true
	}
	lookupEnv = func(name string) (string, bool) { return "", false }
	provenance := buildDiagnosticProvenance(auth.Config{
		PocketIDJWKSURL:  "https://id.example/jwks.json",
		MCPResourceURL:   "https://mcp.example.com/mcp",
		MCPRequiredScope: "mcp:connect",
	}, true)
	if provenance.SourceRevision != sourceRevisionSample {
		t.Errorf("source revision = %q; want %q", provenance.SourceRevision, sourceRevisionSample)
	}
	if provenance.ResourcePathMode != auth.ResourcePathMode("https://mcp.example.com/mcp") {
		t.Errorf("resource path mode = %q", provenance.ResourcePathMode)
	}
	if provenance.PolicySource != "defaults" {
		t.Errorf("policy source = %q; want defaults with unset environment", provenance.PolicySource)
	}
	if provenance.OAuthEnabled == nil || !*provenance.OAuthEnabled {
		t.Error("oauth_enabled must record the configured JWKS authority")
	}
	if provenance.StaticConfigured == nil || !*provenance.StaticConfigured {
		t.Error("static_configured must record the configured client token")
	}
	if provenance.ScopeRequired == nil || !*provenance.ScopeRequired {
		t.Error("scope_required must record the configured scope")
	}
	if provenance.ClockSkewSeconds == nil || *provenance.ClockSkewSeconds != auth.AcceptableSkewSeconds {
		t.Error("clock skew must record the verified 60 second tolerance")
	}
	// No attested nonsecret policy artifact exists: the effective policy
	// revision carries no value (the owner normalizes the absent state to
	// explicit unavailability), and no secret-bearing configuration is hashed
	// into any provenance value.
	if provenance.EffectivePolicyRevisionState != "" || provenance.EffectivePolicyRevision != "" {
		t.Errorf("effective policy = %q/%q; want absent/absent",
			provenance.EffectivePolicyRevisionState, provenance.EffectivePolicyRevision)
	}
}

// origLookupEnvForProvenance restores the production environment seam after
// provenance classification tests.
var origLookupEnvForProvenance = lookupEnv

func TestEnvironmentPolicySourceDetection(t *testing.T) {
	t.Cleanup(func() { lookupEnv = origLookupEnvForProvenance })
	lookupEnv = func(name string) (string, bool) {
		if name == "MCP_RESOURCE_URL" {
			return "https://mcp.example.com", true
		}
		return "", false
	}
	if got := policySourceFromEnv(); got != "environment" {
		t.Errorf("policy source = %q; want environment", got)
	}
}

func TestProvenanceRejectsArbitraryAliases(t *testing.T) {
	// The composed record must never carry an arbitrary or hostile
	// environment alias: it degrades to the absent state instead.
	oauth := true
	sink := &bytes.Buffer{}
	diag := observe.NewHTTPDiagnostics(observe.NewLogger(sink), observe.HTTPDiagnosticsOptions{
		Provenance: observe.HTTPProvenance{
			Environment: "../../etc", PolicySource: "file", OAuthEnabled: &oauth,
			ResourcePathMode: "path_bearing",
		},
	})
	diag.Wrap(http.NotFoundHandler()).ServeHTTP(
		httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/mcp", nil))
	for _, line := range strings.Split(strings.TrimSpace(sink.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("decode record: %v", err)
		}
		if record["event"] != "http_complete" {
			continue
		}
		if _, present := record["diag_environment"]; present {
			t.Errorf("hostile environment alias emitted: %v", record["diag_environment"])
		}
		if record["policy_source"] != "file" {
			t.Errorf("policy_source = %v; want file", record["policy_source"])
		}
	}
}

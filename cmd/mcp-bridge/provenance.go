package main

import (
	"runtime/debug"
	"os"

	"github.com/jjohnson83853/mcp-bridge/internal/auth"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

// readBuildProvenance is the trusted source-identity seam: production reads
// the VCS stamp of the running binary; trusted startup tests may replace it.
var readBuildProvenance = debug.ReadBuildInfo

// sourceRevisionFromBuild extracts the verified immutable VCS source revision
// of the running binary. A dirty (locally modified, unpushed) build yields no
// revision: unpushed local code never claims to be the deployed source. The
// revision is separate from the process incarnation and from mutable tags.
func sourceRevisionFromBuild() (string, bool) {
	info, ok := readBuildProvenance()
	if !ok {
		return "", false
	}
	revision := ""
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			if setting.Value != "false" {
				return "", false
			}
		}
	}
	if !observe.ValidSourceRevision(revision) {
		return "", false
	}
	return revision, true
}

// buildDiagnosticProvenance assembles the safe runtime provenance from
// authoritative existing interfaces: the effective auth configuration for the
// nonsecret policy tuple and the binary's VCS stamp for the source revision.
// No secret-bearing configuration is hashed and no hidden issuer, principal,
// client or audience equality is inferred; no attested nonsecret
// policy-content artifact exists yet, so the effective policy revision stays
// explicitly unavailable.
func buildDiagnosticProvenance(cfg auth.Config, staticConfigured bool) observe.HTTPProvenance {
	provenance := observe.HTTPProvenance{
		PolicySource:     policySourceFromEnv(),
		ResourcePathMode: auth.ResourcePathMode(cfg.MCPResourceURL),
	}
	oauthEnabled := cfg.PocketIDJWKSURL != ""
	scopeRequired := cfg.MCPRequiredScope != ""
	skew := auth.AcceptableSkewSeconds
	provenance.OAuthEnabled = &oauthEnabled
	provenance.StaticConfigured = &staticConfigured
	provenance.ScopeRequired = &scopeRequired
	provenance.ClockSkewSeconds = &skew
	if revision, ok := sourceRevisionFromBuild(); ok {
		provenance.SourceRevision = revision
	}
	return provenance
}

// lookupEnv is the environment seam for policy-source classification;
// production reads the process environment.
var lookupEnv = os.LookupEnv

// policySourceFromEnv classifies where the effective auth policy came from
// without recording any value: an explicit environment source when any auth
// variable is set, defaults otherwise.
func policySourceFromEnv() string {
	for _, name := range []string{
		"POCKET_ID_ISSUER", "POCKET_ID_JWKS_URL", "MCP_RESOURCE_URL", "MCP_REQUIRED_SCOPE",
	} {
		if value, ok := lookupEnv(name); ok && value != "" {
			return "environment"
		}
	}
	return "defaults"
}

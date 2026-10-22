package auth

import "testing"

func TestNewConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("POCKET_ID_ISSUER", "")
	t.Setenv("POCKET_ID_JWKS_URL", "")
	t.Setenv("MCP_RESOURCE_URL", "")
	t.Setenv("MCP_REQUIRED_SCOPE", "")

	config := NewConfigFromEnv()

	if config.PocketIDIssuer != DefaultPocketIDIssuer {
		t.Errorf("PocketIDIssuer = %q, want default %q", config.PocketIDIssuer, DefaultPocketIDIssuer)
	}
	if config.PocketIDJWKSURL != DefaultPocketIDJWKSURL {
		t.Errorf("PocketIDJWKSURL = %q, want default %q", config.PocketIDJWKSURL, DefaultPocketIDJWKSURL)
	}
	if config.MCPResourceURL != DefaultMCPResourceURL {
		t.Errorf("MCPResourceURL = %q, want default %q", config.MCPResourceURL, DefaultMCPResourceURL)
	}
	if config.MCPRequiredScope != DefaultMCPRequiredScope {
		t.Errorf("MCPRequiredScope = %q, want default %q", config.MCPRequiredScope, DefaultMCPRequiredScope)
	}
}

func TestNewConfigFromEnvOverrides(t *testing.T) {
	const (
		issuer  = "https://issuer.example.com"
		jwks    = "https://issuer.example.com/jwks.json"
		wantResource = "https://mcp.example.com"
		scope   = "mcp:tools"
	)

	t.Setenv("POCKET_ID_ISSUER", issuer)
	t.Setenv("POCKET_ID_JWKS_URL", jwks)
	t.Setenv("MCP_RESOURCE_URL", wantResource)
	t.Setenv("MCP_REQUIRED_SCOPE", scope)

	config := NewConfigFromEnv()

	if config.PocketIDIssuer != issuer {
		t.Errorf("PocketIDIssuer = %q, want %q", config.PocketIDIssuer, issuer)
	}
	if config.PocketIDJWKSURL != jwks {
		t.Errorf("PocketIDJWKSURL = %q, want %q", config.PocketIDJWKSURL, jwks)
	}
	if config.MCPResourceURL != wantResource {
		t.Errorf("MCPResourceURL = %q, want %q", config.MCPResourceURL, wantResource)
	}
	if config.MCPRequiredScope != scope {
		t.Errorf("MCPRequiredScope = %q, want %q", config.MCPRequiredScope, scope)
	}
}

func TestNewConfigFromEnvPartialOverrides(t *testing.T) {
	const issuer = "https://partial.example.com"

	t.Setenv("POCKET_ID_ISSUER", issuer)
	t.Setenv("POCKET_ID_JWKS_URL", "")
	t.Setenv("MCP_RESOURCE_URL", "")
	t.Setenv("MCP_REQUIRED_SCOPE", "")

	config := NewConfigFromEnv()

	if config.PocketIDIssuer != issuer {
		t.Errorf("PocketIDIssuer = %q, want %q", config.PocketIDIssuer, issuer)
	}
	if config.PocketIDJWKSURL != DefaultPocketIDJWKSURL {
		t.Errorf("PocketIDJWKSURL = %q, want default %q", config.PocketIDJWKSURL, DefaultPocketIDJWKSURL)
	}
	if config.MCPResourceURL != DefaultMCPResourceURL {
		t.Errorf("MCPResourceURL = %q, want default %q", config.MCPResourceURL, DefaultMCPResourceURL)
	}
	if config.MCPRequiredScope != DefaultMCPRequiredScope {
		t.Errorf("MCPRequiredScope = %q, want default %q", config.MCPRequiredScope, DefaultMCPRequiredScope)
	}
}

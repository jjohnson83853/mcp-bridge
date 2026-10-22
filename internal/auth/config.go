// Package auth provides configuration and JWT claim helpers for the
// MCP bridge's Pocket ID based authentication flow.
package auth

import "os"

// Config holds the authentication settings resolved from the environment.
type Config struct {
	PocketIDIssuer   string
	PocketIDJWKSURL  string
	MCPResourceURL   string
	MCPRequiredScope string
}

// Default values applied when the corresponding environment variables are unset.
const (
	DefaultPocketIDIssuer   = "https://id.bullprog.com"
	DefaultPocketIDJWKSURL  = "https://id.bullprog.com/.well-known/jwks.json"
	DefaultMCPResourceURL   = "https://mcp.bullprog.com"
	DefaultMCPRequiredScope = ""
)

// NewConfigFromEnv builds a Config from environment variables, falling back
// to the package defaults for any variable that is unset or empty.
func NewConfigFromEnv() Config {
	return Config{
		PocketIDIssuer:   envOr("POCKET_ID_ISSUER", DefaultPocketIDIssuer),
		PocketIDJWKSURL:  envOr("POCKET_ID_JWKS_URL", DefaultPocketIDJWKSURL),
		MCPResourceURL:   envOr("MCP_RESOURCE_URL", DefaultMCPResourceURL),
		MCPRequiredScope: envOr("MCP_REQUIRED_SCOPE", DefaultMCPRequiredScope),
	}
}

// envOr returns the value of the named environment variable, or the fallback
// when the variable is unset or empty.
func envOr(name, fallback string) string {
	if value, ok := lookupEnv(name); ok && value != "" {
		return value
	}
	return fallback
}

// lookupEnv is a small indirection so tests can inject values via t.Setenv
// while keeping the lookup logic in one place.
func lookupEnv(name string) (string, bool) {
	return os.LookupEnv(name)
}

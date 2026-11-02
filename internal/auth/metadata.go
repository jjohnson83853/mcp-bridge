package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

// Metadata is the OAuth 2.0 Protected Resource metadata document served at
// the resource server's well-known endpoint (RFC 9727 style response).
type Metadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	ScopesSupported        []string `json:"scopes_supported"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
}

// NewProtectedResourceHandler returns a handler serving the protected
// resource metadata document built from cfg. The payload is marshalled once
// at construction so each request only writes pre-rendered bytes.
func NewProtectedResourceHandler(cfg Config) http.Handler {
	buildMetadata := func(resource string) []byte {
		meta := Metadata{
			Resource:               resource,
			AuthorizationServers:   []string{cfg.PocketIDIssuer},
			ScopesSupported:        []string{"openid", "email", "profile", "mcp:connect"},
			BearerMethodsSupported: []string{"header"},
		}
		// Metadata holds only strings and string slices, so Marshal cannot
		// fail; the error is deliberately discarded to keep the request path
		// branchless.
		payload, _ := json.Marshal(meta)
		return payload
	}

	canonicalResource := cfg.MCPResourceURL
	if !strings.HasSuffix(strings.TrimRight(canonicalResource, "/"), "/mcp") {
		canonicalResource = strings.TrimRight(canonicalResource, "/") + "/mcp"
	}
	canonicalPayload := buildMetadata(canonicalResource)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Dispatch classification: this handler actually served the request.
		// Contribution is a no-op without a completion owner in the context.
		observe.ContributeHTTP(r.Context(), observe.HTTPDecision{RouteOutcome: "metadata"})
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// All well-known targets advertise the canonical MCP endpoint resource.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(canonicalPayload)
	})
}

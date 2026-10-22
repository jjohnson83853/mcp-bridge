package auth

import (
	"encoding/json"
	"net/http"
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
	meta := Metadata{
		Resource:               cfg.MCPResourceURL,
		AuthorizationServers:   []string{cfg.PocketIDIssuer},
		ScopesSupported:        []string{"openid", "email", "profile", "mcp:connect"},
		BearerMethodsSupported: []string{"header"},
	}

	// Metadata holds only strings and string slices, so Marshal cannot fail;
	// the error is deliberately discarded to keep the request path branchless.
	payload, _ := json.Marshal(meta)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	})
}

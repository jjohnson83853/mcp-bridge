package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestProtectedResourceHandlerGet(t *testing.T) {
	cfg := Config{
		PocketIDIssuer:  DefaultPocketIDIssuer,
		MCPResourceURL:  DefaultMCPResourceURL,
		PocketIDJWKSURL: DefaultPocketIDJWKSURL,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)

	NewProtectedResourceHandler(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}

	var meta Metadata
	if err := json.Unmarshal(body, &meta); err != nil {
		t.Fatalf("unmarshalling body %q: %v", body, err)
	}

	if meta.Resource != cfg.MCPResourceURL {
		t.Errorf("Resource = %q, want %q", meta.Resource, cfg.MCPResourceURL)
	}
	if want := []string{cfg.PocketIDIssuer}; !slices.Equal(meta.AuthorizationServers, want) {
		t.Errorf("AuthorizationServers = %v, want %v", meta.AuthorizationServers, want)
	}
	if !slices.Contains(meta.ScopesSupported, "mcp:connect") {
		t.Errorf("ScopesSupported = %v, want it to contain %q", meta.ScopesSupported, "mcp:connect")
	}
	if !slices.Contains(meta.BearerMethodsSupported, "header") {
		t.Errorf("BearerMethodsSupported = %v, want it to contain %q", meta.BearerMethodsSupported, "header")
	}
}

func TestProtectedResourceHandlerRejectsNonGetMethods(t *testing.T) {
	cfg := Config{PocketIDIssuer: DefaultPocketIDIssuer, MCPResourceURL: DefaultMCPResourceURL}
	handler := NewProtectedResourceHandler(cfg)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/.well-known/oauth-protected-resource", nil)

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s: status = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
			}
			if got := rec.Body.String(); got != "method not allowed\n" {
				t.Errorf("%s: body = %q, want %q", method, got, "method not allowed\n")
			}
		})
	}
}

func TestProtectedResourceHandlerDynamicConfig(t *testing.T) {
	cfg := Config{
		PocketIDIssuer: "https://sso.example.org",
		MCPResourceURL: "https://api.example.org/mcp",
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)

	NewProtectedResourceHandler(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var meta Metadata
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatalf("unmarshalling body %q: %v", rec.Body.String(), err)
	}

	if meta.Resource != cfg.MCPResourceURL {
		t.Errorf("Resource = %q, want %q", meta.Resource, cfg.MCPResourceURL)
	}
	if want := []string{cfg.PocketIDIssuer}; !slices.Equal(meta.AuthorizationServers, want) {
		t.Errorf("AuthorizationServers = %v, want %v", meta.AuthorizationServers, want)
	}
}

func TestProtectedResourceHandlerPathBearing(t *testing.T) {
	paths := []string{
		"/.well-known/oauth-protected-resource",
		"/.well-known/oauth-protected-resource/mcp",
		"/mcp/.well-known/oauth-protected-resource",
	}
	for _, tc := range []struct {
		name        string
		resourceURL string
		want        string
	}{
		{
			name:        "origin configured",
			want:        "https://api.example.org/mcp",
			resourceURL: "https://api.example.org",
		},
		{
			name:        "endpoint configured",
			resourceURL: "https://api.example.org/mcp",
			want:        "https://api.example.org/mcp",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewProtectedResourceHandler(Config{
				PocketIDIssuer: "https://sso.example.org",
				MCPResourceURL: tc.resourceURL,
			})
			for _, path := range paths {
				t.Run(path, func(t *testing.T) {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodGet, path, nil)

					handler.ServeHTTP(rec, req)

					if rec.Code != http.StatusOK {
						t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
					}
					if got := rec.Header().Get("Content-Type"); got != "application/json" {
						t.Errorf("Content-Type = %q, want %q", got, "application/json")
					}

					var meta Metadata
					if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
						t.Fatalf("unmarshalling body %q: %v", rec.Body.String(), err)
					}
					if meta.Resource != tc.want {
						t.Errorf("Resource = %q, want %q", meta.Resource, tc.want)
					}
				})
			}
		})
	}
}

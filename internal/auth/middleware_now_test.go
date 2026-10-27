package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// TestWithNowControlsVerifierClock pins the WithNow adapter: an expired token
// relative to the injected clock is rejected without sleeps, while the same
// token is accepted when the injected clock sits inside its validity window.
// Effects are independent of any diagnostics clock.
func TestWithNowControlsVerifierClock(t *testing.T) {
	priv, _, set := mustTestKey(t)
	provider := StaticKeySet{Set: set}

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	token := jwt.New()
	_ = token.Set(jwt.SubjectKey, "clock-test-user")
	_ = token.Set(jwt.IssuerKey, "https://issuer.example.com")
	_ = token.Set(jwt.AudienceKey, []string{"https://resource.example.com"})
	_ = token.Set(jwt.IssuedAtKey, base.Add(-time.Minute))
	_ = token.Set(jwt.ExpirationKey, base.Add(5*time.Minute))
	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256, priv))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	bearer := "Bearer " + string(signed)

	cfg := Config{
		MCPResourceURL: "https://resource.example.com",
		PocketIDIssuer: "https://issuer.example.com",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	newMiddleware := func(now func() time.Time) http.Handler {
		opts := []Option{}
		if now != nil {
			opts = append(opts, WithNow(now))
		}
		return NewAuthMiddleware(cfg, provider, opts...)(handler)
	}

	request := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		r.Header.Set("Authorization", bearer)
		return r
	}

	t.Run("ExpiredUnderInjectedClock", func(t *testing.T) {
		later := base.Add(10 * time.Minute)
		rec := httptest.NewRecorder()
		newMiddleware(func() time.Time { return later }).ServeHTTP(rec, request())
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d; want 401 when the verifier clock is past expiry", rec.Code)
		}
	})
	t.Run("ValidUnderInjectedClock", func(t *testing.T) {
		rec := httptest.NewRecorder()
		newMiddleware(func() time.Time { return base.Add(time.Second) }).ServeHTTP(rec, request())
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d; want 200 when the verifier clock is inside validity", rec.Code)
		}
	})
}

// Keep context imported for the provider contract documentation above.
var _ context.Context = context.Background()

package auth

import (
	"context"
	"testing"
)

func TestContextWithClaimsAndClaimsFromContext(t *testing.T) {
	want := &Claims{
		Subject:  "user-123",
		Email:    "user@example.com",
		Issuer:   DefaultPocketIDIssuer,
		Audience: []string{DefaultMCPResourceURL},
		Scopes:   []string{"mcp:tools"},
	}

	ctx := ContextWithClaims(context.Background(), want)
	got := ClaimsFromContext(ctx)

	if got != want {
		t.Errorf("ClaimsFromContext() = %+v, want %+v", got, want)
	}
	if got.Subject != want.Subject {
		t.Errorf("Subject = %q, want %q", got.Subject, want.Subject)
	}
}

func TestClaimsFromContextWithoutClaims(t *testing.T) {
	ctx := context.Background()

	if got := ClaimsFromContext(ctx); got != nil {
		t.Errorf("ClaimsFromContext() = %+v, want nil", got)
	}
}

func TestClaimsFromContextNilContext(t *testing.T) {
	//nolint:staticcheck // nil context is exactly the nil-handling case under test.
	var ctx context.Context

	if got := ClaimsFromContext(ctx); got != nil {
		t.Errorf("ClaimsFromContext(nil) = %+v, want nil", got)
	}
}

func TestSubjectFromContextWithClaims(t *testing.T) {
	const subject = "user-abc"

	ctx := ContextWithClaims(context.Background(), &Claims{Subject: subject})

	if got := SubjectFromContext(ctx); got != subject {
		t.Errorf("SubjectFromContext() = %q, want %q", got, subject)
	}
}

func TestSubjectFromContextWithoutClaims(t *testing.T) {
	if got := SubjectFromContext(context.Background()); got != "" {
		t.Errorf("SubjectFromContext() = %q, want empty string", got)
	}
}

func TestSubjectFromContextNilContext(t *testing.T) {
	//nolint:staticcheck // nil context is exactly the nil-handling case under test.
	var ctx context.Context

	if got := SubjectFromContext(ctx); got != "" {
		t.Errorf("SubjectFromContext(nil) = %q, want empty string", got)
	}
}

package main

import (
	"context"
	"io"
	"time"
)

// This file implements the trusted startup-context diagnostic hooks. The
// hooks are injected only through the process context that the composition
// root (or a test harness) supplies at startup; request headers and operator
// configuration can never supply clock, entropy or sink authority. Production
// defaults remain time.Now, cryptographic generation and stdout.

// diagnosticDeps carries the injectable diagnostic collaborators for one
// composed server run.
type diagnosticDeps struct {
	Now       func() time.Time
	NewID     func() string
	LogWriter io.Writer
}

type diagDepsContextKey struct{}

// withDiagnosticDeps installs the diagnostic collaborators into a trusted
// startup context.
func withDiagnosticDeps(ctx context.Context, deps diagnosticDeps) context.Context {
	return context.WithValue(ctx, diagDepsContextKey{}, deps)
}

// diagnosticDepsFromContext returns the installed collaborators, or false
// when none are present.
func diagnosticDepsFromContext(ctx context.Context) (diagnosticDeps, bool) {
	deps, ok := ctx.Value(diagDepsContextKey{}).(diagnosticDeps)
	return deps, ok && (deps.Now != nil || deps.NewID != nil || deps.LogWriter != nil)
}

// diagnosticNowFromContext returns the composed clock when trusted startup
// wiring supplies one, otherwise production time.Now.
func diagnosticNowFromContext(ctx context.Context) func() time.Time {
	if deps, ok := diagnosticDepsFromContext(ctx); ok && deps.Now != nil {
		return deps.Now
	}
	return time.Now
}

// diagnosticNewIDFromContext returns the composed entropy source when trusted
// startup wiring supplies one, otherwise the default cryptographic generator.
func diagnosticNewIDFromContext(ctx context.Context) func() string {
	if deps, ok := diagnosticDepsFromContext(ctx); ok && deps.NewID != nil {
		return deps.NewID
	}
	return nil
}

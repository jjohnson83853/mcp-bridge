package main

import (
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
)

func TestDiagnosticsEnvelopeHasRemediationForEveryDiagnosticCode(t *testing.T) {
	for _, code := range config.DiagnosticCodes() {
		t.Run(code, func(t *testing.T) {
			if remediation := configRemediation(code); remediation == "" {
				t.Fatalf("configRemediation(%q) is empty", code)
			}
		})
	}
}

func TestDiagnosticsEnvelopePreservesReasonAndShape(t *testing.T) {
	diagnostic := config.Diagnostic{
		Path: "/tmp/config.yaml", Key: "search.max_top_k", Line: 4, Column: 15,
		Code: "bound_range", Shape: "above_maximum", Reason: "outside permitted bounds",
	}
	envelope := configValidationFailureEnvelope(&config.Diagnostics{Records: []config.Diagnostic{diagnostic}})
	if envelope.Message != diagnostic.Reason {
		t.Fatalf("message = %q, want diagnostic reason %q", envelope.Message, diagnostic.Reason)
	}
	if envelope.Config == nil || envelope.Config.Shape != diagnostic.Shape {
		t.Fatalf("config.shape = %#v, want %q", envelope.Config, diagnostic.Shape)
	}
	if envelope.Error == nil || envelope.Error.Remediation == "" {
		t.Fatal("expected a static remediation")
	}
}

package config

import (
	"errors"
	"strings"
	"testing"
)

// TestDiagnosticCollectionRecordsMetadataOnly verifies the T-15 contract: a
// rejected Parse carries a typed, errors.As-discoverable collection of
// metadata-only records, and the single renderer formats them with the
// supplied config path, known location, finite code/property/shape and a
// static reason — never a value, never raw decoder/OS text.
func TestDiagnosticCollectionRecordsMetadataOnly(t *testing.T) {
	doc := []byte("schema_version: 1\nservers:\n  - id: alpha\n    endpoint: https://alpha.invalid/mcp\n    credentials:\n      bearer:\n        env: DIAG_MISSING_TOKEN\npins: []\n")
	// The environment variable is deliberately left unset: os.LookupEnv must
	// report the unset state, which is the only metadata the record may carry.
	cfg, err := Parse(doc, "/tmp/diag-config.yaml")
	if err == nil {
		t.Fatal("expected Parse to reject the empty secret reference")
	}
	if cfg != nil {
		t.Fatal("never publish a partial Config on failure")
	}
	var diags *Diagnostics
	if !errors.As(err, &diags) {
		t.Fatalf("expected errors.As(*Diagnostics); got %T: %v", err, err)
	}
	if diags == nil || len(diags.Records) == 0 {
		t.Fatalf("expected at least one diagnostic record, got %+v", diags)
	}
	record := diags.Records[0]
	if record.Path != "/tmp/diag-config.yaml" {
		t.Fatalf("expected supplied config path carried unchanged, got %q", record.Path)
	}
	if record.Key != "servers[0].credentials.bearer" {
		t.Fatalf("expected full known location, got %q", record.Key)
	}
	if record.RefKind != "env" || record.RefName != "DIAG_MISSING_TOKEN" {
		t.Fatalf("expected env reference identity, got kind=%q name=%q", record.RefKind, record.RefName)
	}
	if record.Server != "alpha" {
		t.Fatalf("expected validated server owner in metadata, got %q", record.Server)
	}
	// Metadata-only: no value, no preview, no length.
	for _, leak := range []string{"canary-value", "12", "length"} {
		if strings.Contains(record.Reason, leak) {
			t.Fatalf("reason must not disclose values/lengths: %q", record.Reason)
		}
	}
	// Opaque-only bodies are retired: the old combined string must not be the body.
	if strings.Contains(err.Error(), "secret must be nonempty UTF-8, <=8KiB, without CR/LF/NUL") {
		t.Fatalf("opaque-only body must be retired from the error body; got %q", err.Error())
	}
	// Single renderer line: prefix-free here (Parse returns raw), one line per record.
	line := err.Error()
	if strings.Contains(line, "\n") {
		t.Fatalf("expected single physical line, got %q", line)
	}
	if !strings.Contains(line, `config="/tmp/diag-config.yaml"`) {
		t.Fatalf("expected config= prefix in rendered line, got %q", line)
	}
	if !strings.Contains(line, "code=secret_unset") {
		t.Fatalf("expected unset class for an unset env var, got %q", line)
	}
}

// TestDirectConfigOriginIsMemory verifies that a directly constructed Config
// (no Parse origin) reports `<memory>` as its origin rather than an empty or
// invented path.
func TestDirectConfigOriginIsMemory(t *testing.T) {
	cfg := Config{}
	if got := cfg.Origin(); got != "<memory>" {
		t.Fatalf("expected <memory> origin for directly constructed Config, got %q", got)
	}
}

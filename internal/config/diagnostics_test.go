package config

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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


func TestSecretStageDiagnosticCarriesReferenceNodePosition(t *testing.T) {
	t.Setenv("DIAG_MISSING_TOKEN", "")
	_, err := Parse([]byte("schema_version: 1\nservers:\n  - id: alpha\n    endpoint: https://alpha.invalid/mcp\n    credentials:\n      bearer:\n        env: DIAG_MISSING_TOKEN\npins: []\n"), "/tmp/diag-config.yaml")
	diags := requireDiagnostics(t, err)
	record := diags.Records[0]
	if record.Code != "secret_empty" {
		t.Fatalf("code = %q, want secret_empty", record.Code)
	}
	if record.Line != 7 || record.Column != 9 {
		t.Fatalf("secret reference position = %d:%d, want 7:9", record.Line, record.Column)
	}
}

func TestBoundsStageDiagnosticCarriesInspectedValuePosition(t *testing.T) {
	input := "schema_version: 1\nservers:\n  - id: alpha\n    endpoint: https://alpha.invalid/mcp\n    limits:\n      max_concurrent_calls: 0\npins: []\n"
	_, err := Parse([]byte(input), "/tmp/bounds-config.yaml")
	diags := requireDiagnostics(t, err)
	record := diags.Records[0]
	if record.Code != "bound_range" || record.Key != "servers[0].limits.max_concurrent_calls" {
		t.Fatalf("diagnostic = %+v, want bound_range for max_concurrent_calls", record)
	}
	if record.Line != 6 || record.Column != 29 {
		t.Fatalf("bound position = %d:%d, want 6:29", record.Line, record.Column)
	}
}

func TestEmbeddingReferenceDiagnosticCarriesReferenceNodePosition(t *testing.T) {
	input := "schema_version: 1\nservers:\n  - id: alpha\n    endpoint: https://alpha.invalid/mcp\nembedding:\n  model: {env: 'BAD-$NAME'}\npins: []\n"
	_, err := Parse([]byte(input), "/tmp/embedding-config.yaml")
	diags := requireDiagnostics(t, err)
	record := diags.Records[0]
	if record.Code != "reference_literal_required" || record.Key != "embedding.model" {
		t.Fatalf("diagnostic = %+v, want embedding reference_literal_required", record)
	}
	if record.Line != 6 || record.Column != 10 {
		t.Fatalf("embedding reference position = %d:%d, want 6:10", record.Line, record.Column)
	}
}

func TestDiagnosticCodesAreSortedUniqueAndDefensive(t *testing.T) {
	codes := DiagnosticCodes()
	if len(codes) == 0 || !sort.StringsAreSorted(codes) {
		t.Fatalf("DiagnosticCodes() = %v, want a nonempty sorted set", codes)
	}
	for index := 1; index < len(codes); index++ {
		if codes[index] == codes[index-1] {
			t.Fatalf("DiagnosticCodes() contains duplicate %q", codes[index])
		}
	}
	codes[0] = "caller_mutation"
	if got := DiagnosticCodes()[0]; got == "caller_mutation" {
		t.Fatal("DiagnosticCodes() exposes its backing storage")
	}
}

func assertConfigNodePosition(t *testing.T, record Diagnostic, line, column int) {
	t.Helper()
	if record.Line != line || record.Column != column {
		t.Fatalf("%s position = %d:%d, want %d:%d", record.Code, record.Line, record.Column, line, column)
	}
}

func assertDiagnosticAtYAMLPath(t *testing.T, input []byte, record Diagnostic, path string) {
	t.Helper()
	var document yaml.Node
	if err := yaml.Unmarshal(input, &document); err != nil {
		t.Fatalf("decode position fixture: %v", err)
	}
	if len(document.Content) == 0 {
		t.Fatal("position fixture has no YAML document")
	}
	node := nodeAtPath(document.Content[0], path)
	if node == nil {
		t.Fatalf("no YAML node at %q", path)
	}
	assertConfigNodePosition(t, record, node.Line, node.Column)
}

func assertNoDiagnosticNodePosition(t *testing.T, record Diagnostic) {
	t.Helper()
	if record.Line != 0 || record.Column != 0 {
		t.Fatalf("%s has source position %d:%d without a source node", record.Code, record.Line, record.Column)
	}
}

func assertRelatedDiagnosticAtYAMLPath(t *testing.T, input []byte, record Diagnostic, path string) {
	t.Helper()
	var document yaml.Node
	if err := yaml.Unmarshal(input, &document); err != nil {
		t.Fatalf("decode related-position fixture: %v", err)
	}
	if len(document.Content) == 0 {
		t.Fatal("related-position fixture has no YAML document")
	}
	node := nodeAtPath(document.Content[0], path)
	if node == nil {
		t.Fatalf("no related YAML node at %q", path)
	}
	if record.RelatedLine != node.Line || record.RelatedColumn != node.Column {
		t.Fatalf("%s related position = %d:%d, want %d:%d", record.Code, record.RelatedLine, record.RelatedColumn, node.Line, node.Column)
	}
}

func sourceTextPosition(t *testing.T, input, needle string, last bool) (int, int) {
	t.Helper()
	index := strings.Index(input, needle)
	if last {
		index = strings.LastIndex(input, needle)
	}
	if index < 0 {
		t.Fatalf("position fixture does not contain %q", needle)
	}
	line := strings.Count(input[:index], "\n") + 1
	lineStart := strings.LastIndex(input[:index], "\n") + 1
	return line, index - lineStart + 1
}

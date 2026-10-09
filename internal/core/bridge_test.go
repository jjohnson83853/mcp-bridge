package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"strings"
	"testing"
)

// §4.3: a server_id is permanently bound to one normalized endpoint
// fingerprint — a different endpoint must use a new ID, so rebinding is
// rejected and the last-known-good catalog is retained.
func TestRegisterRejectsEndpointRebind(t *testing.T) {
	b := NewBridge()
	defs := []ToolDefinition{{RawName: "read", InputSchema: json.RawMessage(`{"type":"object"}`)}}
	if err := b.Register("portainer", "https://portainer.example/mcp", defs); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := b.Register("portainer", "https://elsewhere.example/mcp", defs); err == nil {
		t.Fatal("endpoint rebind accepted; want rejection")
	}
	if _, ok := b.Binding(PublicName("portainer", "read")); !ok {
		t.Fatal("last-known-good binding lost after rejected rebind")
	}
	// The rejected rebind must not have moved the endpoint fingerprint.
	if err := b.Register("portainer", "https://elsewhere.example/mcp", nil); err == nil {
		t.Fatal("endpoint fingerprint changed by rejected rebind")
	}
}

// §4.3: an empty raw tool name is not a valid identity.
func TestRegisterRejectsEmptyRawName(t *testing.T) {
	b := NewBridge()
	if err := b.Register("portainer", "https://portainer.example/mcp", []ToolDefinition{{RawName: ""}}); err == nil {
		t.Fatal("empty raw tool name accepted")
	}
	if got := b.Revision(); !isHex64(got) {
		t.Fatalf("revision after rejected register = %q; want 64 hex chars", got)
	}
}

// §4.4: an unavailable upstream must fail closed without inventing provenance.
func TestDispatchNilUpstreamFailsClosed(t *testing.T) {
	b := NewBridge()
	if err := b.Register("portainer", "https://portainer.example/mcp", []ToolDefinition{
		{RawName: "read", InputSchema: json.RawMessage(`{"type":"object"}`)},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	env, derr := b.Dispatch(context.Background(), PublicName("portainer", "read"), b.Revision(), json.RawMessage(`{}`), nil)
	if env != nil || derr == nil || derr.Code != "unknown_tool" {
		t.Fatalf("nil upstream dispatch = (%v, %v); want closed failure", env, derr)
	}
	if derr.Retryable || derr.ExecutionState != ExecutionNotStarted {
		t.Fatalf("nil upstream state %+v", derr)
	}
}

func TestAddPinsRejectsNilBridge(t *testing.T) {
	pins, err := AddPins(nil, []PinnedTool{{ServerID: "portainer", RawName: "read"}})
	if err == nil || pins != nil {
		t.Fatalf("AddPins(nil) = (%v, %v); want error and no pins", pins, err)
	}
}

// §4.3 identity: no HTML escaping and no whitespace in the canonical form, so
// raw names containing '<', '>' or '&' hash the literal bytes.
func TestPublicNameUsesUnescapedCanonicalForm(t *testing.T) {
	const server = "portainer"
	const raw = `a<b>&c`
	got := PublicName(server, raw)

	blob, err := json.Marshal([]string{server, raw}) // Marshal escapes HTML
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	escaped := "t_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sha256Sum(blob)))

	var canonical bytes.Buffer
	encoder := json.NewEncoder(&canonical)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode([]string{server, raw}); err != nil {
		t.Fatalf("encode: %v", err)
	}
	unescaped := "t_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sha256Sum(bytes.TrimSuffix(canonical.Bytes(), []byte("\n")))))

	if escaped == unescaped {
		t.Fatal("test fixture does not exercise HTML escaping")
	}
	if got != unescaped {
		t.Fatalf("PublicName HTML-escaped the canonical form: got %s want %s", got, unescaped)
	}
}

func TestPublicNameIgnoresWhitespaceVariants(t *testing.T) {
	if PublicName("portainer", "read") != PublicName("portainer", "read") {
		t.Fatal("PublicName is not deterministic")
	}
	// Distinct raw names never share a public name.
	if PublicName("portainer", "read") == PublicName("portainer", "read ") {
		t.Fatal("trailing whitespace in a raw name was normalized away")
	}
}

// §4.3: revision is deterministic, covers descriptions as part of the exact
// definition, and is a bare 64-character lowercase digest.
func TestRevisionCoversDescriptionAndIsDeterministic(t *testing.T) {
	register := func(description string) *Bridge {
		t.Helper()
		b := NewBridge()
		if err := b.Register("portainer", "https://portainer.example/mcp", []ToolDefinition{
			{RawName: "read", Description: description, InputSchema: json.RawMessage(`{"type":"object"}`)},
		}); err != nil {
			t.Fatalf("register: %v", err)
		}
		return b
	}
	first := register("reads a container")
	if first.Revision() != first.Revision() {
		t.Fatal("revision is not deterministic")
	}
	if !isHex64(first.Revision()) {
		t.Fatalf("revision %q is not 64 lowercase hex characters", first.Revision())
	}
	if first.Revision() == register("reads everything").Revision() {
		t.Fatal("description change did not move the catalog revision")
	}
	empty := NewBridge().Revision()
	if !isHex64(empty) {
		t.Fatalf("empty catalog revision %q is not 64 lowercase hex characters", empty)
	}
	if empty == first.Revision() {
		t.Fatal("empty catalog shares the populated catalog revision")
	}
}

// canonicalSchema must be total: malformed or empty schemas never panic and
// never contribute indistinguishable bytes.
func TestCanonicalSchemaIsTotal(t *testing.T) {
	if canonicalSchema(nil) != nil || canonicalSchema(json.RawMessage(``)) != nil {
		t.Fatal("empty schema did not canonicalize to nil")
	}
	if canonicalSchema(json.RawMessage(`{"a":`)) != nil {
		t.Fatal("malformed schema did not canonicalize to nil")
	}
	canonical := canonicalSchema(json.RawMessage(`{ "a" : 1.10 , "b" : [ 1 , 2 ] }`))
	if string(canonical) != `{"a":1.10,"b":[1,2]}` {
		t.Fatalf("canonical schema = %s; want compact form with exact numbers", canonical)
	}
}

// The recorder is test infrastructure used by the T-05 contract RED; it must
// account calls and forwarded bytes per public name without nil-map panics.
func TestUpstreamRecorderAccounting(t *testing.T) {
	recorder := &UpstreamRecorder{}
	if _, outcome := recorder.CallTool(context.Background(), "t_a", "portainer", "read", json.RawMessage(`{"limit":1}`)); outcome != nil {
		t.Fatalf("outcome = %v; want nil", outcome)
	}
	if _, outcome := recorder.CallTool(context.Background(), "t_b", "developer", "read", json.RawMessage(`{}`)); outcome != nil {
		t.Fatalf("outcome = %v; want nil", outcome)
	}
	if recorder.TotalCalls() != 2 {
		t.Fatalf("TotalCalls = %d; want 2", recorder.TotalCalls())
	}
	if recorder.ServerCalls["portainer"] != 1 || recorder.ServerCalls["developer"] != 1 {
		t.Fatalf("per-server calls = %v", recorder.ServerCalls)
	}
	if string(recorder.ForwardedArgs["t_a"]) != `{"limit":1}` {
		t.Fatalf("forwarded args = %s", recorder.ForwardedArgs["t_a"])
	}
	triggered := &UpstreamRecorder{InputRequired: true}
	if _, outcome := triggered.CallTool(context.Background(), "t_a", "portainer", "read", json.RawMessage(`{}`)); outcome == nil || !outcome.InputRequired {
		t.Fatalf("outcome = %v; want InputRequired", outcome)
	}
}

// §4.4: a safe upstream failure becomes a structured, non-retryable
// DispatchError whose execution state is not_started only when no execution
// was admitted, otherwise unknown.
func TestDispatchMapsUpstreamFailure(t *testing.T) {
	cases := []struct {
		name  string
		fail  *UpstreamFailure
		state string
	}{
		{"not admitted", &UpstreamFailure{Code: "backend_busy", ExecutionState: ExecutionNotStarted}, ExecutionNotStarted},
		{"unknown state", &UpstreamFailure{Code: "backend_unavailable", ExecutionState: ExecutionUnknown}, ExecutionUnknown},
		{"empty state fails closed", &UpstreamFailure{Code: "upstream_protocol_error"}, ExecutionUnknown},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			b := NewBridge()
			if err := b.Register("portainer", "https://portainer.example/mcp", []ToolDefinition{
				{RawName: "read", InputSchema: json.RawMessage(`{"type":"object"}`)},
			}); err != nil {
				t.Fatalf("register: %v", err)
			}
			recorder := &UpstreamRecorder{Failure: testCase.fail}
			env, derr := b.Dispatch(context.Background(), PublicName("portainer", "read"), b.Revision(), json.RawMessage(`{}`), recorder)
			if env != nil || derr == nil {
				t.Fatalf("dispatch = (%v, %v); want structured failure", env, derr)
			}
			if derr.Code != testCase.fail.Code || derr.Retryable || derr.ExecutionState != testCase.state {
				t.Fatalf("dispatch error = %+v; want code %q state %q retryable false", derr, testCase.fail.Code, testCase.state)
			}
		})
	}
}

func sha256Sum(blob []byte) []byte {
	sum := sha256.Sum256(blob)
	return sum[:]
}

func isHex64(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

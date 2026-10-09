package catalog

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

// Contract patterns pinned by r8 §4.4 (describe_tool/call_tool inputSchema).
var (
	validToolNamePattern = regexp.MustCompile(`^t_[a-z2-7]{52}$`)
	validRevisionPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

// wrongdigest: public name digest must be computed over compact canonical
// UTF-8 JSON [server_id,raw_name], full unpadded lowercase base32, no name
// case-folding/normalization (r8 §4.3, R-08).
func TestPublicNameExactDigest(t *testing.T) {
	for _, c := range []struct{ server, raw string }{
		{"portainer", "list_endpoints"},
		{"developer", "list_endpoints"},
	} {
		got := core.PublicName(c.server, c.raw)
		blob, err := json.Marshal([]string{c.server, c.raw})
		if err != nil {
			t.Fatalf("identity tuple marshal: %v", err)
		}
		sum := sha256.Sum256(blob)
		// r8 §4.4 tool_name pattern ^t_[a-z2-7]{52}$ pins the STANDARD base32
		// alphabet (RFC 4648 §6), not base32hex.
		want := "t_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:]))
		if !validToolNamePattern.MatchString(got) {
			t.Fatalf("public name %s violates §4.4 pattern ^t_[a-z2-7]{52}$", got)
		}
		if got != want {
			t.Fatalf("digest over wrong canonical form for [%s,%s]: got %s want %s", c.server, c.raw, got, want)
		}
		if len(got) != 54 {
			t.Fatalf("public name length %d", len(got))
		}
		if strings.ContainsAny(got, "ABCDEFGHIJKLMNOPQRSTUVWXYZ=") {
			t.Fatalf("charset not lowercase unpadded: %s", got)
		}
	}
}

// identicalrawnamescollapse: same raw name across servers must produce
// distinct public names (r8 §4.3; T-05 RED identicalrawnamescollapse).
func TestPublicNameDistinctServersSameRawName(t *testing.T) {
	a := core.PublicName("portainer", "list_endpoints")
	b := core.PublicName("developer", "list_endpoints")
	if a == b {
		t.Fatalf("same raw name across servers collapsed: %s", a)
	}
}

// schema-number: schema admission is number-aware. Integer admits integral
// literals only; number admits integral and fractional literals; string
// instances are never coerced into numbers (r8 §4.3 schema-number RED;
// T-01 U-03).
func TestDispatchSchemaNumberPreserved(t *testing.T) {
	probes := []struct {
		name     string
		schema   string
		instance string
		wantOK   bool
	}{
		{"integer admits integral", `{"type":"integer"}`, `1`, true},
		{"integer admits zero-fraction", `{"type":"integer"}`, `1.0`, true},
		{"integer rejects fractional", `{"type":"integer"}`, `1.1`, false},
		{"integer rejects string", `{"type":"integer"}`, `"1"`, false},
		{"number admits integral", `{"type":"number"}`, `1`, true},
		{"number admits fractional", `{"type":"number"}`, `1.1`, true},
		{"number admits trailing zeros", `{"type":"number"}`, `1.10`, true},
		{"number rejects string", `{"type":"number"}`, `"1"`, false},
		{
			"nested number property",
			`{"type":"object","additionalProperties":false,"properties":{"ratio":{"type":"number"}},"required":["ratio"]}`,
			`{"ratio":1.10}`,
			true,
		},
	}
	for _, p := range probes {
		err := ValidateSchema(p.schema, p.instance)
		if p.wantOK && err != nil {
			t.Fatalf("%s: ValidateSchema(%s, %s) = %v; want accepted", p.name, p.schema, p.instance, err)
		}
		if !p.wantOK && err == nil {
			t.Fatalf("%s: ValidateSchema(%s, %s) = nil; want rejected", p.name, p.schema, p.instance)
		}
	}
}

// Bridge RED contract: registration binds [server_id,raw_name] to the exact
// public name; dispatch admission routes to the correct server only.
func TestBridgeRegistrationAndRouting(t *testing.T) {
	b := core.NewBridge()
	defs := []core.ToolDefinition{
		{RawName: "list_endpoints", Description: "port", InputSchema: json.RawMessage(`{"type":"object"}`)},
	}
	if err := b.Register("portainer", "https://portainer.example/mcp", defs); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := b.Register("developer", "https://dev.example/mcp", defs); err != nil {
		t.Fatalf("register: %v", err)
	}
	// acrossserverwrongroute: dispatch of the portainer tool must call
	// portainer and only portainer, even though developer has the same raw
	// name. (T-05 RED acrossserverwrongroute.)
	pub := core.PublicName("portainer", "list_endpoints")
	if _, ok := b.Binding(pub); !ok {
		t.Fatalf("binding missing for %s", pub)
	}
	devPub := core.PublicName("developer", "list_endpoints")
	if devPub == pub {
		t.Fatalf("same raw name across servers collapsed to %s", pub)
	}
	rec := &core.UpstreamRecorder{}
	env, derr := b.Dispatch(context.Background(), pub, b.Revision(), json.RawMessage(`{"limit":1}`), rec)
	if derr != nil {
		t.Fatalf("dispatch: %v", derr)
	}
	// membershipleak: the envelope exposes only the bound server, and exactly
	// the addressed server is contacted (T-05 RED membershipleak).
	if env.Server != "portainer" {
		t.Fatalf("membership leak: envelope server %q; want portainer", env.Server)
	}
	if len(rec.ServerCalls) != 1 || rec.ServerCalls["portainer"] != 1 {
		t.Fatalf("wrong route: %v", rec.ServerCalls)
	}
	// The sibling server's identically named tool routes to itself.
	if _, derr := b.Dispatch(context.Background(), devPub, b.Revision(), json.RawMessage(`{"limit":1}`), rec); derr != nil {
		t.Fatalf("sibling dispatch: %v", derr)
	}
	if rec.ServerCalls["portainer"] != 1 || rec.ServerCalls["developer"] != 1 {
		t.Fatalf("cross-server route leak: %v", rec.ServerCalls)
	}
}

// collisionoverwrite: duplicate raw name within one server must reject the
// affected refresh and retain the last-known-good binding, no overwrite.
func TestBridgeCollisionRetainsLKG(t *testing.T) {
	b := core.NewBridge()
	if err := b.Register("portainer", "https://portainer.example/mcp", []core.ToolDefinition{
		{RawName: "dup", Description: "first", InputSchema: json.RawMessage(`{"type":"object"}`)},
	}); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := b.Register("portainer", "https://portainer.example/mcp", []core.ToolDefinition{
		{RawName: "dup", Description: "overwrite", InputSchema: json.RawMessage(`{"type":"string"}`)},
	}); err == nil {
		t.Fatalf("collision must reject affected refresh")
	}
	bind, ok := b.Binding(core.PublicName("portainer", "dup"))
	if !ok {
		t.Fatalf("LKG binding lost after collision")
	}
	if bind.RawName != "dup" {
		t.Fatalf("LKG binding overwritten")
	}
}

// invalidstaledispatch: dispatch with wrong revision -> catalog_changed and
// zero dispatch (r8 §4.3 invalidstaledispatch; T-05).
func TestDispatchStaleRevisionRejected(t *testing.T) {
	b := core.NewBridge()
	if err := b.Register("portainer", "https://portainer.example/mcp", []core.ToolDefinition{
		{RawName: "list_endpoints", Description: "port", InputSchema: json.RawMessage(`{"type":"object"}`)},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	rec := &core.UpstreamRecorder{}
	pub := core.PublicName("portainer", "list_endpoints")
	if _, derr := b.Dispatch(context.Background(), pub, "wrong-revision", json.RawMessage(`{"limit":1}`), rec); derr == nil {
		t.Fatalf("stale dispatch must fail")
	} else if derr.Code != "catalog_changed" {
		t.Fatalf("stale dispatch code %s", derr.Code)
	} else if derr.Retryable || derr.ExecutionState != core.ExecutionNotStarted {
		// Admission failure: no execution happened (r8 §4.4).
		t.Fatalf("stale dispatch state %+v", derr)
	}
	if got := rec.TotalCalls(); got != 0 {
		t.Fatalf("stale revision caused %d calls", got)
	}
	// unknown_tool: unknown public name -> unknown_tool, zero dispatch.
	rec = &core.UpstreamRecorder{}
	if _, derr := b.Dispatch(context.Background(), "t_unknown", b.Revision(), json.RawMessage(`{}`), rec); derr == nil || derr.Code != "unknown_tool" {
		t.Fatalf("unknown tool dispatch: %v", derr)
	} else if derr.Retryable || derr.ExecutionState != core.ExecutionNotStarted {
		t.Fatalf("unknown tool state %+v", derr)
	}
	if got := rec.TotalCalls(); got != 0 {
		t.Fatalf("unknown tool caused %d calls", got)
	}
}

// resultenvelopedataaltered: provenance envelope must expose the unaltered
// upstream CallToolResult payload (content/structuredContent/isError bytes
// unchanged) (r8 §4.4 provenance envelope; T-05 RED resultenvelopedataaltered).
func TestDispatchProvenanceEnvelopeUnaltered(t *testing.T) {
	b := core.NewBridge()
	if err := b.Register("portainer", "https://portainer.example/mcp", []core.ToolDefinition{
		{RawName: "read", Description: "r", InputSchema: json.RawMessage(`{"type":"object"}`)},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	upstream := json.RawMessage(`{"content":[{"type":"text","text":"ok"}],"structuredContent":{"key":{"nested":true}},"isError":false}`)
	rec := &core.UpstreamRecorder{CallToolResults: map[string]json.RawMessage{("portainer" + "read"): upstream}}
	env, derr := b.Dispatch(context.Background(), core.PublicName("portainer", "read"), b.Revision(), json.RawMessage(`{"limit":1,"ratio":1.10}`), rec)
	if derr != nil {
		t.Fatalf("dispatch: %v", derr)
	}
	if !bytes.Equal(env.UpstreamResult, upstream) {
		t.Fatalf("envelope altered upstream result bytes")
	}
	if env.Server != "portainer" {
		t.Fatalf("envelope provenance server %s", env.Server)
	}
	if env.CatalogRevision != b.Revision() {
		t.Fatalf("envelope provenance revision %s", env.CatalogRevision)
	}
	// r8 §4.4 catalog_revision pattern ^[a-f0-9]{64}$ applies to every
	// advertised revision (search results, describe/call admission).
	if !validRevisionPattern.MatchString(env.CatalogRevision) {
		t.Fatalf("catalog revision %s violates §4.4 pattern ^[a-f0-9]{64}$", env.CatalogRevision)
	}
	// schema-number: number precision preserved end-to-end in forwarding.
	forwarded := rec.ForwardedArgs[core.PublicName("portainer", "read")]
	if !strings.Contains(string(forwarded), `"ratio":1.10`) {
		t.Fatalf("number precision lost in forwarding: %s", forwarded)
	}
}

// pinwrong: pins resolve EXACT [server_id,raw_name]; same raw name on another
// server must not auto-resolve, and a missing pinned definition is omitted
// with safe reason (r8 §4.4 pins; T-05 RED pinwrong).
func TestPinsExactOnly(t *testing.T) {
	b := core.NewBridge()
	if err := b.Register("portainer", "https://portainer.example/mcp", []core.ToolDefinition{
		{RawName: "raw", Description: "p", InputSchema: json.RawMessage(`{"type":"object"}`)},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	// pinwrong: pin for developer.raw must not auto-resolve to portainer.raw.
	pins, err := core.AddPins(b, []core.PinnedTool{{ServerID: "developer", RawName: "raw"}})
	if err == nil {
		t.Fatalf("pin for missing developer.raw must fail with safe reason")
	}
	if got := len(pins); got != 0 {
		t.Fatalf("missing pinned definition included, got %d", got)
	}
	// pin valid: portainer.raw resolves to its exact binding.
	pins, err = core.AddPins(b, []core.PinnedTool{{ServerID: "portainer", RawName: "raw"}})
	if err != nil {
		t.Fatalf("valid pin: %v", err)
	}
	if len(pins) != 1 || pins[0].Binding == nil {
		t.Fatalf("valid pin not resolved: %v", pins)
	}
	if pins[0].Binding.ServerID != "portainer" || pins[0].Binding.RawName != "raw" {
		t.Fatalf("pin resolved to wrong binding")
	}
}

// repeat: original tools/call is never re-dispatched after execution, even
// when the upstream outcome is a replay trigger (no-replay; r8 §4.4; T-05
// RED repeat).
func TestDispatchNoReplay(t *testing.T) {
	b := core.NewBridge()
	if err := b.Register("portainer", "https://portainer.example/mcp", []core.ToolDefinition{
		{RawName: "read", Description: "r", InputSchema: json.RawMessage(`{"type":"object"}`)},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	rec := &core.UpstreamRecorder{InputRequired: true}
	pub := core.PublicName("portainer", "read")
	if _, derr := b.Dispatch(context.Background(), pub, b.Revision(), json.RawMessage(`{}`), rec); derr == nil {
		t.Fatalf("InputRequired must surface as structured error")
	} else if derr.Code != "unsupported_capability" || derr.Retryable {
		t.Fatalf("replay-shaped outcome: %+v", derr)
	} else if derr.ExecutionState != core.ExecutionUnknown {
		// One original tools/call ran before the interaction request, so a
		// partial effect is possible (r8 §4.4).
		t.Fatalf("post-execution state %+v", derr)
	}
	if rec.TotalCalls() != 1 {
		t.Fatalf("original tools/call re-dispatched: %d", rec.TotalCalls())
	}
}

// revision: the global catalog revision covers the exact definitions, so a
// schema change moves the revision while the stable public name is unchanged
// (r8 §4.3). Insignificant whitespace must not move it.
func TestCatalogRevisionTracksDefinition(t *testing.T) {
	register := func(t *testing.T, schema string) *core.Bridge {
		t.Helper()
		b := core.NewBridge()
		if err := b.Register("portainer", "https://portainer.example/mcp", []core.ToolDefinition{
			{RawName: "read", Description: "r", InputSchema: json.RawMessage(schema)},
		}); err != nil {
			t.Fatalf("register: %v", err)
		}
		return b
	}
	objectSchema := register(t, `{"type":"object"}`)
	if rev := objectSchema.Revision(); !validRevisionPattern.MatchString(rev) {
		t.Fatalf("catalog revision %q violates §4.4 pattern ^[a-f0-9]{64}$", rev)
	}
	// Whitespace-only schema differences must not move the revision.
	if objectSchema.Revision() != register(t, `{ "type" : "object" }`).Revision() {
		t.Fatalf("whitespace-only schema change moved the catalog revision")
	}
	// A real schema change must move it.
	if objectSchema.Revision() == register(t, `{"type":"string"}`).Revision() {
		t.Fatalf("schema change did not move the catalog revision")
	}
	// The public name is stable across a schema change.
	pub := core.PublicName("portainer", "read")
	if _, ok := register(t, `{"type":"string"}`).Binding(pub); !ok {
		t.Fatalf("public name %s not stable across schema change", pub)
	}
}

// execution_state vocabulary is pinned by r8 §4.4: "not_started" when no
// upstream execution occurred, "unknown" once one may have taken effect.
func TestExecutionStateVocabulary(t *testing.T) {
	if core.ExecutionNotStarted != "not_started" {
		t.Fatalf("core.ExecutionNotStarted = %q; want not_started", core.ExecutionNotStarted)
	}
	if core.ExecutionUnknown != "unknown" {
		t.Fatalf("core.ExecutionUnknown = %q; want unknown", core.ExecutionUnknown)
	}
}

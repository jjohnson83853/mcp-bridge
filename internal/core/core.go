// Package core implements the T-05 bridge contract: stable public tool names
// over exact [server_id,raw_name] identities, global catalog revision
// admission, exact routing, pin resolution, and provenance envelopes
// (r8 plan §4.3/§4.4).
package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Binding is an exact [server_id,raw_name] tool binding together with the
// exact definition it was admitted with.
type Binding struct {
	ServerID string
	RawName  string
	Def      ToolDefinition
}

// ToolDefinition is one catalog tool definition as reported by an upstream.
type ToolDefinition struct {
	RawName      string
	Description  string
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage
}

// UpstreamOutcome reports upstream-execution shape beyond a raw result.
// Exactly one of the two signals is set: InputRequired for an interaction
// request, or Failure for a safe bridge-shaped upstream failure.
type UpstreamOutcome struct {
	InputRequired bool
	Failure       *UpstreamFailure
}

// UpstreamFailure is the safe, payload-free failure an upstream facade
// reports. Codes are the §4.4 safe code set; ExecutionState is
// ExecutionNotStarted when no execution was admitted, else ExecutionUnknown.
type UpstreamFailure struct {
	Code           string
	ExecutionState string
}

// Upstream is the upstream-execution facade the bridge forwards through.
type Upstream interface {
	// CallTool forwards exactly one execution. pub is the public tool name
	// (for provenance recording); the exact raw args bytes must reach
	// upstream unchanged. Cancellation is best-effort: ctx is the inbound
	// request context and MUST reach the upstream call (plan §4.3).
	CallTool(ctx context.Context, pub, serverID, rawName string, args json.RawMessage) (json.RawMessage, *UpstreamOutcome)
}

// Envelope wraps an upstream result with bridge provenance.
type Envelope struct {
	UpstreamResult  json.RawMessage
	Server          string
	CatalogRevision string
}

// Execution-state values for bridge errors (r8 §4.4): "not_started" when no
// upstream execution occurred, "unknown" once one may have taken effect.
const (
	ExecutionNotStarted = "not_started"
	ExecutionUnknown    = "unknown"
)

// DispatchError is the structured, replay-shaped dispatch outcome.
type DispatchError struct {
	Code           string // catalog_changed | unknown_tool | unsupported_capability
	Retryable      bool
	ExecutionState string // ExecutionNotStarted | ExecutionUnknown
}

// PinnedTool names an exact tool on a specific server.
type PinnedTool struct {
	ServerID string
	RawName  string
}

// ResolvedPin is a pin resolution against the bridge's catalog.
type ResolvedPin struct {
	PinnedTool PinnedTool
	Binding    *Binding
}

// PublicName builds the canonical stable public tool name: t_ + lowercase
// unpadded RFC 4648 base32 of the full SHA256 digest over the compact
// canonical UTF-8 JSON [server_id,raw_name] (54 ASCII characters). It matches
// the §4.4 tool_name pattern ^t_[a-z2-7]{52}$: the standard base32 alphabet
// (not base32hex), no padding, no case-folding, no HTML escaping, and no name
// normalization.
func PublicName(serverID, rawName string) string {
	var canonical bytes.Buffer
	encoder := json.NewEncoder(&canonical)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode([]string{serverID, rawName}); err != nil {
		// Encoding []string cannot fail, but stay total.
		panic(fmt.Sprintf("core.PublicName: encode: %v", err))
	}
	blob := bytes.TrimSuffix(canonical.Bytes(), []byte("\n"))
	sum := sha256.Sum256(blob)
	return "t_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:]))
}

// NewBridge returns an empty bridge catalog.
func NewBridge() *Bridge {
	return &Bridge{bindings: map[string]*Binding{}, endpoint: map[string]string{}}
}

// Bridge is the in-memory [public tool name → exact binding] catalog with
// stable revision admission.
type Bridge struct {
	bindings map[string]*Binding
	endpoint map[string]string
}

// Register binds tool definitions for one upstream server. Any collision —
// a raw name already bound anywhere, a duplicate raw name within the same
// definitions slice, or a server_id rebound to a different endpoint — rejects
// the whole call and retains the last-known-good catalog unchanged.
func (b *Bridge) Register(serverID, endpoint string, defs []ToolDefinition) error {
	if existing, ok := b.endpoint[serverID]; ok && existing != endpoint {
		return fmt.Errorf("core.Register: server %q already bound to %q, rejecting rebind to %q", serverID, existing, endpoint)
	}
	candidate := map[string]*Binding{}
	for _, def := range defs {
		if def.RawName == "" {
			return fmt.Errorf("core.Register: server %q: empty raw tool name", serverID)
		}
		pub := PublicName(serverID, def.RawName)
		if _, ok := candidate[pub]; ok {
			return fmt.Errorf("core.Register: server %q: duplicate raw tool name %q", serverID, def.RawName)
		}
		if existing, ok := b.bindings[pub]; ok {
			return fmt.Errorf("core.Register: raw tool name %q already bound to server %q (LKG retained)", def.RawName, existing.ServerID)
		}
		candidate[pub] = &Binding{ServerID: serverID, RawName: def.RawName, Def: def}
	}
	for pub, bind := range candidate {
		b.bindings[pub] = bind
	}
	b.endpoint[serverID] = endpoint
	return nil
}

// Bindings lists every bound tool of the catalog in stable public-name order.
// The returned bindings are copies; mutating them never changes the catalog.
func (b *Bridge) Bindings() []Binding {
	names := make([]string, 0, len(b.bindings))
	for pub := range b.bindings {
		names = append(names, pub)
	}
	sort.Strings(names)
	bindings := make([]Binding, 0, len(names))
	for _, pub := range names {
		bindings = append(bindings, *b.bindings[pub])
	}
	return bindings
}

// Binding resolves one public tool name to its exact binding.
func (b *Bridge) Binding(pub string) (Binding, bool) {
	if bind, ok := b.bindings[pub]; ok {
		return *bind, true
	}
	return Binding{}, false
}

// revisionEntry is the canonical revision input for one binding.
type revisionEntry struct {
	Server      string          `json:"server"`
	Raw         string          `json:"raw"`
	Description string          `json:"description"`
	Input       json.RawMessage `json:"input_schema"`
	Output      json.RawMessage `json:"output_schema"`
}

// Revision returns the deterministic global catalog revision: SHA-256 over the
// sorted bound [server_id, raw_name, exact definition] set, rendered as 64
// lowercase hex characters (r8 §4.3; §4.4 catalog_revision pattern
// ^[a-f0-9]{64}$). Timestamps, availability and secrets are excluded; schema
// canonicalization ignores insignificant whitespace so only real definition
// changes move the revision.
func (b *Bridge) Revision() string {
	entries := make([]revisionEntry, 0, len(b.bindings))
	for _, bind := range b.bindings {
		entries = append(entries, revisionEntry{
			Server:      bind.ServerID,
			Raw:         bind.RawName,
			Description: bind.Def.Description,
			Input:       canonicalSchema(bind.Def.InputSchema),
			Output:      canonicalSchema(bind.Def.OutputSchema),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Server != entries[j].Server {
			return entries[i].Server < entries[j].Server
		}
		return entries[i].Raw < entries[j].Raw
	})
	blob, err := json.Marshal(entries)
	if err != nil {
		panic(fmt.Sprintf("core.Revision: marshal: %v", err))
	}
	return fmt.Sprintf("%x", sha256.Sum256(blob))
}

// DefinitionDigest is the canonical 64-hex digest of one exact tool
// definition: SHA-256 over the same compact canonical encoding the catalog
// revision hashes for a single [server_id, raw_name, exact definition] entry
// (plan §4.8/§4.9). Timestamps, availability and secrets are excluded, so the
// digest is a property of the definition alone and only a real definition
// change moves it.
func DefinitionDigest(serverID, rawName string, def ToolDefinition) string {
	entry := revisionEntry{
		Server:      serverID,
		Raw:         rawName,
		Description: def.Description,
		Input:       canonicalSchema(def.InputSchema),
		Output:      canonicalSchema(def.OutputSchema),
	}
	blob, err := json.Marshal(entry)
	if err != nil {
		panic(fmt.Sprintf("core.DefinitionDigest: marshal: %v", err))
	}
	return fmt.Sprintf("%x", sha256.Sum256(blob))
}

// canonicalSchema re-encodes a raw JSON schema in compact canonical form so
// insignificant whitespace never moves the catalog revision. JSON numbers keep
// their exact literal text. Empty or invalid input yields nil.
func canonicalSchema(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return canonical
}

// Dispatch forwards one tool execution after revision admission and exact
// binding resolution. Zero upstream calls on admission failure; exactly one
// on success and on InputRequired (never re-dispatched). Forwarded args are
// the exact raw bytes the client supplied (number precision preserved).
func (b *Bridge) Dispatch(ctx context.Context, pub, revision string, args json.RawMessage, up Upstream) (*Envelope, *DispatchError) {
	if up == nil {
		return nil, &DispatchError{Code: "unknown_tool", Retryable: false, ExecutionState: ExecutionNotStarted}
	}
	if revision != b.Revision() {
		return nil, &DispatchError{Code: "catalog_changed", Retryable: false, ExecutionState: ExecutionNotStarted}
	}
	bind, ok := b.Binding(pub)
	if !ok {
		return nil, &DispatchError{Code: "unknown_tool", Retryable: false, ExecutionState: ExecutionNotStarted}
	}
	result, outcome := up.CallTool(ctx, pub, bind.ServerID, bind.RawName, args)
	if outcome != nil && outcome.InputRequired {
		// One original tools/call already ran: a partial effect is possible,
		// so the execution state is unknown and a retry is not offered.
		return nil, &DispatchError{Code: "unsupported_capability", Retryable: false, ExecutionState: ExecutionUnknown}
	}
	if outcome != nil && outcome.Failure != nil {
		state := outcome.Failure.ExecutionState
		if state != ExecutionNotStarted {
			state = ExecutionUnknown
		}
		return nil, &DispatchError{Code: outcome.Failure.Code, Retryable: false, ExecutionState: state}
	}
	return &Envelope{
		UpstreamResult:  result,
		Server:          bind.ServerID,
		CatalogRevision: b.Revision(),
	}, nil
}

// AddPins resolves pinned prerequisite tools against the bridge's catalog.
// A missing pinned definition fails the whole call with zero resolved pins.
func AddPins(b *Bridge, pins []PinnedTool) ([]ResolvedPin, error) {
	if b == nil {
		return nil, fmt.Errorf("core.AddPins: nil bridge")
	}
	resolved := make([]ResolvedPin, 0, len(pins))
	for _, pin := range pins {
		pub := PublicName(pin.ServerID, pin.RawName)
		bind, ok := b.Binding(pub)
		if !ok {
			return nil, fmt.Errorf("core.AddPins: pinned tool [server=%q,raw=%q] not in catalog", pin.ServerID, pin.RawName)
		}
		bindCopy := bind
		resolved = append(resolved, ResolvedPin{PinnedTool: pin, Binding: &bindCopy})
	}
	return resolved, nil
}

// UpstreamRecorder is the deterministic test upstream facade.
type UpstreamRecorder struct {
	InputRequired   bool
	Failure         *UpstreamFailure
	CallToolResults map[string]json.RawMessage // key: serverID + rawName (no separator)
	ServerCalls     map[string]int             // key: serverID
	ForwardedArgs   map[string]json.RawMessage // key: public tool name
}

// CallTool implements Upstream, recording provenance and preserving exact
// forwarded-args bytes.
func (r *UpstreamRecorder) CallTool(_ context.Context, pub, serverID, rawName string, args json.RawMessage) (json.RawMessage, *UpstreamOutcome) {
	if r.ServerCalls == nil {
		r.ServerCalls = map[string]int{}
	}
	if r.ForwardedArgs == nil {
		r.ForwardedArgs = map[string]json.RawMessage{}
	}
	r.ServerCalls[serverID]++
	r.ForwardedArgs[pub] = args
	var outcome *UpstreamOutcome
	if r.InputRequired {
		outcome = &UpstreamOutcome{InputRequired: true}
	}
	if r.Failure != nil {
		outcome = &UpstreamOutcome{Failure: r.Failure}
	}
	return r.CallToolResults[serverID+rawName], outcome
}

// TotalCalls returns the recorded upstream call count across all servers.
func (r *UpstreamRecorder) TotalCalls() int {
	total := 0
	for _, n := range r.ServerCalls {
		total += n
	}
	return total
}

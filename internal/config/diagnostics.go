package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"gopkg.in/yaml.v3"
)

// This file implements the typed, metadata-only diagnostic contract of spec
// HMB-CD-v1 §4.1–§4.4. Every record carries only: the supplied config path,
// the complete known location, validated owner/reference identity, a finite
// code/property/shape class, and a static reason. It never carries a secret
// value, a preview, a hash, a fingerprint, an exact length, an offset, or any
// raw YAML/URL/OS text.

const (
	// maxDiagnostics bounds the collection: a document with more broken
	// locations is reported up to this cap with an explicit incomplete footer,
	// never a guessed total (spec §4.4).
	maxDiagnostics = 32

	// maxSecretBytes bounds one secret value at 8 KiB (spec §4.2 over_limit
	// class); the reason text discloses the limit without revealing the value.
	maxSecretBytes = 8192

	// stageNotChecked names the stages after secret resolution; a
	// secret-stage failure stops before them, and the renderer reports
	// the pinned literal `not_checked` explicitly (spec §5.1).
	stageNotChecked = "not_checked"
)


var diagnosticCodes = [...]string{
	"bound_range",
	"config_read",
	"config_too_large",
	"credential_overlap",
	"duration_positive",
	"duration_range",
	"duration_type",
	"header_duplicate",
	"header_forbidden",
	"header_mapping",
	"header_name_invalid",
	"pin_ceiling",
	"pin_duplicate",
	"pin_fields",
	"pin_server_missing",
	"pin_server_unknown",
	"pin_tool_missing",
	"pin_tool_name",
	"reference_env_name",
	"reference_invalid",
	"reference_literal_required",
	"reference_required",
	"reference_shape",
	"reference_unresolved",
	"schema_version",
	"secret_cr",
	"secret_empty",
	"secret_file_missing",
	"secret_file_policy",
	"secret_file_read",
	"secret_file_stat",
	"secret_lf",
	"secret_nul",
	"secret_too_large",
	"secret_unset",
	"server_count",
	"server_id_duplicate",
	"server_id_invalid",
	"server_unknown",
	"setting_duration",
	"setting_internal",
	"setting_missing",
	"setting_type",
	"tool_metadata_tool_name",
	"tool_metadata_tool_unknown",
	"tool_metadata_unknown_member",
	"tool_metadata_value",
	"url_invalid",
	"url_secret_parameter",
	"weight_invalid",
	"weights_invalid",
	"yaml_alias",
	"yaml_documents",
	"yaml_key_duplicate",
	"yaml_key_invalid",
	"yaml_key_unknown",
	"yaml_number",
	"yaml_root_type",
	"yaml_syntax",
	"yaml_type",
}

// DiagnosticCodes returns the closed, sorted set of every diagnostic code
// produced by this package. Callers receive their own slice.
func DiagnosticCodes() []string {
	return append([]string(nil), diagnosticCodes[:]...)
}
// Diagnostic carries the metadata-only attribution of one configuration
// rejection. Records are immutable value copies; nothing in the collection
// aliases caller-visible memory.
type Diagnostic struct {
	Path string
	// Key is the complete dotted location with zero-based sequence indices,
	// e.g. `servers[3].credentials.bearer`. Header keys use the validated
	// bracket form, e.g. `servers[0].credentials.headers["X-API-Key"]`.
	Key string
	// Line/Column locate the rejected source node when its position is available.
	Line   int
	Column int
	// Server is the validated owning server id, when the rejection is inside
	// one server's own mapping.
	Server string
	// PinServer/PinTool are the validated pin identity fields. PinServer is
	// also emitted for a pin that references an unknown server: the configured
	// identifier is safe metadata even though no server owns it.
	PinServer string
	PinTool   string
	// RefKind/RefName identify one secret reference: `env:NAME` or
	// `file:relative/path`, exactly as the document configures it (resolved
	// U-02: the configured identifier may appear; the resolved value never
	// does).
	RefKind string
	RefName string
	// RelatedKey/RelatedRefKind/RelatedRefName carry the first occurrence of
	// a duplicate (or the other side of a disjointness violation).
	RelatedKey       string
	RelatedRefKind   string
	RelatedRefName   string
	RelatedLine      int
	RelatedColumn    int
	ActualCount      int
	MaxCount         int
	FirstExcessIndex int
	// Code/Property/Shape are finite classes; Reason is a static explanation
	// chosen by this package, never decoder or OS prose.
	Code     string
	Property string
	Shape    string
	Reason   string
	Hint     string
}

// Diagnostics is the typed, errors.As-discoverable collection of records one
// rejected Parse (or rejected accessor call) produced. A non-empty collection
// is always the error itself, so every rejection attributes its failures.
type Diagnostics struct {
	Records []Diagnostic
	// Incomplete marks the >maxDiagnostics cap: reporting stopped, and the
	// renderer says so explicitly instead of guessing a total.
	Incomplete bool
	// RemainingStages names the first stage that was skipped because an
	// earlier stage failed (spec §4.4 stage barriers).
	RemainingStages string
}

func (d *Diagnostics) Error() string {
	return renderDiagnostics(d)
}

// renderDiagnostics is the single renderer for every diagnostic record this
// package produces (spec §4.1): one escaped physical line, fixed field order,
// Go-quoted strings, no physical controls.
func renderDiagnostics(d *Diagnostics) string {
	if d == nil || len(d.Records) == 0 {
		return "invalid configuration"
	}
	parts := make([]string, 0, len(d.Records))
	for _, record := range d.Records {
		parts = append(parts, renderRecord(record))
	}
	line := strings.Join(parts, " | ")
	if len(d.Records) > 1 {
		line = fmt.Sprintf("diagnostics=%d: %s", len(d.Records), line)
	}
	if d.Incomplete {
		line += "; diagnostics_incomplete=true; further locations may be invalid; fix reported errors and rerun"
	}
	if d.RemainingStages != "" {
		line += fmt.Sprintf("; remaining_stages=%s", d.RemainingStages)
	}
	return line
}

func renderRecord(record Diagnostic) string {
	parts := make([]string, 0, 14)
	parts = append(parts, fmt.Sprintf("config=%q", record.Path))
	parts = append(parts, fmt.Sprintf("key=%q", record.Key))
	if record.Line != 0 {
		parts = append(parts, fmt.Sprintf("line=%d column=%d", record.Line, record.Column))
	}
	if record.Server != "" {
		parts = append(parts, fmt.Sprintf("server=%q", record.Server))
	}
	if record.PinServer != "" {
		parts = append(parts, fmt.Sprintf("pin_server=%q", record.PinServer))
	}
	if record.PinTool != "" {
		parts = append(parts, fmt.Sprintf("pin_tool=%q", record.PinTool))
	}
	if record.RefKind != "" || record.RefName != "" {
		parts = append(parts, fmt.Sprintf("ref=%q", record.RefKind+":"+record.RefName))
	}
	if record.RelatedKey != "" {
		parts = append(parts, fmt.Sprintf("related_key=%q", record.RelatedKey))
	}
	if record.RelatedRefKind != "" || record.RelatedRefName != "" {
		parts = append(parts, fmt.Sprintf("related_ref=%q", record.RelatedRefKind+":"+record.RelatedRefName))
	}
	parts = append(parts, "code="+record.Code, "property="+record.Property, "shape="+record.Shape)
	line := strings.Join(parts, " ") + ": " + record.Reason
	if record.Hint != "" {
		line += "; " + record.Hint
	}
	return line
}

// Origin reports the supplied config path for a parsed document, or `<memory>`
// for a directly constructed Config that carries no parse origin (spec §4.1).
func (c Config) Origin() string {
	if c.origin == "" {
		return "<memory>"
	}
	return c.origin
}

// secretWalk resolves one document's secret references while collecting
// metadata-only records for every independent failure (spec §4.4: the secret
// stage reports all valid occurrences in document order, before any later
// stage runs).
type secretWalk struct {
	base       string
	configPath string
	result     map[string]string
	// fileCache caches one normalized (one-terminal-LF-trimmed) value per
	// resolved file path; an empty string marks a file whose normalized value
	// is empty.
	fileCache  map[string]string
	records    []Diagnostic
	incomplete bool
}

// record appends one metadata-only record. It returns false when the cap is
// reached and the walk must stop, which the caller reports as incomplete
// rather than guessing a total (spec §4.4).
func (w *secretWalk) record(record Diagnostic) bool {
	if len(w.records) >= maxDiagnostics {
		w.incomplete = true
		return false
	}
	w.records = append(w.records, record)
	return true
}

func (w *secretWalk) recordAt(node *yaml.Node, record Diagnostic) bool {
	if node != nil {
		record.Line, record.Column = node.Line, node.Column
	}
	return w.record(record)
}

// walk resolves every reachable secret reference below node, threading the
// dotted location and the validated server owner. It returns false once the
// diagnostics cap stops collection.
func (w *secretWalk) walk(node *yaml.Node, keyPath, server string) bool {
	if node == nil {
		return true
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if !w.walk(child, keyPath, server) {
				return false
			}
		}
		return true
	case yaml.MappingNode:
		if len(node.Content) == 2 && (node.Content[0].Value == "env" || node.Content[0].Value == "file") {
			// The embedding model/base_url env references are resolved once per
			// load by the dedicated embedding pass (spec §4.1) and are not
			// secrets; the secret stage must neither resolve nor fail them.
			if keyPath != "embedding.model" && keyPath != "embedding.base_url" {
				return w.resolve(node, keyPath, server)
			}
			return true
		}
		// A server's own mapping owns the validated id above it; every
		// credential inside it is attributed to that id (spec §3.1 R/S). Only
		// an id matching the validated server-ID rule may own the records;
		// an unsafe scalar is omitted, never echoed (spec §3.1:99/§4.1/AC-37).
		if isServerElement(keyPath) {
			if id := mappingScalar(node, "id"); id != "" && serverIDPattern.MatchString(id) {
				server = id
			}
		}
		for i := 0; i < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			childPath := key.Value
			if keyPath != "" {
				childPath = keyPath + "." + key.Value
			}
			// Header names are quoted with brackets, because a header name may
			// contain dots or spaces and the form is the validated convention
			// of spec §4.1/§4.3.
			if key.Value == "headers" && value.Kind == yaml.MappingNode {
				for h := 0; h < len(value.Content); h += 2 {
					headerPath := childPath + "[" + fmt.Sprintf("%q", value.Content[h].Value) + "]"
					if !w.walk(value.Content[h+1], headerPath, server) {
						return false
					}
				}
				continue
			}
			if !w.walk(value, childPath, server) {
				return false
			}
		}
		return true
	case yaml.SequenceNode:
		for i, child := range node.Content {
			if !w.walk(child, fmt.Sprintf("%s[%d]", keyPath, i), server) {
				return false
			}
		}
		return true
	default:
		return true
	}
}

// resolve classifies one secret reference occurrence and stores its value.
// Failures are recorded, never silently dropped, and one failing occurrence
// never stops the reporting of the others (spec §4.2/§4.4).
func (w *secretWalk) resolve(node *yaml.Node, keyPath, server string) bool {
	kind, name := node.Content[0].Value, node.Content[1].Value
	if kind == "env" {
		value, ok := os.LookupEnv(name)
		if !ok {
			return w.recordAt(node, Diagnostic{
				Path: w.configPath, Key: keyPath, Server: server,
				RefKind: kind, RefName: name,
				Code: "secret_unset", Property: "value", Shape: "unset",
				Reason: "secret environment variable is unset; set the named reference without printing its value",
			})
		}
		if diag := w.classifySecretValue(keyPath, server, kind, name, value, envLFReason); diag != nil {
			return w.recordAt(node, *diag)
		}
		w.result[kind+":"+name] = value
		return true
	}

	// File reference: stat/read before any value check (spec §4.2). The
	// configured relative path is recorded as-is; the config-dir resolution
	// stays internal and is never rendered.
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(w.base, path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		code, property, shape, reason := "secret_file_stat", "metadata", "io_error", "cannot inspect secret file"
		switch {
		case os.IsNotExist(err):
			code, property, shape, reason = "secret_file_missing", "existence", "not_found", "secret file does not exist"
		case os.IsPermission(err):
			code, property, shape, reason = "secret_file_stat", "metadata", "permission_denied", "cannot inspect secret file"
		}
		return w.recordAt(node, Diagnostic{
			Path: w.configPath, Key: keyPath, Server: server,
			RefKind: kind, RefName: name,
			Code: code, Property: property, Shape: shape, Reason: reason,
		})
	}
	if shape := secretFilePolicy(info); shape != "" {
		return w.recordAt(node, Diagnostic{
			Path: w.configPath, Key: keyPath, Server: server,
			RefKind: kind, RefName: name,
			Code: "secret_file_policy", Property: "policy", Shape: shape,
			Reason: "secret file must be regular, non-symlink, operator-owned, and private",
		})
	}
	// One shared read per path: the normalized (one-terminal-LF-trimmed) value
	// is cached after the first successful read, so a second reference to the
	// same file observes the identical value and triggers no second read and
	// no second copy (spec §4.4).
	if cached, ok := w.fileCache[path]; ok {
		return w.storeFileValue(node, keyPath, server, kind, name, cached)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		shape := "io_error"
		if os.IsPermission(err) {
			shape = "permission_denied"
		}
		return w.recordAt(node, Diagnostic{
			Path: w.configPath, Key: keyPath, Server: server,
			RefKind: kind, RefName: name,
			Code: "secret_file_read", Property: "readability", Shape: shape,
			Reason: "cannot read secret file",
		})
	}
	value := strings.TrimSuffix(string(data), "\n")
	w.fileCache[path] = value
	return w.storeFileValue(node, keyPath, server, kind, name, value)
}

// storeFileValue classifies one normalized file value (or the cached value of
// the same path) and stores it on success; every failure is recorded with the
// occurrence's own location and reference identity.
func (w *secretWalk) storeFileValue(node *yaml.Node, keyPath, server, kind, name, value string) bool {
	// File and env values share one classification; the LF reason alone
	// differs because files are newline-stripped once (spec §4.2).
	if diag := w.classifySecretValue(keyPath, server, kind, name, value, fileLFReason); diag != nil {
		return w.recordAt(node, *diag)
	}
	w.result[kind+":"+name] = value
	return true
}

// envLFReason and fileLFReason are the source-specific LF reasons: files are
// newline-stripped once, environment values are not normalized (spec §4.2).
const (
	envLFReason  = "secret contains LF (newline); environment values are not newline-stripped"
	fileLFReason = "secret contains LF (newline) after stripping one final LF; files permit only one removable final LF and no remaining CR/LF/NUL"
)

// classifySecretValue classifies one resolved secret value - env or file,
// after source normalization - into the finite §4.2 value classes. It owns
// the single copy of the empty record for both sources and returns the
// diagnostic for the first failing class, or nil when the value is
// acceptable. The LF reason is supplied by the caller; every other reason is
// static and shared.
func (w *secretWalk) classifySecretValue(keyPath, server, kind, name, value, lfReason string) *Diagnostic {
	if len(value) == 0 {
		return &Diagnostic{
			Path: w.configPath, Key: keyPath, Server: server,
			RefKind: kind, RefName: name,
			Code: "secret_empty", Property: "value", Shape: "empty",
			Reason: "secret must be nonempty; provide a nonempty value for the named reference",
		}
	}
	if len(value) > maxSecretBytes {
		return &Diagnostic{
			Path: w.configPath, Key: keyPath, Server: server,
			RefKind: kind, RefName: name,
			Code: "secret_too_large", Property: "value", Shape: "over_limit",
			Reason: "secret exceeds permitted 8KiB limit; provide a value within the configured limit",
		}
	}
	if strings.ContainsRune(value, '\r') {
		return &Diagnostic{
			Path: w.configPath, Key: keyPath, Server: server,
			RefKind: kind, RefName: name,
			Code: "secret_cr", Property: "value", Shape: "contains_cr",
			Reason: "secret contains CR (carriage return); CRLF is not normalized",
		}
	}
	if strings.ContainsRune(value, 0) {
		return &Diagnostic{
			Path: w.configPath, Key: keyPath, Server: server,
			RefKind: kind, RefName: name,
			Code: "secret_nul", Property: "value", Shape: "contains_nul",
			Reason: "secret contains NUL; remove the NUL from the credential source",
		}
	}
	if strings.ContainsRune(value, '\n') {
		shape := "contains_lf"
		if !strings.ContainsRune(value[:len(value)-1], '\n') {
			shape = "trailing_lf"
		}
		return &Diagnostic{
			Path: w.configPath, Key: keyPath, Server: server,
			RefKind: kind, RefName: name,
			Code: "secret_lf", Property: "value", Shape: shape,
			Reason: lfReason,
		}
	}
	return nil
}

// secretFilePolicy classifies one file's privacy predicates into a finite
// sub-shape (spec §4.2), replacing the old boolean helper. The predicates are
// unchanged and fail closed: symlink first, non-regular (including a nil
// FileInfo), owner unverifiable, wrong owner, then not private. It returns ""
// when every predicate passes.
func secretFilePolicy(info os.FileInfo) string {
	if info == nil {
		return "non_regular"
	}
	mode := info.Mode()
	if mode&os.ModeSymlink != 0 {
		return "symlink"
	}
	if !mode.IsRegular() {
		return "non_regular"
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "owner_unverified"
	}
	if int(stat.Uid) != os.Geteuid() {
		return "wrong_owner"
	}
	if mode.Perm()&0o077 != 0 {
		return "not_private"
	}
	return ""
}

// single wraps one Parse-stage record in the typed collection and names the
// stage barrier: every rejection before the overlap stage leaves the later
// stages unchecked (spec §4.4).
func single(record Diagnostic) *Diagnostics {
	return &Diagnostics{Records: []Diagnostic{record}, RemainingStages: stageNotChecked}
}

// direct wraps one record produced by a direct-Config accessor (Command
// settings, credentials, limits, embedding); accessors are not Parse stages,
// so no stage barrier applies.
func direct(record Diagnostic) *Diagnostics {
	return &Diagnostics{Records: []Diagnostic{record}}
}

// shapeOf names the finite YAML kind/tag class of a node — never decoder
// prose, never a supplied scalar value (spec §4.3).
func shapeOf(node *yaml.Node) string {
	if node == nil {
		return "unknown"
	}
	switch node.Kind {
	case yaml.ScalarNode:
		switch node.Tag {
		case "!!str":
			return "string"
		case "!!int":
			return "integer"
		case "!!float":
			return "number"
		case "!!bool":
			return "boolean"
		case "!!null":
			return "null"
		}
		return "scalar"
	case yaml.MappingNode:
		return "mapping"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.AliasNode:
		return "alias"
	}
	return "unknown"
}

// actualKind names the finite Go kind class of a directly constructed setting
// value — the same finite vocabulary as shapeOf, for values decoded out of a
// parsed document into `any`.
func actualKind(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case int, int64, uint64:
		return "integer"
	case float64:
		return "number"
	case []any:
		return "sequence"
	case map[string]any:
		return "mapping"
	}
	return "unknown"
}

// refParts splits one canonical `env:NAME`/`file:PATH` reference into its
// configured identity fields.
func refParts(ref string) (string, string) {
	if index := strings.Index(ref, ":"); index >= 0 {
		return ref[:index], ref[index+1:]
	}
	return ref, ""
}

// joinKey appends one mapping key to a dotted location.
func joinKey(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// keyOrDocument reports the document-level location for a record that has no
// safe known key (spec §4.1: no invented key).
func keyOrDocument(path string) string {
	if path == "" {
		return "<document>"
	}
	return path
}

func isServerElement(keyPath string) bool {
	if !strings.HasPrefix(keyPath, "servers[") {
		return false
	}
	return strings.HasSuffix(keyPath, "]") && !strings.Contains(keyPath[strings.Index(keyPath, "[")+1:], "].")
}

func mappingScalar(node *yaml.Node, key string) string {
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1].Value
		}
	}
	return ""
}

// resolveSecrets is the secret stage (spec §4.4): it resolves every reference
// it can and collects a record for every independent failure, in document
// order. A non-nil collection stops the stage chain before overlap.
func resolveSecrets(node *yaml.Node, base, configPath string, result map[string]string) *Diagnostics {
	w := &secretWalk{
		base:       base,
		configPath: configPath,
		result:     result,
		fileCache:  make(map[string]string),
	}
	if !w.walk(node, "", "") {
		w.incomplete = true
	}
	if len(w.records) == 0 {
		return nil
	}
	return &Diagnostics{
		Records:         w.records,
		Incomplete:      w.incomplete,
		RemainingStages: stageNotChecked,
	}
}

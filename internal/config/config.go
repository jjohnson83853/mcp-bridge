package config

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const maxConfigBytes = 1 << 20

// sessionIdleMinDuration and sessionIdleMaxDuration bound the SERVER SESSION
// LIFETIME key `session_idle_timeout`. It is not a request timeout: the value
// measures how long an idle session may stay alive, so the request-path
// windows (30s/120s) do not apply. The window 1s..1h admits the plan §4.2
// sample `10m` with headroom for both short-lived test sessions and
// long-running headless deployments, while keeping sessions finite
// (fail-closed: zero and negative durations are still rejected).
const sessionIdleMinDuration = 1 * time.Second
const sessionIdleMaxDuration = 1 * time.Hour

var serverIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var pinToolNamePattern = regexp.MustCompile("^[a-zA-Z0-9_-]+$")

type Server struct {
	ID              string              `yaml:"id"`
	Endpoint        string              `yaml:"endpoint"`
	Stateless       *bool               `yaml:"stateless,omitempty"`
	PromptsDisabled bool                `yaml:"prompts_disabled"`
	Credentials     map[string]any      `yaml:"credentials,omitempty"`
	Limits          map[string]any      `yaml:"limits,omitempty"`
	ToolMetadata    *ToolMetadataConfig `yaml:"tool_metadata,omitempty"`
}

func (s Server) ServerStateless() bool {
	if s.Stateless == nil {
		return true
	}
	return *s.Stateless
}

type Config struct {
	SchemaVersion     int                 `yaml:"schema_version"`
	HTTP              map[string]any      `yaml:"http"`
	Operations        map[string]any      `yaml:"operations"`
	Storage           map[string]any      `yaml:"storage"`
	Catalog           map[string]any      `yaml:"catalog"`
	Search            map[string]any      `yaml:"search"`
	Embedding         map[string]any      `yaml:"embedding"`
	ClientAuth        map[string]any      `yaml:"client_auth"`
	UpstreamDefaults  map[string]any      `yaml:"upstream_defaults"`
	Servers           []Server            `yaml:"servers"`
	Pins              []map[string]string `yaml:"pins"`
	resolved          map[string]string
	clientTokenHashes [][32]byte
	// embeddingSnapshot holds the per-load effective embedding model and base
	// URL resolved from environment references. It is populated only by Parse
	// and is treated as immutable; a directly constructed Config leaves it
	// nil, which keeps literal Config behavior unchanged (spec §4.1). It is a
	// pointer so fmt rendering of a Config shows only an address, never the
	// resolved bytes (spec §4.1, §5).
	embeddingSnapshot *embeddingSnapshotData
	// origin is the supplied config path for a parsed document, and empty for
	// a directly constructed Config (which then reports <memory>; spec §4.1).
	origin string
}

func (c Config) ClientTokenHashes() [][32]byte {
	return append([][32]byte(nil), c.clientTokenHashes...)
}

// yamlLine extracts the one safe datum a YAML syntax error carries — the line
// number — without ever echoing decoder prose (spec §4.1/§4.3).
var yamlLinePattern = regexp.MustCompile(`yaml: line (\d+):`)

func yamlSyntaxPosition(err error) (int, int) {
	if match := yamlLinePattern.FindStringSubmatch(err.Error()); match != nil {
		line, _ := strconv.Atoi(match[1])
		return line, 0
	}
	return 0, 0
}

func Parse(data []byte, configPath string) (*Config, error) {
	if len(data) > maxConfigBytes {
		return nil, single(Diagnostic{
			Path: configPath, Key: "<document>",
			Code: "config_too_large", Property: "document", Shape: "over_limit",
			Reason: "configuration exceeds the 1 MiB limit",
		})
	}
	root := yaml.Node{}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&root); err != nil {
		line, column := yamlSyntaxPosition(err)
		return nil, single(Diagnostic{
			Path: configPath, Key: "<document>", Line: line, Column: column,
			Code: "yaml_syntax", Property: "document", Shape: "invalid_yaml",
			Reason: "cannot decode YAML configuration",
		})
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, single(Diagnostic{
			Path: configPath, Key: "<document>",
			Code: "yaml_documents", Property: "document_count", Shape: "not_one",
			Reason: "configuration must contain one YAML document",
		})
	}
	if len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		shape := shapeOf(root.Content[0])
		if len(root.Content) == 1 && root.Content[0].Kind == yaml.ScalarNode {
			shape = "scalar-" + shape
		}
		return nil, single(Diagnostic{
			Path: configPath, Key: "<document>",
			Code: "yaml_root_type", Property: "root", Shape: shape,
			Reason: "configuration must be a mapping",
		})
	}
	v := &docValidator{configPath: configPath}
	if diags := v.validateNode(root.Content[0], "", "root", ""); diags != nil {
		return nil, diags
	}
	var cfg Config
	strict := yaml.NewDecoder(bytes.NewReader(data))
	strict.KnownFields(true)
	if err := strict.Decode(&cfg); err != nil {
		return nil, single(Diagnostic{
			Path: configPath, Key: "<document>",
			Code: "yaml_type", Property: "type", Shape: "schema",
			Reason: "configuration does not match the schema",
		})
	}
	if cfg.SchemaVersion != 1 {
		shape := "unsupported_integer"
		if cfg.SchemaVersion == 0 {
			shape = "missing"
		}
		return nil, single(Diagnostic{
			Path: configPath, Key: "schema_version",
			Code: "schema_version", Property: "schema_version", Shape: shape,
			Reason: "schema_version must equal 1",
		})
	}
	if len(cfg.Servers) == 0 || len(cfg.Servers) > 1024 {
		shape := "empty"
		if len(cfg.Servers) > 1024 {
			shape = "over_limit"
		}
		return nil, single(Diagnostic{
			Path: configPath, Key: "servers",
			Code: "server_count", Property: "server_count", Shape: shape,
			Reason: "servers must contain 1..1024 entries",
		})
	}
	ids := make(map[string]bool, len(cfg.Servers))
	firstIDIndex := make(map[string]int, len(cfg.Servers))
	for index, server := range cfg.Servers {
		if !serverIDPattern.MatchString(server.ID) {
			// An invalid id is withheld from the owner field entirely (spec
			// §3.1:99/§4.1/AC-37): the record locates the failure by index and
			// never echoes the unvalidated scalar.
			return nil, single(Diagnostic{
				Path: configPath, Key: fmt.Sprintf("servers[%d].id", index),
				Code: "server_id_invalid", Property: "id", Shape: "invalid_identifier",
				Reason: "invalid server id",
			})
		}
		if ids[server.ID] {
			return nil, single(Diagnostic{
				Path: configPath, Key: fmt.Sprintf("servers[%d].id", index), Server: server.ID,
				Code: "server_id_duplicate", Property: "id", Shape: "duplicate",
				Reason:     "duplicate server id",
				RelatedKey: fmt.Sprintf("servers[%d].id", firstIDIndex[server.ID]),
			})
		}
		ids[server.ID] = true
		firstIDIndex[server.ID] = index
		normalized, err := normalizeURL(server.Endpoint)
		if err != nil {
			// The owning URL is attributed by key/server id only; neither the
			// URL, a query name, nor a value is echoed (spec §3.1:183/:189,
			// §4.3).
			reason := err.Error() // static package-owned reason, never runtime text
			code, property, shape := "url_invalid", "absolute_http_url", "invalid_url"
			if strings.Contains(reason, "secret parameter") {
				code, property, shape = "url_secret_parameter", "query", "credential_parameter"
			}
			return nil, single(Diagnostic{
				Path: configPath, Key: fmt.Sprintf("servers[%d].endpoint", index), Server: server.ID,
				Code: code, Property: property, Shape: shape,
				Reason: reason,
			})
		}
		cfg.Servers[index].Endpoint = normalized
	}
	if len(cfg.Pins) > 64 {
		return nil, single(Diagnostic{
			Path: configPath, Key: "pins",
			Code: "pin_ceiling", Property: "pins", Shape: "out_of_bounds",
			Reason: "pins list may contain at most 64 entries",
		})
	}
	seenPins := make(map[string]int, len(cfg.Pins))
	for index, pin := range cfg.Pins {
		if len(pin) != 2 || pin["server"] == "" || pin["tool"] == "" || !ids[pin["server"]] {
			record := Diagnostic{
				Path: configPath, Key: fmt.Sprintf("pins[%d]", index),
				Code: "pin_fields", Property: "identity", Shape: "malformed",
				Reason: "pin must contain exactly server and tool",
			}
			switch {
			case len(pin) == 2 && pin["server"] == "":
				record = Diagnostic{
					Path: configPath, Key: fmt.Sprintf("pins[%d].server", index),
					Code: "pin_server_missing", Property: "server", Shape: "empty",
					Reason: "pin server must be nonempty",
				}
			case len(pin) == 2 && pin["tool"] == "":
				record = Diagnostic{
					Path: configPath, Key: fmt.Sprintf("pins[%d].tool", index),
					Code: "pin_tool_missing", Property: "tool", Shape: "empty",
					Reason: "pin tool must be nonempty",
				}
			case len(pin) == 2 && pin["server"] != "" && !ids[pin["server"]]:
				record = Diagnostic{
					Path: configPath, Key: fmt.Sprintf("pins[%d].server", index),
					Code: "pin_server_unknown", Property: "server", Shape: "unknown_server",
					Reason: "pin references unknown server",
					// The configured pin identity is safe metadata (U-02);
					// omitting the server owner avoids inventing a resolved owner.
					PinServer: pin["server"],
					PinTool:   pin["tool"],
				}
			}
			return nil, single(record)
		}
		tool := pin["tool"]
		if !pinToolNamePattern.MatchString(tool) {
			return nil, single(Diagnostic{
				Path: configPath, Key: fmt.Sprintf("pins[%d].tool", index),
				Code: "pin_tool_name", Property: "tool", Shape: "invalid_chars",
				Reason:    "pin tool name must contain only standard identifier characters",
				PinServer: pin["server"],
				PinTool:   tool,
			})
		}
		if len(tool) > 94 || len(pin["server"])+2+len(tool) > 128 {
			return nil, single(Diagnostic{
				Path: configPath, Key: fmt.Sprintf("pins[%d].tool", index),
				Code: "pin_tool_name", Property: "tool", Shape: "too_long",
				Reason:    "pin tool name exceeds 94 characters or wire name exceeds 128 characters",
				PinServer: pin["server"],
				PinTool:   tool,
			})
		}
		key := pin["server"] + "\x00" + pin["tool"]
		if first, duplicate := seenPins[key]; duplicate {
			return nil, single(Diagnostic{
				Path: configPath, Key: fmt.Sprintf("pins[%d]", index),
				Code: "pin_duplicate", Property: "identity", Shape: "duplicate",
				Reason:     "duplicate pin",
				PinServer:  pin["server"],
				PinTool:    pin["tool"],
				RelatedKey: fmt.Sprintf("pins[%d]", first),
			})
		}
		seenPins[key] = index
	}
	// Exact-tool metadata exceptions must name configured pins of their own
	// server; the join runs once pins are fully validated (spec ticket-05).
	for index, server := range cfg.Servers {
		if server.ToolMetadata == nil {
			continue
		}
		if err := server.ToolMetadata.ValidatePins(server.ID, cfg.Pins); err != nil {
			return nil, single(Diagnostic{
				Path: configPath,
				Key:  fmt.Sprintf("servers[%d].tool_metadata.tools", index), Server: server.ID,
				Code: "tool_metadata_tool_unknown", Property: "tool_metadata", Shape: "unknown_tool",
				Reason: err.Error(),
			})
		}
	}
	if diags := validateBounds(root.Content[0], configPath); diags != nil {
		return nil, diags
	}
	if diags := validateWeights(root.Content[0], configPath); diags != nil {
		return nil, diags
	}
	// Embedding environment references (spec §4.1): strict shape/name checks
	// and one LookupEnv per reference run after document shape validation and
	// before effective field validation, secret resolution, or any consumer.
	// Resolved bytes live only in the private per-load snapshot below; the
	// raw public embedding map keeps the configured reference objects.
	if embeddingSnapshot, diags := resolveEmbeddingEnv(mappingChild(root.Content[0], "embedding"), configPath); diags != nil {
		return nil, diags
	} else {
		cfg.embeddingSnapshot = &embeddingSnapshotData{values: embeddingSnapshot}
	}
	if baseURL := findScalar(root.Content[0], "base_url"); baseURL != "" {
		if err := validateURL(baseURL); err != nil {
			// The owning URL is attributed by key only; neither the URL, a
			// query name, nor a value is echoed (spec §3.1:183/:189, §4.3).
			reason := err.Error() // static package-owned reason, never runtime text
			code, property, shape := "url_invalid", "absolute_http_url", "invalid_url"
			if strings.Contains(reason, "secret parameter") {
				code, property, shape = "url_secret_parameter", "query", "credential_parameter"
			}
			return nil, single(Diagnostic{
				Path: configPath, Key: "embedding.base_url",
				Code: code, Property: property, Shape: shape,
				Reason: reason,
			})
		}
	}
	if directory, ok := cfg.Storage["directory"].(string); ok && !filepath.IsAbs(directory) {
		cfg.Storage["directory"] = filepath.Clean(filepath.Join(filepath.Dir(configPath), directory))
	}
	resolved := make(map[string]string)
	if diags := resolveSecrets(root.Content[0], filepath.Dir(configPath), configPath, resolved); diags != nil {
		return nil, diags
	}
	// Overlap stage (spec §4.2/§4.4): the first document-order client/backend
	// pair that shares a reference or a resolved value is reported as one
	// record naming both references — never a value, a hash, or the equality
	// mechanism. Overlap is the final parse stage, so no stage footer applies.
	var clientOccs, backendOccs []refOccurrence
	for i := 0; i < len(root.Content[0].Content); i += 2 {
		key, value := root.Content[0].Content[i], root.Content[0].Content[i+1]
		if key.Value == "client_auth" {
			clientOccs = collectRefOccurrences(value, "client_auth", clientOccs)
		} else {
			backendOccs = collectRefOccurrences(value, key.Value, backendOccs)
		}
	}
	backendFirstRef := make(map[string]int, len(backendOccs))
	backendFirstValue := make(map[string]int, len(backendOccs))
	for index, occurrence := range backendOccs {
		if _, seen := backendFirstRef[occurrence.reference]; !seen {
			backendFirstRef[occurrence.reference] = index
		}
		if value, ok := resolved[occurrence.reference]; ok {
			if _, seen := backendFirstValue[value]; !seen {
				backendFirstValue[value] = index
			}
		}
	}
	for _, occurrence := range clientOccs {
		value, ok := resolved[occurrence.reference]
		if !ok {
			// Unreachable after the secret stage, which has resolved every
			// collected reference or rejected the document; fail closed anyway.
			kind, name := refParts(occurrence.reference)
			return nil, single(Diagnostic{
				Path: configPath, Key: occurrence.location,
				RefKind: kind, RefName: name,
				Code: "reference_unresolved", Property: "reference", Shape: "unresolved",
				Reason: "secret reference does not resolve",
			})
		}
		related, overlap := backendFirstRef[occurrence.reference]
		if !overlap {
			if first, equal := backendFirstValue[value]; equal {
				related, overlap = first, true
			}
		}
		if !overlap {
			cfg.clientTokenHashes = append(cfg.clientTokenHashes, sha256.Sum256([]byte(value)))
			continue
		}
		other := backendOccs[related]
		kind, name := refParts(occurrence.reference)
		relatedKind, relatedName := refParts(other.reference)
		return nil, &Diagnostics{Records: []Diagnostic{{
			Path:       configPath,
			Key:        occurrence.location,
			RefKind:    kind,
			RefName:    name,
			RelatedKey: other.location,

			RelatedRefKind: relatedKind,
			RelatedRefName: relatedName,
			Code:           "credential_overlap",
			Property:       "identity",
			Shape:          "overlap",
			Reason:         "client tokens and backend credentials must be disjoint",
		}}}
	}
	cfg.resolved = resolved
	cfg.origin = configPath
	return &cfg, nil
}

// embeddingEnvFields names the only embedding settings that accept an unquoted
// native {env: NAME} reference (spec §2 R-35). Every other field keeps its
// existing literal/reference grammar.
var embeddingEnvFields = set("model base_url")

// embeddingSnapshotData wraps the per-load resolved effective values. The
// indirection keeps resolved bytes out of any fmt rendering of Config: only
// an address is shown, never the model or URL strings (spec §4.1, §5).
type embeddingSnapshotData struct {
	values map[string]string
}

// resolveEmbeddingEnv validates the two embedding env-reference fields before
// any consumer runs (spec §4.1). A literal string keeps the existing grammar;
// a mapping value must be exactly {env: NAME} with a valid identifier name.
// The name is validated (shape, then expression syntax, then identifier) before
// os.LookupEnv, and a missing or exactly-empty value is a distinct static
// failure, so no partial or lazy resolution is possible (spec §5).
func resolveEmbeddingEnv(embedding *yaml.Node, configPath string) (map[string]string, *Diagnostics) {
	if embedding == nil {
		return nil, nil
	}
	var snapshot map[string]string
	for i := 0; i+1 < len(embedding.Content); i += 2 {
		key, value := embedding.Content[i], embedding.Content[i+1]
		if !embeddingEnvFields[key.Value] {
			continue
		}
		if value.Kind != yaml.MappingNode {
			continue // literal strings keep their existing grammar and paths
		}
		field := key.Value
		effective, diags := resolveEmbeddingRef(value, configPath, joinKey("embedding", field))
		if diags != nil {
			return nil, diags
		}
		if field == "base_url" {
			// The effective URL is validated before any consumer sees the
			// snapshot (spec §4.3/§5): whitespace and control bytes fail the
			// absolute-HTTP(S) class without being trimmed or rewritten.
			if err := validateEffectiveURL(effective); err != nil {
				return nil, single(Diagnostic{
					Path: configPath, Key: "embedding.base_url",
					Code: "url_invalid", Property: "absolute_http_url", Shape: "invalid_url",
					Reason: err.Error(),
				})
			}
		}
		// Allocated lazily with capacity for the two reference fields only, so
		// a document with no native references never pays for the map.
		if snapshot == nil {
			snapshot = make(map[string]string, 2)
		}
		snapshot[field] = effective
	}
	return snapshot, nil
}

// resolveEmbeddingRef classifies one {env: NAME} embedding reference. Shape
// errors precede lookup; a malformed reference name is withheld from the
// reason, and the missing versus empty distinction is static metadata only
// (spec §5).
func resolveEmbeddingRef(node *yaml.Node, configPath, keyPath string) (string, *Diagnostics) {
	if node.Kind != yaml.MappingNode || len(node.Content) != 2 || node.Content[0].Value != "env" {
		return "", single(Diagnostic{
			Path: configPath, Key: keyPath,
			Code: "reference_shape", Property: "reference", Shape: shapeOf(node),
			Reason: "embedding value must be a literal string or an env reference",
		})
	}
	key, value := node.Content[0], node.Content[1]
	if key.Kind != yaml.ScalarNode || value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
		return "", single(Diagnostic{
			Path: configPath, Key: keyPath,
			Code: "reference_shape", Property: "reference", Shape: shapeOf(node),
			Reason: "embedding value must be a literal string or an env reference",
		})
	}
	name := value.Value
	// Expression syntax is classified before the generic invalid-name check so
	// an interpolated or command-like name is rejected with its own record
	// (spec §5); the rejected name itself is never echoed.
	if strings.ContainsAny(name, "$`") {
		return "", single(Diagnostic{
			Path: configPath, Key: keyPath,
			Code: "reference_literal_required", Property: "reference", Shape: "literal",
			Reason: "environment references cannot contain interpolation or commands",
		})
	}
	if !envNamePattern.MatchString(name) {
		return "", single(Diagnostic{
			Path: configPath, Key: keyPath,
			Code: "reference_env_name", Property: "name", Shape: "invalid_identifier",
			Reason: "invalid environment variable name (name withheld)",
		})
	}
	resolved, present := os.LookupEnv(name)
	if !present {
		return "", single(Diagnostic{
			Path: configPath, Key: keyPath,
			Code: "reference_unresolved", Property: "reference", Shape: "missing",
			Reason: "environment variable is not set",
		})
	}
	if resolved == "" {
		return "", single(Diagnostic{
			Path: configPath, Key: keyPath,
			Code: "reference_unresolved", Property: "reference", Shape: "empty",
			Reason: "environment variable is empty",
		})
	}
	return resolved, nil
}

func ParseFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		shape := "io_error"
		if os.IsNotExist(err) {
			shape = "not_found"
		} else if os.IsPermission(err) {
			shape = "permission_denied"
		}
		// The requested path is the supplied config path; the OS error text is
		// never echoed (spec §4.3).
		return nil, single(Diagnostic{
			Path: path, Key: "<document>",
			Code: "config_read", Property: "document", Shape: shape,
			Reason: "cannot read configuration file",
		})
	}
	return Parse(data, path)
}

func (c Config) String() string {
	names := make([]string, 0, len(c.resolved))
	for ref := range c.resolved {
		names = append(names, ref)
	}
	sort.Strings(names)
	return fmt.Sprintf("Config{schema_version:%d servers:%d secret_refs:%v}", c.SchemaVersion, len(c.Servers), names)
}

// GoString renders Config for the %#v verb with the same redaction as String
// so no fmt verb can echo resolved environment bytes or secret values
// (spec §4.1 byte fidelity).
func (c Config) GoString() string {
	return c.String()
}

func normalizeURL(raw string) (string, error) {
	u, err := url.ParseRequestURI(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Fragment != "" || strings.Contains(raw, "#") {
		return "", fmt.Errorf("invalid absolute HTTP(S) URL")
	}
	for key := range u.Query() {
		lower := strings.ToLower(key)
		for _, marker := range []string{"token", "secret", "password", "key", "authorization", "credential", "signature"} {
			if strings.Contains(lower, marker) {
				return "", fmt.Errorf("URL query contains a secret parameter")
			}
		}
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	return u.String(), nil
}

func validateURL(raw string) error {
	_, err := normalizeURL(raw)
	return err
}

// validateEffectiveURL validates an env-resolved embedding base URL. It first
// rejects every ASCII whitespace and control byte in the raw resolved string
// — including the space that ordinary URL parsing tolerates — so reference
// resolution never trims or rewrites the effective value (spec §4.3).
func validateEffectiveURL(raw string) error {
	for _, b := range []byte(raw) {
		if b == ' ' || b < 0x20 || b == 0x7f {
			return fmt.Errorf("invalid absolute HTTP(S) URL")
		}
	}
	return validateURL(raw)
}

var fields = map[string]map[string]bool{
	"root":              set("schema_version http operations storage catalog search embedding client_auth upstream_defaults servers pins"),
	"http":              set("listen allowed_hosts allowed_origins max_request_bytes requests_per_second burst max_sessions session_idle_timeout"),
	"operations":        set("listen requests_per_second burst"),
	"storage":           set("directory max_snapshot_bytes"),
	"catalog":           set("refresh_interval stale_after max_tools_per_server max_tools_total max_pages_per_server max_schema_bytes max_description_bytes"),
	"search":            set("default_top_k max_top_k timeout query_cache_entries query_cache_ttl keyword_weight embedding_weight rrf_constant"),
	"embedding":         set("provider model model_revision base_url dimensions api_key timeout max_concurrency batch_size max_response_bytes"),
	"client_auth":       set("bearer_tokens"),
	"upstream_defaults": set("connect_timeout call_timeout refresh_timeout max_concurrent_calls max_event_bytes max_response_bytes breaker"),
	"server":            set("id endpoint prompts_disabled credentials limits tool_metadata stateless"), "credentials": set("bearer headers"),
	"tool_metadata": set("defaults tools"),
	"defaults":      set("description title icons annotations _meta"),
	"limits":        set("max_concurrent_calls call_timeout"), "breaker": set("consecutive_failures open_duration half_open_probes"),
	"pin": set("server tool"), "secret": set("env file"),
}

func set(words string) map[string]bool {
	result := map[string]bool{}
	for _, word := range strings.Fields(words) {
		result[word] = true
	}
	return result
}

// docValidator carries the parse context every structural/type record needs:
// the supplied config path (spec §4.1). It rejects at the first record, like
// the previous error-based walk, and every record carries the stage barrier
// via single().
type docValidator struct {
	configPath string
}

func (v *docValidator) fail(record Diagnostic) *Diagnostics {
	record.Path = v.configPath
	return single(record)
}

// mappingOnlySections are the schema sections that must be YAML mappings; a
// sequence or scalar in their place is attributed before the typed decode so
// the record names the section from the parsed node, never decoder prose
// (spec §5.2 C-1 wrong-section-type row).
var mappingOnlySections = set("http operations storage catalog search embedding client_auth upstream_defaults credentials limits breaker headers tool_metadata defaults tools")

func (v *docValidator) validateNode(node *yaml.Node, path, section, server string) *Diagnostics {
	if section == "headers" {
		return v.validateHeaders(node, path, server)
	}
	if section == "secret" {
		return v.validateSecretRef(node, path, server)
	}
	if node.Kind != yaml.MappingNode {
		return v.fail(Diagnostic{
			Key: keyOrDocument(path), Code: "yaml_type", Property: "type", Shape: shapeOf(node),
			Reason: "expected mapping",
		})
	}
	// firstKey records the first occurrence of a mapping key: its dotted path
	// and parser position, so a duplicate can name the first occurrence
	// without echoing the raw key text (spec §3.1:256).
	type firstKey struct {
		path   string
		line   int
		column int
	}
	seen := map[string]firstKey{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		childPath := joinKey(path, key.Value)
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Value == "<<" {
			shape := "non_string"
			switch {
			case key.Value == "<<":
				shape = "merge"
			case key.Kind == yaml.AliasNode:
				shape = "alias"
			}
			return v.fail(Diagnostic{
				Key: keyOrDocument(path), Line: key.Line, Column: key.Column,
				Code: "yaml_key_invalid", Property: "key", Shape: shape,
				Reason: "aliases, merge keys, and non-string keys are not allowed",
			})
		}
		if first, duplicate := seen[key.Value]; duplicate {
			// The duplicate names the first occurrence: its location as
			// RelatedKey and its parser position, never the raw key text
			// (spec §3.1:256).
			return v.fail(Diagnostic{
				Key: childPath, RelatedKey: first.path,
				Line: first.line, Column: first.column,
				Code: "yaml_key_duplicate", Property: "key", Shape: "duplicate",
				Reason: "duplicate key",
			})
		}
		seen[key.Value] = firstKey{path: childPath, line: key.Line, column: key.Column}
		if section == "tool_metadata" && key.Value == "tools" {
			if value.Kind != yaml.MappingNode {
				return v.fail(Diagnostic{
					Key: childPath, Server: server, Line: value.Line, Column: value.Column,
					Code: "tool_metadata_value", Property: "tool_metadata", Shape: shapeOf(value),
					Reason: "tools must be a mapping of exact raw tool names to patches",
				})
			}
			for i := 0; i+1 < len(value.Content); i += 2 {
				name := value.Content[i]
				if name.Kind != yaml.ScalarNode || name.Tag != "!!str" || !pinToolNamePattern.MatchString(name.Value) {
					return v.fail(Diagnostic{
						Key: childPath, Server: server, Line: name.Line, Column: name.Column,
						Code: "tool_metadata_tool_name", Property: "tool_metadata", Shape: shapeOf(name),
						Reason: "tools entry name must be a nonempty standard identifier",
					})
				}
				if err := v.validateNode(value.Content[i+1], joinKey(childPath, name.Value), "defaults", server); err != nil {
					return err
				}
			}
			continue
		}
		if section == "defaults" {
			if err := v.validateDefaultsValue(childPath, key.Value, value, server); err != nil {
				return err
			}
			continue
		}
		allowed, ok := fields[section]
		if !ok || !allowed[key.Value] {
			// The offending key text is withheld; the parent location and the
			// node position locate it without echoing unsafe input (spec §5.2).
			return v.fail(Diagnostic{
				Key: keyOrDocument(path), Line: key.Line, Column: key.Column,
				Code: "yaml_key_unknown", Property: "unknown_key", Shape: "unknown_key",
				Reason: "unknown key",
			})
		}
		if value.Kind == yaml.AliasNode {
			return v.fail(Diagnostic{
				Key: childPath, Code: "yaml_alias", Property: "value", Shape: "alias",
				Reason: "YAML aliases are not allowed",
			})
		}
		if value.Kind == yaml.ScalarNode && value.Tag == "!!float" {
			var number float64
			if err := value.Decode(&number); err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
				return v.fail(Diagnostic{
					Key: childPath, Code: "yaml_number", Property: "value", Shape: "nonfinite",
					Reason: "non-finite numbers are not allowed",
				})
			}
		}
		next := childSection(section, key.Value)
		if mappingOnlySections[next] && value.Kind != yaml.MappingNode {
			return v.fail(Diagnostic{
				Key: childPath, Code: "yaml_type", Property: "type", Shape: shapeOf(value),
				Reason: "must be a mapping",
			})
		}
		if (section == "credentials" && key.Value == "bearer" || section == "embedding" && key.Value == "api_key") && value.Kind != yaml.MappingNode {
			return v.fail(Diagnostic{
				Key: childPath, Server: server, Code: "reference_required", Property: "reference", Shape: shapeOf(value),
				Reason: "secret must use an env or file reference",
			})
		}
		if err := v.validateScalarType(childPath, section, key.Value, value, server); err != nil {
			return err
		}
		if value.Kind == yaml.MappingNode {
			childServer := server
			if section == "root" && key.Value == "servers" {
				// Only an id matching the validated server-ID rule may own the
				// child records; an unsafe scalar is omitted, never echoed
				// (spec §3.1:99/§4.1/AC-37).
				if id := mappingScalar(value, "id"); id != "" && serverIDPattern.MatchString(id) {
					childServer = id
				}
			}
			// A native embedding env reference mapping ({env: NAME}) will be
			// validated by the dedicated embedding pass (shape, keys, name
			// expression, spec §4.1); the generic section walk must not
			// descend into it or it would report `env` as an unknown key.
			if section == "embedding" && embeddingEnvFields[key.Value] {
				continue
			}
			if err := v.validateNode(value, childPath, next, childServer); err != nil {
				return err
			}
		}
		if value.Kind == yaml.SequenceNode {
			for index, child := range value.Content {
				elemPath := fmt.Sprintf("%s[%d]", childPath, index)
				childServer := server
				if next == "server" && child.Kind == yaml.MappingNode {
					// Only an id matching the validated server-ID rule may own
					// the child records; an unsafe scalar is omitted, never
					// echoed (spec §3.1:99/§4.1/AC-37).
					if id := mappingScalar(child, "id"); id != "" && serverIDPattern.MatchString(id) {
						childServer = id
					}
				}
				if child.Kind == yaml.AliasNode {
					return v.fail(Diagnostic{
						Key: elemPath, Code: "yaml_alias", Property: "value", Shape: "alias",
						Reason: "YAML aliases are not allowed",
					})
				}
				if next == "secret" && child.Kind != yaml.MappingNode {
					return v.fail(Diagnostic{
						Key: elemPath, Server: childServer, Code: "reference_required", Property: "reference", Shape: shapeOf(child),
						Reason: "secret must be a reference",
					})
				}
				if child.Kind != yaml.MappingNode && (next == "server" || next == "pin") {
					// A sequence element that must be a mapping is attributed
					// to its indexed location instead of falling through to
					// the generic schema record (spec §3.1:247).
					return v.fail(Diagnostic{
						Key: elemPath, Code: "yaml_type", Property: "type", Shape: shapeOf(child),
						Reason: "expected mapping",
					})
				}
				if child.Kind == yaml.MappingNode {
					if err := v.validateNode(child, elemPath, next, childServer); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func childSection(section, key string) string {
	switch {
	case section == "root" && fields[key] != nil:
		return key
	case section == "root" && key == "servers":
		return "server"
	case section == "root" && key == "pins":
		return "pin"
	case section == "server" && key == "credentials":
		return "credentials"
	case section == "server" && key == "limits":
		return "limits"
	case section == "server" && key == "tool_metadata":
		return "tool_metadata"
	case section == "tool_metadata" && key == "defaults":
		return "defaults"
	case section == "upstream_defaults" && key == "breaker":
		return "breaker"
	case section == "credentials" && key == "bearer":
		return "secret"
	case section == "embedding" && key == "api_key":
		return "secret"
	case section == "client_auth" && key == "bearer_tokens":
		return "secret"
	case section == "credentials" && key == "headers":
		return "headers"
	default:
		return "leaf"
	}
}

func (v *docValidator) validateScalarType(path, section, key string, node *yaml.Node, server string) *Diagnostics {
	stringFields := set("listen directory session_idle_timeout refresh_interval stale_after timeout query_cache_ttl provider model model_revision base_url id endpoint bearer env file tool server connect_timeout call_timeout refresh_timeout open_duration")
	if section == "server" && key == "prompts_disabled" && (node.Kind != yaml.ScalarNode || node.Tag != "!!bool") {
		return v.fail(Diagnostic{
			Key: path, Server: server, Code: "yaml_type", Property: "type", Shape: shapeOf(node),
			Reason: "prompts_disabled must be a boolean",
		})
	}
	integerFields := set("schema_version max_request_bytes requests_per_second burst max_sessions max_snapshot_bytes max_tools_per_server max_tools_total max_pages_per_server max_schema_bytes max_description_bytes default_top_k max_top_k query_cache_entries rrf_constant dimensions max_concurrency batch_size max_response_bytes max_concurrent_calls max_event_bytes consecutive_failures half_open_probes")
	sequenceFields := set("allowed_hosts allowed_origins bearer_tokens")
	if (section == "credentials" && key == "bearer" || section == "embedding" && key == "api_key" || section == "client_auth" && key == "bearer_tokens" || section == "secret" && (key == "env" || key == "file")) && node.Kind == yaml.MappingNode || section == "client_auth" && key == "bearer_tokens" && node.Kind == yaml.SequenceNode {
		return nil
	}
	if section == "secret" && (key == "env" || key == "file") && node.Kind == yaml.ScalarNode && node.Tag == "!!str" {
		return nil
	}
	if key == "bearer" && section == "credentials" || key == "api_key" && section == "embedding" || key == "bearer_tokens" && section == "client_auth" {
		return v.fail(Diagnostic{
			Key: path, Server: server, Code: "reference_required", Property: "reference", Shape: shapeOf(node),
			Reason: "must use secret references",
		})
	}
	if section == "root" && key == "schema_version" && node.Kind == yaml.ScalarNode && node.Tag == "!!int" {
		return nil
	}
	// The two embedding env-reference fields are validated by the dedicated
	// resolution pass (spec §4.1); the string shape check must not reject
	// their strict {env: NAME} mapping before that pass runs.
	if stringFields[key] && (node.Kind != yaml.ScalarNode || node.Tag != "!!str") &&
		!(section == "embedding" && embeddingEnvFields[key] && node.Kind == yaml.MappingNode) {
		return v.fail(Diagnostic{
			Key: path, Server: server, Code: "yaml_type", Property: "type", Shape: shapeOf(node),
			Reason: "must be a string",
		})
	}
	if integerFields[key] && (node.Kind != yaml.ScalarNode || node.Tag != "!!int") {
		return v.fail(Diagnostic{
			Key: path, Server: server, Code: "yaml_type", Property: "type", Shape: shapeOf(node),
			Reason: "must be an integer",
		})
	}
	if (key == "keyword_weight" || key == "embedding_weight") && (node.Kind != yaml.ScalarNode || node.Tag != "!!float" && node.Tag != "!!int") {
		return v.fail(Diagnostic{
			Key: path, Server: server, Code: "yaml_type", Property: "type", Shape: shapeOf(node),
			Reason: "must be numeric",
		})
	}
	if sequenceFields[key] && node.Kind != yaml.SequenceNode {
		return v.fail(Diagnostic{
			Key: path, Server: server, Code: "yaml_type", Property: "type", Shape: shapeOf(node),
			Reason: "must be a sequence",
		})
	}
	if section == "http" && (key == "allowed_hosts" || key == "allowed_origins") {
		for index, value := range node.Content {
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				return v.fail(Diagnostic{
					Key: fmt.Sprintf("%s[%d]", path, index), Server: server,
					Code: "yaml_type", Property: "type", Shape: shapeOf(value),
					Reason: "entries must be strings",
				})
			}
		}
	}
	if section == "root" && node.Kind != yaml.MappingNode && node.Kind != yaml.SequenceNode && key != "schema_version" {
		return v.fail(Diagnostic{
			Key: path, Server: server, Code: "yaml_type", Property: "type", Shape: shapeOf(node),
			Reason: "must be a structured value",
		})
	}
	return nil
}

// validateSecretRef classifies one {env: NAME}/{file: PATH} reference node;
// the configured reference identity is safe metadata, but a rejected env name
// or interpolated path is withheld (spec §4.3/§5.2 F-01).
func (v *docValidator) validateSecretRef(node *yaml.Node, path, server string) *Diagnostics {
	if node.Kind != yaml.MappingNode || len(node.Content) != 2 || (node.Content[0].Value != "env" && node.Content[0].Value != "file") {
		return v.fail(Diagnostic{
			Key: path, Server: server, Code: "reference_shape", Property: "reference", Shape: shapeOf(node),
			Reason: "secret must be exactly {env: NAME} or {file: PATH}",
		})
	}
	key, value := node.Content[0], node.Content[1]
	if key.Kind != yaml.ScalarNode || value.Kind != yaml.ScalarNode || value.Tag != "!!str" || value.Value == "" {
		// A malformed or wrongly typed reference identity is withheld; the
		// shape names only the finite cause class (spec §3.1:236).
		shape := "wrong_type"
		switch {
		case value.Kind == yaml.ScalarNode && value.Tag == "!!null":
			shape = "missing"
		case value.Kind == yaml.ScalarNode && value.Tag == "!!str" && value.Value == "":
			shape = "empty"
		}
		return v.fail(Diagnostic{
			Key: path, Server: server, Code: "reference_invalid", Property: "reference", Shape: shape,
			Reason: "invalid secret reference",
		})
	}
	if key.Value == "env" && !envNamePattern.MatchString(value.Value) {
		return v.fail(Diagnostic{
			Key: path, Server: server, Code: "reference_env_name", Property: "reference", Shape: "invalid_identifier",
			Reason: "invalid environment variable name",
		})
	}
	if strings.ContainsAny(value.Value, "$`\r\n\x00") {
		return v.fail(Diagnostic{
			Key: path, Server: server, Code: "reference_literal_required", Property: "reference", Shape: "literal",
			Reason: "secret references cannot contain interpolation or commands",
		})
	}
	return nil
}

func (v *docValidator) validateHeaders(node *yaml.Node, path, server string) *Diagnostics {
	if node.Kind != yaml.MappingNode {
		return v.fail(Diagnostic{
			Key: path, Server: server, Code: "header_mapping", Property: "type", Shape: shapeOf(node),
			Reason: "credentials.headers must be a mapping",
		})
	}
	seen := make(map[string]string, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		header := node.Content[i]
		entryPath := path + "[" + fmt.Sprintf("%q", header.Value) + "]"
		if header.Kind != yaml.ScalarNode || header.Tag != "!!str" || !validHeaderFieldName(header.Value) {
			// The invalid header text is withheld; the header-mapping parent
			// location and the key node position locate it (spec §5.2 F-01).
			return v.fail(Diagnostic{
				Key: path, Line: header.Line, Column: header.Column, Server: server,
				Code: "header_name_invalid", Property: "name", Shape: "invalid_identifier",
				Reason: "invalid credential header name",
			})
		}
		key := strings.ToLower(header.Value)
		if first, exists := seen[key]; exists {
			return v.fail(Diagnostic{
				Key: entryPath, RelatedKey: first, Server: server,
				Code: "header_duplicate", Property: "name", Shape: "casefold",
				Reason: "duplicate credential header",
			})
		}
		seen[key] = entryPath
		for _, forbidden := range []string{"authorization", "host", "cookie", "origin", "mcp-session-id", "mcp-protocol-version", "x-request-id", "connection", "proxy-", "transfer-encoding", "upgrade", "keep-alive", "te", "trailer"} {
			if key == forbidden || strings.HasPrefix(forbidden, "proxy-") && strings.HasPrefix(key, forbidden) {
				return v.fail(Diagnostic{
					Key: entryPath, Server: server,
					Code: "header_forbidden", Property: "name", Shape: "forbidden",
					Reason: "credential header is forbidden",
				})
			}
		}
		value := node.Content[i+1]
		if value.Kind != yaml.MappingNode {
			return v.fail(Diagnostic{
				Key: entryPath, Server: server, Code: "reference_required", Property: "reference", Shape: shapeOf(value),
				Reason: "credential header value must use a secret reference",
			})
		}
		if err := v.validateNode(value, entryPath, "secret", server); err != nil {
			return err
		}
	}
	return nil
}

func validHeaderFieldName(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", char)) {
			return false
		}
	}
	return true
}

// refOccurrence is one configured secret-reference occurrence with the dotted
// location it appears at (header names in the validated bracket form). The
// overlap stage reports the first document-order pair (spec §4.2).
type refOccurrence struct {
	reference string
	location  string
}

func collectRefOccurrences(node *yaml.Node, path string, occurrences []refOccurrence) []refOccurrence {
	if node == nil {
		return occurrences
	}
	if node.Kind == yaml.MappingNode {
		if len(node.Content) == 2 && (node.Content[0].Value == "env" || node.Content[0].Value == "file") {
			// The embedding model/base_url env references are not secrets and
			// are already resolved by the dedicated embedding pass; the overlap
			// stage never sees them (spec §4.1).
			if path == "embedding.model" || path == "embedding.base_url" {
				return occurrences
			}
			return append(occurrences, refOccurrence{node.Content[0].Value + ":" + node.Content[1].Value, path})
		}
		for i := 0; i < len(node.Content); i += 2 {
			key, child := node.Content[i], node.Content[i+1]
			childPath := joinKey(path, key.Value)
			if key.Value == "headers" && child.Kind == yaml.MappingNode {
				for h := 0; h < len(child.Content); h += 2 {
					occurrences = collectRefOccurrences(child.Content[h+1], childPath+"["+fmt.Sprintf("%q", child.Content[h].Value)+"]", occurrences)
				}
				continue
			}
			occurrences = collectRefOccurrences(child, childPath, occurrences)
		}
		return occurrences
	}
	for index, child := range node.Content {
		occurrences = collectRefOccurrences(child, fmt.Sprintf("%s[%d]", path, index), occurrences)
	}
	return occurrences
}

func findScalar(node *yaml.Node, key string) string {
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == key && node.Content[i+1].Kind == yaml.ScalarNode {
				return node.Content[i+1].Value
			}
			if value := findScalar(node.Content[i+1], key); value != "" {
				return value
			}
		}
	}
	for _, child := range node.Content {
		if value := findScalar(child, key); value != "" {
			return value
		}
	}
	return ""
}

// mappingChild returns the direct child mapping named key, or nil. Unlike
// findScalar it never searches nested sections, so an embedding env reference
// is attributed to its own field without scanning unrelated parts of the
// document (spec §4.1).
func mappingChild(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// WireToolName formats a preloaded tool's canonical wire name as <server_id>__<upstream_tool_name>.
func WireToolName(serverID, toolName string) string {
	return serverID + "__" + toolName
}

// ParseWireToolName splits a canonical preloaded tool wire name into its owning server ID
// and raw upstream tool name. It returns ok=false if the wire name does not contain the
// double-underscore delimiter or if either part is empty.
func ParseWireToolName(wireName string) (serverID, toolName string, ok bool) {
	idx := strings.Index(wireName, "__")
	if idx <= 0 || idx+2 >= len(wireName) {
		return "", "", false
	}
	return wireName[:idx], wireName[idx+2:], true
}

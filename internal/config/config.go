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
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

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

type Server struct {
	ID              string         `yaml:"id"`
	Endpoint        string         `yaml:"endpoint"`
	PromptsDisabled bool           `yaml:"prompts_disabled"`
	Credentials     map[string]any `yaml:"credentials,omitempty"`
	Limits          map[string]any `yaml:"limits,omitempty"`
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
}

func (c Config) ClientTokenHashes() [][32]byte {
	return append([][32]byte(nil), c.clientTokenHashes...)
}

func Parse(data []byte, configPath string) (*Config, error) {
	if len(data) > maxConfigBytes {
		return nil, fmt.Errorf("configuration exceeds 1 MiB")
	}
	root := yaml.Node{}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("decode configuration: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("configuration must contain one YAML document")
	}
	if len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("configuration must be a mapping")
	}
	if err := validateNode(root.Content[0], "root"); err != nil {
		return nil, err
	}
	var cfg Config
	strict := yaml.NewDecoder(bytes.NewReader(data))
	strict.KnownFields(true)
	if err := strict.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode configuration: %w", err)
	}
	if cfg.SchemaVersion != 1 {
		return nil, fmt.Errorf("schema_version must equal 1")
	}
	if len(cfg.Servers) == 0 || len(cfg.Servers) > 1024 {
		return nil, fmt.Errorf("servers must contain 1..1024 entries")
	}
	ids := make(map[string]bool, len(cfg.Servers))
	for _, server := range cfg.Servers {
		if !serverIDPattern.MatchString(server.ID) {
			return nil, fmt.Errorf("invalid server id %q", server.ID)
		}
		if ids[server.ID] {
			return nil, fmt.Errorf("duplicate server id %q", server.ID)
		}
		ids[server.ID] = true
		normalized, err := normalizeURL(server.Endpoint)
		if err != nil {
			return nil, fmt.Errorf("server %s endpoint: %w", server.ID, err)
		}
		cfg.Servers[len(ids)-1].Endpoint = normalized
	}
	seenPins := make(map[string]bool, len(cfg.Pins))
	for _, pin := range cfg.Pins {
		if len(pin) != 2 || pin["server"] == "" || pin["tool"] == "" || !ids[pin["server"]] {
			return nil, fmt.Errorf("invalid pin")
		}
		key := pin["server"] + "\x00" + pin["tool"]
		if seenPins[key] {
			return nil, fmt.Errorf("duplicate pin")
		}
		seenPins[key] = true
	}
	if err := validateBounds(root.Content[0]); err != nil {
		return nil, err
	}
	if err := validateWeights(root.Content[0]); err != nil {
		return nil, err
	}
	if baseURL := findScalar(root.Content[0], "base_url"); baseURL != "" {
		if err := validateURL(baseURL); err != nil {
			return nil, fmt.Errorf("embedding base_url: %w", err)
		}
	}
	if directory, ok := cfg.Storage["directory"].(string); ok && !filepath.IsAbs(directory) {
		cfg.Storage["directory"] = filepath.Clean(filepath.Join(filepath.Dir(configPath), directory))
	}
	resolved := make(map[string]string)
	if err := resolveSecrets(root.Content[0], filepath.Dir(configPath), resolved); err != nil {
		return nil, err
	}
	clientRefs, backendRefs := map[string]bool{}, map[string]bool{}
	for i := 0; i < len(root.Content[0].Content); i += 2 {
		key, value := root.Content[0].Content[i], root.Content[0].Content[i+1]
		if key.Value == "client_auth" {
			collectRefs(value, clientRefs)
		} else {
			collectRefs(value, backendRefs)
		}
	}
	backendValues := make(map[string]bool, len(backendRefs))
	for ref := range backendRefs {
		backendValues[resolved[ref]] = true
	}
	for ref := range clientRefs {
		if backendRefs[ref] || backendValues[resolved[ref]] {
			return nil, fmt.Errorf("client tokens and backend credentials must be disjoint")
		}
		cfg.clientTokenHashes = append(cfg.clientTokenHashes, sha256.Sum256([]byte(resolved[ref])))
	}
	cfg.resolved = resolved
	return &cfg, nil
}

func ParseFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
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
	"server":            set("id endpoint prompts_disabled credentials limits"), "credentials": set("bearer headers"),
	"limits": set("max_concurrent_calls call_timeout"), "breaker": set("consecutive_failures open_duration half_open_probes"),
	"pin": set("server tool"), "secret": set("env file"),
}

func set(words string) map[string]bool {
	result := map[string]bool{}
	for _, word := range strings.Fields(words) {
		result[word] = true
	}
	return result
}

func validateNode(node *yaml.Node, section string) error {
	if section == "headers" {
		return validateHeaders(node)
	}
	if section == "secret" {
		if node.Kind != yaml.MappingNode || len(node.Content) != 2 || (node.Content[0].Value != "env" && node.Content[0].Value != "file") {
			return fmt.Errorf("secret must be exactly {env: NAME} or {file: PATH}")
		}
		key, value := node.Content[0], node.Content[1]
		if key.Kind != yaml.ScalarNode || value.Kind != yaml.ScalarNode || value.Tag != "!!str" || value.Value == "" {
			return fmt.Errorf("invalid secret reference")
		}
		if key.Value == "env" && !envNamePattern.MatchString(value.Value) {
			return fmt.Errorf("invalid environment variable name")
		}
		if strings.ContainsAny(value.Value, "$`\r\n\x00") {
			return fmt.Errorf("secret references cannot contain interpolation or commands")
		}
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping")
	}
	seen := map[string]bool{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Value == "<<" {
			return fmt.Errorf("aliases, merge keys, and non-string keys are not allowed")
		}
		if seen[key.Value] {
			return fmt.Errorf("duplicate key %q", key.Value)
		}
		seen[key.Value] = true
		allowed, ok := fields[section]
		if !ok || !allowed[key.Value] {
			return fmt.Errorf("unknown key %q", key.Value)
		}
		if value.Kind == yaml.AliasNode {
			return fmt.Errorf("YAML aliases are not allowed")
		}
		if value.Kind == yaml.ScalarNode && value.Tag == "!!float" {
			var number float64
			if err := value.Decode(&number); err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
				return fmt.Errorf("non-finite numbers are not allowed")
			}
		}
		next := childSection(section, key.Value)
		if (section == "credentials" && key.Value == "bearer" || section == "embedding" && key.Value == "api_key") && value.Kind != yaml.MappingNode {
			return fmt.Errorf("secret must use an env or file reference")
		}
		if err := validateScalarType(section, key.Value, value); err != nil {
			return err
		}
		if value.Kind == yaml.MappingNode {
			if err := validateNode(value, next); err != nil {
				return err
			}
		}
		if value.Kind == yaml.SequenceNode {
			for _, child := range value.Content {
				if child.Kind == yaml.AliasNode {
					return fmt.Errorf("YAML aliases are not allowed")
				}
				if next == "secret" && child.Kind != yaml.MappingNode {
					return fmt.Errorf("secret must be a reference")
				}
				if child.Kind == yaml.MappingNode {
					if err := validateNode(child, next); err != nil {
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

func validateScalarType(section, key string, node *yaml.Node) error {
	stringFields := set("listen directory session_idle_timeout refresh_interval stale_after timeout query_cache_ttl provider model model_revision base_url id endpoint bearer env file tool server connect_timeout call_timeout refresh_timeout open_duration")
	if section == "server" && key == "prompts_disabled" && (node.Kind != yaml.ScalarNode || node.Tag != "!!bool") {
		return fmt.Errorf("%s must be a boolean", key)
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
		return fmt.Errorf("%s must use secret references", key)
	}
	if section == "root" && key == "schema_version" && node.Kind == yaml.ScalarNode && node.Tag == "!!int" {
		return nil
	}
	if stringFields[key] && (node.Kind != yaml.ScalarNode || node.Tag != "!!str") {
		return fmt.Errorf("%s must be a string", key)
	}
	if integerFields[key] && (node.Kind != yaml.ScalarNode || node.Tag != "!!int") {
		return fmt.Errorf("%s must be an integer", key)
	}
	if (key == "keyword_weight" || key == "embedding_weight") && (node.Kind != yaml.ScalarNode || node.Tag != "!!float" && node.Tag != "!!int") {
		return fmt.Errorf("%s must be numeric", key)
	}
	if sequenceFields[key] && node.Kind != yaml.SequenceNode {
		return fmt.Errorf("%s must be a sequence", key)
	}
	if section == "http" && (key == "allowed_hosts" || key == "allowed_origins") {
		for _, value := range node.Content {
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				return fmt.Errorf("%s entries must be strings", key)
			}
		}
	}
	if section == "root" && node.Kind != yaml.MappingNode && node.Kind != yaml.SequenceNode && key != "schema_version" {
		return fmt.Errorf("%s must be a structured value", key)
	}
	return nil
}

func validateHeaders(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("credentials.headers must be a mapping")
	}
	seen := make(map[string]struct{}, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		header := node.Content[i]
		if header.Kind != yaml.ScalarNode || header.Tag != "!!str" || !validHeaderFieldName(header.Value) {
			return fmt.Errorf("invalid credential header name")
		}
		key := strings.ToLower(header.Value)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate credential header %q", header.Value)
		}
		seen[key] = struct{}{}
		for _, forbidden := range []string{"authorization", "host", "cookie", "origin", "mcp-session-id", "mcp-protocol-version", "x-request-id", "connection", "proxy-", "transfer-encoding", "upgrade", "keep-alive", "te", "trailer"} {
			if key == forbidden || strings.HasPrefix(forbidden, "proxy-") && strings.HasPrefix(key, forbidden) {
				return fmt.Errorf("credential header %q is forbidden", header.Value)
			}
		}
		value := node.Content[i+1]
		if value.Kind != yaml.MappingNode {
			return fmt.Errorf("credential header value must use a secret reference")
		}
		if err := validateNode(value, "secret"); err != nil {
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

func privateSecretFile(info os.FileInfo) bool {
	if info == nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(stat.Uid) == os.Geteuid()
}

func resolveSecrets(node *yaml.Node, base string, result map[string]string) error {
	if node.Kind == yaml.MappingNode {
		if len(node.Content) == 2 && (node.Content[0].Value == "env" || node.Content[0].Value == "file") {
			kind, ref := node.Content[0].Value, node.Content[1].Value
			value := ""
			if kind == "env" {
				value = os.Getenv(ref)
			} else {
				path := ref
				if !filepath.IsAbs(path) {
					path = filepath.Join(base, path)
				}
				info, err := os.Lstat(path)
				if err != nil || !privateSecretFile(info) {
					return fmt.Errorf("secret file must be regular, non-symlink, operator-owned, and private")
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				value = strings.TrimSuffix(string(data), "\n")
			}
			if len(value) == 0 || len(value) > 8192 || !utf8.ValidString(value) || strings.ContainsAny(value, "\r\n\x00") {
				return fmt.Errorf("secret must be nonempty UTF-8, <=8KiB, without CR/LF/NUL")
			}
			result[kind+":"+ref] = value
			return nil
		}
		for _, child := range node.Content {
			if err := resolveSecrets(child, base, result); err != nil {
				return err
			}
		}
	} else {
		for _, child := range node.Content {
			if err := resolveSecrets(child, base, result); err != nil {
				return err
			}
		}
	}
	return nil
}

func collectRefs(node *yaml.Node, refs map[string]bool) {
	if node.Kind == yaml.MappingNode {
		if len(node.Content) == 2 && (node.Content[0].Value == "env" || node.Content[0].Value == "file") {
			refs[node.Content[0].Value+":"+node.Content[1].Value] = true
			return
		}
		for _, child := range node.Content {
			collectRefs(child, refs)
		}
		return
	}
	for _, child := range node.Content {
		collectRefs(child, refs)
	}
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

func validateWeights(root *yaml.Node) error {
	keyword, embedding := findScalar(root, "keyword_weight"), findScalar(root, "embedding_weight")
	if keyword == "" && embedding == "" {
		return nil
	}
	var k, e float64
	if _, err := fmt.Sscan(keyword, &k); err != nil {
		return fmt.Errorf("invalid keyword_weight")
	}
	if _, err := fmt.Sscan(embedding, &e); err != nil {
		return fmt.Errorf("invalid embedding_weight")
	}
	if math.IsNaN(k) || math.IsInf(k, 0) || math.IsNaN(e) || math.IsInf(e, 0) || k <= 0 || e < 0 || math.Abs(k+e-1) > 1e-9 {
		return fmt.Errorf("search weights must be nonnegative, sum to one, and keyword_weight positive")
	}
	return nil
}

func validateBounds(node *yaml.Node) error {
	return validateBoundsAt(node, "")
}

// validateBoundsAt walks the document carrying the dotted path of the mapping
// being inspected, so a rejected bound names the key an operator must edit
// (`embedding.timeout`, not a bare `timeout` that could be any of a dozen keys).
func validateBoundsAt(node *yaml.Node, path string) error {
	bounds := map[string][2]float64{
		"max_top_k": {1, 50}, "default_top_k": {1, 50}, "max_sessions": {1, 4096}, "max_concurrent_calls": {1, 128},
		"max_tools_per_server": {1, 10000}, "max_tools_total": {1, 20000}, "max_pages_per_server": {1, 1000},
		"max_schema_bytes": {1, 262144}, "max_description_bytes": {1, 32768}, "max_snapshot_bytes": {1, 268435456},
		"max_request_bytes": {1, 1048576}, "max_event_bytes": {1, 4194304}, "max_response_bytes": {1, 8388608},
		"dimensions": {1, 4096}, "batch_size": {1, 128}, "max_concurrency": {1, 32}, "query_cache_entries": {0, 4096},
		"rrf_constant": {1, 1e9}, "slots": {1, 128},
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			keyPath := key.Value
			if path != "" {
				keyPath = path + "." + key.Value
			}
			if bound, ok := bounds[key.Value]; ok {
				var number float64
				if err := value.Decode(&number); err != nil || number < bound[0] || number > bound[1] {
					return fmt.Errorf("%s outside permitted bounds", keyPath)
				}
			}
			// session_idle_timeout is a session lifetime, not a request timeout: the
			// spec's documented default is 10m (.omp/plans/headless-mcp-bridge.md §4.2),
			// so it gets an explicit window instead of the request-path suffix heuristic
			// below. 1s..1h admits that default with headroom for longer-lived idle
			// sessions while rejecting effectively permanent sessions (>1h) and
			// sub-second churning; non-positive/unparseable values still fail closed.
			// The branch is exclusive: the key ends in "_timeout" but is NOT a
			// request-path timeout, so it must never fall through to that rule.
			if key.Value == "session_idle_timeout" {
				if value.Kind != yaml.ScalarNode {
					return fmt.Errorf("%s must be a duration", keyPath)
				}
				duration, err := time.ParseDuration(value.Value)
				if err != nil || duration < sessionIdleMinDuration || duration > sessionIdleMaxDuration {
					return fmt.Errorf("%s outside permitted duration bounds", keyPath)
				}
			} else if key.Value == "timeout" || strings.HasSuffix(key.Value, "_timeout") {
				if value.Kind != yaml.ScalarNode {
					return fmt.Errorf("%s must be a duration", keyPath)
				}
				duration, err := time.ParseDuration(value.Value)
				limit := 30 * time.Second
				if key.Value != "timeout" {
					limit = 120 * time.Second
				}
				if strings.Contains(strings.ToLower(key.Value), "search") || strings.Contains(strings.ToLower(key.Value), "embed") {
					limit = 30 * time.Second
				}
				if err != nil || duration <= 0 || duration > limit {
					return fmt.Errorf("%s outside permitted duration bounds", keyPath)
				}
			}
			if key.Value == "refresh_interval" || key.Value == "stale_after" || key.Value == "query_cache_ttl" || key.Value == "open_duration" {
				duration, err := time.ParseDuration(value.Value)
				if err != nil || duration <= 0 {
					return fmt.Errorf("%s must be a positive duration", keyPath)
				}
			}
			if err := validateBoundsAt(value, keyPath); err != nil {
				return err
			}
		}
		return nil
	}
	for _, child := range node.Content {
		if err := validateBoundsAt(child, path); err != nil {
			return err
		}
	}
	return nil
}

package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// fingerprintFormatVersion identifies the canonical projection used by
// Fingerprint. Bumping it deliberately invalidates every stored fingerprint,
// which forces a rebuild rather than silently comparing two incompatible
// projections.
const fingerprintFormatVersion = 1

// publicProjection is the canonical, secret-free view of a configuration.
//
// Every field is either a scalar, a list, or a mapping of scalars that the
// document validator admits. Secret-bearing positions (embedding api_key,
// client_auth bearer_tokens, server credentials) only ever hold
// {env: NAME}/{file: PATH} references — Validate rejects literal secrets — so
// this projection can never carry a resolved value.
type publicProjection struct {
	FormatVersion   int                 `json:"format_version"`
	SchemaVersion   int                 `json:"schema_version"`
	HTTP            map[string]any      `json:"http,omitempty"`
	Operations      map[string]any      `json:"operations,omitempty"`
	Storage         map[string]any      `json:"storage,omitempty"`
	Catalog         map[string]any      `json:"catalog,omitempty"`
	Search          map[string]any      `json:"search,omitempty"`
	Embedding       map[string]any      `json:"embedding,omitempty"`
	ClientAuth      map[string]any      `json:"client_auth,omitempty"`
	UpstreamDefault map[string]any      `json:"upstream_defaults,omitempty"`
	Servers         []projectedServer   `json:"servers"`
	Pins            []map[string]string `json:"pins"`
}

type projectedServer struct {
	ID              string         `json:"id"`
	Endpoint        string         `json:"endpoint"`
	Limits          map[string]any `json:"limits,omitempty"`
	PromptsDisabled bool           `json:"prompts_disabled"`
	Credentials     map[string]any `json:"credentials,omitempty"`
}

// Fingerprint returns the stable public identity of the configuration: a
// lowercase 64-hex SHA-256 over the canonical projection of every non-secret
// field. It is independent of comments, quoting, key order, and the order of
// the `servers`/`pins` lists, and it never changes when only a resolved secret
// value rotates. See plan §4.2 `config_public_fingerprint`.
func (c Config) Fingerprint() string {
	projection := publicProjection{
		FormatVersion:   fingerprintFormatVersion,
		SchemaVersion:   c.SchemaVersion,
		HTTP:            c.HTTP,
		Operations:      c.Operations,
		Storage:         c.Storage,
		Catalog:         c.Catalog,
		Search:          c.Search,
		Embedding:       c.Embedding,
		ClientAuth:      c.ClientAuth,
		UpstreamDefault: c.UpstreamDefaults,
		Pins:            c.Pins,
	}
	for _, server := range c.Servers {
		projection.Servers = append(projection.Servers, projectedServer{
			ID:              server.ID,
			Endpoint:        server.Endpoint,
			PromptsDisabled: server.PromptsDisabled,
			Limits:          server.Limits,
			Credentials:     server.Credentials,
		})
	}
	if projection.Servers == nil {
		projection.Servers = []projectedServer{}
	}
	if projection.Pins == nil {
		projection.Pins = []map[string]string{}
	}
	sort.Slice(projection.Servers, func(i, j int) bool { return projection.Servers[i].ID < projection.Servers[j].ID })
	sort.Slice(projection.Pins, func(i, j int) bool {
		if projection.Pins[i]["server"] != projection.Pins[j]["server"] {
			return projection.Pins[i]["server"] < projection.Pins[j]["server"]
		}
		return projection.Pins[i]["tool"] < projection.Pins[j]["tool"]
	})
	return digest(projection)
}

// ServerCredentialFingerprint returns the per-server credential identity a
// reload uses to decide whether a dedicated upstream client can be reused: a
// lowercase 64-hex SHA-256 over the server id and the resolved credential
// values referenced by that server's own credential mappings (plan §4.10
// steps 2 and 5). The values themselves are never returned. It reports false
// for an unknown server id or when a referenced secret cannot be resolved, so
// a caller fails closed instead of reusing a client on an unknown identity.
func (c Config) ServerCredentialFingerprint(serverID string) (string, bool) {
	for _, server := range c.Servers {
		if server.ID != serverID {
			continue
		}
		values := make(map[string]string)
		for key, raw := range server.Credentials {
			refs := credentialRefs(raw)
			if len(refs) == 0 {
				return "", false
			}
			for _, ref := range refs {
				value, ok := c.resolved[ref]
				if !ok {
					return "", false
				}
				values[key+"\x00"+ref] = value
			}
		}
		return digest(struct {
			Server      string            `json:"server_id"`
			Credentials map[string]string `json:"credentials"`
		}{Server: server.ID, Credentials: values}), true
	}
	return "", false
}

// credentialRefs collects the "env:NAME"/"file:PATH" references nested in one
// server credential value, in canonical form.
func credentialRefs(raw any) []string {
	refs := make([]string, 0, 1)
	var walk func(value any)
	walk = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			if len(node) == 1 {
				for kind, target := range node {
					if name, ok := target.(string); ok && (kind == "env" || kind == "file") {
						refs = append(refs, kind+":"+name)
						return
					}
				}
			}
			for _, child := range node {
				walk(child)
			}
		case []any:
			for _, child := range node {
				walk(child)
			}
		}
	}
	walk(raw)
	sort.Strings(refs)
	return refs
}

// digest hashes the canonical JSON encoding of a validated projection.
// encoding/json sorts mapping keys, so the encoding is deterministic.
func digest(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		// Unreachable: the projection contains only scalars, lists, and
		// string-keyed mappings admitted by Validate. Returning a constant
		// would make distinct configurations compare equal, so fail loudly.
		panic("config: canonical fingerprint encoding failed: " + err.Error())
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

package config

import (
	"fmt"
	"net/http"
	"time"
)

// Credentials is one server's resolved dedicated credential values. The values
// are returned to the command layer only; they never enter a fingerprint, a log
// line, or an error message.
type Credentials struct {
	Bearer  string
	Headers http.Header
}

// Limits is one server's optional overrides of the upstream defaults. A nil
// field is unset, so the caller keeps the default instead of reading a zero
// value as an explicit limit.
type Limits struct {
	MaxConcurrentCalls *int
	CallTimeout        *time.Duration
}

// serverIndex locates one configured server element; -1 means the requested id
// matches no configured server.
func (c Config) serverIndex(serverID string) int {
	for index, server := range c.Servers {
		if server.ID == serverID {
			return index
		}
	}
	return -1
}

// ServerCredentials resolves the credential values one server's own mappings
// reference. Every reference must resolve, and every header name must be one the
// transport accepts, so a caller fails closed instead of opening a client with a
// missing or malformed credential (plan §4.2/§4.6). Rejections are
// metadata-only records attributed to config=<memory> for a directly
// constructed Config (spec §4.1/§5.2 C-1).
func (c Config) ServerCredentials(serverID string) (Credentials, error) {
	index := c.serverIndex(serverID)
	if index < 0 {
		return Credentials{}, direct(Diagnostic{
			Path: c.Origin(), Key: "servers", Server: serverID,
			Code: "server_unknown", Property: "credentials", Shape: "unknown_server",
			Reason: "credentials reference unknown server",
		})
	}
	base := fmt.Sprintf("servers[%d]", index)
	server := c.Servers[index]
	var credentials Credentials
	if raw, ok := server.Credentials["bearer"]; ok {
		value, err := c.resolveOne(base+".credentials.bearer", serverID, raw)
		if err != nil {
			return Credentials{}, err
		}
		credentials.Bearer = value
	}
	if raw, ok := server.Credentials["headers"]; ok {
		headers, ok := raw.(map[string]any)
		if !ok {
			return Credentials{}, direct(Diagnostic{
				Path: c.Origin(), Key: base + ".credentials.headers", Server: serverID,
				Code: "header_mapping", Property: "type", Shape: actualKind(raw),
				Reason: "credentials.headers must be a mapping",
			})
		}
		credentials.Headers = make(http.Header, len(headers))
		for name, value := range headers {
			if !validHeaderFieldName(name) {
				return Credentials{}, direct(Diagnostic{
					Path: c.Origin(), Key: base + ".credentials.headers", Server: serverID,
					Code: "header_name_invalid", Property: "name", Shape: "invalid_identifier",
					Reason: "invalid credential header name",
				})
			}
			resolved, err := c.resolveOne(base+".credentials.headers["+fmt.Sprintf("%q", name)+"]", serverID, value)
			if err != nil {
				return Credentials{}, err
			}
			credentials.Headers.Set(name, resolved)
		}
	}
	return credentials, nil
}

// ServerLimits reports the per-server limit overrides that server declares.
func (c Config) ServerLimits(serverID string) (Limits, error) {
	index := c.serverIndex(serverID)
	if index < 0 {
		return Limits{}, direct(Diagnostic{
			Path: c.Origin(), Key: "servers", Server: serverID,
			Code: "server_unknown", Property: "limits", Shape: "unknown_server",
			Reason: "limits reference unknown server",
		})
	}
	base := fmt.Sprintf("servers[%d]", index)
	server := c.Servers[index]
	var limits Limits
	if raw, ok := server.Limits["max_concurrent_calls"]; ok {
		number, ok := integerValue(raw)
		if !ok {
			return Limits{}, direct(Diagnostic{
				Path: c.Origin(), Key: base + ".limits.max_concurrent_calls", Server: serverID,
				Code: "setting_type", Property: "type", Shape: actualKind(raw),
				Reason: "must be an integer",
			})
		}
		value := int(number)
		limits.MaxConcurrentCalls = &value
	}
	if raw, ok := server.Limits["call_timeout"]; ok {
		text, ok := raw.(string)
		if !ok {
			return Limits{}, direct(Diagnostic{
				Path: c.Origin(), Key: base + ".limits.call_timeout", Server: serverID,
				Code: "setting_duration", Property: "type", Shape: "wrong_type",
				Reason: "must be a duration string",
			})
		}
		duration, err := time.ParseDuration(text)
		if err != nil {
			return Limits{}, direct(Diagnostic{
				Path: c.Origin(), Key: base + ".limits.call_timeout", Server: serverID,
				Code: "setting_duration", Property: "type", Shape: "invalid_duration",
				Reason: "must be a duration string",
			})
		}
		limits.CallTimeout = &duration
	}
	return limits, nil
}

// EmbeddingCredential resolves the embedding provider's own credential, if the
// document configures one. An absent key is reported as empty rather than as an
// error, because an OpenAI-compatible endpoint on a trusted network may need no
// credential; a configured reference that does not resolve is an error, never an
// empty value (plan §4.2/§4.8).
func (c Config) EmbeddingCredential() (string, error) {
	raw, ok := c.Embedding["api_key"]
	if !ok {
		return "", nil
	}
	return c.resolveOne("embedding.api_key", "", raw)
}

// resolveOne resolves exactly one secret reference, and reports a reference it
// cannot resolve rather than substituting an empty value. A multi-reference or
// non-reference shape reports the shape class without inventing a reference
// identity; an unresolved reference reports its configured identity (spec
// §4.3/§5.2 C-1).
func (c Config) resolveOne(keyPath, serverID string, raw any) (string, error) {
	refs := credentialRefs(raw)
	if len(refs) != 1 {
		return "", direct(Diagnostic{
			Path: c.Origin(), Key: keyPath, Server: serverID,
			Code: "reference_shape", Property: "reference", Shape: actualKind(raw),
			Reason: "must reference exactly one secret",
		})
	}
	if value, ok := c.resolved[refs[0]]; ok {
		return value, nil
	}
	kind, name := refParts(refs[0])
	return "", direct(Diagnostic{
		Path: c.Origin(), Key: keyPath, Server: serverID,
		RefKind: kind, RefName: name,
		Code: "reference_unresolved", Property: "reference", Shape: "unresolved",
		Reason: "secret reference does not resolve",
	})
}

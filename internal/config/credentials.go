package config

import (
	"errors"
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

// ServerCredentials resolves the credential values one server's own mappings
// reference. Every reference must resolve, and every header name must be one the
// transport accepts, so a caller fails closed instead of opening a client with a
// missing or malformed credential (plan §4.2/§4.6).
func (c Config) ServerCredentials(serverID string) (Credentials, error) {
	server, ok := c.server(serverID)
	if !ok {
		return Credentials{}, fmt.Errorf("credentials reference unknown server %q", serverID)
	}
	var credentials Credentials
	if raw, ok := server.Credentials["bearer"]; ok {
		value, err := c.resolveOne(serverID, "bearer", raw)
		if err != nil {
			return Credentials{}, err
		}
		credentials.Bearer = value
	}
	if raw, ok := server.Credentials["headers"]; ok {
		headers, ok := raw.(map[string]any)
		if !ok {
			return Credentials{}, fmt.Errorf("server %q credentials.headers must be a mapping", serverID)
		}
		credentials.Headers = make(http.Header, len(headers))
		for name, value := range headers {
			if !validHeaderFieldName(name) {
				return Credentials{}, fmt.Errorf("server %q credential header name is invalid", serverID)
			}
			resolved, err := c.resolveOne(serverID, "headers."+name, value)
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
	server, ok := c.server(serverID)
	if !ok {
		return Limits{}, fmt.Errorf("limits reference unknown server %q", serverID)
	}
	var limits Limits
	if raw, ok := server.Limits["max_concurrent_calls"]; ok {
		number, ok := integerValue(raw)
		if !ok {
			return Limits{}, fmt.Errorf("server %q limits.max_concurrent_calls must be an integer", serverID)
		}
		value := int(number)
		limits.MaxConcurrentCalls = &value
	}
	if raw, ok := server.Limits["call_timeout"]; ok {
		text, ok := raw.(string)
		if !ok {
			return Limits{}, fmt.Errorf("server %q limits.call_timeout must be a duration string", serverID)
		}
		duration, err := time.ParseDuration(text)
		if err != nil {
			return Limits{}, fmt.Errorf("server %q limits.call_timeout must be a duration string", serverID)
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
	refs := credentialRefs(raw)
	if len(refs) != 1 {
		return "", errors.New("embedding.api_key must reference exactly one secret")
	}
	value, ok := c.resolved[refs[0]]
	if !ok {
		return "", errors.New("embedding.api_key does not resolve")
	}
	return value, nil
}

// resolveOne resolves exactly one secret reference, and reports a reference it
// cannot resolve rather than substituting an empty value.
func (c Config) resolveOne(serverID, key string, raw any) (string, error) {
	refs := credentialRefs(raw)
	if len(refs) != 1 {
		return "", fmt.Errorf("server %q credentials.%s must reference exactly one secret", serverID, key)
	}
	value, ok := c.resolved[refs[0]]
	if !ok {
		return "", fmt.Errorf("server %q credentials.%s does not resolve", serverID, key)
	}
	return value, nil
}

func (c Config) server(serverID string) (Server, bool) {
	for _, server := range c.Servers {
		if server.ID == serverID {
			return server, true
		}
	}
	return Server{}, false
}

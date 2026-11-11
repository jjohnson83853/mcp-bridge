package main

import (
	"errors"
	"io"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
)

type startupFailure struct {
	event       string
	operation   string
	level       string
	outcome     string
	code        string
	kind        string
	retryable   bool
	remediation string
	message     string
	resource    map[string]string
}

func (failure startupFailure) Error() string { return failure.message }

func stateDirectoryFailure(path string) startupFailure {
	return startupFailure{
		event: "startup_failed", operation: "startup", level: "error", outcome: "failure",
		code: "state_directory", kind: "resource", retryable: false,
		remediation: "create the configured state directory or fix its permissions",
		message: "application startup failed", resource: map[string]string{"path": path},
	}
}

func jwksCacheFailure() startupFailure {
	return startupFailure{
		event: "startup_failed", operation: "startup", level: "error", outcome: "failure",
		code: "jwks_cache", kind: "dependency", retryable: true,
		remediation: "verify the identity provider URL and network reachability, then restart",
		message: "application startup failed",
	}
}

func embeddingProviderFailure() startupFailure {
	return startupFailure{
		event: "startup_failed", operation: "startup", level: "error", outcome: "failure",
		code: "embedding_provider", kind: "dependency", retryable: false,
		remediation: "fix the embedding provider configuration or credentials",
		message: "application startup failed",
	}
}

func toolSurfaceFailure() startupFailure {
	return startupFailure{
		event: "startup_failed", operation: "startup", level: "error", outcome: "failure",
		code: "tool_surface", kind: "internal", retryable: false,
		remediation: "report this failure; the tool surface failed to initialize",
		message: "application startup failed",
	}
}

func catalogLoopFailure() startupFailure {
	return startupFailure{
		event: "startup_failed", operation: "startup", level: "error", outcome: "failure",
		code: "catalog_loop", kind: "internal", retryable: false,
		remediation: "report this failure; the catalog loop failed to start",
		message: "application startup failed",
	}
}

func listenerBindFailure(listener, address string) startupFailure {
	return startupFailure{
		event: "startup_failed", operation: "startup", level: "error", outcome: "failure",
		code: "listener_bind", kind: "resource", retryable: true,
		remediation: "free the configured address or change the listen setting",
		message: "application startup failed",
		resource: map[string]string{"listener": listener, "address": address},
	}
}

func cliUsageFailure() startupFailure {
	return startupFailure{
		event: "cli_usage_failed", operation: "cli", level: "error", outcome: "failure",
		code: "cli_usage", kind: "usage", retryable: false,
		remediation: "use valid flags and arguments; run mcp-bridge --help for usage",
		message: "command-line arguments are invalid",
	}
}

func startupPublicationFailure() startupFailure {
	return startupFailure{
		event: "startup_publication_failed", operation: "startup", level: "error", outcome: "failure",
		code: "publication_failed", kind: "internal", retryable: true,
		remediation: "verify the startup catalog and restart the service",
		message: "startup publication failed",
	}
}

func fallbackStartupFailure() startupFailure {
	return startupFailure{
		event: "startup_failed", operation: "startup", level: "error", outcome: "failure",
		code: "internal_error", kind: "internal", retryable: false,
		remediation: "report this failure; no specific class matched",
		message: "application startup failed",
	}
}

func startupFailureForError(err error) startupFailure {
	if errors.Is(err, service.ErrPublicationFailed) {
		return startupPublicationFailure()
	}
	return fallbackStartupFailure()
}

func writeStartupFailure(output io.Writer, failure startupFailure) {
	envelope := observe.NewEnvelope(failure.level, failure.event, failure.message, failure.operation, failure.outcome)
	envelope.Error = &observe.ErrorInfo{
		Code:        failure.code,
		Kind:        failure.kind,
		Retryable:   failure.retryable,
		Remediation: failure.remediation,
	}
	if len(failure.resource) != 0 {
		envelope.Resource = &observe.ResourceInfo{
			Path:     failure.resource["path"],
			Listener: failure.resource["listener"],
			Address:  failure.resource["address"],
		}
	}
	observe.WriteJSONLine(output, envelope)
}

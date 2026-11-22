package main

import (
	"errors"
	"io"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
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
	stage       string
	cause       string
	resource    map[string]string
}

const (
	startupStageCatalogLoopStart = "catalog_loop_start"
	startupStageCatalogLoopDrain = "catalog_loop_drain"
	startupStageListenerServe    = "listener_serve"
)

type startupStageError struct {
	stage    string
	listener string
	address  string
	cause    error
}

func (err *startupStageError) Error() string { return err.cause.Error() }

func (err *startupStageError) Unwrap() []error { return []error{err.cause} }

func carryStartupStage(err error, stage, listener, address string) error {
	if err == nil {
		return nil
	}
	return &startupStageError{stage: stage, listener: listener, address: address, cause: err}
}

func (failure startupFailure) Error() string { return failure.message }

func stateDirectoryFailure(path string) startupFailure {
	return startupFailure{
		event: "startup_failed", operation: "startup", level: "error", outcome: "failure",
		code: "state_directory", kind: "resource", retryable: false,
		remediation: "create the configured state directory or fix its permissions",
		message:     "application startup failed", resource: map[string]string{"path": path},
	}
}

func jwksCacheFailure() startupFailure {
	return startupFailure{
		event: "startup_failed", operation: "startup", level: "error", outcome: "failure",
		code: "jwks_cache", kind: "dependency", retryable: true,
		remediation: "verify the identity provider URL and network reachability, then restart",
		message:     "application startup failed",
	}
}

func embeddingProviderFailure() startupFailure {
	return startupFailure{
		event: "startup_failed", operation: "startup", level: "error", outcome: "failure",
		code: "embedding_provider", kind: "dependency", retryable: false,
		remediation: "fix the embedding provider configuration or credentials",
		message:     "application startup failed",
	}
}

func toolSurfaceFailure() startupFailure {
	return startupFailure{
		event: "startup_failed", operation: "startup", level: "error", outcome: "failure",
		code: "tool_surface", kind: "internal", retryable: false,
		remediation: "report this failure; the tool surface failed to initialize",
		message:     "application startup failed",
	}
}

func catalogLoopFailure() startupFailure {
	return startupFailure{
		event: "startup_failed", operation: "startup", level: "error", outcome: "failure",
		code: "catalog_loop", kind: "internal", retryable: false,
		remediation: "report this failure; the catalog loop failed to start",
		message:     "application startup failed",
	}
}

func snapshotRecoveryFailure(path string) startupFailure {
	return startupFailure{
		event: "startup_failed", operation: "startup", level: "error", outcome: "failure",
		code: "snapshot_recovery", kind: "resource", retryable: false,
		remediation: "report this failure; startup snapshot recovery failed",
		message:     "application startup failed", stage: "snapshot_recovery", cause: "snapshot_unrecoverable",
		resource: map[string]string{"path": path},
	}
}

func catalogLoopStartFailure(err error) startupFailure {
	cause := "generic_loop_start"
	if errors.Is(err, storage.ErrCorrupt) {
		cause = "admission_persist_failed"
	}
	return startupFailure{
		event: "startup_failed", operation: "startup", level: "error", outcome: "failure",
		code: startupStageCatalogLoopStart, kind: "internal", retryable: false,
		remediation: "report this failure; the catalog loop failed to start or persist its startup snapshot",
		message:     "application startup failed", stage: startupStageCatalogLoopStart, cause: cause,
	}
}

func catalogLoopDrainFailure() startupFailure {
	return startupFailure{
		event: "startup_failed", operation: "startup", level: "error", outcome: "failure",
		code: startupStageCatalogLoopDrain, kind: "internal", retryable: false,
		remediation: "report this failure; the catalog loop did not drain within the shutdown budget",
		message:     "application startup failed", stage: startupStageCatalogLoopDrain, cause: "drain_budget_exceeded",
	}
}

func listenerServeFailure(listener, address string) startupFailure {
	return startupFailure{
		event: "startup_failed", operation: "startup", level: "error", outcome: "failure",
		code: startupStageListenerServe, kind: "internal", retryable: false,
		remediation: "report this failure; the listener stopped serving",
		message:     "application startup failed", stage: startupStageListenerServe, cause: "listener_serve_failed",
		resource: map[string]string{"listener": listener, "address": address},
	}
}

func listenerBindFailure(listener, address string) startupFailure {
	return startupFailure{
		event: "startup_failed", operation: "startup", level: "error", outcome: "failure",
		code: "listener_bind", kind: "resource", retryable: true,
		remediation: "free the configured address or change the listen setting",
		message:     "application startup failed",
		resource:    map[string]string{"listener": listener, "address": address},
	}
}

func cliUsageFailure() startupFailure {
	return startupFailure{
		event: "cli_usage_failed", operation: "cli", level: "error", outcome: "failure",
		code: "cli_usage", kind: "usage", retryable: false,
		remediation: "use valid flags and arguments; run mcp-bridge --help for usage",
		message:     "command-line arguments are invalid",
	}
}

func startupPublicationFailure() startupFailure {
	return startupFailure{
		event: "startup_publication_failed", operation: "startup", level: "error", outcome: "failure",
		code: "publication_failed", kind: "internal", retryable: true,
		remediation: "verify the startup catalog and restart the service",
		message:     "startup publication failed",
	}
}

func fallbackStartupFailure() startupFailure {
	return startupFailure{
		event: "startup_failed", operation: "startup", level: "error", outcome: "failure",
		code: "internal_error", kind: "internal", retryable: false,
		remediation: "report this failure; no specific class matched",
		message:     "application startup failed",
	}
}

func startupFailureForError(err error) startupFailure {
	if errors.Is(err, service.ErrPublicationFailed) {
		return startupPublicationFailure()
	}
	var stageErr *startupStageError
	if errors.As(err, &stageErr) {
		switch stageErr.stage {
		case startupStageCatalogLoopStart:
			return catalogLoopStartFailure(err)
		case startupStageCatalogLoopDrain:
			return catalogLoopDrainFailure()
		case startupStageListenerServe:
			return listenerServeFailure(stageErr.listener, stageErr.address)
		default:
			return fallbackStartupFailure()
		}
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
	envelope.StartupStage = failure.stage
	envelope.StartupCause = failure.cause
	if err := observe.WriteJSONLine(output, envelope); err != nil {
		observe.WriteLoggingFailureFallback(output)
	}
}

package main

import (
	"errors"
	"io"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

func writeConfigValidationFailure(output io.Writer, err error) {
	observe.WriteJSONLine(output, configValidationFailureEnvelope(err))
}

func configValidationFailureEnvelope(err error) observe.Envelope {
	envelope := observe.NewEnvelope(
		"error",
		"config_validation_failed",
		"configuration could not be validated",
		"load_config",
		"failure",
	)
	envelope.Error = &observe.ErrorInfo{
		Code:        "config_invalid",
		Kind:        "configuration",
		Retryable:   false,
		Remediation: "correct the configuration and rerun validation",
	}

	var diagnostics *config.Diagnostics
	if !errors.As(err, &diagnostics) || diagnostics == nil || len(diagnostics.Records) == 0 {
		return envelope
	}

	record := diagnostics.Records[0]
	envelope.Message = record.Reason
	envelope.Error.Code = record.Code
	envelope.Error.Remediation = configRemediation(record.Code)
	envelope.Config = &observe.ConfigInfo{
		Path:                  record.Path,
		PropertyPath:          record.Key,
		Line:                  record.Line,
		Column:                record.Column,
		Server:                record.Server,
		PinServer:             record.PinServer,
		PinTool:               record.PinTool,
		RefKind:               record.RefKind,
		RefName:               record.RefName,
		Shape:                 record.Shape,
		ActualCount:           record.ActualCount,
		MaxCount:              record.MaxCount,
		FirstExcessIndex:      record.FirstExcessIndex,
		DiagnosticsIncomplete: diagnostics.Incomplete,
		RemainingStages:       diagnostics.RemainingStages,
	}
	if record.RelatedKey != "" || record.RelatedRefKind != "" || record.RelatedRefName != "" || record.RelatedLine != 0 || record.RelatedColumn != 0 {
		envelope.Config.Related = &observe.RelatedConfigInfo{
			PropertyPath: record.RelatedKey,
			Line:         record.RelatedLine,
			Column:       record.RelatedColumn,
			RefKind:      record.RelatedRefKind,
			RefName:      record.RelatedRefName,
		}
	}
	return envelope
}

const defaultConfigRemediation = "correct the configuration and rerun validation"

var configSpecificRemediations = map[string]string{
	"pin_ceiling": "remove pins at index 128 or later",
	"config_read": "check the configuration file path and permissions",
}

func configRemediation(code string) string {
	if remediation, ok := configSpecificRemediations[code]; ok {
		return remediation
	}
	// DiagnosticCodes is the closed set the total-remediation test enumerates;
	// all registered codes without a specific action use this named default.
	return defaultConfigRemediation
}

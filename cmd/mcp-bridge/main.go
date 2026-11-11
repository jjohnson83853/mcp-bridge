package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// run is the process entry point: it turns process signals into the shutdown
// edge the run loop consumes, so every other path stays testable without a
// signal (plan §4.2, §4.12).
func run(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runContext(ctx, args, stdout, stderr)
}

// runContext is run without the process shutdown signal, so a caller can drive
// the same composition root to its own shutdown.
func runContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("mcp-bridge", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	check := flags.Bool("check-config", false, "validate configuration locally")
	path := flags.String("config", "configs/config.yaml", "configuration file")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			flags.SetOutput(stderr)
			flags.Usage()
			flags.SetOutput(io.Discard)
			return 2
		}
		writeStartupFailure(stderr, cliUsageFailure())
		return 1
	}
	if flags.NArg() != 0 {
		writeStartupFailure(stderr, cliUsageFailure())
		return 1
	}
	if *check {
		return checkConfig(*path, stdout, stderr)
	}
	if err := serve(ctx, *path, stdout, stderr); err != nil {
		var diagnostics *config.Diagnostics
		if errors.As(err, &diagnostics) {
			writeConfigValidationFailure(stderr, err)
		} else {
			var failure startupFailure
			if errors.As(err, &failure) {
				writeStartupFailure(stderr, failure)
			} else {
				writeStartupFailure(stderr, fallbackStartupFailure())
			}
		}
		return 1
	}
	return 0
}

// checkConfig validates the document's structure and secret references without
// emitting configuration contents.
func checkConfig(path string, stdout, stderr io.Writer) int {
	if _, err := config.ParseFile(path); err != nil {
		writeConfigValidationFailure(stderr, err)
		return 1
	}
	envelope := observe.NewEnvelope(
		"info",
		"config_validation_succeeded",
		"configuration validated",
		"check_config",
		"success",
	)
	envelope.Config = &observe.ConfigInfo{Path: path, DiagnosticsIncomplete: false}
	observe.WriteJSONLine(stdout, envelope)
	return 0
}

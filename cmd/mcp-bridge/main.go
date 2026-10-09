package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
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
	flags.SetOutput(stderr)
	check := flags.Bool("check-config", false, "validate configuration locally")
	path := flags.String("config", "configs/config.yaml", "configuration file")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *check {
		return checkConfig(*path, stdout, stderr)
	}
	if err := serve(ctx, *path, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "mcp-bridge:", err)
		return 1
	}
	return 0
}

// checkConfig validates the document's structure without resolving or starting
// anything: it reports the reference names a document mentions, never their
// values.
func checkConfig(path string, stdout, stderr io.Writer) int {
	cfg, err := config.ParseFile(path)
	if err != nil {
		fmt.Fprintln(stderr, "invalid configuration:", err)
		return 1
	}
	fmt.Fprintf(stdout, "configuration valid: %s\n", cfg.String())
	return 0
}

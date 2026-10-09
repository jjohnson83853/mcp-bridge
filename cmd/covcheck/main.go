// Command covcheck reports the project's source-line coverage gate.
//
// It reads a coverage profile produced by
//
//	go test -coverpkg=./... -covermode=atomic -coverprofile=coverage.out ./...
//
// and prints the project-wide source-line percentage against the gate, exiting
// non-zero when the gate fails. The measurement itself lives in
// internal/coveragecheck, which is unit-tested.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jjohnson83853/mcp-bridge/internal/coveragecheck"
)

const (
	defaultGatePercent = 90.0
	defaultRoot        = "."
	defaultProfile     = "coverage.out"
)

func main() {
	if err := run(os.Stdout, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "covcheck:", err)
		os.Exit(1)
	}
}

// run measures the profile and reports the verdict, returning an error when the
// measurement fails or the gate is not met.
func run(out io.Writer, args []string) error {
	flags := flag.NewFlagSet("covcheck", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := flags.String("root", defaultRoot, "module root holding the coverage profile")
	profile := flags.String("profile", defaultProfile, "coverage profile path, relative to the module root")
	gate := flags.Float64("gate", defaultGatePercent, "required source-line coverage percentage")
	if err := flags.Parse(args); err != nil {
		return err
	}

	profilePath := *profile
	if !filepath.IsAbs(profilePath) {
		profilePath = filepath.Join(*root, profilePath)
	}
	result, err := coveragecheck.Check(*root, profilePath)
	if err != nil {
		return err
	}
	percent := 0.0
	if result.Total > 0 {
		percent = 100 * float64(result.Hit) / float64(result.Total)
	}
	passed := percent >= *gate
	fmt.Fprintf(out, "source-line coverage %.1f%% (%d/%d), gate %.0f%%: %v\n", percent, result.Hit, result.Total, *gate, passed)
	if !passed {
		return fmt.Errorf("source-line coverage %.1f%% is below the %.0f%% gate", percent, *gate)
	}
	return nil
}

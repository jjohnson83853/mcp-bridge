package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureModule writes a one-package module and the given coverage profile,
// returning the module root.
func fixtureModule(t *testing.T, profile string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example/gate\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := "package gate\n\nfunc Run() { println(\"covered\") }\n"
	if err := os.WriteFile(filepath.Join(root, "p.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if profile != "" {
		if err := os.WriteFile(filepath.Join(root, "coverage.out"), []byte(profile), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestRunReportsGateMet(t *testing.T) {
	root := fixtureModule(t, "mode: atomic\nexample/gate/p.go:3.13,3.33 1 1\n")
	var out bytes.Buffer

	if err := run(&out, []string{"-root", root}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got, want := out.String(), "source-line coverage 100.0% (1/1), gate 90%: true\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRunFailsBelowGate(t *testing.T) {
	root := fixtureModule(t, "mode: atomic\nexample/gate/p.go:3.13,3.33 1 0\n")
	var out bytes.Buffer

	err := run(&out, []string{"-root", root})
	if err == nil {
		t.Fatalf("run succeeded with no covered lines; want gate failure")
	}
	if !strings.Contains(err.Error(), "below the 90% gate") {
		t.Fatalf("error = %v, want below-gate error", err)
	}
	if got, want := out.String(), "source-line coverage 0.0% (0/1), gate 90%: false\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRunHonorsCustomGate(t *testing.T) {
	root := fixtureModule(t, "mode: atomic\nexample/gate/p.go:3.13,3.33 1 1\n")
	var out bytes.Buffer

	if err := run(&out, []string{"-root", root, "-gate", "100"}); err != nil {
		t.Fatalf("run at gate 100 with full coverage: %v", err)
	}
	if !strings.Contains(out.String(), "gate 100%: true") {
		t.Fatalf("output = %q, want gate 100%% verdict", out.String())
	}
}

func TestRunReportsMissingProfile(t *testing.T) {
	root := fixtureModule(t, "")
	var out bytes.Buffer

	err := run(&out, []string{"-root", root, "-profile", "absent.out"})
	if err == nil {
		t.Fatal("run succeeded with no profile; want error")
	}
	if !strings.Contains(err.Error(), "absent.out") {
		t.Fatalf("error = %v, want it to name the profile", err)
	}
}

func TestRunRejectsUnknownFlag(t *testing.T) {
	if err := run(&bytes.Buffer{}, []string{"-nope"}); err == nil {
		t.Fatal("run accepted an unknown flag")
	}
}

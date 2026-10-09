package coveragecheck

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveProfileFileAcceptsAbsoluteAndImportRelativePaths(t *testing.T) {
	pkg := Package{ImportPath: "example/core", Dir: filepath.Join("tmp", "core")}
	absolutePath, err := filepath.Abs(filepath.Join(pkg.Dir, "core.go"))
	if err != nil {
		t.Fatal(err)
	}
	absolute, ok := resolveProfileFile(pkg, absolutePath)
	if !ok || filepath.Clean(absolute) != filepath.Clean(absolutePath) {
		t.Fatalf("absolute resolution = %q, %v", absolute, ok)
	}
	relative, ok := resolveProfileFile(pkg, "example/core/sub/core.go")
	if !ok || filepath.Clean(relative) != filepath.Join(pkg.Dir, "sub", "core.go") {
		t.Fatalf("import-relative resolution = %q, %v", relative, ok)
	}
}

func TestResolveProfileFileRejectsUnrelatedImportPrefix(t *testing.T) {
	pkg := Package{ImportPath: "example/core", Dir: t.TempDir()}
	if resolved, ok := resolveProfileFile(pkg, "example/core-other/core.go"); ok {
		t.Fatalf("unrelated prefix resolved to %q", resolved)
	}
}

func TestMeasureMatchesAbsoluteProfilePathWithRelativePackageDirectory(t *testing.T) {
	dir := t.TempDir()
	source := "package core\nfunc run() { println(\"coverage\") }\n"
	if err := os.WriteFile(filepath.Join(dir, "core.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	workingDir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	relativeDir, err := filepath.Rel(workingDir, dir)
	if err != nil {
		t.Fatal(err)
	}
	pkg := Package{ImportPath: "example/core", Dir: relativeDir, GoFiles: []string{"core.go"}}
	entries := []profileEntry{{
		file:  filepath.Join(dir, "core.go"),
		block: Block{Start: Position{Line: 2, Column: 1}, End: Position{Line: 2, Column: 100}, Count: 1},
	}}
	got, err := measure([]Package{pkg}, entries)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 1 || got.Hit != 1 {
		t.Fatalf("coverage=%+v, want 1/1", got)
	}
}

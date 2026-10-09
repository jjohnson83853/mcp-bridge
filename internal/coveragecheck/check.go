package coveragecheck

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Package struct {
	ImportPath, Dir   string
	GoFiles, CgoFiles []string
}
type Position struct{ Line, Column int }
type Block struct {
	Start, End Position
	Count      uint64
}
type Coverage struct{ Hit, Total int }
type profileEntry struct {
	file  string
	block Block
}

func Check(root, profile string) (Coverage, error) {
	packages, err := listPackages(root)
	if err != nil {
		return Coverage{}, err
	}
	data, err := os.ReadFile(profile)
	if err != nil {
		return Coverage{}, fmt.Errorf("read coverage profile: %w", err)
	}
	entries, err := parseProfile(data)
	if err != nil {
		return Coverage{}, err
	}
	return measure(packages, entries)
}

func listPackages(root string) ([]Package, error) {
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list packages: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(out))
	var packages []Package
	for {
		var item struct {
			ImportPath string
			Dir        string
			GoFiles    []string
			CgoFiles   []string
		}
		if err := decoder.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decode go list output: %w", err)
		}
		packages = append(packages, Package{ImportPath: item.ImportPath, Dir: item.Dir, GoFiles: item.GoFiles, CgoFiles: item.CgoFiles})
	}
	return packages, nil
}

func parseProfile(data []byte) ([]profileEntry, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	if !scanner.Scan() || !strings.HasPrefix(scanner.Text(), "mode: ") {
		return nil, fmt.Errorf("invalid coverage profile header")
	}
	var entries []profileEntry
	for scanner.Scan() {
		line := scanner.Text()
		space := strings.LastIndexByte(line, ' ')
		if space < 0 {
			return nil, fmt.Errorf("invalid coverage profile record %q", line)
		}
		count, err := strconv.ParseUint(line[space+1:], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid coverage count: %w", err)
		}
		prefix := line[:space]
		space = strings.LastIndexByte(prefix, ' ')
		if space < 0 {
			return nil, fmt.Errorf("invalid coverage profile record")
		}
		if _, err := strconv.ParseUint(prefix[space+1:], 10, 64); err != nil {
			return nil, fmt.Errorf("invalid coverage statement count")
		}
		prefix = prefix[:space]
		colon := strings.LastIndexByte(prefix, ':')
		if colon < 0 {
			return nil, fmt.Errorf("invalid coverage profile position %q", prefix)
		}
		file := prefix[:colon]
		positions := strings.Split(prefix[colon+1:], ",")
		if len(positions) != 2 {
			return nil, fmt.Errorf("invalid coverage block %q", line)
		}
		start, err := parsePosition(positions[0])
		if err != nil {
			return nil, err
		}
		end, err := parsePosition(positions[1])
		if err != nil {
			return nil, err
		}
		entries = append(entries, profileEntry{file: file, block: Block{Start: start, End: end, Count: count}})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func parsePosition(text string) (Position, error) {
	parts := strings.Split(text, ".")
	if len(parts) != 2 {
		return Position{}, fmt.Errorf("invalid profile position %q", text)
	}
	line, err := strconv.Atoi(parts[0])
	if err != nil {
		return Position{}, err
	}
	column, err := strconv.Atoi(parts[1])
	if err != nil {
		return Position{}, err
	}
	return Position{Line: line, Column: column}, nil
}

func measure(packages []Package, entries []profileEntry) (Coverage, error) {
	var result Coverage
	for _, pkg := range packages {
		files := append(append([]string(nil), pkg.GoFiles...), pkg.CgoFiles...)
		for _, name := range files {
			filePath, err := filepath.Abs(filepath.Join(pkg.Dir, name))
			if err != nil {
				return Coverage{}, fmt.Errorf("resolve source path %s: %w", name, err)
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
			if err != nil {
				return Coverage{}, fmt.Errorf("parse %s: %w", filePath, err)
			}
			if ast.IsGenerated(file) {
				continue
			}
			eligible := make(map[int][]Position)
			ast.Inspect(file, func(node ast.Node) bool {
				stmt, ok := node.(ast.Stmt)
				if !ok {
					return true
				}
				switch stmt.(type) {
				case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause, *ast.EmptyStmt, *ast.LabeledStmt:
					return true
				}
				pos := fset.Position(stmt.Pos())
				if pos.Line > 0 {
					eligible[pos.Line] = append(eligible[pos.Line], Position{Line: pos.Line, Column: pos.Column})
				}
				return true
			})
			if len(eligible) == 0 {
				continue
			}
			fileHasProfile := false
			for _, entry := range entries {
				profileFile, ok := resolveProfileFile(pkg, entry.file)
				if ok && sameFilePath(profileFile, filePath) {
					fileHasProfile = true
					break
				}
			}
			if !fileHasProfile {
				return Coverage{}, fmt.Errorf("missing eligible profile file: %s/%s", pkg.ImportPath, name)
			}
			for _, positions := range eligible {
				result.Total++
				hit := false
				for _, pos := range positions {
					for _, entry := range entries {
						profileFile, ok := resolveProfileFile(pkg, entry.file)
						if ok && sameFilePath(profileFile, filePath) && entry.block.Count > 0 && within(pos, entry.block.Start, entry.block.End) {
							hit = true
							break
						}
					}
					if hit {
						break
					}
				}
				if hit {
					result.Hit++
				}
			}
		}
	}
	return result, nil
}

func sameFilePath(first, second string) bool {
	firstAbs, err := filepath.Abs(first)
	if err != nil {
		return false
	}
	secondAbs, err := filepath.Abs(second)
	if err != nil {
		return false
	}
	return filepath.Clean(firstAbs) == filepath.Clean(secondAbs)
}

func resolveProfileFile(pkg Package, name string) (string, bool) {
	if filepath.IsAbs(name) {
		return filepath.Clean(name), true
	}
	prefix := pkg.ImportPath + "/"
	if !strings.HasPrefix(name, prefix) {
		return "", false
	}
	return filepath.Join(pkg.Dir, filepath.FromSlash(strings.TrimPrefix(name, prefix))), true
}

func within(pos, start, end Position) bool {
	return (pos.Line > start.Line || pos.Line == start.Line && pos.Column >= start.Column) &&
		(pos.Line < end.Line || pos.Line == end.Line && pos.Column < end.Column)
}

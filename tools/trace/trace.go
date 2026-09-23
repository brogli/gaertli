// Command trace checks requirement traceability: every required
// requirement ID must be named by a Go test, a Vitest or Playwright test,
// or a manual checklist entry.
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Config points trace at the files and directories it reads.
type Config struct {
	FunctionalPath, QualityPath, TestingPath string
	GoRoot, FrontendRoot                     string
	PendingPath                              string
}

var (
	idPattern     = regexp.MustCompile(`^[A-Z]{2,6}-[A-Z]{2,6}-[0-9]{2,3}$`)
	separatorCell = regexp.MustCompile(`^:?-+:?$`)
	goTestFunc    = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]*)\(`)
	goTestID      = regexp.MustCompile(`^Test([A-Z]+)_([A-Z]+)_([0-9]+)(?:_|$)`)
	vitestCall    = regexp.MustCompile(`\b(?:it|test)\(\s*['"]([^'"]*)['"]`)
	vitestID      = regexp.MustCompile(`^([A-Z]{2,6}-[A-Z]{2,6}-[0-9]{2,3})(?:\s|$)`)
)

// run computes the required IDs and their coverage, and reports the
// problems (missing or stale) to out.
func run(cfg Config, out io.Writer) (int, error) {
	required, err := requiredIDs(cfg.FunctionalPath, cfg.QualityPath)
	if err != nil {
		return 0, err
	}
	covered, err := coverage(cfg)
	if err != nil {
		return 0, err
	}
	pending, err := pendingIDs(cfg.PendingPath)
	if err != nil {
		return 0, err
	}
	return report(out, required, covered, pending, cfg.PendingPath), nil
}

// pendingIDs reads the allow-list of IDs whose tests are still owed. A
// missing file is an empty list.
func pendingIDs(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	pending := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			pending[line] = true
		}
	}
	return pending, nil
}

// coverage unions the requirement IDs found by every coverage source.
func coverage(cfg Config) (map[string]bool, error) {
	covered := map[string]bool{}
	for _, ids := range []func() (map[string]bool, error){
		func() (map[string]bool, error) { return goCoverage(cfg.GoRoot) },
		func() (map[string]bool, error) { return frontendCoverage(cfg.FrontendRoot) },
		func() (map[string]bool, error) { return manualCoverage(cfg.TestingPath) },
	} {
		found, err := ids()
		if err != nil {
			return nil, err
		}
		maps.Copy(covered, found)
	}
	return covered, nil
}

// manualCoverage returns the requirement IDs listed in the first cell of a
// docs/architecture/testing.md table row, split on comma.
func manualCoverage(path string) (map[string]bool, error) {
	rows, err := tableRows(path)
	if err != nil {
		return nil, err
	}
	covered := map[string]bool{}
	for _, cells := range rows {
		if len(cells) == 0 {
			continue
		}
		for _, tok := range strings.Split(cells[0], ",") {
			tok = strings.TrimSpace(tok)
			if idPattern.MatchString(tok) {
				covered[tok] = true
			}
		}
	}
	return covered, nil
}

// report prints one sorted line per problem: a required id with no coverage
// that is not pending, or a pending id that is no longer missing. Returns 1
// if there was anything to print.
func report(out io.Writer, required, covered, pending map[string]bool, pendingPath string) int {
	problems := map[string]string{}
	for id := range required {
		if !covered[id] && !pending[id] {
			problems[id] = "no test name and no manual checklist entry"
		}
	}
	for id := range pending {
		if covered[id] || !required[id] {
			problems[id] = "no longer missing; remove it from " + pendingPath
		}
	}
	for _, id := range slices.Sorted(maps.Keys(problems)) {
		_, _ = fmt.Fprintf(out, "%s: %s\n", id, problems[id])
	}
	if len(problems) > 0 {
		return 1
	}
	return 0
}

// goCoverage returns the requirement IDs named by Go test functions below
// root.
func goCoverage(root string) (map[string]bool, error) {
	return coverageInFiles(root, []string{"_test.go"}, func(src string, covered map[string]bool) {
		for _, m := range goTestFunc.FindAllStringSubmatch(src, -1) {
			if id, ok := idFromGoTestName(m[1]); ok {
				covered[id] = true
			}
		}
	})
}

// frontendCoverage returns the requirement IDs named by the first string
// argument of an it(...) or test(...) call in Vitest (*.test.ts) and
// Playwright (*.spec.ts) files below root.
func frontendCoverage(root string) (map[string]bool, error) {
	return coverageInFiles(root, []string{".test.ts", ".spec.ts"}, func(src string, covered map[string]bool) {
		for _, m := range vitestCall.FindAllStringSubmatch(src, -1) {
			if id := vitestID.FindStringSubmatch(m[1]); id != nil {
				covered[id[1]] = true
			}
		}
	})
}

// coverageInFiles walks root, skipping vendor and hidden dirs, and lets
// collect add the IDs it finds in every file ending in one of suffixes.
func coverageInFiles(root string, suffixes []string, collect func(src string, covered map[string]bool)) (map[string]bool, error) {
	covered := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !slices.ContainsFunc(suffixes, func(suffix string) bool { return strings.HasSuffix(path, suffix) }) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		collect(string(data), covered)
		return nil
	})
	return covered, err
}

func idFromGoTestName(name string) (string, bool) {
	m := goTestID.FindStringSubmatch(name)
	if m == nil {
		return "", false
	}
	return m[1] + "-" + m[2] + "-" + m[3], true
}

func skipDir(name string) bool {
	return name == "node_modules" || name == "bin" || strings.HasPrefix(name, ".")
}

// requiredIDs returns every MUST FR id from functionalPath and every QA id
// from qualityPath.
func requiredIDs(functionalPath, qualityPath string) (map[string]bool, error) {
	required := map[string]bool{}

	frRows, err := tableRows(functionalPath)
	if err != nil {
		return nil, err
	}
	for _, cells := range frRows {
		if len(cells) >= 2 && cells[1] == "MUST" && idPattern.MatchString(cells[0]) {
			required[cells[0]] = true
		}
	}

	qaRows, err := tableRows(qualityPath)
	if err != nil {
		return nil, err
	}
	for _, cells := range qaRows {
		if len(cells) >= 1 && idPattern.MatchString(cells[0]) {
			required[cells[0]] = true
		}
	}

	return required, nil
}

// tableRows returns each Markdown table row in path as trimmed cells,
// dividers and non-table lines skipped.
func tableRows(path string) ([][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var rows [][]string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i, c := range cells {
			cells[i] = strings.TrimSpace(c)
		}
		if isDividerRow(cells) {
			continue
		}
		rows = append(rows, cells)
	}
	return rows, nil
}

func isDividerRow(cells []string) bool {
	for _, c := range cells {
		if c != "" && !separatorCell.MatchString(c) {
			return false
		}
	}
	return true
}

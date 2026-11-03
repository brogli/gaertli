// Requirements: DIST-03, OSS-04, QA-MNT-04, QA-MNT-05

// Package repocheck holds the tests that scan the working tree for what no
// linter checks: foreign lockfiles, machine-specific defaults, test-file
// placement, glossary use, the release version script.
package repocheck_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func TestNoOtherPackageManagersLockfile(t *testing.T) {
	root := repoRoot(t)

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() && (name == "node_modules" || strings.HasPrefix(name, ".")) {
			return filepath.SkipDir
		}
		if name == "package-lock.json" || name == "yarn.lock" {
			t.Errorf("foreign lockfile: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// vitestTestPattern matches Vitest's default test file naming (.test. or
// .spec., ts or js).
var vitestTestPattern = regexp.MustCompile(`\.(test|spec)\.(ts|js)$`)

func TestTestFilesLiveInTestsFolderAsSpecTs(t *testing.T) {
	root := filepath.Join(repoRoot(t), "frontend/src")
	var offenders []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		inTestsFolder := filepath.Base(filepath.Dir(path)) == "__tests__"
		isTestFile := vitestTestPattern.MatchString(path)
		if (isTestFile || inTestsFolder) && (!inTestsFolder || !strings.HasSuffix(path, ".spec.ts")) {
			rel, _ := filepath.Rel(root, path)
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Errorf("test files outside src/**/__tests__/*.spec.ts:\n%s", strings.Join(offenders, "\n"))
	}
}

// homePathPattern, remoteURLPattern, and branchFieldPattern flag content
// that assumes one developer's own setup: an absolute home directory, a
// hard-coded git remote address, or a hard-coded "branch" field.
var (
	homePathPattern    = regexp.MustCompile(`/home/[a-z][a-z0-9_-]*/|C:\\Users\\[A-Za-z0-9_ .-]+`)
	remoteURLPattern   = regexp.MustCompile(`git@[A-Za-z0-9._-]+:|[a-z][a-z0-9+.-]*://[^\s"']*\.git\b|ssh://`)
	branchFieldPattern = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_-])branch\b\s*[:=]\s*"([^"]+)"`)
)

// allowedPlumbingBranches are the branch names git itself defaults to;
// anything else hard-coded as a "branch" field is a suspect machine-specific default.
var allowedPlumbingBranches = map[string]bool{"main": true, "master": true, "HEAD": true}

// findMachineSpecificDefaults walks root's files, skipping _test.go files and
// any testing/fixtures/__tests__ directory, and returns the paths of files
// containing a hard-coded home directory, remote URL, or non-plumbing
// branch name.
func findMachineSpecificDefaults(t *testing.T, root string) []string {
	t.Helper()
	var violations []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "fixtures" || d.Name() == "__tests__" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(content)

		if homePathPattern.MatchString(text) || remoteURLPattern.MatchString(text) {
			violations = append(violations, path)
			return nil
		}
		if strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".ts") {
			for _, m := range branchFieldPattern.FindAllStringSubmatch(text, -1) {
				if !allowedPlumbingBranches[m[1]] {
					violations = append(violations, path)
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return violations
}

func TestNoMachineSpecificDefaults(t *testing.T) {
	t.Run("fixture", func(t *testing.T) {
		dir := t.TempDir()
		writeFixture := func(name, content string) {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		writeFixture("dirty.go", "package fixture\n\n// installed at /home/someone/repos/app\n")
		writeFixture("clean.go", "package fixture\n")

		got := findMachineSpecificDefaults(t, dir)
		if len(got) != 1 || !strings.HasSuffix(got[0], "dirty.go") {
			t.Errorf("findMachineSpecificDefaults(fixture) = %v, want exactly one match, dirty.go", got)
		}
	})

	t.Run("real tree", func(t *testing.T) {
		root := repoRoot(t)
		var violations []string
		for _, dir := range []string{"internal", "cmd", "frontend/src", "build"} {
			violations = append(violations, findMachineSpecificDefaults(t, filepath.Join(root, dir))...)
		}
		if len(violations) > 0 {
			t.Errorf("machine-specific defaults found:\n%s", strings.Join(violations, "\n"))
		}
	})
}

func TestReleaseVersionIsConfigsSemanticVersion(t *testing.T) {
	dir := t.TempDir()
	script := readFile(t, repoRoot(t), "build/release-version.sh")
	if err := os.WriteFile(filepath.Join(dir, "release-version.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "version: '3'\n\ninfo:\n  productName: 'nebegit'\n  version: '2.7.1'\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command(filepath.Join(dir, "release-version.sh")).Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "2.7.1" {
		t.Errorf("got %q, want 2.7.1", got)
	}
}

// Package repocheck holds tests that pin repository conventions from
// docs/architecture/technical-requirements.md. They read files, not code.
package repocheck_test

import (
	"encoding/json"
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

func assertFileExists(t *testing.T, root, rel string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
		t.Errorf("%s missing; it must be committed", rel)
	}
}

func assertContainsAll(t *testing.T, name, text string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(text, w) {
			t.Errorf("%s: missing %q", name, w)
		}
	}
}

func TestTR_24_NixShellSuppliesEverythingButGo(t *testing.T) {
	root := repoRoot(t)

	if got := strings.TrimSpace(readFile(t, root, ".envrc")); got != "use flake" {
		t.Errorf(".envrc = %q, want \"use flake\"", got)
	}
	assertFileExists(t, root, "flake.lock")
	assertContainsAll(t, "flake.nix", readFile(t, root, "flake.nix"),
		"nixos-26.05", "nodejs_24", "pnpm_10", "golangci-lint", "pkg-config", "gtk4", "webkitgtk_6_0")
}

type goMod struct {
	Go        string
	Toolchain string
	Tool      []struct{ Path string }
}

func TestTR_25_GoModPinsToolchainAndTools(t *testing.T) {
	cmd := exec.Command("go", "mod", "edit", "-json")
	cmd.Dir = repoRoot(t)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go mod edit -json: %v", err)
	}
	var m goMod
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}

	if m.Go != "1.26.0" {
		t.Errorf("go line = %q, want 1.26.0 (TR-01)", m.Go)
	}
	if m.Toolchain == "" {
		t.Error("toolchain line missing")
	}

	tools := map[string]bool{}
	for _, tl := range m.Tool {
		tools[tl.Path] = true
		if strings.Contains(tl.Path, "golangci-lint") {
			t.Errorf("golangci-lint must not be a tool directive: %s", tl.Path)
		}
	}
	for _, want := range []string{
		"github.com/wailsapp/wails/v3/cmd/wails3",
		"github.com/go-task/task/v3/cmd/task",
	} {
		if !tools[want] {
			t.Errorf("tool directive missing: %s", want)
		}
	}
}

// taskfiles mirrors the includes in Taskfile.yml.
var taskfiles = []string{
	"Taskfile.yml",
	"build/Taskfile.yml",
	"build/linux/Taskfile.yml",
	"build/windows/Taskfile.yml",
	"build/darwin/Taskfile.yml",
}

var callsNpm = regexp.MustCompile(`(^|[\s"'])npm\s`)

func TestTR_23_PnpmIsTheOnlyPackageManager(t *testing.T) {
	root := repoRoot(t)

	var pkg struct{ PackageManager string }
	if err := json.Unmarshal([]byte(readFile(t, root, "frontend/package.json")), &pkg); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(pkg.PackageManager, "pnpm@") {
		t.Errorf("packageManager = %q, want pnpm@<version>", pkg.PackageManager)
	}
	assertFileExists(t, root, "frontend/pnpm-lock.yaml")

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() && (name == "node_modules" || name == ".git" || name == ".direnv") {
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

	assertContainsAll(t, "Taskfile.yml", readFile(t, root, "Taskfile.yml"), "PACKAGE_MANAGER: pnpm")
	for _, tf := range taskfiles {
		for i, line := range strings.Split(readFile(t, root, tf), "\n") {
			if callsNpm.MatchString(line) {
				t.Errorf("%s:%d calls npm: %s", tf, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// offlineEnv is what a test run must set so it can never reach the network (TR-14).
var offlineEnv = []string{"GOPROXY: off", "GOFLAGS: -mod=readonly", "GOTOOLCHAIN: local"}

// taskBody returns the YAML block of one top-level task in a Taskfile.
func taskBody(t *testing.T, taskfile, name string) string {
	t.Helper()
	m := regexp.MustCompile(`(?ms)^  ` + name + `:\n(.*?)(?:^  \S|\z)`).FindStringSubmatch(taskfile)
	if m == nil {
		t.Fatalf("no task %q", name)
	}
	return m[1]
}

func TestQA_MNT_03_TestTaskAndCIRunOffline(t *testing.T) {
	root := repoRoot(t)

	testTask := taskBody(t, readFile(t, root, "Taskfile.yml"), "test")
	assertContainsAll(t, "Taskfile.yml test task", testTask, offlineEnv...)
	assertContainsAll(t, "ci.yml", readFile(t, root, ".github/workflows/ci.yml"), offlineEnv...)
	assertContainsAll(t, "build/Taskfile.yml", readFile(t, root, "build/Taskfile.yml"), "pnpm install --frozen-lockfile")
}

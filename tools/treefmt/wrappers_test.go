// Requirements: QA-MNT-05

package treefmt_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeTool puts an executable named name first on PATH that appends its
// working directory and arguments, one per line, to the returned log.
func fakeTool(t *testing.T, name string) (log string) {
	t.Helper()
	bin := t.TempDir()
	log = filepath.Join(bin, "log")
	script := "#!/bin/sh\npwd >> " + log + "\nfor a in \"$@\"; do echo \"$a\" >> " + log + "; done\necho --- >> " + log + "\n"
	if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

// runs splits the fake tool's log into one line list per invocation.
func runs(t *testing.T, log string) [][]string {
	t.Helper()
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	var out [][]string
	for run := range strings.SplitSeq(strings.TrimSuffix(string(b), "---\n"), "---\n") {
		out = append(out, strings.Split(strings.TrimSuffix(run, "\n"), "\n"))
	}
	return out
}

func run(t *testing.T, script string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), script, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", script, args, err, out)
	}
}

func TestFrontendWrapperRunsTheToolInFrontendOnRelativePaths(t *testing.T) {
	log := fakeTool(t, "pnpm")
	frontend, err := filepath.Abs("../../frontend")
	if err != nil {
		t.Fatal(err)
	}

	run(t, "./frontend", "oxlint", "--fix", "--", "frontend/src/a.ts", "frontend/e2e/b.spec.ts")

	got := runs(t, log)
	want := [][]string{{frontend, "exec", "oxlint", "--fix", "src/a.ts", "e2e/b.spec.ts"}}
	if len(got) != 1 || !slices.Equal(got[0], want[0]) {
		t.Errorf("pnpm runs = %q, want %q", got, want)
	}
}

// Editors format through `treefmt --stdin`, which passes a temporary copy
// outside the repo; the frontend's linters have no config for it.
func TestFrontendWrapperSkipsFilesOutsideFrontend(t *testing.T) {
	log := fakeTool(t, "pnpm")

	run(t, "./frontend", "eslint", "--fix", "--", "/tmp/buffer.ts", "frontend/src/a.ts")
	run(t, "./frontend", "eslint", "--fix", "--", "/tmp/buffer.ts")

	got := runs(t, log)
	if len(got) != 1 || !slices.Equal(got[0][1:], []string{"exec", "eslint", "--fix", "src/a.ts"}) {
		t.Errorf("pnpm runs = %q, want one run on src/a.ts only", got)
	}
}

func TestFrontendWrapperFailsWithoutNodeModules(t *testing.T) {
	fakeTool(t, "pnpm")
	repo := t.TempDir()
	script := filepath.Join(repo, "tools", "treefmt", "frontend")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile("frontend")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, body, 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := exec.CommandContext(t.Context(), script, "oxlint", "--", "frontend/src/a.ts").CombinedOutput()
	if err == nil || !strings.Contains(string(out), "pnpm install") {
		t.Errorf("err = %v, output = %q; want a failure that says to run pnpm install", err, out)
	}
}

func TestGoLintWrapperLintsEachPackageOnce(t *testing.T) {
	log := fakeTool(t, "golangci-lint")

	run(t, "./lint-go-packages", "internal/app/a.go", "internal/app/b.go", "cmd/gaertli/main.go")

	got := runs(t, log)
	if len(got) != 1 {
		t.Fatalf("golangci-lint ran %d times, want once: %q", len(got), got)
	}
	if args := got[0][1:]; !slices.Equal(args, []string{"run", "--fix", "./cmd/gaertli", "./internal/app"}) {
		t.Errorf("golangci-lint args = %q, want run --fix ./cmd/gaertli ./internal/app", args)
	}
}

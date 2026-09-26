// Package gittest builds isolated temp git repositories for tests.
package gittest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/testbin"
)

// Repo is a temp git repository for tests: real git, isolated HOME and
// GIT_CONFIG_GLOBAL. Removed by t.Cleanup.
type Repo struct {
	Dir string // the repo's working directory
}

// Main is the TestMain of every package whose tests call New. It builds bins
// with the developer's own environment (go build needs its module and build
// caches), then points HOME and git's global config at one temp directory
// for the rest of the process, disables the system config, writes the test
// identity into that config once, runs m, and removes both directories.
func Main(m *testing.M, bins ...testbin.Bin) {
	cleanupBins, err := testbin.Build(bins...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	home, err := os.MkdirTemp("", "gittest-home")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("HOME", home); err != nil {
		panic(err)
	}
	if err := os.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig")); err != nil {
		panic(err)
	}
	if err := os.Setenv("GIT_CONFIG_NOSYSTEM", "1"); err != nil {
		panic(err)
	}

	for _, args := range [][]string{
		{"config", "--global", "user.name", "nebegit test"},
		{"config", "--global", "user.email", "test@nebegit.invalid"},
	} {
		if _, err := (git.Runner{}).Run(context.Background(), git.Call{Dir: home, Args: args}); err != nil {
			panic(err)
		}
	}

	m.Run()
	cleanupBins()
	_ = os.RemoveAll(home)
}

// New inits a repo in a temp dir under the process-wide isolated git config
// that Main sets up.
func New(t *testing.T) *Repo {
	t.Helper()
	if msg := isolationError(os.Getenv); msg != "" {
		t.Fatal(msg)
	}

	dir := t.TempDir()
	Run(t, dir, "init")

	return &Repo{Dir: dir}
}

// isolationError reports why the process is not isolated from the
// developer's own git config, or "" when gittest.Main has set it up.
func isolationError(getenv func(string) string) string {
	if getenv("GIT_CONFIG_NOSYSTEM") != "1" || getenv("GIT_CONFIG_GLOBAL") == "" {
		return "gittest: process not isolated; make gittest.Main the package's TestMain"
	}
	return ""
}

// Run runs git with args in dir, failing the test if it does not exit 0.
func Run(t *testing.T, dir string, args ...string) {
	t.Helper()
	Out(t, dir, args...)
}

// Out runs git with args in dir and returns its stdout, trimmed.
func Out(t *testing.T, dir string, args ...string) string {
	t.Helper()
	result, err := (git.Runner{}).Run(context.Background(), git.Call{Dir: dir, Args: args})
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(result.Stdout))
}

// Commit writes content to name in dir, stages it and commits msg.
func Commit(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	Run(t, dir, "add", "--", name)
	CommitAll(t, dir, msg)
}

// CommitAll commits whatever is staged in dir with msg.
func CommitAll(t *testing.T, dir, msg string) {
	t.Helper()
	result, err := (git.Runner{}).Run(context.Background(), git.Call{
		Dir:   dir,
		Args:  []string{"commit", "-F", "-"},
		Stdin: []byte(msg),
	})
	if err != nil {
		t.Fatalf("git commit: %v\n%s", err, result.Stderr)
	}
}

// HeadSHA is dir's HEAD commit.
func HeadSHA(t *testing.T, dir string) string {
	t.Helper()
	return Out(t, dir, "rev-parse", "HEAD")
}

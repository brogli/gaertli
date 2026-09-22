// Package gittest builds isolated temp git repositories for tests.
package gittest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/git"
)

// Repo is a temp git repository for tests: real git, isolated HOME and
// GIT_CONFIG_GLOBAL. Removed by t.Cleanup.
type Repo struct {
	Dir string // the repo's working directory
}

// New sets an isolated HOME, GIT_CONFIG_GLOBAL, and GIT_CONFIG_NOSYSTEM=1
// (no global or system config of the developer is read), runs "git init"
// and sets user.name/user.email in the isolated global config, and returns
// the Repo. Calls t.Fatal on any git failure. Must not be used from a
// parallel subtest (t.Setenv's rule).
func New(t *testing.T) *Repo {
	t.Helper()
	isolateConfig(t)

	dir := t.TempDir()
	Run(t, dir, "init")
	Run(t, dir, "config", "--global", "user.name", "nebegit test")
	Run(t, dir, "config", "--global", "user.email", "test@nebegit.invalid")

	return &Repo{Dir: dir}
}

// isolateConfig points HOME and git's global config at a fresh temp
// directory and disables the system config, so New never reads the
// developer's own git configuration.
func isolateConfig(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
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

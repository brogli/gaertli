// Package gittest builds isolated temp git repositories for tests (TR-14).
package gittest

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/brogli/nebegit/internal/git"
)

// Repo is a temp git repository for tests: real git, isolated HOME and
// GIT_CONFIG_GLOBAL (TR-14). Removed by t.Cleanup.
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
	r := git.Runner{}
	run(t, r, dir, "init")
	run(t, r, dir, "config", "--global", "user.name", "nebegit test")
	run(t, r, dir, "config", "--global", "user.email", "test@nebegit.invalid")

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

func run(t *testing.T, r git.Runner, dir string, args ...string) {
	t.Helper()
	if _, err := r.Run(context.Background(), git.Call{Dir: dir, Args: args}); err != nil {
		t.Fatal(err)
	}
}

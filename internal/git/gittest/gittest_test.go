package gittest

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git"
)

func TestTR_14_NewInitializesIsolatedRepo(t *testing.T) {
	repo := New(t)

	if _, err := os.Stat(filepath.Join(repo.Dir, ".git")); err != nil {
		t.Fatalf(".git missing: %v", err)
	}

	if Out(t, repo.Dir, "config", "--get", "user.email") == "" {
		t.Error("user.email not set in the isolated global config")
	}
}

func TestQA_SEC_03_OddFilenameSurvivesAddAndCommit(t *testing.T) {
	repo := New(t)
	const name = "$(touch pwn).txt"
	marker := filepath.Join(repo.Dir, "pwn") // where a shell running in the repo would create it

	Commit(t, repo.Dir, name, "x", "add the odd filename\n")

	if got := Out(t, repo.Dir, "log", "--name-only", "--format="); got != name {
		t.Errorf("tracked file = %q, want %q", got, name)
	}
	assertAbsent(t, marker)
}

func TestQA_SEC_03_EndOfOptionsBlocksRevisionInjection(t *testing.T) {
	repo := New(t)
	marker := filepath.Join(repo.Dir, "pwn") // where a shell running in the repo would create it

	_, err := (git.Runner{}).Run(t.Context(), git.Call{
		Dir:  repo.Dir,
		Args: []string{"log", "--end-of-options", "$(touch pwn)"},
	})

	coretest.AssertCode(t, err, core.GitFailed)
	assertAbsent(t, marker)
}

func TestQA_COMP_01_HooksRunUnmodified(t *testing.T) {
	repo := New(t)
	marker := filepath.Join(repo.Dir, "hook-ran")
	hook := "#!/bin/sh\ntouch " + marker + "\n"
	hookPath := filepath.Join(repo.Dir, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hookPath, []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}

	Commit(t, repo.Dir, "file.txt", "x", "trigger the hook\n")

	if _, err := os.Stat(marker); err != nil {
		t.Errorf("pre-commit hook did not run: %v", err)
	}
}

func TestQA_SAFE_03_ConfigFilesUntouchedByCalls(t *testing.T) {
	repo := New(t)
	localConfig := filepath.Join(repo.Dir, ".git", "config")
	globalConfig := os.Getenv("GIT_CONFIG_GLOBAL")
	before := snapshot(t, localConfig, globalConfig)

	if err := os.WriteFile(filepath.Join(repo.Dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (git.Runner{}).Run(t.Context(), git.Call{Dir: repo.Dir, Args: []string{"status", "--porcelain=v2", "-z"}, ReadOnly: true}); err != nil {
		t.Fatal(err)
	}
	Run(t, repo.Dir, "add", "--", "file.txt")

	after := snapshot(t, localConfig, globalConfig)
	if !bytes.Equal(before[0], after[0]) {
		t.Error("local .git/config changed")
	}
	if !bytes.Equal(before[1], after[1]) {
		t.Error("global config changed")
	}
}

func snapshot(t *testing.T, paths ...string) [][]byte {
	t.Helper()
	snap := make([][]byte, len(paths))
	for i, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		snap[i] = data
	}
	return snap
}

func assertAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("%s exists: command injection occurred", path)
	}
}

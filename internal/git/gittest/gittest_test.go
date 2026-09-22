package gittest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
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

	result, err := (git.Runner{}).Run(context.Background(), git.Call{
		Dir:      repo.Dir,
		Args:     []string{"config", "--get", "user.email"},
		ReadOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(result.Stdout)) == "" {
		t.Error("user.email not set in the isolated global config")
	}
}

func TestQA_SEC_03_OddFilenameSurvivesAddAndCommit(t *testing.T) {
	repo := New(t)
	const name = "$(touch pwn).txt"
	if err := os.WriteFile(filepath.Join(repo.Dir, name), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "pwn")

	r := git.Runner{}
	ctx := context.Background()
	if _, err := r.Run(ctx, git.Call{Dir: repo.Dir, Args: []string{"add", "--", name}}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Run(ctx, git.Call{Dir: repo.Dir, Args: []string{"commit", "-F", "-"}, Stdin: []byte("add the odd filename\n")}); err != nil {
		t.Fatal(err)
	}

	result, err := r.Run(ctx, git.Call{Dir: repo.Dir, Args: []string{"log", "--name-only", "--format="}, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(result.Stdout)); got != name {
		t.Errorf("tracked file = %q, want %q", got, name)
	}
	assertAbsent(t, marker)
}

func TestQA_SEC_03_EndOfOptionsBlocksRevisionInjection(t *testing.T) {
	repo := New(t)
	marker := filepath.Join(t.TempDir(), "pwn")

	_, err := (git.Runner{}).Run(context.Background(), git.Call{
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
	if err := os.WriteFile(filepath.Join(repo.Dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := git.Runner{}
	ctx := context.Background()
	if _, err := r.Run(ctx, git.Call{Dir: repo.Dir, Args: []string{"add", "--", "file.txt"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Run(ctx, git.Call{Dir: repo.Dir, Args: []string{"commit", "-F", "-"}, Stdin: []byte("trigger the hook\n")}); err != nil {
		t.Fatal(err)
	}

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
	r := git.Runner{}
	ctx := context.Background()
	if _, err := r.Run(ctx, git.Call{Dir: repo.Dir, Args: []string{"status", "--porcelain=v2", "-z"}, ReadOnly: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Run(ctx, git.Call{Dir: repo.Dir, Args: []string{"add", "--", "file.txt"}}); err != nil {
		t.Fatal(err)
	}

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

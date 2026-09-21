package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

// OpenWorktree implements core.API.
func (c *Core) OpenWorktree(ctx context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
	wt, err := c.sessions.Open(ctx, path)
	if err != nil {
		return core.WorktreeInfo{}, err
	}
	return core.WorktreeInfo{ID: wt.ID, Path: wt.Path, RepoKey: wt.Repo.Key}, nil
}

// Summary implements core.API.
func (c *Core) Summary(ctx context.Context, id core.WorktreeID) (core.Summary, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.Summary{}, err
	}

	result, err := wt.Runner.Run(ctx, git.Call{
		Dir: wt.Path, ReadOnly: true,
		Args: []string{"symbolic-ref", "--short", "-q", "HEAD"},
	})
	// exit 1 with empty stdout = detached HEAD (git-symbolic-ref: "-q ...
	// exit with non-zero status"); any other failure is returned as-is
	// (QA-REL-02: show it; no more specific code exists yet).
	var ce *core.Error
	detached := errors.As(err, &ce) && ce.Code == core.GitFailed && result.ExitCode == 1 && len(result.Stdout) == 0
	if err != nil && !detached {
		return core.Summary{}, err
	}

	branch := strings.TrimSpace(string(result.Stdout))
	return core.Summary{Path: wt.Path, Branch: branch, Detached: detached, Operation: operation(wt.GitDir)}, nil
}

// operation checks the state files git-mapping.md names, in the worktree's
// own git dir. File checks only, no subprocess.
func operation(gitDir string) string {
	for _, f := range []struct{ name, op string }{
		{"MERGE_HEAD", "merge"}, {"CHERRY_PICK_HEAD", "cherry-pick"}, {"REVERT_HEAD", "revert"},
	} {
		if fileExists(filepath.Join(gitDir, f.name)) {
			return f.op
		}
	}
	for _, dir := range []string{"rebase-merge", "rebase-apply"} {
		if fileExists(filepath.Join(gitDir, dir)) {
			return "rebase"
		}
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Refresh implements core.API.
func (c *Core) Refresh(ctx context.Context, id core.WorktreeID) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Repo.Watcher().CheckNow()
	wt.Poller().CheckNow()
	return nil
}

// SetFocused implements core.API.
func (c *Core) SetFocused(ctx context.Context, id core.WorktreeID, focused bool) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	if focused {
		wt.Poller().Focus()
	} else {
		wt.Poller().Blur()
	}
	return nil
}

// CloseWorktree implements core.API.
func (c *Core) CloseWorktree(ctx context.Context, id core.WorktreeID) error {
	c.sessions.Close(id)
	return nil
}

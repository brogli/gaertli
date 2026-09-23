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

	branch, detached, err := headBranch(ctx, wt.Runner, wt.Path)
	if err != nil {
		return core.Summary{}, err
	}
	sha, err := verifyRev(ctx, wt.Runner, wt.Path, "HEAD")
	if err != nil {
		return core.Summary{}, err
	}
	stash, err := pullStash(ctx, wt.Runner, wt.Path)
	if err != nil {
		return core.Summary{}, err
	}
	return core.Summary{
		Path: wt.Path, Branch: branch, Detached: detached, Operation: operation(wt.GitDir), SHA: sha,
		EditorName: c.currentProfile().Name, MessageDraft: wt.MessageDraft(), PullStash: stash,
	}, nil
}

// headBranch returns HEAD's branch name, or detached if HEAD is not a
// symbolic ref.
func headBranch(ctx context.Context, r git.Runner, dir string) (branch string, detached bool, err error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true, Background: true,
		Args: []string{"symbolic-ref", "--short", "-q", "HEAD"},
	})
	if quietFailure(err, result) {
		return "", true, nil
	}
	return strings.TrimSpace(string(result.Stdout)), false, err
}

// verifyRev resolves rev to its object name, "" if rev does not exist (an
// unborn HEAD, no stash).
func verifyRev(ctx context.Context, r git.Runner, dir, rev string) (string, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true, Background: true,
		Args: []string{"rev-parse", "-q", "--verify", "--end-of-options", rev},
	})
	if err != nil && !quietFailure(err, result) {
		return "", err
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

// quietFailure reports whether err is a -q call's "nothing to report": exit
// 1 with empty stdout.
func quietFailure(err error, result git.Result) bool {
	return gitExited(err, result, 1) && len(result.Stdout) == 0
}

// gitFailure returns err's *core.Error if it is git's own failure (Code
// GitFailed), else nil.
func gitFailure(err error) *core.Error {
	if ce, ok := errors.AsType[*core.Error](err); ok && ce.Code == core.GitFailed {
		return ce
	}
	return nil
}

// gitExited reports whether err is git's own failure with exit code code.
func gitExited(err error, result git.Result, code int) bool {
	return gitFailure(err) != nil && result.ExitCode == code
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

// remapIfPending reclassifies git's failure as Code Conflicts if pending: an
// operation-state file was left behind, so git stopped mid-way rather than
// refusing outright.
func remapIfPending(err error, pending bool) error {
	if ce := gitFailure(err); ce != nil && pending {
		ce.Code = core.Conflicts
	}
	return err
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

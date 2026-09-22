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
		Dir: wt.Path, ReadOnly: true, Background: true,
		Args: []string{"symbolic-ref", "--short", "-q", "HEAD"},
	})
	// exit 1 with empty stdout = detached HEAD (git-symbolic-ref: "-q ...
	// exit with non-zero status"); any other failure is returned as-is
	// (QA-REL-02: show it; no more specific code exists yet).
	detached := quietFailure(err, result)
	if err != nil && !detached {
		return core.Summary{}, err
	}

	branch := strings.TrimSpace(string(result.Stdout))

	sha, err := resolveHeadSHA(ctx, wt.Runner, wt.Path)
	if err != nil {
		return core.Summary{}, err
	}
	return core.Summary{
		Path: wt.Path, Branch: branch, Detached: detached, Operation: operation(wt.GitDir), SHA: sha,
		EditorName: c.currentProfile().Name,
	}, nil
}

// resolveHeadSHA resolves HEAD's own commit (QA-REL-02): "" for an unborn
// HEAD, never an error — same exit-1/empty-stdout shape as the detached
// check just above it (quietFailure). Named apart from the test suite's own
// headSHA(t, dir) helper (log_test.go), already established across this
// package's tests (Design note: the spec's own draft named this the same as
// that helper — adapted to the code found, not the spec's exact name).
func resolveHeadSHA(ctx context.Context, r git.Runner, dir string) (string, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true, Background: true,
		Args: []string{"rev-parse", "-q", "--verify", "--end-of-options", "HEAD"},
	})
	if err != nil && !quietFailure(err, result) {
		return "", err
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

// quietFailure reports whether err is git's own "-q ... nothing to report"
// shape — a Code GitFailed, exit-1 result with empty stdout — rather than a
// real failure: an unattached HEAD's symbolic-ref, or an unborn HEAD's
// rev-parse, both used above. Shared so this exit-code/empty-stdout idiom
// (errors-security.md: classified from exit code, not stderr text) is
// checked identically in both places instead of copy-pasted.
func quietFailure(err error, result git.Result) bool {
	var ce *core.Error
	return errors.As(err, &ce) && ce.Code == core.GitFailed && result.ExitCode == 1 && len(result.Stdout) == 0
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

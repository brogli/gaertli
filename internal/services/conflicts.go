package services

import (
	"context"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

// ContinueOperation implements core.API.
func (c *Core) ContinueOperation(ctx context.Context, id core.WorktreeID) error {
	return c.runOperationVerb(ctx, id, "--continue")
}

// AbortOperation implements core.API.
func (c *Core) AbortOperation(ctx context.Context, id core.WorktreeID) error {
	return c.runOperationVerb(ctx, id, "--abort")
}

// runOperationVerb is ContinueOperation's and AbortOperation's shared body:
// the current operation kind (repo.go's operation(), "merge", "rebase",
// "cherry-pick", or "revert") is itself the git subcommand to run flag
// against, reclassifying a still-conflicted refusal.
func (c *Core) runOperationVerb(ctx context.Context, id core.WorktreeID, flag string) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Lock() // continue/abort write this worktree's index and files
	defer wt.Unlock()

	kind := operation(wt.GitDir)
	if kind == "" {
		return core.Errorf(core.Internal, "no operation in progress")
	}
	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Continue: true, Args: []string{kind, flag}})
	return remapOperationConflict(err, wt.GitDir)
}

// remapOperationConflict classifies a failed continue, abort, or
// pull-integrate call: a state file left behind under gitDir means the
// operation is still stopped, Code Conflicts; no state file means it
// genuinely finished, or was never running, so any error is unrelated and
// passed through as-is.
func remapOperationConflict(err error, gitDir string) error {
	return remapIfPending(err, operation(gitDir) != "")
}

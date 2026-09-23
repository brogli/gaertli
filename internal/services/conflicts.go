package services

import (
	"context"
	"slices"

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

// SkipOperation implements core.API. Merge has no --skip, and rebase's is
// not supported.
func (c *Core) SkipOperation(ctx context.Context, id core.WorktreeID) error {
	return c.runOperationVerb(ctx, id, "--skip", "cherry-pick", "revert")
}

// runOperationVerb runs flag against the in-progress operation, whose kind
// ("merge", "rebase", "cherry-pick", "revert") is itself the git subcommand.
// allowed, if given, limits the kinds flag runs against.
func (c *Core) runOperationVerb(ctx context.Context, id core.WorktreeID, flag string, allowed ...string) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Lock() // continue/abort/skip write this worktree's index and files
	defer wt.Unlock()

	kind := operation(wt.GitDir)
	if kind == "" || (len(allowed) > 0 && !slices.Contains(allowed, kind)) {
		return core.Errorf(core.Internal, "no operation in progress")
	}
	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Continue: true, Args: []string{kind, flag}})
	return remapOperationConflict(err, wt.GitDir)
}

// remapOperationConflict reclassifies a failed operation step as Code
// Conflicts while an operation-state file is left under gitDir: the
// operation is still stopped. Without one, err is unrelated and returned
// as-is.
func remapOperationConflict(err error, gitDir string) error {
	return remapIfPending(err, operation(gitDir) != "")
}

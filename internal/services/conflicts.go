package services

import (
	"context"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/session"
)

// ContinueOperation implements core.API.
func (c *Core) ContinueOperation(ctx context.Context, id core.WorktreeID) error {
	return c.runOperationVerb(ctx, id, "--continue")
}

// AbortOperation implements core.API.
func (c *Core) AbortOperation(ctx context.Context, id core.WorktreeID) error {
	return c.runOperationVerb(ctx, id, "--abort")
}

// SkipOperation implements core.API.
func (c *Core) SkipOperation(ctx context.Context, id core.WorktreeID) error {
	return c.runOperationVerb(ctx, id, "--skip")
}

// runOperationVerb runs flag against the in-progress operation, whose kind
// ("merge", "rebase", "cherry-pick", "revert") is itself the git subcommand.
// Only a cherry-pick or revert is skipped (merge has no --skip, rebase's is
// not offered). A refusal stands in for git's own, as GitFailed. Continue
// and skip may commit with a message, so no editor waits; abort never commits.
func (c *Core) runOperationVerb(ctx context.Context, id core.WorktreeID, flag string) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Lock() // continue/abort/skip write this worktree's index and files
	defer wt.Unlock()

	kind := operation(wt.GitDir)
	switch {
	case kind == "":
		return core.ErrorfArgs(core.GitFailed, "noOperation", nil, "No merge, rebase, cherry-pick or revert is in progress.")
	case flag == "--skip" && kind != "cherry-pick" && kind != "revert":
		return core.ErrorfArgs(core.GitFailed, "operationNotSkippable", map[string]string{"operation": kind}, "A %s can't be skipped; only a cherry-pick or revert can.", kind)
	}
	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, NoEditor: flag != "--abort", Timeout: operationTimeout, Args: []string{kind, flag}})
	return stopOf(ctx, wt, err)
}

// stopOf returns err from an operation step as it is, unless the step
// left an operation in progress: then it is a stop left for continue or
// abort (asStop). The status read ignores ctx's end, so a step that timed
// out is still told apart.
func stopOf(ctx context.Context, wt *session.Worktree, err error) error {
	if err == nil || operation(wt.GitDir) == "" {
		return err
	}
	st, statusErr := readHeadStatus(context.WithoutCancel(ctx), wt)
	return asStop(err, statusErr == nil && st.conflicted)
}

// asStop reclassifies err, from a step that left an operation in progress,
// as a stop: Code Conflicts with conflicted files, else Code Stopped,
// whatever err's code was (a timeout included).
func asStop(err error, conflicted bool) error {
	ce := core.ErrorOf(err)
	ce.Code = core.Stopped
	if conflicted {
		ce.Code = core.Conflicts
	}
	return ce
}

// operationTimeout bounds a rebase, or one --continue/--abort/--skip of an
// operation: replaying many commits, or a hook or signing prompt on the
// way, outlasts the runner's default, and a kill mid-way leaves the
// operation half done.
const operationTimeout = 10 * time.Minute

// remapOperationConflict reclassifies git's failure as Code Conflicts while
// an operation is still in progress under gitDir.
func remapOperationConflict(err error, gitDir string) error {
	return remapIfPending(err, operation(gitDir) != "")
}

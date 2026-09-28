package services

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

// CreateBranch implements core.API.
func (c *Core) CreateBranch(ctx context.Context, id core.WorktreeID, name, start string, checkout bool) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}

	if err := validateBranchName(ctx, wt.Runner, wt.Path, name); err != nil {
		return err // a read-only check: not worth holding a write lock for
	}

	if checkout {
		wt.Lock() // create+checkout writes this worktree's HEAD/index
		defer wt.Unlock()
	} else {
		wt.Repo.Lock() // a plain ref create is a repo-wide one-off
		defer wt.Repo.Unlock()
	}

	args := []string{"branch", "--", name, start}
	if checkout {
		args = []string{"switch", "-c", name, "--end-of-options", start} // atomic: no orphan branch if the checkout fails
	}
	if startsAtARemoteBranch(start) { // its own first push gives it its own upstream, not this one's
		args = slices.Insert(args, 1, "--no-track")
	}
	result, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: args})
	return remapDirty(err, result)
}

// startsAtARemoteBranch reports whether start is a remote-tracking branch's
// full ref.
func startsAtARemoteBranch(start string) bool {
	return strings.HasPrefix(start, "refs/remotes/")
}

// validateBranchName rejects an invalid name before any mutating call as
// Code InvalidName, from check-ref-format's exit code.
func validateBranchName(ctx context.Context, r git.Runner, dir, name string) error {
	_, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: []string{"check-ref-format", "--branch", name}, // safe without a -- of its own
	})
	if ce, ok := errors.AsType[*core.Error](err); ok {
		ce.Retag(core.InvalidName, "invalidBranchName", map[string]string{"name": name}, "%q is not a valid branch name", name)
	}
	return err
}

// Checkout implements core.API. A remote-tracking branch goes by its full
// ref with --track: a bare name's DWIM refuses when a tag has that name.
func (c *Core) Checkout(ctx context.Context, id core.WorktreeID, branch string) error {
	args := []string{"switch", "--end-of-options", branch}
	if startsAtARemoteBranch(branch) {
		args = []string{"switch", "--track", "--end-of-options", branch}
	}
	result, err := c.runLocked(ctx, id, args...)
	return remapDirty(err, result)
}

// CheckoutCommit implements core.API.
func (c *Core) CheckoutCommit(ctx context.Context, id core.WorktreeID, sha string) error {
	result, err := c.runLocked(ctx, id, "switch", "--detach", "--end-of-options", sha)
	return remapDirty(err, result)
}

// Reset implements core.API.
func (c *Core) Reset(ctx context.Context, id core.WorktreeID, sha string, mode core.ResetMode) (core.ResetResult, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.ResetResult{}, err
	}
	flag, err := resetFlag(mode)
	if err != nil {
		return core.ResetResult{}, err
	}

	wt.Lock() // reset writes this worktree's index/files
	defer wt.Unlock()

	previous, err := verifyRev(ctx, wt.Runner, wt.Path, "HEAD") // "" only for an unborn HEAD
	if err != nil {
		return core.ResetResult{}, err
	}
	// no --end-of-options: sha is always an already-resolved hash
	if _, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"reset", flag, sha}}); err != nil {
		return core.ResetResult{}, err
	}
	return core.ResetResult{PreviousHead: previous}, nil
}

// UncommittedCount implements core.API. --no-renames and
// --untracked-files=no together guarantee every record is exactly one
// NUL-terminated field, so the count is the field count.
func (c *Core) UncommittedCount(ctx context.Context, id core.WorktreeID) (int, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return 0, err
	}
	result, err := wt.Runner.Run(ctx, git.Call{
		Dir: wt.Path, ReadOnly: true,
		Args: []string{"status", "--porcelain=v2", "-z", "--untracked-files=no", "--no-renames"},
	})
	if err != nil {
		return 0, err
	}
	return bytes.Count(result.Stdout, []byte{0}), nil
}

// resetFlag maps mode to reset's flag. An unknown mode is a caller bug: Code
// Internal.
func resetFlag(mode core.ResetMode) (string, error) {
	switch mode {
	case core.ResetSoft:
		return "--soft", nil
	case core.ResetMixed:
		return "--mixed", nil
	case core.ResetHard:
		return "--hard", nil
	case core.ResetKeep:
		return "--keep", nil
	default:
		return "", core.Errorf(core.Internal, "unknown reset mode %q", mode)
	}
}

// remapDirty turns a switch refused with exit 1 (a local change or untracked
// file would be overwritten) into Code DirtyWorktree. Every other failure
// (exit 128) stays GitFailed with git's own message.
func remapDirty(err error, result git.Result) error {
	if ce := gitFailure(err); ce != nil && result.ExitCode == 1 {
		ce.Code = core.DirtyWorktree
	}
	return err
}

// RenameBranch implements core.API.
func (c *Core) RenameBranch(ctx context.Context, id core.WorktreeID, name, newName string) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	if err := validateBranchName(ctx, wt.Runner, wt.Path, newName); err != nil {
		return err
	}

	wt.Repo.Lock() // branch rename is a repo-wide one-off
	defer wt.Repo.Unlock()

	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"branch", "-m", "--", name, newName}})
	return err
}

// DeleteBranch implements core.API.
func (c *Core) DeleteBranch(ctx context.Context, id core.WorktreeID, name string, force bool) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	flag := "-d"
	if force {
		flag = "-D"
	}

	wt.Repo.Lock() // branch delete is a repo-wide one-off
	defer wt.Repo.Unlock()

	result, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"branch", flag, "--", name}})
	return remapUnmerged(err, result)
}

// remapUnmerged reclassifies -d's "not fully merged" refusal as Code
// Unmerged, so the caller can offer a forced retry. Every other exit-1
// refusal ("used by worktree") stays plain GitFailed: forcing does not fix
// that one.
func remapUnmerged(err error, result git.Result) error {
	if ce := gitFailure(err); ce != nil && result.ExitCode == 1 && bytes.Contains(result.Stderr, []byte("is not fully merged")) {
		ce.Code = core.Unmerged
	}
	return err
}

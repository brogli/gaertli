package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"

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
		wt.Lock() // ADR-6: create+checkout writes this worktree's HEAD/index
		defer wt.Unlock()
	} else {
		wt.Repo.Lock() // ADR-6: a plain ref create is a repo-wide one-off
		defer wt.Repo.Unlock()
	}

	args := []string{"branch", "--", name, start}
	if checkout {
		args = []string{"switch", "-c", name, "--end-of-options", start} // atomic: no orphan branch if the checkout fails
	}
	result, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: args})
	return remapDirty(err, result)
}

// validateBranchName rejects an invalid name before any mutating call
// (FR-BR-01) as Code InvalidName, from check-ref-format's own exit code,
// never from stderr text (TR-08).
func validateBranchName(ctx context.Context, r git.Runner, dir, name string) error {
	_, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: []string{"check-ref-format", "--branch", name}, // safe without a -- of its own
	})
	var ce *core.Error
	if errors.As(err, &ce) {
		ce.Code = core.InvalidName
		ce.Message = fmt.Sprintf("%q is not a valid branch name", name)
	}
	return err
}

// Checkout implements core.API.
func (c *Core) Checkout(ctx context.Context, id core.WorktreeID, branch string) error {
	return c.switchWorktree(ctx, id, []string{"switch", "--end-of-options", branch})
}

// CheckoutCommit implements core.API.
func (c *Core) CheckoutCommit(ctx context.Context, id core.WorktreeID, sha string) error {
	return c.switchWorktree(ctx, id, []string{"switch", "--detach", "--end-of-options", sha})
}

// switchWorktree runs a git switch call under id's worktree write lock
// (ADR-6) and applies remapDirty.
func (c *Core) switchWorktree(ctx context.Context, id core.WorktreeID, args []string) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Lock()
	defer wt.Unlock()

	result, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: args})
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

	wt.Lock() // ADR-6: reset writes this worktree's index/files
	defer wt.Unlock()

	previous, err := resolveHeadSHA(ctx, wt.Runner, wt.Path) // "" only for an unborn HEAD
	if err != nil {
		return core.ResetResult{}, err
	}
	if _, err := wt.Runner.Run(ctx, git.Call{
		Dir: wt.Path, Args: []string{"reset", flag, sha},
		// no ReadOnly (this writes), no --end-of-options/-- (sha is always
		// an already-resolved hash)
	}); err != nil {
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

// resetFlag maps mode to reset's own flag. mode is a closed UI choice,
// never free text, so an unrecognised value is this app's own bug: Code
// Internal, not git's failure.
func resetFlag(mode core.ResetMode) (string, error) {
	switch mode {
	case core.ResetSoft:
		return "--soft", nil
	case core.ResetMixed:
		return "--mixed", nil
	case core.ResetHard:
		return "--hard", nil
	default:
		return "", core.Errorf(core.Internal, "unknown reset mode %q", mode)
	}
}

// remapDirty turns a "would be overwritten" checkout refusal (exit 1) into
// Code DirtyWorktree: a tracked file's local change, or an untracked file,
// in the way of a switch. Every other failure (exit 128 "fatal:": bad or
// duplicate branch name, an ambiguous multi-remote DWIM match, the branch
// already checked out elsewhere) stays plain GitFailed with git's own
// message, per FR-BR-05.
func remapDirty(err error, result git.Result) error {
	var ce *core.Error
	if errors.As(err, &ce) && ce.Code == core.GitFailed && result.ExitCode == 1 {
		ce.Code = core.DirtyWorktree
	}
	return err
}

package services

import (
	"bytes"
	"context"
	"strings"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/session"
)

// ListWorktrees implements core.API.
func (c *Core) ListWorktrees(ctx context.Context, path core.RepoPath) ([]core.Worktree, error) {
	toplevel, _, _, err := c.sessions.Resolve(ctx, path)
	if err != nil {
		return nil, err
	}
	result, err := c.discover.Run(ctx, git.Call{
		Dir: toplevel, ReadOnly: true, Args: []string{"worktree", "list", "--porcelain", "-z"},
	})
	if err != nil {
		return nil, err
	}

	worktrees := parseWorktreeRecords(result.Stdout)
	for i := range worktrees {
		if !worktrees[i].Missing {
			worktrees[i].Dirty = c.isDirty(ctx, worktrees[i].Path)
		}
	}
	return worktrees, nil
}

// isDirty runs the same status call the poller uses. A run error counts as
// clean: this is display data, and the error surfaces when the worktree is
// opened.
func (c *Core) isDirty(ctx context.Context, dir string) bool {
	result, err := c.discover.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true, Args: []string{"status", "--porcelain=v2", "-z", "--untracked-files=all"},
	})
	return err == nil && len(result.Stdout) > 0
}

// ListBranches implements core.API.
func (c *Core) ListBranches(ctx context.Context, path core.RepoPath) ([]core.BranchSummary, error) {
	toplevel, _, _, err := c.sessions.Resolve(ctx, path)
	if err != nil {
		return nil, err
	}
	result, err := c.discover.Run(ctx, git.Call{
		Dir: toplevel, ReadOnly: true,
		Args: []string{"for-each-ref", "--format=%(refname:short)%1f%(worktreepath)%00", "refs/heads"},
	})
	if err != nil {
		return nil, err
	}
	return parseBranchSummaries(result.Stdout), nil
}

// AddWorktree implements core.API.
func (c *Core) AddWorktree(ctx context.Context, path core.RepoPath, target, branch, start string, newBranch bool) (core.Worktree, error) {
	wt, err := c.sessions.Open(ctx, path) // path is an existing worktree of the repo
	if err != nil {
		return core.Worktree{}, err
	}
	wt.Repo.Lock() // worktree add writes repo-wide refs, like creating a branch
	defer wt.Repo.Unlock()

	args := []string{"worktree", "add", "--", target, branch}
	if newBranch {
		if err := validateBranchName(ctx, wt.Runner, wt.Path, branch); err != nil { // worktree add -b's name argument is not protected by --
			return core.Worktree{}, err
		}
		args = []string{"worktree", "add", "-b", branch, "--", target, start}
	}
	if _, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: args}); err != nil {
		return core.Worktree{}, err
	}
	return c.describeWorktree(ctx, wt, target)
}

// describeWorktree returns target's row from wt's repo's worktree list.
func (c *Core) describeWorktree(ctx context.Context, wt *session.Worktree, target string) (core.Worktree, error) {
	result, err := wt.Runner.Run(ctx, git.Call{
		Dir: wt.Path, ReadOnly: true, Args: []string{"worktree", "list", "--porcelain", "-z"},
	})
	if err != nil {
		return core.Worktree{}, err
	}
	for _, row := range parseWorktreeRecords(result.Stdout) {
		if row.Path == target {
			return row, nil // just created: clean by construction
		}
	}
	return core.Worktree{}, core.Errorf(core.Internal, "worktree %q not found after creation", target)
}

// parseWorktreeRecords parses "git worktree list --porcelain -z" output
// (each field is its own NUL-terminated "key[ value]"; an empty field ends
// a record). git lists the main worktree first and porcelain has no marker
// for it, so Main is set from position alone. Dirty is left to the caller.
func parseWorktreeRecords(out []byte) []core.Worktree {
	var worktrees []core.Worktree
	for _, block := range bytes.Split(out, []byte{0, 0}) {
		if len(block) == 0 {
			continue
		}
		worktrees = append(worktrees, parseWorktreeRecord(block, len(worktrees) == 0))
	}
	return worktrees
}

func parseWorktreeRecord(block []byte, main bool) core.Worktree {
	wt := core.Worktree{Main: main}
	for _, line := range bytes.Split(block, []byte{0}) {
		key, value, _ := strings.Cut(string(line), " ")
		switch key {
		case "worktree":
			wt.Path = value
		case "HEAD":
			wt.SHA = value
		case "branch":
			wt.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "detached":
			wt.Detached = true
		case "locked":
			wt.Locked = true
		case "prunable":
			wt.Missing = true
		}
	}
	return wt
}

// parseBranchSummaries parses ListBranches' for-each-ref output.
func parseBranchSummaries(out []byte) []core.BranchSummary {
	var summaries []core.BranchSummary
	for _, record := range nulRecords(out) {
		name, worktree, _ := strings.Cut(record, "\x1f")
		summaries = append(summaries, core.BranchSummary{Name: name, CheckedOutIn: worktree})
	}
	return summaries
}

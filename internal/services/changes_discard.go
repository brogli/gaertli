package services

import (
	"context"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
)

// DiscardFile implements core.API. An untracked directory, which Status
// reports as one "?" entry, needs no -d: an explicit pathspec naming it
// removes it.
func (c *Core) DiscardFile(ctx context.Context, id core.WorktreeID, path string, untracked bool) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Lock() // discard writes this worktree's files
	defer wt.Unlock()

	literal := literalPath(path)
	args := []string{"restore", "--worktree", "--", literal}
	if untracked {
		args = []string{"clean", "-f", "--", literal}
	}
	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: args})
	return err
}

// DiscardLines implements core.API: sel's lines of the unstaged diff,
// reverse-applied to the working tree (no --cached).
func (c *Core) DiscardLines(ctx context.Context, id core.WorktreeID, path string, sel core.LineSelection) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Lock()
	defer wt.Unlock()

	patch, err := selectionPatch(ctx, wt.Runner, wt.Path, path, sel, false, true)
	if err != nil {
		return err
	}
	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"apply", "--recount", "--reverse", "-"}, Stdin: patch})
	return err
}

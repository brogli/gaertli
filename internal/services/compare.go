package services

import (
	"cmp"
	"context"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

// CompareCommits implements core.API. Two-revision diff (a b), not
// merge-base: swapping a and b exactly mirrors every file's status.
func (c *Core) CompareCommits(ctx context.Context, id core.WorktreeID, a, b string) ([]core.ChangedFile, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return nil, err
	}
	result, err := wt.Runner.Run(ctx, git.Call{
		Dir: wt.Path, ReadOnly: true,
		Args: []string{"diff", "--name-status", "-z", "-M", "--end-of-options", a, b},
	})
	if err != nil {
		return nil, err
	}
	return parseNameStatus(result.Stdout)
}

// CompareWithWorking implements core.API. Single-revision diff (sha, no
// second revision): git compares it against the working tree implicitly,
// bypassing the index entirely — unlike CompareCommits' two-revision form.
func (c *Core) CompareWithWorking(ctx context.Context, id core.WorktreeID, sha string) ([]core.ChangedFile, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return nil, err
	}
	result, err := wt.Runner.Run(ctx, git.Call{
		Dir: wt.Path, ReadOnly: true,
		Args: []string{"diff", "--name-status", "-z", "-M", "--end-of-options", sha},
	})
	if err != nil {
		return nil, err
	}
	return parseNameStatus(result.Stdout)
}

// DiffRange implements core.API.
func (c *Core) DiffRange(ctx context.Context, id core.WorktreeID, a, b, path, oldPath string, force bool) (core.DiffFile, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.DiffFile{}, err
	}
	sides := diffSidesSpec{
		oldRev: a + ":" + cmp.Or(oldPath, path), newRev: b + ":" + path,
		path: path, oldPath: oldPath, force: force,
		isBinary: func() (bool, error) {
			return numstatIsBinary(ctx, wt.Runner, wt.Path, oldPath, path, "diff", "--numstat", "-z", "-M", "--end-of-options", a, b)
		},
	}
	return diffSides(ctx, wt.Runner, wt.Path, sides)
}

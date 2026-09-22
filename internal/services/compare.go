package services

import (
	"context"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

// CompareCommits implements core.API (FR-CMP-01/02). Two-revision diff
// (a b), not merge-base: swapping a and b exactly mirrors every file's
// status.
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

// diffIsBinaryBetween is diffIsBinary for two explicit revisions instead of
// one commit vs its first parent.
func diffIsBinaryBetween(ctx context.Context, r git.Runner, dir, a, b, oldPath, path string) (bool, error) {
	return numstatIsBinary(ctx, r, dir, oldPath, path,
		"diff", "--numstat", "-z", "-M", "--end-of-options", a, b)
}

// DiffRange implements core.API (FR-CMP-01/02).
func (c *Core) DiffRange(ctx context.Context, id core.WorktreeID, a, b, path, oldPath string, force bool) (core.DiffFile, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.DiffFile{}, err
	}
	sides := diffSidesSpec{
		oldRev: a + ":" + firstNonEmpty(oldPath, path), newRev: b + ":" + path,
		path: path, oldPath: oldPath, force: force,
		isBinary: func() (bool, error) { return diffIsBinaryBetween(ctx, wt.Runner, wt.Path, a, b, oldPath, path) },
	}
	return diffSides(ctx, wt.Runner, wt.Path, sides)
}

// compare.go implements CompareCommits and DiffRange (spec 21, FR-CMP-01/02):
// arbitrary two-commit comparisons, alongside DiffFile's (spec 20) one-
// commit-vs-parent comparisons in diff.go.
package services

import (
	"context"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

// CompareCommits implements core.API. Two-revision diff (a..b), not
// merge-base (Research used): swapping a and b exactly mirrors every file's
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
	return parseNameStatus(result.Stdout) // commitdetails.go, spec 10 — reused unmodified
}

// diffIsBinaryBetween is diff.go's diffIsBinary (spec 20) for two explicit
// revisions instead of one commit vs its first parent.
func diffIsBinaryBetween(ctx context.Context, r git.Runner, dir, a, b, oldPath, path string) (bool, error) {
	return numstatIsBinary(ctx, r, dir, oldPath, path,
		"diff", "--numstat", "-z", "-M", "--end-of-options", a, b)
}

// DiffRange implements core.API. Shares spec 20's driver: diffSides runs
// the size cap, the binary check, the content fetch and the UTF-8 check;
// DiffFile and DiffRange differ only in how the two revisions and the
// binary check are built (sha^/sha with git show --first-parent, vs a/b
// with git diff).
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

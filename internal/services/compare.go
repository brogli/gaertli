package services

import (
	"cmp"
	"context"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

// CompareCommits implements core.API.
func (c *Core) CompareCommits(ctx context.Context, id core.WorktreeID, a, b string) ([]core.ChangedFile, error) {
	return c.changedFiles(ctx, id, a, b)
}

// CompareWithWorking implements core.API.
func (c *Core) CompareWithWorking(ctx context.Context, id core.WorktreeID, sha string) ([]core.ChangedFile, error) {
	return c.changedFiles(ctx, id, sha)
}

// changedFiles lists the files git diff names between revs.
func (c *Core) changedFiles(ctx context.Context, id core.WorktreeID, revs ...string) ([]core.ChangedFile, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return nil, err
	}
	result, err := wt.Runner.Run(ctx, git.Call{
		Dir: wt.Path, ReadOnly: true,
		Args: append([]string{"diff", "--name-status", "-z", "-M", "--end-of-options"}, revs...),
	})
	if err != nil {
		return nil, err
	}
	return parseNameStatus(result.Stdout)
}

// DiffRange implements core.API.
func (c *Core) DiffRange(ctx context.Context, id core.WorktreeID, a, b, path, oldPath string, opts core.DiffOptions) (core.DiffFile, error) {
	old := cmp.Or(oldPath, path)
	d := fileDiff{
		path: path, oldPath: oldPath, oldRev: a + ":" + old, newRev: b + ":" + path,
		cmd: "diff", args: []string{"-M", "--end-of-options", a, b, "--", old, path},
	}
	return c.diff(ctx, id, d, opts)
}

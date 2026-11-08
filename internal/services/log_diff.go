package services

import (
	"cmp"
	"context"

	"github.com/brogli/gaertli/internal/core"
)

// DiffFile implements core.API. --root diffs a root commit against nothing
// whatever log.showRoot says.
func (c *Core) DiffFile(ctx context.Context, id core.WorktreeID, sha core.SHA, path, oldPath string, opts core.DiffOptions) (core.DiffFile, error) {
	old := cmp.Or(oldPath, path)
	d := fileDiff{
		path: path, oldPath: oldPath, oldRev: string(sha) + "^:" + old, newRev: string(sha) + ":" + path,
		cmd: "show", args: []string{"--root", "--format=", "--first-parent", "-M", "--end-of-options", string(sha), "--", literalPath(old), literalPath(path)},
	}
	return c.diff(ctx, id, d, opts)
}

// DiffAgainstWorking implements core.API.
func (c *Core) DiffAgainstWorking(ctx context.Context, id core.WorktreeID, sha core.SHA, path, oldPath string, opts core.DiffOptions) (core.DiffFile, error) {
	old := cmp.Or(oldPath, path)
	d := fileDiff{
		path: path, oldPath: oldPath, oldRev: string(sha) + ":" + old,
		cmd: "diff", args: []string{"-M", "--end-of-options", string(sha), "--", literalPath(old), literalPath(path)},
	}
	return c.diff(ctx, id, d, opts)
}

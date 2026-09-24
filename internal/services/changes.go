package services

import (
	"cmp"
	"context"
	"strings"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/watch"
)

// Status implements core.API.
func (c *Core) Status(ctx context.Context, id core.WorktreeID) (core.ChangedFiles, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.ChangedFiles{}, err
	}
	result, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, ReadOnly: true, Background: true, Args: watch.StatusArgs})
	if err != nil {
		return core.ChangedFiles{}, err
	}
	return parseStatus(result.Stdout), nil
}

// StageFile implements core.API.
func (c *Core) StageFile(ctx context.Context, id core.WorktreeID, path string) error {
	_, err := c.runLocked(ctx, id, "add", "--", path)
	return err
}

// UnstageFile implements core.API.
func (c *Core) UnstageFile(ctx context.Context, id core.WorktreeID, path string) error {
	_, err := c.runLocked(ctx, id, "restore", "--staged", "--", path)
	return err
}

// StageAll implements core.API.
func (c *Core) StageAll(ctx context.Context, id core.WorktreeID) error {
	_, err := c.runLocked(ctx, id, "add", "-A")
	return err
}

// UnstageAll implements core.API.
func (c *Core) UnstageAll(ctx context.Context, id core.WorktreeID) error {
	_, err := c.runLocked(ctx, id, "restore", "--staged", "--", ".")
	return err
}

// runLocked runs git with args in id's worktree under its write lock.
func (c *Core) runLocked(ctx context.Context, id core.WorktreeID, args ...string) (git.Result, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return git.Result{}, err
	}
	wt.Lock()
	defer wt.Unlock()
	return wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: args})
}

// Commit implements core.API.
func (c *Core) Commit(ctx context.Context, id core.WorktreeID, message string, amend bool) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Lock() // commit writes this worktree's index and HEAD
	defer wt.Unlock()

	args := []string{"commit"}
	if amend {
		args = append(args, "--amend")
	}
	args = append(args, "-F", "-")
	if _, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: args, Stdin: []byte(message)}); err != nil {
		return err
	}
	wt.SetMessageDraft("") // cleared only on success
	return nil
}

// SetMessageDraft implements core.API.
func (c *Core) SetMessageDraft(ctx context.Context, id core.WorktreeID, message string) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.SetMessageDraft(message)
	return nil
}

// parseStatus splits git status --porcelain=v2 -z output into four
// groups: each record is placed by its X (staged, vs HEAD) and Y
// (unstaged, vs the index) halves independently, so a file changed both
// ways (a "MM" record) lands in both groups with its two distinct letters.
// Header lines ("# ...") are skipped by prefix, not position — a
// "# stash <N>" line is framed identically to an ordinary record.
func parseStatus(data []byte) core.ChangedFiles {
	var out core.ChangedFiles
	records := splitRecords(data)
	for i := 0; i < len(records); i++ {
		rec := records[i]
		switch {
		case strings.HasPrefix(rec, "# "):
			// header or stash count, not a change
		case strings.HasPrefix(rec, "1 "):
			f := strings.SplitN(rec[2:], " ", 8) // 7 fixed fields, then path
			appendChange(&out, f[0], f[7], "")
		case strings.HasPrefix(rec, "2 "):
			f := strings.SplitN(rec[2:], " ", 9) // 8 fixed fields, then path
			i++                                  // origPath is the next NUL-terminated record, not a field of this one
			appendChange(&out, f[0], f[8], records[i])
		case strings.HasPrefix(rec, "u "):
			appendUnmerged(&out, rec)
		case strings.HasPrefix(rec, "? "):
			out.Untracked = append(out.Untracked, core.ChangedFile{Status: "?", Path: rec[2:]})
			// "!" (ignored) never appears: --ignored is not passed.
		}
	}
	return out
}

// splitRecords splits porcelain v2 -z output on NUL, dropping the trailing
// empty element after the last record's terminator.
func splitRecords(data []byte) []string {
	s := strings.TrimSuffix(string(data), "\x00")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\x00")
}

// appendChange places a "1" (ordinary) or "2" (rename/copy) record by its
// XY status letters; origPath, a rename's old path or "", goes on the staged
// half.
func appendChange(out *core.ChangedFiles, xy, path, origPath string) {
	if xy[0] != '.' {
		out.Staged = append(out.Staged, core.ChangedFile{Status: string(xy[0]), Path: path, OldPath: origPath})
	}
	if xy[1] != '.' {
		out.Unstaged = append(out.Unstaged, core.ChangedFile{Status: string(xy[1]), Path: path})
	}
}

// appendUnmerged handles a "u" (unmerged) record: git's own two-letter
// conflict code becomes Status as-is.
func appendUnmerged(out *core.ChangedFiles, rec string) {
	f := strings.SplitN(rec[2:], " ", 10) // 9 fixed fields, then path
	out.Conflicted = append(out.Conflicted, core.ChangedFile{Status: f[0], Path: f[9]})
}

// DiffWorking implements core.API.
func (c *Core) DiffWorking(ctx context.Context, id core.WorktreeID, path, oldPath string, staged bool, opts core.DiffOptions) (core.DiffFile, error) {
	// the index against the working tree, or HEAD against the index
	d := fileDiff{path: path, oldRev: ":" + path, cmd: "diff", args: []string{"--", path}}
	if staged {
		old := cmp.Or(oldPath, path)
		d = fileDiff{
			path: path, oldPath: oldPath, oldRev: "HEAD:" + old, newRev: ":" + path,
			cmd: "diff", args: []string{"--cached", "-M", "--", old, path},
		}
	}
	return c.diff(ctx, id, d, opts)
}

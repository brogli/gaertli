package services

import (
	"cmp"
	"context"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

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
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Lock() // staging writes this worktree's index
	defer wt.Unlock()
	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"add", "--", path}})
	return err
}

// UnstageFile implements core.API.
func (c *Core) UnstageFile(ctx context.Context, id core.WorktreeID, path string) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Lock()
	defer wt.Unlock()
	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"restore", "--staged", "--", path}})
	return err
}

// StageAll implements core.API.
func (c *Core) StageAll(ctx context.Context, id core.WorktreeID) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Lock()
	defer wt.Unlock()
	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"add", "-A"}})
	return err
}

// UnstageAll implements core.API.
func (c *Core) UnstageAll(ctx context.Context, id core.WorktreeID) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Lock()
	defer wt.Unlock()
	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"restore", "--staged", "--", "."}})
	return err
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
			appendOrdinary(&out, rec)
		case strings.HasPrefix(rec, "2 "):
			i++ // origPath is the next NUL-terminated record, not a field of this one
			appendRename(&out, rec, records[i])
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

// appendOrdinary handles a "1" (ordinary changed) record.
func appendOrdinary(out *core.ChangedFiles, rec string) {
	f := strings.SplitN(rec[2:], " ", 8) // 7 fixed fields, then path
	xy, path := f[0], f[7]
	if xy[0] != '.' {
		out.Staged = append(out.Staged, core.ChangedFile{Status: string(xy[0]), Path: path})
	}
	if xy[1] != '.' {
		out.Unstaged = append(out.Unstaged, core.ChangedFile{Status: string(xy[1]), Path: path})
	}
}

// appendRename handles a "2" (rename/copy) record: origPath is the next
// NUL-terminated token in the stream, not a field of this record.
func appendRename(out *core.ChangedFiles, rec, origPath string) {
	f := strings.SplitN(rec[2:], " ", 9) // 8 fixed fields, then path
	xy, path := f[0], f[8]
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

// DiffWorking implements core.API. The staged case is a thin wrapper over
// DiffFile's own diffSides: HEAD and the index are both git objects, so
// only the two revision strings differ from DiffFile's sha^/sha pair.
func (c *Core) DiffWorking(ctx context.Context, id core.WorktreeID, path, oldPath string, staged, force bool) (core.DiffFile, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.DiffFile{}, err
	}
	old := cmp.Or(oldPath, path)
	if !staged {
		return diffWorkingTree(ctx, wt.Runner, wt.Path, path, old, force)
	}

	spec := diffSidesSpec{
		oldRev: "HEAD:" + old, newRev: ":" + path,
		path: path, oldPath: oldPath, force: force,
		isBinary: func() (bool, error) {
			return numstatIsBinary(ctx, wt.Runner, wt.Path, oldPath, path, "diff", "--cached", "-z", "-M")
		},
	}
	return diffSides(ctx, wt.Runner, wt.Path, spec)
}

// diffWorkingTree handles DiffWorking's !staged path: the index (or
// nothing, for an untracked file) against the literal working-tree file,
// which is not a git object until staged, so it is read straight off disk
// rather than via cat-file — the one place in this codebase where a diff
// side isn't a git object. Mirrors diffSides' size-cap -> binary-check ->
// content-fetch -> UTF-8-check shape.
func diffWorkingTree(ctx context.Context, r git.Runner, dir, path, indexPath string, force bool) (core.DiffFile, error) {
	oldSize, oldMissing, err := indexBlobSize(ctx, r, dir, indexPath)
	if err != nil {
		return core.DiffFile{}, err
	}
	full := filepath.Join(dir, path)
	info, err := os.Stat(full)
	if err != nil {
		return core.DiffFile{}, core.Errorf(core.Internal, "stat %s: %v", path, err)
	}

	if !force && (oldSize > maxDiffFileSize || info.Size() > maxDiffFileSize) {
		return core.DiffFile{Path: path, TooLarge: true}, nil
	}

	binary, err := diffWorkingIsBinary(ctx, r, dir, path, oldMissing)
	if err != nil {
		return core.DiffFile{}, err
	}
	if binary {
		return core.DiffFile{Path: path, Binary: true}, nil
	}

	newContent, err := os.ReadFile(full)
	if err != nil {
		return core.DiffFile{}, core.Errorf(core.Internal, "read %s: %v", path, err)
	}
	var oldContent []byte
	if !oldMissing {
		if oldContent, err = indexBlobContent(ctx, r, dir, indexPath); err != nil {
			return core.DiffFile{}, err
		}
	}
	if !utf8.Valid(oldContent) || !utf8.Valid(newContent) {
		return core.DiffFile{Path: path, Binary: true}, nil
	}
	return core.DiffFile{Path: path, OldContent: string(oldContent), NewContent: string(newContent)}, nil
}

// indexBlobSize is diffBlobSizes' single-object counterpart, for the index
// side of a working-tree diff alone.
func indexBlobSize(ctx context.Context, r git.Runner, dir, path string) (size int64, missingEntirely bool, err error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: []string{"cat-file", "--batch-check=%(objectsize)"}, Stdin: []byte(":" + path + "\n"),
	})
	if err != nil {
		return 0, false, err
	}
	line := strings.TrimRight(string(result.Stdout), "\n")
	if missing(line) {
		return 0, true, nil
	}
	size, err = blobSize(line)
	return size, false, err
}

// indexBlobContent is diffBlobContent's single-object counterpart, for the
// index side of a working-tree diff alone.
func indexBlobContent(ctx context.Context, r git.Runner, dir, path string) ([]byte, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: []string{"cat-file", "--batch"}, Stdin: []byte(":" + path + "\n"),
	})
	if err != nil {
		return nil, err
	}
	blobs, err := parseCatFileBatch(result.Stdout, 1)
	if err != nil {
		return nil, err
	}
	return blobs[0], nil
}

// diffWorkingIsBinary classifies path the way git itself would: an
// index-tracked path compares index to worktree exactly like diffIsBinary's
// own git diff shape (always exits 0); an untracked path has no index side
// to compare against, so it diffs against /dev/null via --no-index — the
// one call in this codebase that exits 1 on a real difference, tolerated
// the same way resolveHeadSHA (repo.go) tolerates an expected exit 1.
func diffWorkingIsBinary(ctx context.Context, r git.Runner, dir, path string, untracked bool) (bool, error) {
	if !untracked {
		return numstatIsBinary(ctx, r, dir, "", path, "diff", "--numstat", "-z", "-M")
	}
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: []string{"diff", "--no-index", "--numstat", "-z", "--", os.DevNull, path},
	})
	differs := gitExited(err, result, 1)
	if err != nil && !differs {
		return false, err
	}
	return parseBinaryNumstat(result.Stdout), nil
}

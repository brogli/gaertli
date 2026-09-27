package services

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

// maxDiffFileSize caps a Diff* call's auto-loaded content. Force bypasses
// it; nothing bypasses Binary.
const maxDiffFileSize = 5 * 1024 * 1024

// fullContext is -U's largest value: git prints one hunk holding every line
// of both sides.
const fullContext = "-U2147483647"

// DiffFile implements core.API. --root diffs a root commit against nothing
// whatever log.showRoot says.
func (c *Core) DiffFile(ctx context.Context, id core.WorktreeID, sha, path, oldPath string, opts core.DiffOptions) (core.DiffFile, error) {
	old := cmp.Or(oldPath, path)
	d := fileDiff{
		path: path, oldPath: oldPath, oldRev: sha + "^:" + old, newRev: sha + ":" + path,
		cmd: "show", args: []string{"--root", "--format=", "--first-parent", "-M", "--end-of-options", sha, "--", old, path},
	}
	return c.diff(ctx, id, d, opts)
}

// DiffAgainstWorking implements core.API.
func (c *Core) DiffAgainstWorking(ctx context.Context, id core.WorktreeID, sha, path, oldPath string, opts core.DiffOptions) (core.DiffFile, error) {
	old := cmp.Or(oldPath, path)
	d := fileDiff{
		path: path, oldPath: oldPath, oldRev: sha + ":" + old,
		cmd: "diff", args: []string{"-M", "--end-of-options", sha, "--", old, path},
	}
	return c.diff(ctx, id, d, opts)
}

// diff runs d in id's worktree.
func (c *Core) diff(ctx context.Context, id core.WorktreeID, d fileDiff, opts core.DiffOptions) (core.DiffFile, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.DiffFile{}, err
	}
	return d.run(ctx, wt.Runner, wt.Path, opts)
}

// fileDiff is one Diff* call: the file, its two sides' objects, and the git
// diff or show that compares them. Both paths go in the pathspec so -M
// still pairs a rename.
type fileDiff struct {
	path, oldPath  string
	oldRev, newRev string   // the sides' objects; newRev "" for the working-tree file
	cmd            string   // "diff" or "show"
	args           []string // after the patch options: revisions and pathspec
	noIndex        bool     // git diff --no-index, which exits 1 on a difference
}

// run caps the size, then takes both sides from git's full-context diff and
// the hunks from its -U0 diff.
func (d fileDiff) run(ctx context.Context, r git.Runner, dir string, opts core.DiffOptions) (core.DiffFile, error) {
	out := core.DiffFile{OldPath: d.oldPath, Path: d.path}
	tooLarge, untracked, err := d.check(ctx, r, dir)
	if err != nil {
		return core.DiffFile{}, err
	}
	if tooLarge && !opts.Force {
		out.TooLarge = true
		return out, nil
	}
	if untracked { // git diff lists tracked files only
		d.args, d.noIndex = []string{"--no-index", "--", os.DevNull, d.path}, true
	}

	file, err := d.patch(ctx, r, dir, fullContext)
	if err != nil {
		return core.DiffFile{}, err
	}
	if file != nil && file.IsBinary {
		out.Binary = true
		return out, nil
	}
	oldContent, newContent, err := d.contents(ctx, r, dir, file)
	if err != nil {
		return core.DiffFile{}, err
	}
	if !utf8.Valid(oldContent) || !utf8.Valid(newContent) { // JSON would corrupt it
		out.Binary = true
		return out, nil
	}
	out.OldContent, out.NewContent = string(oldContent), string(newContent)
	out.Fingerprint = fingerprint(oldContent, newContent)
	if file == nil || len(file.TextFragments) == 0 {
		return out, nil
	}
	hunks, err := d.hunks(ctx, r, dir, opts.IgnoreWhitespace)
	if err != nil {
		return core.DiffFile{}, err
	}
	out.Hunks = hunks
	return out, nil
}

// check reads the sides' sizes before any content: whether one is over the
// cap, and, for a working-tree diff, whether git tracks neither the old
// path nor path.
func (d fileDiff) check(ctx context.Context, r git.Runner, dir string) (tooLarge, untracked bool, err error) {
	if d.newRev != "" {
		sizes, err := objectSizes(ctx, r, dir, d.oldRev, d.newRev)
		if err != nil {
			return false, false, err
		}
		return max(sizes[0].size, sizes[1].size) > maxDiffFileSize, false, nil
	}
	sizes, err := objectSizes(ctx, r, dir, d.oldRev, ":"+d.path)
	if err != nil {
		return false, false, err
	}
	newSize, err := workingFileSize(filepath.Join(dir, d.path))
	if err != nil {
		return false, false, core.Errorf(core.Internal, "stat %s: %v", d.path, err)
	}
	return max(sizes[0].size, newSize) > maxDiffFileSize, sizes[0].missing && sizes[1].missing, nil
}

// patch runs d's git command with the patch options opts and returns path's
// entry, nil if git lists no change to it.
func (d fileDiff) patch(ctx context.Context, r git.Runner, dir string, opts ...string) (*gitdiff.File, error) {
	result, err := r.Run(ctx, git.Call{Dir: dir, ReadOnly: true, Args: patchArgs(d.cmd, slices.Concat(opts, d.args)...)})
	differs := d.noIndex && gitExited(err, result, 1)
	if err != nil && !differs {
		return nil, err
	}
	return parsePatch(result.Stdout, d.path)
}

// contents returns both sides of file, git's full-context diff. Without a
// hunk (equal content: no change, a pure rename, a mode change) both sides
// are the old side's object.
func (d fileDiff) contents(ctx context.Context, r git.Runner, dir string, file *gitdiff.File) (oldContent, newContent []byte, err error) {
	if file == nil || len(file.TextFragments) == 0 {
		blob, err := objectContent(ctx, r, dir, d.oldRev)
		if err != nil {
			return nil, nil, err
		}
		return blob, blob, nil
	}
	if n := len(file.TextFragments); n > 1 {
		return nil, nil, core.Errorf(core.Internal, "full-context diff of %s has %d hunks", d.path, n)
	}
	oldContent, newContent = sides(file.TextFragments[0])
	return oldContent, newContent, nil
}

// sides splits a full-context hunk into its old side (context and deleted
// lines) and its new side (context and added lines).
func sides(frag *gitdiff.TextFragment) (oldContent, newContent []byte) {
	var o, n bytes.Buffer
	for _, l := range frag.Lines {
		if l.Op != gitdiff.OpAdd {
			o.WriteString(l.Line)
		}
		if l.Op != gitdiff.OpDelete {
			n.WriteString(l.Line)
		}
	}
	return o.Bytes(), n.Bytes()
}

// fingerprint identifies a diff's two sides together: DiffFile.Fingerprint,
// which a LineSelection sends back to show which diff it was taken from.
func fingerprint(oldContent, newContent []byte) string {
	h := sha256.New()
	h.Write(strconv.AppendInt(nil, int64(len(oldContent)), 10))
	h.Write([]byte{'\n'})
	h.Write(oldContent)
	h.Write(newContent)
	return hex.EncodeToString(h.Sum(nil))
}

// hunks is git's -U0 diff of d as hunk headers, with -w if
// ignoreWhitespace. --inter-hunk-context=0 overrides diff.interHunkContext,
// which joins nearby hunks with the context lines between them.
func (d fileDiff) hunks(ctx context.Context, r git.Runner, dir string, ignoreWhitespace bool) ([]core.DiffHunk, error) {
	opts := []string{"-U0", "--inter-hunk-context=0"}
	if ignoreWhitespace {
		opts = append(opts, "-w")
	}
	file, err := d.patch(ctx, r, dir, opts...)
	if err != nil || file == nil {
		return nil, err
	}
	return hunksOf(file), nil
}

// missing reports cat-file's "<request> missing" line for an absent object
// (added/deleted file, or a root commit's nonexistent parent).
func missing(line string) bool {
	return strings.HasSuffix(line, " missing")
}

// objectSize is one object's size; 0 if missing.
type objectSize struct {
	size    int64
	missing bool
}

// objectSizes runs one cat-file --batch-check for revs, so a size cap is
// enforced before any content byte is fetched. Object specs go on stdin, so
// no --end-of-options is needed.
func objectSizes(ctx context.Context, r git.Runner, dir string, revs ...string) ([]objectSize, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args:  []string{"cat-file", "--batch-check=%(objectsize)", "-Z"},
		Stdin: batchStdin(revs),
	})
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSuffix(string(result.Stdout), "\x00"), "\x00")
	if len(lines) != len(revs) {
		return nil, core.Errorf(core.Internal, "cat-file --batch-check: %d lines, want %d", len(lines), len(revs))
	}
	sizes := make([]objectSize, len(lines))
	for i, line := range lines {
		size, err := blobSize(line)
		if err != nil {
			return nil, err
		}
		sizes[i] = objectSize{size: size, missing: missing(line)}
	}
	return sizes, nil
}

// blobSize parses one --batch-check='%(objectsize)' line: a bare number, or
// a missing line, which reads as size 0 (nothing to cap or fetch).
func blobSize(line string) (int64, error) {
	if missing(line) {
		return 0, nil
	}
	n, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return 0, core.Errorf(core.Internal, "cat-file --batch-check size %q: %v", line, err)
	}
	return n, nil
}

// workingFileSize is the size of the working-tree file at full; 0 for a
// deleted file, whose new side is empty.
func workingFileSize(full string) (int64, error) {
	info, err := os.Stat(full)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// objectContent fetches rev's raw bytes with cat-file --batch, which reads
// a missing object back as nil, not an error.
func objectContent(ctx context.Context, r git.Runner, dir, rev string) ([]byte, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args:  []string{"cat-file", "--batch", "-Z"},
		Stdin: batchStdin([]string{rev}),
	})
	if err != nil {
		return nil, err
	}
	return parseCatFileBatch(result.Stdout)
}

// parseCatFileBatch reads cat-file --batch -Z's framing for one object:
// "<oid> <type> <size>\x00<size bytes of content>\x00", or a missing entry.
func parseCatFileBatch(data []byte) ([]byte, error) {
	header, content, ok := bytes.Cut(data, []byte{0})
	if !ok {
		return nil, core.Errorf(core.Internal, "cat-file --batch: unterminated header")
	}
	if missing(string(header)) {
		return nil, nil
	}
	parts := strings.SplitN(string(header), " ", 3)
	size, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	if len(parts) != 3 || err != nil || int64(len(content)) != size+1 { // the content, then its NUL
		return nil, core.Errorf(core.Internal, "cat-file --batch: bad header %q", header)
	}
	return content[:size], nil
}

// batchStdin is cat-file -Z's input: object specs, each ended by a NUL, so a
// path may hold a newline.
func batchStdin(revs []string) []byte {
	return []byte(strings.Join(revs, "\x00") + "\x00")
}

package services

import (
	"bytes"
	"cmp"
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

// maxDiffFileSize caps DiffFile's auto-loaded content. force bypasses it;
// nothing bypasses Binary.
const maxDiffFileSize = 5 * 1024 * 1024

// parseCatFileBatch reads cat-file --batch's own framing: per object,
// "<oid> <type> <size>\n<size bytes of content>\n", or a missing line.
// want is the number of objects requested, a length sanity check.
func parseCatFileBatch(data []byte, want int) ([][]byte, error) {
	var blobs [][]byte
	for len(data) > 0 {
		nl := bytes.IndexByte(data, '\n')
		if nl < 0 {
			return nil, core.Errorf(core.Internal, "cat-file --batch: unterminated header")
		}
		header := string(data[:nl])
		data = data[nl+1:]
		if missing(header) {
			blobs = append(blobs, nil)
			continue
		}
		parts := strings.SplitN(header, " ", 3)
		size, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
		if len(parts) != 3 || err != nil || int64(len(data)) < size+1 {
			return nil, core.Errorf(core.Internal, "cat-file --batch: bad header %q", header)
		}
		blobs = append(blobs, data[:size])
		data = data[size+1:] // the content's own trailing \n
	}
	if len(blobs) != want {
		return nil, core.Errorf(core.Internal, "cat-file --batch: %d objects, want %d", len(blobs), want)
	}
	return blobs, nil
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

// diffIsBinary asks git's own diff machinery whether path is binary: a
// NUL-byte sniff of our own would ignore .gitattributes overrides. Both the
// old-side and new-side path are passed so -M's rename detection still
// applies when path-restricted.
func diffIsBinary(ctx context.Context, r git.Runner, dir, sha, oldPath, path string) (bool, error) {
	return numstatIsBinary(ctx, r, dir, oldPath, path,
		"show", "--format=", "--numstat", "-z", "-M", "--first-parent", "--end-of-options", sha)
}

// numstatIsBinary runs gitArgs (a show or diff command producing one
// --numstat -z record) scoped to the file's old and new path and reads the
// record's binary marker.
func numstatIsBinary(ctx context.Context, r git.Runner, dir, oldPath, path string, gitArgs ...string) (bool, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: append(gitArgs, "--", cmp.Or(oldPath, path), path),
	})
	if err != nil {
		return false, err
	}
	return parseBinaryNumstat(result.Stdout), nil
}

// parseBinaryNumstat reads one --numstat -z record ("<added>\t<removed>\t..."
// or "-\t-\t..." for binary) and reports whether it marks a binary file.
func parseBinaryNumstat(record []byte) bool {
	record = bytes.TrimSuffix(record, []byte{0})
	fields := bytes.SplitN(record, []byte{'\t'}, 3)
	return len(fields) >= 2 && string(fields[0]) == "-" && string(fields[1]) == "-"
}

// DiffFile implements core.API.
func (c *Core) DiffFile(ctx context.Context, id core.WorktreeID, sha, path, oldPath string, force bool) (core.DiffFile, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.DiffFile{}, err
	}
	spec := diffSidesSpec{
		oldRev: sha + "^:" + cmp.Or(oldPath, path), newRev: sha + ":" + path,
		path: path, oldPath: oldPath, force: force,
		isBinary: func() (bool, error) { return diffIsBinary(ctx, wt.Runner, wt.Path, sha, oldPath, path) },
	}
	return diffSides(ctx, wt.Runner, wt.Path, spec)
}

// diffSidesSpec drives diffSides: DiffFile (sha^/sha via git show
// --first-parent) and DiffRange (a/b via git diff) differ only in how
// oldRev/newRev and isBinary are built.
type diffSidesSpec struct {
	oldRev, newRev, path, oldPath string
	force                         bool
	isBinary                      func() (bool, error)
}

// diffSides runs the size cap, the binary check, the content fetch, and the
// UTF-8 check shared by DiffFile and DiffRange. The frontend computes the
// visual diff.
func diffSides(ctx context.Context, r git.Runner, dir string, s diffSidesSpec) (core.DiffFile, error) {
	sizes, err := diffBlobSizes(ctx, r, dir, s.oldRev, s.newRev)
	if err != nil {
		return core.DiffFile{}, err
	}
	if !s.force && (sizes.oldSize > maxDiffFileSize || sizes.newSize > maxDiffFileSize) {
		return core.DiffFile{OldPath: s.oldPath, Path: s.path, TooLarge: true}, nil
	}

	binary, err := s.isBinary()
	if err != nil {
		return core.DiffFile{}, err
	}
	if binary {
		return core.DiffFile{OldPath: s.oldPath, Path: s.path, Binary: true}, nil
	}

	oldContent, newContent, err := diffBlobContent(ctx, r, dir, s.oldRev, s.newRev)
	if err != nil {
		return core.DiffFile{}, err
	}
	if !utf8.Valid(oldContent) || !utf8.Valid(newContent) {
		// Not binary by git's own classification, but not valid UTF-8 either
		// (e.g. a Latin-1 file): shown as binary rather than corrupting
		// through JSON's UTF-8 requirement.
		return core.DiffFile{OldPath: s.oldPath, Path: s.path, Binary: true}, nil
	}
	return core.DiffFile{OldPath: s.oldPath, Path: s.path, OldContent: string(oldContent), NewContent: string(newContent)}, nil
}

type blobSizes struct{ oldSize, newSize int64 }

// batchStdin is cat-file --batch/--batch-check's input: one object spec per line.
func batchStdin(oldRev, newRev string) []byte {
	return []byte(oldRev + "\n" + newRev + "\n")
}

// missing reports cat-file's "<request> missing" line for an absent object
// (added/deleted file, or a root commit's nonexistent parent).
func missing(line string) bool {
	return strings.HasSuffix(line, " missing")
}

// diffBlobSizes runs one cat-file --batch-check for both sides, so the size
// cap is enforced before any content byte is fetched. Object specs go on
// stdin, so no --end-of-options is needed.
func diffBlobSizes(ctx context.Context, r git.Runner, dir, oldRev, newRev string) (blobSizes, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args:  []string{"cat-file", "--batch-check=%(objectsize)"},
		Stdin: batchStdin(oldRev, newRev),
	})
	if err != nil {
		return blobSizes{}, err
	}
	lines := strings.Split(strings.TrimRight(string(result.Stdout), "\n"), "\n")
	if len(lines) != 2 {
		return blobSizes{}, core.Errorf(core.Internal, "cat-file --batch-check: %d lines, want 2", len(lines))
	}
	oldSize, err := blobSize(lines[0])
	if err != nil {
		return blobSizes{}, err
	}
	newSize, err := blobSize(lines[1])
	if err != nil {
		return blobSizes{}, err
	}
	return blobSizes{oldSize: oldSize, newSize: newSize}, nil
}

// diffBlobContent fetches both sides' raw bytes in one cat-file --batch
// call. A missing side (add/delete, or a root commit's nonexistent parent)
// yields nil, not an error.
func diffBlobContent(ctx context.Context, r git.Runner, dir, oldRev, newRev string) (oldContent, newContent []byte, err error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args:  []string{"cat-file", "--batch"},
		Stdin: batchStdin(oldRev, newRev),
	})
	if err != nil {
		return nil, nil, err
	}
	blobs, err := parseCatFileBatch(result.Stdout, 2)
	if err != nil {
		return nil, nil, err
	}
	return blobs[0], blobs[1], nil
}

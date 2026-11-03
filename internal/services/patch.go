package services

import (
	"bytes"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/brogli/nebegit/internal/core"
)

// patchArgs starts every git diff or show whose patch the app parses. It
// overrides config that changes the patch's framing (colour, an external
// diff tool, textconv, path prefixes, relative paths, blank context lines
// without their space) and keeps config that changes the edit itself
// (diff.algorithm, diff.indentHeuristic, attributes, filters).
func patchArgs(cmd string, args ...string) []string {
	return append([]string{
		"-c", "diff.suppressBlankEmpty=false", cmd,
		"--no-color", "--no-ext-diff", "--no-textconv", "--src-prefix=a/", "--dst-prefix=b/", "--no-relative",
	}, args...)
}

// parsePatch parses a git patch and returns path's entry: the file whose new
// side is path, or whose deleted old side is. nil if the patch lists no
// change to path.
func parsePatch(patch []byte, path string) (*gitdiff.File, error) {
	files, _, err := gitdiff.Parse(bytes.NewReader(patch))
	if err != nil {
		return nil, core.Errorf(core.Internal, "parse diff for %s: %v", path, err)
	}
	for _, f := range files {
		if f.NewName == path || f.IsDelete && f.OldName == path {
			return f, nil
		}
	}
	return nil, nil
}

// hunksOf is file's fragments as their hunk headers.
func hunksOf(file *gitdiff.File) []core.DiffHunk {
	hunks := make([]core.DiffHunk, len(file.TextFragments))
	for i, f := range file.TextFragments {
		hunks[i] = core.DiffHunk{
			OldStart: int(f.OldPosition), OldLines: int(f.OldLines),
			NewStart: int(f.NewPosition), NewLines: int(f.NewLines),
		}
	}
	return hunks
}

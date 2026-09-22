package services

import (
	"bytes"
	"context"
	"fmt"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

// diffArgsFor is the fresh re-diff StageLines builds its patch from: the
// staged diff when reverse (unstaging peels off what's already staged),
// the unstaged diff otherwise. -U3: git apply needs at least one context
// line by default.
func diffArgsFor(path string, reverse bool) []string {
	args := []string{"diff"}
	if reverse {
		args = append(args, "--cached")
	}
	return append(args, "-U3", "--", path)
}

// hunkFragments re-diffs path and parses it.
func hunkFragments(ctx context.Context, r git.Runner, dir, path string, reverse bool) (frags []*gitdiff.TextFragment, header []byte, err error) {
	result, err := r.Run(ctx, git.Call{Dir: dir, ReadOnly: true, Args: diffArgsFor(path, reverse)})
	if err != nil {
		return nil, nil, err
	}
	files, _, err := gitdiff.Parse(bytes.NewReader(result.Stdout))
	if err != nil {
		return nil, nil, core.Errorf(core.Internal, "parse diff for %s: %v", path, err)
	}
	if len(files) != 1 { // git diff lists no file for an untracked or unchanged path
		return nil, nil, core.Errorf(core.Internal, "%s has no changes to stage or unstage", path)
	}
	return files[0].TextFragments, fileHeaderLines(result.Stdout), nil
}

// fileHeaderLines returns everything before the first hunk header — only
// the "@@ ... @@" line and body differ per hunk.
func fileHeaderLines(diff []byte) []byte {
	if i := bytes.Index(diff, []byte("\n@@ ")); i >= 0 {
		return diff[:i+1]
	}
	return diff
}

// selectLines marks which of frag's +/- lines sel selects, keyed by each
// line's index among frag's +/- lines in diff order. A "-" line is selected
// by its old-side line number in sel.Old, a "+" line by its new-side line
// number in sel.New.
func selectLines(frag *gitdiff.TextFragment, sel core.LineSelection) map[int]bool {
	oldSel, newSel := lineSet(sel.Old), lineSet(sel.New)
	selected := make(map[int]bool)
	changed := 0
	oldLine, newLine := int(frag.OldPosition), int(frag.NewPosition)
	for _, l := range frag.Lines {
		switch l.Op {
		case gitdiff.OpContext:
			oldLine++
			newLine++
		case gitdiff.OpDelete:
			if oldSel[oldLine] {
				selected[changed] = true
			}
			oldLine++
			changed++
		case gitdiff.OpAdd:
			if newSel[newLine] {
				selected[changed] = true
			}
			newLine++
			changed++
		}
	}
	return selected
}

func lineSet(nums []int) map[int]bool {
	set := make(map[int]bool, len(nums))
	for _, n := range nums {
		set[n] = true
	}
	return set
}

// buildPartialPatch rebuilds one hunk's body keeping only selected +/-
// lines. An unselected line of the removal side folds to context; one of
// the other side is dropped with its NoEOL marker. The removal side is "-"
// when staging and "+" when unstaging (applied with --reverse), since git
// apply checks context against the patch's unmodified side.
//
// git groups a change as all its "-" lines followed by all its "+" lines.
// A "-" folded to context between those groups would misplace the
// new-side reading, so writeChangeBlock re-pairs them by position.
//
// The header keeps the source fragment's line counts: git apply --recount
// infers the real ones from the body.
func buildPartialPatch(frag *gitdiff.TextFragment, selected map[int]bool, reverse bool) []byte {
	var body bytes.Buffer
	changed := 0
	lines := frag.Lines
	for i := 0; i < len(lines); {
		if lines[i].Op == gitdiff.OpContext {
			writeHunkLine(&body, ' ', lines[i])
			i++
			continue
		}
		// a change block: its "-" lines (none for a pure insert), then its
		// "+" lines (none for a pure delete)
		delEnd := i
		for delEnd < len(lines) && lines[delEnd].Op == gitdiff.OpDelete {
			delEnd++
		}
		addEnd := delEnd
		for addEnd < len(lines) && lines[addEnd].Op == gitdiff.OpAdd {
			addEnd++
		}
		changed = writeChangeBlock(&body, lines[i:delEnd], lines[delEnd:addEnd], selected, changed, reverse)
		i = addEnd
	}
	header := fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", frag.OldPosition, frag.OldLines, frag.NewPosition, frag.NewLines)
	return append([]byte(header), body.Bytes()...)
}

// writeChangeBlock writes one change block's dels and adds paired by
// position (dels[0] with adds[0], and so on), so a selected pair ends up
// adjacent. changed is selectLines' index of the block's first line in
// diff order; it returns the index after this block.
func writeChangeBlock(body *bytes.Buffer, dels, adds []gitdiff.Line, selected map[int]bool, changed int, reverse bool) int {
	n, m := len(dels), len(adds)
	for p := 0; p < n || p < m; p++ {
		if p < n {
			writeChangedLine(body, dels[p], '-', selected[changed+p], !reverse)
		}
		if p < m {
			writeChangedLine(body, adds[p], '+', selected[changed+n+p], reverse)
		}
	}
	return changed + n + m
}

// writeChangedLine writes one +/- line: with its prefix if selected, as
// context if on the removal side, not at all otherwise.
func writeChangedLine(b *bytes.Buffer, l gitdiff.Line, prefix byte, selected, removalSide bool) {
	switch {
	case selected:
		writeHunkLine(b, prefix, l)
	case removalSide:
		writeHunkLine(b, ' ', l)
	}
}

// writeHunkLine writes one line of a hunk body. l.Line carries its trailing
// "\n" unless NoEOL is set; then the content line still needs one before
// the "\ No newline at end of file" marker.
func writeHunkLine(b *bytes.Buffer, prefix byte, l gitdiff.Line) {
	b.WriteByte(prefix)
	b.WriteString(l.Line)
	if l.NoEOL() {
		b.WriteString("\n\\ No newline at end of file\n")
	}
}

// StageLines implements core.API. It re-diffs path under the write lock,
// keeps only the lines sel selects from each fragment, and applies the
// rebuilt patch to the index. A selection matching no changed line of the
// fresh diff is stale: nothing is applied, Code Internal.
func (c *Core) StageLines(ctx context.Context, id core.WorktreeID, path string, sel core.LineSelection, reverse bool) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Lock() // writes this worktree's index
	defer wt.Unlock()

	frags, header, err := hunkFragments(ctx, wt.Runner, wt.Path, path, reverse)
	if err != nil {
		return err
	}

	patch := append([]byte{}, header...)
	matched := false
	for _, frag := range frags {
		selected := selectLines(frag, sel)
		if len(selected) == 0 {
			continue
		}
		matched = true
		patch = append(patch, buildPartialPatch(frag, selected, reverse)...)
	}
	if !matched {
		return core.Errorf(core.Internal, "line selection matches no changed line of %s's current diff", path)
	}

	args := []string{"apply", "--cached", "--recount"}
	if reverse {
		args = append(args, "--reverse")
	}
	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: append(args, "-"), Stdin: patch})
	return err
}

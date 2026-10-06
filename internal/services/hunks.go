package services

import (
	"bytes"
	"context"
	"fmt"
	"slices"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

// diffArgsFor is path's staged or unstaged diff at full context, as the
// display reads it: its one hunk carries every line of both sides with its
// number, so a selection can be checked against the file as it is now.
func diffArgsFor(path string, staged bool) []string {
	args := []string{fullContext, "--", literalPath(path)}
	if staged {
		args = append([]string{"--cached"}, args...)
	}
	return patchArgs("diff", args...)
}

// hunkFragments re-diffs path and parses it.
func hunkFragments(ctx context.Context, r git.Runner, dir, path string, staged bool) (frags []*gitdiff.TextFragment, header []byte, err error) {
	result, err := r.Run(ctx, git.Call{Dir: dir, ReadOnly: true, Args: diffArgsFor(path, staged)})
	if err != nil {
		return nil, nil, err
	}
	file, err := parsePatch(result.Stdout, path)
	if err != nil {
		return nil, nil, err
	}
	if file == nil { // git diff lists no file for an untracked or unchanged path
		return nil, nil, core.Errorf(core.Internal, "%s has no changes to stage or unstage", path)
	}
	return file.TextFragments, fileHeaderLines(result.Stdout), nil
}

// fileHeaderLines returns everything before the first hunk header — only
// the "@@ ... @@" line and body differ per hunk — except a mode change: a
// hunk action changes lines, never the mode.
func fileHeaderLines(diff []byte) []byte {
	if i := bytes.Index(diff, []byte("\n@@ ")); i >= 0 {
		diff = diff[:i+1]
	}
	var header []byte
	for line := range bytes.Lines(diff) {
		if !bytes.HasPrefix(line, []byte("old mode ")) && !bytes.HasPrefix(line, []byte("new mode ")) {
			header = append(header, line...)
		}
	}
	return header
}

// selectLines marks which of frag's +/- lines sel selects, keyed by each
// line's index among frag's +/- lines in diff order. A "-" line is selected
// by its old-side line number in sel.Old, a "+" line by its new-side line
// number in sel.New; a number naming a context line selects nothing.
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
// lines, after lazygit's patch package (MIT; its notice is in
// tools/licensecheck/adapted/lazygit.LICENSE). An unselected line of the
// removal side folds to context (unless it is a last line without newline
// that selected lines follow: see hunkBody.keep); one of the other side is
// dropped with its NoEOL marker. The removal side is "-" when staging and
// "+" when unstaging (applied with --reverse), since git apply checks
// context against the patch's unmodified side.
//
// git groups a change as all its "-" lines followed by all its "+" lines.
// A "-" folded to context between those groups would misplace the
// new-side reading, so writeBlock places each selected line of the other
// side next to its partner, or at its own position in the block.
//
// The header counts the body's lines. git apply looks for a hunk near the
// start of the side it produces (the new side; the old side under
// --reverse), which in a one-hunk patch is where the other side starts.
func buildPartialPatch(frag *gitdiff.TextFragment, selected map[int]bool, reverse bool) []byte {
	body := hunkBody{produced: '+'}
	if reverse {
		body.produced = '-'
	}
	changed := 0
	lines := frag.Lines
	for i := 0; i < len(lines); {
		if lines[i].Op == gitdiff.OpContext {
			body.line(' ', lines[i])
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
	body.close()
	start := frag.OldPosition
	if reverse {
		start = frag.NewPosition
	}
	header := fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", start, body.oldLines, start, body.newLines)
	return append([]byte(header), body.Bytes()...)
}

// hunkBody is a hunk's body being written, with its line count per side.
// produced is the prefix of the side git apply produces: '+', or '-' under
// --reverse.
type hunkBody struct {
	bytes.Buffer
	oldLines, newLines int
	produced           byte

	// kept and added are last lines without newline, held back until it
	// is known whether a line lands after them on the produced side: kept
	// an unselected line of the other side (see keep), added a selected
	// line of the produced side (see line).
	kept, added *gitdiff.Line
}

// keep writes l, an unselected line of the side git apply takes lines
// from, as context. A last line without newline is held back: if a line
// lands after it on the produced side, it must end in a newline there, so
// it is written as removed and added back with one; otherwise close
// writes it as context.
func (b *hunkBody) keep(l gitdiff.Line) {
	if !l.NoEOL() {
		b.line(' ', l)
		return
	}
	b.land() // as context or added back, l lands on the produced side
	b.kept = &l
}

// line writes one line of the body. A produced-side line without newline
// is held back: if a line lands after it, it is written with a newline
// and without the "\ No newline at end of file" marker.
func (b *hunkBody) line(prefix byte, l gitdiff.Line) {
	if prefix != otherSide(b.produced) {
		b.land()
	}
	if prefix == b.produced && l.NoEOL() {
		b.added = &l
		return
	}
	b.write(prefix, l)
}

// land writes the held-back last lines before a line that lands after
// them on the produced side: each gains a newline there.
func (b *hunkBody) land() {
	if kept := b.kept; kept != nil {
		b.kept = nil
		b.write(otherSide(b.produced), *kept)
		b.write(b.produced, withNewline(*kept))
	}
	if added := b.added; added != nil {
		b.added = nil
		b.write(b.produced, withNewline(*added))
	}
}

// close writes the held-back last lines, which no line followed.
func (b *hunkBody) close() {
	if b.kept != nil {
		b.write(' ', *b.kept)
	}
	if b.added != nil {
		b.write(b.produced, *b.added)
	}
	b.kept, b.added = nil, nil
}

func withNewline(l gitdiff.Line) gitdiff.Line {
	return gitdiff.Line{Op: l.Op, Line: l.Line + "\n"}
}

// otherSide is the prefix of the side opposite prefix's: '+' for '-'.
func otherSide(prefix byte) byte {
	if prefix == '-' {
		return '+'
	}
	return '-'
}

// write writes one line. l.Line carries its trailing "\n" unless NoEOL is
// set; then the content line still needs one before the "\ No newline at
// end of file" marker.
func (b *hunkBody) write(prefix byte, l gitdiff.Line) {
	b.WriteByte(prefix)
	b.WriteString(l.Line)
	if l.NoEOL() {
		b.WriteString("\n\\ No newline at end of file\n")
	}
	if prefix != '+' {
		b.oldLines++
	}
	if prefix != '-' {
		b.newLines++
	}
}

// writeChangeBlock writes one change block (its dels, then its adds) with
// exactly its selected lines as changes. changed is selectLines' index of
// the block's first line in diff order; it returns the index after this
// block.
func writeChangeBlock(body *hunkBody, dels, adds []gitdiff.Line, selected map[int]bool, changed int, reverse bool) int {
	delSide := blockSide{dels, '-', chosen(selected, changed, len(dels))}
	addSide := blockSide{adds, '+', chosen(selected, changed+len(dels), len(adds))}
	if reverse {
		writeBlock(body, addSide, delSide)
	} else {
		writeBlock(body, delSide, addSide)
	}
	return changed + len(dels) + len(adds)
}

// blockSide is one side of a change block: its lines, their prefix, and
// the indices of the selected ones.
type blockSide struct {
	lines    []gitdiff.Line
	prefix   byte
	selected []int
}

// chosen lists which of count lines, starting at selectLines index first,
// are selected.
func chosen(selected map[int]bool, first, count int) []int {
	var indices []int
	for i := range count {
		if selected[first+i] {
			indices = append(indices, i)
		}
	}
	return indices
}

// writeBlock walks from, the side git apply takes lines from: an
// unselected line is kept as context, a selected one removed. The selected
// lines of onto are added: the i-th right after the i-th selected line of
// from, so a replacement stays together; one left over goes before the
// line of from at its own index in the block.
func writeBlock(body *hunkBody, from, onto blockSide) {
	pairs := min(len(from.selected), len(onto.selected))
	partner := make(map[int]int, pairs)
	for i := range pairs {
		partner[from.selected[i]] = onto.selected[i]
	}
	leftover := onto.selected[pairs:]
	for p, l := range from.lines {
		for ; len(leftover) > 0 && leftover[0] <= p; leftover = leftover[1:] {
			body.line(onto.prefix, onto.lines[leftover[0]])
		}
		o, paired := partner[p]
		switch {
		case !slices.Contains(from.selected, p):
			body.keep(l)
		case paired: // a replacement reads "-" then "+"
			del, add := l, onto.lines[o]
			if from.prefix == '+' {
				del, add = add, del
			}
			body.line('-', del)
			body.line('+', add)
		default:
			body.line(from.prefix, l)
		}
	}
	for _, o := range leftover {
		body.line(onto.prefix, onto.lines[o])
	}
}

// selectionPatch re-diffs path (the staged diff if staged, else the unstaged
// one) and rebuilds it keeping only the selected lines the re-diff changes,
// for git apply [--reverse if reverse]. A selected line it shows unchanged
// is skipped, so a selection taken from another diff of the same sides,
// such as git diff -w's, changes nothing outside itself. Code Internal if
// the re-diff's sides differ from the shown ones (path changed since it was
// shown), or if no selected line is a change.
func selectionPatch(ctx context.Context, r git.Runner, dir, path string, sel core.LineSelection, staged, reverse bool) ([]byte, error) {
	frags, header, err := hunkFragments(ctx, r, dir, path, staged)
	if err != nil {
		return nil, err
	}
	if len(frags) != 1 || fingerprint(sides(frags[0])) != sel.Fingerprint {
		return nil, core.Errorf(core.Internal, "%s changed since it was shown", path)
	}
	selected := selectLines(frags[0], sel)
	if len(selected) == 0 {
		return nil, core.Errorf(core.Internal, "line selection matches no changed line of %s's current diff", path)
	}
	return append(header, buildPartialPatch(frags[0], selected, reverse)...), nil
}

// StageLines implements core.API: it applies selectionPatch to the index
// under the write lock.
func (c *Core) StageLines(ctx context.Context, id core.WorktreeID, path string, sel core.LineSelection, reverse bool) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	wt.Lock() // writes this worktree's index
	defer wt.Unlock()

	patch, err := selectionPatch(ctx, wt.Runner, wt.Path, path, sel, reverse, reverse)
	if err != nil {
		return err
	}
	args := []string{"apply", "--cached", "--recount"}
	if reverse {
		args = append(args, "--reverse")
	}
	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: append(args, "-"), Stdin: patch})
	return err
}

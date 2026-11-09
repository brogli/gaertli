package services

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
)

// markerBlock is one complete conflict block of a text: the line indexes of
// its opening and closing marker lines and the lines of its current and
// incoming sections, line breaks included, and of its base section when it has one (diff3 and
// zdiff3 styles).
type markerBlock struct {
	open, close             int
	current, incoming, base []string
	hasBase                 bool
}

// parseMarkerBlocks finds the conflict blocks of text for markers size
// characters long, in git's merge, diff3 and zdiff3 styles. A block that
// does not close before the next opening marker or the end of the text is
// not a block.
func parseMarkerBlocks(text string, size int) []markerBlock {
	lines := splitLines(text)
	runs := newMarkerRuns(size)
	var blocks []markerBlock
	for i := 0; i < len(lines); i++ {
		if !isMarker(lines[i], runs.open) {
			continue
		}
		if b, ok := blockAt(lines, i, runs); ok {
			blocks = append(blocks, b)
			i = b.close
		}
	}
	return blocks
}

// blockAt reads the block opened at lines[open].
func blockAt(lines []string, open int, runs markerRuns) (markerBlock, bool) {
	b := markerBlock{open: open}
	section := &b.current
	seenSeparator := false
	for i := open + 1; i < len(lines); i++ {
		switch line := lines[i]; {
		case isMarker(line, runs.open):
			return markerBlock{}, false
		case isMarker(line, runs.base) && !seenSeparator:
			section, b.hasBase = &b.base, true
		case isMarker(line, runs.separator) && !seenSeparator:
			section, seenSeparator = &b.incoming, true
		case isMarker(line, runs.close) && seenSeparator:
			b.close = i
			return b, true
		case section != nil:
			*section = append(*section, line)
		}
	}
	return markerBlock{}, false
}

// markerRuns are the character runs that start the four kinds of marker line.
type markerRuns struct{ open, base, separator, close string }

func newMarkerRuns(size int) markerRuns {
	run := func(char string) string { return strings.Repeat(char, size) }
	return markerRuns{open: run("<"), base: run("|"), separator: run("="), close: run(">")}
}

// isMarker reports whether line is run, alone or followed by a space, with a CR before the line
// break tolerated.
func isMarker(line, run string) bool {
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	return line == run || strings.HasPrefix(line, run+" ")
}

// splitLines splits text after each LF; a last line without one stays whole.
func splitLines(text string) []string {
	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// textDiffs holds git's -U0 hunks between the texts conflictChanges places by:
// the base against each side (none without a base), and each side against the
// result with every block filled from that side (none when the result has no
// block). currentInKept and incomingInKept are where each block's section sits in
// that filled result.
type textDiffs struct {
	baseToCurrent, baseToIncoming []core.DiffHunk
	currentToKept, incomingToKept []core.DiffHunk
	currentInKept, incomingInKept []core.LineRange
}

// readTextDiffs runs the diffs conflictChanges needs; text is file.Result with
// its marker blocks.
func readTextDiffs(ctx context.Context, r git.Runner, dir string, file core.ConflictFile, text markedText) (textDiffs, error) {
	var d textDiffs
	var err error
	if file.Base.Present {
		if d.baseToCurrent, err = diffHunks(ctx, r, dir, file.Base.Content, file.Current.Content); err != nil {
			return textDiffs{}, err
		}
		if d.baseToIncoming, err = diffHunks(ctx, r, dir, file.Base.Content, file.Incoming.Content); err != nil {
			return textDiffs{}, err
		}
	}
	if len(text.blocks) == 0 {
		return d, nil
	}
	keptCurrent, currentInKept := text.keeping(currentSection)
	d.currentInKept = currentInKept
	if d.currentToKept, err = diffHunks(ctx, r, dir, file.Current.Content, keptCurrent); err != nil {
		return textDiffs{}, err
	}
	keptIncoming, incomingInKept := text.keeping(incomingSection)
	d.incomingInKept = incomingInKept
	if d.incomingToKept, err = diffHunks(ctx, r, dir, file.Incoming.Content, keptIncoming); err != nil {
		return textDiffs{}, err
	}
	return d, nil
}

func currentSection(b markerBlock) []string  { return b.current }
func incomingSection(b markerBlock) []string { return b.incoming }

// conflictChanges places each block of text, file.Result with its marker
// blocks, in every side: a block's range in a side is where its section sits
// in the result with each block filled from that side, mapped back through the
// side's diff against it. The base lines are the block's base section where it
// has one, else they come from the current lines through d.baseToCurrent.
// Without a base it lists the conflicts alone, else also the changes only one
// side made.
func conflictChanges(file core.ConflictFile, text markedText, d textDiffs) []core.MergeChange {
	if len(text.blocks) == 0 {
		return nil
	}
	conflicts := make([]core.MergeChange, len(text.blocks))
	for i, b := range text.blocks {
		conflicts[i] = core.MergeChange{
			Kind:     core.ConflictChange,
			Result:   core.LineRange{Start: b.open + 1, Lines: b.close - b.open + 1},
			Current:  mapToOld(d.currentToKept, d.currentInKept[i]),
			Incoming: mapToOld(d.incomingToKept, d.incomingInKept[i]),
		}
	}
	if !file.Base.Present {
		return conflicts
	}
	baseLines := splitLines(file.Base.Content)
	from := 0 // the base line after the previous conflict
	for i := range conflicts {
		base := mapToOld(d.baseToCurrent, conflicts[i].Current)
		if text.blocks[i].hasBase {
			base = locateSection(baseLines, text.blocks[i].base, base, from)
		}
		conflicts[i].Base = withoutMovedOut(base, conflicts[i], d.baseToCurrent, d.baseToIncoming)
		from = edgeEnd(conflicts[i].Base)
	}
	conflicts = append(conflicts, changes(file, text, d)...)
	sortChanges(conflicts)
	return conflicts
}

// locateSection finds where a base section sits in the base's lines: the occurrence at or after
// line from nearest the estimate. A section that cannot be found keeps the estimate.
func locateSection(base, section []string, estimate core.LineRange, from int) core.LineRange {
	want := edgeStart(estimate)
	if len(section) == 0 {
		at := max(want, from)
		return rangeOfEdges(at, at)
	}
	last := len(base) - len(section)
	for d := 0; want-d >= from || want+d <= last; d++ {
		for _, at := range []int{want - d, want + d} {
			if at >= from && at <= last && slices.Equal(base[at:at+len(section)], section) {
				return rangeOfEdges(at, at+len(section))
			}
		}
	}
	return estimate
}

// withoutMovedOut is a conflict's base less the lines the merge and zdiff3 styles moved out of the
// markers because both sides changed them alike. The diff3 style moves none out, so its sides'
// sections reach as far as the base section.
func withoutMovedOut(base core.LineRange, change core.MergeChange, toCurrent, toIncoming []core.DiffHunk) core.LineRange {
	currentBefore, currentAfter := movedOut(toCurrent, change.Current)
	incomingBefore, incomingAfter := movedOut(toIncoming, change.Incoming)
	start := edgeStart(base) + min(currentBefore, incomingBefore)
	end := edgeEnd(base) - min(currentAfter, incomingAfter)
	return rangeOfEdges(start, max(start, end))
}

// movedOut returns how many base lines before and after the section rng the hunk it lies in
// reaches beyond it. Only a hunk that changes line for line ties a side line to a base line; in
// any other, none of the base lines count as moved out.
func movedOut(hunks []core.DiffHunk, rng core.LineRange) (before, after int) {
	if rng.Lines == 0 {
		return 0, 0
	}
	start, end := edges(rng)
	for _, h := range hunks {
		if h.OldLines != h.NewLines {
			continue
		}
		first, last := edges(newLines(h))
		if first < start && start < last {
			before = start - first
		}
		if first < end && end < last {
			after = last - end
		}
	}
	return before, after
}

func edgeStart(rng core.LineRange) int { start, _ := edges(rng); return start }
func edgeEnd(rng core.LineRange) int   { _, end := edges(rng); return end }

// sideChanges returns where side differs from base, in side's lines, from hunks, the diff of base
// against side: a hunk without base lines is an added change, any other a changed one. Without a
// base the whole side is one added change.
func sideChanges(base core.ConflictSide, side string, hunks []core.DiffHunk) []core.SideChange {
	if !base.Present {
		if side == "" {
			return nil
		}
		return []core.SideChange{{Lines: core.LineRange{Start: 1, Lines: len(splitLines(side))}, Added: true}}
	}
	var changes []core.SideChange
	for _, h := range hunks {
		if h.NewLines > 0 {
			changes = append(changes, core.SideChange{
				Lines: core.LineRange{Start: h.NewStart, Lines: h.NewLines},
				Added: h.OldLines == 0,
			})
		}
	}
	return changes
}

// markedText is a text's lines and the conflict blocks among them.
type markedText struct {
	lines  []string
	blocks []markerBlock
}

// keeping returns the text with each block replaced by its section, and where each section sits
// in it.
func (m markedText) keeping(section func(markerBlock) []string) (string, []core.LineRange) {
	var kept []string
	ranges := make([]core.LineRange, len(m.blocks))
	next := 0
	for i, b := range m.blocks {
		kept = append(kept, m.lines[next:b.open]...)
		ranges[i] = rangeOfEdges(len(kept), len(kept)+len(section(b)))
		kept = append(kept, section(b)...)
		next = b.close + 1
	}
	kept = append(kept, m.lines[next:]...)
	return strings.Join(kept, ""), ranges
}

// diffHunks is git's -U0 diff of oldText against newText as hunk headers.
func diffHunks(ctx context.Context, r git.Runner, dir, oldText, newText string) ([]core.DiffHunk, error) {
	if oldText == newText {
		return nil, nil
	}
	tmp, err := os.MkdirTemp("", "gaertli-conflict-")
	if err != nil {
		return nil, core.Errorf(core.Internal, "temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	oldPath, newPath := filepath.Join(tmp, "old"), filepath.Join(tmp, "new")
	if err := os.WriteFile(oldPath, []byte(oldText), 0o600); err != nil {
		return nil, core.Errorf(core.Internal, "write %s: %v", oldPath, err)
	}
	if err := os.WriteFile(newPath, []byte(newText), 0o600); err != nil {
		return nil, core.Errorf(core.Internal, "write %s: %v", newPath, err)
	}
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: patchArgs("diff", "-U0", "--inter-hunk-context=0", "--text", "--no-index", "--", oldPath, newPath),
	})
	if err != nil && !gitExited(err, result, 1) { // 1: the texts differ
		return nil, err
	}
	files, _, err := gitdiff.Parse(bytes.NewReader(result.Stdout))
	if err != nil {
		return nil, core.Errorf(core.Internal, "parse diff of conflict sides: %v", err)
	}
	if len(files) == 0 {
		return nil, nil
	}
	return hunksOf(files[0]), nil
}

// hunkSide is one text of a hunk: the old or the new lines.
type hunkSide func(core.DiffHunk) core.LineRange

func oldLines(h core.DiffHunk) core.LineRange {
	return core.LineRange{Start: h.OldStart, Lines: h.OldLines}
}

func newLines(h core.DiffHunk) core.LineRange {
	return core.LineRange{Start: h.NewStart, Lines: h.NewLines}
}

// mapToOld maps a range of the new side of hunks onto the old side.
func mapToOld(hunks []core.DiffHunk, rng core.LineRange) core.LineRange {
	return mapRange(hunks, rng, newLines, oldLines)
}

// mapRange maps a range of the from text of hunks onto the to text. Lines outside the hunks shift
// by the hunks before them; an edge inside a hunk moves out to that hunk's lines.
func mapRange(hunks []core.DiffHunk, rng core.LineRange, from, to hunkSide) core.LineRange {
	start, end := edges(rng)
	return rangeOfEdges(mapEdge(hunks, start, false, from, to), mapEdge(hunks, end, true, from, to))
}

// mapEdge maps one edge; isEnd picks the hunk's end rather than its start for an edge inside a
// hunk. A hunk with no from lines at the edge itself lies beyond a start edge, and within an end
// edge.
func mapEdge(hunks []core.DiffHunk, edge int, isEnd bool, from, to hunkSide) int {
	shift := 0
	for _, h := range hunks {
		src, dst := from(h), to(h)
		first, last := edges(src)
		switch {
		case src.Lines == 0 && first == edge && !isEnd:
			return edge + shift
		case last <= edge:
			shift += dst.Lines - src.Lines
		case first < edge && isEnd:
			return first + shift + dst.Lines
		case first < edge:
			return first + shift
		default:
			return edge + shift
		}
	}
	return edge + shift
}

// edges returns the number of lines before rng and before its end.
func edges(rng core.LineRange) (start, end int) {
	if rng.Lines == 0 {
		return rng.Start, rng.Start
	}
	return rng.Start - 1, rng.Start - 1 + rng.Lines
}

// rangeOfEdges is edges' inverse.
func rangeOfEdges(start, end int) core.LineRange {
	if start == end {
		return core.LineRange{Start: start}
	}
	return core.LineRange{Start: start + 1, Lines: end - start}
}

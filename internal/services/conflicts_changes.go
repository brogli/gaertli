package services

import (
	"cmp"
	"slices"

	"github.com/brogli/gaertli/internal/core"
)

// changes finds the changes only one side made, which git applied to Result. text is Result with
// its marker blocks. A change both sides made alike is applied and listed nowhere.
//
// A side with every block filled from it differs from that side only by the other side's changes,
// so the diff of the two places those in Result and in the side. The change's base lines are those
// of the side's own diff against the base, and the changed side's lines are the Result lines. Each
// range is then settled on the lines it must hold: the unchanged side's and the base's hold the
// same text, and the changed side's the Result's.
func changes(file core.ConflictFile, text markedText, d textDiffs) []core.MergeChange {
	var out []core.MergeChange
	_, currentInKept := text.keeping(currentSection)
	keptCurrent := keptSide{text: text, inKept: currentInKept, hunks: d.currentToKept}
	_, incomingInKept := text.keeping(incomingSection)
	keptIncoming := keptSide{text: text, inKept: incomingInKept, hunks: d.incomingToKept}
	for _, h := range keptCurrent.hunks {
		own := oldLines(h)
		inResult := keptCurrent.inResult(newLines(h))
		base := settle(file.Base.Content, linesAt(file.Current.Content, own), mapToOld(d.baseToCurrent, own))
		out = append(out, core.MergeChange{
			Kind: core.IncomingOnlyChange, Added: own.Lines == 0,
			Result: inResult, Base: base, Current: own,
			Incoming: settle(file.Incoming.Content, linesAt(file.Result, inResult), mapRange(d.baseToIncoming, base, oldLines, newLines)),
		})
	}
	for _, h := range keptIncoming.hunks {
		own := oldLines(h)
		inResult := keptIncoming.inResult(newLines(h))
		base := settle(file.Base.Content, linesAt(file.Incoming.Content, own), mapToOld(d.baseToIncoming, own))
		out = append(out, core.MergeChange{
			Kind: core.CurrentOnlyChange, Added: own.Lines == 0,
			Result: inResult, Base: base, Incoming: own,
			Current: settle(file.Current.Content, linesAt(file.Result, inResult), mapRange(d.baseToCurrent, base, oldLines, newLines)),
		})
	}
	return out
}

// settle returns the range of content that holds want: estimate if it does, else the nearest
// place that does. An estimate of lines that want lacks becomes a point when want is empty.
func settle(content string, want []string, estimate core.LineRange) core.LineRange {
	lines := splitLines(content)
	start, end := edges(estimate)
	if end-start == len(want) && slices.Equal(lines[min(start, len(lines)):min(end, len(lines))], want) && end <= len(lines) {
		return estimate
	}
	if len(want) == 0 {
		return rangeOfEdges(min(start, len(lines)), min(start, len(lines)))
	}
	for d := 0; start-d >= 0 || start+d+len(want) <= len(lines); d++ {
		for _, at := range []int{start - d, start + d} {
			if at >= 0 && at+len(want) <= len(lines) && slices.Equal(lines[at:at+len(want)], want) {
				return rangeOfEdges(at, at+len(want))
			}
		}
	}
	return estimate
}

// linesAt returns the lines rng selects in content.
func linesAt(content string, rng core.LineRange) []string {
	start, end := edges(rng)
	return splitLines(content)[start:end]
}

// keptSide is a side of the conflict and Result with each marker block replaced by the side's
// section of it.
type keptSide struct {
	text   markedText
	inKept []core.LineRange // where each block's section sits in the replaced text
	hunks  []core.DiffHunk  // the side's diff against the replaced text: the other side's changes
}

// inResult maps a range of the replaced text to Result. A point at an empty section lies after
// that block.
func (k keptSide) inResult(rng core.LineRange) core.LineRange {
	start, end := edges(rng)
	return rangeOfEdges(k.resultEdge(start), k.resultEdge(end))
}

func (k keptSide) resultEdge(edge int) int {
	shift := 0
	for i, b := range k.text.blocks {
		keptStart, keptEnd := edges(k.inKept[i])
		if keptEnd <= edge {
			shift += (b.close + 1 - b.open) - (keptEnd - keptStart)
		}
	}
	return edge + shift
}

// sortChanges orders changes by their place in Result, then in the base.
func sortChanges(changes []core.MergeChange) {
	slices.SortStableFunc(changes, func(a, b core.MergeChange) int {
		return cmp.Or(cmp.Compare(edgeStart(a.Result), edgeStart(b.Result)), cmp.Compare(edgeStart(a.Base), edgeStart(b.Base)))
	})
}

// Requirements: FR-CONF-05

package services

import (
	"reflect"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git/gittest"
)

func TestConflictSideChangesMarkWhereASideDiffersFromTheBase(t *testing.T) {
	t.Parallel()
	base := "l1\nl2\nl3\nl4\nl5\n"
	current := "l1\nc2\nl3\nl4\nc5\n"
	incoming := "l1\nl2\nl3\nl4\ni5\nend\n"
	c, wt, _ := stoppedOn(t, "merge", "a.txt", version(base), version(current), version(incoming))

	got := readConflict(t, c, wt, "a.txt")

	wantCurrent := []core.SideChange{{Lines: core.LineRange{Start: 2, Lines: 1}}, {Lines: core.LineRange{Start: 5, Lines: 1}}}
	if !reflect.DeepEqual(got.CurrentChanges, wantCurrent) {
		t.Errorf("CurrentChanges = %+v, want %+v", got.CurrentChanges, wantCurrent)
	}
	wantIncoming := []core.SideChange{{Lines: core.LineRange{Start: 5, Lines: 2}}}
	if !reflect.DeepEqual(got.IncomingChanges, wantIncoming) {
		t.Errorf("IncomingChanges = %+v, want %+v", got.IncomingChanges, wantIncoming)
	}
	if len(conflictRegionsOf(got)) != 1 || conflictRegionsOf(got)[0].Current.Start != 5 || conflictRegionsOf(got)[0].Incoming.Start != 5 {
		t.Errorf("Changes = %+v, want one at line 5 in both sides", conflictRegionsOf(got))
	}
}

func TestConflictSideChangesMarkAddedLines(t *testing.T) {
	t.Parallel()
	c, wt, _ := stoppedOn(t, "merge", "a.txt", version("l1\nl2\nl3\n"), version("l1\nl2\nc3\n"), version("l1\nnew\nl2\ni3\n"))

	got := readConflict(t, c, wt, "a.txt")

	want := []core.SideChange{{Lines: core.LineRange{Start: 2, Lines: 1}, Added: true}, {Lines: core.LineRange{Start: 4, Lines: 1}}}
	if !reflect.DeepEqual(got.IncomingChanges, want) {
		t.Errorf("IncomingChanges = %+v, want %+v", got.IncomingChanges, want)
	}
}

func TestConflictSideChangesWithoutABaseAreTheWholeSide(t *testing.T) {
	t.Parallel()
	dir := gittest.New(t).Dir
	commitVersion(t, dir, "other.txt", version("x\n"), "start")
	gittest.Run(t, dir, "checkout", "-q", "-b", "side")
	commitVersion(t, dir, "a.txt", version("i1\ni2\n"), "incoming add")
	gittest.Run(t, dir, "checkout", "-q", "master")
	commitVersion(t, dir, "a.txt", version("c1\nc2\nc3\n"), "current add")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)

	got := readConflict(t, c, wt, "a.txt")

	if got.Base.Present {
		t.Fatal("base present, want an add/add conflict")
	}
	if want := []core.SideChange{{Lines: core.LineRange{Start: 1, Lines: 3}, Added: true}}; !reflect.DeepEqual(got.CurrentChanges, want) {
		t.Errorf("CurrentChanges = %+v, want %+v", got.CurrentChanges, want)
	}
	if want := []core.SideChange{{Lines: core.LineRange{Start: 1, Lines: 2}, Added: true}}; !reflect.DeepEqual(got.IncomingChanges, want) {
		t.Errorf("IncomingChanges = %+v, want %+v", got.IncomingChanges, want)
	}
}

func TestConflictRegionsListEveryChangeWithItsKindAndRanges(t *testing.T) {
	t.Parallel()
	base := "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n"
	current := "l1\nc2\nl3\nl4\nc5\nl6\nl7\nl8\nl9\nl10\n"
	incoming := "l1\nl2\nl3\nl4\ni5\nl6\nl7\ni8\nl9\nl10\nend\n"
	c, wt, _ := stoppedOn(t, "merge", "a.txt", version(base), version(current), version(incoming))

	got := readConflict(t, c, wt, "a.txt")

	rng := func(start, lines int) core.LineRange { return core.LineRange{Start: start, Lines: lines} }
	want := []core.MergeChange{
		{Kind: core.CurrentOnlyChange, Result: rng(2, 1), Base: rng(2, 1), Current: rng(2, 1), Incoming: rng(2, 1)},
		{Kind: core.ConflictChange, Result: rng(5, 5), Base: rng(5, 1), Current: rng(5, 1), Incoming: rng(5, 1)},
		{Kind: core.IncomingOnlyChange, Result: rng(12, 1), Base: rng(8, 1), Current: rng(8, 1), Incoming: rng(8, 1)},
		{Kind: core.IncomingOnlyChange, Added: true, Result: rng(15, 1), Base: rng(10, 0), Current: rng(10, 0), Incoming: rng(11, 1)},
	}
	if !reflect.DeepEqual(got.Changes, want) {
		t.Errorf("Changes = %+v\nwant %+v\nresult:\n%s", got.Changes, want, got.Result)
	}
}

func TestConflictRegionsLeaveOutAChangeBothSidesMadeAlike(t *testing.T) {
	t.Parallel()
	base := "l1\nl2\nl3\nl4\nl5\n"
	c, wt, _ := stoppedOn(t, "merge", "a.txt", version(base), version("l1\nsame\nl3\nl4\nc5\n"), version("l1\nsame\nl3\nl4\ni5\n"))

	got := readConflict(t, c, wt, "a.txt")

	if len(got.Changes) != 1 || got.Changes[0].Kind != core.ConflictChange {
		t.Errorf("Changes = %+v, want only the conflict", got.Changes)
	}
}

func TestConflictChangesPlaceEveryChangeInEachText(t *testing.T) {
	t.Parallel()
	// Each triple's hunks are what git diff -U0 --inter-hunk-context=0 --no-index printed for its
	// texts; git's zdiff3 style wrote the same results as diff3.
	type triple struct {
		base, current, incoming string
		d                       textDiffs
	}
	deleted := triple{"a\nb\nc\n", "a\nc\n", "a\nB\nc\n", textDiffs{
		baseToCurrent:  []core.DiffHunk{hunk(2, 1, 1, 0)},
		baseToIncoming: []core.DiffHunk{hunk(2, 1, 2, 1)},
	}}
	two := triple{twoConflictsBase, twoConflictsCurrent, twoConflictsIncoming, textDiffs{
		baseToCurrent:  []core.DiffHunk{hunk(2, 1, 2, 1), hunk(12, 1, 12, 2)},
		baseToIncoming: []core.DiffHunk{hunk(2, 1, 2, 1), hunk(7, 1, 7, 3), hunk(12, 1, 14, 1)},
		currentToKept:  []core.DiffHunk{hunk(7, 1, 7, 3)},
	}}
	oneSided := triple{"1\n2\n3\n4\n5\n6\n7\n", "C1\n2\n3\nC4\n5\n6\n7\n", "1\n2\n3\nI4\n5\n6\nI7\n", textDiffs{
		baseToCurrent:  []core.DiffHunk{hunk(1, 1, 1, 1), hunk(4, 1, 4, 1)},
		baseToIncoming: []core.DiffHunk{hunk(4, 1, 4, 1), hunk(7, 1, 7, 1)},
		currentToKept:  []core.DiffHunk{hunk(7, 1, 7, 1)},
		incomingToKept: []core.DiffHunk{hunk(1, 1, 1, 1)},
	}}
	twoMid := "f1\nf2\nf3\nf4\nM1\nM2\nM3\nf5\nf6\nf7\nf8\n"
	change := func(kind core.MergeChangeKind, result, base, current, incoming core.LineRange) core.MergeChange {
		return core.MergeChange{Kind: kind, Result: result, Base: base, Current: current, Incoming: incoming}
	}
	tests := map[string]struct {
		texts  triple
		noBase bool
		result string
		want   []core.MergeChange
	}{
		"a deleted side, merge": {deleted, false, "a\n<<<<<<< HEAD\n=======\nB\n>>>>>>> side\nc\n", []core.MergeChange{
			change(core.ConflictChange, span(2, 4), span(2, 1), span(1, 0), span(2, 1)),
		}},
		"a deleted side, diff3": {deleted, false, "a\n<<<<<<< HEAD\n||||||| base\nb\n=======\nB\n>>>>>>> side\nc\n", []core.MergeChange{
			change(core.ConflictChange, span(2, 6), span(2, 1), span(1, 0), span(2, 1)),
		}},
		"no base": {deleted, true, "a\n<<<<<<< HEAD\n=======\nB\n>>>>>>> side\nc\n", []core.MergeChange{
			change(core.ConflictChange, span(2, 4), core.LineRange{}, span(1, 0), span(2, 1)),
		}},
		"no block": {deleted, false, "a\nB\nc\n", nil},
		"two conflicts, merge": {two, false, "top\n<<<<<<< HEAD\nc1\n=======\ni1\n>>>>>>> side\n" + twoMid + "<<<<<<< HEAD\nc2a\nc2b\n=======\ni2\n>>>>>>> side\nbottom\n", []core.MergeChange{
			change(core.ConflictChange, span(2, 5), span(2, 1), span(2, 1), span(2, 1)),
			change(core.IncomingOnlyChange, span(11, 3), span(7, 1), span(7, 1), span(7, 3)),
			change(core.ConflictChange, span(18, 6), span(12, 1), span(12, 2), span(14, 1)),
		}},
		"two conflicts, diff3": {two, false, "top\n<<<<<<< HEAD\nc1\n||||||| base\nx1\n=======\ni1\n>>>>>>> side\n" + twoMid + "<<<<<<< HEAD\nc2a\nc2b\n||||||| base\nx2\n=======\ni2\n>>>>>>> side\nbottom\n", []core.MergeChange{
			change(core.ConflictChange, span(2, 7), span(2, 1), span(2, 1), span(2, 1)),
			change(core.IncomingOnlyChange, span(13, 3), span(7, 1), span(7, 1), span(7, 3)),
			change(core.ConflictChange, span(20, 8), span(12, 1), span(12, 2), span(14, 1)),
		}},
		"one-sided changes around a conflict, merge": {oneSided, false, "C1\n2\n3\n<<<<<<< HEAD\nC4\n=======\nI4\n>>>>>>> side\n5\n6\nI7\n", []core.MergeChange{
			change(core.CurrentOnlyChange, span(1, 1), span(1, 1), span(1, 1), span(1, 1)),
			change(core.ConflictChange, span(4, 5), span(4, 1), span(4, 1), span(4, 1)),
			change(core.IncomingOnlyChange, span(11, 1), span(7, 1), span(7, 1), span(7, 1)),
		}},
		"one-sided changes around a conflict, diff3": {oneSided, false, "C1\n2\n3\n<<<<<<< HEAD\nC4\n||||||| base\n4\n=======\nI4\n>>>>>>> side\n5\n6\nI7\n", []core.MergeChange{
			change(core.CurrentOnlyChange, span(1, 1), span(1, 1), span(1, 1), span(1, 1)),
			change(core.ConflictChange, span(4, 7), span(4, 1), span(4, 1), span(4, 1)),
			change(core.IncomingOnlyChange, span(13, 1), span(7, 1), span(7, 1), span(7, 1)),
		}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			file := core.ConflictFile{
				Result:   tc.result,
				Base:     core.ConflictSide{Present: !tc.noBase, Content: tc.texts.base},
				Current:  core.ConflictSide{Present: true, Content: tc.texts.current},
				Incoming: core.ConflictSide{Present: true, Content: tc.texts.incoming},
			}
			d := tc.texts.d
			if tc.noBase {
				d.baseToCurrent, d.baseToIncoming = nil, nil
			}
			text := markedText{lines: splitLines(tc.result), blocks: parseMarkerBlocks(tc.result, 7)}

			if got := conflictChanges(file, text, d); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("conflictChanges =\n%+v\nwant\n%+v", got, tc.want)
			}
		})
	}
}

func TestSettleKeepsAnEstimateThatHoldsTheLinesElseFindsTheNearest(t *testing.T) {
	t.Parallel()
	content := "a\nb\nc\nb\n"
	tests := map[string]struct {
		want     []string
		estimate core.LineRange
		expect   core.LineRange
	}{
		"an estimate that holds":      {[]string{"b\n"}, span(4, 1), span(4, 1)},
		"an estimate that does not":   {[]string{"b\n"}, span(3, 1), span(2, 1)},
		"no lines wanted, a point":    {nil, span(2, 1), span(1, 0)},
		"lines nowhere, the estimate": {[]string{"z\n"}, span(2, 1), span(2, 1)},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := settle(content, tc.want, tc.estimate); got != tc.expect {
				t.Errorf("settle = %+v, want %+v", got, tc.expect)
			}
		})
	}
}

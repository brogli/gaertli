// Requirements: FR-CONF-05

package services

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
	"github.com/brogli/gaertli/internal/git/gittest"
)

func TestParseMarkerBlocksInEachStyle(t *testing.T) {
	t.Parallel()
	merge := "pre\n<<<<<<< HEAD\ncur\n=======\ninc\n>>>>>>> side\npost\n"
	diff3 := "pre\n<<<<<<< HEAD\ncur\n||||||| base\nbas\n=======\ninc\n>>>>>>> side\npost\n"
	for style, text := range map[string]string{"merge": merge, "diff3": diff3, "zdiff3": diff3} {
		t.Run(style, func(t *testing.T) {
			t.Parallel()
			blocks := parseMarkerBlocks(text, 7)
			if len(blocks) != 1 {
				t.Fatalf("blocks = %d, want 1", len(blocks))
			}
			b := blocks[0]
			wantClose := len(splitLines(text)) - 2
			if b.open != 1 || b.close != wantClose {
				t.Errorf("open, close = %d, %d, want 1, %d", b.open, b.close, wantClose)
			}
			if !reflect.DeepEqual(b.current, []string{"cur\n"}) || !reflect.DeepEqual(b.incoming, []string{"inc\n"}) {
				t.Errorf("current, incoming = %q, %q", b.current, b.incoming)
			}
		})
	}
}

func TestParseMarkerBlocksHonoursTheMarkerSize(t *testing.T) {
	t.Parallel()
	text := "<<<<<<<<<< HEAD\ncur\n==========\ninc\n>>>>>>>>>> side\n<<<<<<< x\n=======\n>>>>>>> y\n"

	blocks := parseMarkerBlocks(text, 10)

	if len(blocks) != 1 || blocks[0].open != 0 || blocks[0].close != 4 {
		t.Errorf("blocks = %+v, want one block over lines 0..4", blocks)
	}
}

func TestParseMarkerBlocksIgnoresIncompleteBlocks(t *testing.T) {
	t.Parallel()
	text := "<<<<<<< HEAD\ncur\n=======\ninc\n<<<<<<< HEAD\na\n=======\nb\n>>>>>>> side\n<<<<<<< HEAD\nno end\n"

	blocks := parseMarkerBlocks(text, 7)

	if len(blocks) != 1 || blocks[0].open != 4 || blocks[0].close != 8 {
		t.Errorf("blocks = %+v, want only the complete block, lines 4..8", blocks)
	}
}

func TestParseMarkerBlocksIgnoresALoneSeparator(t *testing.T) {
	t.Parallel()

	blocks := parseMarkerBlocks("title\n=======\ntext\n<<<<<<<< not a marker\n", 7)

	if len(blocks) != 0 {
		t.Errorf("blocks = %+v, want none", blocks)
	}
}

// selected returns the lines r selects in content.
func selected(content string, r core.LineRange) string {
	lines := splitLines(content)
	start, end := edges(r)
	if start < 0 || end > len(lines) {
		return "<out of range>"
	}
	return strings.Join(lines[start:end], "")
}

// wantRegion is the base, current and incoming lines a conflict region
// selects.
type wantRegion struct{ base, current, incoming string }

// assertRegions fails unless got's conflict regions select want's lines, in
// order, each over one marker block of the result.
func assertRegions(t *testing.T, got core.ConflictFile, want ...wantRegion) {
	t.Helper()
	regions := conflictRegionsOf(got)
	if len(regions) != len(want) {
		t.Fatalf("regions = %d, want %d; result:\n%s", len(regions), len(want), got.Result)
	}
	for i, r := range regions {
		if s := selected(got.Base.Content, r.Base); s != want[i].base {
			t.Errorf("region %d base = %q (%+v), want %q", i, s, r.Base, want[i].base)
		}
		if s := selected(got.Current.Content, r.Current); s != want[i].current {
			t.Errorf("region %d current = %q (%+v), want %q", i, s, r.Current, want[i].current)
		}
		if s := selected(got.Incoming.Content, r.Incoming); s != want[i].incoming {
			t.Errorf("region %d incoming = %q (%+v), want %q", i, s, r.Incoming, want[i].incoming)
		}
		block := selected(got.Result, r.Result)
		if !strings.HasPrefix(block, "<<<<<<<") || strings.Count(block, ">>>>>>>") != 1 || !strings.HasSuffix(block, "\n") {
			t.Errorf("region %d result = %q, want one marker block", i, block)
		}
	}
}

const (
	twoConflictsBase     = "top\nx1\nf1\nf2\nf3\nf4\nmid\nf5\nf6\nf7\nf8\nx2\nbottom\n"
	twoConflictsCurrent  = "top\nc1\nf1\nf2\nf3\nf4\nmid\nf5\nf6\nf7\nf8\nc2a\nc2b\nbottom\n"
	twoConflictsIncoming = "top\ni1\nf1\nf2\nf3\nf4\nM1\nM2\nM3\nf5\nf6\nf7\nf8\ni2\nbottom\n"
)

func TestConflictRegionsWithZdiff3AndAnIncomingOnlyChangeBetween(t *testing.T) {
	t.Parallel()
	dir := divergedRepo(t, "a.txt", version(twoConflictsBase), version(twoConflictsCurrent), version(twoConflictsIncoming))
	gittest.Run(t, dir, "config", "merge.conflictStyle", "zdiff3")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)

	got := readConflict(t, c, wt, "a.txt")

	assertRegions(t, got, wantRegion{"x1\n", "c1\n", "i1\n"}, wantRegion{"x2\n", "c2a\nc2b\n", "i2\n"})
}

func TestConflictRegionsSkipABlockTheUserResolved(t *testing.T) {
	t.Parallel()
	c, wt, dir := stoppedOn(t, "merge", "a.txt", version(twoConflictsBase), version(twoConflictsCurrent), version(twoConflictsIncoming))
	path := filepath.Join(dir, "a.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	afterFirst := strings.Index(text, ">>>>>>>")
	afterFirst += strings.Index(text[afterFirst:], "\n") + 1
	if err := os.WriteFile(path, []byte("top\nmine\n"+text[afterFirst:]), 0o644); err != nil {
		t.Fatal(err)
	}

	got := readConflict(t, c, wt, "a.txt")

	assertRegions(t, got, wantRegion{"x2\n", "c2a\nc2b\n", "i2\n"})
}

func TestConflictRegionsUseTheMarkerSizeAttribute(t *testing.T) {
	t.Parallel()
	dir := gittest.New(t).Dir
	commitVersion(t, dir, ".gitattributes", version("* conflict-marker-size=10\n"), "attributes")
	commitVersion(t, dir, "a.txt", version("one\nbase\ntwo\n"), "base")
	gittest.Run(t, dir, "checkout", "-q", "-b", "side")
	commitVersion(t, dir, "a.txt", version("one\nincoming\ntwo\n"), "incoming change")
	gittest.Run(t, dir, "checkout", "-q", "master")
	commitVersion(t, dir, "a.txt", version("one\ncurrent\ntwo\n"), "current change")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)

	got := readConflict(t, c, wt, "a.txt")

	if got.MarkerSize != 10 {
		t.Errorf("MarkerSize = %d, want 10", got.MarkerSize)
	}
	assertRegions(t, got, wantRegion{"base\n", "current\n", "incoming\n"})
}

func TestConflictRegionsIgnoreTheDiffAttribute(t *testing.T) {
	t.Parallel()
	dir := gittest.New(t).Dir
	commitVersion(t, dir, ".gitattributes", version("* -diff\n"), "attributes")
	commitVersion(t, dir, "a.txt", version("one\nbase\ntwo\n"), "base")
	gittest.Run(t, dir, "checkout", "-q", "-b", "side")
	commitVersion(t, dir, "a.txt", version("one\nincoming\ntwo\n"), "incoming change")
	gittest.Run(t, dir, "checkout", "-q", "master")
	commitVersion(t, dir, "a.txt", version("one\ncurrent\ntwo\n"), "current change")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)

	got := readConflict(t, c, wt, "a.txt")

	assertRegions(t, got, wantRegion{"base\n", "current\n", "incoming\n"})
}

// conflictRegionsOf returns the regions of f that are conflicts.
func conflictRegionsOf(f core.ConflictFile) []core.MergeChange {
	return slices.DeleteFunc(slices.Clone(f.Changes), func(r core.MergeChange) bool { return r.Kind != core.ConflictChange })
}

func TestReadingATextConflictRunsFourDiffs(t *testing.T) {
	t.Parallel()
	// One conflict at line 4 and a change only one side made on either side of it.
	base, current, incoming := "1\n2\n3\n4\n5\n6\n7\n", "C1\n2\n3\nC4\n5\n6\n7\n", "1\n2\n3\nI4\n5\n6\nI7\n"
	dir := stoppedDir(t, "merge", "a.txt", version(base), version(current), version(incoming))
	var diffs int
	r := git.Runner{Log: func(e git.Entry) {
		if strings.Contains(e.Command, "--no-index") {
			diffs++
		}
	}}
	stages, err := unmergedStages(t.Context(), r, dir, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	present := core.ConflictSide{Present: true}
	out := core.ConflictFile{Path: "a.txt", Base: present, Current: present, Incoming: present}

	got, err := readConflictText(t.Context(), r, dir, out, stages)

	if err != nil || len(conflictRegionsOf(got)) != 1 {
		t.Fatalf("readConflictText = %d conflicts, %v; want 1", len(conflictRegionsOf(got)), err)
	}
	if diffs != 4 {
		t.Errorf("git diff --no-index ran %d times, want 4", diffs)
	}
}

// span is the range of n lines from line start, or the point after line start when n is 0.
func span(start, n int) core.LineRange { return core.LineRange{Start: start, Lines: n} }

func TestKeepingFillsEachBlockFromOneSide(t *testing.T) {
	t.Parallel()
	result := "top\n<<<<<<< HEAD\nc1\n=======\ni1\n>>>>>>> side\nf1\nf2\nf3\nf4\nM1\nM2\nM3\nf5\nf6\nf7\nf8\n" +
		"<<<<<<< HEAD\nc2a\nc2b\n=======\ni2\n>>>>>>> side\nbottom\n"
	text := markedText{lines: splitLines(result), blocks: parseMarkerBlocks(result, 7)}

	tests := map[string]struct {
		section    func(markerBlock) []string
		wantText   string
		wantRanges []core.LineRange
	}{
		"current":  {currentSection, "top\nc1\nf1\nf2\nf3\nf4\nM1\nM2\nM3\nf5\nf6\nf7\nf8\nc2a\nc2b\nbottom\n", []core.LineRange{span(2, 1), span(14, 2)}},
		"incoming": {incomingSection, twoConflictsIncoming, []core.LineRange{span(2, 1), span(14, 1)}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			kept, ranges := text.keeping(tc.section)
			if kept != tc.wantText || !slices.Equal(ranges, tc.wantRanges) {
				t.Errorf("keeping = %q, %+v, want %q, %+v", kept, ranges, tc.wantText, tc.wantRanges)
			}
		})
	}
}

func TestMapRangeMapsThroughHunksOfEveryShape(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		hunks []core.DiffHunk
		rng   core.LineRange
		want  core.LineRange
	}{
		"after a pure insert":           {[]core.DiffHunk{hunk(2, 0, 3, 2)}, span(5, 1), span(3, 1)},
		"the inserted lines":            {[]core.DiffHunk{hunk(2, 0, 3, 2)}, span(3, 2), span(2, 0)},
		"a range from a pure delete":    {[]core.DiffHunk{hunk(2, 1, 1, 0)}, span(2, 1), span(2, 2)},
		"a point at a pure delete":      {[]core.DiffHunk{hunk(2, 1, 1, 0)}, span(1, 0), span(2, 1)},
		"an edge inside a replacement":  {[]core.DiffHunk{hunk(2, 1, 2, 2)}, span(3, 1), span(2, 1)},
		"after a replacement":           {[]core.DiffHunk{hunk(2, 1, 2, 2)}, span(4, 1), span(3, 1)},
		"before every hunk":             {[]core.DiffHunk{hunk(5, 1, 5, 2)}, span(1, 2), span(1, 2)},
		"past two hunks, shifted twice": {[]core.DiffHunk{hunk(1, 0, 2, 1), hunk(3, 1, 4, 3)}, span(8, 1), span(5, 1)},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := mapToOld(tc.hunks, tc.rng); got != tc.want {
				t.Errorf("mapToOld(%+v, %+v) = %+v, want %+v", tc.hunks, tc.rng, got, tc.want)
			}
		})
	}
	if got := mapRange([]core.DiffHunk{hunk(2, 1, 2, 2)}, span(2, 1), oldLines, newLines); got != span(2, 2) {
		t.Errorf("mapRange old to new = %+v, want the replacing lines 2..3", got)
	}
}

func TestEdgesAndRangeOfEdgesAreInverses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		rng        core.LineRange
		start, end int
	}{{span(3, 0), 3, 3}, {span(3, 2), 2, 4}, {span(1, 1), 0, 1}, {span(0, 0), 0, 0}} {
		if start, end := edges(tc.rng); start != tc.start || end != tc.end {
			t.Errorf("edges(%+v) = %d, %d, want %d, %d", tc.rng, start, end, tc.start, tc.end)
		}
		if got := rangeOfEdges(tc.start, tc.end); got != tc.rng {
			t.Errorf("rangeOfEdges(%d, %d) = %+v, want %+v", tc.start, tc.end, got, tc.rng)
		}
	}
}

func TestLocateSectionFindsTheOccurrenceNearestTheEstimate(t *testing.T) {
	t.Parallel()
	base := splitLines("x\na\nx\na\n")
	tests := map[string]struct {
		section  []string
		estimate core.LineRange
		from     int
		want     core.LineRange
	}{
		"at the estimate":                 {[]string{"x\n", "a\n"}, span(3, 2), 0, span(3, 2)},
		"the earlier repeat":              {[]string{"x\n", "a\n"}, span(1, 2), 0, span(1, 2)},
		"not before from":                 {[]string{"x\n", "a\n"}, span(1, 2), 1, span(3, 2)},
		"an empty section at from":        {nil, span(2, 1), 3, span(3, 0)},
		"a missing section, the estimate": {[]string{"z\n"}, span(2, 1), 0, span(2, 1)},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := locateSection(base, tc.section, tc.estimate, tc.from); got != tc.want {
				t.Errorf("locateSection = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestWithoutMovedOutDropsBaseLinesBothSidesChangedAlike(t *testing.T) {
	t.Parallel()
	// zdiff3 of a b c x, A B C x and A Y C x: the block holds B and Y; both sides changed a and c alike.
	toSide := []core.DiffHunk{hunk(1, 3, 1, 3)}
	tests := map[string]struct {
		hunks                 []core.DiffHunk
		rng                   core.LineRange
		wantBefore, wantAfter int
	}{
		"inside a line-for-line hunk":   {toSide, span(2, 1), 1, 1},
		"inside a hunk of other length": {[]core.DiffHunk{hunk(1, 3, 1, 4)}, span(2, 1), 0, 0},
		"a point":                       {toSide, span(2, 0), 0, 0},
		"the whole hunk":                {toSide, span(1, 3), 0, 0},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if before, after := movedOut(tc.hunks, tc.rng); before != tc.wantBefore || after != tc.wantAfter {
				t.Errorf("movedOut = %d, %d, want %d, %d", before, after, tc.wantBefore, tc.wantAfter)
			}
		})
	}
	change := core.MergeChange{Current: span(2, 1), Incoming: span(2, 1)}
	if got := withoutMovedOut(span(1, 3), change, toSide, toSide); got != span(2, 1) {
		t.Errorf("withoutMovedOut = %+v, want base line 2 alone", got)
	}
}

func TestSideChangesListAddedAndChangedLines(t *testing.T) {
	t.Parallel()
	present := core.ConflictSide{Present: true}
	tests := map[string]struct {
		base  core.ConflictSide
		side  string
		hunks []core.DiffHunk
		want  []core.SideChange
	}{
		"no base, the whole side added": {core.ConflictSide{}, "a\nb\n", nil, []core.SideChange{{Lines: span(1, 2), Added: true}}},
		"no base, an empty side":        {core.ConflictSide{}, "", nil, nil},
		"an insert, a change, a delete": {present, "", []core.DiffHunk{hunk(2, 0, 3, 1), hunk(4, 1, 5, 1), hunk(6, 1, 6, 0)}, []core.SideChange{{Lines: span(3, 1), Added: true}, {Lines: span(5, 1)}}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := sideChanges(tc.base, tc.side, tc.hunks); !slices.Equal(got, tc.want) {
				t.Errorf("sideChanges = %+v, want %+v", got, tc.want)
			}
		})
	}
}

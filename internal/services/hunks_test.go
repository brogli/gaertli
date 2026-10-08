package services

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
)

// numberedLines builds an n-line fixture "line1\n".."lineN\n", with
// overrides replacing specific 1-based line numbers.
func numberedLines(n int, overrides map[int]string) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		if line, ok := overrides[i]; ok {
			b.WriteString(line)
		} else {
			fmt.Fprintf(&b, "line%d\n", i)
		}
	}
	return b.String()
}

// shown is the selection the UI sends for these old and new line numbers
// of path's diff as DiffWorking shows it now.
func shown(t *testing.T, c *Core, id core.WorktreeID, path string, staged bool, oldLines, newLines []int) core.LineSelection {
	t.Helper()
	diff, err := c.DiffWorking(t.Context(), id, path, "", staged, core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return core.LineSelection{Old: oldLines, New: newLines, Fingerprint: diff.Fingerprint}
}

func TestFR_CHG_03_StageLinesRefusesLinesThatChangedSinceTheyWereShown(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, base, displayed, now string
		old, new                   []int
	}{
		{"another line added where the shown one was", "1\n2\n3\n4\n5\n6\n", "1\n2\n3\n4\nX\n5\n6\n", "1\n2\n3\n4\nY\nX\n5\n6\n", nil, []int{5}},
		{"a line inserted above the shown hunk", "a\nb\nc\nd\ne\nf\ng\n", "a\nB\nc\nd\ne\nE\nf\ng\n", "z\na\nB\nc\nd\ne\nE\nf\ng\n", []int{2}, []int{2}},
		{"an identical line added above the shown one", "1\n2\n3\n4\n5\n6\n", "1\n2\n3\n4\nX\n5\n6\n", "1\n2\n3\n4\nX\nY\nX\n5\n6\n", nil, []int{5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := gittest.New(t)
			gittest.Commit(t, repo.Dir, "f.txt", tt.base, "base")
			writeFile(t, repo.Dir, "f.txt", tt.displayed)
			c, id := openWorktree(t, repo.Dir)
			sel := shown(t, c, id, "f.txt", false, tt.old, tt.new)
			writeFile(t, repo.Dir, "f.txt", tt.now)

			err := c.StageLines(t.Context(), id, "f.txt", sel, false)

			ce := coretest.AsError(t, err, core.Internal)
			if !strings.Contains(ce.Message, "changed since it was shown") {
				t.Errorf("Message = %q, want it to say the file changed since it was shown", ce.Message)
			}
			if got := indexBlob(t, repo.Dir, "f.txt"); got != strings.TrimSuffix(tt.base, "\n") {
				t.Errorf("index f.txt = %q, want it unchanged", got)
			}
		})
	}
}

// twinBlocks is a file of top, extra, a1..a8, a 7-line block around first,
// f1, the same block around second, and end: the blocks differ only in
// their middle line.
func twinBlocks(extra, first, second string) string {
	block := func(mid string) string { return "x1\nx2\nx3\n" + mid + "\ny1\ny2\ny3\n" }
	return "top\n" + extra + "a1\na2\na3\na4\na5\na6\na7\na8\n" + block(first) + "f1\n" + block(second) + "end\n"
}

const tenLines = "n1\nn2\nn3\nn4\nn5\nn6\nn7\nn8\nn9\nn10\n"

func TestFR_CHG_03_StageLinesOfTheSecondHunkAloneChangesItsOwnBlockNotAnIdenticalOne(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", twinBlocks("", "TARGET", "TARGET"), "base")
	writeFile(t, repo.Dir, "f.txt", twinBlocks(tenLines, "CHANGED", "TARGET"))
	c, id := openWorktree(t, repo.Dir)

	sel := shown(t, c, id, "f.txt", false, []int{13}, []int{23})
	if err := c.StageLines(t.Context(), id, "f.txt", sel, false); err != nil {
		t.Fatal(err)
	}

	want := strings.TrimSuffix(twinBlocks("", "CHANGED", "TARGET"), "\n")
	if got := indexBlob(t, repo.Dir, "f.txt"); got != want {
		t.Errorf("index f.txt = %q\nwant %q", got, want)
	}
}

// partial is buildPartialPatch as text.
func partial(frag *gitdiff.TextFragment, selected map[int]bool, reverse bool) string {
	return string(buildPartialPatch(frag, selected, reverse))
}

func TestFR_CHG_03_BuildPartialPatchStartsTheProducedSideWhereTheOtherSideStarts(t *testing.T) {
	t.Parallel()
	frag := &gitdiff.TextFragment{
		OldPosition: 10, OldLines: 3,
		NewPosition: 20, NewLines: 4,
		Lines: []gitdiff.Line{
			{Op: gitdiff.OpContext, Line: "ctx\n"},
			{Op: gitdiff.OpDelete, Line: "old\n"},
			{Op: gitdiff.OpAdd, Line: "new\n"},
			{Op: gitdiff.OpAdd, Line: "more\n"},
			{Op: gitdiff.OpContext, Line: "ctx\n"},
		},
	}
	selected := selectLines(frag, core.LineSelection{Old: []int{11}, New: []int{21, 22}})

	staging := partial(frag, selected, false)
	unstaging := partial(frag, selected, true)

	if header := strings.SplitAfter(staging, "\n")[0]; header != "@@ -10,3 +10,4 @@\n" {
		t.Errorf("staging header = %q, want \"@@ -10,3 +10,4 @@\"", header)
	}
	if header := strings.SplitAfter(unstaging, "\n")[0]; header != "@@ -20,3 +20,4 @@\n" {
		t.Errorf("unstaging header = %q, want \"@@ -20,3 +20,4 @@\"", header)
	}
}

func TestFR_CHG_03_StageLinesOfLinesAddedAfterALastLineWithoutNewlineGivesThatLineANewline(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "first\nlast", "base")
	writeFile(t, repo.Dir, "f.txt", "first\nlast\nadded1\nadded2\n")
	c, id := openWorktree(t, repo.Dir)

	if err := c.StageLines(t.Context(), id, "f.txt", shown(t, c, id, "f.txt", false, nil, []int{3, 4}), false); err != nil {
		t.Fatal(err)
	}

	if got := indexBlob(t, repo.Dir, "f.txt"); got != "first\nlast\nadded1\nadded2" {
		t.Errorf("index f.txt = %q, want the added lines after last, each on its own line", got)
	}
}

// indexBytes is name's staged content in dir, byte for byte.
func indexBytes(t *testing.T, dir, name string) string {
	t.Helper()
	result, err := (git.Runner{}).Run(t.Context(), git.Call{Dir: dir, Args: []string{"cat-file", "-p", ":" + name}})
	if err != nil {
		t.Fatal(err)
	}
	return string(result.Stdout)
}

// A replacement's selected new line comes with the old line the UI pairs
// it with (see DiffView's selectedLines).
func TestFR_CHG_03_StageLinesOfAReplacementStagesItAsOneChange(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "a\nb", "base")
	writeFile(t, repo.Dir, "f.txt", "a\nb\n")
	c, id := openWorktree(t, repo.Dir)

	if err := c.StageLines(t.Context(), id, "f.txt", shown(t, c, id, "f.txt", false, []int{2}, []int{2}), false); err != nil {
		t.Fatal(err)
	}

	if got := indexBytes(t, repo.Dir, "f.txt"); got != "a\nb\n" {
		t.Errorf("index f.txt = %q, want a last line given its newline", got)
	}
}

// Selecting X of a replacement removing lines sends the whole hunk (see
// DiffView's selectedLines).
func TestFR_CHG_03_StageLinesOfAReplacementRemovingLinesStagesTheWholeHunk(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "a\nx\ny\n", "base")
	writeFile(t, repo.Dir, "f.txt", "a\nX")
	c, id := openWorktree(t, repo.Dir)

	if err := c.StageLines(t.Context(), id, "f.txt", shown(t, c, id, "f.txt", false, []int{2, 3}, []int{2}), false); err != nil {
		t.Fatal(err)
	}

	if got := indexBytes(t, repo.Dir, "f.txt"); got != "a\nX" {
		t.Errorf("index f.txt = %q, want %q", got, "a\nX")
	}
}

func TestFR_CHG_03_StageLinesOfALineWithoutNewlineThatAKeptLineFollowsGivesItANewline(t *testing.T) {
	t.Parallel()
	for _, base := range []string{"a\nx\ny\n", "a\nx\ny"} {
		t.Run(base, func(t *testing.T) {
			repo := gittest.New(t)
			gittest.Commit(t, repo.Dir, "f.txt", base, "base")
			writeFile(t, repo.Dir, "f.txt", "a\nX")
			c, id := openWorktree(t, repo.Dir)

			if err := c.StageLines(t.Context(), id, "f.txt", shown(t, c, id, "f.txt", false, []int{2}, []int{2}), false); err != nil {
				t.Fatal(err)
			}

			if got, want := indexBytes(t, repo.Dir, "f.txt"), strings.Replace(base, "x\n", "X\n", 1); got != want {
				t.Errorf("index f.txt = %q, want %q: X on its own line before the kept y", got, want)
			}
		})
	}
}

func TestFR_CHG_03_StageLinesStagesAHunkOnABlankLineExactly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, base, now string
		old, new        []int
	}{
		{"a blank line replaced", "a\n\nb\n", "a\nX\nb\n", []int{2}, []int{2}},
		{"a line replaced by a blank one", "a\nX\nb\n", "a\n\nb\n", []int{2}, []int{2}},
		{"a lone blank line deleted", "a\n\nb\n", "a\nb\n", []int{2}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := gittest.New(t)
			gittest.Commit(t, repo.Dir, "f.txt", tt.base, "base")
			writeFile(t, repo.Dir, "f.txt", tt.now)
			c, id := openWorktree(t, repo.Dir)

			if err := c.StageLines(t.Context(), id, "f.txt", shown(t, c, id, "f.txt", false, tt.old, tt.new), false); err != nil {
				t.Fatal(err)
			}

			if got := indexBlob(t, repo.Dir, "f.txt"); got != strings.TrimSuffix(tt.now, "\n") {
				t.Errorf("index f.txt = %q, want %q", got, tt.now)
			}
		})
	}
}

func TestFR_CHG_03_StageLinesStagesPartOfWhatAnEmptyFileGained(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "", "base")
	writeFile(t, repo.Dir, "f.txt", "a\nb\n")
	c, id := openWorktree(t, repo.Dir)

	if err := c.StageLines(t.Context(), id, "f.txt", shown(t, c, id, "f.txt", false, nil, []int{1}), false); err != nil {
		t.Fatal(err)
	}

	if got := indexBlob(t, repo.Dir, "f.txt"); got != "a" {
		t.Errorf("index f.txt = %q, want only line 1 staged", got)
	}
}

func TestFR_CHG_03_BuildPartialPatchKeepsSelectedFoldsUnselectedRemovalToContextDropsUnselectedAddition(t *testing.T) {
	t.Parallel()
	frag := &gitdiff.TextFragment{
		OldPosition: 1, OldLines: 4,
		NewPosition: 1, NewLines: 4,
		Lines: []gitdiff.Line{
			{Op: gitdiff.OpContext, Line: "ctx1\n"},          // old 1, new 1
			{Op: gitdiff.OpDelete, Line: "selected old\n"},   // old 2
			{Op: gitdiff.OpAdd, Line: "selected new\n"},      // new 2
			{Op: gitdiff.OpDelete, Line: "unselected old\n"}, // old 3
			{Op: gitdiff.OpAdd, Line: "unselected new\n"},    // new 3
			{Op: gitdiff.OpContext, Line: "ctx2\n"},          // old 4, new 4
		},
	}
	selected := selectLines(frag, core.LineSelection{Old: []int{2}, New: []int{2}})

	got := partial(frag, selected, false)

	want := "@@ -1,4 +1,4 @@\n" +
		" ctx1\n" +
		"-selected old\n" +
		"+selected new\n" +
		" unselected old\n" + // unselected "-" folds to context, content kept
		" ctx2\n" // unselected "+" dropped entirely
	if got != want {
		t.Errorf("buildPartialPatch body =\n%q\nwant\n%q", got, want)
	}
}

func TestFR_CHG_03_BuildPartialPatchSelectingEveryChangedLineKeepsAllOfThem(t *testing.T) {
	t.Parallel()
	frag := &gitdiff.TextFragment{
		OldPosition: 1, OldLines: 2,
		NewPosition: 1, NewLines: 2,
		Lines: []gitdiff.Line{
			{Op: gitdiff.OpDelete, Line: "old1\n"}, // old 1
			{Op: gitdiff.OpAdd, Line: "new1\n"},    // new 1
			{Op: gitdiff.OpDelete, Line: "old2\n"}, // old 2
			{Op: gitdiff.OpAdd, Line: "new2\n"},    // new 2
		},
	}
	selected := selectLines(frag, core.LineSelection{Old: []int{1, 2}, New: []int{1, 2}})

	got := partial(frag, selected, false)

	want := "@@ -1,2 +1,2 @@\n-old1\n+new1\n-old2\n+new2\n"
	if got != want {
		t.Errorf("buildPartialPatch body =\n%q\nwant\n%q", got, want)
	}
}

func TestFR_CHG_03_BuildPartialPatchFoldedNoEOLLineKeepsItsOwnMarkerDroppedLineLosesIt(t *testing.T) {
	t.Parallel()
	frag := &gitdiff.TextFragment{
		OldPosition: 1, OldLines: 1,
		NewPosition: 1, NewLines: 1,
		Lines: []gitdiff.Line{
			{Op: gitdiff.OpDelete, Line: "last"}, // old 1, no trailing \n: NoEOL
			{Op: gitdiff.OpAdd, Line: "LAST"},    // new 1, no trailing \n: NoEOL
		},
	}
	selected := selectLines(frag, core.LineSelection{}) // the pair is unselected

	got := partial(frag, selected, false)

	if !strings.Contains(got, " last\n\\ No newline at end of file\n") {
		t.Errorf("buildPartialPatch body = %q, want the folded context line to keep its NoEOL marker", got)
	}
	if strings.Contains(got, "LAST") {
		t.Errorf("buildPartialPatch body = %q, want the dropped \"+\" line and its marker absent", got)
	}
}

// In a reverse (unstage) patch, applied with --reverse, "+" is the removal
// side.
func TestFR_CHG_03_BuildPartialPatchReverseFoldsTheOppositeSide(t *testing.T) {
	t.Parallel()
	frag := &gitdiff.TextFragment{
		OldPosition: 1, OldLines: 2,
		NewPosition: 1, NewLines: 2,
		Lines: []gitdiff.Line{
			{Op: gitdiff.OpDelete, Line: "selected old\n"},   // old 1
			{Op: gitdiff.OpAdd, Line: "selected new\n"},      // new 1
			{Op: gitdiff.OpDelete, Line: "unselected old\n"}, // old 2
			{Op: gitdiff.OpAdd, Line: "unselected new\n"},    // new 2
		},
	}
	selected := selectLines(frag, core.LineSelection{Old: []int{1}, New: []int{1}}) // the first change only

	got := partial(frag, selected, true)

	want := "@@ -1,2 +1,2 @@\n" +
		"-selected old\n" +
		"+selected new\n" +
		" unselected new\n" // unselected "+" folds to context; unselected "-" dropped
	if got != want {
		t.Errorf("buildPartialPatch(reverse=true) body =\n%q\nwant\n%q", got, want)
	}
}

// git lists adjacent changed lines as every "-" line, then every "+" line.
func TestFR_CHG_03_BuildPartialPatchPairsABlockOfDeletesThenAddsByPosition(t *testing.T) {
	t.Parallel()
	frag := &gitdiff.TextFragment{
		OldPosition: 1, OldLines: 2,
		NewPosition: 1, NewLines: 2,
		Lines: []gitdiff.Line{
			{Op: gitdiff.OpDelete, Line: "selected old\n"},   // old 1
			{Op: gitdiff.OpDelete, Line: "unselected old\n"}, // old 2
			{Op: gitdiff.OpAdd, Line: "selected new\n"},      // new 1
			{Op: gitdiff.OpAdd, Line: "unselected new\n"},    // new 2
		},
	}
	selected := selectLines(frag, core.LineSelection{Old: []int{1}, New: []int{1}}) // the first change only

	got := partial(frag, selected, false)

	want := "@@ -1,2 +1,2 @@\n" +
		"-selected old\n" +
		"+selected new\n" +
		" unselected old\n" // paired with the dropped "unselected new", not left stranded before it
	if got != want {
		t.Errorf("buildPartialPatch body =\n%q\nwant\n%q", got, want)
	}
}

func TestFR_CHG_03_FileHeaderLinesStopsBeforeTheFirstHunk(t *testing.T) {
	t.Parallel()
	diff := []byte("diff --git a/f b/f\nindex abc..def 100644\n--- a/f\n+++ b/f\n@@ -1,2 +1,2 @@\n-old\n+new\n")

	got := fileHeaderLines(diff)

	want := "diff --git a/f b/f\nindex abc..def 100644\n--- a/f\n+++ b/f\n"
	if string(got) != want {
		t.Errorf("fileHeaderLines = %q, want %q", got, want)
	}
}

// twoHunksFixture writes a 20-line base commit and an unstaged edit of
// lines 2 and 15 (two separate -U3 hunks), and returns the working-tree
// content. Each edit replaces one line, so its line number is the same on
// both diff sides.
func twoHunksFixture(t *testing.T, dir string) string {
	t.Helper()
	changed := numberedLines(20, map[int]string{2: "line2-changed\n", 15: "line15-changed\n"})
	gittest.Commit(t, dir, "f.txt", numberedLines(20, nil), "base")
	writeFile(t, dir, "f.txt", changed)
	return changed
}

func TestFR_CHG_03_StageLinesStagesAndUnstagesOnlyTheSelectedHunk(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	changed := twoHunksFixture(t, repo.Dir)
	c, id := openWorktree(t, repo.Dir)
	ctx := t.Context()
	staged := func() string { return gittest.Out(t, repo.Dir, "diff", "--cached", "--", "f.txt") }
	unstaged := func() string { return gittest.Out(t, repo.Dir, "diff", "--", "f.txt") }

	if err := c.StageLines(ctx, id, "f.txt", shown(t, c, id, "f.txt", false, []int{2}, []int{2}), false); err != nil {
		t.Fatal(err)
	}
	if got := staged(); !strings.Contains(got, "+line2-changed") || strings.Contains(got, "line15-changed") {
		t.Errorf("staged diff = %q, want only the line2 change", got)
	}
	if got := unstaged(); strings.Contains(got, "line2-changed") || !strings.Contains(got, "+line15-changed") {
		t.Errorf("unstaged diff = %q, want only the line15 change left", got)
	}
	if onDisk := readFile(t, repo.Dir, "f.txt"); onDisk != changed {
		t.Error("working tree file changed, want it byte-identical to before staging (--cached only)")
	}

	if err := c.StageLines(ctx, id, "f.txt", shown(t, c, id, "f.txt", false, []int{15}, []int{15}), false); err != nil {
		t.Fatal(err)
	}
	if got := staged(); !strings.Contains(got, "line2-changed") || !strings.Contains(got, "line15-changed") {
		t.Fatalf("setup: want both changes staged, got %q", got)
	}

	if err := c.StageLines(ctx, id, "f.txt", shown(t, c, id, "f.txt", true, []int{2}, []int{2}), true); err != nil {
		t.Fatal(err)
	}
	if got := staged(); strings.Contains(got, "line2-changed") || !strings.Contains(got, "line15-changed") {
		t.Errorf("staged diff = %q, want only the line15 change to remain staged", got)
	}
	if got := unstaged(); !strings.Contains(got, "+line2-changed") || strings.Contains(got, "line15-changed") {
		t.Errorf("unstaged diff = %q, want the line2 change back", got)
	}
}

// oneHunkFixture writes a base commit and an unstaged edit with exactly
// one hunk, one changed line at line 2 on both diff sides.
func oneHunkFixture(t *testing.T, dir string) {
	t.Helper()
	gittest.Commit(t, dir, "f.txt", "a\nb\nc\n", "base")
	writeFile(t, dir, "f.txt", "a\nB\nc\n")
}

// twoChangeFixture writes a base commit and an unstaged edit of lines 3 and
// 5, which git diff -U3 reports as one hunk. Each edit replaces one line,
// so its line number is the same on both diff sides.
func twoChangeFixture(t *testing.T, dir string) {
	t.Helper()
	gittest.Commit(t, dir, "f.txt", "a\nb\nc\nd\ne\nf\ng\nh\n", "base")
	writeFile(t, dir, "f.txt", "a\nb\nC\nd\nE\nf\ng\nh\n")
}

func TestFR_CHG_03_StageLinesStagesAndUnstagesOnlyTheSelectedChangeEvenWhenGitMergesTwoChangesIntoOneHunk(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	twoChangeFixture(t, repo.Dir)
	c, id := openWorktree(t, repo.Dir)
	stage := func(change int, reverse bool) {
		t.Helper()
		sel := shown(t, c, id, "f.txt", reverse, []int{change}, []int{change})
		if err := c.StageLines(t.Context(), id, "f.txt", sel, reverse); err != nil {
			t.Fatal(err)
		}
	}
	staged := func() string { return gittest.Out(t, repo.Dir, "diff", "--cached", "--", "f.txt") }
	unstaged := func() string { return gittest.Out(t, repo.Dir, "diff", "--", "f.txt") }

	stage(5, false)
	if got := staged(); !strings.Contains(got, "+E") || strings.Contains(got, "+C") {
		t.Errorf("staged diff = %q, want only the e->E change", got)
	}
	if got := unstaged(); strings.Contains(got, "-e") || !strings.Contains(got, "+C") {
		t.Errorf("unstaged diff = %q, want only the c->C change left, in the same hunk", got)
	}

	stage(5, true)
	if got := staged(); got != "" {
		t.Errorf("staged diff = %q, want empty after unstaging the only staged change", got)
	}
	if got := unstaged(); !strings.Contains(got, "+E") || !strings.Contains(got, "+C") {
		t.Errorf("unstaged diff = %q, want both changes back", got)
	}

	stage(3, false)
	stage(5, false)
	stage(3, true)
	if got := staged(); strings.Contains(got, "+C") || !strings.Contains(got, "+E") {
		t.Errorf("staged diff = %q, want only the e->E change to remain staged", got)
	}
	if got := unstaged(); !strings.Contains(got, "+C") || strings.Contains(got, "+E") {
		t.Errorf("unstaged diff = %q, want the c->C change back", got)
	}
}

// A selection naming lines the diff does not change, or lines already
// staged, is Internal, not a git failure, and applies nothing.
func TestFR_CHG_03_StageLinesSelectingLinesTheDiffDoesNotChangeIsInternal(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	oneHunkFixture(t, repo.Dir)
	c, id := openWorktree(t, repo.Dir)
	ctx := t.Context()
	staged := func() string { return gittest.Out(t, repo.Dir, "diff", "--cached", "--", "f.txt") }

	outside := shown(t, c, id, "f.txt", false, []int{99}, []int{99})
	err := c.StageLines(ctx, id, "f.txt", outside, false)

	ce := coretest.AsError(t, err, core.Internal)
	if !strings.Contains(ce.Message, "f.txt") {
		t.Errorf("Message = %q, want it to name the path", ce.Message)
	}
	if got := staged(); got != "" {
		t.Errorf("staged diff = %q, want nothing applied", got)
	}

	sel := shown(t, c, id, "f.txt", false, []int{2}, []int{2})
	if err := c.StageLines(ctx, id, "f.txt", sel, false); err != nil {
		t.Fatal(err)
	}
	stagedAfterFirst := staged()

	err = c.StageLines(ctx, id, "f.txt", sel, false)

	coretest.AssertCode(t, err, core.Internal)
	if got := staged(); got != stagedAfterFirst {
		t.Errorf("staged diff changed after the repeated call = %q, want unchanged %q", got, stagedAfterFirst)
	}
}

// The re-diff of an untracked path lists no file.
func TestFR_CHG_03_StageLinesOnAPathWithNoDiffIsAClearInternalError(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "tracked.txt", "a\n", "base")
	writeFile(t, repo.Dir, "untracked.txt", "new\n") // never `git add`ed

	c, id := openWorktree(t, repo.Dir)
	sel := shown(t, c, id, "untracked.txt", false, []int{1}, []int{1})
	err := c.StageLines(t.Context(), id, "untracked.txt", sel, false)

	ce := coretest.AsError(t, err, core.Internal)
	if !strings.Contains(ce.Message, "untracked.txt") {
		t.Errorf("Message = %q, want it to name the path", ce.Message)
	}
	if strings.Contains(ce.Message, "<nil>") {
		t.Errorf("Message = %q, want a clear message, not a formatted nil error", ce.Message)
	}
	if got := gittest.Out(t, repo.Dir, "diff", "--cached", "--", "untracked.txt"); got != "" {
		t.Errorf("staged diff = %q, want nothing applied", got)
	}
}

func TestFR_CHG_03_StageLinesPreservesTheFinalLineExactlyWhenItsChangeIsNotSelected(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "line1\nline2\nlast", "base") // no trailing newline
	writeFile(t, repo.Dir, "f.txt", "line1\nline2-changed\nLAST")      // still no trailing newline

	c, id := openWorktree(t, repo.Dir)
	ctx := t.Context()
	// only the line2 change, line 2 on both sides
	sel := shown(t, c, id, "f.txt", false, []int{2}, []int{2})
	if err := c.StageLines(ctx, id, "f.txt", sel, false); err != nil {
		t.Fatal(err)
	}

	staged, err := c.DiffWorking(ctx, id, "f.txt", "", true, core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if want := "line1\nline2-changed\nlast"; staged.NewContent != want {
		t.Errorf("staged content = %q, want %q — the final line byte-for-byte untouched, still no trailing newline", staged.NewContent, want)
	}

	unstaged, err := c.DiffWorking(ctx, id, "f.txt", "", false, core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if unstaged.OldContent != staged.NewContent || unstaged.NewContent != "line1\nline2-changed\nLAST" {
		t.Errorf("unstaged diff = %+v, want the final line's own change still pending", unstaged)
	}
}

func TestFR_CHG_03_StageLinesPreservesCRLFLineEndingsExactly(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "line1\r\nline2\r\nline3\r\n", "base")
	writeFile(t, repo.Dir, "f.txt", "line1\r\nline2-changed\r\nline3\r\n")

	c, id := openWorktree(t, repo.Dir)
	ctx := t.Context()
	sel := shown(t, c, id, "f.txt", false, []int{2}, []int{2})
	if err := c.StageLines(ctx, id, "f.txt", sel, false); err != nil {
		t.Fatal(err)
	}

	staged, err := c.DiffWorking(ctx, id, "f.txt", "", true, core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := "line1\r\nline2-changed\r\nline3\r\n"
	if staged.NewContent != want {
		t.Errorf("staged content = %q, want %q — CRLF preserved exactly", staged.NewContent, want)
	}

	unstaged, err := c.DiffWorking(ctx, id, "f.txt", "", false, core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if unstaged.OldContent != want || unstaged.NewContent != want {
		t.Errorf("unstaged diff = %+v, want no remaining difference, CRLF intact", unstaged)
	}
}

func TestFR_CHG_03_StageLinesStagesAHistogramHunkExactlyAsGitShowsIt(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	movedLineFixture(t, repo.Dir)
	gittest.Run(t, repo.Dir, "config", "diff.algorithm", "histogram")

	c, id := openWorktree(t, repo.Dir)
	sel := shown(t, c, id, "m.txt", false, []int{4, 5, 6}, nil)
	if err := c.StageLines(t.Context(), id, "m.txt", sel, false); err != nil {
		t.Fatal(err)
	}

	staged := gittest.Out(t, repo.Dir, "diff", "--cached", "-U0", "--", "m.txt")
	if !strings.HasSuffix(staged, "@@ -4,3 +3,0 @@ C\n-A\n-B\n-C") {
		t.Errorf("staged diff = %q, want only -A -B -C at old lines 4-6", staged)
	}
	if got := readFile(t, repo.Dir, "m.txt"); got != "A\nB\nC\nD\nA\nB\nC\n" {
		t.Errorf("m.txt = %q, want the working tree unchanged", got)
	}
}

func TestFR_CHG_03_StageLinesUnderAutocrlfStagesTheCleanFormAndKeepsTheWorkingFileCRLF(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	autocrlfFixture(t, repo.Dir)

	c, id := openWorktree(t, repo.Dir)
	sel := shown(t, c, id, "f.txt", false, []int{2}, []int{2})
	if err := c.StageLines(t.Context(), id, "f.txt", sel, false); err != nil {
		t.Fatal(err)
	}

	if got := indexBlob(t, repo.Dir, "f.txt"); got != "l1\nCHANGED\nl3" {
		t.Errorf("index f.txt = %q, want LF with line 2 changed", got)
	}
	if got := readFile(t, repo.Dir, "f.txt"); got != "l1\r\nCHANGED\r\nl3\r\n" {
		t.Errorf("f.txt = %q, want the working tree unchanged, CRLF", got)
	}
}

// patternTwins commits f.txt and the file named "[f].txt", which a pathspec
// pattern would also match, then changes line 2 of both the same way.
func patternTwins(t *testing.T, dir string) {
	t.Helper()
	names := []string{"f.txt", "[f].txt"}
	for _, name := range names {
		gittest.Commit(t, dir, name, "a\nb\nc\n", "add "+name)
	}
	for _, name := range names {
		writeFile(t, dir, name, "a\nX\nc\n")
	}
}

func TestFR_CHG_03_StageLinesOfAPatternNamedFileStagesOnlyThatFile(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	patternTwins(t, repo.Dir)
	c, id := openWorktree(t, repo.Dir)

	sel := shown(t, c, id, "[f].txt", false, []int{2}, []int{2})
	if err := c.StageLines(t.Context(), id, "[f].txt", sel, false); err != nil {
		t.Fatal(err)
	}

	if got := indexBlob(t, repo.Dir, "[f].txt"); got != "a\nX\nc" {
		t.Errorf("index [f].txt = %q, want the change staged", got)
	}
	if got := indexBlob(t, repo.Dir, "f.txt"); got != "a\nb\nc" {
		t.Errorf("index f.txt = %q, want it untouched", got)
	}
}

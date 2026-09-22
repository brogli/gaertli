package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
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

func TestFR_CHG_03_BuildPartialPatchKeepsSelectedFoldsUnselectedRemovalToContextDropsUnselectedAddition(t *testing.T) {
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

	got := string(buildPartialPatch(frag, selected, false))

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

	got := string(buildPartialPatch(frag, selected, false))

	want := "@@ -1,2 +1,2 @@\n-old1\n+new1\n-old2\n+new2\n"
	if got != want {
		t.Errorf("buildPartialPatch body =\n%q\nwant\n%q", got, want)
	}
}

func TestFR_CHG_03_BuildPartialPatchFoldedNoEOLLineKeepsItsOwnMarkerDroppedLineLosesIt(t *testing.T) {
	frag := &gitdiff.TextFragment{
		OldPosition: 1, OldLines: 1,
		NewPosition: 1, NewLines: 1,
		Lines: []gitdiff.Line{
			{Op: gitdiff.OpDelete, Line: "last"}, // old 1, no trailing \n: NoEOL
			{Op: gitdiff.OpAdd, Line: "LAST"},    // new 1, no trailing \n: NoEOL
		},
	}
	selected := selectLines(frag, core.LineSelection{}) // the pair is unselected

	got := string(buildPartialPatch(frag, selected, false))

	if !strings.Contains(got, " last\n\\ No newline at end of file\n") {
		t.Errorf("buildPartialPatch body = %q, want the folded context line to keep its NoEOL marker", got)
	}
	if strings.Contains(got, "LAST") {
		t.Errorf("buildPartialPatch body = %q, want the dropped \"+\" line and its marker absent", got)
	}
}

// TestFR_CHG_03_BuildPartialPatchReverseFoldsTheOppositeSide locks down
// that for a reverse (unstage) patch, sourced from `git diff --cached` and
// applied with --reverse, "+" is the removal side — the opposite of the
// stage-direction fold above.
func TestFR_CHG_03_BuildPartialPatchReverseFoldsTheOppositeSide(t *testing.T) {
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

	got := string(buildPartialPatch(frag, selected, true))

	want := "@@ -1,2 +1,2 @@\n" +
		"-selected old\n" +
		"+selected new\n" +
		" unselected new\n" // unselected "+" folds to context; unselected "-" dropped
	if got != want {
		t.Errorf("buildPartialPatch(reverse=true) body =\n%q\nwant\n%q", got, want)
	}
}

// TestFR_CHG_03_BuildPartialPatchPairsABlockOfDeletesThenAddsByPosition
// covers adjacent changed lines, which git lists as every "-" line followed
// by every "+" line.
func TestFR_CHG_03_BuildPartialPatchPairsABlockOfDeletesThenAddsByPosition(t *testing.T) {
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

	got := string(buildPartialPatch(frag, selected, false))

	want := "@@ -1,2 +1,2 @@\n" +
		"-selected old\n" +
		"+selected new\n" +
		" unselected old\n" // paired with the dropped "unselected new", not left stranded before it
	if got != want {
		t.Errorf("buildPartialPatch body =\n%q\nwant\n%q", got, want)
	}
}

func TestFR_CHG_03_FileHeaderLinesStopsBeforeTheFirstHunk(t *testing.T) {
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

func TestFR_CHG_03_StageLinesStagesOnlyTheSelectedHunkLeavingTheOtherUntouched(t *testing.T) {
	repo := gittest.New(t)
	changed := twoHunksFixture(t, repo.Dir)

	c, id := openWorktree(t, repo.Dir)
	sel := core.LineSelection{Old: []int{2}, New: []int{2}}
	if err := c.StageLines(context.Background(), id, "f.txt", sel, false); err != nil {
		t.Fatal(err)
	}

	staged := gittest.Out(t, repo.Dir, "diff", "--cached", "--", "f.txt")
	if !strings.Contains(staged, "+line2-changed") || strings.Contains(staged, "line15-changed") {
		t.Errorf("staged diff = %q, want only the line2 change", staged)
	}
	unstaged := gittest.Out(t, repo.Dir, "diff", "--", "f.txt")
	if strings.Contains(unstaged, "line2-changed") || !strings.Contains(unstaged, "+line15-changed") {
		t.Errorf("unstaged diff = %q, want only the line15 change left", unstaged)
	}
	onDisk, err := os.ReadFile(filepath.Join(repo.Dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(onDisk) != changed {
		t.Error("working tree file changed, want it byte-identical to before staging (--cached only)")
	}
}

func TestFR_CHG_03_StageLinesReverseUnstagesOnlyTheSelectedHunk(t *testing.T) {
	repo := gittest.New(t)
	twoHunksFixture(t, repo.Dir)

	c, id := openWorktree(t, repo.Dir)
	ctx := context.Background()
	both := core.LineSelection{Old: []int{2, 15}, New: []int{2, 15}}
	if err := c.StageLines(ctx, id, "f.txt", both, false); err != nil {
		t.Fatal(err)
	}
	if staged := gittest.Out(t, repo.Dir, "diff", "--cached", "--", "f.txt"); !strings.Contains(staged, "line2-changed") || !strings.Contains(staged, "line15-changed") {
		t.Fatalf("setup: want both changes staged, got %q", staged)
	}

	only2 := core.LineSelection{Old: []int{2}, New: []int{2}}
	if err := c.StageLines(ctx, id, "f.txt", only2, true); err != nil {
		t.Fatal(err)
	}

	staged := gittest.Out(t, repo.Dir, "diff", "--cached", "--", "f.txt")
	if strings.Contains(staged, "line2-changed") || !strings.Contains(staged, "line15-changed") {
		t.Errorf("staged diff = %q, want only the line15 change to remain staged", staged)
	}
	unstaged := gittest.Out(t, repo.Dir, "diff", "--", "f.txt")
	if !strings.Contains(unstaged, "+line2-changed") || strings.Contains(unstaged, "line15-changed") {
		t.Errorf("unstaged diff = %q, want the line2 change back", unstaged)
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

func TestFR_CHG_03_StageLinesStagesOnlyTheSelectedChangeWithinAHunk(t *testing.T) {
	repo := gittest.New(t)
	twoChangeFixture(t, repo.Dir)

	c, id := openWorktree(t, repo.Dir)
	sel := core.LineSelection{Old: []int{3}, New: []int{3}}
	if err := c.StageLines(context.Background(), id, "f.txt", sel, false); err != nil {
		t.Fatal(err)
	}

	staged := gittest.Out(t, repo.Dir, "diff", "--cached", "--", "f.txt")
	if !strings.Contains(staged, "+C") || strings.Contains(staged, "+E") {
		t.Errorf("staged diff = %q, want only the c->C change", staged)
	}
	unstaged := gittest.Out(t, repo.Dir, "diff", "--", "f.txt")
	if strings.Contains(unstaged, "-c") || !strings.Contains(unstaged, "+E") {
		t.Errorf("unstaged diff = %q, want only the e->E change left, in the same hunk", unstaged)
	}
}

func TestFR_CHG_03_StageLinesReverseUnstagesOnlyTheSelectedChange(t *testing.T) {
	repo := gittest.New(t)
	twoChangeFixture(t, repo.Dir)

	c, id := openWorktree(t, repo.Dir)
	ctx := context.Background()
	both := core.LineSelection{Old: []int{3, 5}, New: []int{3, 5}}
	if err := c.StageLines(ctx, id, "f.txt", both, false); err != nil {
		t.Fatal(err)
	}
	if staged := gittest.Out(t, repo.Dir, "diff", "--cached", "--", "f.txt"); !strings.Contains(staged, "+C") || !strings.Contains(staged, "+E") {
		t.Fatalf("setup: want both changes staged, got %q", staged)
	}

	only3 := core.LineSelection{Old: []int{3}, New: []int{3}}
	if err := c.StageLines(ctx, id, "f.txt", only3, true); err != nil {
		t.Fatal(err)
	}

	staged := gittest.Out(t, repo.Dir, "diff", "--cached", "--", "f.txt")
	if strings.Contains(staged, "+C") || !strings.Contains(staged, "+E") {
		t.Errorf("staged diff = %q, want only the e->E change to remain staged", staged)
	}
	unstaged := gittest.Out(t, repo.Dir, "diff", "--", "f.txt")
	if !strings.Contains(unstaged, "+C") || strings.Contains(unstaged, "+E") {
		t.Errorf("unstaged diff = %q, want the c->C change back", unstaged)
	}
}

// TestFR_CHG_03_StageLinesSelectsTheRequestedChangeEvenWhenGitMergesTwoChangesIntoOneHunk
// stages, and separately unstages, one of the two edits git reports as a
// single hunk.
func TestFR_CHG_03_StageLinesSelectsTheRequestedChangeEvenWhenGitMergesTwoChangesIntoOneHunk(t *testing.T) {
	repo := gittest.New(t)
	twoChangeFixture(t, repo.Dir)

	c, id := openWorktree(t, repo.Dir)
	ctx := context.Background()
	second := core.LineSelection{Old: []int{5}, New: []int{5}}
	if err := c.StageLines(ctx, id, "f.txt", second, false); err != nil {
		t.Fatal(err)
	}

	staged := gittest.Out(t, repo.Dir, "diff", "--cached", "--", "f.txt")
	if !strings.Contains(staged, "+E") || strings.Contains(staged, "+C") {
		t.Errorf("staged diff = %q, want only the e->E change", staged)
	}
	unstaged := gittest.Out(t, repo.Dir, "diff", "--", "f.txt")
	if strings.Contains(unstaged, "-e") || !strings.Contains(unstaged, "+C") {
		t.Errorf("unstaged diff = %q, want only the c->C change left, in the same hunk", unstaged)
	}

	if err := c.StageLines(ctx, id, "f.txt", second, true); err != nil {
		t.Fatal(err)
	}

	if staged := gittest.Out(t, repo.Dir, "diff", "--cached", "--", "f.txt"); staged != "" {
		t.Errorf("staged diff = %q, want empty after unstaging the only staged change", staged)
	}
	if unstaged := gittest.Out(t, repo.Dir, "diff", "--", "f.txt"); !strings.Contains(unstaged, "+E") || !strings.Contains(unstaged, "+C") {
		t.Errorf("unstaged diff = %q, want both changes back", unstaged)
	}
}

// TestFR_CHG_03_StageLinesSelectingNoLongerChangedLinesIsInternal proves a
// selection that once matched a change but no longer does — repeating a
// stage call for lines that call already staged — is a stale selection,
// Code Internal, not a git failure.
func TestFR_CHG_03_StageLinesSelectingNoLongerChangedLinesIsInternal(t *testing.T) {
	repo := gittest.New(t)
	oneHunkFixture(t, repo.Dir)

	c, id := openWorktree(t, repo.Dir)
	ctx := context.Background()
	sel := core.LineSelection{Old: []int{2}, New: []int{2}}
	if err := c.StageLines(ctx, id, "f.txt", sel, false); err != nil {
		t.Fatal(err)
	}
	stagedAfterFirst := gittest.Out(t, repo.Dir, "diff", "--cached", "--", "f.txt")

	err := c.StageLines(ctx, id, "f.txt", sel, false)

	coretest.AssertCode(t, err, core.Internal)
	if got := gittest.Out(t, repo.Dir, "diff", "--cached", "--", "f.txt"); got != stagedAfterFirst {
		t.Errorf("staged diff changed after the repeated call = %q, want unchanged %q", got, stagedAfterFirst)
	}
}

// TestFR_CHG_03_StageLinesSelectingLinesNotInTheDiffIsInternal proves a
// selection naming line numbers the current diff never changed is also a
// stale selection, Code Internal, applying nothing.
func TestFR_CHG_03_StageLinesSelectingLinesNotInTheDiffIsInternal(t *testing.T) {
	repo := gittest.New(t)
	oneHunkFixture(t, repo.Dir)

	c, id := openWorktree(t, repo.Dir)
	sel := core.LineSelection{Old: []int{99}, New: []int{99}}
	err := c.StageLines(context.Background(), id, "f.txt", sel, false)

	ce := coretest.AsError(t, err, core.Internal)
	if !strings.Contains(ce.Message, "f.txt") {
		t.Errorf("Message = %q, want it to name the path", ce.Message)
	}
	if got := gittest.Out(t, repo.Dir, "diff", "--cached", "--", "f.txt"); got != "" {
		t.Errorf("staged diff = %q, want nothing applied", got)
	}
}

// TestFR_CHG_03_StageLinesOnAPathWithNoDiffIsAClearInternalError covers a
// re-diff that lists no file at all, as for an untracked path.
func TestFR_CHG_03_StageLinesOnAPathWithNoDiffIsAClearInternalError(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "tracked.txt", "a\n", "base")
	writeFile(t, repo.Dir, "untracked.txt", "new\n") // never `git add`ed

	c, id := openWorktree(t, repo.Dir)
	sel := core.LineSelection{Old: []int{1}, New: []int{1}}
	err := c.StageLines(context.Background(), id, "untracked.txt", sel, false)

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
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "line1\nline2\nlast", "base") // no trailing newline
	writeFile(t, repo.Dir, "f.txt", "line1\nline2-changed\nLAST")      // still no trailing newline

	c, id := openWorktree(t, repo.Dir)
	ctx := context.Background()
	// The line2 change and the final line's own change are both single-line
	// substitutions, so each keeps its own line number (2 and 3) on both
	// diff sides. Select only the line2 change.
	sel := core.LineSelection{Old: []int{2}, New: []int{2}}
	if err := c.StageLines(ctx, id, "f.txt", sel, false); err != nil {
		t.Fatal(err)
	}

	staged, err := c.DiffWorking(ctx, id, "f.txt", "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := "line1\nline2-changed\nlast"; staged.NewContent != want {
		t.Errorf("staged content = %q, want %q — the final line byte-for-byte untouched, still no trailing newline", staged.NewContent, want)
	}

	unstaged, err := c.DiffWorking(ctx, id, "f.txt", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if unstaged.OldContent != staged.NewContent || unstaged.NewContent != "line1\nline2-changed\nLAST" {
		t.Errorf("unstaged diff = %+v, want the final line's own change still pending", unstaged)
	}
}

func TestFR_CHG_03_StageLinesPreservesCRLFLineEndingsExactly(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "line1\r\nline2\r\nline3\r\n", "base")
	writeFile(t, repo.Dir, "f.txt", "line1\r\nline2-changed\r\nline3\r\n")

	c, id := openWorktree(t, repo.Dir)
	ctx := context.Background()
	sel := core.LineSelection{Old: []int{2}, New: []int{2}}
	if err := c.StageLines(ctx, id, "f.txt", sel, false); err != nil {
		t.Fatal(err)
	}

	staged, err := c.DiffWorking(ctx, id, "f.txt", "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	want := "line1\r\nline2-changed\r\nline3\r\n"
	if staged.NewContent != want {
		t.Errorf("staged content = %q, want %q — CRLF preserved exactly", staged.NewContent, want)
	}

	unstaged, err := c.DiffWorking(ctx, id, "f.txt", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if unstaged.OldContent != want || unstaged.NewContent != want {
		t.Errorf("unstaged diff = %+v, want no remaining difference, CRLF intact", unstaged)
	}
}

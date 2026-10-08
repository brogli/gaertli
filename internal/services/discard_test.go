package services

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestFR_CHG_10_DiscardFileRestoresATrackedFileAndRemovesAnUntrackedFileOrDirectory(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "base\n", "base")
	writeFile(t, repo.Dir, "f.txt", "changed\n")
	writeFile(t, repo.Dir, "new.txt", "new")
	// An untracked directory is one "?" record, its path ending in "/". git
	// clean -f removes it without -d: an explicit pathspec names it.
	if err := os.Mkdir(filepath.Join(repo.Dir, "newdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, repo.Dir, "newdir/inner.txt", "content")
	c, id := openWorktree(t, repo.Dir)
	ctx := t.Context()

	if err := c.DiscardFile(ctx, id, "f.txt", false); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, repo.Dir, "f.txt"); got != "base\n" {
		t.Errorf("f.txt = %q, want restored to %q", got, "base\n")
	}

	for _, name := range []string{"new.txt", "newdir"} {
		if err := c.DiscardFile(ctx, id, name, true); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(repo.Dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s still exists, want removed (err = %v)", name, err)
		}
	}
	if status := statusOf(t, c, id); len(status.Unstaged) != 0 || len(status.Untracked) != 0 {
		t.Errorf("Unstaged = %+v, Untracked = %+v, want both empty", status.Unstaged, status.Untracked)
	}
}

func TestFR_CHG_10_DiscardLinesDiscardsOnlyTheSelectedHunkLeavingTheOtherUnstagedChangePending(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	changed := twoHunksFixture(t, repo.Dir)

	c, id := openWorktree(t, repo.Dir)
	sel := shown(t, c, id, "f.txt", false, []int{2}, []int{2})
	if err := c.DiscardLines(t.Context(), id, "f.txt", sel); err != nil {
		t.Fatal(err)
	}

	onDisk := readFile(t, repo.Dir, "f.txt")
	if strings.Contains(onDisk, "line2-changed") {
		t.Errorf("f.txt still has the discarded line2 change: %q", onDisk)
	}
	if !strings.Contains(onDisk, "line15-changed") {
		t.Errorf("f.txt lost the untouched line15 change: %q", onDisk)
	}
	if onDisk == changed {
		t.Error("f.txt unchanged, want the selected hunk discarded")
	}

	status := statusOf(t, c, id)
	if !slices.ContainsFunc(status.Unstaged, func(f core.ChangedFile) bool { return f.Path == "f.txt" }) {
		t.Error("f.txt no longer listed Unstaged, want the remaining line15 change still pending")
	}
}

func TestFR_CHG_10_DiscardLinesOfTheSecondHunkAloneRevertsItsOwnBlockNotAnIdenticalOne(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", twinBlocks("", "CHANGED", "TARGET"), "base")
	writeFile(t, repo.Dir, "f.txt", twinBlocks(tenLines, "CHANGED", "CHANGED"))
	c, id := openWorktree(t, repo.Dir)

	sel := shown(t, c, id, "f.txt", false, []int{21}, []int{31})
	if err := c.DiscardLines(t.Context(), id, "f.txt", sel); err != nil {
		t.Fatal(err)
	}

	if got, want := readFile(t, repo.Dir, "f.txt"), twinBlocks(tenLines, "CHANGED", "TARGET"); got != want {
		t.Errorf("f.txt = %q\nwant %q", got, want)
	}
}

func TestFR_CHG_10_DiscardLinesRestoringLinesAfterALastLineWithoutNewlineGivesThatLineANewline(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "first\nlast\nold1\nold2\n", "base")
	writeFile(t, repo.Dir, "f.txt", "first\nlast")
	c, id := openWorktree(t, repo.Dir)

	if err := c.DiscardLines(t.Context(), id, "f.txt", shown(t, c, id, "f.txt", false, []int{3, 4}, nil)); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, repo.Dir, "f.txt"); got != "first\nlast\nold1\nold2\n" {
		t.Errorf("f.txt = %q, want old1 and old2 back after last, each on its own line", got)
	}
}

// A selection that names a replacement's old lines with its new ones, as a
// hunk action does and as the UI sends a line selection in a replacement
// (see DiffView's selectedLines).
func TestFR_CHG_10_DiscardLinesOfAReplacementRestoresTheReplacedLine(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "a\nb", "base")
	writeFile(t, repo.Dir, "f.txt", "a\nb\nc\n")
	c, id := openWorktree(t, repo.Dir)

	if err := c.DiscardLines(t.Context(), id, "f.txt", shown(t, c, id, "f.txt", false, []int{2}, []int{2, 3})); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, repo.Dir, "f.txt"); got != "a\nb" {
		t.Errorf("f.txt = %q, want the committed last line without newline back", got)
	}
}

func TestFR_CHG_10_DiscardLinesRestoringALineWithoutNewlineThatAKeptLineFollowsGivesItANewline(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "a\nX", "base")
	writeFile(t, repo.Dir, "f.txt", "a\nx\ny\n")
	c, id := openWorktree(t, repo.Dir)

	if err := c.DiscardLines(t.Context(), id, "f.txt", shown(t, c, id, "f.txt", false, []int{2}, []int{2})); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, repo.Dir, "f.txt"); got != "a\nX\ny\n" {
		t.Errorf("f.txt = %q, want X back on its own line before the kept y", got)
	}
}

func TestFR_CHG_10_DiscardLinesRefusesStaleSelectionsAndAppliesNothing(t *testing.T) {
	t.Parallel()
	const base, displayed = "1\n2\n3\n4\n5\n6\n", "1\n2\n3\n4\nX\n5\n6\n"
	tests := []struct {
		name, now, wantMessage string
		old, new               []int
	}{
		{"another line added where the shown one was", "1\n2\n3\n4\nY\nX\n5\n6\n", "changed since it was shown", nil, []int{5}},
		{"an identical line added above the shown one", "1\n2\n3\n4\nX\nY\nX\n5\n6\n", "changed since it was shown", nil, []int{5}},
		{"lines the diff does not change", displayed, "", []int{99}, []int{99}},
	}
	repo := gittest.New(t)
	files := map[string]string{}
	for i := range tests {
		files[fmt.Sprintf("f%d.txt", i)] = base
	}
	gittest.History(t, repo.Dir, gittest.Step{Message: "base", Files: files})
	for i := range tests {
		writeFile(t, repo.Dir, fmt.Sprintf("f%d.txt", i), displayed)
	}
	c, id := openWorktree(t, repo.Dir)
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := fmt.Sprintf("f%d.txt", i)
			sel := shown(t, c, id, name, false, tt.old, tt.new)
			writeFile(t, repo.Dir, name, tt.now)

			err := c.DiscardLines(t.Context(), id, name, sel)

			ce := coretest.AsError(t, err, core.Internal)
			if !strings.Contains(ce.Message, tt.wantMessage) {
				t.Errorf("Message = %q, want it to contain %q", ce.Message, tt.wantMessage)
			}
			if got := readFile(t, repo.Dir, name); got != tt.now {
				t.Errorf("%s = %q, want it unchanged", name, got)
			}
		})
	}
}

func TestFR_CHG_10_DiscardLinesRevertsOnlyTheSelectedHunk(t *testing.T) {
	t.Parallel()
	t.Run("a CRLF working file under autocrlf keeps CRLF on every line", func(t *testing.T) {
		repo := gittest.New(t)
		autocrlfFixture(t, repo.Dir)

		c, id := openWorktree(t, repo.Dir)
		sel := shown(t, c, id, "f.txt", false, []int{2}, []int{2})
		if err := c.DiscardLines(t.Context(), id, "f.txt", sel); err != nil {
			t.Fatal(err)
		}

		if got := readFile(t, repo.Dir, "f.txt"); got != "l1\r\nl2\r\nl3\r\n" {
			t.Errorf("f.txt = %q, want line 2 reverted, CRLF on every line", got)
		}
	})

	t.Run("a histogram hunk", func(t *testing.T) {
		repo := gittest.New(t)
		movedLineFixture(t, repo.Dir)
		gittest.Run(t, repo.Dir, "config", "diff.algorithm", "histogram")

		c, id := openWorktree(t, repo.Dir)
		sel := shown(t, c, id, "m.txt", false, []int{4, 5, 6}, nil)
		if err := c.DiscardLines(t.Context(), id, "m.txt", sel); err != nil {
			t.Fatal(err)
		}

		if got := readFile(t, repo.Dir, "m.txt"); got != "A\nB\nC\nA\nB\nC\nD\nA\nB\nC\n" {
			t.Errorf("m.txt = %q, want the deleted A B C back and the added one kept", got)
		}
	})
}

func TestFR_CHG_10_DiscardFileTakesAGlobLikeNameLiterally(t *testing.T) {
	t.Parallel()
	dir := globRepo(t)
	writeFile(t, dir, "a.txt", "changed\n")
	writeFile(t, dir, "[ab].txt", "changed\n")
	c, id := openWorktree(t, dir)

	if err := c.DiscardFile(t.Context(), id, "[ab].txt", false); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, dir, "a.txt"); got != "changed\n" {
		t.Errorf("a.txt = %q, want the change kept", got)
	}
	if got := readFile(t, dir, "[ab].txt"); got != "ab\n" {
		t.Errorf("[ab].txt = %q, want restored", got)
	}
}

func TestFR_CHG_10_DiscardUntrackedFileTakesAGlobLikeNameLiterally(t *testing.T) {
	t.Parallel()
	dir := gittest.New(t).Dir
	gittest.Commit(t, dir, "seed.txt", "x", "seed")
	writeFile(t, dir, "a.txt", "a")
	writeFile(t, dir, "[ab].txt", "ab")
	c, id := openWorktree(t, dir)

	if err := c.DiscardFile(t.Context(), id, "[ab].txt", true); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "a.txt")); err != nil {
		t.Errorf("a.txt = %v, want kept", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "[ab].txt")); !os.IsNotExist(err) {
		t.Errorf("[ab].txt still exists (err = %v)", err)
	}
}

func TestFR_CHG_10_DiscardLinesOfAPatternNamedFileChangesOnlyThatFile(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	patternTwins(t, repo.Dir)
	c, id := openWorktree(t, repo.Dir)

	sel := shown(t, c, id, "[f].txt", false, []int{2}, []int{2})
	if err := c.DiscardLines(t.Context(), id, "[f].txt", sel); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, repo.Dir, "[f].txt"); got != "a\nb\nc\n" {
		t.Errorf("[f].txt = %q, want the change discarded", got)
	}
	if got := readFile(t, repo.Dir, "f.txt"); got != "a\nX\nc\n" {
		t.Errorf("f.txt = %q, want it untouched", got)
	}
}

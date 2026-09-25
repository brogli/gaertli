package services

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestFR_CHG_10_DiscardFileRestoresATrackedFilesWorkingTreeContent(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "base\n", "base")
	writeFile(t, repo.Dir, "f.txt", "changed\n")

	c, id := openWorktree(t, repo.Dir)
	if err := c.DiscardFile(t.Context(), id, "f.txt", false); err != nil {
		t.Fatal(err)
	}

	content := readFile(t, repo.Dir, "f.txt")
	if content != "base\n" {
		t.Errorf("f.txt = %q, want restored to %q", content, "base\n")
	}
	status := statusOf(t, c, id)
	if len(status.Unstaged) != 0 {
		t.Errorf("Unstaged = %+v, want empty", status.Unstaged)
	}
}

func TestFR_CHG_10_DiscardFileRemovesAnUntrackedFile(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	writeFile(t, repo.Dir, "new.txt", "new")

	c, id := openWorktree(t, repo.Dir)
	if err := c.DiscardFile(t.Context(), id, "new.txt", true); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(repo.Dir, "new.txt")); !os.IsNotExist(err) {
		t.Errorf("new.txt still exists, want removed (err = %v)", err)
	}
	status := statusOf(t, c, id)
	if len(status.Untracked) != 0 {
		t.Errorf("Untracked = %+v, want empty", status.Untracked)
	}
}

// TestFR_CHG_10_DiscardFileRemovesAnUntrackedDirectory: an untracked
// directory (Status reports it as one "?" record, the path ending in "/")
// is fully removed by plain `git clean -f -- path`, with no -d needed —
// an explicit pathspec naming a wholly-untracked directory matches and
// removes it without -d, unlike a directory-less `git clean -f` scanning
// the whole tree.
func TestFR_CHG_10_DiscardFileRemovesAnUntrackedDirectory(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	if err := os.MkdirAll(filepath.Join(repo.Dir, "newdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, repo.Dir, "newdir/inner.txt", "content")

	c, id := openWorktree(t, repo.Dir)
	if err := c.DiscardFile(t.Context(), id, "newdir", true); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(repo.Dir, "newdir")); !os.IsNotExist(err) {
		t.Errorf("newdir still exists, want removed (err = %v)", err)
	}
}

func TestFR_CHG_10_DiscardLinesDiscardsOnlyTheSelectedHunkLeavingTheOtherUnstagedChangePending(t *testing.T) {
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

func TestFR_CHG_10_DiscardLinesStaleSelectionIsInternalAndAppliesNothing(t *testing.T) {
	repo := gittest.New(t)
	oneHunkFixture(t, repo.Dir)
	before := readFile(t, repo.Dir, "f.txt")

	c, id := openWorktree(t, repo.Dir)
	sel := shown(t, c, id, "f.txt", false, []int{99}, []int{99})
	err := c.DiscardLines(t.Context(), id, "f.txt", sel)

	coretest.AssertCode(t, err, core.Internal)
	after := readFile(t, repo.Dir, "f.txt")
	if after != before {
		t.Errorf("f.txt changed = %q, want byte-identical to before the call %q", after, before)
	}
}

func TestFR_CHG_10_DiscardLinesOfTheSecondHunkAloneRevertsItsOwnBlockNotAnIdenticalOne(t *testing.T) {
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
	tests := []struct {
		name, base, now string
		old, new        []int
	}{
		{"a committed last line without newline", "a\nb", "a\nb\nc\n", []int{2}, []int{2, 3}},
		{"a modified line", "a\nx\nc\n", "a\nX\nc\n", []int{2}, []int{2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := gittest.New(t)
			gittest.Commit(t, repo.Dir, "f.txt", tt.base, "base")
			writeFile(t, repo.Dir, "f.txt", tt.now)
			c, id := openWorktree(t, repo.Dir)

			if err := c.DiscardLines(t.Context(), id, "f.txt", shown(t, c, id, "f.txt", false, tt.old, tt.new)); err != nil {
				t.Fatal(err)
			}

			if got := readFile(t, repo.Dir, "f.txt"); got != tt.base {
				t.Errorf("f.txt = %q, want the committed %q back", got, tt.base)
			}
		})
	}
}

func TestFR_CHG_10_DiscardLinesRestoringALineWithoutNewlineThatAKeptLineFollowsGivesItANewline(t *testing.T) {
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

func TestFR_CHG_10_DiscardLinesRefusesLinesThatChangedSinceTheyWereShown(t *testing.T) {
	for name, now := range map[string]string{
		"another line added where the shown one was":  "1\n2\n3\n4\nY\nX\n5\n6\n",
		"an identical line added above the shown one": "1\n2\n3\n4\nX\nY\nX\n5\n6\n",
	} {
		t.Run(name, func(t *testing.T) {
			repo := gittest.New(t)
			gittest.Commit(t, repo.Dir, "f.txt", "1\n2\n3\n4\n5\n6\n", "base")
			writeFile(t, repo.Dir, "f.txt", "1\n2\n3\n4\nX\n5\n6\n")
			c, id := openWorktree(t, repo.Dir)
			sel := shown(t, c, id, "f.txt", false, nil, []int{5})
			writeFile(t, repo.Dir, "f.txt", now)

			err := c.DiscardLines(t.Context(), id, "f.txt", sel)

			ce := coretest.AsError(t, err, core.Internal)
			if !strings.Contains(ce.Message, "changed since it was shown") {
				t.Errorf("Message = %q, want it to say the file changed since it was shown", ce.Message)
			}
			if got := readFile(t, repo.Dir, "f.txt"); got != now {
				t.Errorf("f.txt = %q, want it unchanged", got)
			}
		})
	}
}

func TestFR_CHG_10_DiscardLinesRevertsOnlyTheSelectedHunk(t *testing.T) {
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

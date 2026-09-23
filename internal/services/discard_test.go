package services

import (
	"context"
	"os"
	"path/filepath"
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
	if err := c.DiscardFile(context.Background(), id, "f.txt", false); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(repo.Dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "base\n" {
		t.Errorf("f.txt = %q, want restored to %q", content, "base\n")
	}
	status, err := c.Status(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Unstaged) != 0 {
		t.Errorf("Unstaged = %+v, want empty", status.Unstaged)
	}
}

func TestFR_CHG_10_DiscardFileRemovesAnUntrackedFile(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	writeFile(t, repo.Dir, "new.txt", "new")

	c, id := openWorktree(t, repo.Dir)
	if err := c.DiscardFile(context.Background(), id, "new.txt", true); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(repo.Dir, "new.txt")); !os.IsNotExist(err) {
		t.Errorf("new.txt still exists, want removed (err = %v)", err)
	}
	status, err := c.Status(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
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
	if err := c.DiscardFile(context.Background(), id, "newdir", true); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(repo.Dir, "newdir")); !os.IsNotExist(err) {
		t.Errorf("newdir still exists, want removed (err = %v)", err)
	}
}

// twoUnstagedHunksFixture is twoHunksFixture (hunks_test.go), reused here
// for DiscardLines.
func TestFR_CHG_10_DiscardLinesDiscardsOnlyTheSelectedHunkLeavingTheOtherUnstagedChangePending(t *testing.T) {
	repo := gittest.New(t)
	changed := twoHunksFixture(t, repo.Dir)

	c, id := openWorktree(t, repo.Dir)
	sel := core.LineSelection{Old: []int{2}, New: []int{2}}
	if err := c.DiscardLines(context.Background(), id, "f.txt", sel); err != nil {
		t.Fatal(err)
	}

	onDisk, err := os.ReadFile(filepath.Join(repo.Dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(onDisk), "line2-changed") {
		t.Errorf("f.txt still has the discarded line2 change: %q", onDisk)
	}
	if !strings.Contains(string(onDisk), "line15-changed") {
		t.Errorf("f.txt lost the untouched line15 change: %q", onDisk)
	}
	if string(onDisk) == changed {
		t.Error("f.txt unchanged, want the selected hunk discarded")
	}

	status, err := c.Status(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range status.Unstaged {
		if f.Path == "f.txt" {
			found = true
		}
	}
	if !found {
		t.Error("f.txt no longer listed Unstaged, want the remaining line15 change still pending")
	}
}

func TestFR_CHG_10_DiscardLinesStaleSelectionIsInternalAndAppliesNothing(t *testing.T) {
	repo := gittest.New(t)
	oneHunkFixture(t, repo.Dir)
	before, err := os.ReadFile(filepath.Join(repo.Dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}

	c, id := openWorktree(t, repo.Dir)
	sel := core.LineSelection{Old: []int{99}, New: []int{99}}
	err = c.DiscardLines(context.Background(), id, "f.txt", sel)

	coretest.AssertCode(t, err, core.Internal)
	after, err2 := os.ReadFile(filepath.Join(repo.Dir, "f.txt"))
	if err2 != nil {
		t.Fatal(err2)
	}
	if string(after) != string(before) {
		t.Errorf("f.txt changed = %q, want byte-identical to before the call %q", after, before)
	}
}

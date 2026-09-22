package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git/gittest"
)

// renameWithEdit commits a git-mv rename of name to newName plus a small
// content edit; a large shared body keeps -M's rename similarity above its
// 50% default threshold.
func renameWithEdit(t *testing.T, dir, name, newName, ending string) {
	t.Helper()
	gittest.Run(t, dir, "mv", name, newName)
	base := strings.Repeat("line unchanged\n", 20)
	if err := os.WriteFile(filepath.Join(dir, newName), []byte(base+ending+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, dir, "add", newName)
	gittest.Run(t, dir, "commit", "-m", "rename "+name+" to "+newName)
}

func openCompareRepo(t *testing.T) (*gittest.Repo, *Core, core.WorktreeID) {
	t.Helper()
	repo := gittest.New(t)
	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	return repo, c, wt.ID
}

func TestFR_CMP_01_CompareCommitsListsModifiedAddedAndDeletedFiles(t *testing.T) {
	repo, c, id := openCompareRepo(t)
	gittest.Commit(t, repo.Dir, "f1", "one", "add f1")
	gittest.Commit(t, repo.Dir, "f3", "three", "add f3")
	a := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "f1", "one changed", "modify f1")
	gittest.Commit(t, repo.Dir, "f2", "two", "add f2")
	gittest.Run(t, repo.Dir, "rm", "f3")
	gittest.Run(t, repo.Dir, "commit", "-m", "delete f3")
	b := gittest.HeadSHA(t, repo.Dir)

	files, err := c.CompareCommits(context.Background(), id, a, b)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{"f1": "M", "f2": "A", "f3": "D"}
	if len(files) != len(want) {
		t.Fatalf("CompareCommits = %+v, want %d files", files, len(want))
	}
	for _, f := range files {
		if want[f.Path] != f.Status {
			t.Errorf("file %q status = %q, want %q", f.Path, f.Status, want[f.Path])
		}
	}
}

func TestFR_CMP_01_CompareCommitsDetectsARename(t *testing.T) {
	repo, c, id := openCompareRepo(t)
	gittest.Commit(t, repo.Dir, "f1", strings.Repeat("line unchanged\n", 20)+"old ending\n", "add f1")
	a := gittest.HeadSHA(t, repo.Dir)
	renameWithEdit(t, repo.Dir, "f1", "f2", "new ending")
	b := gittest.HeadSHA(t, repo.Dir)

	files, err := c.CompareCommits(context.Background(), id, a, b)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 1 || files[0].Status != "R" || files[0].OldPath != "f1" || files[0].Path != "f2" {
		t.Errorf("CompareCommits = %+v, want one Rename f1 -> f2", files)
	}
}

func TestFR_CMP_03_CompareCommitsSwappedMirrorsEveryStatus(t *testing.T) {
	repo, c, id := openCompareRepo(t)
	gittest.Commit(t, repo.Dir, "f1", "one", "add f1")
	gittest.Commit(t, repo.Dir, "f3", strings.Repeat("line unchanged\n", 20)+"old ending\n", "add f3")
	a := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "f1", "one changed", "modify f1")
	gittest.Commit(t, repo.Dir, "f2", "two", "add f2")
	renameWithEdit(t, repo.Dir, "f3", "f4", "new ending")
	b := gittest.HeadSHA(t, repo.Dir)

	forward, err := c.CompareCommits(context.Background(), id, a, b)
	if err != nil {
		t.Fatal(err)
	}
	backward, err := c.CompareCommits(context.Background(), id, b, a)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{"f1": "M", "f2": "A", "f4": "R"}
	wantSwapped := map[string]string{"f1": "M", "f2": "D", "f3": "R"}
	if len(forward) != len(want) || len(backward) != len(wantSwapped) {
		t.Fatalf("forward = %+v, backward = %+v", forward, backward)
	}
	for _, f := range forward {
		if want[f.Path] != f.Status {
			t.Errorf("forward file %q status = %q, want %q", f.Path, f.Status, want[f.Path])
		}
	}
	for _, f := range backward {
		if wantSwapped[f.Path] != f.Status {
			t.Errorf("backward file %q status = %q, want %q", f.Path, f.Status, wantSwapped[f.Path])
		}
	}
	var forwardRename, backwardRename core.ChangedFile
	for _, f := range forward {
		if f.Status == "R" {
			forwardRename = f
		}
	}
	for _, f := range backward {
		if f.Status == "R" {
			backwardRename = f
		}
	}
	if forwardRename.OldPath != "f3" || forwardRename.Path != "f4" {
		t.Errorf("forward rename = %+v, want f3 -> f4", forwardRename)
	}
	if backwardRename.OldPath != "f4" || backwardRename.Path != "f3" {
		t.Errorf("backward rename = %+v, want f4 -> f3 (swapped)", backwardRename)
	}
}

func TestFR_CMP_01_CompareCommitsSameCommitIsEmpty(t *testing.T) {
	repo, c, id := openCompareRepo(t)
	gittest.Commit(t, repo.Dir, "f1", "one", "add f1")
	a := gittest.HeadSHA(t, repo.Dir)

	files, err := c.CompareCommits(context.Background(), id, a, a)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("CompareCommits(a, a) = %+v, want empty", files)
	}
}

func TestFR_CMP_02_DiffRangeReturnsContentAtTwoArbitraryCommits(t *testing.T) {
	repo, c, id := openCompareRepo(t)
	gittest.Commit(t, repo.Dir, "f1", "x", "add f1")
	a := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "other", "unrelated", "unrelated commit")
	gittest.Commit(t, repo.Dir, "f1", "y", "change f1")
	b := gittest.HeadSHA(t, repo.Dir)

	diff, err := c.DiffRange(context.Background(), id, a, b, "f1", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "x" || diff.NewContent != "y" {
		t.Errorf("DiffRange = %+v, want OldContent %q NewContent %q", diff, "x", "y")
	}
}

func TestFR_CMP_02_DiffRangeOnFileAddedBetweenTheTwoCommits(t *testing.T) {
	repo, c, id := openCompareRepo(t)
	gittest.Commit(t, repo.Dir, "other", "unrelated", "add other")
	a := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "f1", "added later", "add f1")
	b := gittest.HeadSHA(t, repo.Dir)

	diff, err := c.DiffRange(context.Background(), id, a, b, "f1", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "" || diff.NewContent != "added later" {
		t.Errorf("DiffRange = %+v, want OldContent \"\" NewContent %q (not limited to parent/child commits)", diff, "added later")
	}
}

func TestFR_CMP_02_DiffRangeOnEmbeddedNULByteIsBinaryWithNoContent(t *testing.T) {
	repo, c, id := openCompareRepo(t)
	gittest.Commit(t, repo.Dir, "bin.dat", "abc\x00def", "add binary file")
	a := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "bin.dat", "xyz\x00def", "change binary file")
	b := gittest.HeadSHA(t, repo.Dir)

	diff, err := c.DiffRange(context.Background(), id, a, b, "bin.dat", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !diff.Binary || diff.OldContent != "" || diff.NewContent != "" {
		t.Errorf("DiffRange = %+v, want Binary with no content", diff)
	}
}

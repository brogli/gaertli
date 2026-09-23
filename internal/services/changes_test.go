package services

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestFR_CHG_01_ParseStatusGroupsByXAndYAndHandlesRenameHeaderUnmergedUntracked(t *testing.T) {
	data := []byte(
		"# branch.oid abc123\x00" +
			"1 MM N... 100644 100644 100644 aaaa aaaa a.txt\x00" +
			"2 R. N... 100644 100644 100644 bbbb bbbb R100 d.txt\x00" +
			"b.txt\x00" +
			"u UU N... 100644 100644 100644 100644 cccc dddd eeee f.txt\x00" +
			"? e.txt\x00",
	)

	got := parseStatus(data)

	want := core.ChangedFiles{
		Staged:     []core.ChangedFile{{Status: "M", Path: "a.txt"}, {Status: "R", Path: "d.txt", OldPath: "b.txt"}},
		Unstaged:   []core.ChangedFile{{Status: "M", Path: "a.txt"}},
		Untracked:  []core.ChangedFile{{Status: "?", Path: "e.txt"}},
		Conflicted: []core.ChangedFile{{Status: "UU", Path: "f.txt"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseStatus = %+v, want %+v", got, want)
	}
}

func TestFR_CHG_01_ParseStatusRenameWithUnstagedEditKeepsGroupsSeparate(t *testing.T) {
	data := []byte(
		"2 RM N... 100644 100644 100644 bbbb bbbb R100 newpath.txt\x00" +
			"oldpath.txt\x00",
	)

	got := parseStatus(data)

	wantStaged := []core.ChangedFile{{Status: "R", Path: "newpath.txt", OldPath: "oldpath.txt"}}
	wantUnstaged := []core.ChangedFile{{Status: "M", Path: "newpath.txt"}}
	if !reflect.DeepEqual(got.Staged, wantStaged) {
		t.Errorf("Staged = %+v, want %+v", got.Staged, wantStaged)
	}
	if !reflect.DeepEqual(got.Unstaged, wantUnstaged) {
		t.Errorf("Unstaged = %+v, want %+v", got.Unstaged, wantUnstaged)
	}
}

// openWorktree opens dir on a fresh core that discards its events.
func openWorktree(t *testing.T, dir string) (*Core, core.WorktreeID) {
	t.Helper()
	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	return c, wt.ID
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFR_CHG_01_StatusListsAllFourGroupsIncludingARealConflict(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "conflict.txt", "base\n", "base")
	gittest.Commit(t, repo.Dir, "tracked.txt", "tracked\n", "add tracked.txt")
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "checkout", "feature")
	gittest.Commit(t, repo.Dir, "conflict.txt", "feature\n", "feature change")
	gittest.Run(t, repo.Dir, "checkout", "master")
	gittest.Commit(t, repo.Dir, "conflict.txt", "master\n", "master change")

	// Real merge conflict, set up directly via the runner: leaves MERGE_HEAD
	// and an unmerged conflict.txt.
	_, _ = (git.Runner{}).Run(context.Background(), git.Call{
		Dir: repo.Dir, Args: []string{"merge", "--end-of-options", "feature"},
	})
	if !fileExists(filepath.Join(repo.Dir, ".git", "MERGE_HEAD")) {
		t.Fatal("fixture did not leave MERGE_HEAD")
	}

	writeFile(t, repo.Dir, "staged.txt", "staged")
	gittest.Run(t, repo.Dir, "add", "--", "staged.txt")
	writeFile(t, repo.Dir, "tracked.txt", "changed")
	writeFile(t, repo.Dir, "untracked.txt", "new")

	c, id := openWorktree(t, repo.Dir)
	got, err := c.Status(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}

	if want := (core.ChangedFile{Status: "A", Path: "staged.txt"}); !reflect.DeepEqual(got.Staged, []core.ChangedFile{want}) {
		t.Errorf("Staged = %+v, want [%+v]", got.Staged, want)
	}
	if want := (core.ChangedFile{Status: "M", Path: "tracked.txt"}); !reflect.DeepEqual(got.Unstaged, []core.ChangedFile{want}) {
		t.Errorf("Unstaged = %+v, want [%+v]", got.Unstaged, want)
	}
	if want := (core.ChangedFile{Status: "?", Path: "untracked.txt"}); !reflect.DeepEqual(got.Untracked, []core.ChangedFile{want}) {
		t.Errorf("Untracked = %+v, want [%+v]", got.Untracked, want)
	}
	if want := (core.ChangedFile{Status: "UU", Path: "conflict.txt"}); !reflect.DeepEqual(got.Conflicted, []core.ChangedFile{want}) {
		t.Errorf("Conflicted = %+v, want [%+v]", got.Conflicted, want)
	}
}

func TestFR_CHG_01_StatusListsAFileChangedBothWaysOnce(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "orig\n", "add a.txt")
	writeFile(t, repo.Dir, "a.txt", "staged\n")
	gittest.Run(t, repo.Dir, "add", "--", "a.txt")
	writeFile(t, repo.Dir, "a.txt", "staged\nmore\n")

	c, id := openWorktree(t, repo.Dir)
	got, err := c.Status(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}

	if want := (core.ChangedFile{Status: "M", Path: "a.txt"}); !reflect.DeepEqual(got.Staged, []core.ChangedFile{want}) {
		t.Errorf("Staged = %+v, want [%+v]", got.Staged, want)
	}
	if want := (core.ChangedFile{Status: "M", Path: "a.txt"}); !reflect.DeepEqual(got.Unstaged, []core.ChangedFile{want}) {
		t.Errorf("Unstaged = %+v, want [%+v]", got.Unstaged, want)
	}
}

func TestFR_CHG_01_StatusListsAStagedRenameWithOldPath(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "old.txt", "content\n", "add old.txt")
	gittest.Run(t, repo.Dir, "mv", "old.txt", "new.txt")

	c, id := openWorktree(t, repo.Dir)
	got, err := c.Status(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}

	if want := (core.ChangedFile{Status: "R", Path: "new.txt", OldPath: "old.txt"}); !reflect.DeepEqual(got.Staged, []core.ChangedFile{want}) {
		t.Errorf("Staged = %+v, want [%+v]", got.Staged, want)
	}
	for _, f := range got.Unstaged {
		if f.Path == "new.txt" {
			t.Errorf("Unstaged unexpectedly contains the renamed file: %+v", f)
		}
	}
}

func TestFR_CHG_02_StageFileStagesAnUntrackedFile(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a\n", "add a.txt")
	writeFile(t, repo.Dir, "b.txt", "untracked content\n")

	c, id := openWorktree(t, repo.Dir)
	if err := c.StageFile(context.Background(), id, "b.txt"); err != nil {
		t.Fatal(err)
	}

	got, err := c.Status(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if want := (core.ChangedFile{Status: "A", Path: "b.txt"}); !reflect.DeepEqual(got.Staged, []core.ChangedFile{want}) {
		t.Errorf("Staged = %+v, want [%+v]", got.Staged, want)
	}
	if len(got.Untracked) != 0 {
		t.Errorf("Untracked = %+v, want none", got.Untracked)
	}
}

func TestFR_CHG_02_UnstageFileReturnsAStagedModificationToUnstaged(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "orig\n", "add a.txt")
	writeFile(t, repo.Dir, "a.txt", "modified\n")
	gittest.Run(t, repo.Dir, "add", "--", "a.txt")

	c, id := openWorktree(t, repo.Dir)
	if err := c.UnstageFile(context.Background(), id, "a.txt"); err != nil {
		t.Fatal(err)
	}

	got, err := c.Status(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Staged) != 0 {
		t.Errorf("Staged = %+v, want none", got.Staged)
	}
	if want := (core.ChangedFile{Status: "M", Path: "a.txt"}); !reflect.DeepEqual(got.Unstaged, []core.ChangedFile{want}) {
		t.Errorf("Unstaged = %+v, want [%+v]", got.Unstaged, want)
	}
	if got := gittest.Out(t, repo.Dir, "cat-file", "-p", ":a.txt"); got != "orig" {
		t.Errorf("index content = %q, want HEAD's %q", got, "orig")
	}
}

// git restore --staged is a silent no-op on a clean tracked file; it
// refuses a path with no index entry at all, so the race is a
// never-tracked path.
func TestFR_CHG_02_UnstageFileOnAPathRaceFailsWithGitFailed(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "clean\n", "add a.txt")

	c, id := openWorktree(t, repo.Dir)
	err := c.UnstageFile(context.Background(), id, "never-tracked.txt")

	ce := coretest.AsError(t, err, core.GitFailed)
	if !strings.Contains(ce.GitStderr, "pathspec") {
		t.Errorf("GitStderr = %q, want it to mention the pathspec", ce.GitStderr)
	}
	if got := gittest.Out(t, repo.Dir, "status", "--porcelain"); got != "" {
		t.Errorf("status = %q, want clean (nothing changed)", got)
	}
}

func TestFR_CHG_02_StageAllStagesEveryChangeAtOnce(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "tracked.txt", "orig\n", "add tracked.txt")
	writeFile(t, repo.Dir, "tracked.txt", "modified\n")
	writeFile(t, repo.Dir, "untracked.txt", "new\n")

	c, id := openWorktree(t, repo.Dir)
	if err := c.StageAll(context.Background(), id); err != nil {
		t.Fatal(err)
	}

	got, err := c.Status(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	wantStaged := []core.ChangedFile{{Status: "M", Path: "tracked.txt"}, {Status: "A", Path: "untracked.txt"}}
	if !reflect.DeepEqual(got.Staged, wantStaged) {
		t.Errorf("Staged = %+v, want %+v", got.Staged, wantStaged)
	}
	if len(got.Unstaged) != 0 || len(got.Untracked) != 0 {
		t.Errorf("Unstaged = %+v, Untracked = %+v, want both empty", got.Unstaged, got.Untracked)
	}
}

func TestFR_CHG_02_UnstageAllReturnsEveryFileToItsOwnGroup(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "tracked.txt", "orig\n", "add tracked.txt")
	writeFile(t, repo.Dir, "tracked.txt", "modified\n")
	writeFile(t, repo.Dir, "untracked.txt", "new\n")

	c, id := openWorktree(t, repo.Dir)
	if err := c.StageAll(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if err := c.UnstageAll(context.Background(), id); err != nil {
		t.Fatal(err)
	}

	got, err := c.Status(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Staged) != 0 {
		t.Errorf("Staged = %+v, want none", got.Staged)
	}
	if want := (core.ChangedFile{Status: "M", Path: "tracked.txt"}); !reflect.DeepEqual(got.Unstaged, []core.ChangedFile{want}) {
		t.Errorf("Unstaged = %+v, want [%+v]", got.Unstaged, want)
	}
	if want := (core.ChangedFile{Status: "?", Path: "untracked.txt"}); !reflect.DeepEqual(got.Untracked, []core.ChangedFile{want}) {
		t.Errorf("Untracked = %+v, want [%+v]", got.Untracked, want)
	}
}

func TestFR_CHG_04_DiffWorkingStagedReturnsHeadAndIndexContent(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "head content\n", "add a.txt")
	writeFile(t, repo.Dir, "a.txt", "staged content\n")
	gittest.Run(t, repo.Dir, "add", "--", "a.txt")

	c, id := openWorktree(t, repo.Dir)
	diff, err := c.DiffWorking(context.Background(), id, "a.txt", "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "head content\n" || diff.NewContent != "staged content\n" {
		t.Errorf("DiffWorking = %+v, want OldContent %q NewContent %q", diff, "head content\n", "staged content\n")
	}
}

func TestFR_CHG_04_DiffWorkingStagedOnNewFileHasNoOldContent(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "existing.txt", "x\n", "add existing.txt")
	writeFile(t, repo.Dir, "new.txt", "brand new\n")
	gittest.Run(t, repo.Dir, "add", "--", "new.txt")

	c, id := openWorktree(t, repo.Dir)
	diff, err := c.DiffWorking(context.Background(), id, "new.txt", "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "" || diff.NewContent != "brand new\n" {
		t.Errorf("DiffWorking = %+v, want OldContent \"\" NewContent %q", diff, "brand new\n")
	}
}

func TestFR_CHG_04_DiffWorkingStagedRenameUsesOldPath(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "old.txt", "pre-rename\n", "add old.txt")
	gittest.Run(t, repo.Dir, "mv", "old.txt", "new.txt")
	writeFile(t, repo.Dir, "new.txt", "post-rename\n")
	gittest.Run(t, repo.Dir, "add", "--", "new.txt")

	c, id := openWorktree(t, repo.Dir)
	diff, err := c.DiffWorking(context.Background(), id, "new.txt", "old.txt", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "pre-rename\n" || diff.NewContent != "post-rename\n" {
		t.Errorf("DiffWorking = %+v, want OldContent %q NewContent %q", diff, "pre-rename\n", "post-rename\n")
	}
}

func TestFR_CHG_04_DiffWorkingUnstagedReturnsIndexAndWorkingTreeContent(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "index content\n", "add a.txt")
	writeFile(t, repo.Dir, "a.txt", "on-disk content\n")

	c, id := openWorktree(t, repo.Dir)
	diff, err := c.DiffWorking(context.Background(), id, "a.txt", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "index content\n" || diff.NewContent != "on-disk content\n" {
		t.Errorf("DiffWorking = %+v, want OldContent %q NewContent %q", diff, "index content\n", "on-disk content\n")
	}
}

func TestFR_CHG_04_DiffWorkingUnstagedDeletionHasNoNewContent(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "index content\n", "add a.txt")
	if err := os.Remove(filepath.Join(repo.Dir, "a.txt")); err != nil {
		t.Fatal(err)
	}

	c, id := openWorktree(t, repo.Dir)
	diff, err := c.DiffWorking(context.Background(), id, "a.txt", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "index content\n" || diff.NewContent != "" {
		t.Errorf("DiffWorking = %+v, want OldContent %q NewContent \"\"", diff, "index content\n")
	}
}

func TestFR_CHG_04_DiffWorkingUntrackedHasNoOldContent(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "existing.txt", "x\n", "add existing.txt")
	writeFile(t, repo.Dir, "new.txt", "on-disk\n")

	c, id := openWorktree(t, repo.Dir)
	diff, err := c.DiffWorking(context.Background(), id, "new.txt", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "" || diff.NewContent != "on-disk\n" {
		t.Errorf("DiffWorking = %+v, want OldContent \"\" NewContent %q", diff, "on-disk\n")
	}
}

func TestFR_CHG_04_DiffWorkingHandlesAFileChangedBothWaysCorrectlyForBothCalls(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "orig\n", "add a.txt")
	writeFile(t, repo.Dir, "a.txt", "staged\n")
	gittest.Run(t, repo.Dir, "add", "--", "a.txt")
	writeFile(t, repo.Dir, "a.txt", "staged\nmore\n")

	c, id := openWorktree(t, repo.Dir)

	staged, err := c.DiffWorking(context.Background(), id, "a.txt", "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if staged.NewContent != "staged\n" {
		t.Errorf("staged.NewContent = %q, want %q", staged.NewContent, "staged\n")
	}

	unstaged, err := c.DiffWorking(context.Background(), id, "a.txt", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if unstaged.OldContent != "staged\n" || unstaged.NewContent != "staged\nmore\n" {
		t.Errorf("unstaged = %+v, want OldContent %q NewContent %q", unstaged, "staged\n", "staged\nmore\n")
	}
}

func TestFR_CHG_04_DiffWorkingUntrackedBinaryFileHasNoContent(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "existing.txt", "x\n", "add existing.txt")
	writeFile(t, repo.Dir, "bin.dat", "abc\x00def")

	c, id := openWorktree(t, repo.Dir)
	diff, err := c.DiffWorking(context.Background(), id, "bin.dat", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !diff.Binary || diff.OldContent != "" || diff.NewContent != "" {
		t.Errorf("DiffWorking = %+v, want Binary with no content", diff)
	}
}

func TestFR_CHG_04_DiffWorkingUnstagedBinaryFileHasNoContent(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "bin.dat", "abc\x00def", "add binary file")
	writeFile(t, repo.Dir, "bin.dat", "xyz\x00123")

	c, id := openWorktree(t, repo.Dir)
	diff, err := c.DiffWorking(context.Background(), id, "bin.dat", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !diff.Binary {
		t.Errorf("DiffWorking.Binary = false, want true")
	}
}

func TestFR_CHG_04_DiffWorkingUnstagedCapsHugeFilesWithAForceReloadBypass(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "big.txt", strings.Repeat("a", maxDiffFileSize+1), "add big file")
	big := strings.Repeat("b", maxDiffFileSize+1)
	writeFile(t, repo.Dir, "big.txt", big)

	c, id := openWorktree(t, repo.Dir)
	capped, err := c.DiffWorking(context.Background(), id, "big.txt", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !capped.TooLarge || capped.OldContent != "" || capped.NewContent != "" {
		t.Errorf("DiffWorking = %+v, want TooLarge with no content", capped)
	}

	full, err := c.DiffWorking(context.Background(), id, "big.txt", "", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if full.TooLarge || full.NewContent != big {
		t.Error("DiffWorking with force=true: still TooLarge or content mismatch, want the full content")
	}
}

func TestFR_CHG_04_DiffWorkingIsBinaryTreatsNoIndexExit1AsATolerableDifference(t *testing.T) {
	tests := []struct {
		name       string
		exit       string
		wantErr    bool
		wantBinary bool
	}{
		{"exit 1: --no-index's real-difference convention, tolerated", "1", false, true},
		{"exit 2: a genuine failure, not tolerated", "2", true, false},
		{"exit 0: no difference, parses normally", "0", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("FAKEGIT_STDOUT", "-\t-\tfile")
			t.Setenv("FAKEGIT_EXIT", tt.exit)
			r := git.Runner{Git: fakegit}

			binary, err := diffWorkingIsBinary(context.Background(), r, t.TempDir(), "file", true, oldSide{})
			if tt.wantErr {
				if err == nil {
					t.Fatal("diffWorkingIsBinary() error = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if binary != tt.wantBinary {
				t.Errorf("binary = %v, want %v", binary, tt.wantBinary)
			}
		})
	}
}

func TestFR_CHG_04_DiffWorkingTreeOldSideIsPassedExplicitlyNotParsedFromAString(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "head\n", "add a.txt")
	head := gittest.HeadSHA(t, repo.Dir)
	writeFile(t, repo.Dir, "a.txt", "working\n") // unstaged edit, index still holds "head\n"

	r := git.Runner{}

	index, err := diffWorkingTree(context.Background(), r, repo.Dir, "a.txt", oldSide{path: "a.txt"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if index.OldContent != "head\n" || index.NewContent != "working\n" {
		t.Errorf("diffWorkingTree(index old side) = %+v, want OldContent %q NewContent %q", index, "head\n", "working\n")
	}

	commit, err := diffWorkingTree(context.Background(), r, repo.Dir, "a.txt", oldSide{sha: head, path: "a.txt"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if commit.OldContent != "head\n" || commit.NewContent != "working\n" {
		t.Errorf("diffWorkingTree(commit old side) = %+v, want OldContent %q NewContent %q", commit, "head\n", "working\n")
	}
}

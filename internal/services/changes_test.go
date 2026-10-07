package services

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestFR_CHG_01_ParseStatusGroupsByXAndYAndHandlesRenameHeaderUnmergedUntracked(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	return c, openOn(t, c, dir).ID
}

// openOn opens dir on c, closing the session when the test ends.
func openOn(t *testing.T, c *Core, dir string) core.WorktreeInfo {
	t.Helper()
	wt, err := c.OpenWorktree(t.Context(), core.RepoPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.CloseWorktree(context.Background(), wt.ID) })
	return wt
}

// statusOf is id's Status; the test fails on an error.
func statusOf(t *testing.T, c *Core, id core.WorktreeID) core.ChangedFiles {
	t.Helper()
	status, err := c.Status(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	return status
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFR_CHG_01_StatusListsAllFourGroupsIncludingARealConflict(t *testing.T) {
	t.Parallel()
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
	_, _ = (git.Runner{}).Run(t.Context(), git.Call{
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
	got := statusOf(t, c, id)

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
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "orig\n", "add a.txt")
	writeFile(t, repo.Dir, "a.txt", "staged\n")
	gittest.Run(t, repo.Dir, "add", "--", "a.txt")
	writeFile(t, repo.Dir, "a.txt", "staged\nmore\n")

	c, id := openWorktree(t, repo.Dir)
	got := statusOf(t, c, id)

	if want := (core.ChangedFile{Status: "M", Path: "a.txt"}); !reflect.DeepEqual(got.Staged, []core.ChangedFile{want}) {
		t.Errorf("Staged = %+v, want [%+v]", got.Staged, want)
	}
	if want := (core.ChangedFile{Status: "M", Path: "a.txt"}); !reflect.DeepEqual(got.Unstaged, []core.ChangedFile{want}) {
		t.Errorf("Unstaged = %+v, want [%+v]", got.Unstaged, want)
	}
}

func TestFR_CHG_01_StatusListsAStagedRenameWithOldPath(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "old.txt", "content\n", "add old.txt")
	gittest.Run(t, repo.Dir, "mv", "old.txt", "new.txt")

	c, id := openWorktree(t, repo.Dir)
	got := statusOf(t, c, id)

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
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a\n", "add a.txt")
	writeFile(t, repo.Dir, "b.txt", "untracked content\n")

	c, id := openWorktree(t, repo.Dir)
	if err := c.StageFile(t.Context(), id, "b.txt"); err != nil {
		t.Fatal(err)
	}

	got := statusOf(t, c, id)
	if want := (core.ChangedFile{Status: "A", Path: "b.txt"}); !reflect.DeepEqual(got.Staged, []core.ChangedFile{want}) {
		t.Errorf("Staged = %+v, want [%+v]", got.Staged, want)
	}
	if len(got.Untracked) != 0 {
		t.Errorf("Untracked = %+v, want none", got.Untracked)
	}
}

func TestFR_CHG_02_UnstageFileReturnsAStagedModificationToUnstaged(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "orig\n", "add a.txt")
	writeFile(t, repo.Dir, "a.txt", "modified\n")
	gittest.Run(t, repo.Dir, "add", "--", "a.txt")

	c, id := openWorktree(t, repo.Dir)
	if err := c.UnstageFile(t.Context(), id, "a.txt"); err != nil {
		t.Fatal(err)
	}

	got := statusOf(t, c, id)
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
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "clean\n", "add a.txt")

	c, id := openWorktree(t, repo.Dir)
	err := c.UnstageFile(t.Context(), id, "never-tracked.txt")

	ce := coretest.AsError(t, err, core.GitFailed)
	if !strings.Contains(ce.GitStderr, "pathspec") {
		t.Errorf("GitStderr = %q, want it to mention the pathspec", ce.GitStderr)
	}
	if got := gittest.Out(t, repo.Dir, "status", "--porcelain"); got != "" {
		t.Errorf("status = %q, want clean (nothing changed)", got)
	}
}

func TestFR_CHG_02_StageAllStagesEveryChangeAtOnce(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "tracked.txt", "orig\n", "add tracked.txt")
	writeFile(t, repo.Dir, "tracked.txt", "modified\n")
	writeFile(t, repo.Dir, "untracked.txt", "new\n")

	c, id := openWorktree(t, repo.Dir)
	if err := c.StageAll(t.Context(), id); err != nil {
		t.Fatal(err)
	}

	got := statusOf(t, c, id)
	wantStaged := []core.ChangedFile{{Status: "M", Path: "tracked.txt"}, {Status: "A", Path: "untracked.txt"}}
	if !reflect.DeepEqual(got.Staged, wantStaged) {
		t.Errorf("Staged = %+v, want %+v", got.Staged, wantStaged)
	}
	if len(got.Unstaged) != 0 || len(got.Untracked) != 0 {
		t.Errorf("Unstaged = %+v, Untracked = %+v, want both empty", got.Unstaged, got.Untracked)
	}
}

func TestFR_CHG_02_UnstageAllReturnsEveryFileToItsOwnGroup(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "tracked.txt", "orig\n", "add tracked.txt")
	writeFile(t, repo.Dir, "tracked.txt", "modified\n")
	writeFile(t, repo.Dir, "untracked.txt", "new\n")

	c, id := openWorktree(t, repo.Dir)
	if err := c.StageAll(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	if err := c.UnstageAll(t.Context(), id); err != nil {
		t.Fatal(err)
	}

	got := statusOf(t, c, id)
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
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "head content\n", "add a.txt")
	writeFile(t, repo.Dir, "a.txt", "staged content\n")
	gittest.Run(t, repo.Dir, "add", "--", "a.txt")

	c, id := openWorktree(t, repo.Dir)
	diff, err := c.DiffWorking(t.Context(), id, "a.txt", "", true, core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "head content\n" || diff.NewContent != "staged content\n" {
		t.Errorf("DiffWorking = %+v, want OldContent %q NewContent %q", diff, "head content\n", "staged content\n")
	}
}

func TestFR_CHG_04_DiffWorkingStagedOnNewFileHasNoOldContent(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "existing.txt", "x\n", "add existing.txt")
	writeFile(t, repo.Dir, "new.txt", "brand new\n")
	gittest.Run(t, repo.Dir, "add", "--", "new.txt")

	c, id := openWorktree(t, repo.Dir)
	diff, err := c.DiffWorking(t.Context(), id, "new.txt", "", true, core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "" || diff.NewContent != "brand new\n" {
		t.Errorf("DiffWorking = %+v, want OldContent \"\" NewContent %q", diff, "brand new\n")
	}
}

// A rename git detects: -M pairs paths whose content is at least 50%
// similar.
func TestFR_CHG_04_DiffWorkingStagedRenameUsesOldPath(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	base := strings.Repeat("line unchanged\n", 20)
	gittest.Commit(t, repo.Dir, "old.txt", base+"pre-rename\n", "add old.txt")
	gittest.Run(t, repo.Dir, "mv", "old.txt", "new.txt")
	writeFile(t, repo.Dir, "new.txt", base+"post-rename\n")
	gittest.Run(t, repo.Dir, "add", "--", "new.txt")

	c, id := openWorktree(t, repo.Dir)
	diff, err := c.DiffWorking(t.Context(), id, "new.txt", "old.txt", true, core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != base+"pre-rename\n" || diff.NewContent != base+"post-rename\n" {
		t.Errorf("DiffWorking = %+v, want OldContent %q NewContent %q", diff, base+"pre-rename\n", base+"post-rename\n")
	}
}

func TestFR_CHG_04_DiffWorkingUnstagedReturnsIndexAndWorkingTreeContent(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "index content\n", "add a.txt")
	writeFile(t, repo.Dir, "a.txt", "on-disk content\n")

	c, id := openWorktree(t, repo.Dir)
	diff, err := c.DiffWorking(t.Context(), id, "a.txt", "", false, core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "index content\n" || diff.NewContent != "on-disk content\n" {
		t.Errorf("DiffWorking = %+v, want OldContent %q NewContent %q", diff, "index content\n", "on-disk content\n")
	}
}

func TestFR_CHG_04_DiffWorkingUnstagedDeletionHasNoNewContent(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "index content\n", "add a.txt")
	if err := os.Remove(filepath.Join(repo.Dir, "a.txt")); err != nil {
		t.Fatal(err)
	}

	c, id := openWorktree(t, repo.Dir)
	diff, err := c.DiffWorking(t.Context(), id, "a.txt", "", false, core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "index content\n" || diff.NewContent != "" {
		t.Errorf("DiffWorking = %+v, want OldContent %q NewContent \"\"", diff, "index content\n")
	}
}

func TestFR_CHG_04_DiffWorkingUntrackedHasNoOldContent(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "existing.txt", "x\n", "add existing.txt")
	writeFile(t, repo.Dir, "new.txt", "on-disk\n")

	c, id := openWorktree(t, repo.Dir)
	diff, err := c.DiffWorking(t.Context(), id, "new.txt", "", false, core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if diff.OldContent != "" || diff.NewContent != "on-disk\n" {
		t.Errorf("DiffWorking = %+v, want OldContent \"\" NewContent %q", diff, "on-disk\n")
	}
}

func TestFR_CHG_04_DiffWorkingHandlesAFileChangedBothWaysCorrectlyForBothCalls(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "orig\n", "add a.txt")
	writeFile(t, repo.Dir, "a.txt", "staged\n")
	gittest.Run(t, repo.Dir, "add", "--", "a.txt")
	writeFile(t, repo.Dir, "a.txt", "staged\nmore\n")

	c, id := openWorktree(t, repo.Dir)

	staged, err := c.DiffWorking(t.Context(), id, "a.txt", "", true, core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if staged.NewContent != "staged\n" {
		t.Errorf("staged.NewContent = %q, want %q", staged.NewContent, "staged\n")
	}

	unstaged, err := c.DiffWorking(t.Context(), id, "a.txt", "", false, core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if unstaged.OldContent != "staged\n" || unstaged.NewContent != "staged\nmore\n" {
		t.Errorf("unstaged = %+v, want OldContent %q NewContent %q", unstaged, "staged\n", "staged\nmore\n")
	}
}

func TestFR_CHG_04_DiffWorkingUntrackedBinaryFileHasNoContent(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "existing.txt", "x\n", "add existing.txt")
	writeFile(t, repo.Dir, "bin.dat", "abc\x00def")

	c, id := openWorktree(t, repo.Dir)
	diff, err := c.DiffWorking(t.Context(), id, "bin.dat", "", false, core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !diff.Binary || diff.OldContent != "" || diff.NewContent != "" {
		t.Errorf("DiffWorking = %+v, want Binary with no content", diff)
	}
}

func TestFR_CHG_04_DiffWorkingUnstagedBinaryFileHasNoContent(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "bin.dat", "abc\x00def", "add binary file")
	writeFile(t, repo.Dir, "bin.dat", "xyz\x00123")

	c, id := openWorktree(t, repo.Dir)
	diff, err := c.DiffWorking(t.Context(), id, "bin.dat", "", false, core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !diff.Binary {
		t.Errorf("DiffWorking.Binary = false, want true")
	}
}

func TestFR_CHG_04_DiffWorkingUnstagedCapsHugeFilesWithAForceReloadBypass(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "big.txt", strings.Repeat("a", maxDiffFileSize+1), "add big file")
	big := strings.Repeat("b", maxDiffFileSize+1)
	writeFile(t, repo.Dir, "big.txt", big)

	c, id := openWorktree(t, repo.Dir)
	capped, err := c.DiffWorking(t.Context(), id, "big.txt", "", false, core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !capped.TooLarge || capped.OldContent != "" || capped.NewContent != "" {
		t.Errorf("DiffWorking = %+v, want TooLarge with no content", capped)
	}

	full, err := c.DiffWorking(t.Context(), id, "big.txt", "", false, core.DiffOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if full.TooLarge || full.NewContent != big {
		t.Error("DiffWorking with force=true: still TooLarge or content mismatch, want the full content")
	}
	if want := []core.DiffHunk{hunk(1, 1, 1, 1)}; !slices.Equal(full.Hunks, want) {
		t.Errorf("DiffWorking with force=true: Hunks = %v, want %v", full.Hunks, want)
	}
}

func TestFR_CHG_04_DiffWorkingShowsAFileWhoseNameHoldsANewline(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows file names cannot hold a newline")
	}
	const name = "a\nb.txt"
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, name, "old\n", "add")
	writeFile(t, repo.Dir, name, "new\n")

	c, id := openWorktree(t, repo.Dir)
	for _, staged := range []bool{false, true} {
		if staged {
			gittest.Run(t, repo.Dir, "add", "--", name)
		}
		diff, err := c.DiffWorking(t.Context(), id, name, "", staged, core.DiffOptions{})
		if err != nil {
			t.Fatalf("staged=%v: %v", staged, err)
		}
		if diff.OldContent != "old\n" || diff.NewContent != "new\n" {
			t.Errorf("staged=%v: DiffWorking = %q → %q, want %q → %q", staged, diff.OldContent, diff.NewContent, "old\n", "new\n")
		}
	}
}

// globRepo commits a.txt and [ab].txt; the second's name matches the first
// as a glob.
func globRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.New(t).Dir
	writeFile(t, dir, "a.txt", "a\n")
	writeFile(t, dir, "[ab].txt", "ab\n")
	gittest.Run(t, dir, "add", "--", ":(literal)a.txt", ":(literal)[ab].txt")
	gittest.CommitAll(t, dir, "base")
	return dir
}

func commitAll(t *testing.T, dir, msg string) {
	t.Helper()
	gittest.Run(t, dir, "add", "-A")
	gittest.CommitAll(t, dir, msg)
}

func TestFR_CONF_03_StageFileTakesAGlobLikeNameLiterally(t *testing.T) {
	t.Parallel()
	dir := globRepo(t)
	gittest.Run(t, dir, "checkout", "-q", "-b", "side")
	writeFile(t, dir, "a.txt", "side\n")
	writeFile(t, dir, "[ab].txt", "side\n")
	commitAll(t, dir, "side")
	gittest.Run(t, dir, "checkout", "-q", "master")
	writeFile(t, dir, "a.txt", "cur\n")
	writeFile(t, dir, "[ab].txt", "cur\n")
	commitAll(t, dir, "cur")
	tryGit(t, dir, "merge", "side")
	c, id := openWorktree(t, dir)

	if err := c.StageFile(t.Context(), id, "[ab].txt"); err != nil {
		t.Fatal(err)
	}

	got := statusOf(t, c, id)
	if len(got.Conflicted) != 1 || got.Conflicted[0].Path != "a.txt" {
		t.Errorf("Conflicted = %+v, want only a.txt", got.Conflicted)
	}
}

func TestFR_CHG_02_UnstageFileTakesAGlobLikeNameLiterally(t *testing.T) {
	t.Parallel()
	dir := globRepo(t)
	writeFile(t, dir, "a.txt", "changed\n")
	writeFile(t, dir, "[ab].txt", "changed\n")
	gittest.Run(t, dir, "add", "-A")
	c, id := openWorktree(t, dir)

	if err := c.UnstageFile(t.Context(), id, "[ab].txt"); err != nil {
		t.Fatal(err)
	}

	got := statusOf(t, c, id)
	if len(got.Staged) != 1 || got.Staged[0].Path != "a.txt" {
		t.Errorf("Staged = %+v, want only a.txt", got.Staged)
	}
}

func TestFR_CHG_04_DiffWorkingOfAPatternNamedFileShowsItsOwnContent(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "f.txt", "one\n", "add f.txt")
	gittest.Commit(t, repo.Dir, "[f].txt", "other\n", "add [f].txt")
	writeFile(t, repo.Dir, "f.txt", "one changed\n")
	writeFile(t, repo.Dir, "[f].txt", "other changed\n")
	c, id := openWorktree(t, repo.Dir)

	diff, err := c.DiffWorking(t.Context(), id, "[f].txt", "", false, core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if diff.OldContent != "other\n" || diff.NewContent != "other changed\n" {
		t.Errorf("DiffWorking = %+v, want only [f].txt's contents", diff)
	}
}

func TestFR_CHG_04_DiffWorkingOfAPathLikeAStageNumberShowsItsOwnContent(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows file names cannot hold a colon")
	}
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "x", "plain x\n", "add x")
	gittest.Commit(t, repo.Dir, "1:x", "one colon x\n", "add 1:x")
	writeFile(t, repo.Dir, "1:x", "one colon x changed\n")
	c, id := openWorktree(t, repo.Dir)

	diff, err := c.DiffWorking(t.Context(), id, "1:x", "", false, core.DiffOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if diff.OldContent != "one colon x\n" || diff.NewContent != "one colon x changed\n" {
		t.Errorf("DiffWorking = %+v, want only 1:x's contents", diff)
	}
}

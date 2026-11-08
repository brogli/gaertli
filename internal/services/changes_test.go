// Requirements: FR-CHG-01, FR-CHG-02, FR-CHG-04, FR-CONF-03

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

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/core/coretest"
	"github.com/brogli/gaertli/internal/git/gittest"
)

func TestParseStatusGroupsByXAndYAndHandlesRenameHeaderUnmergedUntracked(t *testing.T) {
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

func TestParseStatusRenameWithUnstagedEditKeepsGroupsSeparate(t *testing.T) {
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

// readFile returns name's working-tree content in dir.
func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(got)
}

// indexBlob returns name's staged content in dir.
func indexBlob(t *testing.T, dir, name string) string {
	t.Helper()
	return gittest.Out(t, dir, "show", ":"+name)
}

func TestStatusListsAllFourGroupsIncludingARealConflictAndAFileChangedBothWaysAndAStagedRename(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "conflict.txt", "base\n", "base")
	gittest.Commit(t, repo.Dir, "tracked.txt", "tracked\n", "add tracked.txt")
	gittest.Commit(t, repo.Dir, "both.txt", "orig\n", "add both.txt")
	gittest.Commit(t, repo.Dir, "old.txt", "content\n", "add old.txt")
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "checkout", "feature")
	gittest.Commit(t, repo.Dir, "conflict.txt", "feature\n", "feature change")
	gittest.Run(t, repo.Dir, "checkout", "master")
	gittest.Commit(t, repo.Dir, "conflict.txt", "master\n", "master change")

	// Real merge conflict, set up directly via the runner: leaves MERGE_HEAD
	// and an unmerged conflict.txt.
	tryGit(t, repo.Dir, "merge", "--end-of-options", "feature")
	if !fileExists(filepath.Join(repo.Dir, ".git", "MERGE_HEAD")) {
		t.Fatal("fixture did not leave MERGE_HEAD")
	}

	writeFile(t, repo.Dir, "staged.txt", "staged")
	gittest.Run(t, repo.Dir, "add", "--", "staged.txt")
	writeFile(t, repo.Dir, "tracked.txt", "changed")
	writeFile(t, repo.Dir, "untracked.txt", "new")
	writeFile(t, repo.Dir, "both.txt", "staged\n")
	gittest.Run(t, repo.Dir, "add", "--", "both.txt")
	writeFile(t, repo.Dir, "both.txt", "staged\nmore\n")
	gittest.Run(t, repo.Dir, "mv", "old.txt", "new.txt")

	c, id := openWorktree(t, repo.Dir)
	got := statusOf(t, c, id)

	wantStaged := []core.ChangedFile{
		{Status: "M", Path: "both.txt"},
		{Status: "R", Path: "new.txt", OldPath: "old.txt"},
		{Status: "A", Path: "staged.txt"},
	}
	if !reflect.DeepEqual(got.Staged, wantStaged) {
		t.Errorf("Staged = %+v, want %+v", got.Staged, wantStaged)
	}
	wantUnstaged := []core.ChangedFile{{Status: "M", Path: "both.txt"}, {Status: "M", Path: "tracked.txt"}}
	if !reflect.DeepEqual(got.Unstaged, wantUnstaged) {
		t.Errorf("Unstaged = %+v, want %+v", got.Unstaged, wantUnstaged)
	}
	if want := (core.ChangedFile{Status: "?", Path: "untracked.txt"}); !reflect.DeepEqual(got.Untracked, []core.ChangedFile{want}) {
		t.Errorf("Untracked = %+v, want [%+v]", got.Untracked, want)
	}
	if want := (core.ChangedFile{Status: "UU", Path: "conflict.txt"}); !reflect.DeepEqual(got.Conflicted, []core.ChangedFile{want}) {
		t.Errorf("Conflicted = %+v, want [%+v]", got.Conflicted, want)
	}
}

func TestStageFileStagesAnUntrackedFile(t *testing.T) {
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

func TestUnstageFileReturnsAStagedModificationToUnstaged(t *testing.T) {
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
	if got := indexBlob(t, repo.Dir, "a.txt"); got != "orig" {
		t.Errorf("index content = %q, want HEAD's %q", got, "orig")
	}
}

// git restore --staged is a silent no-op on a clean tracked file; it
// refuses a path with no index entry at all, so the race is a
// never-tracked path.
func TestUnstageFileOnAPathRaceFailsWithGitFailed(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "clean\n", "add a.txt")

	c, id := openWorktree(t, repo.Dir)
	err := c.UnstageFile(t.Context(), id, "never-tracked.txt")

	ce := coretest.AsError(t, err, core.GitFailed)
	if ce.GitStderr == "" {
		t.Error("GitStderr empty, want git's own explanation")
	}
	if got := gittest.Out(t, repo.Dir, "status", "--porcelain"); got != "" {
		t.Errorf("status = %q, want clean (nothing changed)", got)
	}
}

func TestStageAllStagesEveryChangeAndUnstageAllReturnsEveryFileToItsOwnGroup(t *testing.T) {
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
		t.Errorf("after StageAll: Staged = %+v, want %+v", got.Staged, wantStaged)
	}
	if len(got.Unstaged) != 0 || len(got.Untracked) != 0 {
		t.Errorf("after StageAll: Unstaged = %+v, Untracked = %+v, want both empty", got.Unstaged, got.Untracked)
	}

	if err := c.UnstageAll(t.Context(), id); err != nil {
		t.Fatal(err)
	}

	got = statusOf(t, c, id)
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

func TestDiffWorkingReturnsTheSidesOfEachKindOfChange(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	base := strings.Repeat("line unchanged\n", 20)
	gittest.Commit(t, repo.Dir, "staged.txt", "head content\n", "add staged.txt")
	gittest.Commit(t, repo.Dir, "unstaged.txt", "index content\n", "add unstaged.txt")
	gittest.Commit(t, repo.Dir, "both.txt", "orig\n", "add both.txt")
	gittest.Commit(t, repo.Dir, "old.txt", base+"pre-rename\n", "add old.txt")
	gittest.Commit(t, repo.Dir, "bin.dat", "abc\x00def", "add binary file")
	writeFile(t, repo.Dir, "staged.txt", "staged content\n")
	writeFile(t, repo.Dir, "unstaged.txt", "on-disk content\n")
	writeFile(t, repo.Dir, "both.txt", "staged\n")
	gittest.Run(t, repo.Dir, "add", "--", "staged.txt", "both.txt")
	writeFile(t, repo.Dir, "both.txt", "staged\nmore\n")
	gittest.Run(t, repo.Dir, "mv", "old.txt", "new.txt")
	writeFile(t, repo.Dir, "new.txt", base+"post-rename\n")
	gittest.Run(t, repo.Dir, "add", "--", "new.txt")
	writeFile(t, repo.Dir, "bin.dat", "xyz\x00123")
	writeFile(t, repo.Dir, "untracked.dat", "abc\x00def")
	c, id := openWorktree(t, repo.Dir)

	tests := []struct {
		name, path, oldPath string
		staged, binary      bool
		wantOld, wantNew    string
	}{
		{name: "staged", path: "staged.txt", staged: true, wantOld: "head content\n", wantNew: "staged content\n"},
		{name: "staged rename", path: "new.txt", oldPath: "old.txt", staged: true, wantOld: base + "pre-rename\n", wantNew: base + "post-rename\n"},
		{name: "unstaged", path: "unstaged.txt", wantOld: "index content\n", wantNew: "on-disk content\n"},
		{name: "changed both ways, staged side", path: "both.txt", staged: true, wantOld: "orig\n", wantNew: "staged\n"},
		{name: "changed both ways, unstaged side", path: "both.txt", wantOld: "staged\n", wantNew: "staged\nmore\n"},
		{name: "unstaged binary", path: "bin.dat", binary: true},
		{name: "untracked binary", path: "untracked.dat", binary: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff, err := c.DiffWorking(t.Context(), id, tt.path, tt.oldPath, tt.staged, core.DiffOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if diff.Binary != tt.binary || diff.OldContent != tt.wantOld || diff.NewContent != tt.wantNew {
				t.Errorf("DiffWorking = %+v, want Binary %v OldContent %q NewContent %q", diff, tt.binary, tt.wantOld, tt.wantNew)
			}
		})
	}
}

func TestDiffWorkingUnstagedCapsHugeFilesWithAForceReloadBypass(t *testing.T) {
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

func TestDiffWorkingShowsAFileWhoseNameHoldsANewline(t *testing.T) {
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
	gittest.History(t, dir, gittest.Step{Message: "base", Files: map[string]string{"a.txt": "a\n", "[ab].txt": "ab\n"}})
	return dir
}

func TestStageFileAndUnstageFileTakeAGlobLikeNameLiterally(t *testing.T) {
	t.Parallel()
	dir := globRepo(t)
	writeFile(t, dir, "a.txt", "changed\n")
	writeFile(t, dir, "[ab].txt", "changed\n")
	c, id := openWorktree(t, dir)

	if err := c.StageFile(t.Context(), id, "[ab].txt"); err != nil {
		t.Fatal(err)
	}
	got := statusOf(t, c, id)
	if len(got.Staged) != 1 || got.Staged[0].Path != "[ab].txt" || len(got.Unstaged) != 1 || got.Unstaged[0].Path != "a.txt" {
		t.Errorf("after StageFile: Staged = %+v, Unstaged = %+v, want only [ab].txt staged", got.Staged, got.Unstaged)
	}

	gittest.Run(t, dir, "add", "-A")
	if err := c.UnstageFile(t.Context(), id, "[ab].txt"); err != nil {
		t.Fatal(err)
	}
	got = statusOf(t, c, id)
	if len(got.Staged) != 1 || got.Staged[0].Path != "a.txt" {
		t.Errorf("after UnstageFile: Staged = %+v, want only a.txt", got.Staged)
	}
}

func TestDiffWorkingOfAPatternNamedFileShowsItsOwnContent(t *testing.T) {
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

func TestDiffWorkingOfAPathLikeAStageNumberShowsItsOwnContent(t *testing.T) {
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

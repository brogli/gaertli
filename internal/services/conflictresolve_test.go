// Requirements: FR-CONF-07, FR-CONF-08

package services

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git/gittest"
)

// stageBytes is the index entry of path at stage; nil if there is none.
func stageBytes(t *testing.T, dir string, stage int, path string) []byte {
	t.Helper()
	return tryGit(t, dir, "show", ":"+strconv.Itoa(stage)+":"+path)
}

func unmergedList(t *testing.T, dir, path string) string {
	t.Helper()
	return gittest.Out(t, dir, "ls-files", "-u", "--", path)
}

func TestResolveConflictRefusesAFileNoLongerConflicted(t *testing.T) {
	t.Parallel()
	c, wt, dir := stoppedOn(t, "merge", "f", version("a\nbase\nz\n"), version("a\ncur\nz\n"), version("a\ninc\nz\n"))
	gittest.Run(t, dir, "add", "--", "f")
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("my edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := c.ResolveConflict(t.Context(), wt, "f", core.ChooseIncoming)

	requireGitFailedKey(t, err, "notConflicted")
	if got := readFile(t, dir, "f"); got != "my edit\n" {
		t.Errorf("working file = %q, want the edit kept", got)
	}
}

func TestReopenConflictRestoresASymlinkAsGitLeftIt(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("git on Windows does not create symlinks in the working tree")
	}
	dir := gittest.New(t).Dir
	link := func(target, msg string) {
		_ = os.Remove(filepath.Join(dir, "l"))
		if err := os.Symlink(target, filepath.Join(dir, "l")); err != nil {
			t.Fatal(err)
		}
		gittest.Run(t, dir, "add", "--", "l")
		gittest.CommitAll(t, dir, msg)
	}
	link("base", "base")
	gittest.Run(t, dir, "checkout", "-q", "-b", "side")
	link("incoming", "incoming")
	gittest.Run(t, dir, "checkout", "-q", "master")
	link("current", "current")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)
	if err := c.ResolveConflict(t.Context(), wt, "l", core.ChooseIncoming); err != nil {
		t.Fatal(err)
	}

	if err := c.ReopenConflict(t.Context(), wt, "l"); err != nil {
		t.Fatal(err)
	}

	if target, err := os.Readlink(filepath.Join(dir, "l")); err != nil || target != "current" {
		t.Errorf("link = %q, %v; want a symlink to current", target, err)
	}
	if got := unmergedList(t, dir, "l"); got == "" {
		t.Error("l is not unmerged again")
	}
}

func TestReopenConflictRefusesAFileStillConflicted(t *testing.T) {
	t.Parallel()
	c, wt, dir := stoppedOn(t, "merge", "f", version("base\n"), nil, version("incoming\n"))
	gittest.Run(t, dir, "rm", "-q", "--", "f")
	if err := c.ReopenConflict(t.Context(), wt, "f"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("my edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := c.ReopenConflict(t.Context(), wt, "f")

	requireGitFailedKey(t, err, "notReopenable")
	if got := readFile(t, dir, "f"); got != "my edit\n" {
		t.Errorf("working file = %q, want the edit kept", got)
	}
}

func TestReopenConflictRestoresAFileTheCurrentSideDeletedAsGitLeftIt(t *testing.T) {
	t.Parallel()
	c, wt, dir := stoppedOn(t, "merge", "a.txt", version("base\n"), nil, version("incoming\n"))
	gittest.Run(t, dir, "rm", "-q", "--", "a.txt")

	if err := c.ReopenConflict(t.Context(), wt, "a.txt"); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, dir, "a.txt"); got != "incoming\n" {
		t.Errorf("a.txt = %q, want the incoming content git left", got)
	}
	if got := gittest.Out(t, dir, "status", "--porcelain", "--", "a.txt"); got != "DU a.txt" {
		t.Errorf("status = %q, want DU a.txt", got)
	}
}

var stoppedOperations = []string{"merge", "rebase"}

func stoppedOn(t *testing.T, op, path string, base, current, incoming version) (*Core, core.WorktreeID, string) {
	t.Helper()
	dir := stoppedDir(t, op, path, base, current, incoming)
	c, id := openWorktree(t, dir)
	return c, id, dir
}

// stoppedDir is a repo stopped by op on the diverged path.
func stoppedDir(t *testing.T, op, path string, base, current, incoming version) string {
	t.Helper()
	dir := divergedRepo(t, path, base, current, incoming)
	tryGit(t, dir, op, "side")
	return dir
}

func TestResolveConflictTakesASideOrDeletes(t *testing.T) {
	t.Parallel()
	shapes := map[string][3]version{
		"binary UU": {version("a\x00b"), version("a\x00c"), version("a\x00d")},
		"DU":        {version("x\n"), nil, version("y\n")},
		"UD":        {version("x\n"), version("y\n"), nil},
	}
	for shape, v := range shapes {
		for _, op := range stoppedOperations {
			stopped := stoppedDir(t, op, "f.bin", v[0], v[1], v[2])
			for _, choice := range []core.ConflictChoice{core.ChooseCurrent, core.ChooseIncoming, core.ChooseDelete} {
				t.Run(shape+"/"+op+"/"+string(choice), func(t *testing.T) {
					t.Parallel()
					dir := gittest.Copy(t, stopped).Dir
					c, wt := openWorktree(t, dir)
					stages := map[int][]byte{2: stageBytes(t, dir, 2, "f.bin"), 3: stageBytes(t, dir, 3, "f.bin")}
					want := map[core.ConflictChoice][]byte{core.ChooseCurrent: stages[2], core.ChooseIncoming: stages[3]}[choice]

					err := c.ResolveConflict(t.Context(), wt, "f.bin", choice)

					if choice != core.ChooseDelete && want == nil {
						_ = coretest.AsError(t, err, core.GitFailed) // a side without a version
						if unmergedList(t, dir, "f.bin") == "" {
							t.Error("the conflict is gone, want it kept")
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					if unmergedList(t, dir, "f.bin") != "" {
						t.Error("still unmerged")
					}
					if choice == core.ChooseDelete {
						if _, err := os.Stat(filepath.Join(dir, "f.bin")); !os.IsNotExist(err) {
							t.Errorf("file still exists: %v", err)
						}
						if listed := gittest.Out(t, dir, "ls-files", "--", "f.bin"); listed != "" {
							t.Errorf("index still lists %q", listed)
						}
						return
					}
					if got, _ := os.ReadFile(filepath.Join(dir, "f.bin")); !bytes.Equal(got, want) {
						t.Errorf("working file = %q, want %q", got, want)
					}
					if staged := tryGit(t, dir, "show", ":f.bin"); !bytes.Equal(staged, want) {
						t.Errorf("staged = %q, want %q", staged, want)
					}
				})
			}
		}
	}
}

func TestResolveConflictRefusesAnUnknownChoice(t *testing.T) {
	t.Parallel()
	c, wt, dir := stoppedOn(t, "merge", "a.txt", version("a\nbase\n"), version("a\ncur\n"), version("a\ninc\n"))

	err := c.ResolveConflict(t.Context(), wt, "a.txt", core.ConflictChoice("both"))

	_ = coretest.AsError(t, err, core.Internal)
	if unmergedList(t, dir, "a.txt") == "" {
		t.Error("the conflict is gone")
	}
}

// twoConflictsStopped stops a merge on t.txt (UU) and d.txt (DU).
func twoConflictsStopped(t *testing.T) (*Core, core.WorktreeID, string) {
	t.Helper()
	dir := gittest.New(t).Dir
	writeFile(t, dir, "t.txt", "base\n")
	writeFile(t, dir, "d.txt", "x\n")
	gittest.Run(t, dir, "add", "--", "t.txt", "d.txt")
	gittest.CommitAll(t, dir, "base")
	gittest.Run(t, dir, "checkout", "-q", "-b", "side")
	writeFile(t, dir, "t.txt", "inc\n")
	writeFile(t, dir, "d.txt", "y\n")
	gittest.Run(t, dir, "add", "--", "t.txt", "d.txt")
	gittest.CommitAll(t, dir, "incoming")
	gittest.Run(t, dir, "checkout", "-q", "master")
	writeFile(t, dir, "t.txt", "cur\n")
	gittest.Run(t, dir, "add", "--", "t.txt")
	gittest.Run(t, dir, "rm", "-q", "--", "d.txt")
	gittest.CommitAll(t, dir, "current")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)
	return c, wt, dir
}

func TestStatusListsResolvedFilesAndReopenConflictRestoresThem(t *testing.T) {
	t.Parallel()
	c, wt, dir := twoConflictsStopped(t)
	writeFile(t, dir, "t.txt", "resolved\n")
	gittest.Run(t, dir, "add", "--", "t.txt")
	gittest.Run(t, dir, "rm", "-q", "--", "d.txt")

	status := statusOf(t, c, wt)
	if want := []string{"d.txt", "t.txt"}; !slices.Equal(slices.Sorted(slices.Values(status.Resolved)), want) {
		t.Fatalf("Resolved = %v, want %v", status.Resolved, want)
	}
	if len(status.Conflicted) != 0 {
		t.Fatalf("Conflicted = %v, want none", status.Conflicted)
	}

	for _, path := range []string{"t.txt", "d.txt"} {
		if err := c.ReopenConflict(t.Context(), wt, path); err != nil {
			t.Fatalf("ReopenConflict(%s) = %v", path, err)
		}
	}

	if got := unmergedList(t, dir, "t.txt"); strings.Count(got, "\n") != 2 {
		t.Errorf("t.txt stages = %q, want 1, 2 and 3", got)
	}
	if work := readFile(t, dir, "t.txt"); !strings.Contains(work, "<<<<<<<") {
		t.Errorf("t.txt = %q, want git's markers", work)
	}
	if got := unmergedList(t, dir, "d.txt"); strings.Count(got, "\n") != 1 || !strings.Contains(got, " 1\t") || !strings.Contains(got, " 3\t") {
		t.Errorf("d.txt stages = %q, want 1 and 3", got)
	}
	status = statusOf(t, c, wt)
	if len(status.Conflicted) != 2 || len(status.Resolved) != 0 {
		t.Errorf("Conflicted = %v, Resolved = %v, want both conflicted, none resolved", status.Conflicted, status.Resolved)
	}
}

func TestAConcludedMergeLeavesNothingToReopen(t *testing.T) {
	t.Parallel()
	c, wt, dir := twoConflictsStopped(t)
	writeFile(t, dir, "t.txt", "resolved\n")
	gittest.Run(t, dir, "add", "--", "t.txt")
	gittest.Run(t, dir, "rm", "-q", "--", "d.txt")
	if err := c.ContinueOperation(t.Context(), wt); err != nil {
		t.Fatal(err)
	}
	head := gittest.HeadSHA(t, dir)

	if status := statusOf(t, c, wt); len(status.Resolved) != 0 {
		t.Errorf("Resolved = %v, want none after the merge concluded", status.Resolved)
	}
	err := c.ReopenConflict(t.Context(), wt, "t.txt")

	requireGitFailedKey(t, err, "noOperation")
	if unmerged := gittest.Out(t, dir, "ls-files", "-u"); unmerged != "" {
		t.Errorf("unmerged = %q, want none", unmerged)
	}
	if work := readFile(t, dir, "t.txt"); work != "resolved\n" || gittest.HeadSHA(t, dir) != head {
		t.Errorf("t.txt = %q, HEAD changed = %v, want both untouched", work, gittest.HeadSHA(t, dir) != head)
	}
}

func TestReopenConflictRefusesAFileThatNeverConflicted(t *testing.T) {
	t.Parallel()
	c, wt, dir := twoConflictsStopped(t)
	writeFile(t, dir, "new.txt", "new\n")
	gittest.Run(t, dir, "add", "--", "new.txt")

	err := c.ReopenConflict(t.Context(), wt, "new.txt")

	requireGitFailedKey(t, err, "notReopenable")
	if staged := indexBlob(t, dir, "new.txt"); staged != "new" {
		t.Errorf("staged = %q, want it untouched", staged)
	}
}

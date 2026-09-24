package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git/gittest"
)

// pushArgvWriter installs an editor profile whose folder/file commands are
// argvWriterBin (built in TestMain, log_test.go), and returns the file it
// writes its received argv to, removed on cleanup.
func pushArgvWriter(t *testing.T, c *Core) string {
	t.Helper()
	profile := core.EditorProfile{FolderCmd: argvWriterBin + " {path}", FileCmd: argvWriterBin + " {path}"}
	if err := c.SetEditorProfile(t.Context(), profile); err != nil {
		t.Fatal(err)
	}
	outFile := argvWriterBin + ".out"
	_ = os.Remove(outFile)
	t.Cleanup(func() { _ = os.Remove(outFile) })
	return outFile
}

// waitForFile polls for outFile to appear (the helper runs detached) and
// returns its content, failing t if it never shows up.
func waitForFile(t *testing.T, outFile string) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if content, err := os.ReadFile(outFile); err == nil {
			return string(content)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("helper never wrote its argv file")
	return ""
}

// assertNoEditorStarted fails t if outFile exists: the helper never ran.
func assertNoEditorStarted(t *testing.T, outFile string) {
	t.Helper()
	if _, err := os.Stat(outFile); !os.IsNotExist(err) {
		t.Error("editor process started, want none")
	}
}

func TestFR_WT_04_OpenFolderAtStartsTheEditorOnTheWorktreesToplevelWithNoPriorSession(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	linkedDir := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", linkedDir, "feature")

	c := New(func(core.Event) {})
	outFile := pushArgvWriter(t, c)

	if err := c.OpenFolderAt(t.Context(), core.RepoPath(linkedDir)); err != nil {
		t.Fatal(err)
	}

	if got := waitForFile(t, outFile); got != linkedDir {
		t.Errorf("helper argv = %q, want %q", got, linkedDir)
	}
}

func TestFR_WT_04_OpenFolderAtRejectsANonRepoPathStartingNoEditor(t *testing.T) {
	c := New(func(core.Event) {})
	outFile := pushArgvWriter(t, c)

	err := c.OpenFolderAt(t.Context(), core.RepoPath(t.TempDir()))

	coretest.AssertCode(t, err, core.NotARepo)
	assertNoEditorStarted(t, outFile)
}

func TestFR_ED_02_OpenFileJoinsThePathAgainstTheWorktreeToplevel(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	if err := os.MkdirAll(filepath.Join(repo.Dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "sub", "foo.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := New(func(core.Event) {})
	outFile := pushArgvWriter(t, c)
	wt := openOn(t, c, repo.Dir)

	if err := c.OpenFile(t.Context(), wt.ID, "sub/foo.txt", 0); err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(repo.Dir, "sub", "foo.txt")
	if got := waitForFile(t, outFile); got != want {
		t.Errorf("helper argv = %q, want %q", got, want)
	}
}

func TestFR_ED_02_OpenFileReturnsFileNotFoundForAMissingPathStartingNoEditor(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")

	c := New(func(core.Event) {})
	outFile := pushArgvWriter(t, c)
	wt := openOn(t, c, repo.Dir)

	err := c.OpenFile(t.Context(), wt.ID, "gone.txt", 0)

	coretest.AssertCode(t, err, core.FileNotFound)
	assertNoEditorStarted(t, outFile)
}

func TestFR_UX_11_OpenFileNamesTheMissingPathInArgs(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")

	c := New(func(core.Event) {})
	pushArgvWriter(t, c)
	wt := openOn(t, c, repo.Dir)

	err := c.OpenFile(t.Context(), wt.ID, "gone.txt", 0)

	ce := coretest.AsError(t, err, core.FileNotFound)
	if ce.Args["path"] != "gone.txt" {
		t.Errorf("Args[path] = %q, want %q", ce.Args["path"], "gone.txt")
	}
	if ce.Key != "fileNotFound" {
		t.Errorf("Key = %q, want fileNotFound", ce.Key)
	}
}

func TestFR_ED_02_OpenFileRejectsAPathEscapingTheWorktreeStartingNoEditor(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")

	c := New(func(core.Event) {})
	outFile := pushArgvWriter(t, c)
	wt := openOn(t, c, repo.Dir)

	err := c.OpenFile(t.Context(), wt.ID, "../../etc/passwd", 0)

	coretest.AssertCode(t, err, core.Internal)
	assertNoEditorStarted(t, outFile)
}

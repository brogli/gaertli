package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
)

// pushArgvWriter installs an editor profile whose folder/file commands are
// argvWriterBin (built in TestMain, log_test.go), and returns the file it
// writes its received argv to, removed on cleanup.
func pushArgvWriter(t *testing.T, c *Core) string {
	t.Helper()
	profile := core.EditorProfile{FolderCmd: argvWriterBin + " {path}", FileCmd: argvWriterBin + " {path}"}
	if err := c.SetEditorProfile(context.Background(), profile); err != nil {
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

	if err := c.OpenFolderAt(context.Background(), core.RepoPath(linkedDir)); err != nil {
		t.Fatal(err)
	}

	if got := waitForFile(t, outFile); got != linkedDir {
		t.Errorf("helper argv = %q, want %q", got, linkedDir)
	}
}

func TestFR_WT_04_OpenFolderAtRejectsANonRepoPathStartingNoEditor(t *testing.T) {
	c := New(func(core.Event) {})
	outFile := pushArgvWriter(t, c)

	err := c.OpenFolderAt(context.Background(), core.RepoPath(t.TempDir()))

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
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	if err := c.OpenFile(context.Background(), wt.ID, "sub/foo.txt", 0); err != nil {
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
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	err = c.OpenFile(context.Background(), wt.ID, "gone.txt", 0)

	coretest.AssertCode(t, err, core.FileNotFound)
	assertNoEditorStarted(t, outFile)
}

// unsetEnv removes key from the environment for t's duration: t.Setenv sets
// an empty value, which git's own env lookups still treat as "set" — not
// the same as unset.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	old, existed := os.LookupEnv(key)
	_ = os.Unsetenv(key)
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, old)
		}
	})
}

func TestFR_ED_10_GitEditorProfileResolvesGitVarGitEditor(t *testing.T) {
	repo := gittest.New(t)
	// GIT_EDITOR/VISUAL/EDITOR each outrank core.editor in git's own
	// precedence (git var GIT_EDITOR): cleared so this test proves the
	// config is read, regardless of the ambient shell.
	unsetEnv(t, "GIT_EDITOR")
	unsetEnv(t, "VISUAL")
	unsetEnv(t, "EDITOR")
	gittest.Run(t, repo.Dir, "config", "core.editor", argvWriterBin)

	profile, err := gitEditorProfile(context.Background(), git.Runner{}, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}

	want := argvWriterBin + " {path}"
	if profile.FolderCmd != want || profile.FileCmd != want {
		t.Errorf("gitEditorProfile FolderCmd/FileCmd = %q/%q, want %q", profile.FolderCmd, profile.FileCmd, want)
	}
}

func TestFR_ED_10_OpenFolderFallbackUsesTheGitConfiguredEditor(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	t.Setenv("GIT_EDITOR", argvWriterBin)
	outFile := argvWriterBin + ".out"
	_ = os.Remove(outFile)
	t.Cleanup(func() { _ = os.Remove(outFile) })

	c, wt := openWorktree(t, repo.Dir)

	if err := c.OpenFolderFallback(context.Background(), wt, core.FallbackGitEditor); err != nil {
		t.Fatal(err)
	}

	if got := waitForFile(t, outFile); got != repo.Dir {
		t.Errorf("helper argv = %q, want %q", got, repo.Dir)
	}
}

// TestFR_ED_10_GitEditorProfileSplitsAnEditorCommandWithItsOwnArguments pins
// a real-world core.editor/GIT_EDITOR value with its own flag (e.g. "code
// --wait"), not just a bare path: gitEditorProfile appends " {path}" to
// whatever git var GIT_EDITOR returns verbatim, so splitCommand (run later,
// inside editor.OpenFolder) must still split the flag from the editor
// binary and from the path.
func TestFR_ED_10_GitEditorProfileSplitsAnEditorCommandWithItsOwnArguments(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	t.Setenv("GIT_EDITOR", argvWriterBin+" --wait")
	outFile := argvWriterBin + ".out"
	_ = os.Remove(outFile)
	t.Cleanup(func() { _ = os.Remove(outFile) })

	c, wt := openWorktree(t, repo.Dir)

	if err := c.OpenFolderFallback(context.Background(), wt, core.FallbackGitEditor); err != nil {
		t.Fatal(err)
	}

	want := "--wait\n" + repo.Dir
	if got := waitForFile(t, outFile); got != want {
		t.Errorf("helper argv = %q, want %q", got, want)
	}
}

// TestFR_ED_10_GitEditorProfileSplitsAQuotedEditorPathWithItsOwnArguments
// pins a Windows-style quoted-path-plus-flags value (e.g. `"C:\Program
// Files\...\notepad++.exe" -multiInst`): the quoted token must survive as
// one argv element, its embedded backslashes untouched, with the trailing
// flag split out separately.
func TestFR_ED_10_GitEditorProfileSplitsAQuotedEditorPathWithItsOwnArguments(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	t.Setenv("GIT_EDITOR", `"`+argvWriterBin+`" -multiInst`)
	outFile := argvWriterBin + ".out"
	_ = os.Remove(outFile)
	t.Cleanup(func() { _ = os.Remove(outFile) })

	c, wt := openWorktree(t, repo.Dir)

	if err := c.OpenFolderFallback(context.Background(), wt, core.FallbackGitEditor); err != nil {
		t.Fatal(err)
	}

	want := "-multiInst\n" + repo.Dir
	if got := waitForFile(t, outFile); got != want {
		t.Errorf("helper argv = %q, want %q", got, want)
	}
}

func TestFR_ED_10_OpenFileFallbackUsesTheGitConfiguredEditor(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	t.Setenv("GIT_EDITOR", argvWriterBin)
	outFile := argvWriterBin + ".out"
	_ = os.Remove(outFile)
	t.Cleanup(func() { _ = os.Remove(outFile) })

	c, wt := openWorktree(t, repo.Dir)

	if err := c.OpenFileFallback(context.Background(), wt, "seed.txt", 0, core.FallbackGitEditor); err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(repo.Dir, "seed.txt")
	if got := waitForFile(t, outFile); got != want {
		t.Errorf("helper argv = %q, want %q", got, want)
	}
}

func TestFR_ED_02_OpenFileRejectsAPathEscapingTheWorktreeStartingNoEditor(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")

	c := New(func(core.Event) {})
	outFile := pushArgvWriter(t, c)
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	err = c.OpenFile(context.Background(), wt.ID, "../../etc/passwd", 0)

	coretest.AssertCode(t, err, core.Internal)
	assertNoEditorStarted(t, outFile)
}

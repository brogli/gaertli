// Requirements: FR-ED-02, FR-UX-02, FR-WT-04

package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/core/coretest"
	"github.com/brogli/gaertli/internal/git/gittest"
	"github.com/brogli/gaertli/internal/testwait"
)

// onlyEditorRecord finds the one "editor" event among events and decodes
// its payload, failing t if there is none.
func onlyEditorRecord(t *testing.T, events []core.Event) core.EditorRecord {
	t.Helper()
	for _, ev := range events {
		if ev.Name != "editor" {
			continue
		}
		var record core.EditorRecord
		if err := json.Unmarshal(ev.Payload, &record); err != nil {
			t.Fatal(err)
		}
		return record
	}
	t.Fatal("no editor event sunk")
	return core.EditorRecord{}
}

// pushArgvWriter installs an editor profile whose folder/file commands are
// argvWriterBin (built in TestMain, log_test.go), and returns the file it
// writes its received argv to.
func pushArgvWriter(t *testing.T, c *Core) string {
	t.Helper()
	outFile := filepath.Join(t.TempDir(), "argv")
	cmd := argvWriterBin + " " + outFile + " {path}"
	if err := c.SetEditorProfile(t.Context(), core.EditorProfile{FolderCmd: cmd, FileCmd: cmd}); err != nil {
		t.Fatal(err)
	}
	return outFile
}

// waitForFile polls for outFile to appear (the helper runs detached) and
// returns its content, failing t if it never shows up.
func waitForFile(t *testing.T, outFile string) string {
	t.Helper()
	var content []byte
	testwait.Until(func() bool {
		var err error
		content, err = os.ReadFile(outFile)
		return err == nil
	})
	return string(content)
}

// assertNoEditorStarted fails t if outFile exists: the helper never ran.
func assertNoEditorStarted(t *testing.T, outFile string) {
	t.Helper()
	if _, err := os.Stat(outFile); !os.IsNotExist(err) {
		t.Error("editor process started, want none")
	}
}

func TestOpenFolderAtStartsTheEditorOnTheWorktreesToplevelWithNoPriorSession(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")
	gittest.Run(t, repo.Dir, "branch", "feature")
	linkedDir := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", linkedDir, "feature")
	linkedPath := gitPath(t, linkedDir)

	c := New(func(core.Event) {})
	outFile := pushArgvWriter(t, c)

	if err := c.OpenFolderAt(t.Context(), core.RepoPath(linkedDir)); err != nil {
		t.Fatal(err)
	}

	if got := waitForFile(t, outFile); got != linkedPath {
		t.Errorf("helper argv = %q, want %q", got, linkedPath)
	}
}

func TestOpenFolderAtRejectsANonRepoPathStartingNoEditor(t *testing.T) {
	t.Parallel()
	c := New(func(core.Event) {})
	outFile := pushArgvWriter(t, c)

	err := c.OpenFolderAt(t.Context(), core.RepoPath(t.TempDir()))

	coretest.AssertCode(t, err, core.NotARepo)
	assertNoEditorStarted(t, outFile)
}

func TestOpenFileJoinsThePathAgainstTheToplevelLogsTheStartAndRefusesMissingOrEscapingPaths(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	if err := os.MkdirAll(filepath.Join(repo.Dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, repo.Dir, "sub/foo.txt", "x")

	var events []core.Event
	c := New(func(ev core.Event) { events = append(events, ev) })
	outFile := pushArgvWriter(t, c)
	wt := openOn(t, c, repo.Dir)

	if err := c.OpenFile(t.Context(), wt.ID, "sub/foo.txt", 0); err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(gitPath(t, repo.Dir), "sub", "foo.txt")
	if got := waitForFile(t, outFile); got != want {
		t.Errorf("helper argv = %q, want %q", got, want)
	}
	record := onlyEditorRecord(t, events)
	if wantArgv := []string{argvWriterBin, outFile, want}; !slices.Equal(record.Argv, wantArgv) {
		t.Errorf("logged argv = %v, want %v", record.Argv, wantArgv)
	}
	if record.Error != "" {
		t.Errorf("logged error = %q, want none", record.Error)
	}

	outFile = pushArgvWriter(t, c)
	err := c.OpenFile(t.Context(), wt.ID, "gone.txt", 0)
	ce := coretest.AsError(t, err, core.FileNotFound)
	if ce.Args["path"] != "gone.txt" {
		t.Errorf("Args[path] = %q, want %q", ce.Args["path"], "gone.txt")
	}
	if ce.Key != "fileNotFound" {
		t.Errorf("Key = %q, want fileNotFound", ce.Key)
	}
	coretest.AssertCode(t, c.OpenFile(t.Context(), wt.ID, "../../etc/passwd", 0), core.Internal)
	assertNoEditorStarted(t, outFile)
}

func TestOpenFileWithACommandNotOnPathLogsTheArgvAndTheError(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "a", "first")

	var events []core.Event
	c := New(func(ev core.Event) { events = append(events, ev) })
	profile := core.EditorProfile{FolderCmd: "definitely-not-a-real-editor {path}", FileCmd: "definitely-not-a-real-editor {path}"}
	if err := c.SetEditorProfile(t.Context(), profile); err != nil {
		t.Fatal(err)
	}
	wt := openOn(t, c, repo.Dir)
	repoPath := gitPath(t, repo.Dir) // before PATH is wiped: gitPath itself needs to run git
	t.Setenv("PATH", t.TempDir())    // empty: nothing resolves, set after opening (which itself needs git)

	err := c.OpenFile(t.Context(), wt.ID, "seed.txt", 0)
	coretest.AssertCode(t, err, core.EditorNotFound)

	record := onlyEditorRecord(t, events)
	if want := []string{"definitely-not-a-real-editor", filepath.Join(repoPath, "seed.txt")}; !slices.Equal(record.Argv, want) {
		t.Errorf("argv = %v, want %v", record.Argv, want)
	}
	if record.Error == "" {
		t.Error("error = \"\", want the start failure")
	}
}

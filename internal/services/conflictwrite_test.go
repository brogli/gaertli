package services

import (
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git/gittest"
)

// requireGitFailedKey fails unless err is GitFailed with key.
func requireGitFailedKey(t *testing.T, err error, key string) {
	t.Helper()
	if ce := coretest.AsError(t, err, core.GitFailed); ce.Key != key {
		t.Fatalf("err = %v, want GitFailed key %s", err, key)
	}
}

func TestFR_CONF_06_SaveConflictWritesCRLFWhenTheFileUsesCRLF(t *testing.T) {
	t.Parallel()
	c, wt, dir := stoppedOn(t, "merge", "a.txt", version("a\r\nbase\r\nz\r\n"), version("a\r\ncur\r\nz\r\n"), version("a\r\ninc\r\nz\r\n"))
	got := readConflict(t, c, wt, "a.txt")
	if strings.Contains(got.Result, "\r") {
		t.Fatalf("Result = %q, want LF only", got.Result)
	}

	fp, err := c.SaveConflict(t.Context(), wt, "a.txt", "a\nmerged\nz\n", got.Fingerprint, true)
	if err != nil {
		t.Fatal(err)
	}

	const want = "a\r\nmerged\r\nz\r\n"
	if work := readFile(t, dir, "a.txt"); work != want {
		t.Errorf("working file = %q, want %q", work, want)
	}
	if fp != fingerprintOf([]byte(want)) {
		t.Errorf("fingerprint = %s, want the written bytes'", fp)
	}
	if staged := gittest.Out(t, dir, "show", ":a.txt"); staged != strings.TrimSpace(want) {
		t.Errorf("staged blob = %q, want CRLF %q", staged, want)
	}
	if stat := gittest.Out(t, dir, "diff", "--cached", "--numstat", "HEAD"); stat != "1\t1\ta.txt" {
		t.Errorf("diff --cached HEAD = %q, want only the conflict's line", stat)
	}
	if status := statusOf(t, c, wt); len(status.Conflicted) != 0 {
		t.Errorf("Conflicted = %v, want none", status.Conflicted)
	}
}

func TestFR_CONF_06_SaveConflictStagesLFBlobsUnderAutocrlf(t *testing.T) {
	t.Parallel()
	dir := divergedRepo(t, "a.txt", version("a\nbase\nz\n"), version("a\ncur\nz\n"), version("a\ninc\nz\n"))
	gittest.Run(t, dir, "config", "core.autocrlf", "true")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)
	got := readConflict(t, c, wt, "a.txt")

	if _, err := c.SaveConflict(t.Context(), wt, "a.txt", "a\nmerged\nz\n", got.Fingerprint, true); err != nil {
		t.Fatal(err)
	}

	if staged := gittest.Out(t, dir, "show", ":a.txt"); staged != "a\nmerged\nz" {
		t.Errorf("staged blob = %q, want LF", staged)
	}
}

func TestFR_CONF_06_SaveConflictOfAnUnchangedResultKeepsTheBOMFileBytes(t *testing.T) {
	t.Parallel()
	const bom = "\xef\xbb\xbf"
	c, wt, dir := stoppedOn(t, "merge", "a.txt", version(bom+"a\nbase\nz\n"), version(bom+"a\ncur\nz\n"), version(bom+"a\ninc\nz\n"))
	before := readFile(t, dir, "a.txt")
	got := readConflict(t, c, wt, "a.txt")

	if _, err := c.SaveConflict(t.Context(), wt, "a.txt", got.Result, got.Fingerprint, false); err != nil {
		t.Fatal(err)
	}

	if after := readFile(t, dir, "a.txt"); after != before {
		t.Errorf("working file = %q, want %q", after, before)
	}
}

func TestFR_CONF_06_SaveConflictRefusesAFileChangedSinceItWasOpened(t *testing.T) {
	t.Parallel()
	c, wt, dir := stoppedOn(t, "merge", "a.txt", version("a\nbase\n"), version("a\ncur\n"), version("a\ninc\n"))
	got := readConflict(t, c, wt, "a.txt")
	writeFile(t, dir, "a.txt", "edited elsewhere\n")

	_, err := c.SaveConflict(t.Context(), wt, "a.txt", "mine\n", got.Fingerprint, true)

	requireGitFailedKey(t, err, "fileChangedSinceOpened")
	if work := readFile(t, dir, "a.txt"); work != "edited elsewhere\n" {
		t.Errorf("working file = %q, want it untouched", work)
	}
}

func TestFR_CONF_06_SaveConflictRefusesToResolveWithMarkersLeft(t *testing.T) {
	t.Parallel()
	c, wt, dir := stoppedOn(t, "merge", "a.txt", version("a\nbase\n"), version("a\ncur\n"), version("a\ninc\n"))
	got := readConflict(t, c, wt, "a.txt")
	withMarkers := "a\n<<<<<<< HEAD\nmine\n"

	fp, err := c.SaveConflict(t.Context(), wt, "a.txt", withMarkers, got.Fingerprint, false)
	if err != nil {
		t.Fatalf("save without resolving = %v", err)
	}
	if work := readFile(t, dir, "a.txt"); work != withMarkers {
		t.Errorf("working file = %q, want %q", work, withMarkers)
	}
	if status := statusOf(t, c, wt); len(status.Conflicted) != 1 {
		t.Errorf("Conflicted = %v, want still one", status.Conflicted)
	}

	_, err = c.SaveConflict(t.Context(), wt, "a.txt", "a\n>>>>>>> side\n", fp, true)

	requireGitFailedKey(t, err, "conflictMarkersLeft")
	if work := readFile(t, dir, "a.txt"); work != withMarkers {
		t.Errorf("working file = %q, want it untouched", work)
	}
}

func TestFR_CONF_06_SaveConflictIgnoresLinesThatOnlyStartLikeMarkers(t *testing.T) {
	t.Parallel()
	c, wt, _ := stoppedOn(t, "merge", "a.txt", version("a\nbase\n"), version("a\ncur\n"), version("a\ninc\n"))
	got := readConflict(t, c, wt, "a.txt")

	_, err := c.SaveConflict(t.Context(), wt, "a.txt", "<<<<<<<<\n<<<<<<<x\n<<<<<\n", got.Fingerprint, true)
	if err != nil {
		t.Errorf("err = %v, want nil: none is a marker line of size 7", err)
	}
}

func TestFR_CONF_06_SaveConflictFollowsTheMarkerSizeAttribute(t *testing.T) {
	t.Parallel()
	dir := divergedRepo(t, "a.txt", version("a\nbase\n"), version("a\ncur\n"), version("a\ninc\n"))
	writeFile(t, dir, ".gitattributes", "a.txt conflict-marker-size=10\n")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)
	got := readConflict(t, c, wt, "a.txt")

	if _, err := c.SaveConflict(t.Context(), wt, "a.txt", "<<<<<<<\n", got.Fingerprint, true); err != nil {
		t.Fatalf("a size-7 line is no marker here: %v", err)
	}
}

func TestFR_CONF_06_SaveConflictWithTheAttributeSizeMarkerIsRefused(t *testing.T) {
	t.Parallel()
	dir := divergedRepo(t, "a.txt", version("a\nbase\n"), version("a\ncur\n"), version("a\ninc\n"))
	writeFile(t, dir, ".gitattributes", "a.txt conflict-marker-size=10\n")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)
	got := readConflict(t, c, wt, "a.txt")

	_, err := c.SaveConflict(t.Context(), wt, "a.txt", "<<<<<<<<<< x\n", got.Fingerprint, true)

	requireGitFailedKey(t, err, "conflictMarkersLeft")
}

func TestFR_CONF_06_SaveConflictKeepsTheLineEndsOfAMixedFile(t *testing.T) {
	t.Parallel()
	dir := gittest.New(t).Dir
	gittest.Run(t, dir, "config", "core.autocrlf", "false")
	commitVersion(t, dir, "a.txt", version("l1\r\nl2\nb\nl4\n"), "base")
	gittest.Run(t, dir, "checkout", "-q", "-b", "side")
	commitVersion(t, dir, "a.txt", version("l1\r\nl2\ninc\nl4\n"), "incoming change")
	gittest.Run(t, dir, "checkout", "-q", "master")
	commitVersion(t, dir, "a.txt", version("l1\r\nl2\ncur\nl4\n"), "current change")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)
	got := readConflict(t, c, wt, "a.txt")
	if !strings.HasPrefix(got.Result, "l1\r\nl2\n") {
		t.Fatalf("Result = %q, want the CR of line 1 kept", got.Result)
	}
	start, end := strings.Index(got.Result, "<<<<<<<"), strings.Index(got.Result, ">>>>>>> side\n")+len(">>>>>>> side\n")
	text := got.Result[:start] + "RES\n" + got.Result[end:]

	if _, err := c.SaveConflict(t.Context(), wt, "a.txt", text, got.Fingerprint, true); err != nil {
		t.Fatal(err)
	}

	if staged := gittest.Out(t, dir, "show", ":a.txt"); staged != "l1\r\nl2\nRES\nl4" {
		t.Errorf("staged blob = %q, want only the conflict's line changed", staged)
	}
	if stat := gittest.Out(t, dir, "diff", "--cached", "--numstat", "HEAD"); stat != "1\t1\ta.txt" {
		t.Errorf("diff --cached HEAD = %q, want only the conflict's line", stat)
	}
}

func TestFR_CONF_06_ConflictFindsTheMarkersOfAMixedLineEndingFile(t *testing.T) {
	t.Parallel()
	dir := gittest.New(t).Dir
	gittest.Run(t, dir, "config", "core.autocrlf", "false")
	commitVersion(t, dir, "a.txt", version("one\r\ntwo\nthree\r\nfour\n"), "base")
	gittest.Run(t, dir, "checkout", "-q", "-b", "side")
	commitVersion(t, dir, "a.txt", version("one\r\ninc\nthree\r\nfour\n"), "incoming change")
	gittest.Run(t, dir, "checkout", "-q", "master")
	commitVersion(t, dir, "a.txt", version("one\r\ncur\nthree\r\nfour\n"), "current change")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)

	got := readConflict(t, c, wt, "a.txt")

	assertRegions(t, got, wantRegion{"two\n", "cur\n", "inc\n"})
	_, err := c.SaveConflict(t.Context(), wt, "a.txt", "one\r\n>>>>>>> side\r\n", got.Fingerprint, true)
	requireGitFailedKey(t, err, "conflictMarkersLeft")
}

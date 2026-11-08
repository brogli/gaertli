// Requirements: FR-ED-03, FR-REPO-01, FR-REPO-03, FR-REPO-04, FR-REPO-05, QA-REL-02

package services

import (
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/core/coretest"
	"github.com/brogli/gaertli/internal/git"
	"github.com/brogli/gaertli/internal/git/gittest"
)

// commandRecorder collects the git calls made through one worktree's
// session.
type commandRecorder struct {
	mu      sync.Mutex
	entries []git.Entry
}

func (r *commandRecorder) record(e git.Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
}

func (r *commandRecorder) snapshot() []git.Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.entries)
}

// recordCommands wraps wt's session Runner.Log to also feed a
// commandRecorder, on top of whatever hook session wiring already set (the
// sink).
func recordCommands(t *testing.T, c *Core, wt core.WorktreeID) *commandRecorder {
	t.Helper()
	w, err := c.sessions.Get(wt)
	if err != nil {
		t.Fatal(err)
	}
	rec := &commandRecorder{}
	prev := w.Runner.Log
	w.Runner.Log = func(e git.Entry) {
		rec.record(e)
		if prev != nil {
			prev(e)
		}
	}
	return rec
}

func TestOpenWorktreeReusesSessionForSameToplevel(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	sub := filepath.Join(repo.Dir, "sub", "dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	c := New(func(core.Event) {})

	first := openOn(t, c, repo.Dir)
	second, err := c.OpenWorktree(t.Context(), core.RepoPath(sub))
	if err != nil {
		t.Fatal(err)
	}

	if first.ID != second.ID {
		t.Errorf("ID = %q, %q, want equal", first.ID, second.ID)
	}
}

func TestOpenWorktreeGivesLinkedWorktreesDistinctIDsSameRepoKey(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", "-b", "feature", linked)
	c := New(func(core.Event) {})

	main := openOn(t, c, repo.Dir)
	other := openOn(t, c, linked)

	if main.ID == other.ID {
		t.Errorf("linked worktrees got the same ID %q", main.ID)
	}
	if main.RepoKey != other.RepoKey {
		t.Errorf("RepoKey = %q, %q, want equal", main.RepoKey, other.RepoKey)
	}
}

func TestOpenWorktreeRejectsPlainDirectory(t *testing.T) {
	t.Parallel()
	c := New(func(core.Event) {})

	_, err := c.OpenWorktree(t.Context(), core.RepoPath(t.TempDir()))

	coretest.AssertCode(t, err, core.NotARepo)
}

func TestOpenWorktreeRejectsMissingPath(t *testing.T) {
	t.Parallel()
	c := New(func(core.Event) {})

	_, err := c.OpenWorktree(t.Context(), core.RepoPath(filepath.Join(t.TempDir(), "missing")))

	coretest.AssertCode(t, err, core.NotARepo)
}

func TestSummaryReportsBranch(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	gittest.Run(t, repo.Dir, "switch", "-c", "feature")
	c, id := openWorktree(t, repo.Dir)

	got, err := c.Summary(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}

	if got.Branch != "feature" || got.Detached || got.Operation != "" {
		t.Errorf("Summary = %+v, want Branch feature, Detached false, Operation \"\"", got)
	}
}

func TestSummaryReportsTheBranchByItsNameWhenATagSharesIt(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	gittest.Run(t, repo.Dir, "switch", "-c", "v1")
	gittest.Run(t, repo.Dir, "tag", "v1")
	c, id := openWorktree(t, repo.Dir)

	got, err := c.Summary(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}

	if got.Branch != "v1" {
		t.Errorf("Branch = %q, want v1", got.Branch)
	}
}

func TestSummaryReportsTheEffectiveEditorProfileName(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	c, id := openWorktree(t, repo.Dir)

	before, err := c.Summary(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	if before.EditorName != "" {
		t.Errorf("EditorName before any push = %q, want empty: there is no built-in editor", before.EditorName)
	}

	if err := c.SetEditorProfile(t.Context(), core.EditorProfile{Name: "Beta", FolderCmd: "beta {path}"}); err != nil {
		t.Fatal(err)
	}

	after, err := c.Summary(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	if after.EditorName != "Beta" {
		t.Errorf("EditorName after push = %q, want %q", after.EditorName, "Beta")
	}
}

func TestSummaryReportsMergeInProgress(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", "--detach", linked)
	gitDir := gittest.Out(t, linked, "rev-parse", "--absolute-git-dir")
	if err := os.WriteFile(filepath.Join(gitDir, "MERGE_HEAD"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	c, id := openWorktree(t, linked)

	got, err := c.Summary(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Operation != "merge" {
		t.Errorf("Operation = %q, want %q", got.Operation, "merge")
	}
}

func TestSummaryReportsRebaseInProgress(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	if err := os.Mkdir(filepath.Join(repo.Dir, ".git", "rebase-merge"), 0o755); err != nil {
		t.Fatal(err)
	}
	c, id := openWorktree(t, repo.Dir)

	got, err := c.Summary(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Operation != "rebase" {
		t.Errorf("Operation = %q, want %q", got.Operation, "rebase")
	}
}

func TestSummaryReportsHeadSHA(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	gittest.Commit(t, repo.Dir, "seed.txt", "y", "second")
	sha := gittest.HeadSHA(t, repo.Dir)
	c, id := openWorktree(t, repo.Dir)

	got, err := c.Summary(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}

	if got.SHA != sha {
		t.Errorf("SHA = %q, want %q", got.SHA, sha)
	}
}

func TestSummaryReportsEmptySHAOnAnUnbornHEAD(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	c, id := openWorktree(t, repo.Dir)

	got, err := c.Summary(t.Context(), id)
	if err != nil {
		t.Fatalf("Summary on an empty repo: %v, want no error", err)
	}
	if got.SHA != "" {
		t.Errorf("SHA = %q, want empty for an unborn HEAD", got.SHA)
	}
}

func TestSummaryReportsDetachedHEADWithItsSHA(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	sha := gittest.HeadSHA(t, repo.Dir)
	c, id := openWorktree(t, repo.Dir)
	if err := c.CheckoutCommit(t.Context(), id, sha); err != nil {
		t.Fatal(err)
	}

	got, err := c.Summary(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}

	if !got.Detached || got.Branch != "" {
		t.Errorf("Summary = %+v, want Detached true, Branch \"\"", got)
	}
	if got.SHA != sha {
		t.Errorf("SHA = %q, want %q", got.SHA, sha)
	}
}

func TestSetFocusedTrueRunsAnImmediatePollerCheck(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	events := make(chan core.Event, 16)
	c := New(func(e core.Event) { events <- e })
	wt := openOn(t, c, repo.Dir)
	if err := c.SetFocused(t.Context(), wt.ID, false); err != nil {
		t.Fatal(err)
	}
	coretest.DrainEvents(events)

	if err := c.SetFocused(t.Context(), wt.ID, true); err != nil {
		t.Fatal(err)
	}

	if got := countCommandEvents(events); got != 1 {
		t.Fatalf("command events = %d, want 1 (an immediate poll)", got)
	}
}

// countCommandEvents drains events, without waiting, counting how many are
// "command" events. It covers a poller's own git calls, which run through a
// frozen copy of the worktree's Runner that recordCommands cannot intercept.
func countCommandEvents(events chan core.Event) int {
	n := 0
	for {
		select {
		case e := <-events:
			if e.Name == "command" {
				n++
			}
		default:
			return n
		}
	}
}

func TestRefreshRunsTheWatcherAndPollerChecksPromptly(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	events := make(chan core.Event, 16)
	c := New(func(e core.Event) { events <- e })
	wt := openOn(t, c, repo.Dir)
	coretest.DrainEvents(events)

	// an external commit just before Refresh: the watcher's baseline
	// predates it, so only a fresh CheckNow notices it without waiting for
	// the fallback ticker's own interval.
	gittest.Commit(t, repo.Dir, "seed.txt", "y", "second")

	start := time.Now()
	if err := c.Refresh(t.Context(), wt.ID); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("Refresh took %v, want a prompt return", elapsed)
	}

	var sawCommand, sawRefsChanged bool
	deadline := time.After(2 * time.Second)
	for !sawRefsChanged {
		select {
		case e := <-events:
			sawCommand = sawCommand || e.Name == "command"
			sawRefsChanged = e.Name == "repo:changed" && string(e.Payload) == `{"scope":"refs"}`
		case <-deadline:
			t.Fatal("no repo:changed(refs) event after Refresh (the watcher's CheckNow)")
		}
	}
	// The watcher's and the poller's events are both already enqueued by the
	// time Refresh returns, in either order: drain what is left too.
	if !sawCommand {
		sawCommand = countCommandEvents(events) > 0
	}
	if !sawCommand {
		t.Error("no command event, want the poller's CheckNow to have run too")
	}
}

// Requirements: FR-LOG-05, FR-REPO-04, FR-REPO-05, QA-PERF-06

package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/core/coretest"
	"github.com/brogli/gaertli/internal/git"
	"github.com/brogli/gaertli/internal/git/gittest"
	"github.com/brogli/gaertli/internal/watch"
)

func TestManagerOpenReusesSessionForSameToplevel(t *testing.T) {
	repo := gittest.New(t)
	sub := filepath.Join(repo.Dir, "sub", "dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	m := NewManager(func(core.Event) {})

	first := openSession(t, m, repo.Dir)
	second := openSession(t, m, sub)

	if first != second {
		t.Errorf("Open on a subdirectory minted a new session, want the same *Worktree")
	}
}

// openSession opens path's session on m and closes it when the test ends.
func openSession(t *testing.T, m *Manager, path string) *Worktree {
	t.Helper()
	wt, err := m.Open(t.Context(), core.RepoPath(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close(wt.ID) })
	return wt
}

func TestManagerOpenSharesRepoAcrossLinkedWorktrees(t *testing.T) {
	repo := gittest.New(t)
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", "-b", "feature", linked)
	m := NewManager(func(core.Event) {})

	main := openSession(t, m, repo.Dir)
	other := openSession(t, m, linked)

	if main.ID == other.ID {
		t.Errorf("linked worktrees got the same ID %q", main.ID)
	}
	if main.Repo != other.Repo {
		t.Errorf("linked worktrees did not share a *Repo")
	}
}

func TestManagerOpenRejectsNonRepoPathSayingSo(t *testing.T) {
	m := NewManager(func(core.Event) {})
	dir := t.TempDir()

	_, err := m.Open(t.Context(), core.RepoPath(dir))

	ce := coretest.AsError(t, err, core.NotARepo)
	if want := dir + " is not a git repository"; ce.Message != want {
		t.Errorf("Message = %q, want %q", ce.Message, want)
	}
	if ce.Args["path"] != dir {
		t.Errorf("Args[path] = %q, want %q", ce.Args["path"], dir)
	}
	if ce.Key != "notARepo" {
		t.Errorf("Key = %q, want notARepo", ce.Key)
	}
}

func TestManagerOpenRejectsMissingPath(t *testing.T) {
	m := NewManager(func(core.Event) {})
	missing := filepath.Join(t.TempDir(), "missing")

	_, err := m.Open(t.Context(), core.RepoPath(missing))

	ce := coretest.AsError(t, err, core.NotARepo)
	if ce.Args["path"] != missing {
		t.Errorf("Args[path] = %q, want %q", ce.Args["path"], missing)
	}
	if ce.Key != "notARepo" {
		t.Errorf("Key = %q, want notARepo", ce.Key)
	}
}

func TestManagerGetReturnsInternalErrorForUnknownID(t *testing.T) {
	m := NewManager(func(core.Event) {})

	_, err := m.Get("bogus")

	coretest.AssertCode(t, err, core.Internal)
}

func TestWorktreeRunnerEmitsACommandSinkEvent(t *testing.T) {
	repo := gittest.New(t)
	events := make(chan core.Event, 8)
	m := NewManager(func(e core.Event) { events <- e })

	wt := openSession(t, m, repo.Dir)
	coretest.DrainEvents(events) // the poller's own baseline status check

	if _, err := wt.Runner.Run(t.Context(), git.Call{Dir: wt.Path, ReadOnly: true, Args: []string{"status"}}); err != nil {
		t.Fatal(err)
	}

	select {
	case e := <-events:
		if e.Name != "command" || e.Worktree != wt.ID {
			t.Fatalf("event = %+v, want Name \"command\", Worktree %q", e, wt.ID)
		}
		var entry core.CommandRecord
		if err := json.Unmarshal(e.Payload, &entry); err != nil {
			t.Fatal(err)
		}
		if entry.Command != "git status" {
			t.Errorf("entry.Command = %q, want %q", entry.Command, "git status")
		}
	default:
		t.Error("sink received no event")
	}
}

func TestRepoLogBeginLoadIsIdempotent(t *testing.T) {
	r := &Repo{}

	if _, ok := r.LogBeginLoad(); !ok {
		t.Fatal("first LogBeginLoad() ok = false, want true")
	}
	if _, ok := r.LogBeginLoad(); ok {
		t.Error("second LogBeginLoad() while loading ok = true, want false")
	}

	r.LogFinish(0, nil, nil)
	if _, ok := r.LogBeginLoad(); ok {
		t.Error("LogBeginLoad() after LogFinish ok = true, want false (already loaded)")
	}
}

func TestRepoLogSnapshotClampsWindowAndReportsState(t *testing.T) {
	r := &Repo{}
	gen, _ := r.LogBeginLoad()
	r.LogAppend(gen, core.LogCommit{SHA: "a"}, core.LogCommit{SHA: "b"})

	view := r.LogSnapshot(0, 10)
	if view.Loaded != 2 || view.Done || view.Err != nil {
		t.Fatalf("LogSnapshot = %d rows, loaded=%d, done=%v, err=%v", len(view.Rows), view.Loaded, view.Done, view.Err)
	}
	if len(view.Rows) != 2 {
		t.Fatalf("rows = %d, want 2 (clamped to loaded)", len(view.Rows))
	}

	want := core.Errorf(core.Internal, "boom")
	r.LogFinish(gen, want, nil)
	view = r.LogSnapshot(0, 10)
	if !view.Done || view.Err != want {
		t.Errorf("after LogFinish: done=%v err=%v, want true, %v", view.Done, view.Err, want)
	}
}

func TestRepoLogIndicesOfMapsSHAsToRowIndices(t *testing.T) {
	r := &Repo{}
	gen, _ := r.LogBeginLoad()
	r.LogAppend(gen, core.LogCommit{SHA: "a"}, core.LogCommit{SHA: "b"}, core.LogCommit{SHA: "c"})

	indices, resultGen := r.LogIndicesOf(map[string]bool{"c": true, "a": true, "z": true})

	if !slices.Equal(indices, []int{0, 2}) {
		t.Errorf("indices = %v, want [0 2] (a and c, in row order; z absent)", indices)
	}
	if resultGen != gen {
		t.Errorf("generation = %d, want %d", resultGen, gen)
	}
}

func TestWorktreeLogIndicesOfPrefersTheFilteredViewWhenActive(t *testing.T) {
	w := &Worktree{Repo: &Repo{}}
	baseGen, _ := w.Repo.LogBeginLoad()
	w.Repo.LogAppend(baseGen, core.LogCommit{SHA: "base"})
	filterGen := w.FilterBegin(core.LogFilter{Path: "x"}, func() {})
	w.FilterAppend(filterGen, core.LogCommit{SHA: "filtered"})

	indices, gen, filtered := w.LogIndicesOf(map[string]bool{"filtered": true, "base": true})

	if !filtered {
		t.Error("filtered = false, want true: a filter is active")
	}
	if !slices.Equal(indices, []int{0}) {
		t.Errorf("indices = %v, want [0]: the filtered view's own row, base's sha is invisible to it", indices)
	}
	if gen != filterGen {
		t.Errorf("generation = %d, want the filtered view's %d", gen, filterGen)
	}
}

func TestWorktreeLogIndicesOfFallsBackToTheBaseLogWithNoFilterActive(t *testing.T) {
	w := &Worktree{Repo: &Repo{}}
	gen, _ := w.Repo.LogBeginLoad()
	w.Repo.LogAppend(gen, core.LogCommit{SHA: "a"})

	indices, resultGen, filtered := w.LogIndicesOf(map[string]bool{"a": true})

	if filtered {
		t.Error("filtered = true, want false: no filter active")
	}
	if !slices.Equal(indices, []int{0}) || resultGen != gen {
		t.Errorf("indices = %v, generation = %d, want [0], %d", indices, resultGen, gen)
	}
}

func TestLogBeginReloadFailsWhileALoadIsInFlight(t *testing.T) {
	r := &Repo{}
	gen, ok := r.LogBeginLoad()
	if !ok {
		t.Fatal("LogBeginLoad() ok = false, want true")
	}

	if _, ok := r.LogBeginReload(); ok {
		t.Error("LogBeginReload() while a load is in flight ok = true, want false")
	}

	r.LogFinish(gen, nil, nil)
}

// FilterRestart reads and acts on "is a filter active" under one lock, so a
// SetLogFilter landing between a separate read and restart cannot revive a
// filter FilterClear just cleared.
func TestFilterRestartFindsNothingActiveAfterFilterClear(t *testing.T) {
	w := &Worktree{}
	w.FilterBegin(core.LogFilter{Path: "a"}, func() {})

	w.FilterClear()

	if _, _, ok := w.FilterRestart(func() {}); ok {
		t.Error("FilterRestart() after FilterClear ok = true, want false")
	}
}

// Nor can it restart a filter SetLogFilter already replaced.
func TestFilterRestartRestartsTheCurrentFilterNotAnEarlierOne(t *testing.T) {
	w := &Worktree{}
	w.FilterBegin(core.LogFilter{Path: "a"}, func() {})
	second := core.LogFilter{Path: "b"}
	gen2 := w.FilterBegin(second, func() {})

	filter, gen, ok := w.FilterRestart(func() {})

	if !ok {
		t.Fatal("FilterRestart() ok = false, want true")
	}
	if filter.Path != second.Path {
		t.Errorf("FilterRestart() filter = %+v, want the current one %+v", filter, second)
	}
	if gen <= gen2 {
		t.Errorf("FilterRestart() gen = %d, want greater than the current load's %d", gen, gen2)
	}
}

func TestManagerOpenWiresARealRepoWatcher(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")

	events := make(chan core.Event, 16)
	m := NewManager(func(e core.Event) { events <- e })
	wt := openSession(t, m, repo.Dir)
	coretest.DrainEvents(events)

	gittest.Commit(t, repo.Dir, "seed.txt", "y", "second")

	// above FallbackPollInterval: still passes when fsnotify cannot start and
	// only the fallback poll reports.
	waitForEvent(t, events, "repo:changed(refs)", func(e core.Event) bool {
		// a real commit also touches the index; only the refs scope matters here.
		return e.Name == "repo:changed" && e.Repo == wt.Repo.Key && string(e.Payload) == `{"scope":"refs"}`
	})
}

func TestManagerOpenWiresARealWorktreePoller(t *testing.T) {
	orig := watch.PollInterval
	watch.PollInterval = 30 * time.Millisecond
	t.Cleanup(func() { watch.PollInterval = orig })

	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")

	events := make(chan core.Event, 16)
	m := NewManager(func(e core.Event) { events <- e })
	wt := openSession(t, m, repo.Dir)
	coretest.DrainEvents(events)

	if err := os.WriteFile(filepath.Join(repo.Dir, "seed.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := waitForEvent(t, events, "repo:changed(index)", func(e core.Event) bool {
		return e.Name == "repo:changed" && e.Worktree == wt.ID && string(e.Payload) == `{"scope":"index"}`
	})
	if e.Repo != "" {
		t.Errorf("Repo = %q, want empty (worktree-targeted)", e.Repo)
	}
}

// waitForEvent returns the first event on events matching cond. desc names
// the awaited event for the stack dump of a go test timeout.
func waitForEvent(t *testing.T, events chan core.Event, desc string, cond func(core.Event) bool) core.Event {
	t.Helper()
	for {
		if e := <-events; cond(e) {
			return e
		}
	}
}

// fakeRepoWatcher and fakeWorktreePoller stand in for the real watch types,
// counting Close() without touching fsnotify or git.
type fakeRepoWatcher struct{ closeCount int }

func (f *fakeRepoWatcher) CheckNow()    {}
func (f *fakeRepoWatcher) Close() error { f.closeCount++; return nil }

type fakeWorktreePoller struct{ closeCount int }

func (f *fakeWorktreePoller) Focus()    {}
func (f *fakeWorktreePoller) Blur()     {}
func (f *fakeWorktreePoller) CheckNow() {}
func (f *fakeWorktreePoller) Close()    { f.closeCount++ }

func TestManagerCloseTearsDownThePollerAndTheLastWorktreesWatcher(t *testing.T) {
	repo := gittest.New(t)
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", "-b", "feature", linked)

	repoWatcherFake := &fakeRepoWatcher{}
	var pollers []*fakeWorktreePoller
	m := NewManager(func(core.Event) {})
	m.newWatcher = func(string, func(core.ChangeScope)) (repoWatcher, error) { return repoWatcherFake, nil }
	m.newPoller = func(git.Runner, string, func(core.ChangeScope)) worktreePoller {
		p := &fakeWorktreePoller{}
		pollers = append(pollers, p)
		return p
	}

	main := openSession(t, m, repo.Dir)
	other := openSession(t, m, linked)

	m.Close(main.ID)
	if repoWatcherFake.closeCount != 0 {
		t.Errorf("repo watcher Close() called %d times after the first worktree closed, want 0", repoWatcherFake.closeCount)
	}
	if pollers[0].closeCount != 1 {
		t.Errorf("first worktree's poller Close() called %d times, want 1", pollers[0].closeCount)
	}

	m.Close(other.ID)
	if repoWatcherFake.closeCount != 1 {
		t.Errorf("repo watcher Close() called %d times after the last worktree closed, want 1", repoWatcherFake.closeCount)
	}
	if pollers[1].closeCount != 1 {
		t.Errorf("second worktree's poller Close() called %d times, want 1", pollers[1].closeCount)
	}
}

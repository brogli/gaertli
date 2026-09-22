package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
	"github.com/brogli/nebegit/internal/watch"
)

func TestManagerOpenReusesSessionForSameToplevel(t *testing.T) {
	repo := gittest.New(t)
	sub := filepath.Join(repo.Dir, "sub", "dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	m := NewManager(func(core.Event) {})

	first, err := m.Open(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.Open(context.Background(), core.RepoPath(sub))
	if err != nil {
		t.Fatal(err)
	}

	if first != second {
		t.Errorf("Open on a subdirectory minted a new session, want the same *Worktree")
	}
}

func TestManagerOpenSharesRepoAcrossLinkedWorktrees(t *testing.T) {
	repo := gittest.New(t)
	linked := filepath.Join(t.TempDir(), "linked")
	run(t, repo.Dir, "worktree", "add", "-b", "feature", linked)
	m := NewManager(func(core.Event) {})

	main, err := m.Open(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	other, err := m.Open(context.Background(), core.RepoPath(linked))
	if err != nil {
		t.Fatal(err)
	}

	if main.ID == other.ID {
		t.Errorf("linked worktrees got the same ID %q", main.ID)
	}
	if main.Repo != other.Repo {
		t.Errorf("linked worktrees did not share a *Repo")
	}
}

func TestFR_REPO_05_ManagerOpenRejectsNonRepoPathSayingSo(t *testing.T) {
	m := NewManager(func(core.Event) {})
	dir := t.TempDir()

	_, err := m.Open(context.Background(), core.RepoPath(dir))

	ce := assertCode(t, err, core.NotARepo)
	if want := dir + " is not a git repository"; ce.Message != want {
		t.Errorf("Message = %q, want %q (FR-REPO-05: say so clearly)", ce.Message, want)
	}
}

func TestManagerOpenRejectsMissingPath(t *testing.T) {
	m := NewManager(func(core.Event) {})

	_, err := m.Open(context.Background(), core.RepoPath(filepath.Join(t.TempDir(), "missing")))

	_ = assertCode(t, err, core.NotARepo)
}

func TestManagerGetReturnsInternalErrorForUnknownID(t *testing.T) {
	m := NewManager(func(core.Event) {})

	_, err := m.Get("bogus")

	_ = assertCode(t, err, core.Internal)
}

func TestWorktreeRunnerAppendsToCommandLogAndEmitsSink(t *testing.T) {
	repo := gittest.New(t)
	events := make(chan core.Event, 8)
	m := NewManager(func(e core.Event) { events <- e })

	wt, err := m.Open(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	drainEvents(events) // the poller's own baseline status check (spec 12)

	if _, err := wt.Runner.Run(context.Background(), git.Call{Dir: wt.Path, ReadOnly: true, Args: []string{"status"}}); err != nil {
		t.Fatal(err)
	}

	got := wt.CommandLog()
	if last := got[len(got)-1]; last.Command != "git status" {
		t.Fatalf("last CommandLog entry = %q, want %q", last.Command, "git status")
	}

	select {
	case e := <-events:
		if e.Name != "command" || e.Worktree != wt.ID {
			t.Errorf("event = %+v, want Name \"command\", Worktree %q", e, wt.ID)
		}
	default:
		t.Error("sink received no event")
	}
}

// drainEvents discards whatever is already buffered, without waiting.
func drainEvents(events chan core.Event) {
	for {
		select {
		case <-events:
		default:
			return
		}
	}
}

func TestWorktreeCommandLogCapsAtLogCap(t *testing.T) {
	wt := &Worktree{}
	for i := range logCap + 1 {
		wt.append(core.CommandLogEntry{Command: string(rune('a' + i%26))})
	}

	got := wt.CommandLog()
	if len(got) != logCap {
		t.Fatalf("CommandLog() = %d entries, want %d", len(got), logCap)
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

func TestFR_REPO_04_LogBeginReloadFailsWhileALoadIsInFlight(t *testing.T) {
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

func TestFR_REPO_04_ManagerOpenWiresARealRepoWatcher(t *testing.T) {
	repo := gittest.New(t)
	commit(t, repo.Dir, "seed.txt", "x", "seed")

	events := make(chan core.Event, 16)
	m := NewManager(func(e core.Event) { events <- e })
	wt, err := m.Open(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	drainEvents(events)

	commit(t, repo.Dir, "seed.txt", "y", "second")

	deadline := time.After(2 * time.Second)
	for {
		select {
		case e := <-events:
			// a real commit also touches the index; only the refs scope matters here.
			if e.Name == "repo:changed" && e.Repo == wt.Repo.Key && string(e.Payload) == `{"scope":"refs"}` {
				return
			}
		case <-deadline:
			t.Fatal("no repo:changed(refs) event within 2s")
		}
	}
}

func TestFR_REPO_04_ManagerOpenWiresARealWorktreePoller(t *testing.T) {
	orig := watch.PollInterval
	watch.PollInterval = 30 * time.Millisecond
	defer func() { watch.PollInterval = orig }()

	repo := gittest.New(t)
	commit(t, repo.Dir, "seed.txt", "x", "seed")

	events := make(chan core.Event, 16)
	m := NewManager(func(e core.Event) { events <- e })
	wt, err := m.Open(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	drainEvents(events)

	if err := os.WriteFile(filepath.Join(repo.Dir, "seed.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case e := <-events:
			if e.Name != "repo:changed" || e.Worktree != wt.ID || string(e.Payload) != `{"scope":"index"}` {
				continue
			}
			if e.Repo != "" {
				t.Errorf("Repo = %q, want empty (worktree-targeted)", e.Repo)
			}
			return
		case <-deadline:
			t.Fatal("no repo:changed(index) event within 2s")
		}
	}
}

// fakeRepoWatcher and fakeWorktreePoller stand in for the real watch types
// (case 18): counting Close() without touching fsnotify or git.
type fakeRepoWatcher struct{ closeCount int }

func (f *fakeRepoWatcher) CheckNow()    {}
func (f *fakeRepoWatcher) Close() error { f.closeCount++; return nil }

type fakeWorktreePoller struct{ closeCount int }

func (f *fakeWorktreePoller) Focus()    {}
func (f *fakeWorktreePoller) Blur()     {}
func (f *fakeWorktreePoller) CheckNow() {}
func (f *fakeWorktreePoller) Close()    { f.closeCount++ }

func TestQA_PERF_06_ManagerCloseTearsDownThePollerAndTheLastWorktreesWatcher(t *testing.T) {
	repo := gittest.New(t)
	linked := filepath.Join(t.TempDir(), "linked")
	run(t, repo.Dir, "worktree", "add", "-b", "feature", linked)

	repoWatcherFake := &fakeRepoWatcher{}
	var pollers []*fakeWorktreePoller
	m := NewManager(func(core.Event) {})
	m.newWatcher = func(string, func(watch.Scope)) (repoWatcher, error) { return repoWatcherFake, nil }
	m.newPoller = func(git.Runner, string, func(watch.Scope)) worktreePoller {
		p := &fakeWorktreePoller{}
		pollers = append(pollers, p)
		return p
	}

	main, err := m.Open(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	other, err := m.Open(context.Background(), core.RepoPath(linked))
	if err != nil {
		t.Fatal(err)
	}

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

func commit(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", "--", name)
	if _, err := (git.Runner{}).Run(context.Background(), git.Call{Dir: dir, Args: []string{"commit", "-F", "-"}, Stdin: []byte(msg)}); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := (git.Runner{}).Run(context.Background(), git.Call{Dir: dir, Args: args}); err != nil {
		t.Fatal(err)
	}
}

func assertCode(t *testing.T, err error, want core.Code) *core.Error {
	t.Helper()
	var ce *core.Error
	if !errors.As(err, &ce) || ce.Code != want {
		t.Fatalf("err = %v, want *core.Error{Code: %s}", err, want)
	}
	return ce
}

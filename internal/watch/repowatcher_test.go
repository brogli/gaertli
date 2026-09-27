package watch

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
)

// setInterval overrides a process-wide timing var for the duration of t;
// tests in this package never run in parallel.
func setInterval(t *testing.T, v *time.Duration, d time.Duration) {
	t.Helper()
	orig := *v
	*v = d
	t.Cleanup(func() { *v = orig })
}

func TestFR_REPO_04_DebounceCoalescesBurstsIntoOncePerScope(t *testing.T) {
	setInterval(t, &DebounceInterval, 20*time.Millisecond)

	var mu sync.Mutex
	var got []core.ChangeScope
	w := &RepoWatcher{pending: map[core.ChangeScope]bool{}, onChange: func(s core.ChangeScope) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, s)
	}}

	for _, s := range []core.ChangeScope{core.RefsChanged, core.StateChanged, core.RefsChanged, core.StateChanged, core.RefsChanged} {
		w.notify(s)
	}
	time.Sleep(10 * DebounceInterval)

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 {
		t.Fatalf("onChange calls = %d %v, want 2", len(got), got)
	}
	seen := map[core.ChangeScope]bool{got[0]: true, got[1]: true}
	if !seen[core.RefsChanged] || !seen[core.StateChanged] {
		t.Errorf("onChange scopes = %v, want one core.RefsChanged and one State", got)
	}
}

func TestQA_PERF_06_FallbackTickerCallsCheckNowAtABoundedRate(t *testing.T) {
	const interval = 20 * time.Millisecond
	var count atomic.Int32
	w := &RepoWatcher{done: make(chan struct{})}
	go w.fallbackLoop(interval, func() { count.Add(1) })
	defer func() { _ = w.Close() }()

	time.Sleep(4*interval + interval/2)

	if got := count.Load(); got < 3 || got > 5 {
		t.Errorf("CheckNow invocations = %d, want 4±1", got)
	}
}

func TestFR_REPO_04_RepoWatcherFiresOnRefsChangeFromOutside(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	_, got := watchRepo(t, repo)

	gittest.Commit(t, repo.Dir, "seed.txt", "y", "second")

	waitForScope(t, got, core.RefsChanged)
}

// watchRepo starts a RepoWatcher on repo's git dir, closed when the test
// ends, and returns it with the channel its onChange feeds.
func watchRepo(t *testing.T, repo *gittest.Repo) (*RepoWatcher, chan core.ChangeScope) {
	t.Helper()
	got := make(chan core.ChangeScope, 10)
	w, err := NewRepoWatcher(filepath.Join(repo.Dir, ".git"), func(s core.ChangeScope) { got <- s })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return w, got
}

// TestFR_REPO_04_RepoWatcherFiresOnRefsChangeFromAConfigEditFromOutside
// covers a terminal `git remote add` or `git branch -u`, which writes only
// .git/config: PushTarget and Refs must reload to pick it up.
func TestFR_REPO_04_RepoWatcherFiresOnRefsChangeFromAConfigEditFromOutside(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	_, got := watchRepo(t, repo)

	gittest.Run(t, repo.Dir, "remote", "add", "origin", "https://example.invalid/repo.git")

	waitForScope(t, got, core.RefsChanged)
}

func TestFR_REPO_04_RepoWatcherFiresOnStateDirCreatedDirectly(t *testing.T) {
	repo := gittest.New(t)
	_, got := watchRepo(t, repo)

	if err := os.Mkdir(filepath.Join(repo.Dir, ".git", "rebase-merge"), 0o755); err != nil {
		t.Fatal(err)
	}

	waitForScope(t, got, core.StateChanged)
}

// pollOnly is a RepoWatcher on commonDir with no fsnotify and no ticker:
// only an explicit CheckNow reports, as the fallback poll would.
func pollOnly(t *testing.T, commonDir string) (*RepoWatcher, chan core.ChangeScope) {
	t.Helper()
	setInterval(t, &DebounceInterval, 10*time.Millisecond)
	got := make(chan core.ChangeScope, 10)
	w := &RepoWatcher{commonDir: commonDir, pending: map[core.ChangeScope]bool{}, onChange: func(s core.ChangeScope) { got <- s }}
	w.mtimes = w.walk()
	return w, got
}

func TestFR_REPO_04_FallbackPollReportsEachKindOfChange(t *testing.T) {
	for _, tc := range []struct {
		name   string
		before func(gitDir string) error // the state the poll starts from
		change func(gitDir string) error
		want   core.ChangeScope
	}{
		{
			"deleted state file",
			func(d string) error { return os.WriteFile(filepath.Join(d, "MERGE_HEAD"), nil, 0o644) },
			func(d string) error { return os.Remove(filepath.Join(d, "MERGE_HEAD")) },
			core.StateChanged,
		},
		{
			"created state dir",
			func(string) error { return nil },
			func(d string) error { return os.Mkdir(filepath.Join(d, "rebase-merge"), 0o755) },
			core.StateChanged,
		},
		{
			"deleted state dir",
			func(d string) error { return os.Mkdir(filepath.Join(d, "sequencer"), 0o755) },
			func(d string) error { return os.Remove(filepath.Join(d, "sequencer")) },
			core.StateChanged,
		},
		{
			"removed worktree",
			func(d string) error {
				wt := filepath.Join(d, "worktrees", "wt")
				return errors.Join(os.MkdirAll(wt, 0o755), os.WriteFile(filepath.Join(wt, "HEAD"), nil, 0o644))
			},
			func(d string) error { return os.RemoveAll(filepath.Join(d, "worktrees", "wt")) },
			core.WorktreesChanged,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gitDir := t.TempDir()
			if err := tc.before(gitDir); err != nil {
				t.Fatal(err)
			}
			w, got := pollOnly(t, gitDir)

			if err := tc.change(gitDir); err != nil {
				t.Fatal(err)
			}
			w.CheckNow()

			waitForScope(t, got, tc.want)
		})
	}
}

func TestQA_COMP_02_RepoWatcherToleratesConcurrentReadsWhileWriting(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	w, got := watchRepo(t, repo)

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	wg.Go(func() {
		if _, err := (git.Runner{}).Run(t.Context(), git.Call{
			Dir: repo.Dir, Args: []string{"commit", "--allow-empty", "-m", "concurrent"},
		}); err != nil {
			errs <- err
		}
	})
	for range 5 {
		wg.Go(func() {
			w.CheckNow()
			if _, err := (git.Runner{}).Run(t.Context(), git.Call{
				Dir: repo.Dir, ReadOnly: true, Args: []string{"status"},
			}); err != nil {
				errs <- err
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent call failed: %v", err)
	}

	waitForScope(t, got, core.RefsChanged)
}

func waitForScope(t *testing.T, got chan core.ChangeScope, want core.ChangeScope) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case scope := <-got:
			if scope == want {
				return
			}
		case <-deadline:
			t.Fatalf("onChange(%s) did not fire within 2s", want)
		}
	}
}

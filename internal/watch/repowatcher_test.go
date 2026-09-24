package watch

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	var got []Scope
	w := &RepoWatcher{pending: map[Scope]bool{}, onChange: func(s Scope) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, s)
	}}

	for _, s := range []Scope{Refs, State, Refs, State, Refs} {
		w.notify(s)
	}
	time.Sleep(10 * DebounceInterval)

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 {
		t.Fatalf("onChange calls = %d %v, want 2", len(got), got)
	}
	seen := map[Scope]bool{got[0]: true, got[1]: true}
	if !seen[Refs] || !seen[State] {
		t.Errorf("onChange scopes = %v, want one Refs and one State", got)
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

	waitForScope(t, got, Refs)
}

// watchRepo starts a RepoWatcher on repo's git dir, closed when the test
// ends, and returns it with the channel its onChange feeds.
func watchRepo(t *testing.T, repo *gittest.Repo) (*RepoWatcher, chan Scope) {
	t.Helper()
	got := make(chan Scope, 10)
	w, err := NewRepoWatcher(filepath.Join(repo.Dir, ".git"), func(s Scope) { got <- s })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return w, got
}

func TestFR_REPO_04_RepoWatcherFiresOnStateDirCreatedDirectly(t *testing.T) {
	repo := gittest.New(t)
	_, got := watchRepo(t, repo)

	if err := os.Mkdir(filepath.Join(repo.Dir, ".git", "rebase-merge"), 0o755); err != nil {
		t.Fatal(err)
	}

	waitForScope(t, got, State)
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

	waitForScope(t, got, Refs)
}

func waitForScope(t *testing.T, got chan Scope, want Scope) {
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

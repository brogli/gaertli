package watch

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
)

// setInterval overrides a timing var for the duration of a test, restoring
// it afterwards (the vars are process-wide; tests never run in parallel
// with each other in this package).
func setInterval(v *time.Duration, d time.Duration) func() {
	orig := *v
	*v = d
	return func() { *v = orig }
}

func TestFR_REPO_04_DebounceCoalescesBurstsIntoOncePerScope(t *testing.T) {
	defer setInterval(&DebounceInterval, 20*time.Millisecond)()

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
	defer setInterval(&FallbackPollInterval, 20*time.Millisecond)()

	var count atomic.Int32
	w := &RepoWatcher{
		fallback:   time.NewTicker(FallbackPollInterval),
		fallbackFn: func() { count.Add(1) },
		done:       make(chan struct{}),
	}
	go w.fallbackLoop()
	defer func() { _ = w.Close() }()

	time.Sleep(4*FallbackPollInterval + FallbackPollInterval/2)

	if got := count.Load(); got < 3 || got > 5 {
		t.Errorf("CheckNow invocations = %d, want 4±1", got)
	}
}

func TestFR_REPO_04_RepoWatcherFiresOnRefsChangeFromOutside(t *testing.T) {
	repo := gittest.New(t)
	commit(t, repo.Dir, "seed.txt", "x", "seed")
	commonDir := filepath.Join(repo.Dir, ".git")

	got := make(chan Scope, 10)
	w, err := NewRepoWatcher(commonDir, func(s Scope) { got <- s })
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()

	commit(t, repo.Dir, "seed.txt", "y", "second")

	waitForScope(t, got, Refs)
}

func TestFR_REPO_04_RepoWatcherFiresOnStateDirCreatedDirectly(t *testing.T) {
	repo := gittest.New(t)
	commonDir := filepath.Join(repo.Dir, ".git")

	got := make(chan Scope, 10)
	w, err := NewRepoWatcher(commonDir, func(s Scope) { got <- s })
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()

	if err := os.Mkdir(filepath.Join(commonDir, "rebase-merge"), 0o755); err != nil {
		t.Fatal(err)
	}

	waitForScope(t, got, State)
}

func TestQA_COMP_02_RepoWatcherToleratesConcurrentReadsWhileWriting(t *testing.T) {
	repo := gittest.New(t)
	commit(t, repo.Dir, "seed.txt", "x", "seed")
	commonDir := filepath.Join(repo.Dir, ".git")

	got := make(chan Scope, 10)
	w, err := NewRepoWatcher(commonDir, func(s Scope) { got <- s })
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := (git.Runner{}).Run(context.Background(), git.Call{
			Dir: repo.Dir, Args: []string{"commit", "--allow-empty", "-m", "concurrent"},
		}); err != nil {
			errs <- err
		}
	}()
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.CheckNow()
			if _, err := (git.Runner{}).Run(context.Background(), git.Call{
				Dir: repo.Dir, ReadOnly: true, Args: []string{"status"},
			}); err != nil {
				errs <- err
			}
		}()
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

func commit(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (git.Runner{}).Run(context.Background(), git.Call{Dir: dir, Args: []string{"add", "--", name}}); err != nil {
		t.Fatal(err)
	}
	if _, err := (git.Runner{}).Run(context.Background(), git.Call{Dir: dir, Args: []string{"commit", "-F", "-"}, Stdin: []byte(msg)}); err != nil {
		t.Fatal(err)
	}
}

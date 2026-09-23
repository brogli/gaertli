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
	"github.com/brogli/nebegit/internal/testbin"
)

// fakegit is internal/git/testdata/fakegit, for the test that needs a
// controllable delay.
var fakegit string

func TestMain(m *testing.M) {
	testbin.Main(m, testbin.Bin{Path: &fakegit, Pkg: "../git/testdata/fakegit"})
}

func TestFR_REPO_04_WorktreePollerObservesAWorkingFileEditDirectly(t *testing.T) {
	defer setInterval(&PollInterval, 30*time.Millisecond)()

	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")

	got := make(chan Scope, 10)
	p := NewWorktreePoller(git.Runner{}, repo.Dir, func(s Scope) { got <- s })
	p.Focus()
	defer p.Close()

	if err := os.WriteFile(filepath.Join(repo.Dir, "seed.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}

	waitForScope(t, got, Index)
}

// A second edit to a file already listed as changed leaves git status's
// output identical; the poll still reports it, so an open diff catches up.
func TestFR_REPO_04_WorktreePollerSeesAFurtherEditToAnAlreadyChangedFile(t *testing.T) {
	for _, file := range []string{"seed.txt", "untracked.txt"} {
		t.Run(file, func(t *testing.T) {
			repo := gittest.New(t)
			gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
			path := filepath.Join(repo.Dir, file)
			if err := os.WriteFile(path, []byte("first edit"), 0o644); err != nil {
				t.Fatal(err)
			}
			var changes atomic.Int32
			p := NewWorktreePoller(git.Runner{}, repo.Dir, func(Scope) { changes.Add(1) })
			defer p.Close()
			p.CheckNow() // baseline: the file already changed

			if err := os.WriteFile(path, []byte("second, longer edit"), 0o644); err != nil {
				t.Fatal(err)
			}
			p.CheckNow()

			if changes.Load() != 1 {
				t.Errorf("changes reported = %d, want 1", changes.Load())
			}
		})
	}
}

func TestFR_REPO_04_WorktreePollerBacksOffAfterASlowPollThenResets(t *testing.T) {
	defer setInterval(&PollInterval, 40*time.Millisecond)()

	sleepFile := filepath.Join(t.TempDir(), "sleep-once")
	t.Setenv("FAKEGIT_SLEEP_ONCE_FILE", sleepFile)
	t.Setenv("FAKEGIT_SLEEP_ONCE_MS", "150")

	var mu sync.Mutex
	var polls []time.Time
	runner := git.Runner{Git: fakegit, Log: func(git.Entry) {
		mu.Lock()
		polls = append(polls, time.Now())
		mu.Unlock()
	}}
	p := NewWorktreePoller(runner, t.TempDir(), func(Scope) {})

	p.Focus() // poll #1: slow, the baseline
	defer p.Close()

	deadline := time.Now().Add(3 * time.Second)
	for {
		mu.Lock()
		n := len(polls)
		mu.Unlock()
		if n >= 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d polls observed in time, want at least 3", n)
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	afterSlow := polls[1].Sub(polls[0])
	afterFast := polls[2].Sub(polls[1])
	if afterSlow < PollInterval*3/2 {
		t.Errorf("gap after the slow poll = %v, want roughly doubled (> %v)", afterSlow, PollInterval*3/2)
	}
	if afterFast > PollInterval*3/2 {
		t.Errorf("gap after the fast poll = %v, want back to ~%v", afterFast, PollInterval)
	}
}

func TestQA_PERF_06_BlurStopsPollingAfterTheTimer(t *testing.T) {
	defer setInterval(&PollInterval, 20*time.Millisecond)()

	var count atomic.Int32
	runner := git.Runner{Git: "/nonexistent/nebegit-test-git", Log: func(git.Entry) { count.Add(1) }}
	p := NewWorktreePoller(runner, t.TempDir(), func(Scope) {})

	p.Focus()
	time.Sleep(2*PollInterval + PollInterval/2)
	p.Blur()
	time.Sleep(PollInterval) // let any poll already in flight finish
	afterBlur := count.Load()
	time.Sleep(2*PollInterval + PollInterval/2)

	if got := count.Load(); got != afterBlur {
		t.Errorf("call count after Blur: %d -> %d, want unchanged", afterBlur, got)
	}
}

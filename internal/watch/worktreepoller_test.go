// Requirements: FR-REPO-04, QA-PERF-06

package watch

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
)

// nonexistentGit makes every call fail before any git runs, so the timer
// tests can run in a synctest bubble: its fake clock advances only while
// every goroutine in it is durably blocked, which one waiting on a child
// process never is.
const nonexistentGit = "/nonexistent/nebegit-test-git"

func TestWorktreePollerObservesAWorkingFileEditDirectly(t *testing.T) {
	setInterval(t, &PollInterval, 30*time.Millisecond)

	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")

	got := make(chan core.ChangeScope, 10)
	p := NewWorktreePoller(git.Runner{}, repo.Dir, func(s core.ChangeScope) { got <- s })
	p.Focus()
	defer p.Close()

	if err := os.WriteFile(filepath.Join(repo.Dir, "seed.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}

	waitForScope(t, got, core.IndexChanged)
}

// A second edit to a file already listed as changed leaves git status's
// output identical; the poll still reports it, so an open diff catches up.
func TestWorktreePollerSeesAFurtherEditToAnAlreadyChangedFile(t *testing.T) {
	for _, file := range []string{"seed.txt", "untracked.txt"} {
		t.Run(file, func(t *testing.T) {
			repo := gittest.New(t)
			gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
			path := filepath.Join(repo.Dir, file)
			if err := os.WriteFile(path, []byte("first edit"), 0o644); err != nil {
				t.Fatal(err)
			}
			var changes atomic.Int32
			p := NewWorktreePoller(git.Runner{}, repo.Dir, func(core.ChangeScope) { changes.Add(1) })
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

func TestWorktreePollerBacksOffAfterASlowPollThenResets(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var slowOnce sync.Once
		var mu sync.Mutex
		var polls []time.Time
		runner := git.Runner{Git: nonexistentGit, Log: func(git.Entry) {
			slowOnce.Do(func() { time.Sleep(2 * PollInterval) })
			mu.Lock()
			polls = append(polls, time.Now())
			mu.Unlock()
		}}
		p := NewWorktreePoller(runner, t.TempDir(), func(core.ChangeScope) {})

		p.Focus() // poll #1: slow, the baseline
		defer p.Close()
		time.Sleep(3*PollInterval + PollInterval/2)

		mu.Lock()
		defer mu.Unlock()
		if len(polls) < 3 {
			t.Fatalf("only %d polls observed in time, want at least 3", len(polls))
		}
		afterSlow := polls[1].Sub(polls[0])
		afterFast := polls[2].Sub(polls[1])
		if afterSlow < PollInterval*3/2 {
			t.Errorf("gap after the slow poll = %v, want roughly doubled (> %v)", afterSlow, PollInterval*3/2)
		}
		if afterFast > PollInterval*3/2 {
			t.Errorf("gap after the fast poll = %v, want back to ~%v", afterFast, PollInterval)
		}
	})
}

// A test's Cleanup restores PollInterval while a poller it built may still
// be polling.
func TestWorktreePollerKeepsTheIntervalItWasBuiltWith(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var count atomic.Int32
		runner := git.Runner{Git: nonexistentGit, Log: func(git.Entry) { count.Add(1) }}
		p := NewWorktreePoller(runner, t.TempDir(), func(core.ChangeScope) {})
		defer p.Close()
		builtWith := PollInterval
		setInterval(t, &PollInterval, time.Hour)

		p.Focus()
		time.Sleep(2*builtWith + builtWith/2)

		if got := count.Load(); got != 3 {
			t.Errorf("polls = %d, want 3 at the interval the poller was built with", got)
		}
	})
}

func TestBlurStopsPollingAfterTheTimer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var count atomic.Int32
		runner := git.Runner{Git: nonexistentGit, Log: func(git.Entry) { count.Add(1) }}
		p := NewWorktreePoller(runner, t.TempDir(), func(core.ChangeScope) {})
		defer p.Close()

		p.Focus()
		time.Sleep(2*PollInterval + PollInterval/2)
		p.Blur()
		synctest.Wait() // let any poll already in flight finish
		afterBlur := count.Load()
		time.Sleep(2*PollInterval + PollInterval/2)

		if got := count.Load(); got != afterBlur {
			t.Errorf("call count after Blur: %d -> %d, want unchanged", afterBlur, got)
		}
	})
}

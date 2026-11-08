// Requirements: FR-SYNC-07, QA-RESP-01, QA-SAFE-04

package session

import (
	"context"
	"encoding/json"
	"runtime"
	"slices"
	"sync"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/core/coretest"
	"github.com/brogli/gaertli/internal/git/gittest"
	"github.com/brogli/gaertli/internal/jobs"
)

func TestRepoJobsEmitsJobUpdateEventsScopedToTheRepo(t *testing.T) {
	repo := gittest.New(t)
	events := make(chan core.Event, 16)
	m := NewManager(func(e core.Event) { events <- e })

	wt := openSession(t, m, repo.Dir)
	coretest.DrainEvents(events)

	wt.Repo.Jobs().Start(t.Context(), "test", func(ctx context.Context, progress jobs.Progress) (any, error) {
		progress(50, "halfway")
		return nil, nil
	})

	sawProgress, sawDone := false, false
	for !sawProgress || !sawDone {
		{
			e := <-events
			if e.Name != "job:update" {
				continue
			}
			if e.Worktree != "" {
				t.Errorf("job:update Worktree = %q, want empty (repo-scoped)", e.Worktree)
			}
			if e.Repo != wt.Repo.Key {
				t.Errorf("job:update Repo = %q, want %q", e.Repo, wt.Repo.Key)
			}
			var info core.JobInfo
			if err := json.Unmarshal(e.Payload, &info); err != nil {
				t.Fatal(err)
			}
			if info.Progress == 50 {
				sawProgress = true
			}
			if info.State == core.JobDone {
				sawDone = true
			}
		}
	}
}

// A job whose fn takes the worktree lock is no bypass: a concurrent direct
// Lock() waits for it.
func TestJobHoldingWorktreeLockSerialisesWithADirectLock(t *testing.T) {
	wt := &Worktree{}
	m := jobs.NewManager(func(jobs.Job) {})

	var mu sync.Mutex
	var order []string
	record := func(step string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, step)
	}

	locked, release := make(chan struct{}), make(chan struct{})
	m.Start(t.Context(), "test", func(ctx context.Context, progress jobs.Progress) (any, error) {
		wt.Lock()
		defer wt.Unlock()
		close(locked)
		<-release
		record("job released")
		return nil, nil
	})
	<-locked

	attempting, direct := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(direct)
		close(attempting)
		wt.Lock()
		defer wt.Unlock()
		record("direct lock acquired")
	}()
	<-attempting
	// Give a lock that does not serialise the chance to be acquired first.
	for range 1000 {
		runtime.Gosched()
	}
	close(release)
	<-direct

	if want := []string{"job released", "direct lock acquired"}; !slices.Equal(order, want) {
		t.Errorf("order = %v, want %v (direct Lock() must wait for the job)", order, want)
	}
}

// fetch and push touch refs and objects only, so their jobs take no lock.
func TestJobWithNoLockNeverBlocksOnAConcurrentWorktreeLock(t *testing.T) {
	wt := &Worktree{}
	m := jobs.NewManager(func(jobs.Job) {})

	wt.Lock()
	defer wt.Unlock()

	jobDone := make(chan struct{})
	m.Start(t.Context(), "test", func(ctx context.Context, progress jobs.Progress) (any, error) {
		close(jobDone)
		return nil, nil
	})

	<-jobDone
}

// The fn's deferred Unlock runs once ctx.Done() unblocks it.
func TestCancelReleasesAWorktreeLockHeldByTheJob(t *testing.T) {
	wt := &Worktree{}
	m := jobs.NewManager(func(jobs.Job) {})

	locked := make(chan struct{})
	id := m.Start(t.Context(), "test", func(ctx context.Context, progress jobs.Progress) (any, error) {
		wt.Lock()
		defer wt.Unlock()
		close(locked)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	<-locked

	if ok := m.Cancel(id); !ok {
		t.Fatal("Cancel returned false for a running job")
	}

	acquired := make(chan struct{})
	go func() {
		wt.Lock()
		close(acquired)
	}()

	<-acquired
	wt.Unlock()
}

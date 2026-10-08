package session

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git/gittest"
	"github.com/brogli/nebegit/internal/jobs"
)

func TestFR_SYNC_07_RepoJobsEmitsJobUpdateEventsScopedToTheRepo(t *testing.T) {
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
	deadline := time.After(2 * time.Second)
	for !sawProgress || !sawDone {
		select {
		case e := <-events:
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
			if info.State == "done" {
				sawDone = true
			}
		case <-deadline:
			t.Fatal("timed out waiting for job:update progress and terminal events")
		}
	}
}

// A job whose fn takes the worktree lock is no bypass: a concurrent direct
// Lock() waits for it.
func TestQA_SAFE_04_JobHoldingWorktreeLockSerialisesWithADirectLock(t *testing.T) {
	// Real time: a goroutine blocked on the mutex keeps a synctest clock from advancing.
	const hold = 60 * time.Millisecond
	wt := &Worktree{}
	m := jobs.NewManager(func(jobs.Job) {})

	locked := make(chan struct{})
	m.Start(t.Context(), "test", func(ctx context.Context, progress jobs.Progress) (any, error) {
		wt.Lock()
		defer wt.Unlock()
		close(locked)
		time.Sleep(hold)
		return nil, nil
	})
	<-locked
	heldSince := time.Now()
	time.Sleep(10 * time.Millisecond)

	wt.Lock()
	held := time.Since(heldSince)
	wt.Unlock()

	if held < hold/2 {
		t.Errorf("direct Lock() returned %s after the job took the lock, want >= %s (should wait for the job's own hold)", held, hold/2)
	}
}

// fetch and push touch refs and objects only, so their jobs take no lock.
func TestQA_RESP_01_JobWithNoLockNeverBlocksOnAConcurrentWorktreeLock(t *testing.T) {
	wt := &Worktree{}
	m := jobs.NewManager(func(jobs.Job) {})

	wt.Lock()
	defer wt.Unlock()

	jobDone := make(chan struct{})
	m.Start(t.Context(), "test", func(ctx context.Context, progress jobs.Progress) (any, error) {
		close(jobDone)
		return nil, nil
	})

	select {
	case <-jobDone:
	case <-time.After(time.Second):
		t.Fatal("job with no lock never ran while the worktree lock was held")
	}
}

// The fn's deferred Unlock runs once ctx.Done() unblocks it.
func TestQA_SAFE_04_CancelReleasesAWorktreeLockHeldByTheJob(t *testing.T) {
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

	select {
	case <-acquired:
		wt.Unlock()
	case <-time.After(time.Second):
		t.Error("wt.Lock() after Cancel never succeeded — the cancelled job's lock leaked")
	}
}

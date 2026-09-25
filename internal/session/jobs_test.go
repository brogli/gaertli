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

// TestQA_SAFE_04_JobHoldingWorktreeLockSerialisesWithADirectLock proves a
// job whose fn takes the worktree lock is not a bypass: a concurrent direct
// Lock() waits for it, like any other write.
func TestQA_SAFE_04_JobHoldingWorktreeLockSerialisesWithADirectLock(t *testing.T) {
	wt := &Worktree{}
	m := jobs.NewManager(func(jobs.Job) {})

	locked := make(chan struct{})
	m.Start(t.Context(), "test", func(ctx context.Context, progress jobs.Progress) (any, error) {
		wt.Lock()
		defer wt.Unlock()
		close(locked)
		time.Sleep(150 * time.Millisecond)
		return nil, nil
	})
	<-locked
	heldSince := time.Now()
	time.Sleep(10 * time.Millisecond)

	wt.Lock()
	held := time.Since(heldSince)
	wt.Unlock()

	if held < 140*time.Millisecond {
		t.Errorf("direct Lock() returned %s after the job took the lock, want >= 140ms (should wait for the job's own hold)", held)
	}
}

// TestQA_RESP_01_JobWithNoLockNeverBlocksOnAConcurrentWorktreeLock proves
// a job whose fn takes no lock (fetch/push touch refs and objects only)
// runs to completion regardless of a concurrent Worktree.Lock() section.
func TestQA_RESP_01_JobWithNoLockNeverBlocksOnAConcurrentWorktreeLock(t *testing.T) {
	wt := &Worktree{}
	m := jobs.NewManager(func(jobs.Job) {})

	wt.Lock()
	unlockedAt := make(chan time.Duration, 1)
	start := time.Now()
	go func() {
		time.Sleep(150 * time.Millisecond)
		wt.Unlock()
		unlockedAt <- time.Since(start)
	}()

	jobDone := make(chan time.Duration, 1)
	m.Start(t.Context(), "test", func(ctx context.Context, progress jobs.Progress) (any, error) {
		jobDone <- time.Since(start)
		return nil, nil
	})

	select {
	case elapsed := <-jobDone:
		if elapsed > 50*time.Millisecond {
			t.Errorf("job with no lock ran after %s, want well under the worktree lock's 150ms hold", elapsed)
		}
	case <-time.After(time.Second):
		t.Fatal("job with no lock never ran — blocked by an unrelated worktree lock?")
	}

	if elapsed := <-unlockedAt; elapsed < 140*time.Millisecond {
		t.Errorf("the lock-held section finished after only %s, want ~150ms (not shortened by the job)", elapsed)
	}
}

// TestQA_SAFE_04_CancelReleasesAWorktreeLockHeldByTheJob proves cancelling
// a job whose fn holds wt.Lock() releases it: the fn's deferred Unlock runs
// once ctx.Done() unblocks it.
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

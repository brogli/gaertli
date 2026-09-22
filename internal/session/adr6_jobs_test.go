package session

import (
	"context"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/jobs"
)

// TestQA_SAFE_04_JobHoldingWorktreeLockSerialisesWithADirectLock proves a
// job whose fn takes the worktree lock is not a bypass: a concurrent direct
// Lock() waits for it, like any other write.
func TestQA_SAFE_04_JobHoldingWorktreeLockSerialisesWithADirectLock(t *testing.T) {
	wt := &Worktree{}
	m := jobs.NewManager(func(jobs.Job) {})

	locked := make(chan struct{})
	m.Start("test", func(ctx context.Context, progress jobs.Progress) (any, error) {
		wt.Lock()
		defer wt.Unlock()
		close(locked)
		time.Sleep(150 * time.Millisecond)
		return nil, nil
	})
	<-locked
	time.Sleep(10 * time.Millisecond)

	start := time.Now()
	wt.Lock()
	elapsed := time.Since(start)
	wt.Unlock()

	if elapsed < 140*time.Millisecond {
		t.Errorf("direct Lock() returned after %s, want >= 140ms (should wait for the job's own hold)", elapsed)
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
	m.Start("test", func(ctx context.Context, progress jobs.Progress) (any, error) {
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
	id := m.Start("test", func(ctx context.Context, progress jobs.Progress) (any, error) {
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

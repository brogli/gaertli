package jobs

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// updateLog collects onUpdate calls (with arrival time) from whatever
// goroutine a job runs on, safe for a test's own goroutine to read
// concurrently (-race).
type updateLog struct {
	mu    sync.Mutex
	items []timedJob
}

type timedJob struct {
	Job
	at time.Time
}

func (u *updateLog) record(j Job) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.items = append(u.items, timedJob{Job: j, at: time.Now()})
}

func (u *updateLog) snapshot() []Job {
	u.mu.Lock()
	defer u.mu.Unlock()
	jobs := make([]Job, len(u.items))
	for i, it := range u.items {
		jobs[i] = it.Job
	}
	return jobs
}

func (u *updateLog) snapshotTimed() []timedJob {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]timedJob(nil), u.items...)
}

func (u *updateLog) terminalSeen() bool {
	for _, j := range u.snapshot() {
		if isTerminal(j.State) {
			return true
		}
	}
	return false
}

func (u *updateLog) terminalFor(id ID) (Job, bool) {
	for _, j := range u.snapshot() {
		if j.ID == id && isTerminal(j.State) {
			return j, true
		}
	}
	return Job{}, false
}

func TestQA_RESP_01_StartReturnsImmediatelyWithARunningJobRecorded(t *testing.T) {
	release := make(chan struct{})
	var log updateLog
	m := NewManager(log.record)

	start := time.Now()
	id := m.Start("test", func(ctx context.Context, progress Progress) (any, error) {
		<-release
		return nil, nil
	})
	elapsed := time.Since(start)

	if elapsed > 300*time.Millisecond {
		t.Errorf("Start took %s, want well under 300ms", elapsed)
	}
	if id == "" {
		t.Fatal("Start returned an empty id")
	}
	if len(m.List()) != 1 {
		t.Fatalf("List() = %d jobs, want 1", len(m.List()))
	}

	waitFor(t, func() bool { return len(log.snapshot()) >= 1 }, time.Second)
	first := log.snapshot()[0]
	if first.State != Running || first.Percent != -1 {
		t.Errorf("first update = %+v, want State Running, Percent -1", first)
	}

	close(release)
}

func TestFR_SYNC_07_ProgressEmitsImmediatelyThenThrottled(t *testing.T) {
	var mu updateLog
	m := NewManager(mu.record)

	start := time.Now()
	m.Start("test", func(ctx context.Context, progress Progress) (any, error) {
		for range 20 {
			progress(50, "burst")
		}
		time.Sleep(150 * time.Millisecond)
		progress(100, "final")
		return nil, nil
	})

	waitFor(t, func() bool { return mu.terminalSeen() }, 2*time.Second)

	timed := mu.snapshotTimed()
	// timed[0] is the Running transition (always emitted); a progress
	// update with Percent 50 must follow almost immediately (well under
	// the 100ms throttle), proving the burst's first call was not held
	// back.
	var firstBurstAt time.Duration
	progressUpdates := 0
	for _, u := range timed {
		if u.Percent == 50 {
			if progressUpdates == 0 {
				firstBurstAt = u.at.Sub(start)
			}
			progressUpdates++
		}
	}
	if progressUpdates == 0 {
		t.Fatal("no progress update carried Percent 50")
	}
	if firstBurstAt > 50*time.Millisecond {
		t.Errorf("first burst update arrived after %s, want well under 100ms", firstBurstAt)
	}
	if progressUpdates > 2 {
		t.Errorf("got %d updates with Percent 50 during a 50ms burst, want at most 2 (100ms throttle: immediate + at most one more)", progressUpdates)
	}

	last := timed[len(timed)-1]
	if last.State != Done || last.Percent != 100 {
		t.Errorf("last update = %+v, want the terminal Done at 100 regardless of throttle", last.Job)
	}
}

func TestFR_SYNC_07_CancelledJobEndsInCancelledNotFailed(t *testing.T) {
	var mu updateLog
	m := NewManager(mu.record)

	id := m.Start("test", func(ctx context.Context, progress Progress) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})

	waitFor(t, func() bool { return len(mu.snapshot()) >= 1 }, time.Second)
	if ok := m.Cancel(id); !ok {
		t.Fatal("Cancel returned false for a running job")
	}

	waitFor(t, func() bool { return mu.terminalSeen() }, 2*time.Second)

	updates := mu.snapshot()
	terminals := 0
	var last Job
	for _, u := range updates {
		if isTerminal(u.State) {
			terminals++
			last = u
		}
	}
	if terminals != 1 {
		t.Fatalf("got %d terminal updates, want exactly 1", terminals)
	}
	if last.State != Cancelled {
		t.Errorf("final State = %s, want %s", last.State, Cancelled)
	}
	if last.EndedAt.IsZero() {
		t.Error("EndedAt is zero, want it set")
	}
}

func TestQA_RESP_02_CancelUnknownIDReturnsFalseNoPanicNoUpdate(t *testing.T) {
	calls := 0
	m := NewManager(func(Job) { calls++ })

	if ok := m.Cancel("unknown"); ok {
		t.Error("Cancel(\"unknown\") = true, want false")
	}
	if calls != 0 {
		t.Errorf("onUpdate called %d times, want 0", calls)
	}
}

func TestFR_SYNC_07_FnErrorEndsFailedFnNilEndsDone(t *testing.T) {
	var mu updateLog
	m := NewManager(mu.record)

	failID := m.Start("fail", func(ctx context.Context, progress Progress) (any, error) {
		return nil, errors.New("boom")
	})
	okID := m.Start("ok", func(ctx context.Context, progress Progress) (any, error) {
		return nil, nil
	})

	waitFor(t, func() bool {
		f, fok := mu.terminalFor(failID)
		o, ook := mu.terminalFor(okID)
		return fok && f.State == Failed && ook && o.State == Done
	}, 2*time.Second)

	f, _ := mu.terminalFor(failID)
	if f.Err == nil || f.Err.Error() != "boom" {
		t.Errorf("failed job Err = %v, want \"boom\"", f.Err)
	}
}

func TestQA_RESP_01_NoManagerImposedConcurrencyCap(t *testing.T) {
	m := NewManager(func(Job) {})
	block := make(chan struct{})

	m.Start("first", func(ctx context.Context, progress Progress) (any, error) {
		<-block
		return nil, nil
	})
	secondDone := make(chan struct{})
	m.Start("second", func(ctx context.Context, progress Progress) (any, error) {
		close(secondDone)
		return nil, nil
	})

	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("second job never ran while the first was still blocked (a concurrency cap?)")
	}
	waitFor(t, func() bool { return findKind(m.List(), "second").State == Done }, time.Second)
	waitFor(t, func() bool { return findKind(m.List(), "first").State == Running }, time.Second)

	close(block)
}

func TestFR_SYNC_07_StartDedupesAKindAlreadyQueuedOrRunning(t *testing.T) {
	var calls atomic.Int32
	done := make(chan Job, 1)
	m := NewManager(func(j Job) {
		if j.State == Done {
			done <- j
		}
	})

	release := make(chan struct{})
	fn := func(ctx context.Context, progress Progress) (any, error) {
		calls.Add(1)
		<-release
		return nil, nil
	}

	first := m.Start("fetch", fn)
	second := m.Start("fetch", fn)

	if second != first {
		t.Errorf("Start returned a new id %q for a kind already running (%q), want the running job's own id", second, first)
	}

	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("job never finished")
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("fn ran %d times, want 1 (no second process for a kind already running on this repo)", got)
	}
}

func TestFR_UPD_01_StartForBranchRunsOneJobPerBranchOfAKind(t *testing.T) {
	m := NewManager(func(Job) {})
	release := make(chan struct{})
	defer close(release)
	fn := func(ctx context.Context, progress Progress) (any, error) {
		<-release
		return nil, nil
	}

	a := m.StartForBranch("update-branch", "A", fn)
	b := m.StartForBranch("update-branch", "B", fn)
	again := m.StartForBranch("update-branch", "A", fn)

	if a == b {
		t.Errorf("branches A and B share job %q, want one job each", a)
	}
	if again != a {
		t.Errorf("a second start for A returned %q, want the running job %q", again, a)
	}
	for _, j := range m.List() {
		if info := Info(j); info.Kind != "update-branch" || (info.Branch != "A" && info.Branch != "B") {
			t.Errorf("Info = kind %q, branch %q, want update-branch on A or B", info.Kind, info.Branch)
		}
	}
}

func TestQA_RESP_02_ListBoundsFinishedHistoryButNeverDropsAnActiveJob(t *testing.T) {
	m := NewManager(func(Job) {})

	for range historyCap + 3 {
		done := make(chan struct{})
		id := m.Start("fast", func(ctx context.Context, progress Progress) (any, error) {
			close(done)
			return nil, nil
		})
		<-done
		waitFor(t, func() bool { return findID(m.List(), id).State == Done }, time.Second)
	}

	block := make(chan struct{})
	blockedID := m.Start("blocked", func(ctx context.Context, progress Progress) (any, error) {
		<-block
		return nil, nil
	})
	waitFor(t, func() bool { return findID(m.List(), blockedID).State == Running }, time.Second)

	list := m.List()
	finished := 0
	sawBlocked := false
	for _, j := range list {
		if j.ID == blockedID {
			sawBlocked = true
			continue
		}
		if isTerminal(j.State) {
			finished++
		}
	}
	if !sawBlocked {
		t.Error("the still-running job is missing from List(), want it always present")
	}
	if finished > historyCap {
		t.Errorf("List() returned %d finished jobs, want at most %d", finished, historyCap)
	}

	close(block)
}

func findID(list []Job, id ID) Job {
	for _, j := range list {
		if j.ID == id {
			return j
		}
	}
	return Job{}
}

func findKind(list []Job, kind string) Job {
	for _, j := range list {
		if j.Kind == kind {
			return j
		}
	}
	return Job{}
}

func waitFor(t *testing.T, cond func() bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !cond() {
		t.Fatal("condition never became true")
	}
}

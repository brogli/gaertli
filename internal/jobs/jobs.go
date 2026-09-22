// Package jobs runs and tracks background tasks for one repo session.
package jobs

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// State is a job's lifecycle stage.
type State string

const (
	Queued    State = "queued"
	Running   State = "running"
	Done      State = "done"
	Failed    State = "failed"
	Cancelled State = "cancelled"
)

func isTerminal(s State) bool {
	return s == Done || s == Failed || s == Cancelled
}

// Progress reports a job's own status as its function runs. percent is
// 0-100, or -1 while indeterminate (no total known yet, or the underlying
// git output carries no percent at all).
type Progress func(percent int, message string)

// ID identifies one job, unique for its Manager's lifetime.
type ID string

// Job is one job's current state, JSON-serialisable at the core.API
// boundary via core.JobInfo (internal/session converts).
type Job struct {
	ID        ID
	Kind      string
	State     State
	Percent   int
	Message   string
	StartedAt time.Time
	EndedAt   time.Time // zero until State is terminal
	Err       error     // set only when State is Failed
	Result    any       // set from fn's first return value when State ends Done; nil otherwise
}

// progressInterval throttles progress emission (same value and reasoning as
// internal/services's own log:progress throttle).
const progressInterval = 100 * time.Millisecond

// historyCap bounds finished jobs kept for the UI; a queued or running job
// is never trimmed regardless of count.
const historyCap = 20

// Manager runs and tracks jobs for one repo session: a job's repo scope is
// which Manager you call, not a parameter to Start.
//
// It has no concurrency cap: what may run at once is decided by the locks
// a job's fn takes. Manager only tracks state, progress, cancellation, and
// history.
type Manager struct {
	onUpdate func(Job)

	mu    sync.Mutex
	jobs  map[ID]*entry
	order []ID // insertion order; List returns oldest-first
	next  int
}

type entry struct {
	job      Job
	cancel   context.CancelFunc
	lastEmit time.Time
}

// NewManager builds a Manager that reports every job's changes to onUpdate.
func NewManager(onUpdate func(Job)) *Manager {
	return &Manager{onUpdate: onUpdate, jobs: make(map[ID]*entry)}
}

// Start runs fn in its own goroutine under a fresh context, detached from
// any request context (a job outlives the RPC call that started it). Returns
// immediately with the job's id; fn's own return decides the terminal
// state: nil error -> Done (its first return value stored as Result), a
// plain error -> Failed (Err set), ctx cancelled via this job's own Cancel
// -> Cancelled, classified by ctx.Err() after fn returns.
//
// A kind already queued or running on this Manager is not started twice:
// Start returns that job's id instead of launching a second process (two
// fetches of one repo at once only race git's ref locks, so a second one is
// redundant).
func (m *Manager) Start(kind string, fn func(ctx context.Context, progress Progress) (any, error)) ID {
	m.mu.Lock()
	if id, ok := m.activeKindLocked(kind); ok {
		m.mu.Unlock()
		return id
	}

	m.next++
	id := ID(strconv.Itoa(m.next))
	ctx, cancel := context.WithCancel(context.Background())
	m.jobs[id] = &entry{
		job:    Job{ID: id, Kind: kind, State: Queued, Percent: -1, StartedAt: time.Now()},
		cancel: cancel,
	}
	m.order = append(m.order, id)
	m.mu.Unlock()

	go m.run(id, ctx, fn)
	return id
}

// activeKindLocked returns the id of a queued or running job of kind, if
// any. m.mu must be held.
func (m *Manager) activeKindLocked(kind string) (ID, bool) {
	for _, id := range m.order {
		if e := m.jobs[id]; e.job.Kind == kind && !isTerminal(e.job.State) {
			return id, true
		}
	}
	return "", false
}

func (m *Manager) run(id ID, ctx context.Context, fn func(context.Context, Progress) (any, error)) {
	m.setState(id, Running)
	result, err := fn(ctx, func(percent int, message string) { m.updateProgress(id, percent, message) })
	m.finish(id, ctx, result, err)
}

func (m *Manager) setState(id ID, state State) {
	job, ok := m.update(id, func(e *entry) { e.job.State = state })
	if ok {
		m.onUpdate(job)
	}
}

// updateProgress applies percent/message, throttled: the first call for a
// job emits immediately, subsequent calls at most once per progressInterval.
func (m *Manager) updateProgress(id ID, percent int, message string) {
	var emit bool
	job, ok := m.update(id, func(e *entry) {
		e.job.Percent, e.job.Message = percent, message
		emit = e.lastEmit.IsZero() || time.Since(e.lastEmit) >= progressInterval
		if emit {
			e.lastEmit = time.Now()
		}
	})
	if ok && emit {
		m.onUpdate(job)
	}
}

// finish classifies fn's outcome, trims history, and emits unconditionally,
// regardless of the progress throttle.
func (m *Manager) finish(id ID, ctx context.Context, result any, err error) {
	m.mu.Lock()
	e, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return
	}
	e.job.EndedAt = time.Now()
	switch {
	case ctx.Err() != nil:
		e.job.State = Cancelled
	case err != nil:
		e.job.State, e.job.Err = Failed, err
	default:
		e.job.State, e.job.Result = Done, result
	}
	job := e.job
	m.trimHistoryLocked()
	m.mu.Unlock()

	m.onUpdate(job)
}

// trimHistoryLocked drops the oldest finished jobs beyond historyCap,
// newest-first; a still-queued/running job is never counted or dropped.
// m.mu must be held.
func (m *Manager) trimHistoryLocked() {
	finished := 0
	for i := len(m.order) - 1; i >= 0; i-- {
		id := m.order[i]
		if !isTerminal(m.jobs[id].job.State) {
			continue
		}
		finished++
		if finished > historyCap {
			delete(m.jobs, id)
			m.order = append(m.order[:i], m.order[i+1:]...)
		}
	}
}

// update applies mutate to id's entry under m.mu and returns its job
// snapshot; ok is always true in practice — every caller here runs on a
// job's own goroutine after Start already registered its entry.
func (m *Manager) update(id ID, mutate func(*entry)) (Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.jobs[id]
	if !ok {
		return Job{}, false
	}
	mutate(e)
	return e.job, true
}

// Cancel cancels id's context if it is still running. ok is false for an
// unknown or already-finished id.
func (m *Manager) Cancel(id ID) bool {
	m.mu.Lock()
	e, ok := m.jobs[id]
	if !ok || isTerminal(e.job.State) {
		m.mu.Unlock()
		return false
	}
	m.mu.Unlock()

	e.cancel()
	return true
}

// List returns every job Manager knows about, oldest first.
func (m *Manager) List() []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := make([]Job, len(m.order))
	for i, id := range m.order {
		list[i] = m.jobs[id].job
	}
	return list
}

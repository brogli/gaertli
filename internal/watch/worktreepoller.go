package watch

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/brogli/nebegit/internal/git"
)

// StatusArgs is the one status call shape: the poll here and the changes
// list read the same records.
var StatusArgs = []string{"status", "--porcelain=v2", "-z", "--branch", "--untracked-files=all"}

// WorktreePoller polls one worktree's git status while its window has
// focus. It compares Result.Stdout's raw bytes only.
type WorktreePoller struct {
	runner   git.Runner
	dir      string
	onChange func(Scope)

	mu       sync.Mutex
	focused  bool
	interval time.Duration
	inFlight bool
	baseline []byte
	haveBase bool
	timer    *time.Timer
}

// NewWorktreePoller builds a poller for dir, unfocused until Focus is
// called.
func NewWorktreePoller(runner git.Runner, dir string, onChange func(Scope)) *WorktreePoller {
	return &WorktreePoller{runner: runner, dir: dir, onChange: onChange, interval: PollInterval}
}

// Focus establishes a baseline with one immediate CheckNow (no onChange on
// this first call — nothing to compare against yet) and starts a timer at
// PollInterval. Calling Focus while already focused is a no-op.
func (p *WorktreePoller) Focus() {
	p.mu.Lock()
	if p.focused {
		p.mu.Unlock()
		return
	}
	p.focused = true
	p.interval = PollInterval
	p.mu.Unlock()

	p.pollAndAdjust()
	p.scheduleNext()
}

// Blur stops the timer: a backgrounded window's poller makes no git calls
// at all.
func (p *WorktreePoller) Blur() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.focused = false
	p.stopTimer()
}

func (p *WorktreePoller) stopTimer() {
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
}

func (p *WorktreePoller) scheduleNext() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.focused {
		return
	}
	p.timer = time.AfterFunc(p.interval, p.tick)
}

// tick runs a poll and backs off while status is slow.
func (p *WorktreePoller) tick() {
	p.pollAndAdjust()
	p.scheduleNext()
}

func (p *WorktreePoller) pollAndAdjust() {
	start := time.Now()
	p.CheckNow()
	elapsed := time.Since(start)

	p.mu.Lock()
	if elapsed > p.interval {
		p.interval = min(p.interval*2, MaxPollInterval)
	} else {
		p.interval = PollInterval
	}
	p.mu.Unlock()
}

// CheckNow runs one immediate status poll outside the timer schedule. Used
// by Focus and by Refresh (services.go). Never runs two polls concurrently
// (a poll already in flight makes this a no-op).
func (p *WorktreePoller) CheckNow() {
	if !p.startPoll() {
		return
	}
	defer p.endPoll()

	result, err := p.runner.Run(context.Background(), git.Call{
		Dir: p.dir, ReadOnly: true, Background: true, Args: StatusArgs,
	})
	if err != nil {
		return
	}

	p.mu.Lock()
	changed := p.haveBase && !bytes.Equal(p.baseline, result.Stdout)
	p.baseline = result.Stdout
	p.haveBase = true
	p.mu.Unlock()

	if changed {
		p.onChange(Index)
	}
}

func (p *WorktreePoller) startPoll() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.inFlight {
		return false
	}
	p.inFlight = true
	return true
}

func (p *WorktreePoller) endPoll() {
	p.mu.Lock()
	p.inFlight = false
	p.mu.Unlock()
}

// Close stops the poller.
func (p *WorktreePoller) Close() {
	p.Blur()
}

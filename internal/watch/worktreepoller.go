package watch

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

// StatusArgs is the one status call shape: the poll here and the changes
// list read the same records.
var StatusArgs = []string{"status", "--porcelain=v2", "-z", "--branch", "--untracked-files=all"}

// WorktreePoller polls one worktree's git status while its window has
// focus. It compares the status output plus the size and modification time
// of every file whose working-tree side is changed: a further edit to a
// file already listed leaves the status output identical.
type WorktreePoller struct {
	runner   git.Runner
	dir      string
	onChange func(core.ChangeScope)

	inFlight atomic.Bool

	baseInterval time.Duration

	mu       sync.Mutex
	focused  bool
	interval time.Duration
	baseline []byte
	haveBase bool
	timer    *time.Timer
}

// NewWorktreePoller builds a poller for dir that polls at the PollInterval
// of this moment, unfocused until Focus is called.
func NewWorktreePoller(runner git.Runner, dir string, onChange func(core.ChangeScope)) *WorktreePoller {
	return &WorktreePoller{runner: runner, dir: dir, onChange: onChange, baseInterval: PollInterval, interval: PollInterval}
}

// Focus runs one immediate CheckNow and starts the poll timer.
// The first poll ever only sets the baseline: nothing to compare against
// yet. Calling Focus while already focused is a no-op.
func (p *WorktreePoller) Focus() {
	p.mu.Lock()
	if p.focused {
		p.mu.Unlock()
		return
	}
	p.focused = true
	p.interval = p.baseInterval
	p.mu.Unlock()

	p.tick()
}

// Blur stops the timer, so an unfocused worktree is polled only on
// Refresh.
func (p *WorktreePoller) Blur() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.focused = false
	if p.timer != nil {
		p.timer.Stop()
	}
}

// tick runs a poll, backs off while status is slow, and schedules the next
// poll while focused.
func (p *WorktreePoller) tick() {
	start := time.Now()
	p.CheckNow()
	elapsed := time.Since(start)

	p.mu.Lock()
	defer p.mu.Unlock()
	if elapsed > p.interval {
		p.interval = min(p.interval*2, MaxPollInterval)
	} else {
		p.interval = p.baseInterval
	}
	if p.focused {
		p.timer = time.AfterFunc(p.interval, p.tick)
	}
}

// CheckNow runs one status poll now, outside the timer schedule; a no-op
// while a poll is in flight.
func (p *WorktreePoller) CheckNow() {
	if !p.inFlight.CompareAndSwap(false, true) {
		return
	}
	defer p.inFlight.Store(false)

	result, err := p.runner.Run(context.Background(), git.Call{
		Dir: p.dir, ReadOnly: true, Background: true, Args: StatusArgs,
	})
	if err != nil {
		return
	}

	fp := fingerprint(p.dir, result.Stdout)
	p.mu.Lock()
	changed := p.haveBase && !bytes.Equal(p.baseline, fp)
	p.baseline = fp
	p.haveBase = true
	p.mu.Unlock()

	if changed {
		p.onChange(core.IndexChanged)
	}
}

// Close stops the poller.
func (p *WorktreePoller) Close() {
	p.Blur()
}

// fingerprint is status's porcelain v2 -z output followed by the size and
// modification time of each path whose working-tree side is changed
// (modified, unmerged, untracked); a path that can't be read counts as gone.
func fingerprint(dir string, status []byte) []byte {
	out := bytes.Clone(status)
	for _, path := range worktreeChangedPaths(status) {
		info, err := os.Lstat(filepath.Join(dir, path))
		if err != nil {
			out = append(out, "\x00gone"...)
			continue
		}
		out = fmt.Appendf(out, "\x00%d %d", info.Size(), info.ModTime().UnixNano())
	}
	return out
}

// worktreeChangedPaths picks from porcelain v2 -z records the paths whose
// working-tree side differs: an ordinary or rename record with a Y status,
// every unmerged and every untracked record.
func worktreeChangedPaths(status []byte) []string {
	records := strings.Split(strings.TrimSuffix(string(status), "\x00"), "\x00")
	var paths []string
	for i := 0; i < len(records); i++ {
		rec := records[i]
		switch {
		case strings.HasPrefix(rec, "1 "):
			if f := strings.SplitN(rec[2:], " ", 8); len(f) == 8 && f[0][1] != '.' {
				paths = append(paths, f[7])
			}
		case strings.HasPrefix(rec, "2 "):
			if f := strings.SplitN(rec[2:], " ", 9); len(f) == 9 && f[0][1] != '.' {
				paths = append(paths, f[8])
			}
			i++ // the original path is the next record
		case strings.HasPrefix(rec, "u "):
			if f := strings.SplitN(rec[2:], " ", 10); len(f) == 10 {
				paths = append(paths, f[9])
			}
		case strings.HasPrefix(rec, "? "):
			paths = append(paths, rec[2:])
		}
	}
	return paths
}

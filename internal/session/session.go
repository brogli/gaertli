// Package session mints and reuses per-worktree and per-repo sessions from
// a validated path (ADR-9).
package session

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

// logCap bounds Worktree's command-log ring buffer.
const logCap = 500

// Worktree is a per-window session (ADR-9). Thin here: only identity and a
// git.Runner scoped to it. Later specs (60+, 70) add status/index/conflict
// state, a write queue, a message draft.
type Worktree struct {
	ID     core.WorktreeID
	Path   string
	GitDir string
	Repo   *Repo
	Runner git.Runner // Log emits a "command" event scoped to ID and appends to log

	mu  sync.Mutex
	log []core.CommandLogEntry // ring buffer, capped at logCap
}

// CommandLog returns a copy of the buffered entries, oldest first.
func (w *Worktree) CommandLog() []core.CommandLogEntry {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]core.CommandLogEntry(nil), w.log...)
}

func (w *Worktree) append(e core.CommandLogEntry) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.log = append(w.log, e)
	if len(w.log) > logCap {
		w.log = w.log[len(w.log)-logCap:]
	}
}

// Repo is a per-repo session (ADR-9), shared by every Worktree with the same
// Key. Thin here: later specs (10, 12, 70) add the log/graph/refs cache, the
// watcher, the repo-level write queue.
type Repo struct{ Key string }

// Manager mints and reuses sessions: opening the same worktree twice (by
// any path that resolves to it, FR-REPO-01) returns the same *Worktree, and
// worktrees sharing a repo share one *Repo.
type Manager struct {
	sink     core.EventSink
	discover git.Runner // no Log: discovery calls predate any session

	mu        sync.Mutex
	worktrees map[core.WorktreeID]*Worktree
	repos     map[string]*Repo
}

// NewManager builds a Manager that reports command events to sink.
func NewManager(sink core.EventSink) *Manager {
	return &Manager{
		sink:      sink,
		worktrees: make(map[core.WorktreeID]*Worktree),
		repos:     make(map[string]*Repo),
	}
}

// Open validates path and returns its worktree session, minting one on
// first use. Errors are *core.Error:
//   - Code GitTooOld: from Runner.CheckVersion (git absent, unparsable, or
//     older than MinVersion — spec 02)
//   - Code NotARepo: rev-parse failed (bad Dir, or not inside a work tree)
//     or --is-inside-work-tree printed anything but "true"
func (m *Manager) Open(ctx context.Context, path core.RepoPath) (*Worktree, error) {
	if _, err := m.discover.CheckVersion(ctx); err != nil {
		return nil, err
	}

	result, err := m.discover.Run(ctx, git.Call{
		Dir:      string(path),
		ReadOnly: true,
		Args:     []string{"rev-parse", "--path-format=absolute", "--show-toplevel", "--git-dir", "--git-common-dir", "--is-inside-work-tree"},
	})
	if err != nil {
		return nil, remapNotARepo(err)
	}

	lines := strings.Split(strings.TrimSpace(string(result.Stdout)), "\n")
	if len(lines) != 4 || lines[3] != "true" {
		return nil, core.Errorf(core.NotARepo, "not a git repository")
	}
	return m.session(lines[0], lines[1], lines[2]), nil
}

// Get returns an already-open session. core.Errorf(core.Internal, ...) for
// an unknown id — same convention as app.Router.CoreOf (spec 01).
func (m *Manager) Get(wt core.WorktreeID) (*Worktree, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w, ok := m.worktrees[wt]
	if !ok {
		return nil, core.Errorf(core.Internal, "unknown worktree %q", wt)
	}
	return w, nil
}

// session returns the existing worktree for toplevel, minting one (and its
// repo, reused by key) on first use.
func (m *Manager) session(toplevel, gitDir, repoKey string) *Worktree {
	id := core.WorktreeID(toplevel)

	m.mu.Lock()
	defer m.mu.Unlock()

	if wt, ok := m.worktrees[id]; ok {
		return wt
	}

	repo, ok := m.repos[repoKey]
	if !ok {
		repo = &Repo{Key: repoKey}
		m.repos[repoKey] = repo
	}

	wt := &Worktree{ID: id, Path: toplevel, GitDir: gitDir, Repo: repo}
	wt.Runner = git.Runner{Log: func(e git.Entry) {
		entry := toCommandLogEntry(e)
		wt.append(entry)
		m.sink(core.Event{Name: "command", Worktree: id, Payload: marshalEntry(entry)})
	}}
	m.worktrees[id] = wt
	return wt
}

// remapNotARepo turns a discovery failure into Code NotARepo, keeping
// Message/GitStderr/Command (the same remap convention CheckVersion already
// established for GitTooOld).
func remapNotARepo(err error) error {
	var src *core.Error
	if errors.As(err, &src) {
		return &core.Error{Code: core.NotARepo, Message: src.Message, GitStderr: src.GitStderr, Command: src.Command}
	}
	return core.Errorf(core.NotARepo, "%v", err)
}

// toCommandLogEntry converts a git.Entry into the JSON DTO buffered and
// emitted for it.
func toCommandLogEntry(e git.Entry) core.CommandLogEntry {
	entry := core.CommandLogEntry{
		At:         time.Now().Format(time.RFC3339Nano),
		Command:    e.Command,
		Dir:        e.Dir,
		Background: e.Background,
		DurationMS: e.Duration.Milliseconds(),
		ExitCode:   e.Result.ExitCode,
		Stderr:     string(e.Result.Stderr),
	}
	var ce *core.Error
	if errors.As(e.Err, &ce) {
		entry.ErrorCode = string(ce.Code)
	}
	return entry
}

func marshalEntry(entry core.CommandLogEntry) json.RawMessage {
	b, err := json.Marshal(entry)
	if err != nil {
		return nil
	}
	return b
}

// Package session mints and reuses per-worktree and per-repo sessions from
// a validated path.
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
	"github.com/brogli/nebegit/internal/jobs"
	"github.com/brogli/nebegit/internal/watch"
)

// logCap bounds Worktree's command-log ring buffer.
const logCap = 500

// repoWatcher is the part of *watch.RepoWatcher session needs (testability,
// same pattern as app.shellWindow).
type repoWatcher interface {
	CheckNow()
	Close() error
}

// worktreePoller is the part of *watch.WorktreePoller session needs.
type worktreePoller interface {
	Focus()
	Blur()
	CheckNow()
	Close()
}

// Worktree is a per-window session: identity, a git.Runner scoped to it,
// and the per-window log state below.
type Worktree struct {
	ID     core.WorktreeID
	Path   string
	GitDir string
	Repo   *Repo
	Runner git.Runner // Log emits a "command" event scoped to ID and appends to log

	poller worktreePoller // the working-file half of outside-change detection, focused while the window is

	// writeMu serialises everything that writes this worktree's index or
	// files: stage, commit, reset, checkout, cherry-pick, revert,
	// merge/rebase steps, discard.
	writeMu sync.Mutex

	mu  sync.Mutex
	log []core.CommandLogEntry // ring buffer, capped at logCap

	// messageMu guards messageDraft apart from writeMu, so the draft never
	// waits on a running git write.
	messageMu    sync.Mutex
	messageDraft string

	// The window's own filtered log view, apart from the repo's shared log.
	filterMu     sync.Mutex
	filterActive bool
	filterFlat   bool
	filterCancel context.CancelFunc
	filterGen    int
	filterRows   []core.LogCommit
	filterDone   bool
	filterErr    *core.Error
}

// Poller is the worktree's status poller (internal/services.Refresh/SetFocused).
func (w *Worktree) Poller() worktreePoller { return w.poller }

// Lock/Unlock serialise this worktree's own writes (the per-worktree
// queue), held for the duration of one write call.
func (w *Worktree) Lock()   { w.writeMu.Lock() }
func (w *Worktree) Unlock() { w.writeMu.Unlock() }

// SetMessageDraft replaces the stored commit message draft, shared by
// every window currently selecting this worktree.
func (w *Worktree) SetMessageDraft(message string) {
	w.messageMu.Lock()
	defer w.messageMu.Unlock()
	w.messageDraft = message
}

// MessageDraft returns the stored commit message draft, "" if none.
func (w *Worktree) MessageDraft() string {
	w.messageMu.Lock()
	defer w.messageMu.Unlock()
	return w.messageDraft
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

// Repo is a per-repo session, shared by every Worktree with the same Key.
type Repo struct {
	Key string

	watcher repoWatcher   // the git-state half of outside-change detection
	jobs    *jobs.Manager // remote jobs

	// writeMu serialises this repo's writes that touch no worktree's index
	// or files: worktree add/remove/prune, branch create (without checkout),
	// rename, delete.
	writeMu sync.Mutex

	mu       sync.Mutex
	log      []core.LogCommit
	logDone  bool
	logErr   *core.Error
	loading  bool
	tips     map[string]string // ref-tip snapshot recorded when the log last finished
	gen      int               // bumped by LogBeginLoad/LogBeginReload
	refcount int               // open worktrees sharing this repo
}

// Watcher is the repo's git-dir watcher (internal/services.Refresh).
func (r *Repo) Watcher() repoWatcher { return r.watcher }

// Jobs is the repo's job manager (internal/services.Jobs/CancelJob).
func (r *Repo) Jobs() *jobs.Manager { return r.jobs }

// Lock/Unlock serialise this repo's write one-offs (the per-repo queue),
// held for the duration of one write call.
func (r *Repo) Lock()   { r.writeMu.Lock() }
func (r *Repo) Unlock() { r.writeMu.Unlock() }

// acquire records one more worktree session referencing r.
func (r *Repo) acquire() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refcount++
}

// release drops one worktree session referencing r, reporting whether none
// remain (the caller then tears r down).
func (r *Repo) release() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refcount--
	return r.refcount == 0
}

// LogBeginLoad marks a load started, returning the generation it started
// (threaded through streamLog to LogAppend/LogFinish). ok is false (no-op
// for the caller) if a load is already running or has already finished —
// LoadLog's idempotency.
func (r *Repo) LogBeginLoad() (gen int, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.loading || r.logDone {
		return r.gen, false
	}
	r.loading = true
	return r.gen, true
}

// LogBeginReload starts a load that replaces whatever is cached: ok is
// false only if a load is currently running (never "already finished" —
// that is the case this method exists for). Resets log/logDone/logErr and
// bumps gen, so a still-running superseded stream's late
// LogAppend/LogFinish calls (carrying the old gen) are dropped.
func (r *Repo) LogBeginReload() (gen int, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.loading {
		return 0, false
	}
	r.loading = true
	r.log = nil
	r.logDone = false
	r.logErr = nil
	r.gen++
	return r.gen, true
}

// LogAppend adds rows as they stream in; a no-op if gen is stale (a
// superseded stream still draining).
func (r *Repo) LogAppend(gen int, rows ...core.LogCommit) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if gen != r.gen {
		return
	}
	r.log = append(r.log, rows...)
}

// LogFinish marks the load complete; err is non-nil on failure. tips is the
// ref-tip snapshot recorded for the next ReloadLog's comparison. A no-op if
// gen is stale.
func (r *Repo) LogFinish(gen int, err *core.Error, tips map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if gen != r.gen {
		return
	}
	r.loading = false
	r.logDone = true
	r.logErr = err
	r.tips = tips
}

// LogTips is the ref-tip snapshot recorded when the log last finished
// loading, nil until then.
func (r *Repo) LogTips() map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tips
}

// LogReady reports whether a reload is meaningful: a load has finished and
// none is currently running.
func (r *Repo) LogReady() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.logDone && !r.loading
}

// LogView is one snapshot of a log's rows: Repo's own shared cache
// (Repo.LogSnapshot) or a window's filtered view (Worktree.FilterSnapshot).
// Generation bumps whenever that view restarts from row 0 (a reload, a new
// filter); Flat is true only for a text/author search.
type LogView struct {
	Rows       []core.LogCommit
	Loaded     int
	Done       bool
	Flat       bool
	Generation int
	Err        *core.Error
}

// LogSnapshot returns a snapshot of rows [from, to), clamped to
// [0, len(log)), the current loaded count, and whether the load has
// finished.
func (r *Repo) LogSnapshot(from, to int) LogView {
	r.mu.Lock()
	defer r.mu.Unlock()
	return LogView{
		Rows:       window(r.log, from, to),
		Loaded:     len(r.log),
		Done:       r.logDone,
		Generation: r.gen,
		Err:        r.logErr,
	}
}

// window copies rows [from, to), clamped to [0, len(rows)).
func window(rows []core.LogCommit, from, to int) []core.LogCommit {
	from = min(max(from, 0), len(rows))
	to = min(max(to, from), len(rows))
	return append([]core.LogCommit(nil), rows[from:to]...)
}

// LogIndexOf returns sha's row index in the currently loaded log and
// whether it was found: a linear scan, regardless of r.gen. A racing reload
// can make the result momentarily stale; the frontend's next LogRows call
// corrects it.
func (r *Repo) LogIndexOf(sha string) (index int, found bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, c := range r.log {
		if c.SHA == sha {
			return i, true
		}
	}
	return 0, false
}

// FilterBegin starts a new filtered load, cancelling whatever filtered load
// is running. The returned generation stamps this load's
// FilterAppend/FilterFinish calls, so a late call from the cancelled load is
// dropped even before its cancel takes effect.
func (w *Worktree) FilterBegin(flat bool, cancel context.CancelFunc) (gen int) {
	w.filterMu.Lock()
	defer w.filterMu.Unlock()
	if w.filterCancel != nil {
		w.filterCancel()
	}
	w.filterGen++
	w.filterActive, w.filterFlat, w.filterCancel = true, flat, cancel
	w.filterRows, w.filterDone, w.filterErr = nil, false, nil
	return w.filterGen
}

// FilterAppend adds rows as a filtered load streams in; a no-op if gen is
// stale (a superseded stream still draining).
func (w *Worktree) FilterAppend(gen int, rows ...core.LogCommit) {
	w.filterMu.Lock()
	defer w.filterMu.Unlock()
	if gen != w.filterGen {
		return
	}
	w.filterRows = append(w.filterRows, rows...)
}

// FilterFinish marks a filtered load complete; err is non-nil on failure. A
// no-op if gen is stale.
func (w *Worktree) FilterFinish(gen int, err *core.Error) {
	w.filterMu.Lock()
	defer w.filterMu.Unlock()
	if gen != w.filterGen {
		return
	}
	w.filterDone, w.filterErr = true, err
}

// LogSnapshot is what this window shows: its own filtered view while a
// filter is active, the repo's shared log otherwise.
func (w *Worktree) LogSnapshot(from, to int) LogView {
	if view, ok := w.FilterSnapshot(from, to); ok {
		return view
	}
	return w.Repo.LogSnapshot(from, to)
}

// FilterSnapshot mirrors Repo.LogSnapshot, scoped to this window's own
// filtered view. ok is false when no filter is active.
func (w *Worktree) FilterSnapshot(from, to int) (view LogView, ok bool) {
	w.filterMu.Lock()
	defer w.filterMu.Unlock()
	if !w.filterActive {
		return LogView{}, false
	}
	return LogView{
		Rows:       window(w.filterRows, from, to),
		Loaded:     len(w.filterRows),
		Done:       w.filterDone,
		Flat:       w.filterFlat,
		Generation: w.filterGen,
		Err:        w.filterErr,
	}, true
}

// FilterClear cancels any running filtered stream and reverts LogRows to
// wt.Repo's shared cache — instant, no git call.
func (w *Worktree) FilterClear() {
	w.filterMu.Lock()
	defer w.filterMu.Unlock()
	if w.filterCancel != nil {
		w.filterCancel()
	}
	w.filterActive, w.filterCancel = false, nil
	w.filterRows, w.filterDone, w.filterErr = nil, false, nil
}

// Manager mints and reuses sessions: opening the same worktree twice (by
// any path that resolves to it) returns the same *Worktree, and worktrees
// sharing a repo share one *Repo.
type Manager struct {
	sink     core.EventSink
	discover git.Runner // no Log: discovery calls predate any session

	newWatcher func(commonDir string, onChange func(watch.Scope)) (repoWatcher, error)
	newPoller  func(runner git.Runner, dir string, onChange func(watch.Scope)) worktreePoller

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
		newWatcher: func(commonDir string, onChange func(watch.Scope)) (repoWatcher, error) {
			return watch.NewRepoWatcher(commonDir, onChange)
		},
		newPoller: func(runner git.Runner, dir string, onChange func(watch.Scope)) worktreePoller {
			return watch.NewWorktreePoller(runner, dir, onChange)
		},
	}
}

// Resolve validates path and returns its worktree's identity (toplevel, git
// dir, repo key) without minting or reusing any session: Open's stateless
// half, so listing a repo's worktrees or branches never starts a poller or
// watcher for a repo the panel has only expanded, not selected. Errors are
// exactly Open's own:
//   - Code GitTooOld: from Runner.CheckVersion (git absent, unparsable, or
//     older than MinVersion)
//   - Code NotARepo: rev-parse failed (bad Dir, or not inside a work tree)
//     or --is-inside-work-tree printed anything but "true"
func (m *Manager) Resolve(ctx context.Context, path core.RepoPath) (toplevel, gitDir, repoKey string, err error) {
	if _, err := m.discover.CheckVersion(ctx); err != nil {
		return "", "", "", err
	}

	result, err := m.discover.Run(ctx, git.Call{
		Dir:      string(path),
		ReadOnly: true,
		Args:     []string{"rev-parse", "--path-format=absolute", "--show-toplevel", "--git-dir", "--git-common-dir", "--is-inside-work-tree"},
	})
	if err != nil {
		return "", "", "", remapNotARepo(err, path)
	}

	lines := strings.Split(strings.TrimSpace(string(result.Stdout)), "\n")
	if len(lines) != 4 || lines[3] != "true" {
		return "", "", "", notARepo(path)
	}
	return lines[0], lines[1], lines[2], nil
}

// Open resolves path, then mints or reuses its session.
func (m *Manager) Open(ctx context.Context, path core.RepoPath) (*Worktree, error) {
	toplevel, gitDir, repoKey, err := m.Resolve(ctx, path)
	if err != nil {
		return nil, err
	}
	return m.session(toplevel, gitDir, repoKey), nil
}

// Get returns an already-open session. core.Errorf(core.Internal, ...) for
// an unknown id — same convention as app.Router.CoreOf.
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
		repo = m.newRepo(repoKey)
		m.repos[repoKey] = repo
	}
	repo.acquire()

	wt := m.newWorktree(id, toplevel, gitDir, repo)
	m.worktrees[id] = wt
	return wt
}

// newRepo mints repo's session and starts its watcher: a start failure is
// not fatal (watch.NewRepoWatcher's own doc) — the repo still opens,
// running on the fallback poll alone.
func (m *Manager) newRepo(key string) *Repo {
	repo := &Repo{Key: key}
	repo.watcher, _ = m.newWatcher(key, func(scope watch.Scope) {
		m.sink(core.Event{Name: "repo:changed", Repo: key, Payload: changedPayload(scope)})
	})
	repo.jobs = jobs.NewManager(func(j jobs.Job) {
		m.sink(core.Event{Name: "job:update", Repo: key, Payload: payload(jobs.Info(j))})
	})
	return repo
}

// newWorktree mints wt's session and focuses its status poller immediately:
// simpler and more robust than waiting for a first WindowFocus event, whose
// timing on window creation is not guaranteed.
func (m *Manager) newWorktree(id core.WorktreeID, toplevel, gitDir string, repo *Repo) *Worktree {
	wt := &Worktree{ID: id, Path: toplevel, GitDir: gitDir, Repo: repo}
	wt.Runner = git.Runner{Log: func(e git.Entry) {
		entry := toCommandLogEntry(e)
		wt.append(entry)
		m.sink(core.Event{Name: "command", Worktree: id, Payload: payload(entry)})
	}}
	wt.poller = m.newPoller(wt.Runner, toplevel, func(scope watch.Scope) {
		m.sink(core.Event{Name: "repo:changed", Worktree: id, Payload: changedPayload(scope)})
	})
	wt.poller.Focus()
	return wt
}

// changedPayload is a "repo:changed" event's payload.
func changedPayload(scope watch.Scope) json.RawMessage {
	return payload(core.RepoChanged{Scope: string(scope)})
}

// payload marshals an event's payload; nil if v cannot be marshalled.
func payload(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// Close ends wt's session: stops its poller, and, if wt was the last
// worktree referencing its repo, stops the repo's watcher and drops the
// repo session. A no-op for an unknown id.
func (m *Manager) Close(id core.WorktreeID) {
	m.mu.Lock()
	wt, ok := m.worktrees[id]
	if ok {
		delete(m.worktrees, id)
	}
	m.mu.Unlock()
	if !ok {
		return
	}

	wt.poller.Close()

	if wt.Repo.release() {
		m.mu.Lock()
		delete(m.repos, wt.Repo.Key)
		m.mu.Unlock()
		_ = wt.Repo.watcher.Close()
	}
}

// remapNotARepo turns a discovery failure into Code NotARepo with
// notARepo's own message, keeping GitStderr/Command for the details.
func remapNotARepo(err error, path core.RepoPath) error {
	if src, ok := errors.AsType[*core.Error](err); ok {
		src.Code, src.Message = core.NotARepo, notARepo(path).Message
		return src
	}
	return notARepo(path)
}

// notARepo builds the message: the folder, and that it is not a repo.
func notARepo(path core.RepoPath) *core.Error {
	return core.Errorf(core.NotARepo, "%s is not a git repository", path)
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
	if ce, ok := errors.AsType[*core.Error](e.Err); ok {
		entry.ErrorCode = string(ce.Code)
	}
	return entry
}

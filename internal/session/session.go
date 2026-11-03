// Package session mints and reuses per-worktree and per-repo sessions from
// a validated path.
package session

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/jobs"
	"github.com/brogli/nebegit/internal/watch"
)

// repoWatcher is the part of *watch.RepoWatcher session uses; tests fake it.
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

// Worktree is one worktree's session, shared by every window selecting it:
// identity, a git.Runner scoped to it, and the filtered log state below.
type Worktree struct {
	ID     core.WorktreeID
	Path   string
	GitDir string
	Repo   *Repo
	Runner git.Runner // Log emits a "command" event scoped to ID

	poller worktreePoller // the working-file half of outside-change detection, focused while any window on this worktree is

	// writeMu serialises everything that writes this worktree's index or
	// files: stage, commit, reset, checkout, cherry-pick, revert,
	// merge/rebase steps, discard.
	writeMu sync.Mutex

	// messageMu guards messageDraft apart from writeMu, so the draft never
	// waits on a running git write.
	messageMu    sync.Mutex
	messageDraft string

	// mergeMu guards mergeDrafts, the unsaved merge results by path.
	mergeMu     sync.Mutex
	mergeDrafts map[string]core.MergeDraft

	// This worktree's filtered log view, apart from the repo's shared log.
	filterMu     sync.Mutex
	filterActive bool
	filter       core.LogFilter
	filterCancel context.CancelFunc
	filtered     logState
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

// SetMergeDraft keeps draft as path's unsaved merge result; nil clears it.
func (w *Worktree) SetMergeDraft(path string, draft *core.MergeDraft) {
	w.mergeMu.Lock()
	defer w.mergeMu.Unlock()
	if draft == nil {
		delete(w.mergeDrafts, path)
		return
	}
	if w.mergeDrafts == nil {
		w.mergeDrafts = map[string]core.MergeDraft{}
	}
	w.mergeDrafts[path] = *draft
}

// MergeDraft returns path's unsaved merge result, nil if none.
func (w *Worktree) MergeDraft(path string) *core.MergeDraft {
	w.mergeMu.Lock()
	defer w.mergeMu.Unlock()
	draft, ok := w.mergeDrafts[path]
	if !ok {
		return nil
	}
	return &draft
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
	log      logState
	loading  bool
	tips     map[string]string // ref-tip snapshot recorded when the log last finished
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

// LogBeginLoad marks a load started and returns its generation. ok is false
// if a load is running or has finished.
func (r *Repo) LogBeginLoad() (gen int, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.loading || r.log.done {
		return r.log.gen, false
	}
	r.loading = true
	return r.log.gen, true
}

// LogBeginReload starts a load that replaces whatever is cached; ok is
// false if a load is running. It bumps gen, so a superseded stream's late
// LogAppend/LogFinish calls are dropped.
func (r *Repo) LogBeginReload() (gen int, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.loading {
		return 0, false
	}
	r.loading = true
	return r.log.restart(), true
}

// LogAppend adds rows as they stream in; a no-op if gen is stale (a
// superseded stream still draining).
func (r *Repo) LogAppend(gen int, rows ...core.LogCommit) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.log.add(gen, rows)
}

// LogFinish marks the load complete; err is non-nil on failure. tips is the
// ref-tip snapshot recorded for the next ReloadLog's comparison. A no-op if
// gen is stale.
func (r *Repo) LogFinish(gen int, err *core.Error, tips map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.log.finish(gen, err) {
		r.loading, r.tips = false, tips
	}
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
	return r.log.done && !r.loading
}

// LogView is one snapshot of a log's rows: Repo's own shared cache
// (Repo.LogSnapshot) or a worktree's filtered view (Worktree.FilterSnapshot).
// Generation bumps whenever that view restarts from row 0 (a reload, a new
// filter).
type LogView struct {
	Rows       []core.LogCommit
	Loaded     int
	Done       bool
	Generation int
	Err        *core.Error
}

// LogSnapshot returns a snapshot of rows [from, to), clamped to
// [0, len(log)).
func (r *Repo) LogSnapshot(from, to int) LogView {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.log.view(from, to)
}

// LogIndexOf returns sha's row index in the currently loaded log. A racing
// reload can make it momentarily stale; the frontend's next LogRows call
// corrects it.
func (r *Repo) LogIndexOf(sha string) (index int, found bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, c := range r.log.rows {
		if c.SHA == sha {
			return i, true
		}
	}
	return 0, false
}

// LogIndicesOf returns, in row order, the indices of every row whose SHA is
// in shas, and the log's current generation. Reads r.log.rows in place
// under the lock: no copy, unlike LogSnapshot.
func (r *Repo) LogIndicesOf(shas map[string]bool) (indices []int, generation int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.log.indicesOf(shas)
}

// FilterBegin starts a new filtered load for filter, cancelling whatever
// filtered load is running. The returned generation stamps this load's
// FilterAppend/FilterFinish calls, so a late call from the cancelled load is
// dropped even before its cancel takes effect.
func (w *Worktree) FilterBegin(filter core.LogFilter, cancel context.CancelFunc) (gen int) {
	w.filterMu.Lock()
	defer w.filterMu.Unlock()
	w.filterActive, w.filter = true, filter
	return w.restartLocked(cancel)
}

// FilterRestart restarts the active filtered load in place, with its own
// filter, under the same lock that reads whether one is active, so a
// concurrent SetLogFilter cannot revive a cleared filter or overwrite a
// newer one. ok is false, filter and gen zero, while no filter is active.
func (w *Worktree) FilterRestart(cancel context.CancelFunc) (filter core.LogFilter, gen int, ok bool) {
	w.filterMu.Lock()
	defer w.filterMu.Unlock()
	if !w.filterActive {
		return core.LogFilter{}, 0, false
	}
	return w.filter, w.restartLocked(cancel), true
}

// restartLocked cancels whatever filtered load is running, bumps the
// generation and clears the accumulated rows. filterMu must be held.
func (w *Worktree) restartLocked(cancel context.CancelFunc) (gen int) {
	if w.filterCancel != nil {
		w.filterCancel()
	}
	w.filterCancel = cancel
	return w.filtered.restart()
}

// FilterAppend adds rows as a filtered load streams in; a no-op if gen is
// stale (a superseded stream still draining).
func (w *Worktree) FilterAppend(gen int, rows ...core.LogCommit) {
	w.filterMu.Lock()
	defer w.filterMu.Unlock()
	w.filtered.add(gen, rows)
}

// FilterFinish marks a filtered load complete; err is non-nil on failure. A
// no-op if gen is stale.
func (w *Worktree) FilterFinish(gen int, err *core.Error) {
	w.filterMu.Lock()
	defer w.filterMu.Unlock()
	w.filtered.finish(gen, err)
}

// LogSnapshot is what windows on this worktree show: its filtered view
// while a filter is active, the repo's shared log otherwise.
func (w *Worktree) LogSnapshot(from, to int) LogView {
	if view, ok := w.FilterSnapshot(from, to); ok {
		return view
	}
	return w.Repo.LogSnapshot(from, to)
}

// FilterSnapshot mirrors Repo.LogSnapshot, scoped to this worktree's
// filtered view. ok is false when no filter is active.
func (w *Worktree) FilterSnapshot(from, to int) (view LogView, ok bool) {
	w.filterMu.Lock()
	defer w.filterMu.Unlock()
	if !w.filterActive {
		return LogView{}, false
	}
	return w.filtered.view(from, to), true
}

// ActiveFilter returns the filtered view's own filter; ok is false while no
// filter is active, filter the zero value.
func (w *Worktree) ActiveFilter() (filter core.LogFilter, ok bool) {
	w.filterMu.Lock()
	defer w.filterMu.Unlock()
	return w.filter, w.filterActive
}

// LogIndicesOf mirrors LogSnapshot: it maps shas to their row indices in
// this worktree's current view (its filtered view while one is active,
// else the repo's shared log), without copying that view's rows. filtered
// says which view answered.
func (w *Worktree) LogIndicesOf(shas map[string]bool) (indices []int, generation int, filtered bool) {
	if indices, generation, ok := w.filteredIndicesOf(shas); ok {
		return indices, generation, true
	}
	indices, generation = w.Repo.LogIndicesOf(shas)
	return indices, generation, false
}

// filteredIndicesOf mirrors FilterSnapshot, scoped to LogIndicesOf.
func (w *Worktree) filteredIndicesOf(shas map[string]bool) (indices []int, generation int, ok bool) {
	w.filterMu.Lock()
	defer w.filterMu.Unlock()
	if !w.filterActive {
		return nil, 0, false
	}
	indices, generation = w.filtered.indicesOf(shas)
	return indices, generation, true
}

// FilterClear cancels any running filtered stream and reverts LogRows to
// the repo's shared cache.
func (w *Worktree) FilterClear() {
	w.filterMu.Lock()
	defer w.filterMu.Unlock()
	if w.filterCancel != nil {
		w.filterCancel()
	}
	w.filterActive, w.filter, w.filterCancel = false, core.LogFilter{}, nil
	w.filtered = logState{gen: w.filtered.gen}
}

// logState is one streamed log: its rows so far, whether the stream has
// finished and how, and the generation stamping the current stream. Its
// owner's mutex guards it.
type logState struct {
	rows []core.LogCommit
	done bool
	err  *core.Error
	gen  int
}

// restart drops the rows and bumps the generation, so a superseded
// stream's late add and finish calls are dropped.
func (s *logState) restart() (gen int) {
	*s = logState{gen: s.gen + 1}
	return s.gen
}

func (s *logState) add(gen int, rows []core.LogCommit) {
	if gen == s.gen {
		s.rows = append(s.rows, rows...)
	}
}

// finish marks the stream done, reporting false if gen is stale.
func (s *logState) finish(gen int, err *core.Error) bool {
	if gen != s.gen {
		return false
	}
	s.done, s.err = true, err
	return true
}

// view is the stream's state with a copy of rows [from, to), clamped to
// [0, len(rows)).
func (s *logState) view(from, to int) LogView {
	from = min(max(from, 0), len(s.rows))
	to = min(max(to, from), len(s.rows))
	return LogView{
		Rows:       append([]core.LogCommit(nil), s.rows[from:to]...),
		Loaded:     len(s.rows),
		Done:       s.done,
		Generation: s.gen,
		Err:        s.err,
	}
}

// indicesOf returns, in row order, the indices of every row whose SHA is in
// shas, and the stream's current generation; no row is copied. An empty
// shas skips the scan.
func (s *logState) indicesOf(shas map[string]bool) (indices []int, generation int) {
	if len(shas) > 0 {
		for i, c := range s.rows {
			if shas[c.SHA] {
				indices = append(indices, i)
			}
		}
	}
	return indices, s.gen
}

// Manager mints and reuses sessions: opening the same worktree twice (by
// any path that resolves to it) returns the same *Worktree, and worktrees
// sharing a repo share one *Repo.
type Manager struct {
	sink     core.EventSink
	discover git.Runner // no Log: discovery calls predate any session

	newWatcher func(commonDir string, onChange func(core.ChangeScope)) (repoWatcher, error)
	newPoller  func(runner git.Runner, dir string, onChange func(core.ChangeScope)) worktreePoller

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
		newWatcher: func(commonDir string, onChange func(core.ChangeScope)) (repoWatcher, error) {
			return watch.NewRepoWatcher(commonDir, onChange)
		},
		newPoller: func(runner git.Runner, dir string, onChange func(core.ChangeScope)) worktreePoller {
			return watch.NewWorktreePoller(runner, dir, onChange)
		},
	}
}

// Resolve validates path and returns its worktree's identity (toplevel, git
// dir, repo key) without minting or reusing a session, so listing a repo's
// worktrees starts no poller or watcher. A missing or too old git is Code
// GitTooOld; a path outside a work tree is Code NotARepo.
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
		return "", "", "", RemapNotARepo(err, path)
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

// Get returns an already-open session; Code Internal for an unknown id.
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

// newRepo mints repo's session and starts its watcher. A watcher that
// fails to start still runs on its fallback poll.
func (m *Manager) newRepo(key string) *Repo {
	repo := &Repo{Key: key}
	repo.watcher, _ = m.newWatcher(key, func(scope core.ChangeScope) {
		m.sink(core.Event{Name: "repo:changed", Repo: key, Payload: changedPayload(scope)})
	})
	repo.jobs = jobs.NewManager(func(j jobs.Job) {
		m.sink(core.Event{Name: "job:update", Repo: key, Payload: payload(jobs.Info(j))})
	})
	return repo
}

// newWorktree mints wt's session and focuses its status poller at once: a
// new window's first focus event is not guaranteed.
func (m *Manager) newWorktree(id core.WorktreeID, toplevel, gitDir string, repo *Repo) *Worktree {
	wt := &Worktree{ID: id, Path: toplevel, GitDir: gitDir, Repo: repo}
	wt.Runner = git.Runner{Log: func(e git.Entry) {
		m.sink(core.Event{Name: "command", Worktree: id, Payload: payload(toCommandRecord(e))})
	}}
	wt.poller = m.newPoller(wt.Runner, toplevel, func(scope core.ChangeScope) {
		m.sink(core.Event{Name: "repo:changed", Worktree: id, Payload: changedPayload(scope)})
	})
	wt.poller.Focus()
	return wt
}

// changedPayload is a "repo:changed" event's payload.
func changedPayload(scope core.ChangeScope) json.RawMessage {
	return payload(core.RepoChanged{Scope: scope})
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
	delete(m.worktrees, id)
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

// RemapNotARepo turns a discovery failure into Code NotARepo with
// notARepo's own message, keeping GitStderr/Command for the details.
func RemapNotARepo(err error, path core.RepoPath) error {
	if src, ok := errors.AsType[*core.Error](err); ok {
		want := notARepo(path)
		src.Retag(want.Code, want.Key, want.Args, "%s", want.Message)
		return src
	}
	return notARepo(path)
}

func notARepo(path core.RepoPath) *core.Error {
	return core.ErrorfArgs(core.NotARepo, "notARepo", map[string]string{"path": string(path)}, "%s is not a git repository", path)
}

// toCommandRecord converts a git.Entry into the "command" event's payload.
func toCommandRecord(e git.Entry) core.CommandRecord {
	entry := core.CommandRecord{
		Command:    e.Command,
		Dir:        e.Dir,
		Background: e.Background,
		DurationMS: e.Duration.Milliseconds(),
		ExitCode:   e.Result.ExitCode,
		Stderr:     string(e.Result.Stderr),
	}
	if ce, ok := errors.AsType[*core.Error](e.Err); ok {
		entry.ErrorCode = ce.Code
	}
	return entry
}

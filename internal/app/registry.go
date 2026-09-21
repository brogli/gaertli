package app

import (
	"context"
	"net/url"
	"sync"

	"github.com/brogli/nebegit/internal/core"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// shellWindow is the part of a Wails window this package needs; satisfied
// by *application.WebviewWindow.
type shellWindow interface {
	Focus()
	EmitEvent(name string, data ...any) bool
	OnWindowEvent(eventType events.WindowEventType, callback func(*application.WindowEvent)) func()
}

// windowOpener creates a shellWindow; satisfied by wailsOpener in
// production, a fake in tests.
type windowOpener interface {
	NewWithOptions(application.WebviewWindowOptions) shellWindow
}

// wailsOpener adapts *application.WindowManager to windowOpener.
type wailsOpener struct{ wm *application.WindowManager }

func (o wailsOpener) NewWithOptions(opts application.WebviewWindowOptions) shellWindow {
	return o.wm.NewWithOptions(opts)
}

type windowEntry struct {
	window shellWindow
	path   string
	repo   string // common git dir, set by SetRepo
}

// Registry is the window registry (ADR-9) and the Router the Facade
// forwards through. Local repos only; WSL classification is spec 30, so
// CoreOf and CoreFor both answer Local() for now.
type Registry struct {
	local  core.API
	opener windowOpener // nil until Bind; set once, after application.New returns

	mu   sync.Mutex
	byID map[core.WorktreeID]*windowEntry
	home shellWindow
}

// NewRegistry builds an empty Registry.
func NewRegistry() *Registry {
	return &Registry{byID: make(map[core.WorktreeID]*windowEntry)}
}

// SetLocal wires the in-process core once built.
func (r *Registry) SetLocal(local core.API) { r.local = local }

// Bind sets the opener that creates real Wails windows.
func (r *Registry) Bind(a *application.App) { r.opener = wailsOpener{wm: a.Window} }

// Local returns the local core.
func (r *Registry) Local() core.API { return r.local }

// CoreOf returns the local core; worktree-based routing is spec 30.
func (r *Registry) CoreOf(core.WorktreeID) (core.API, error) { return r.local, nil }

// CoreFor returns the local core; path-based routing is spec 30.
func (r *Registry) CoreFor(core.RepoPath) (core.API, error) { return r.local, nil }

// OpenOrFocus opens a window for a validated worktree (FR-WT-08's existing
// behaviour, now keyed by the resolved id instead of the raw launch path —
// spec 04's own placeholder for this, ADR-9), or focuses it if already open.
func (r *Registry) OpenOrFocus(id core.WorktreeID, path string) {
	if w := r.openOrRegister(id, path); w != nil {
		w.Focus()
	}
}

// openOrRegister returns the already open window for id, or registers a new
// one and returns nil. Focus happens outside the lock.
func (r *Registry) openOrRegister(id core.WorktreeID, path string) shellWindow {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entry, ok := r.byID[id]; ok {
		return entry.window
	}
	window := r.newWindow(windowTitle(path, ""), "/?path="+url.QueryEscape(path), func() {
		r.Remove(id)
		_ = r.local.CloseWorktree(context.Background(), id) // ADR-9: best-effort, nothing user-visible to show
	})
	wireFocusEvents(window, func(focused bool) { _ = r.local.SetFocused(context.Background(), id, focused) })
	r.byID[id] = &windowEntry{window: window, path: path}
	return nil
}

// wireFocusEvents relays a window's OS-level focus state to setFocused
// (FR-REPO-04, QA-PERF-06): a worktree window only, not the home window.
func wireFocusEvents(w shellWindow, setFocused func(focused bool)) {
	w.OnWindowEvent(events.Common.WindowFocus, func(*application.WindowEvent) { go setFocused(true) })
	w.OnWindowEvent(events.Common.WindowLostFocus, func(*application.WindowEvent) { go setFocused(false) })
}

// newWindow creates a window that drops itself from the registry when the
// OS closes it (Wails destroys the window on WindowClosing).
func (r *Registry) newWindow(title, url string, onClose func()) shellWindow {
	window := r.opener.NewWithOptions(application.WebviewWindowOptions{
		Title:  title,
		URL:    url,
		Width:  1200,
		Height: 800,
	})
	window.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { onClose() })
	return window
}

// OpenHome opens the no-worktree home window (FR-REPO-02's empty-restore
// case, FR-REPO-01/02's entry point), or focuses it. At most one.
func (r *Registry) OpenHome() {
	if w := r.openOrRegisterHome(); w != nil {
		w.Focus()
	}
}

func (r *Registry) openOrRegisterHome() shellWindow {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.home != nil {
		return r.home
	}
	r.home = r.newWindow(windowTitle("", ""), "/", r.removeHome)
	return nil
}

func (r *Registry) removeHome() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.home = nil
}

// Remove drops a closed window's entry.
func (r *Registry) Remove(id core.WorktreeID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byID, id)
}

// OpenWorktreePaths lists every currently open worktree window's path, for
// FR-REPO-02's snapshot.
func (r *Registry) OpenWorktreePaths() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	paths := make([]string, 0, len(r.byID))
	for _, entry := range r.byID {
		paths = append(paths, entry.path)
	}
	return paths
}

// SetRepo records the repo key (common git dir) of an open window so
// repo-scoped events reach every window of that repo. Called by spec 05
// once a session knows its repo; a no-op for an unknown id.
func (r *Registry) SetRepo(wt core.WorktreeID, repo string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entry, ok := r.byID[wt]; ok {
		entry.repo = repo
	}
}

// Dispatch is the core.EventSink of the local core (ADR-9): an event with
// Worktree set goes to that window, one with only Repo set to every window
// of that repo.
func (r *Registry) Dispatch(ev core.Event) {
	for _, w := range r.targets(ev) {
		w.EmitEvent(ev.Name, ev.Payload)
	}
}

// targets picks the windows an event concerns; the native calls happen
// after the lock is released.
func (r *Registry) targets(ev core.Event) []shellWindow {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ev.Worktree != "" {
		if entry, ok := r.byID[ev.Worktree]; ok {
			return []shellWindow{entry.window}
		}
		return nil
	}
	var windows []shellWindow
	for _, entry := range r.byID {
		if ev.Repo != "" && entry.repo == ev.Repo {
			windows = append(windows, entry.window)
		}
	}
	return windows
}

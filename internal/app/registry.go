package app

import (
	"context"
	"maps"
	"sync"

	"github.com/brogli/nebegit/internal/buildinfo"
	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/editor"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// shellWindow is the part of a Wails window this package needs; satisfied
// by *application.WebviewWindow.
type shellWindow interface {
	Focus()
	EmitEvent(name string, data ...any) bool
	OnWindowEvent(eventType events.WindowEventType, callback func(*application.WindowEvent)) func()
	ID() uint // matches application.Window.ID(), the ctx-carried window
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

// windowEntry is one window's bookkeeping; the selection is mutable, since
// a window can switch.
type windowEntry struct {
	window   shellWindow
	selected core.WorktreeID // "" = no selection (the panel's empty state)
	path     string          // selected worktree's canonical path; "" if none
	repo     string          // common git dir, set by SetRepo
	distro   string          // "" = local; else the WSL distro this window's core runs in
}

// distroCorer is what Registry needs from a WSL distro's live core
// connection; satisfied by *remote.DistroCore in production, a fake in
// tests.
type distroCorer interface {
	Ensure(ctx context.Context) (*remote.Client, bool, error)
	InstallLauncher(ctx context.Context, guiExePath string) error

	// OnConnect and LiveClient back PushEditorProfiles: OnConnect registers
	// the push for every fresh connect, LiveClient lets a settings change
	// re-push without forcing one.
	OnConnect(fn func(context.Context, *remote.Client))
	LiveClient() (*remote.Client, bool)
}

// compareWindowKey identifies one worktree's a..b comparison: the compare
// window is scoped per comparison, not per file.
type compareWindowKey struct {
	wt   core.WorktreeID
	a, b string
}

// compareShowFilePayload is "compare:show-file"'s payload.
type compareShowFilePayload struct {
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"`
}

// Registry is the window registry and the Router the Facade forwards
// through.
type Registry struct {
	local  core.API
	opener windowOpener // nil until Bind; set once, after application.New returns

	consent remote.Consent  // nil until SetConsent, wired by Run
	store   *settings.Store // nil until SetStore; read by pushEditorProfile

	mu             sync.Mutex
	windows        map[uint]*windowEntry            // one entry per window, keyed by its Wails window id
	compareWindows map[compareWindowKey]shellWindow // a distinct, non-selecting window kind
	distros        map[string]distroCorer           // lazily created, one per distro in use
	newDistroCore  func(distro string) distroCorer
}

// NewRegistry builds an empty Registry.
func NewRegistry() *Registry {
	r := &Registry{
		windows:        make(map[uint]*windowEntry),
		compareWindows: make(map[compareWindowKey]shellWindow),
		distros:        make(map[string]distroCorer),
	}
	r.newDistroCore = func(distro string) distroCorer {
		return remote.NewDistroCore(distro, buildinfo.App, remote.WslRun, r.consent, r.Dispatch)
	}
	return r
}

// SetLocal wires the in-process core once built.
func (r *Registry) SetLocal(local core.API) { r.local = local }

// SetConsent wires the consent dialog a distro's first deploy asks.
func (r *Registry) SetConsent(c remote.Consent) { r.consent = c }

// SetStore wires the settings store PushEditorProfiles reads.
func (r *Registry) SetStore(store *settings.Store) { r.store = store }

// Bind sets the opener that creates real Wails windows.
func (r *Registry) Bind(a *application.App) { r.opener = wailsOpener{wm: a.Window} }

// Local returns the local core.
func (r *Registry) Local() core.API { return r.local }

// distroCore lazily creates (or returns the existing) DistroCore for
// distro, wiring its editor profile to be pushed on every fresh connect,
// first or reconnect.
func (r *Registry) distroCore(distro string) distroCorer {
	r.mu.Lock()
	defer r.mu.Unlock()

	if dc, ok := r.distros[distro]; ok {
		return dc
	}
	dc := r.newDistroCore(distro)
	dc.OnConnect(func(ctx context.Context, c *remote.Client) { r.pushEditorProfile(ctx, c, distro) })
	r.distros[distro] = dc
	return dc
}

// pushEditorProfile sends env's current profile to api. Best-effort: a push
// failure leaves a stale/default profile in effect until the next
// successful push, and is not surfaced to the user.
func (r *Registry) pushEditorProfile(ctx context.Context, api core.API, env string) {
	if r.store == nil {
		return
	}
	_ = api.SetEditorProfile(ctx, profileFor(r.store.Get(), env))
}

// PushEditorProfiles re-pushes every connected core's profile: the local
// core, plus every distro with a live client. A distro not yet connected
// gets its profile through OnConnect.
func (r *Registry) PushEditorProfiles(ctx context.Context) {
	r.pushEditorProfile(ctx, r.local, "")

	r.mu.Lock()
	distros := make(map[string]distroCorer, len(r.distros))
	maps.Copy(distros, r.distros)
	r.mu.Unlock()

	for name, dc := range distros {
		if client, ok := dc.LiveClient(); ok {
			r.pushEditorProfile(ctx, client, name)
		}
	}
}

// profileFor resolves env's editor profile ("" = local, else a WSL distro
// name), falling back to editor.Default() when unconfigured.
func profileFor(s settings.Settings, env string) core.EditorProfile {
	return editor.OrDefault(s.EditorProfiles[env])
}

// CoreOf routes an already-open worktree's calls to the core its window was
// opened against. The WorktreeID alone cannot tell: for a WSL repo it is a
// Linux-side path, and two distros can each have the same one.
func (r *Registry) CoreOf(id core.WorktreeID) (core.API, error) {
	entry, ok := r.lookup(id)
	if !ok {
		return nil, core.Errorf(core.Internal, "unknown worktree")
	}
	return r.coreForDistro(entry.distro)
}

// coreForDistro resolves distro ("" = local) to its core. It routes a
// worktree a switch or close just orphaned, which CoreOf can no longer
// find. If Ensure had to respawn, every window this distro backs is
// re-opened from its remembered path first.
func (r *Registry) coreForDistro(distro string) (core.API, error) {
	if distro == "" {
		return r.local, nil
	}
	client, respawned, err := r.distroCore(distro).Ensure(context.Background())
	if err != nil {
		return nil, err
	}
	if respawned {
		r.reopenAfterReconnect(distro, client)
	}
	return client, nil
}

// CoreFor routes a WSL-shaped path to that distro's core, deploying
// and connecting on first use; a local path is unchanged (Local()).
func (r *Registry) CoreFor(path core.RepoPath) (core.API, error) {
	distro, _, ok := remote.ClassifyWSLPath(string(path))
	if !ok {
		return r.local, nil
	}
	client, _, err := r.distroCore(distro).Ensure(context.Background())
	if err != nil {
		return nil, err
	}
	return remote.NewOpener(client, distro), nil
}

// findSelectingLocked returns the window currently selecting wt, or nil.
// Caller holds r.mu. "" matches an empty-selection window, so the dedup
// also keeps at most one empty window.
func (r *Registry) findSelectingLocked(wt core.WorktreeID) *windowEntry {
	for _, e := range r.windows {
		if e.selected == wt {
			return e
		}
	}
	return nil
}

// lookup finds the window currently selecting id.
func (r *Registry) lookup(id core.WorktreeID) (*windowEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.findSelectingLocked(id)
	return e, e != nil
}

// reopenAfterReconnect re-opens every window's session backed by distro
// after DistroCore.Ensure had to respawn, then tells each window to reload.
// Best-effort per window: one failing surfaces the normal error on its next
// action.
func (r *Registry) reopenAfterReconnect(distro string, client *remote.Client) {
	for id, path := range r.pathsForDistro(distro) {
		if _, err := client.OpenWorktree(context.Background(), core.RepoPath(path)); err != nil {
			continue
		}
		r.Dispatch(core.Event{Name: "core:reconnected", Worktree: id})
	}
}

// pathsForDistro collects every open window's remembered path for distro,
// and releases r.mu before the caller's own OpenWorktree/Dispatch calls,
// which take the same lock.
func (r *Registry) pathsForDistro(distro string) map[core.WorktreeID]string {
	r.mu.Lock()
	defer r.mu.Unlock()

	paths := make(map[core.WorktreeID]string)
	for _, entry := range r.windows {
		if entry.selected != "" && entry.distro == distro {
			paths[entry.selected] = entry.path
		}
	}
	return paths
}

// OpenOrFocus opens a window for a validated worktree, keyed by its
// resolved id, or focuses it if already open. distro is
// "" for a local repo, else the WSL distro its core runs in.
func (r *Registry) OpenOrFocus(id core.WorktreeID, path, distro string) {
	if w := r.openOrRegister(id, path, distro); w != nil {
		w.Focus()
	}
}

// openOrRegister returns the already open window selecting id, or registers
// a new one and returns nil, so the caller focuses outside the lock. The
// close callback closes over window before it is assigned: safe, because
// newWindow returns before the OS can fire WindowClosing.
func (r *Registry) openOrRegister(id core.WorktreeID, path, distro string) shellWindow {
	r.mu.Lock()
	defer r.mu.Unlock()

	if e := r.findSelectingLocked(id); e != nil {
		return e.window
	}
	var window shellWindow
	window = r.newWindow(windowTitle(path, ""), selectionURL(path), func() {
		closed, distro := r.closeWindow(window.ID())
		r.Release(closed, distro)
	})
	wireFocusEvents(window, func(focused bool) { _ = r.local.SetFocused(context.Background(), id, focused) })
	r.windows[window.ID()] = &windowEntry{window: window, selected: id, path: path, distro: distro}
	return nil
}

// SelectInWindow moves windowID's entry to select wt. It reports the
// previous selection and the distro that backed it, so the caller can
// release it through its core. ok is false if windowID names no known
// window.
func (r *Registry) SelectInWindow(windowID uint, wt core.WorktreeID, path, distro string) (previous core.WorktreeID, previousDistro string, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, exists := r.windows[windowID]
	if !exists {
		return "", "", false
	}
	previous, previousDistro = e.selected, e.distro
	e.selected, e.path, e.distro, e.repo = wt, path, distro, ""
	return previous, previousDistro, true
}

// StillSelected reports whether any window currently selects wt.
func (r *Registry) StillSelected(wt core.WorktreeID) bool {
	_, ok := r.lookup(wt)
	return ok
}

// Release closes wt's session through its core once no window selects it
// any more; best-effort, nothing user-visible.
func (r *Registry) Release(wt core.WorktreeID, distro string) {
	if wt == "" || r.StillSelected(wt) {
		return
	}
	if c, err := r.coreForDistro(distro); err == nil {
		_ = c.CloseWorktree(context.Background(), wt)
	}
}

// closeWindow drops windowID's entry (a real OS close) and reports its
// former selection and the distro that backed it.
func (r *Registry) closeWindow(windowID uint) (closed core.WorktreeID, distro string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.windows[windowID]
	if !ok {
		return "", ""
	}
	delete(r.windows, windowID)
	return e.selected, e.distro
}

// wireFocusEvents relays a window's OS-level focus state to setFocused.
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

// OpenOrFocusCompareDiff opens the compare diff window for wt's a..b
// comparison, or focuses and retargets it if already open. It is not a
// worktree-session window: never added to r.windows, never
// CloseWorktree'd on close.
func (r *Registry) OpenOrFocusCompareDiff(wt core.WorktreeID, a, b, path, oldPath string) {
	key := compareWindowKey{wt, a, b}
	if w := r.openOrRegisterCompareDiff(key, path, oldPath); w != nil {
		w.Focus()
		w.EmitEvent("compare:show-file", compareShowFilePayload{Path: path, OldPath: oldPath})
	}
}

// openOrRegisterCompareDiff follows openOrRegister's nil-means-just-created
// convention: nil for a fresh window (already visible, no Focus() needed),
// the existing window otherwise.
func (r *Registry) openOrRegisterCompareDiff(key compareWindowKey, path, oldPath string) shellWindow {
	r.mu.Lock()
	defer r.mu.Unlock()

	if w, ok := r.compareWindows[key]; ok {
		return w
	}
	w := r.newWindow(
		compareWindowTitle(string(key.wt), key.a, key.b),
		compareDiffURL(key.wt, key.a, key.b, path, oldPath),
		func() { r.removeCompareWindow(key) },
	)
	r.compareWindows[key] = w
	return nil
}

func (r *Registry) removeCompareWindow(key compareWindowKey) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.compareWindows, key)
}

// OpenWorktreePaths lists every currently selected worktree's path, one per
// window that has one, for the restore-at-launch snapshot.
func (r *Registry) OpenWorktreePaths() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	paths := make([]string, 0, len(r.windows))
	for _, entry := range r.windows {
		if entry.selected != "" {
			paths = append(paths, entry.path)
		}
	}
	return paths
}

// SetRepo records the repo key (common git dir) of the window currently
// selecting wt, so repo-scoped events reach it; a no-op if no window
// selects wt.
func (r *Registry) SetRepo(wt core.WorktreeID, repo string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if e := r.findSelectingLocked(wt); e != nil {
		e.repo = repo
	}
}

// Dispatch is the core.EventSink of every core: an event with
// Worktree set goes to the window selecting it, one with only Repo set to
// every window of that repo.
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

	var windows []shellWindow
	for _, entry := range r.windows {
		switch {
		case ev.Worktree != "":
			if entry.selected == ev.Worktree { // every window sharing the selection
				windows = append(windows, entry.window)
			}
		case ev.Repo != "" && entry.repo == ev.Repo:
			windows = append(windows, entry.window)
		}
	}
	return windows
}

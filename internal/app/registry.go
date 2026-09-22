package app

import (
	"context"
	"maps"
	"net/url"
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
	distro string // "" = local (ADR-3); else the WSL distro this window's core runs in
}

// distroCorer is what Registry needs from a WSL distro's live core
// connection; satisfied by *remote.DistroCore in production, a fake in
// tests.
type distroCorer interface {
	Ensure(ctx context.Context) (*remote.Client, bool, error)
	InstallLauncher(ctx context.Context, guiExePath string) error

	// OnConnect and LiveClient back PushEditorProfiles (TR-13): OnConnect
	// registers the push for every fresh connect, LiveClient lets a
	// settings change re-push without forcing one.
	OnConnect(fn func(context.Context, *remote.Client))
	LiveClient() (*remote.Client, bool)
}

// compareWindowKey identifies one worktree's a..b comparison (Design: this
// window is scoped per comparison, not per file, spec 23).
type compareWindowKey struct {
	wt   core.WorktreeID
	a, b string
}

// compareShowFilePayload is "compare:show-file"'s payload (Design, Events,
// spec 23).
type compareShowFilePayload struct {
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"`
}

// Registry is the window registry (ADR-9) and the Router the Facade
// forwards through.
type Registry struct {
	local  core.API
	opener windowOpener // nil until Bind; set once, after application.New returns

	consent remote.Consent  // nil until SetConsent; wired by internal/app's Run (spec 30, QA-PORT-02)
	store   *settings.Store // nil until SetStore; read by pushEditorProfile (TR-13)

	mu             sync.Mutex
	byID           map[core.WorktreeID]*windowEntry
	home           shellWindow
	compareWindows map[compareWindowKey]shellWindow
	distros        map[string]distroCorer // lazily created, one per distro in use (ADR-3)
	newDistroCore  func(distro string) distroCorer
}

// NewRegistry builds an empty Registry.
func NewRegistry() *Registry {
	r := &Registry{
		byID:           make(map[core.WorktreeID]*windowEntry),
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

// SetConsent wires the consent dialog a distro's first deploy asks
// (QA-PORT-02).
func (r *Registry) SetConsent(c remote.Consent) { r.consent = c }

// SetStore wires the settings store, so PushEditorProfiles has something to
// read (TR-13). Run calls this before SetLocal, so the first push has a
// store to read from.
func (r *Registry) SetStore(store *settings.Store) { r.store = store }

// Bind sets the opener that creates real Wails windows.
func (r *Registry) Bind(a *application.App) { r.opener = wailsOpener{wm: a.Window} }

// Local returns the local core.
func (r *Registry) Local() core.API { return r.local }

// distroCore lazily creates (or returns the existing) DistroCore for distro
// (ADR-3: one RPC core per distro in use), wiring its own editor profile to
// be pushed on every fresh connect (TR-13) — covers both a first connect
// (CoreFor, opening a WSL repo for the first time) and a reconnect (CoreOf,
// after the distro's core process died), since both go through this same
// factory and DistroCore.Ensure.
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
// failure just leaves a stale/default profile in effect until the next
// successful push — not surfaced to the user, same framing as
// reopenAfterReconnect's own best-effort handling.
func (r *Registry) pushEditorProfile(ctx context.Context, api core.API, env string) {
	if r.store == nil {
		return
	}
	_ = api.SetEditorProfile(ctx, profileFor(r.store.Get(), env))
}

// PushEditorProfiles re-pushes every already-connected core's own profile:
// the local core, plus every distro whose DistroCore already has a live
// client — a distro not yet connected is left alone, it gets its profile
// the normal way (OnConnect) the first time it is. Called once at startup
// (Run, right after SetLocal — no distro is connected yet, so this only
// reaches the local core) and by SettingsService.Set after every settings
// change, so an edit takes effect immediately wherever it is already in
// use.
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
// name), falling back to editor.Default() (FR-ED-05) when unconfigured.
func profileFor(s settings.Settings, env string) core.EditorProfile {
	return editor.OrDefault(s.EditorProfiles[env])
}

// CoreOf routes an already-open worktree's calls to the same core its
// window was opened against, tracked in windowEntry.distro at open time —
// not re-derived from the WorktreeID alone, which for a WSL repo is a
// Linux-side path and is not by itself distro-identifying (two distros can
// each have /home/user/project). Reconnect (ADR-4): if Ensure had to
// respawn, every window this distro backs is re-opened from its own
// remembered path before this call proceeds.
func (r *Registry) CoreOf(id core.WorktreeID) (core.API, error) {
	entry, ok := r.lookup(id)
	if !ok {
		return nil, core.Errorf(core.Internal, "unknown worktree")
	}
	if entry.distro == "" {
		return r.local, nil
	}
	client, respawned, err := r.distroCore(entry.distro).Ensure(context.Background())
	if err != nil {
		return nil, err
	}
	if respawned {
		r.reopenAfterReconnect(entry.distro, client)
	}
	return client, nil
}

// CoreFor routes a WSL-shaped path to that distro's core (ADR-3), deploying
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

// lookup finds id's window entry.
func (r *Registry) lookup(id core.WorktreeID) (*windowEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.byID[id]
	return entry, ok
}

// reopenAfterReconnect re-opens every window backed by distro (ADR-4:
// nothing to restore beyond re-opening the worktree sessions that were open
// against it) after DistroCore.Ensure had to respawn, then tells each one
// its data needs a fresh reload. Best-effort per window: one failing does
// not stop the others, and it will simply surface the normal error on its
// next action.
func (r *Registry) reopenAfterReconnect(distro string, client *remote.Client) {
	for id, path := range r.pathsForDistro(distro) {
		if _, err := client.OpenWorktree(context.Background(), core.RepoPath(path)); err != nil {
			continue
		}
		r.Dispatch(core.Event{Name: "core:reconnected", Worktree: id})
	}
}

// pathsForDistro collects every open window's remembered path for distro,
// released before the caller's own OpenWorktree/Dispatch calls (Design:
// Dispatch/targets already take the same lock).
func (r *Registry) pathsForDistro(distro string) map[core.WorktreeID]string {
	r.mu.Lock()
	defer r.mu.Unlock()

	paths := make(map[core.WorktreeID]string)
	for id, entry := range r.byID {
		if entry.distro == distro {
			paths[id] = entry.path
		}
	}
	return paths
}

// OpenOrFocus opens a window for a validated worktree (FR-WT-08's existing
// behaviour, now keyed by the resolved id instead of the raw launch path —
// spec 04's own placeholder for this, ADR-9), or focuses it if already open.
// distro is "" for a local repo, else the WSL distro its core runs in.
func (r *Registry) OpenOrFocus(id core.WorktreeID, path, distro string) {
	if w := r.openOrRegister(id, path, distro); w != nil {
		w.Focus()
	}
}

// openOrRegister returns the already open window for id, or registers a new
// one and returns nil. Focus happens outside the lock.
func (r *Registry) openOrRegister(id core.WorktreeID, path, distro string) shellWindow {
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
	r.byID[id] = &windowEntry{window: window, path: path, distro: distro}
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

// OpenOrFocusCompareDiff opens the compare diff window for wt's a..b
// comparison, or focuses and retargets it if already open (spec 23,
// FR-CMP-02). Never added to r.byID and never calls CloseWorktree on close
// (Design, UI): it is not a worktree-session window.
func (r *Registry) OpenOrFocusCompareDiff(wt core.WorktreeID, a, b, path, oldPath string) {
	key := compareWindowKey{wt, a, b}
	if w := r.openOrRegisterCompareDiff(key, path, oldPath); w != nil {
		w.Focus()
		w.EmitEvent("compare:show-file", compareShowFilePayload{Path: path, OldPath: oldPath})
	}
}

// openOrRegisterCompareDiff mirrors openOrRegister's own nil-means-
// just-created convention (FR-WT-08): nil for a fresh window (already
// visible, no Focus() needed), the existing window otherwise.
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

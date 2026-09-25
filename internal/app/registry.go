package app

import (
	"context"
	"log/slog"
	"maps"
	"os"
	"slices"
	"sync"
	"sync/atomic"

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
	SetTitle(title string) application.Window
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
// a window can switch. A diff window's selection is the worktree it shows.
type windowEntry struct {
	window   shellWindow
	selected core.WorktreeID // "" = no selection (the repo list as the page)
	path     string          // selected worktree's canonical path; "" if none
	repo     string          // common git dir, set by SetRepo
	distro   string          // "" = local; else the WSL distro this window's core runs in
	focused  bool            // has OS focus
}

// distroCorer is what Registry needs from a WSL distro's live core
// connection; satisfied by *remote.DistroCore in production, a fake in
// tests.
type distroCorer interface {
	Ensure(ctx context.Context) (*remote.Client, bool, error)

	// OnConnect and LiveClient back PushEditorProfiles: OnConnect registers
	// the push for every fresh connect, LiveClient lets a settings change
	// re-push without forcing one.
	OnConnect(fn func(context.Context, *remote.Client))
	LiveClient() (*remote.Client, bool)
}

// Registry is the window registry and the Router the Facade forwards
// through.
type Registry struct {
	// ctx is the context of the core calls the registry makes on its own
	// rather than for one request: a distro core's spawn and the re-opens
	// after a respawn, a closed window's release, a focus change.
	ctx context.Context
	log *slog.Logger // reports a dropped event

	local  core.API
	opener windowOpener // nil until Bind; set once, after application.New returns

	consent remote.Consent  // nil until SetConsent, wired by Run
	store   *settings.Store // nil until SetStore; read by pushEditorProfile

	mu            sync.Mutex
	windows       map[uint]*windowEntry          // one entry per main window, keyed by its Wails window id
	diffWindows   map[diffWindowKey]*windowEntry // each outlives its main window
	pages         map[shellWindow]*windowEntry   // server-mode browser pages, see pageKey; kept until the process ends, so their worktrees' sessions stay open
	distros       map[string]distroCorer         // lazily created, one per distro in use
	newDistroCore func(distro string, sink core.EventSink) distroCorer

	focusMu sync.Mutex // orders setFocused's core calls

	quitting atomic.Bool // set by Quitting; the windows closing after it stay remembered
}

// NewRegistry builds an empty Registry.
func NewRegistry() *Registry {
	r := &Registry{
		ctx:         context.Background(),
		log:         slog.Default(),
		windows:     make(map[uint]*windowEntry),
		diffWindows: make(map[diffWindowKey]*windowEntry),
		pages:       make(map[shellWindow]*windowEntry),
		distros:     make(map[string]distroCorer),
	}
	guiExe, _ := os.Executable() // "" leaves a distro's nebegit command unset, and the log says so
	r.newDistroCore = func(distro string, sink core.EventSink) distroCorer {
		return remote.NewDistroCore(distro, buildinfo.App, guiExe, remote.WslRun, r.consent, sink, r.log)
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
	dc := r.newDistroCore(distro, r.sinkFor(distro))
	dc.OnConnect(func(ctx context.Context, c *remote.Client) { r.pushEditorProfile(ctx, c, distro) })
	r.distros[distro] = dc
	return dc
}

// pushEditorProfile sends env's current profile ("" = local, else a WSL
// distro name) to api. Best-effort: a failed push leaves the previous
// profile in effect until the next one, and is not shown to the user.
func (r *Registry) pushEditorProfile(ctx context.Context, api core.API, env string) {
	if r.store == nil {
		return
	}
	_ = api.SetEditorProfile(ctx, editor.OrDefault(r.store.Get().EditorProfiles[env]))
}

// PushEditorProfiles re-pushes every connected core's profile: the local
// core, plus every distro with a live client. A distro not yet connected
// gets its profile through OnConnect.
func (r *Registry) PushEditorProfiles(ctx context.Context) {
	r.pushEditorProfile(ctx, r.local, "")

	r.mu.Lock()
	distros := maps.Clone(r.distros)
	r.mu.Unlock()

	for name, dc := range distros {
		if client, ok := dc.LiveClient(); ok {
			r.pushEditorProfile(ctx, client, name)
		}
	}
}

// CoreOf routes a worktree's calls by its id alone: a WSL worktree's id is
// its core's id in UNC form (\\wsl.localhost\<distro>\...), which goes to
// that distro's core as the POSIX id the core knows; any other id goes to
// the local core unchanged. A call from a server-mode page records the
// page as showing id.
func (r *Registry) CoreOf(ctx context.Context, id core.WorktreeID) (core.API, core.WorktreeID, error) {
	if page, ok := ctx.Value(pageKey{}).(shellWindow); ok {
		r.showInPage(page, id)
	}
	distro, linuxPath, ok := remote.ClassifyWSLPath(string(id))
	if !ok {
		return r.local, id, nil
	}
	client, err := r.distroClient(distro)
	if err != nil {
		return nil, "", err
	}
	return client, core.WorktreeID(linuxPath), nil
}

// CoreFor routes a WSL-shaped path to that distro's core, deploying
// and connecting on first use; a local path is unchanged (Local()).
func (r *Registry) CoreFor(path core.RepoPath) (core.API, error) {
	distro, _, ok := remote.ClassifyWSLPath(string(path))
	if !ok {
		return r.local, nil
	}
	client, err := r.distroClient(distro)
	if err != nil {
		return nil, err
	}
	return remote.NewDistroPathClient(client, distro), nil
}

// distroClient returns distro's live client. If Ensure had to respawn,
// every window this distro backs is re-opened from its remembered path
// first, whichever call found the core dead.
func (r *Registry) distroClient(distro string) (*remote.Client, error) {
	client, respawned, err := r.distroCore(distro).Ensure(r.ctx)
	if err != nil {
		return nil, err
	}
	if respawned {
		r.reopenAfterReconnect(distro, client)
	}
	return client, nil
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

// reopenAfterReconnect re-opens every window's session backed by distro
// after DistroCore.Ensure had to respawn, then tells each window to reload.
// Best-effort per window: one failing surfaces the normal error on its next
// action.
func (r *Registry) reopenAfterReconnect(distro string, client *remote.Client) {
	for id, path := range r.pathsForDistro(distro) {
		if _, err := client.OpenWorktree(r.ctx, core.RepoPath(path)); err == nil {
			r.dispatch(distro, core.Event{Name: "core:reconnected", Worktree: id})
		}
	}
}

// pathsForDistro collects every open window's remembered path for distro,
// and releases r.mu before the caller's own OpenWorktree/Dispatch calls,
// which take the same lock.
func (r *Registry) pathsForDistro(distro string) map[core.WorktreeID]string {
	r.mu.Lock()
	defer r.mu.Unlock()

	paths := make(map[core.WorktreeID]string)
	for _, entry := range r.allEntriesLocked() {
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
// a new one and returns nil, so the caller focuses outside the lock.
func (r *Registry) openOrRegister(id core.WorktreeID, path, distro string) shellWindow {
	r.mu.Lock()
	defer r.mu.Unlock()

	if e := r.findSelectingLocked(id); e != nil {
		return e.window
	}
	r.addWindowLocked(&windowEntry{selected: id, path: path, distro: distro})
	return nil
}

// addWindowLocked opens entry's main window on its selection (none when
// entry selects nothing) and registers it. The close callback reads
// entry.window before it is assigned: safe, because newWindow returns
// before the OS can fire WindowClosing.
func (r *Registry) addWindowLocked(entry *windowEntry) {
	entry.window = r.newWindow(windowTitle(entry.path, ""), selectionURL(entry.path, entry.distro), mainWindowMinWidth, func() {
		r.Release(r.closeWindow(entry.window.ID()))
		r.rememberOpenWorktrees()
	})
	r.wireFocus(entry)
	r.windows[entry.window.ID()] = entry
}

// windowShowing returns the main window, other than exclude, already
// selecting wt, or nil: PickAndOpen's "brings forward".
func (r *Registry) windowShowing(wt core.WorktreeID, exclude uint) shellWindow {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, e := range r.windows {
		if id != exclude && e.selected == wt {
			return e.window
		}
	}
	return nil
}

// NewWindow opens a new main window with no worktree selected: unlike
// OpenOrFocus("", "", ""), which would focus one already open, this always
// opens another.
func (r *Registry) NewWindow() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.addWindowLocked(&windowEntry{})
}

// SelectInWindow moves windowID's entry to select wt and reports the
// previous selection, for the caller to release. ok is false if windowID
// names no known window.
func (r *Registry) SelectInWindow(windowID uint, wt core.WorktreeID, path, distro string) (previous core.WorktreeID, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, exists := r.windows[windowID]
	if !exists {
		return "", false
	}
	previous = e.selected
	e.selected, e.path, e.distro, e.repo = wt, path, distro, ""
	return previous, true
}

// inUse reports whether a main window selects wt or a diff window shows it.
func (r *Registry) inUse(wt core.WorktreeID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.ContainsFunc(r.allEntriesLocked(), func(e *windowEntry) bool { return e.selected == wt })
}

// allEntriesLocked lists the main and the diff windows' entries, and the
// pages'. Caller holds r.mu.
func (r *Registry) allEntriesLocked() []*windowEntry {
	return slices.Concat(
		slices.Collect(maps.Values(r.windows)),
		slices.Collect(maps.Values(r.diffWindows)),
		slices.Collect(maps.Values(r.pages)),
	)
}

// Release closes wt's session through its core once no window uses it any
// more; best-effort, nothing user-visible.
func (r *Registry) Release(wt core.WorktreeID) {
	if wt == "" || r.inUse(wt) {
		return
	}
	if c, id, err := r.CoreOf(r.ctx, wt); err == nil {
		_ = c.CloseWorktree(r.ctx, id)
	}
}

// closeWindow drops windowID's entry (a real OS close) and reports its
// former selection.
func (r *Registry) closeWindow(windowID uint) (closed core.WorktreeID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.windows[windowID]
	if !ok {
		return ""
	}
	delete(r.windows, windowID)
	return e.selected
}

// wireFocus relays e's window's OS focus to setFocused.
func (r *Registry) wireFocus(e *windowEntry) {
	e.window.OnWindowEvent(events.Common.WindowFocus, func(*application.WindowEvent) { go r.setFocused(e, true) })
	e.window.OnWindowEvent(events.Common.WindowLostFocus, func(*application.WindowEvent) { go r.setFocused(e, false) })
}

// setFocused records e's focus and tells the core of e's selection whether
// any window on it has focus: a main window and its diff window hand focus
// to each other, in either order. focusMu makes the core keep the answer
// to the last change. A window without a selection tells no one.
func (r *Registry) setFocused(e *windowEntry, focused bool) {
	r.focusMu.Lock()
	defer r.focusMu.Unlock()

	wt, anyFocused := r.recordFocus(e, focused)
	if wt == "" {
		return
	}
	if c, id, err := r.CoreOf(r.ctx, wt); err == nil {
		_ = c.SetFocused(r.ctx, id, anyFocused)
	}
}

// recordFocus stores e's focus and reports e's selection and whether any
// window on it has focus.
func (r *Registry) recordFocus(e *windowEntry, focused bool) (wt core.WorktreeID, anyFocused bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e.focused = focused
	anyFocused = slices.ContainsFunc(r.allEntriesLocked(), func(o *windowEntry) bool {
		return o.selected == e.selected && o.focused
	})
	return e.selected, anyFocused
}

// mainWindowMinWidth is the narrowest a main window can be resized to: the
// window body's own narrow layout goes down to this width. A diff window
// has no such floor.
const mainWindowMinWidth = 400

// newWindow creates a window that drops itself from the registry when the
// OS closes it (Wails destroys the window on WindowClosing). minWidth is 0
// for a window with no minimum.
func (r *Registry) newWindow(title, url string, minWidth int, onClose func()) shellWindow {
	window := r.opener.NewWithOptions(application.WebviewWindowOptions{
		Title:    title,
		URL:      url,
		Width:    1200,
		Height:   800,
		MinWidth: minWidth,
	})
	window.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { onClose() })
	return window
}

// OpenOrFocusDiff opens key's diff window on wt's target, or focuses it and
// shows the target there, releasing the worktree it showed before. A diff
// window is not a main window, but it keeps its worktree's session open and
// gets its events until it closes.
func (r *Registry) OpenOrFocusDiff(key diffWindowKey, wt core.WorktreeID, target DiffTarget) {
	w, previous := r.openOrRetargetDiff(key, wt, target)
	if w == nil {
		return
	}
	w.SetTitle(diffWindowTitle(wt, target))
	w.Focus()
	w.EmitEvent("diff:show", DiffShow{Wt: wt, Target: target})
	r.Release(previous)
}

// openOrRetargetDiff moves key's open diff window to wt and returns it with
// the worktree it showed before, or registers a new window on target and
// returns nil.
func (r *Registry) openOrRetargetDiff(key diffWindowKey, wt core.WorktreeID, target DiffTarget) (existing shellWindow, previous core.WorktreeID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if e, ok := r.diffWindows[key]; ok {
		previous = e.selected
		r.showLocked(e, wt)
		return e.window, previous
	}
	entry := &windowEntry{}
	r.showLocked(entry, wt)
	entry.window = r.newWindow(diffWindowTitle(wt, target), diffURL(wt, target), 0, func() {
		r.Release(r.closeDiffWindow(key))
	})
	r.wireFocus(entry)
	r.diffWindows[key] = entry
	return nil, ""
}

// showLocked makes e, a diff window or a page, show wt, with the path, repo
// and distro of the main window selecting it, so wt's events reach e.
// Caller holds r.mu.
func (r *Registry) showLocked(e *windowEntry, wt core.WorktreeID) {
	e.selected, e.path, e.repo, e.distro = wt, "", "", ""
	if main := r.findSelectingLocked(wt); main != nil {
		e.path, e.repo, e.distro = main.path, main.repo, main.distro
	}
}

// showInPage records that page shows wt, on its first call: a page never
// switches, and a later call may be about another worktree.
func (r *Registry) showInPage(page shellWindow, wt core.WorktreeID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.pages[page]; ok {
		return
	}
	entry := &windowEntry{window: page}
	r.showLocked(entry, wt)
	r.pages[page] = entry
}

// closeDiffWindow drops key's diff window and reports the worktree it
// showed.
func (r *Registry) closeDiffWindow(key diffWindowKey) core.WorktreeID {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.diffWindows[key]
	if !ok {
		return ""
	}
	delete(r.diffWindows, key)
	return e.selected
}

// Quitting marks the app as shutting down: the shutdown then closes every
// window, and those were all open at quit.
func (r *Registry) Quitting() { r.quitting.Store(true) }

// rememberOpenWorktrees records the windows still open for the next
// launch's restore. Closing the last window quits the app, so a close that
// leaves no window keeps that window recorded; so does every close during
// a shutdown.
func (r *Registry) rememberOpenWorktrees() {
	if r.store == nil || r.quitting.Load() || r.windowCount() == 0 {
		return
	}
	paths := r.OpenWorktreePaths()
	_ = r.store.Update(func(s *settings.Settings) { s.OpenWorktrees = paths })
}

func (r *Registry) windowCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.windows)
}

// OpenWorktreePaths lists every currently selected worktree's path, one per
// window that has one, for the restore-at-launch snapshot. A WSL window's
// path comes back in its UNC form, so restore's OpenWorktree call routes
// through CoreFor to the same distro.
func (r *Registry) OpenWorktreePaths() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	paths := make([]string, 0, len(r.windows))
	for _, entry := range r.windows {
		if entry.selected != "" {
			paths = append(paths, shellPath(entry.path, entry.distro))
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

// Dispatch is the local core's core.EventSink: an event with Worktree set
// goes to every window showing it, one with only Repo set to every local
// window of that repo.
func (r *Registry) Dispatch(ev core.Event) { r.dispatch("", ev) }

// sinkFor is distro's core.EventSink: it names the event's worktree by its
// shell id, then dispatches it among distro's windows.
func (r *Registry) sinkFor(distro string) core.EventSink {
	return func(ev core.Event) {
		if ev.Worktree != "" {
			ev.Worktree = core.WorktreeID(remote.BuildWSLPath(distro, string(ev.Worktree)))
		}
		r.dispatch(distro, ev)
	}
}

// dispatch emits ev to the windows it concerns among those distro backs
// ("" = local), its payload decoded into the event's registered type. An
// event that does not decode is dropped and logged. A "command" event never
// reaches a window: it is written to the app log instead.
func (r *Registry) dispatch(distro string, ev core.Event) {
	data, err := decodeEvent(ev)
	if err != nil {
		r.log.Warn("event dropped", "name", ev.Name, "err", err)
		return
	}
	if entry, ok := data.(core.CommandRecord); ok {
		logCommand(r.log, ev.Worktree, entry)
		return
	}
	if entry, ok := data.(core.EditorRecord); ok {
		logEditor(r.log, ev.Worktree, entry)
		return
	}
	for _, w := range r.targets(distro, ev) {
		w.EmitEvent(ev.Name, data)
	}
}

// targets picks the windows an event concerns; the native calls happen
// after the lock is released. A worktree id names its distro itself; a repo
// key does not.
func (r *Registry) targets(distro string, ev core.Event) []shellWindow {
	r.mu.Lock()
	defer r.mu.Unlock()

	var windows []shellWindow
	for _, entry := range r.allEntriesLocked() {
		switch {
		case ev.Worktree != "":
			if entry.selected == ev.Worktree { // every window sharing the selection
				windows = append(windows, entry.window)
			}
		case ev.Repo != "" && entry.repo == ev.Repo && entry.distro == distro:
			windows = append(windows, entry.window)
		}
	}
	return windows
}

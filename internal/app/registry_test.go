package app

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/remote/remotetest"
	"github.com/brogli/nebegit/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// emittedEvent is one EmitEvent call recorded by fakeWindow: name and
// payload, not just a count, so a test can assert what an event carried.
type emittedEvent struct {
	name    string
	payload any
}

func newEmittedEvent(name string, data []any) emittedEvent {
	ev := emittedEvent{name: name}
	if len(data) > 0 {
		ev.payload = data[0]
	}
	return ev
}

// fakeWindow is a shellWindow that records what was called on it, and lets
// a test simulate the OS focusing or closing it.
type fakeWindow struct {
	id         uint
	title      string
	url        string
	minWidth   int
	focusCount int
	emitted    []emittedEvent
	handlers   map[events.WindowEventType]func(*application.WindowEvent)
}

func (w *fakeWindow) ID() uint { return w.id }

func (w *fakeWindow) Focus() { w.focusCount++ }

func (w *fakeWindow) SetTitle(title string) application.Window {
	w.title = title
	return nil
}

func (w *fakeWindow) EmitEvent(name string, data ...any) bool {
	w.emitted = append(w.emitted, newEmittedEvent(name, data))
	return true
}

func (w *fakeWindow) emitCount() int { return len(w.emitted) }

func (w *fakeWindow) OnWindowEvent(eventType events.WindowEventType, callback func(*application.WindowEvent)) func() {
	if w.handlers == nil {
		w.handlers = map[events.WindowEventType]func(*application.WindowEvent){}
	}
	w.handlers[eventType] = callback
	return func() {}
}

// fire simulates the OS sending the window eventType.
func (w *fakeWindow) fire(eventType events.WindowEventType) {
	if handler := w.handlers[eventType]; handler != nil {
		handler(&application.WindowEvent{})
	}
}

// close simulates the OS destroying the window.
func (w *fakeWindow) close() { w.fire(events.Common.WindowClosing) }

// fakeOpener is a windowOpener that hands out fakeWindows and records them
// in creation order.
type fakeOpener struct {
	windows []*fakeWindow
}

func (o *fakeOpener) NewWithOptions(opts application.WebviewWindowOptions) shellWindow {
	w := &fakeWindow{id: uint(len(o.windows)) + 1, title: opts.Title, url: opts.URL, minWidth: opts.MinWidth}
	o.windows = append(o.windows, w)
	return w
}

// worktreeEvent and repoEvent are registered events for the windows of a
// worktree and of a repo. "command" is not used here: it never reaches a
// window, so a routing test needs an event that does.
func worktreeEvent(wt core.WorktreeID) core.Event {
	return core.Event{Name: "log:progress", Worktree: wt, Payload: json.RawMessage(`{}`)}
}

func repoEvent(repo string) core.Event {
	return core.Event{Name: "job:update", Repo: repo, Payload: json.RawMessage(`{}`)}
}

func newTestRegistry() (*Registry, *fakeOpener) {
	opener := &fakeOpener{}
	r := NewRegistry()
	r.opener = opener
	r.SetLocal(fakeAPI{})
	return r, opener
}

func TestFR_UX_08_MainWindowGetsTheMinimumWidthADiffWindowDoesNot(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("wt", "/wt", "")
	r.OpenOrFocusDiff(diffWindowKey{wt: "wt"}, "wt", DiffTarget{Kind: CommitDiff, Sha: "a", Path: "f1"})

	if len(opener.windows) != 2 {
		t.Fatalf("opened %d windows, want 2", len(opener.windows))
	}
	if got := opener.windows[0].minWidth; got != mainWindowMinWidth {
		t.Errorf("main window MinWidth = %d, want %d", got, mainWindowMinWidth)
	}
	if got := opener.windows[1].minWidth; got != 0 {
		t.Errorf("diff window MinWidth = %d, want 0 (unset)", got)
	}
}

func TestFR_WT_05_OpenOrFocusOpensADistinctWindowPerPath(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("/a", "/a", "")
	r.OpenOrFocus("/b", "/b", "")

	if len(opener.windows) != 2 {
		t.Fatalf("windows created = %d, want 2", len(opener.windows))
	}
	if opener.windows[0].focusCount != 0 {
		t.Errorf("/a's window focused %d times, want 0", opener.windows[0].focusCount)
	}
}

func TestFR_WT_08_OpenOrFocusFocusesAnExistingWindow(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("/a", "/a", "")
	r.OpenOrFocus("/a", "/a", "")

	if len(opener.windows) != 1 {
		t.Fatalf("windows created = %d, want 1", len(opener.windows))
	}
	if got := opener.windows[0].focusCount; got != 1 {
		t.Errorf("focus count = %d, want 1", got)
	}
}

func TestADR_9_DispatchTargetsTheWindowsAnEventConcerns(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("wtA", "wtA", "")
	r.OpenOrFocus("wtB", "wtB", "")
	r.OpenOrFocus("wtC", "wtC", "")
	r.SetRepo("wtA", "R")
	r.SetRepo("wtB", "R")
	r.SetRepo("wtC", "S")
	wtA, wtB, wtC := opener.windows[0], opener.windows[1], opener.windows[2]

	r.Dispatch(worktreeEvent("wtA"))

	if wtA.emitCount() != 1 {
		t.Errorf("wtA emitCount = %d, want 1", wtA.emitCount())
	}
	if wtB.emitCount() != 0 || wtC.emitCount() != 0 {
		t.Errorf("wtB, wtC emitCount = %d, %d, want 0, 0", wtB.emitCount(), wtC.emitCount())
	}

	r.Dispatch(repoEvent("R"))

	if wtA.emitCount() != 2 || wtB.emitCount() != 1 {
		t.Errorf("wtA, wtB emitCount = %d, %d, want 2, 1", wtA.emitCount(), wtB.emitCount())
	}
	if wtC.emitCount() != 0 {
		t.Errorf("wtC emitCount = %d, want 0", wtC.emitCount())
	}
}

// SetRepo/targets route a repo-scoped event by current selection: only the
// window whose selection belongs to that repo.
func TestFR_WT_11_DispatchByRepoReachesOnlyTheWindowSelectingThatRepo(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("A", "/a", "")
	r.OpenOrFocus("B", "/b", "")
	r.SetRepo("A", "R1")
	r.SetRepo("B", "R2")

	r.Dispatch(repoEvent("R1"))

	if opener.windows[0].emitCount() != 1 {
		t.Errorf("window selecting R1's worktree emitCount = %d, want 1", opener.windows[0].emitCount())
	}
	if opener.windows[1].emitCount() != 0 {
		t.Errorf("window selecting R2's worktree emitCount = %d, want 0", opener.windows[1].emitCount())
	}
}

// Wails hands every event to every window: only the window an event is
// emitted through is named as its sender, so it goes out through its own
// window alone.
func TestFR_SYNC_07_AJobGoesOutOnlyThroughItsWorktreesWindow(t *testing.T) {
	r, opener := newTestRegistry()
	r.OpenOrFocus("/a", "/a", "")
	r.OpenOrFocus("/b", "/b", "")
	r.SetRepo("/a", "/a/.git")
	r.SetRepo("/b", "/b/.git")
	first, second := opener.windows[0], opener.windows[1]

	r.Dispatch(core.Event{Name: "job:update", Repo: "/a/.git", Payload: json.RawMessage(`{"id":"j1"}`)})

	if got, want := eventNames(first), []string{"job:update"}; !slices.Equal(got, want) {
		t.Errorf("first window emitted %v, want %v", got, want)
	}
	if len(second.emitted) != 0 {
		t.Errorf("second window emitted %v, want nothing", eventNames(second))
	}
}

// A server-mode page is no window of the registry's: its calls name it,
// and from its first call on it gets the events of the worktree it asked
// for, and of that worktree's repo, through itself.
func TestFR_REPO_04_APageGetsTheEventsOfTheWorktreeItCallsFor(t *testing.T) {
	r, opener := newTestRegistry()
	r.OpenOrFocus("/a", "/a", "")
	r.OpenOrFocus("/b", "/b", "")
	r.SetRepo("/a", "/a/.git")
	page := &fakeWindow{}
	ctx := context.WithValue(t.Context(), pageKey{}, shellWindow(page))

	if _, _, err := r.CoreOf(ctx, "/a"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.CoreOf(ctx, "/b"); err != nil {
		t.Fatal(err)
	}
	r.Dispatch(worktreeEvent("/a"))
	r.Dispatch(repoEvent("/a/.git"))
	r.Dispatch(worktreeEvent("/b"))

	if got, want := eventNames(page), []string{"log:progress", "job:update"}; !slices.Equal(got, want) {
		t.Errorf("page emitted %v, want %v", got, want)
	}
	if got := eventNames(opener.windows[0]); len(got) != 2 {
		t.Errorf("/a's window emitted %v, want both of /a's events", got)
	}
}

func TestFR_REPO_02_ClosingAWindowLetsOpenOrFocusCreateANewOne(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("X", "/x", "")
	opener.windows[0].close()
	r.OpenOrFocus("X", "/x", "")

	if len(opener.windows) != 2 {
		t.Fatalf("windows created = %d, want 2", len(opener.windows))
	}
	if opener.windows[0].focusCount != 0 {
		t.Errorf("closed window focused %d times, want 0", opener.windows[0].focusCount)
	}
}

// An empty selection ("" = the panel's empty state) is just another
// selection, deduped by findSelectingLocked the same way a worktree id is.
func TestADR_9_OpenOrFocusDedupsTheEmptySelection(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("", "", "")
	r.OpenOrFocus("", "", "")

	if len(opener.windows) != 1 {
		t.Fatalf("windows created = %d, want 1", len(opener.windows))
	}
	if opener.windows[0].focusCount != 1 {
		t.Errorf("focus count = %d, want 1", opener.windows[0].focusCount)
	}
}

func TestADR_9_OpenOrFocusReopensTheEmptySelectionAfterItIsClosed(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("", "", "")
	opener.windows[0].close()
	r.OpenOrFocus("", "", "")

	if len(opener.windows) != 2 {
		t.Fatalf("windows created = %d, want 2", len(opener.windows))
	}
}

func TestFR_REPO_02_OpenWorktreePathsListsOpenWindows(t *testing.T) {
	r, _ := newTestRegistry()

	r.OpenOrFocus("a", "/a", "")
	r.OpenOrFocus("b", "/b", "")

	got := r.OpenWorktreePaths()
	slices.Sort(got)
	if want := []string{"/a", "/b"}; !slices.Equal(got, want) {
		t.Errorf("OpenWorktreePaths() = %v, want %v", got, want)
	}
}

// A WSL window's path comes back as its UNC form, so restore's OpenWorktree
// call routes through Registry.CoreFor to the same distro instead of the
// local core.
func TestFR_REPO_02_OpenWorktreePathsBuildsUNCForADistroWindow(t *testing.T) {
	r, _ := newTestRegistry()

	r.OpenOrFocus("wt", "/home/u/r", "Ubuntu")

	got := r.OpenWorktreePaths()
	if want := []string{`\\wsl.localhost\Ubuntu\home\u\r`}; !slices.Equal(got, want) {
		t.Errorf("OpenWorktreePaths() = %v, want %v", got, want)
	}
}

func TestFR_REPO_02_OpenOrFocusBuildsAURLWithTheDistro(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("wt", "/home/u/r", "Ubuntu")

	if want := "/?path=%2Fhome%2Fu%2Fr&distro=Ubuntu"; opener.windows[0].url != want {
		t.Errorf("url = %q, want %q", opener.windows[0].url, want)
	}
}

func TestQA_PERF_06_ClosingAWindowCallsCloseWorktreeOnTheLocalCore(t *testing.T) {
	r, opener := newTestRegistry()
	var closed []core.WorktreeID
	r.SetLocal(fakeAPI{closed: &closed})

	r.OpenOrFocus("X", "/x", "")
	opener.windows[0].close()

	if len(closed) != 1 || closed[0] != "X" {
		t.Errorf("CloseWorktree calls = %v, want [\"X\"]", closed)
	}
}

func TestFR_REPO_02_OpenWorktreePathsDropsAClosedWindow(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("a", "/a", "")
	opener.windows[0].close()

	if got := r.OpenWorktreePaths(); len(got) != 0 {
		t.Errorf("OpenWorktreePaths() = %v, want empty", got)
	}
}

func TestFR_REPO_02_ClosingAWindowForgetsItForTheNextLaunch(t *testing.T) {
	r, opener := newTestRegistry()
	store := newTestStore(t)
	r.SetStore(store)
	r.OpenOrFocus("a", "/a", "")
	r.OpenOrFocus("b", "/b", "")

	opener.windows[0].close()

	if got := store.Get().OpenWorktrees; !slices.Equal(got, []string{"/b"}) {
		t.Errorf("OpenWorktrees = %v, want [/b]", got)
	}
}

// Closing the last window quits the app: that window was open at quit.
func TestFR_REPO_02_ClosingTheLastWindowKeepsItForTheNextLaunch(t *testing.T) {
	r, opener := newTestRegistry()
	store := newTestStore(t)
	r.SetStore(store)
	r.OpenOrFocus("a", "/a", "")
	if err := store.Update(func(s *settings.Settings) { s.OpenWorktrees = []string{"/a"} }); err != nil {
		t.Fatal(err)
	}

	opener.windows[0].close()

	if got := store.Get().OpenWorktrees; !slices.Equal(got, []string{"/a"}) {
		t.Errorf("OpenWorktrees = %v, want [/a]", got)
	}
}

// Quitting closes every window through the same close event a user's close
// fires; the app's shutdown hook runs first.
func TestFR_REPO_02_QuittingKeepsEveryOpenWindowForTheNextLaunch(t *testing.T) {
	r, opener := newTestRegistry()
	store := newTestStore(t)
	r.SetStore(store)
	r.OpenOrFocus("a", "/a", "")
	r.OpenOrFocus("b", "/b", "")
	if err := store.Update(func(s *settings.Settings) { s.OpenWorktrees = []string{"/a", "/b"} }); err != nil {
		t.Fatal(err)
	}

	options(fstest.MapFS{}, r, func(string) {}, nil).OnShutdown()
	opener.windows[0].close()
	opener.windows[1].close()

	if got := store.Get().OpenWorktrees; !slices.Equal(got, []string{"/a", "/b"}) {
		t.Errorf("OpenWorktrees = %v, want [/a /b]", got)
	}
}

// pipeAPIClient serves api over an in-memory pipe: these WSL routing
// tests exercise the real generated client/dispatcher without a distro.
func pipeAPIClient(t *testing.T, api core.API) *remote.Client {
	t.Helper()
	return remotetest.NewPipeClient(t, func(core.EventSink) core.API { return api }, nil)
}

// serveDistroCores makes every distro core r creates serve api over one
// pipe, and returns that pipe's client.
func serveDistroCores(t *testing.T, r *Registry, api core.API) *remote.Client {
	t.Helper()
	client := pipeAPIClient(t, api)
	r.newDistroCore = func(string, core.EventSink) distroCorer { return &fakeDistroCore{client: client} }
	return client
}

// fakeDistroCore is a distroCorer that always returns client, reporting
// respawned only on its first Ensure call, and calling onConnect on that
// same first call, the way the real DistroCore.Ensure fires it.
type fakeDistroCore struct {
	client    *remote.Client
	calls     int
	onConnect func(context.Context, *remote.Client)
}

func (f *fakeDistroCore) Ensure(ctx context.Context) (*remote.Client, bool, error) {
	f.calls++
	if f.calls == 1 && f.onConnect != nil {
		f.onConnect(ctx, f.client)
	}
	return f.client, f.calls == 1, nil
}

func (f *fakeDistroCore) OnConnect(fn func(context.Context, *remote.Client)) { f.onConnect = fn }

// LiveClient reports a live connection once Ensure has run at least once
// (calls > 0), whether for real or seeded by a test as "already connected".
func (f *fakeDistroCore) LiveClient() (*remote.Client, bool) {
	return f.client, f.calls > 0
}

func TestADR_3_CoreForRoutesAWSLPathToItsDistro(t *testing.T) {
	r, _ := newTestRegistry()
	serveDistroCores(t, r, fakeAPI{})

	got, err := r.CoreFor(core.RepoPath(`\\wsl.localhost\Ubuntu-22.04\home\user\project`))
	if err != nil {
		t.Fatal(err)
	}
	if _, isLocal := got.(fakeAPI); isLocal {
		t.Error("CoreFor(WSL path) returned Local()")
	}

	local, err := r.CoreFor(core.RepoPath("/local/path"))
	if err != nil {
		t.Fatal(err)
	}
	if _, isLocal := local.(fakeAPI); !isLocal {
		t.Error("CoreFor(local path) did not return Local()")
	}
}

func TestADR_3_CoreOfRoutesByTheIDAlone(t *testing.T) {
	r, _ := newTestRegistry()
	distroClient := serveDistroCores(t, r, fakeAPI{})

	local, localID, err := r.CoreOf(t.Context(), "/local/r")
	if err != nil {
		t.Fatal(err)
	}
	if _, isLocal := local.(fakeAPI); !isLocal || localID != "/local/r" {
		t.Errorf("CoreOf(/local/r) = (%T, %q), want (the local core, /local/r)", local, localID)
	}

	wsl, wslID, err := r.CoreOf(t.Context(), `\\wsl.localhost\A\home\u\r`)
	if err != nil {
		t.Fatal(err)
	}
	if wsl != core.API(distroClient) || wslID != "/home/u/r" {
		t.Errorf(`CoreOf(\\wsl.localhost\A\home\u\r) = (%T, %q), want (A's core, /home/u/r)`, wsl, wslID)
	}
}

// newTwoDistroShell is a shell over a fresh registry with distros A and B,
// each with a repo at /home/u/r. calls receives each distro core's calls;
// sinks holds the event sink the registry gave each distro core.
func newTwoDistroShell(t *testing.T) (sh shell, opener *fakeOpener, calls map[string]chan routedCall, sinks map[string]core.EventSink) {
	t.Helper()
	r, opener := newTestRegistry()
	calls = map[string]chan routedCall{}
	clients := map[string]*remote.Client{}
	for _, distro := range []string{"A", "B"} {
		calls[distro] = make(chan routedCall, 1)
		clients[distro] = pipeAPIClient(t, fakeAPI{calls: calls[distro], openWorktree: func(context.Context, core.RepoPath) (core.WorktreeInfo, error) {
			return core.WorktreeInfo{ID: "/home/u/r", Path: "/home/u/r", RepoKey: "/home/u/r/.git"}, nil
		}})
	}
	sinks = map[string]core.EventSink{}
	r.newDistroCore = func(distro string, sink core.EventSink) distroCorer {
		sinks[distro] = sink
		return &fakeDistroCore{client: clients[distro]}
	}
	sh = shell{api: NewForwarder(r), reg: r, store: newTestStore(t), notify: &fakeNotifier{}}
	ctx := t.Context()
	sh.open(ctx, `\\wsl.localhost\A\home\u\r`)
	sh.open(ctx, `\\wsl.localhost\B\home\u\r`)
	return sh, opener, calls, sinks
}

func TestFR_WT_11_SamePathInTwoDistrosOpensTwoWindowsEachOnItsOwnCore(t *testing.T) {
	sh, opener, calls, _ := newTwoDistroShell(t)

	if len(opener.windows) != 2 {
		t.Fatalf("windows created = %d, want 2", len(opener.windows))
	}
	for _, distro := range []string{"A", "B"} {
		id := core.WorktreeID(`\\wsl.localhost\` + distro + `\home\u\r`)
		if _, err := sh.api.Summary(t.Context(), id); err != nil {
			t.Fatal(err)
		}
		if got, want := nextCall(t, calls[distro]), (routedCall{"Summary", "/home/u/r"}); got != want {
			t.Errorf("Summary(%s): %s's core got %+v, want %+v", id, distro, got, want)
		}
	}
}

func TestFR_WT_11_ADistroCoresEventsReachOnlyItsOwnWindow(t *testing.T) {
	_, opener, _, sinks := newTwoDistroShell(t)
	windowA, windowB := opener.windows[0], opener.windows[1]

	sinks["A"](worktreeEvent("/home/u/r"))
	sinks["A"](repoEvent("/home/u/r/.git"))

	if got := eventNames(windowA); !slices.Equal(got, []string{"log:progress", "job:update"}) {
		t.Errorf("A's window received %v, want both events", got)
	}
	if got := eventNames(windowB); len(got) != 0 {
		t.Errorf("B's window received %v, want none", got)
	}
}

func TestFR_REPO_04_FocusReachesTheWindowsCurrentSelection(t *testing.T) {
	calls := make(chan routedCall, 1)
	sh, opener := newLocalShell(t, nil, calls)
	ctx := t.Context()
	sh.open(ctx, "/wt")
	window := opener.windows[0]
	if err := sh.switchTo(ctx, &fakeCallingWindow{id: window.ID()}, "/wt2"); err != nil {
		t.Fatal(err)
	}

	window.fire(events.Common.WindowFocus)

	if got, want := nextCall(t, calls), (routedCall{"SetFocused(true)", "/wt2"}); got != want {
		t.Errorf("local core got %+v, want %+v", got, want)
	}
}

func TestFR_REPO_04_AWSLWindowsFocusReachesItsDistroCore(t *testing.T) {
	_, opener, calls, _ := newTwoDistroShell(t)

	opener.windows[0].fire(events.Common.WindowFocus)

	if got, want := nextCall(t, calls["A"]), (routedCall{"SetFocused(true)", "/home/u/r"}); got != want {
		t.Errorf("A's core got %+v, want %+v", got, want)
	}
}

func eventNames(w *fakeWindow) []string {
	var names []string
	for _, ev := range w.emitted {
		names = append(names, ev.name)
	}
	return names
}

func TestADR_4_CoreOfReopensEveryWindowOnItsDistroAfterARespawn(t *testing.T) {
	r, opener := newTestRegistry()

	var mu sync.Mutex
	var opened []string
	distroClient := pipeAPIClient(t, fakeAPI{openWorktree: func(_ context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
		mu.Lock()
		opened = append(opened, string(path))
		mu.Unlock()
		return core.WorktreeInfo{Path: string(path)}, nil
	}})
	fdc := &fakeDistroCore{client: distroClient}
	r.newDistroCore = func(string, core.EventSink) distroCorer { return fdc }

	r.OpenOrFocus(`\\wsl.localhost\Ubuntu-22.04\a`, "/a", "Ubuntu-22.04")
	r.OpenOrFocus(`\\wsl.localhost\Ubuntu-22.04\b`, "/b", "Ubuntu-22.04")
	r.OpenOrFocus(`\\wsl.localhost\Debian\c`, "/c", "Debian") // different distro; must stay untouched

	if _, _, err := r.CoreOf(t.Context(), `\\wsl.localhost\Ubuntu-22.04\a`); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	got := slices.Clone(opened)
	mu.Unlock()
	slices.Sort(got)
	if want := []string{"/a", "/b"}; !slices.Equal(got, want) {
		t.Errorf("reopened paths = %v, want %v", got, want)
	}

	wtA, wtB, wtC := opener.windows[0], opener.windows[1], opener.windows[2]
	if wtA.emitCount() != 1 || wtB.emitCount() != 1 {
		t.Errorf("emitCount wtA=%d wtB=%d, want 1, 1", wtA.emitCount(), wtB.emitCount())
	}
	if wtC.emitCount() != 0 {
		t.Errorf("wtC (different distro) emitCount = %d, want 0", wtC.emitCount())
	}
}

// A path call (opening another folder of the distro) can be the first to
// find the core dead; the distro's windows are reopened and told all the
// same.
func TestADR_4_CoreForReopensEveryWindowOnItsDistroAfterARespawn(t *testing.T) {
	r, opener := newTestRegistry()
	var mu sync.Mutex
	var opened []string
	serveDistroCores(t, r, fakeAPI{openWorktree: func(_ context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
		mu.Lock()
		defer mu.Unlock()
		opened = append(opened, string(path))
		return core.WorktreeInfo{Path: string(path)}, nil
	}})
	r.OpenOrFocus(`\\wsl.localhost\Ubuntu-22.04\a`, "/a", "Ubuntu-22.04")

	if _, err := r.CoreFor(`\\wsl.localhost\Ubuntu-22.04\other`); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if want := []string{"/a"}; !slices.Equal(opened, want) {
		t.Errorf("reopened paths = %v, want %v", opened, want)
	}
	if got := eventNames(opener.windows[0]); !slices.Equal(got, []string{"core:reconnected"}) {
		t.Errorf("events = %v, want [core:reconnected]", got)
	}
}

func TestADR_9_SelectInWindowMovesTheWindowsSelection(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("A", "/a", "")

	previous, ok := r.SelectInWindow(opener.windows[0].ID(), "B", "/b", "")
	if previous != "A" || !ok {
		t.Fatalf("SelectInWindow(...) = (%q, %v), want (\"A\", true)", previous, ok)
	}
	if r.inUse("A") || !r.inUse("B") {
		t.Errorf("inUse(A), inUse(B) = %v, %v, want false, true", r.inUse("A"), r.inUse("B"))
	}
}

func TestADR_9_SwitchingReleasesThePreviousSelectionThroughItsOwnCore(t *testing.T) {
	r, opener := newTestRegistry()
	var closed []core.WorktreeID
	serveDistroCores(t, r, fakeAPI{closed: &closed})

	r.OpenOrFocus(`\\wsl.localhost\Ubuntu-22.04\a`, "/a", "Ubuntu-22.04")
	previous, ok := r.SelectInWindow(opener.windows[0].ID(), "B", "/b", "")
	if !ok {
		t.Fatal("SelectInWindow ok = false")
	}
	r.Release(previous)

	if want := []core.WorktreeID{"/a"}; !slices.Equal(closed, want) {
		t.Errorf("distro core's CloseWorktree calls = %v, want %v", closed, want)
	}
}

// Two windows sharing a selection both receive a worktree-scoped event.
func TestADR_9_DispatchReachesEveryWindowSharingASelection(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("A", "/a", "")
	r.OpenOrFocus("other", "/other", "")
	if _, ok := r.SelectInWindow(opener.windows[1].ID(), "A", "/a", ""); !ok {
		t.Fatal("SelectInWindow ok = false")
	}

	r.Dispatch(worktreeEvent("A"))

	if opener.windows[0].emitCount() != 1 || opener.windows[1].emitCount() != 1 {
		t.Errorf("emitCount = %d, %d, want 1, 1", opener.windows[0].emitCount(), opener.windows[1].emitCount())
	}
}

// A switch releases a worktree only when inUse is false.
func TestADR_9_InUseIsTrueWhenAnotherWindowSharesTheSelection(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("A", "/a", "")
	r.OpenOrFocus("other", "/other", "")
	r.SelectInWindow(opener.windows[1].ID(), "A", "/a", "")

	previous, ok := r.SelectInWindow(opener.windows[0].ID(), "B", "/b", "")
	if !ok || previous != "A" {
		t.Fatalf("SelectInWindow(...) = (%q, %v), want (\"A\", true)", previous, ok)
	}

	if !r.inUse(previous) {
		t.Error("inUse(A) = false, want true — another window still selects it")
	}
}

// Closing a window's own OS window (not a switch) releases its selection
// through the real core too, exercised through a real WindowClosing.
func TestADR_9_ClosingAWindowClosesItsSelectionThroughTheRealCore(t *testing.T) {
	r, opener := newTestRegistry()
	var closed []core.WorktreeID
	serveDistroCores(t, r, fakeAPI{closed: &closed})

	r.OpenOrFocus(`\\wsl.localhost\Ubuntu-22.04\a`, "/a", "Ubuntu-22.04")
	opener.windows[0].close()

	if want := []core.WorktreeID{"/a"}; !slices.Equal(closed, want) {
		t.Errorf("distro core's CloseWorktree calls = %v, want %v", closed, want)
	}
}

// Closing one window sharing a selection does not release it, and
// leaves the other window's own entry untouched — it still receives events
// for that selection afterwards.
func TestADR_9_ClosingAWindowDoesNotCloseASelectionStillHeldElsewhere(t *testing.T) {
	r, opener := newTestRegistry()
	var closed []core.WorktreeID
	r.SetLocal(fakeAPI{closed: &closed})

	r.OpenOrFocus("A", "/a", "")
	r.OpenOrFocus("other", "/other", "")
	r.SelectInWindow(opener.windows[1].ID(), "A", "/a", "")

	opener.windows[0].close()

	if len(closed) != 0 {
		t.Errorf("CloseWorktree calls = %v, want none (window 1 still selects A)", closed)
	}
	r.Dispatch(worktreeEvent("A"))
	if opener.windows[1].emitCount() != 1 {
		t.Errorf("window 1 (still selecting A) emitCount = %d, want 1 — its own entry must be untouched", opener.windows[1].emitCount())
	}
}

// newLocalShell is a shell over a fresh registry whose local core opens
// every path as is, records CloseWorktree calls in closed and its
// recorded calls on calls.
func newLocalShell(t *testing.T, closed *[]core.WorktreeID, calls chan<- routedCall) (shell, *fakeOpener) {
	t.Helper()
	r, opener := newTestRegistry()
	r.SetLocal(fakeAPI{openWorktree: openAsIs, closed: closed, calls: calls})
	return shell{api: NewForwarder(r), reg: r, store: newTestStore(t), notify: &fakeNotifier{}}, opener
}

// openDiffFrom opens mainWindow's diff window on wt's working-tree changes.
func openDiffFrom(t *testing.T, sh shell, mainWindow *fakeWindow, wt core.WorktreeID) {
	t.Helper()
	s := &AppService{shell: sh}
	s.OpenDiff(callFrom(mainWindow.ID()), wt, DiffTarget{Kind: WorkingDiff, Path: "f"})
}

func TestFR_DIFF_07_ADiffWindowOutlivesItsMainWindowAndGetsItsWorktreesEvents(t *testing.T) {
	var closed []core.WorktreeID
	sh, opener := newLocalShell(t, &closed, nil)
	sh.open(t.Context(), "/wt")
	sh.reg.SetRepo("/wt", "/wt/.git")
	mainWindow := opener.windows[0]
	openDiffFrom(t, sh, mainWindow, "/wt")
	diffWindow := opener.windows[1]

	mainWindow.close()
	index := json.RawMessage(`{"scope":"index"}`)
	sh.reg.Dispatch(core.Event{Name: "repo:changed", Repo: "/wt/.git", Payload: index})
	sh.reg.Dispatch(core.Event{Name: "repo:changed", Worktree: "/wt", Payload: index})

	want := []emittedEvent{
		{"repo:changed", core.RepoChanged{Scope: core.IndexChanged}},
		{"repo:changed", core.RepoChanged{Scope: core.IndexChanged}},
	}
	if !slices.Equal(diffWindow.emitted, want) {
		t.Errorf("diff window received %+v, want %+v", diffWindow.emitted, want)
	}
	if len(closed) != 0 {
		t.Errorf("CloseWorktree calls = %v, want none while the diff window is open", closed)
	}

	diffWindow.close()

	if want := []core.WorktreeID{"/wt"}; !slices.Equal(closed, want) {
		t.Errorf("CloseWorktree calls = %v, want %v", closed, want)
	}
}

func TestFR_DIFF_07_RetargetingTheDiffWindowToAnotherWorktreeReleasesTheOneLeft(t *testing.T) {
	var closed []core.WorktreeID
	sh, opener := newLocalShell(t, &closed, nil)
	ctx := t.Context()
	sh.open(ctx, "/wt")
	mainWindow := opener.windows[0]
	openDiffFrom(t, sh, mainWindow, "/wt")

	if err := sh.switchTo(ctx, &fakeCallingWindow{id: mainWindow.ID()}, "/wt2"); err != nil {
		t.Fatal(err)
	}
	if len(closed) != 0 {
		t.Errorf("CloseWorktree calls = %v, want none while the diff window shows /wt", closed)
	}

	openDiffFrom(t, sh, mainWindow, "/wt2")

	if want := []core.WorktreeID{"/wt"}; !slices.Equal(closed, want) {
		t.Errorf("CloseWorktree calls = %v, want %v", closed, want)
	}
	diffWindow := opener.windows[1]
	if got := diffWindow.emitted[0].payload.(DiffShow).Wt; got != "/wt2" {
		t.Errorf("diff:show wt = %q, want /wt2", got)
	}
}

// A main window and its diff window hand focus to each other in either
// order; the worktree's status poller runs while either has focus.
func TestFR_REPO_04_FocusingTheDiffWindowKeepsItsWorktreePolled(t *testing.T) {
	calls := make(chan routedCall, 1)
	sh, opener := newLocalShell(t, nil, calls)
	sh.open(t.Context(), "/wt")
	mainWindow := opener.windows[0]
	openDiffFrom(t, sh, mainWindow, "/wt")
	diffWindow := opener.windows[1]

	steps := []struct {
		window    *fakeWindow
		event     events.WindowEventType
		wantFocus bool
	}{
		{mainWindow, events.Common.WindowFocus, true},
		{diffWindow, events.Common.WindowFocus, true},
		{mainWindow, events.Common.WindowLostFocus, true},
		{mainWindow, events.Common.WindowFocus, true},
		{diffWindow, events.Common.WindowLostFocus, true},
		{mainWindow, events.Common.WindowLostFocus, false},
	}
	for i, step := range steps {
		step.window.fire(step.event)
		want := routedCall{fmt.Sprintf("SetFocused(%t)", step.wantFocus), "/wt"}
		if got := nextCall(t, calls); got != want {
			t.Errorf("step %d: local core got %+v, want %+v", i, got, want)
		}
	}
}

// setEditorProfiles saves profiles into a fresh test store, for the
// PushEditorProfiles tests below.
func setEditorProfiles(t *testing.T, store *settings.Store, profiles map[string]core.EditorProfile) {
	t.Helper()
	if err := store.Update(func(s *settings.Settings) { s.EditorProfiles = profiles }); err != nil {
		t.Fatal(err)
	}
}

func TestTR_13_PushEditorProfilesPushesTheLocalCoresProfile(t *testing.T) {
	r, _ := newTestRegistry()
	var pushed []core.EditorProfile
	r.SetLocal(fakeAPI{editorProfiles: &pushed})
	store := newTestStore(t)
	setEditorProfiles(t, store, map[string]core.EditorProfile{"": {Name: "another editor", FolderCmd: "other {path}"}})
	r.SetStore(store)

	r.PushEditorProfiles(t.Context())

	if len(pushed) != 1 || pushed[0].Name != "another editor" {
		t.Errorf("local core received %+v, want exactly one push of another editor", pushed)
	}
}

func TestFR_ED_11_CoreForPushesTheDistrosOwnProfileOnFirstConnect(t *testing.T) {
	r, _ := newTestRegistry()
	var pushed []core.EditorProfile
	serveDistroCores(t, r, fakeAPI{editorProfiles: &pushed})
	store := newTestStore(t)
	setEditorProfiles(t, store, map[string]core.EditorProfile{
		"":             {Name: "the editor", FolderCmd: "editor {path}"},
		"Ubuntu-22.04": {Name: "another editor", FolderCmd: "other {path}"},
	})
	r.SetStore(store)

	if _, err := r.CoreFor(core.RepoPath(`\\wsl.localhost\Ubuntu-22.04\home\user\project`)); err != nil {
		t.Fatal(err)
	}

	if len(pushed) != 1 || pushed[0].Name != "another editor" {
		t.Errorf("distro core received %+v, want exactly one push of another editor (its own profile, not local's)", pushed)
	}
}

func TestFR_ED_11_PushEditorProfilesOnlyPushesToAlreadyConnectedDistros(t *testing.T) {
	r, _ := newTestRegistry()
	var pushedConnected []core.EditorProfile
	connectedClient := pipeAPIClient(t, fakeAPI{editorProfiles: &pushedConnected})
	connected := &fakeDistroCore{client: connectedClient, calls: 1} // already connected, as if by an earlier Ensure
	notConnected := &fakeDistroCore{}                               // calls == 0: never connected

	r.mu.Lock()
	r.distros["Connected"] = connected
	r.distros["NotConnected"] = notConnected
	r.mu.Unlock()

	store := newTestStore(t)
	setEditorProfiles(t, store, map[string]core.EditorProfile{
		"Connected":    {Name: "another editor", FolderCmd: "other {path}"},
		"NotConnected": {Name: "a third editor", FolderCmd: "myed {path}"},
	})
	r.SetStore(store)

	r.PushEditorProfiles(t.Context())

	if len(pushedConnected) != 1 || pushedConnected[0].Name != "another editor" {
		t.Errorf("connected distro received %+v, want exactly one push of another editor", pushedConnected)
	}
	if notConnected.calls != 0 {
		t.Errorf("NotConnected.calls = %d, want 0 (PushEditorProfiles must not trigger Ensure/deploy)", notConnected.calls)
	}
}

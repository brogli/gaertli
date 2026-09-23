package app

import (
	"context"
	"slices"
	"sort"
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

// fakeWindow is a shellWindow that records what was called on it, and lets
// a test simulate the OS closing it.
type fakeWindow struct {
	id         uint
	title      string
	url        string
	focusCount int
	emitCount  int
	emitted    []emittedEvent
	closing    func(*application.WindowEvent)
}

func (w *fakeWindow) ID() uint { return w.id }

func (w *fakeWindow) Focus() { w.focusCount++ }

func (w *fakeWindow) EmitEvent(name string, data ...any) bool {
	w.emitCount++
	var payload any
	if len(data) > 0 {
		payload = data[0]
	}
	w.emitted = append(w.emitted, emittedEvent{name: name, payload: payload})
	return true
}

func (w *fakeWindow) OnWindowEvent(eventType events.WindowEventType, callback func(*application.WindowEvent)) func() {
	if eventType == events.Common.WindowClosing {
		w.closing = callback
	}
	return func() {}
}

// close simulates the OS destroying the window.
func (w *fakeWindow) close() {
	if w.closing != nil {
		w.closing(&application.WindowEvent{})
	}
}

// fakeOpener is a windowOpener that hands out fakeWindows and records them
// in creation order.
type fakeOpener struct {
	windows []*fakeWindow
}

func (o *fakeOpener) NewWithOptions(opts application.WebviewWindowOptions) shellWindow {
	w := &fakeWindow{id: uint(len(o.windows)) + 1, title: opts.Title, url: opts.URL}
	o.windows = append(o.windows, w)
	return w
}

func newTestRegistry() (*Registry, *fakeOpener) {
	opener := &fakeOpener{}
	r := NewRegistry()
	r.opener = opener
	r.SetLocal(fakeAPI{})
	return r, opener
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

	r.Dispatch(core.Event{Worktree: "wtA"})

	if wtA.emitCount != 1 {
		t.Errorf("wtA emitCount = %d, want 1", wtA.emitCount)
	}
	if wtB.emitCount != 0 || wtC.emitCount != 0 {
		t.Errorf("wtB, wtC emitCount = %d, %d, want 0, 0", wtB.emitCount, wtC.emitCount)
	}

	r.Dispatch(core.Event{Repo: "R"})

	if wtA.emitCount != 2 || wtB.emitCount != 1 {
		t.Errorf("wtA, wtB emitCount = %d, %d, want 2, 1", wtA.emitCount, wtB.emitCount)
	}
	if wtC.emitCount != 0 {
		t.Errorf("wtC emitCount = %d, want 0", wtC.emitCount)
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

	r.Dispatch(core.Event{Repo: "R1"})

	if opener.windows[0].emitCount != 1 {
		t.Errorf("window selecting R1's worktree emitCount = %d, want 1", opener.windows[0].emitCount)
	}
	if opener.windows[1].emitCount != 0 {
		t.Errorf("window selecting R2's worktree emitCount = %d, want 0", opener.windows[1].emitCount)
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

func TestOpenWorktreePathsListsOpenWindows(t *testing.T) {
	r, _ := newTestRegistry()

	r.OpenOrFocus("a", "/a", "")
	r.OpenOrFocus("b", "/b", "")

	got := r.OpenWorktreePaths()
	sort.Strings(got)
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

func TestOpenWorktreePathsDropsAClosedWindow(t *testing.T) {
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

	options(fstest.MapFS{}, r, func(string) {}).OnShutdown()
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

// fakeDistroCore is a distroCorer that always returns client, reporting
// respawned only on its first Ensure call, and calling onConnect on that
// same first call, the way the real DistroCore.Ensure fires it.
type fakeDistroCore struct {
	client               *remote.Client
	calls                int
	installLauncherCalls []string // guiExePath values passed to InstallLauncher
	onConnect            func(context.Context, *remote.Client)
}

func (f *fakeDistroCore) Ensure(ctx context.Context) (*remote.Client, bool, error) {
	f.calls++
	if f.calls == 1 && f.onConnect != nil {
		f.onConnect(ctx, f.client)
	}
	return f.client, f.calls == 1, nil
}

func (f *fakeDistroCore) InstallLauncher(_ context.Context, guiExePath string) error {
	f.installLauncherCalls = append(f.installLauncherCalls, guiExePath)
	return nil
}

func (f *fakeDistroCore) OnConnect(fn func(context.Context, *remote.Client)) { f.onConnect = fn }

// LiveClient reports a live connection once Ensure has run at least once
// (calls > 0), whether for real or seeded by a test as "already connected".
func (f *fakeDistroCore) LiveClient() (*remote.Client, bool) {
	return f.client, f.calls > 0
}

func TestADR_3_CoreForRoutesAWSLPathToItsDistro(t *testing.T) {
	r, _ := newTestRegistry()
	distroClient := pipeAPIClient(t, fakeAPI{})
	r.newDistroCore = func(string) distroCorer { return &fakeDistroCore{client: distroClient} }

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

func TestADR_3_CoreOfRoutesThroughTheWindowsDistro(t *testing.T) {
	r, _ := newTestRegistry()
	distroClient := pipeAPIClient(t, fakeAPI{})
	r.newDistroCore = func(string) distroCorer { return &fakeDistroCore{client: distroClient} }

	r.OpenOrFocus("wt1", "/home/user/project", "Ubuntu-22.04")

	got, err := r.CoreOf("wt1")
	if err != nil {
		t.Fatal(err)
	}
	if got != core.API(distroClient) {
		t.Error("CoreOf did not resolve through the window's distro core")
	}
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
	r.newDistroCore = func(string) distroCorer { return fdc }

	r.OpenOrFocus("wtA", "/a", "Ubuntu-22.04")
	r.OpenOrFocus("wtB", "/b", "Ubuntu-22.04")
	r.OpenOrFocus("wtC", "/c", "Debian") // different distro; must stay untouched

	if _, err := r.CoreOf("wtA"); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	got := append([]string(nil), opened...)
	mu.Unlock()
	sort.Strings(got)
	if want := []string{"/a", "/b"}; !slices.Equal(got, want) {
		t.Errorf("reopened paths = %v, want %v", got, want)
	}

	wtA, wtB, wtC := opener.windows[0], opener.windows[1], opener.windows[2]
	if wtA.emitCount != 1 || wtB.emitCount != 1 {
		t.Errorf("emitCount wtA=%d wtB=%d, want 1, 1", wtA.emitCount, wtB.emitCount)
	}
	if wtC.emitCount != 0 {
		t.Errorf("wtC (different distro) emitCount = %d, want 0", wtC.emitCount)
	}
}

// SelectInWindow moves a window's selection and reports the one it
// replaced; a subsequent CoreOf resolves through the window's new
// selection, not its old one.
func TestADR_9_SelectInWindowMovesTheWindowsSelection(t *testing.T) {
	r, opener := newTestRegistry()
	distroClient := pipeAPIClient(t, fakeAPI{})
	r.newDistroCore = func(string) distroCorer { return &fakeDistroCore{client: distroClient} }

	r.OpenOrFocus("A", "/a", "")
	windowID := opener.windows[0].ID()

	previous, _, ok := r.SelectInWindow(windowID, "B", "/b", "Ubuntu-22.04")
	if previous != "A" || !ok {
		t.Fatalf("SelectInWindow(...) = (%q, %v), want (\"A\", true)", previous, ok)
	}

	got, err := r.CoreOf("B")
	if err != nil {
		t.Fatal(err)
	}
	if got != core.API(distroClient) {
		t.Error("CoreOf(B) did not resolve through the window's new selection")
	}
}

// Releasing the selection a switch leaves behind routes through its own
// real core (a distro's), not the local one: CoreOf can no longer find it
// once SelectInWindow has moved the only window that selected it.
func TestADR_9_SwitchingReleasesThePreviousSelectionThroughItsRealCore(t *testing.T) {
	r, opener := newTestRegistry()
	var closed []core.WorktreeID
	distroClient := pipeAPIClient(t, fakeAPI{closed: &closed})
	r.newDistroCore = func(string) distroCorer { return &fakeDistroCore{client: distroClient} }

	r.OpenOrFocus("A", "/a", "Ubuntu-22.04")
	windowID := opener.windows[0].ID()

	previous, previousDistro, ok := r.SelectInWindow(windowID, "B", "/b", "")
	if !ok {
		t.Fatal("SelectInWindow ok = false")
	}
	if r.StillSelected(previous) {
		t.Fatal("StillSelected(A) = true, want false")
	}

	prevCore, err := r.coreForDistro(previousDistro)
	if err != nil {
		t.Fatal(err)
	}
	_ = prevCore.CloseWorktree(context.Background(), previous)

	if len(closed) != 1 || closed[0] != "A" {
		t.Errorf("CloseWorktree calls = %v, want [\"A\"], routed through the distro core, not r.local", closed)
	}
}

// Two windows sharing a selection both receive a worktree-scoped event.
func TestADR_9_DispatchReachesEveryWindowSharingASelection(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("A", "/a", "")
	r.OpenOrFocus("other", "/other", "")
	if _, _, ok := r.SelectInWindow(opener.windows[1].ID(), "A", "/a", ""); !ok {
		t.Fatal("SelectInWindow ok = false")
	}

	r.Dispatch(core.Event{Worktree: "A"})

	if opener.windows[0].emitCount != 1 || opener.windows[1].emitCount != 1 {
		t.Errorf("emitCount = %d, %d, want 1, 1", opener.windows[0].emitCount, opener.windows[1].emitCount)
	}
}

// StillSelected is the safety check a switch relies on before
// releasing a worktree — true here since a second window still selects it.
func TestADR_9_StillSelectedIsTrueWhenAnotherWindowSharesTheSelection(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("A", "/a", "")
	r.OpenOrFocus("other", "/other", "")
	r.SelectInWindow(opener.windows[1].ID(), "A", "/a", "")

	previous, _, ok := r.SelectInWindow(opener.windows[0].ID(), "B", "/b", "")
	if !ok || previous != "A" {
		t.Fatalf("SelectInWindow(...) = (%q, %v), want (\"A\", true)", previous, ok)
	}

	if !r.StillSelected(previous) {
		t.Error("StillSelected(A) = false, want true — another window still selects it")
	}
}

// Closing a window's own OS window (not a switch) releases its selection
// through the real core too, exercised through a real WindowClosing.
func TestADR_9_ClosingAWindowClosesItsSelectionThroughTheRealCore(t *testing.T) {
	r, opener := newTestRegistry()
	var closed []core.WorktreeID
	distroClient := pipeAPIClient(t, fakeAPI{closed: &closed})
	r.newDistroCore = func(string) distroCorer { return &fakeDistroCore{client: distroClient} }

	r.OpenOrFocus("A", "/a", "Ubuntu-22.04")
	opener.windows[0].close()

	if len(closed) != 1 || closed[0] != "A" {
		t.Errorf("CloseWorktree calls = %v, want [\"A\"], routed through the distro core, not r.local", closed)
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
	r.Dispatch(core.Event{Worktree: "A"})
	if opener.windows[1].emitCount != 1 {
		t.Errorf("window 1 (still selecting A) emitCount = %d, want 1 — its own entry must be untouched", opener.windows[1].emitCount)
	}
}

func TestFR_CMP_02_OpenOrFocusCompareDiffReusesWindowByComparisonKey(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocusCompareDiff("wt", "a", "b", "f1", "")
	r.OpenOrFocusCompareDiff("wt", "a", "c", "f1", "")
	r.OpenOrFocusCompareDiff("wt", "a", "b", "f2", "")

	if len(opener.windows) != 2 {
		t.Fatalf("windows created = %d, want 2", len(opener.windows))
	}
}

func TestFR_CMP_02_OpenOrFocusCompareDiffRetargetsTheReusedWindow(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocusCompareDiff("wt", "a", "b", "f1", "")
	r.OpenOrFocusCompareDiff("wt", "a", "c", "f1", "")
	r.OpenOrFocusCompareDiff("wt", "a", "b", "f2", "")

	reused := opener.windows[0]
	if reused.focusCount != 1 {
		t.Errorf("reused window focusCount = %d, want 1", reused.focusCount)
	}
	if len(reused.emitted) != 1 {
		t.Fatalf("reused window emitted %d events, want 1", len(reused.emitted))
	}
	got := reused.emitted[0]
	if got.name != "compare:show-file" {
		t.Errorf("emitted event name = %q, want %q", got.name, "compare:show-file")
	}
	if got.payload != (compareShowFilePayload{Path: "f2", OldPath: ""}) {
		t.Errorf("emitted event payload = %+v, want {Path: f2, OldPath: \"\"}", got.payload)
	}
}

func TestFR_CMP_02_OpenOrFocusCompareDiffDoesNotFocusOrEmitOnAFreshWindow(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocusCompareDiff("wt", "a", "b", "f1", "")

	if len(opener.windows) != 1 {
		t.Fatalf("windows created = %d, want 1", len(opener.windows))
	}
	if opener.windows[0].focusCount != 0 {
		t.Errorf("fresh window focusCount = %d, want 0", opener.windows[0].focusCount)
	}
	if len(opener.windows[0].emitted) != 0 {
		t.Errorf("fresh window emitted %d events, want 0", len(opener.windows[0].emitted))
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

	r.PushEditorProfiles(context.Background())

	if len(pushed) != 1 || pushed[0].Name != "another editor" {
		t.Errorf("local core received %+v, want exactly one push of another editor", pushed)
	}
}

func TestFR_ED_11_CoreForPushesTheDistrosOwnProfileOnFirstConnect(t *testing.T) {
	r, _ := newTestRegistry()
	var pushed []core.EditorProfile
	distroClient := pipeAPIClient(t, fakeAPI{editorProfiles: &pushed})
	r.newDistroCore = func(string) distroCorer { return &fakeDistroCore{client: distroClient} }
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

	r.PushEditorProfiles(context.Background())

	if len(pushedConnected) != 1 || pushedConnected[0].Name != "another editor" {
		t.Errorf("connected distro received %+v, want exactly one push of another editor", pushedConnected)
	}
	if notConnected.calls != 0 {
		t.Errorf("NotConnected.calls = %d, want 0 (PushEditorProfiles must not trigger Ensure/deploy)", notConnected.calls)
	}
}

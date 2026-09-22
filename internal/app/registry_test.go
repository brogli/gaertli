package app

import (
	"context"
	"slices"
	"sort"
	"sync"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/remote/remotetest"
	"github.com/brogli/nebegit/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// emittedEvent is one EmitEvent call recorded by fakeWindow: name and
// payload, not just a count (spec 23, so a test can assert what
// "compare:show-file" carried).
type emittedEvent struct {
	name    string
	payload any
}

// fakeWindow is a shellWindow that records what was called on it, and lets
// a test simulate the OS closing it.
type fakeWindow struct {
	focusCount int
	emitCount  int
	emitted    []emittedEvent
	closing    func(*application.WindowEvent)
}

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

func (o *fakeOpener) NewWithOptions(application.WebviewWindowOptions) shellWindow {
	w := &fakeWindow{}
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

func TestOpenHomeOpensAtMostOneWindow(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenHome()
	r.OpenHome()

	if len(opener.windows) != 1 {
		t.Fatalf("windows created = %d, want 1", len(opener.windows))
	}
	if opener.windows[0].focusCount != 1 {
		t.Errorf("focus count = %d, want 1", opener.windows[0].focusCount)
	}
}

func TestOpenHomeReopensAfterItIsClosed(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenHome()
	opener.windows[0].close()
	r.OpenHome()

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

// pipeAPIClient serves api over an in-memory pipe: these WSL routing
// tests exercise the real generated client/dispatcher without a distro.
func pipeAPIClient(t *testing.T, api core.API) *remote.Client {
	t.Helper()
	return remotetest.NewPipeClient(t, func(core.EventSink) core.API { return api }, nil)
}

// fakeDistroCore is a distroCorer that always returns client, reporting
// respawned only on its first Ensure call, and calling onConnect on that
// same first call (real DistroCore.Ensure's own "on connect" firing,
// spec 31 case 18's seam).
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
// (calls > 0), whether that happened for real or was set directly by a test
// seeding an "already connected" fakeDistroCore (case 19).
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

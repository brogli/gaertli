package app

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// fakeAPI is a core.API whose OpenWorktree behaviour a test picks; the
// other methods this package reaches record their calls. The embedded
// interface is nil, so any method a test does not expect to be called
// panics instead of quietly answering.
type fakeAPI struct {
	core.API
	openWorktree   func(context.Context, core.RepoPath) (core.WorktreeInfo, error)
	closed         *[]core.WorktreeID
	editorProfiles *[]core.EditorProfile // records every SetEditorProfile call, if set
}

func (f fakeAPI) OpenWorktree(ctx context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
	return f.openWorktree(ctx, path)
}

func (f fakeAPI) CloseWorktree(_ context.Context, id core.WorktreeID) error {
	if f.closed != nil {
		*f.closed = append(*f.closed, id)
	}
	return nil
}

func (f fakeAPI) SetEditorProfile(_ context.Context, profile core.EditorProfile) error {
	if f.editorProfiles != nil {
		*f.editorProfiles = append(*f.editorProfiles, profile)
	}
	return nil
}

// SetFocused is reached from a window focus callback's own goroutine, where
// a panic would take the test binary down with it.
func (f fakeAPI) SetFocused(context.Context, core.WorktreeID, bool) error { return nil }

func (f fakeAPI) Jobs(context.Context, core.WorktreeID) ([]core.JobInfo, error) { return nil, nil }

func (f fakeAPI) CancelJob(context.Context, core.WorktreeID, string) error { return nil }

// fakeNotifier is an errorNotifier that counts its calls.
type fakeNotifier struct{ calls int }

func (n *fakeNotifier) Show(string, string) { n.calls++ }

func newTestStore(t *testing.T) *settings.Store {
	t.Helper()
	st, err := settings.Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestFR_REPO_06_PathArg(t *testing.T) {
	cases := []struct {
		args     []string
		wantPath string
		wantOK   bool
	}{
		{[]string{"nebegit"}, "", true},
		{[]string{"nebegit", "/a"}, "/a", true},
		{[]string{"nebegit", "/a", "/b"}, "", false},
	}
	for _, c := range cases {
		path, ok := pathArg(c.args)
		if path != c.wantPath || ok != c.wantOK {
			t.Errorf("pathArg(%v) = (%q, %v), want (%q, %v)", c.args, path, ok, c.wantPath, c.wantOK)
		}
	}
}

func TestFR_REPO_06_PathArgThenOpenOrFocusOpensAWindow(t *testing.T) {
	r, opener := newTestRegistry()

	path, ok := pathArg([]string{"nebegit", "/a"})
	if !ok {
		t.Fatal("pathArg ok = false")
	}
	r.OpenOrFocus(core.WorktreeID(path), path, "")

	if len(opener.windows) != 1 {
		t.Fatalf("windows created = %d, want 1", len(opener.windows))
	}
}

func TestADR_9_OptionsRegistersTheFacadeAndSingleInstance(t *testing.T) {
	reg := NewRegistry()
	opts := options(fstest.MapFS{}, reg, func(string) {})

	if opts.Name != "nebegit" {
		t.Errorf("Name = %q, want %q", opts.Name, "nebegit")
	}
	if len(opts.Services) != 1 {
		t.Fatalf("Services = %d entries, want 1", len(opts.Services))
	}
	if _, ok := opts.Services[0].Instance().(*Facade); !ok {
		t.Errorf("Services[0].Instance() = %T, want *Facade", opts.Services[0].Instance())
	}
	if opts.SingleInstance == nil || opts.SingleInstance.UniqueID == "" {
		t.Error("SingleInstance.UniqueID not set")
	}
}

func TestFR_REPO_06_ASecondInstancesPathResolvesAgainstItsOwnWorkingDir(t *testing.T) {
	launchDir, elsewhere := t.TempDir(), t.TempDir()
	cases := []struct{ arg, want string }{
		{".", launchDir},
		{"sub/repo", filepath.Join(launchDir, "sub", "repo")},
		{elsewhere, elsewhere},
	}
	for _, c := range cases {
		var got string
		opts := options(fstest.MapFS{}, NewRegistry(), func(path string) { got = path })

		opts.SingleInstance.OnSecondInstanceLaunch(application.SecondInstanceData{Args: []string{"nebegit", c.arg}, WorkingDir: launchDir})

		if got != c.want {
			t.Errorf("relayed path for %q = %q, want %q", c.arg, got, c.want)
		}
	}
}

func TestFR_REPO_02_ASecondInstanceWithoutAPathRelaysNone(t *testing.T) {
	got := "unset"
	opts := options(fstest.MapFS{}, NewRegistry(), func(path string) { got = path })

	opts.SingleInstance.OnSecondInstanceLaunch(application.SecondInstanceData{Args: []string{"nebegit"}, WorkingDir: t.TempDir()})

	if got != "" {
		t.Errorf("relayed path = %q, want \"\" (restore)", got)
	}
}

// A window per remembered selection, each with a real, non-empty id —
// never the empty-selection placeholder.
func TestFR_REPO_02_RestoreOpensAWindowForEachRememberedSelection(t *testing.T) {
	reg, opener := newTestRegistry()
	store := newTestStore(t)
	if err := store.Update(func(s *settings.Settings) { s.OpenWorktrees = []string{"/p1", "/p2"} }); err != nil {
		t.Fatal(err)
	}
	opened := map[core.RepoPath]bool{}
	api := fakeAPI{openWorktree: func(_ context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
		opened[path] = true
		return core.WorktreeInfo{ID: core.WorktreeID(path), Path: string(path)}, nil
	}}

	shell{api: api, reg: reg, store: store, notify: &fakeNotifier{}}.restore(context.Background())

	if len(opener.windows) != 2 {
		t.Fatalf("windows opened = %d, want 2", len(opener.windows))
	}
	if !opened["/p1"] || !opened["/p2"] {
		t.Errorf("opened = %v, want both /p1 and /p2", opened)
	}
	for _, w := range opener.windows {
		if w.title == "nebegit" {
			t.Errorf("a window opened with the empty-selection placeholder title, want none (OpenOrFocus never called with \"\")")
		}
	}
}

func TestFR_REPO_02_RestoreOpensOneEmptySelectionWindowWhenNothingWasOpen(t *testing.T) {
	reg, opener := newTestRegistry()
	store := newTestStore(t)
	api := fakeAPI{openWorktree: func(context.Context, core.RepoPath) (core.WorktreeInfo, error) {
		t.Fatal("OpenWorktree should not be called")
		return core.WorktreeInfo{}, nil
	}}

	shell{api: api, reg: reg, store: store, notify: &fakeNotifier{}}.restore(context.Background())

	if len(opener.windows) != 1 {
		t.Fatalf("windows opened = %d, want 1", len(opener.windows))
	}
	if got := opener.windows[0].title; got != "nebegit" {
		t.Errorf("window title = %q, want %q (OpenOrFocus(\"\", \"\", \"\") called exactly once)", got, "nebegit")
	}
}

func TestFR_REPO_02_RestoreFallsBackToTheEmptySelectionWindowWhenNoRememberedWorktreeOpens(t *testing.T) {
	reg, opener := newTestRegistry()
	store := newTestStore(t)
	if err := store.Update(func(s *settings.Settings) { s.OpenWorktrees = []string{"/gone1", "/gone2"} }); err != nil {
		t.Fatal(err)
	}
	api := fakeAPI{openWorktree: func(context.Context, core.RepoPath) (core.WorktreeInfo, error) {
		return core.WorktreeInfo{}, &core.Error{Code: core.NotARepo, Message: "not a git repository"}
	}}
	notify := &fakeNotifier{}

	shell{api: api, reg: reg, store: store, notify: notify}.restore(context.Background())

	if notify.calls != 2 {
		t.Errorf("notify.Show calls = %d, want 2", notify.calls)
	}
	if len(opener.windows) != 1 || opener.windows[0].title != "nebegit" {
		t.Fatalf("windows = %d, want the one empty-selection window", len(opener.windows))
	}
}

func TestFR_REPO_05_OpenReportsWhetherTheWindowOpened(t *testing.T) {
	reg, _ := newTestRegistry()
	store := newTestStore(t)
	api := fakeAPI{openWorktree: func(_ context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
		if path == "/bad" {
			return core.WorktreeInfo{}, &core.Error{Code: core.NotARepo, Message: "not a git repository"}
		}
		return core.WorktreeInfo{ID: core.WorktreeID(path), Path: string(path)}, nil
	}}
	sh := shell{api: api, reg: reg, store: store, notify: &fakeNotifier{}}

	if sh.open(context.Background(), "/bad") {
		t.Error("open(/bad) = true, want false")
	}
	if !sh.open(context.Background(), "/good") {
		t.Error("open(/good) = false, want true")
	}
}

// fakeCallingWindow is a callingWindow that records what was called on it.
type fakeCallingWindow struct {
	id        uint
	setURLs   []string
	setTitles []string
}

func (w *fakeCallingWindow) ID() uint { return w.id }

func (w *fakeCallingWindow) SetURL(s string) application.Window {
	w.setURLs = append(w.setURLs, s)
	return nil
}

func (w *fakeCallingWindow) SetTitle(title string) application.Window {
	w.setTitles = append(w.setTitles, title)
	return nil
}

// switchTo validates path the same way open() does, moves the
// calling window's own registry selection, and navigates it in place.
func TestFR_WT_05_FR_WT_08_SwitchToMovesTheWindowsSelection(t *testing.T) {
	reg, opener := newTestRegistry()
	reg.OpenOrFocus("A", "/a", "")
	windowID := opener.windows[0].ID()

	var opened []core.RepoPath
	api := fakeAPI{openWorktree: func(_ context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
		opened = append(opened, path)
		return core.WorktreeInfo{ID: core.WorktreeID(path), Path: string(path), RepoKey: "repo-b"}, nil
	}}
	sh := shell{api: api, reg: reg, store: newTestStore(t), notify: &fakeNotifier{}}
	fw := &fakeCallingWindow{id: windowID}

	if err := sh.switchTo(context.Background(), fw, "/b"); err != nil {
		t.Fatal(err)
	}

	if want := []core.RepoPath{"/b"}; !slices.Equal(opened, want) {
		t.Errorf("OpenWorktree calls = %v, want %v", opened, want)
	}
	if reg.StillSelected("A") {
		t.Error("the window's own selection still names A, want it moved to the new id")
	}
	if _, err := reg.CoreOf("/b"); err != nil {
		t.Errorf("CoreOf(/b) = %v, want the window's entry to now select it", err)
	}
	if want := []string{"/?path=%2Fb"}; !slices.Equal(fw.setURLs, want) {
		t.Errorf("SetURL calls = %v, want %v", fw.setURLs, want)
	}
}

// A WSL window's URL carries the distro the path arrived from, so a later
// MainView/WorktreeContent load knows which core to route through.
func TestFR_REPO_02_SwitchToCarriesTheDistroInTheURL(t *testing.T) {
	reg, opener := newTestRegistry()
	reg.OpenOrFocus("A", "/a", "")
	windowID := opener.windows[0].ID()

	api := fakeAPI{openWorktree: func(context.Context, core.RepoPath) (core.WorktreeInfo, error) {
		return core.WorktreeInfo{ID: "B", Path: "/home/u/r", RepoKey: "repo-b"}, nil
	}}
	sh := shell{api: api, reg: reg, store: newTestStore(t), notify: &fakeNotifier{}}
	fw := &fakeCallingWindow{id: windowID}

	if err := sh.switchTo(context.Background(), fw, `\\wsl.localhost\Ubuntu\home\u\r`); err != nil {
		t.Fatal(err)
	}

	if want := []string{"/?path=%2Fhome%2Fu%2Fr&distro=Ubuntu"}; !slices.Equal(fw.setURLs, want) {
		t.Errorf("SetURL calls = %v, want %v", fw.setURLs, want)
	}
}

func TestFR_REPO_02_SelectionURLCarriesTheDistro(t *testing.T) {
	cases := []struct{ path, distro, want string }{
		{"", "", "/"},
		{"/home/u/r", "", "/?path=%2Fhome%2Fu%2Fr"},
		{"/home/u/r", "Ubuntu", "/?path=%2Fhome%2Fu%2Fr&distro=Ubuntu"},
	}
	for _, c := range cases {
		if got := selectionURL(c.path, c.distro); got != c.want {
			t.Errorf("selectionURL(%q, %q) = %q, want %q", c.path, c.distro, got, c.want)
		}
	}
}

func TestFR_REPO_05_OpenWorktreePathNotifiesAndOpensNoWindowOnFailure(t *testing.T) {
	reg, opener := newTestRegistry()
	store := newTestStore(t)
	api := fakeAPI{openWorktree: func(context.Context, core.RepoPath) (core.WorktreeInfo, error) {
		return core.WorktreeInfo{}, &core.Error{Code: core.NotARepo, Message: "not a git repository"}
	}}
	notify := &fakeNotifier{}

	shell{api: api, reg: reg, store: store, notify: notify}.open(context.Background(), "/bad")

	if notify.calls != 1 {
		t.Errorf("notify.Show calls = %d, want 1", notify.calls)
	}
	if len(opener.windows) != 0 {
		t.Errorf("windows opened = %d, want 0", len(opener.windows))
	}
}

// The WSL failures need no bespoke handling: openFailedMessage's generic
// *core.Error path already names each situation.
func TestOpenFailedMessageNamesWSLFailures(t *testing.T) {
	cases := []struct {
		name string
		err  *core.Error
		want string
	}{
		{"wsl not found", core.Errorf(core.WslNotFound, "WSL is not installed (wsl.exe not found on PATH)"), "WSL is not installed"},
		{"distro not found", core.Errorf(core.DistroNotFound, "distro %q not found", "bogus"), `distro "bogus" not found`},
		{"deploy refused", core.Errorf(core.DeployRefused, "copying nebegit's helper into %s was declined", "Ubuntu-22.04"), "declined"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := openFailedMessage(tt.err)
			if !strings.Contains(got, tt.want) {
				t.Errorf("openFailedMessage(%v) = %q, want to contain %q", tt.err, got, tt.want)
			}
		})
	}
}

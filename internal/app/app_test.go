package app

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/settings"
	"github.com/brogli/nebegit/internal/userpath"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// fakeAPI is a core.API whose OpenWorktree behaviour a test picks; the
// other methods this package reaches record their calls. The embedded
// interface is nil, so any method a test does not expect to be called
// panics instead of quietly answering.
type fakeAPI struct {
	core.API
	openWorktree   func(context.Context, core.RepoPath) (core.WorktreeInfo, error)
	mainWorktree   func(context.Context, core.RepoPath) (core.Worktree, error)
	closed         *[]core.WorktreeID
	editorProfiles *[]core.EditorProfile // records every SetEditorProfile call, if set
	calls          chan<- routedCall     // receives every Summary, CompareCommits and SetFocused call, if set
}

// routedCall is one worktree-routed call a fake core received.
type routedCall struct {
	method string
	wt     core.WorktreeID
}

func (f fakeAPI) record(method string, wt core.WorktreeID) {
	if f.calls != nil {
		f.calls <- routedCall{method, wt}
	}
}

// nextCall waits for the next call a fake core records on calls.
func nextCall(t *testing.T, calls <-chan routedCall) routedCall {
	t.Helper()
	select {
	case c := <-calls:
		return c
	case <-time.After(5 * time.Second):
		t.Fatal("no core call arrived")
		return routedCall{}
	}
}

// openAsIs opens path as a worktree whose id and path are path itself.
func openAsIs(_ context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
	return core.WorktreeInfo{ID: core.WorktreeID(path), Path: string(path)}, nil
}

// notARepo fails to open any path.
func notARepo(context.Context, core.RepoPath) (core.WorktreeInfo, error) {
	return core.WorktreeInfo{}, &core.Error{Code: core.NotARepo, Message: "not a git repository"}
}

func (f fakeAPI) OpenWorktree(ctx context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
	return f.openWorktree(ctx, path)
}

func (f fakeAPI) MainWorktree(ctx context.Context, gitDir core.RepoPath) (core.Worktree, error) {
	return f.mainWorktree(ctx, gitDir)
}

func (f fakeAPI) Summary(_ context.Context, wt core.WorktreeID) (core.Summary, error) {
	f.record("Summary", wt)
	return core.Summary{}, nil
}

func (f fakeAPI) CompareCommits(_ context.Context, wt core.WorktreeID, _, _ string) ([]core.ChangedFile, error) {
	f.record("CompareCommits", wt)
	return nil, nil
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
func (f fakeAPI) SetFocused(_ context.Context, wt core.WorktreeID, focused bool) error {
	f.record(fmt.Sprintf("SetFocused(%t)", focused), wt)
	return nil
}

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

func TestFR_REPO_06_LaunchPath(t *testing.T) {
	abs := t.TempDir()
	cases := []struct {
		args     []string
		wantPath string
		wantOK   bool
	}{
		{[]string{"nebegit"}, "", true},
		{[]string{"nebegit", abs}, abs, true},
		{[]string{"nebegit", abs, abs}, "", false},
	}
	for _, c := range cases {
		path, ok := launchPath(c.args, t.TempDir())
		if path != c.wantPath || ok != c.wantOK {
			t.Errorf("launchPath(%v, ...) = (%q, %v), want (%q, %v)", c.args, path, ok, c.wantPath, c.wantOK)
		}
	}
}

func TestFR_REPO_06_LaunchPathThenOpenOrFocusOpensAWindow(t *testing.T) {
	r, opener := newTestRegistry()

	path, ok := launchPath([]string{"nebegit", "/a"}, "/wd")
	if !ok {
		t.Fatal("launchPath ok = false")
	}
	r.OpenOrFocus(core.WorktreeID(path), path, "")

	if len(opener.windows) != 1 {
		t.Fatalf("windows created = %d, want 1", len(opener.windows))
	}
}

// The first launch resolves its path argument the same way the
// second-instance relay resolves a relayed one: against the launching
// working directory.
func TestFR_REPO_06_LaunchPathResolvesAgainstTheWorkingDir(t *testing.T) {
	wslDir := `\\wsl.localhost\Ubuntu\home\u\repo`
	abs := t.TempDir()
	cases := []struct {
		args     []string
		wantPath string
	}{
		{[]string{"nebegit", "."}, wslDir},
		{[]string{"nebegit", abs}, abs},
		{[]string{"nebegit"}, ""},
	}
	for _, c := range cases {
		path, ok := launchPath(c.args, wslDir)
		if !ok || path != c.wantPath {
			t.Errorf("launchPath(%v, %q) = (%q, %v), want (%q, true)", c.args, wslDir, path, ok, c.wantPath)
		}
	}

	resolved, _ := launchPath([]string{"nebegit", "."}, wslDir)
	distro, linuxPath, ok := remote.ClassifyWSLPath(resolved)
	if !ok || distro != "Ubuntu" || linuxPath != "/home/u/repo" {
		t.Errorf("ClassifyWSLPath(%q) = (%q, %q, %v), want (Ubuntu, /home/u/repo, true)", resolved, distro, linuxPath, ok)
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
		opts := options(fstest.MapFS{}, NewRegistry(), func(path string) { got = path }, nil)

		opts.SingleInstance.OnSecondInstanceLaunch(application.SecondInstanceData{Args: []string{"nebegit", c.arg}, WorkingDir: launchDir})

		if got != c.want {
			t.Errorf("relayed path for %q = %q, want %q", c.arg, got, c.want)
		}
	}
}

func TestFR_REPO_02_ASecondInstanceWithoutAPathRelaysNone(t *testing.T) {
	got := "unset"
	opts := options(fstest.MapFS{}, NewRegistry(), func(path string) { got = path }, nil)

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
	api := fakeAPI{openWorktree: func(ctx context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
		opened[path] = true
		return openAsIs(ctx, path)
	}}

	shell{api: api, reg: reg, store: store, notify: &fakeNotifier{}}.restore(t.Context())

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

	shell{api: api, reg: reg, store: store, notify: &fakeNotifier{}}.restore(t.Context())

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
	api := fakeAPI{openWorktree: notARepo}
	notify := &fakeNotifier{}

	shell{api: api, reg: reg, store: store, notify: notify}.restore(t.Context())

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
	api := fakeAPI{openWorktree: func(ctx context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
		if path == "/bad" {
			return notARepo(ctx, path)
		}
		return openAsIs(ctx, path)
	}}
	sh := shell{api: api, reg: reg, store: store, notify: &fakeNotifier{}}

	if sh.open(t.Context(), "/bad") {
		t.Error("open(/bad) = true, want false")
	}
	if !sh.open(t.Context(), "/good") {
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

	if err := sh.switchTo(t.Context(), fw, "/b"); err != nil {
		t.Fatal(err)
	}

	if want := []core.RepoPath{"/b"}; !slices.Equal(opened, want) {
		t.Errorf("OpenWorktree calls = %v, want %v", opened, want)
	}
	if reg.inUse("A") {
		t.Error("the window's own selection still names A, want it moved to the new id")
	}
	if !reg.inUse("/b") {
		t.Error("the window's entry does not select /b")
	}
	if want := []string{"/?path=%2Fb"}; !slices.Equal(fw.setURLs, want) {
		t.Errorf("SetURL calls = %v, want %v", fw.setURLs, want)
	}
}

// switchTo records the new worktree as the repo's recent entry, replacing
// the one the repo picker's fallback would otherwise still see for an old,
// since-removed worktree.
func TestFR_WT_13_SwitchToUpdatesTheRepoRecentEntry(t *testing.T) {
	reg, opener := newTestRegistry()
	reg.OpenOrFocus("A", "/a", "")
	windowID := opener.windows[0].ID()

	api := fakeAPI{openWorktree: func(_ context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
		return core.WorktreeInfo{ID: core.WorktreeID(path), Path: string(path), RepoKey: "repo-b"}, nil
	}}
	store := newTestStore(t)
	if err := store.Update(func(s *settings.Settings) {
		s.RecentRepos = []settings.RecentRepo{{Path: "/b-old", RepoKey: "repo-b"}}
	}); err != nil {
		t.Fatal(err)
	}
	sh := shell{api: api, reg: reg, store: store, notify: &fakeNotifier{}}
	fw := &fakeCallingWindow{id: windowID}

	if err := sh.switchTo(t.Context(), fw, "/b"); err != nil {
		t.Fatal(err)
	}

	recent := store.Get().RecentRepos
	if want := []string{"/b"}; !slices.Equal(recentPaths(recent), want) {
		t.Errorf("RecentRepos paths = %v, want %v", recentPaths(recent), want)
	}
}

func recentPaths(recent []settings.RecentRepo) []string {
	paths := make([]string, len(recent))
	for i, r := range recent {
		paths[i] = r.Path
	}
	return paths
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

	if err := sh.switchTo(t.Context(), fw, `\\wsl.localhost\Ubuntu\home\u\r`); err != nil {
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

func TestDIST_04_PathActionForDispatchesTheInstallerFlagsOnly(t *testing.T) {
	cases := []struct {
		args []string
		want func(string) error
	}{
		{[]string{"nebegit", "--add-to-path"}, userpath.Add},
		{[]string{"nebegit", "--remove-from-path"}, userpath.Remove},
		{[]string{"nebegit"}, nil},
		{[]string{"nebegit", `C:\repo`}, nil},
		{[]string{"nebegit", "--add-to-path", "extra"}, nil},
	}
	for _, c := range cases {
		if got := pathActionFor(c.args); fmt.Sprintf("%p", got) != fmt.Sprintf("%p", c.want) {
			t.Errorf("pathActionFor(%v) = %p, want %p", c.args, got, c.want)
		}
	}
}

func TestFR_REPO_05_OpenWorktreePathNotifiesAndOpensNoWindowOnFailure(t *testing.T) {
	reg, opener := newTestRegistry()
	store := newTestStore(t)
	api := fakeAPI{openWorktree: notARepo}
	notify := &fakeNotifier{}

	shell{api: api, reg: reg, store: store, notify: notify}.open(t.Context(), "/bad")

	if notify.calls != 1 {
		t.Errorf("notify.Show calls = %d, want 1", notify.calls)
	}
	if len(opener.windows) != 0 {
		t.Errorf("windows opened = %d, want 0", len(opener.windows))
	}
}

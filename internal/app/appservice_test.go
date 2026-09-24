package app

import (
	"context"
	"os"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// fakePicker answers with path and ok, and records the dir it was seeded at.
type fakePicker struct {
	path string
	ok   bool
	dir  string
}

func (p *fakePicker) Pick(dir string) (string, bool) {
	p.dir = dir
	return p.path, p.ok
}

func TestAppService_PickAndOpenDoesNothingOnCancel(t *testing.T) {
	reg, opener := newTestRegistry()
	s := &AppService{shell: shell{reg: reg, store: newTestStore(t), notify: &fakeNotifier{}}, picker: &fakePicker{}}

	if err := s.PickAndOpen(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(opener.windows) != 0 {
		t.Errorf("windows opened = %d, want 0", len(opener.windows))
	}
}

func TestAppService_PickAndOpenOpensThePickedPath(t *testing.T) {
	reg, opener := newTestRegistry()
	api := fakeAPI{openWorktree: openAsIs}
	s := &AppService{shell: shell{api: api, reg: reg, store: newTestStore(t), notify: &fakeNotifier{}}, picker: &fakePicker{path: "/picked", ok: true}}

	if err := s.PickAndOpen(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(opener.windows) != 1 {
		t.Errorf("windows opened = %d, want 1", len(opener.windows))
	}
}

func TestFR_WT_08_OpenPathOpensALocalPath(t *testing.T) {
	reg, opener := newTestRegistry()
	api := fakeAPI{openWorktree: openAsIs}
	s := &AppService{shell: shell{api: api, reg: reg, store: newTestStore(t), notify: &fakeNotifier{}}}

	if err := s.OpenPath(t.Context(), "/repo-feature"); err != nil {
		t.Fatal(err)
	}
	if len(opener.windows) != 1 {
		t.Errorf("windows opened = %d, want 1", len(opener.windows))
	}
}

func TestAppService_RecentReposListsStoredEntries(t *testing.T) {
	store := newTestStore(t)
	if err := store.Update(func(s *settings.Settings) { s.AddRecent(settings.RecentRepo{Path: "/a"}, time.Now()) }); err != nil {
		t.Fatal(err)
	}
	s := &AppService{shell: shell{store: store}}

	got, err := s.RecentRepos(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "/a" {
		t.Errorf("RecentRepos() = %+v, want one entry for /a", got)
	}
}

func TestFR_REPO_07_PickAndOpenWSLSeedsThePickerAtTheDistroRoot(t *testing.T) {
	reg, _ := newTestRegistry()
	picker := &fakePicker{}
	s := &AppService{shell: shell{reg: reg, store: newTestStore(t), notify: &fakeNotifier{}}, picker: picker}

	if err := s.PickAndOpenWSL(t.Context(), "Ubuntu-22.04"); err != nil {
		t.Fatal(err)
	}

	if want := `\\wsl.localhost\Ubuntu-22.04`; picker.dir != want {
		t.Errorf("Pick dir = %q, want %q", picker.dir, want)
	}
}

func TestFR_REPO_07_PickAndOpenWSLOpensThePickedPath(t *testing.T) {
	reg, opener := newTestRegistry()
	api := fakeAPI{openWorktree: openAsIs}
	s := &AppService{
		shell:  shell{api: api, reg: reg, store: newTestStore(t), notify: &fakeNotifier{}},
		picker: &fakePicker{path: `\\wsl.localhost\Ubuntu-22.04\home\user\project`, ok: true},
	}

	if err := s.PickAndOpenWSL(t.Context(), "Ubuntu-22.04"); err != nil {
		t.Fatal(err)
	}
	if len(opener.windows) != 1 {
		t.Errorf("windows opened = %d, want 1", len(opener.windows))
	}
}

// An unknown distro needs no bespoke handling either: open()'s generic
// notify-and-do-nothing-else covers it.
func TestFR_REPO_07_PickAndOpenWSLNotifiesOnDistroNotFound(t *testing.T) {
	reg, opener := newTestRegistry()
	api := fakeAPI{openWorktree: func(context.Context, core.RepoPath) (core.WorktreeInfo, error) {
		return core.WorktreeInfo{}, core.Errorf(core.DistroNotFound, "distro %q not found", "bogus")
	}}
	notify := &fakeNotifier{}
	s := &AppService{
		shell:  shell{api: api, reg: reg, store: newTestStore(t), notify: notify},
		picker: &fakePicker{path: `\\wsl.localhost\bogus\home\x`, ok: true},
	}

	if err := s.PickAndOpenWSL(t.Context(), "bogus"); err != nil {
		t.Fatal(err)
	}
	if notify.calls != 1 {
		t.Errorf("notify.Show calls = %d, want 1", notify.calls)
	}
	if len(opener.windows) != 0 {
		t.Errorf("windows opened = %d, want 0", len(opener.windows))
	}
}

func TestFR_REPO_07_OpenPathOpensTheGivenPath(t *testing.T) {
	reg, opener := newTestRegistry()
	api := fakeAPI{openWorktree: openAsIs}
	s := &AppService{shell: shell{api: api, reg: reg, store: newTestStore(t), notify: &fakeNotifier{}}}

	if err := s.OpenPath(t.Context(), `\\wsl.localhost\Ubuntu-22.04\home\user\project`); err != nil {
		t.Fatal(err)
	}
	if len(opener.windows) != 1 {
		t.Errorf("windows opened = %d, want 1", len(opener.windows))
	}
}

// callFrom is the context of a call from the window with id.
func callFrom(id uint) context.Context {
	return context.WithValue(context.Background(), application.WindowKey, &fakeCallingWindow{id: id})
}

func TestFR_DIFF_07_OpenDiffOpensADiffWindowOnTheTarget(t *testing.T) {
	reg, opener := newTestRegistry()
	reg.OpenOrFocus("/x/project", "/x/project", "")
	s := &AppService{shell: shell{reg: reg}}
	target := DiffTarget{Kind: CommitDiff, Sha: "1111111111a", Path: "f.txt"}

	if err := s.OpenDiff(callFrom(opener.windows[0].ID()), "/x/project", target); err != nil {
		t.Fatal(err)
	}

	if len(opener.windows) != 2 {
		t.Fatalf("windows opened = %d, want the main window and one diff window", len(opener.windows))
	}
	diffWindow := opener.windows[1]
	if want := diffURL("/x/project", target); diffWindow.url != want {
		t.Errorf("url = %q, want %q", diffWindow.url, want)
	}
	if want := "project: 1111111"; diffWindow.title != want {
		t.Errorf("title = %q, want %q", diffWindow.title, want)
	}
}

func TestFR_DIFF_07_OpenDiffFromTheSameMainWindowRetargetsItsDiffWindow(t *testing.T) {
	reg, opener := newTestRegistry()
	reg.OpenOrFocus("/x/project", "/x/project", "")
	s := &AppService{shell: shell{reg: reg}}
	ctx := callFrom(opener.windows[0].ID())
	targets := []DiffTarget{
		{Kind: CommitDiff, Sha: "s", Path: "a"},
		{Kind: RangeDiff, From: "a", To: "b", Path: "b"},
		{Kind: WorkingDiff, Path: "c", Staged: true},
	}

	for _, target := range targets {
		if err := s.OpenDiff(ctx, "/x/project", target); err != nil {
			t.Fatal(err)
		}
	}

	if len(opener.windows) != 2 {
		t.Fatalf("windows opened = %d, want the main window and one diff window", len(opener.windows))
	}
	diffWindow := opener.windows[1]
	if diffWindow.focusCount != 2 {
		t.Errorf("diff window focused %d times, want 2", diffWindow.focusCount)
	}
	want := []emittedEvent{
		{"diff:show", DiffShow{Wt: "/x/project", Target: targets[1]}},
		{"diff:show", DiffShow{Wt: "/x/project", Target: targets[2]}},
	}
	if !slices.Equal(diffWindow.emitted, want) {
		t.Errorf("diff window received %+v, want %+v", diffWindow.emitted, want)
	}
	if want := "project: changes"; diffWindow.title != want {
		t.Errorf("title = %q, want %q", diffWindow.title, want)
	}
}

func TestFR_DIFF_07_EachMainWindowHasItsOwnDiffWindow(t *testing.T) {
	reg, opener := newTestRegistry()
	reg.OpenOrFocus("/a", "/a", "")
	reg.OpenOrFocus("/b", "/b", "")
	reg.SelectInWindow(opener.windows[1].ID(), "/a", "/a", "")
	s := &AppService{shell: shell{reg: reg}}
	target := DiffTarget{Kind: WorkingDiff, Path: "f"}

	for _, mainWindow := range opener.windows[:2] {
		if err := s.OpenDiff(callFrom(mainWindow.ID()), "/a", target); err != nil {
			t.Fatal(err)
		}
	}

	if len(opener.windows) != 4 {
		t.Fatalf("windows opened = %d, want two main windows and a diff window each", len(opener.windows))
	}
	for _, diffWindow := range opener.windows[2:] {
		if diffWindow.focusCount != 0 || len(diffWindow.emitted) != 0 {
			t.Errorf("fresh diff window focused %d times, received %v; want neither", diffWindow.focusCount, diffWindow.emitted)
		}
	}
}

// A call from outside any window, such as the second-instance hand-over, carries no window.
func TestFR_DIFF_07_WithNoCallingWindowTheDiffWindowIsKeyedByItsWorktree(t *testing.T) {
	reg, opener := newTestRegistry()
	s := &AppService{shell: shell{reg: reg}}
	target := DiffTarget{Kind: WorkingDiff, Path: "f"}

	for _, wt := range []core.WorktreeID{"/a", "/a", "/b"} {
		if err := s.OpenDiff(t.Context(), wt, target); err != nil {
			t.Fatal(err)
		}
	}

	if len(opener.windows) != 2 {
		t.Fatalf("windows opened = %d, want one diff window per worktree", len(opener.windows))
	}
	if got := len(opener.windows[0].emitted); got != 1 {
		t.Errorf("/a's diff window received %d events, want 1", got)
	}
}

func TestFR_REPO_06_InstallWSLLauncherInstallsTheRunningBinaryIntoTheDistro(t *testing.T) {
	reg, _ := newTestRegistry()
	fdc := &fakeDistroCore{}
	reg.newDistroCore = func(string, core.EventSink) distroCorer { return fdc }
	s := &AppService{shell: shell{reg: reg}}

	if err := s.InstallWSLLauncher(t.Context(), "Ubuntu-22.04"); err != nil {
		t.Fatal(err)
	}

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if len(fdc.installLauncherCalls) != 1 || fdc.installLauncherCalls[0] != self {
		t.Errorf("InstallLauncher calls = %v, want one call with %q", fdc.installLauncherCalls, self)
	}
}

// The window comes from ctx; a windowless/background call is a no-op, not
// an error.
func TestFR_WT_09_UpdateWindowTitleSetsTheCallingWindowsTitle(t *testing.T) {
	fw := &fakeCallingWindow{}
	ctx := context.WithValue(t.Context(), application.WindowKey, fw)
	s := &AppService{}

	if err := s.UpdateWindowTitle(ctx, "/x/project", "main"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"project [main]"}; !slices.Equal(fw.setTitles, want) {
		t.Errorf("SetTitle calls = %v, want %v", fw.setTitles, want)
	}

	if err := s.UpdateWindowTitle(t.Context(), "/x/project", "main"); err != nil {
		t.Errorf("UpdateWindowTitle with no window in ctx = %v, want nil", err)
	}
	if len(fw.setTitles) != 1 {
		t.Errorf("SetTitle calls = %d, want still 1 (no window in ctx)", len(fw.setTitles))
	}
}

func TestFR_UX_02_LogPathReportsTheAppLogsLocation(t *testing.T) {
	s := &AppService{shell: shell{logPath: "/x/nebegit.log"}}

	got, err := s.LogPath(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "/x/nebegit.log" || got.Error != "" {
		t.Errorf("LogPath() = %+v, want Path /x/nebegit.log, no Error", got)
	}
}

func TestFR_UX_02_LogPathReportsWhyTheLogCouldNotOpen(t *testing.T) {
	s := &AppService{shell: shell{logErr: "permission denied"}}

	got, err := s.LogPath(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "" || got.Error != "permission denied" {
		t.Errorf("LogPath() = %+v, want empty Path, Error %q", got, "permission denied")
	}
}

// A windowless/background call fails without ever touching OpenWorktree.
func TestFR_WT_05_SelectWorktreeFailsWithNoCallingWindow(t *testing.T) {
	called := false
	api := fakeAPI{openWorktree: func(context.Context, core.RepoPath) (core.WorktreeInfo, error) {
		called = true
		return core.WorktreeInfo{}, nil
	}}
	s := &AppService{shell: shell{api: api}}

	err := s.SelectWorktree(t.Context(), "/x")

	coretest.AssertCode(t, err, core.Internal)
	if called {
		t.Error("OpenWorktree should not be called")
	}
}

// SelectRecentRepo switches straight to the entry's own path when it still
// opens; the fallback is never consulted.
func TestFR_WT_13_SelectRecentRepoSwitchesToTheEntrysOwnPath(t *testing.T) {
	reg, opener := newTestRegistry()
	reg.OpenOrFocus("A", "/a", "")
	api := fakeAPI{
		openWorktree: openAsIs,
		mainWorktree: func(context.Context, core.RepoPath) (core.Worktree, error) {
			t.Fatal("MainWorktree should not be called")
			return core.Worktree{}, nil
		},
	}
	s := &AppService{shell: shell{api: api, reg: reg, store: newTestStore(t)}}

	err := s.SelectRecentRepo(callFrom(opener.windows[0].ID()), settings.RecentRepo{Path: "/b"})

	if err != nil {
		t.Fatal(err)
	}
}

// A gone entry (Code NotARepo) with a RepoKey retries at the repo's main
// worktree, found through MainWorktree; a WSL entry's RepoKey and the
// worktree MainWorktree returns both cross in UNC form.
func TestFR_WT_13_SelectRecentRepoFallsBackToTheMainWorktreeWhenTheEntryIsGone(t *testing.T) {
	reg, opener := newTestRegistry()
	reg.OpenOrFocus("A", "/a", "")
	var mainWorktreeArg core.RepoPath
	var openedPaths []core.RepoPath
	api := fakeAPI{
		openWorktree: func(_ context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
			openedPaths = append(openedPaths, path)
			if path == `\\wsl.localhost\Ubuntu\home\u\r-old` {
				return core.WorktreeInfo{}, core.Errorf(core.NotARepo, "gone")
			}
			return core.WorktreeInfo{ID: core.WorktreeID(path), Path: string(path)}, nil
		},
		mainWorktree: func(_ context.Context, gitDir core.RepoPath) (core.Worktree, error) {
			mainWorktreeArg = gitDir
			return core.Worktree{Path: "/home/u/r", Main: true}, nil
		},
	}
	s := &AppService{shell: shell{api: api, reg: reg, store: newTestStore(t)}}

	err := s.SelectRecentRepo(callFrom(opener.windows[0].ID()), settings.RecentRepo{
		Path: "/home/u/r-old", Distro: "Ubuntu", RepoKey: "/home/u/r/.git",
	})

	if err != nil {
		t.Fatal(err)
	}
	if want := core.RepoPath(`\\wsl.localhost\Ubuntu\home\u\r\.git`); mainWorktreeArg != want {
		t.Errorf("MainWorktree called with %q, want %q", mainWorktreeArg, want)
	}
	if want := []core.RepoPath{
		`\\wsl.localhost\Ubuntu\home\u\r-old`, `\\wsl.localhost\Ubuntu\home\u\r`,
	}; !slices.Equal(openedPaths, want) {
		t.Errorf("OpenWorktree calls = %v, want %v", openedPaths, want)
	}
}

// No RepoKey (an entry from before this fallback existed): the entry's own
// error is returned as-is, no fallback attempted.
func TestFR_WT_13_SelectRecentRepoReturnsTheErrorWithNoRepoKey(t *testing.T) {
	reg, opener := newTestRegistry()
	reg.OpenOrFocus("A", "/a", "")
	api := fakeAPI{
		openWorktree: func(context.Context, core.RepoPath) (core.WorktreeInfo, error) {
			return core.WorktreeInfo{}, core.Errorf(core.NotARepo, "gone")
		},
		mainWorktree: func(context.Context, core.RepoPath) (core.Worktree, error) {
			t.Fatal("MainWorktree should not be called")
			return core.Worktree{}, nil
		},
	}
	s := &AppService{shell: shell{api: api, reg: reg, store: newTestStore(t)}}

	err := s.SelectRecentRepo(callFrom(opener.windows[0].ID()), settings.RecentRepo{Path: "/b"})

	coretest.AssertCode(t, err, core.NotARepo)
}

// A bare repo whose only worktree is also gone: MainWorktree's own failure
// is returned, no second OpenWorktree attempted.
func TestFR_WT_13_SelectRecentRepoReturnsMainWorktreesErrorOnASecondFailure(t *testing.T) {
	reg, opener := newTestRegistry()
	reg.OpenOrFocus("A", "/a", "")
	openCount := 0
	api := fakeAPI{
		openWorktree: func(context.Context, core.RepoPath) (core.WorktreeInfo, error) {
			openCount++
			return core.WorktreeInfo{}, core.Errorf(core.NotARepo, "gone")
		},
		mainWorktree: func(context.Context, core.RepoPath) (core.Worktree, error) {
			return core.Worktree{}, core.Errorf(core.NotARepo, "no worktree left")
		},
	}
	s := &AppService{shell: shell{api: api, reg: reg, store: newTestStore(t)}}

	err := s.SelectRecentRepo(callFrom(opener.windows[0].ID()), settings.RecentRepo{
		Path: "/b", RepoKey: "/bare.git",
	})

	coretest.AssertCode(t, err, core.NotARepo)
	if openCount != 1 {
		t.Errorf("OpenWorktree called %d times, want 1 (no retry with a wrong folder)", openCount)
	}
}

func TestFR_REPO_07_ListWSLDistrosOnAHostWithoutWSLIsEmptyNotAnError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a Windows host may have WSL")
	}
	distros, err := (&AppService{}).ListWSLDistros(t.Context())
	if err != nil || len(distros) != 0 {
		t.Errorf("ListWSLDistros = %v, %v; want no distros and no error", distros, err)
	}
}

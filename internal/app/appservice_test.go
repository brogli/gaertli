package app

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/settings"
)

type fakePicker struct {
	path string
	ok   bool
}

func (p fakePicker) Pick(string) (string, bool) { return p.path, p.ok }

// recordingPicker records the dir it was asked to seed at, always cancels.
type recordingPicker struct{ dir string }

func (p *recordingPicker) Pick(dir string) (string, bool) {
	p.dir = dir
	return "", false
}

func TestAppService_PickAndOpenDoesNothingOnCancel(t *testing.T) {
	reg, opener := newTestRegistry()
	s := &AppService{shell: shell{reg: reg, store: newTestStore(t), notify: &fakeNotifier{}}, picker: fakePicker{ok: false}}

	if err := s.PickAndOpen(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(opener.windows) != 0 {
		t.Errorf("windows opened = %d, want 0", len(opener.windows))
	}
}

func TestAppService_PickAndOpenOpensThePickedPath(t *testing.T) {
	reg, opener := newTestRegistry()
	api := fakeAPI{openWorktree: func(_ context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
		return core.WorktreeInfo{ID: core.WorktreeID(path), Path: string(path)}, nil
	}}
	s := &AppService{shell: shell{api: api, reg: reg, store: newTestStore(t), notify: &fakeNotifier{}}, picker: fakePicker{path: "/picked", ok: true}}

	if err := s.PickAndOpen(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(opener.windows) != 1 {
		t.Errorf("windows opened = %d, want 1", len(opener.windows))
	}
}

func TestAppService_OpenRecentOpensThePath(t *testing.T) {
	reg, opener := newTestRegistry()
	api := fakeAPI{openWorktree: func(_ context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
		return core.WorktreeInfo{ID: core.WorktreeID(path), Path: string(path)}, nil
	}}
	s := &AppService{shell: shell{api: api, reg: reg, store: newTestStore(t), notify: &fakeNotifier{}}}

	if err := s.OpenRecent(context.Background(), "/recent"); err != nil {
		t.Fatal(err)
	}
	if len(opener.windows) != 1 {
		t.Errorf("windows opened = %d, want 1", len(opener.windows))
	}
}

func TestAppService_RecentReposListsStoredEntries(t *testing.T) {
	store := newTestStore(t)
	if err := store.Update(func(s *settings.Settings) { s.AddRecent("/a", time.Now(), "") }); err != nil {
		t.Fatal(err)
	}
	s := &AppService{shell: shell{store: store}}

	got, err := s.RecentRepos(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "/a" {
		t.Errorf("RecentRepos() = %+v, want one entry for /a", got)
	}
}

// ListWSLDistros forwards to remote.ListDistros unchanged; on this
// (non-Windows) platform that is WslNotFound (case 16's first fabricated
// failure needs no fabrication here — distros_other.go already returns it).
func TestFR_REPO_07_ListWSLDistrosReturnsWslNotFoundOnThisPlatform(t *testing.T) {
	s := &AppService{}

	_, err := s.ListWSLDistros(context.Background())

	var ce *core.Error
	if !errors.As(err, &ce) || ce.Code != core.WslNotFound {
		t.Fatalf("ListWSLDistros() err = %v, want a *core.Error{Code: WslNotFound}", err)
	}
}

func TestFR_REPO_07_PickAndOpenWSLSeedsThePickerAtTheDistroRoot(t *testing.T) {
	reg, _ := newTestRegistry()
	picker := &recordingPicker{}
	s := &AppService{shell: shell{reg: reg, store: newTestStore(t), notify: &fakeNotifier{}}, picker: picker}

	if err := s.PickAndOpenWSL(context.Background(), "Ubuntu-22.04"); err != nil {
		t.Fatal(err)
	}

	if want := `\\wsl.localhost\Ubuntu-22.04`; picker.dir != want {
		t.Errorf("Pick dir = %q, want %q", picker.dir, want)
	}
}

func TestFR_REPO_07_PickAndOpenWSLOpensThePickedPath(t *testing.T) {
	reg, opener := newTestRegistry()
	api := fakeAPI{openWorktree: func(_ context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
		return core.WorktreeInfo{ID: core.WorktreeID(path), Path: string(path)}, nil
	}}
	s := &AppService{
		shell:  shell{api: api, reg: reg, store: newTestStore(t), notify: &fakeNotifier{}},
		picker: fakePicker{path: `\\wsl.localhost\Ubuntu-22.04\home\user\project`, ok: true},
	}

	if err := s.PickAndOpenWSL(context.Background(), "Ubuntu-22.04"); err != nil {
		t.Fatal(err)
	}
	if len(opener.windows) != 1 {
		t.Errorf("windows opened = %d, want 1", len(opener.windows))
	}
}

// case 16's DistroNotFound path: routing an unknown distro's failure needs
// no bespoke handling either — open()'s existing generic notify-and-do-
// nothing-else covers it (FR-REPO-05).
func TestFR_REPO_07_PickAndOpenWSLNotifiesOnDistroNotFound(t *testing.T) {
	reg, opener := newTestRegistry()
	api := fakeAPI{openWorktree: func(context.Context, core.RepoPath) (core.WorktreeInfo, error) {
		return core.WorktreeInfo{}, core.Errorf(core.DistroNotFound, "distro %q not found", "bogus")
	}}
	notify := &fakeNotifier{}
	s := &AppService{
		shell:  shell{api: api, reg: reg, store: newTestStore(t), notify: notify},
		picker: fakePicker{path: `\\wsl.localhost\bogus\home\x`, ok: true},
	}

	if err := s.PickAndOpenWSL(context.Background(), "bogus"); err != nil {
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
	api := fakeAPI{openWorktree: func(_ context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
		return core.WorktreeInfo{ID: core.WorktreeID(path), Path: string(path)}, nil
	}}
	s := &AppService{shell: shell{api: api, reg: reg, store: newTestStore(t), notify: &fakeNotifier{}}}

	if err := s.OpenPath(context.Background(), `\\wsl.localhost\Ubuntu-22.04\home\user\project`); err != nil {
		t.Fatal(err)
	}
	if len(opener.windows) != 1 {
		t.Errorf("windows opened = %d, want 1", len(opener.windows))
	}
}

func TestFR_CMP_01_FR_CMP_02_OpenCompareDiffOpensAWindow(t *testing.T) {
	reg, opener := newTestRegistry()
	s := &AppService{shell: shell{reg: reg, store: newTestStore(t), notify: &fakeNotifier{}}}

	if err := s.OpenCompareDiff(context.Background(), "wt", "a", "b", "f1", ""); err != nil {
		t.Fatal(err)
	}
	if len(opener.windows) != 1 {
		t.Errorf("windows opened = %d, want 1", len(opener.windows))
	}
}

// TestFR_REPO_06_InstallWSLLauncherInstallsTheRunningBinaryIntoTheDistro is
// case 9's Go half: the frontend "Install nebegit command" click (task 8/9,
// component-tested there) is exercised as a direct unit call here in the
// meantime.
func TestFR_REPO_06_InstallWSLLauncherInstallsTheRunningBinaryIntoTheDistro(t *testing.T) {
	reg, _ := newTestRegistry()
	fdc := &fakeDistroCore{}
	reg.newDistroCore = func(string) distroCorer { return fdc }
	s := &AppService{shell: shell{reg: reg}}

	if err := s.InstallWSLLauncher(context.Background(), "Ubuntu-22.04"); err != nil {
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

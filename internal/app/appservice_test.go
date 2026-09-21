package app

import (
	"context"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/settings"
)

type fakePicker struct {
	path string
	ok   bool
}

func (p fakePicker) Pick() (string, bool) { return p.path, p.ok }

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
	if err := store.Update(func(s *settings.Settings) { s.AddRecent("/a", time.Now()) }); err != nil {
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

func TestSettingsService_SetThenGetRoundTrips(t *testing.T) {
	s := NewSettingsService(newTestStore(t))
	want := settings.Settings{Editor: "editor", DefaultPullMode: "rebase", DiffMode: "unified"}

	if err := s.Set(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Editor != want.Editor || got.DefaultPullMode != want.DefaultPullMode {
		t.Errorf("Get() = %+v, want %+v", got, want)
	}
}

package app

import (
	"context"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/settings"
)

// fakeAPI is a core.API whose OpenWorktree behaviour a test picks; other
// calls are recorded or no-op.
type fakeAPI struct {
	openWorktree func(context.Context, core.RepoPath) (core.WorktreeInfo, error)
	closed       *[]core.WorktreeID
}

func (f fakeAPI) Version(context.Context) (core.Version, error) { return core.Version{}, nil }

func (f fakeAPI) OpenWorktree(ctx context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
	return f.openWorktree(ctx, path)
}

func (f fakeAPI) Summary(context.Context, core.WorktreeID) (core.Summary, error) {
	return core.Summary{}, nil
}

func (f fakeAPI) Refs(context.Context, core.WorktreeID) (core.Refs, error) {
	return core.Refs{}, nil
}

func (f fakeAPI) CommandLog(context.Context, core.WorktreeID) ([]core.CommandLogEntry, error) {
	return nil, nil
}

func (f fakeAPI) LoadLog(context.Context, core.WorktreeID) error { return nil }

func (f fakeAPI) LogRows(context.Context, core.WorktreeID, int, int) (core.LogRowsResult, error) {
	return core.LogRowsResult{}, nil
}

func (f fakeAPI) CommitDetails(context.Context, core.WorktreeID, string) (core.CommitDetails, error) {
	return core.CommitDetails{}, nil
}

func (f fakeAPI) Refresh(context.Context, core.WorktreeID) error { return nil }

func (f fakeAPI) SetFocused(context.Context, core.WorktreeID, bool) error { return nil }

func (f fakeAPI) CloseWorktree(_ context.Context, id core.WorktreeID) error {
	if f.closed != nil {
		*f.closed = append(*f.closed, id)
	}
	return nil
}

func (f fakeAPI) ReloadLog(context.Context, core.WorktreeID) error { return nil }

func (f fakeAPI) LogIndexOf(context.Context, core.WorktreeID, string) (core.LogIndexResult, error) {
	return core.LogIndexResult{}, nil
}

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
	r.OpenOrFocus(core.WorktreeID(path), path)

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

func TestFR_REPO_02_RestoreOrHomeOpensAWindowForEachRememberedWorktree(t *testing.T) {
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

	shell{api: api, reg: reg, store: store, notify: &fakeNotifier{}}.restoreOrHome(context.Background())

	if len(opener.windows) != 2 {
		t.Fatalf("windows opened = %d, want 2", len(opener.windows))
	}
	if !opened["/p1"] || !opened["/p2"] {
		t.Errorf("opened = %v, want both /p1 and /p2", opened)
	}
}

func TestFR_REPO_02_RestoreOrHomeOpensHomeWhenNothingWasOpen(t *testing.T) {
	reg, opener := newTestRegistry()
	store := newTestStore(t)
	api := fakeAPI{openWorktree: func(context.Context, core.RepoPath) (core.WorktreeInfo, error) {
		t.Fatal("OpenWorktree should not be called")
		return core.WorktreeInfo{}, nil
	}}

	shell{api: api, reg: reg, store: store, notify: &fakeNotifier{}}.restoreOrHome(context.Background())

	if len(opener.windows) != 1 {
		t.Fatalf("windows opened = %d, want 1", len(opener.windows))
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

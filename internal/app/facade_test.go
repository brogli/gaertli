package app_test

import (
	"context"
	"testing"

	"github.com/brogli/nebegit/internal/app"
	"github.com/brogli/nebegit/internal/core"
)

type stubCore struct{ v core.Version }

func (s stubCore) Version(context.Context) (core.Version, error) { return s.v, nil }

func (s stubCore) OpenWorktree(context.Context, core.RepoPath) (core.WorktreeInfo, error) {
	return core.WorktreeInfo{}, nil
}

func (s stubCore) Summary(context.Context, core.WorktreeID) (core.Summary, error) {
	return core.Summary{}, nil
}

func (s stubCore) Refs(context.Context, core.WorktreeID) (core.Refs, error) {
	return core.Refs{}, nil
}

func (s stubCore) CommandLog(context.Context, core.WorktreeID) ([]core.CommandLogEntry, error) {
	return nil, nil
}

func (s stubCore) LoadLog(context.Context, core.WorktreeID) error { return nil }

func (s stubCore) LogRows(context.Context, core.WorktreeID, int, int) (core.LogRowsResult, error) {
	return core.LogRowsResult{}, nil
}

func (s stubCore) CommitDetails(context.Context, core.WorktreeID, string) (core.CommitDetails, error) {
	return core.CommitDetails{}, nil
}

func (s stubCore) Refresh(context.Context, core.WorktreeID) error { return nil }

func (s stubCore) SetFocused(context.Context, core.WorktreeID, bool) error { return nil }

func (s stubCore) CloseWorktree(context.Context, core.WorktreeID) error { return nil }

func (s stubCore) ReloadLog(context.Context, core.WorktreeID) error { return nil }

func (s stubCore) LogIndexOf(context.Context, core.WorktreeID, string) (core.LogIndexResult, error) {
	return core.LogIndexResult{}, nil
}

type stubRouter struct {
	local core.API
	other int
}

func (r *stubRouter) Local() core.API                          { return r.local }
func (r *stubRouter) CoreOf(core.WorktreeID) (core.API, error) { r.other++; return nil, nil }
func (r *stubRouter) CoreFor(core.RepoPath) (core.API, error)  { r.other++; return nil, nil }

func TestADR_3_FacadeRoutesWorktreeLessCallsToTheLocalCore(t *testing.T) {
	r := &stubRouter{local: stubCore{core.Version{App: "local"}}}
	f := app.NewFacade(r)

	v, err := f.Version(context.Background())
	if err != nil || v.App != "local" {
		t.Errorf("Version = %+v, %v; want the local core's", v, err)
	}
	if r.other != 0 {
		t.Errorf("CoreOf/CoreFor called %d times", r.other)
	}
}

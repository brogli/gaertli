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

func (s stubCore) DiffFile(context.Context, core.WorktreeID, string, string, string, bool) (core.DiffFile, error) {
	return core.DiffFile{}, nil
}

func (s stubCore) Status(context.Context, core.WorktreeID) (core.ChangedFiles, error) {
	return core.ChangedFiles{}, nil
}

func (s stubCore) StageFile(context.Context, core.WorktreeID, string) error { return nil }

func (s stubCore) UnstageFile(context.Context, core.WorktreeID, string) error { return nil }

func (s stubCore) StageAll(context.Context, core.WorktreeID) error { return nil }

func (s stubCore) UnstageAll(context.Context, core.WorktreeID) error { return nil }

func (s stubCore) StageLines(context.Context, core.WorktreeID, string, core.LineSelection, bool) error {
	return nil
}

func (s stubCore) Commit(context.Context, core.WorktreeID, string, bool) error { return nil }

func (s stubCore) SetMessageDraft(context.Context, core.WorktreeID, string) error { return nil }

func (s stubCore) DiffWorking(context.Context, core.WorktreeID, string, string, bool, bool) (core.DiffFile, error) {
	return core.DiffFile{}, nil
}

func (s stubCore) Refresh(context.Context, core.WorktreeID) error { return nil }

func (s stubCore) SetFocused(context.Context, core.WorktreeID, bool) error { return nil }

func (s stubCore) CloseWorktree(context.Context, core.WorktreeID) error { return nil }

func (s stubCore) ReloadLog(context.Context, core.WorktreeID) error { return nil }

func (s stubCore) LogIndexOf(context.Context, core.WorktreeID, string) (core.LogIndexResult, error) {
	return core.LogIndexResult{}, nil
}

func (s stubCore) CompareCommits(context.Context, core.WorktreeID, string, string) ([]core.ChangedFile, error) {
	return nil, nil
}

func (s stubCore) DiffRange(context.Context, core.WorktreeID, string, string, string, string, bool) (core.DiffFile, error) {
	return core.DiffFile{}, nil
}

func (s stubCore) SetLogFilter(context.Context, core.WorktreeID, core.LogFilter) error { return nil }

func (s stubCore) CreateBranch(context.Context, core.WorktreeID, string, string, bool) error {
	return nil
}

func (s stubCore) Checkout(context.Context, core.WorktreeID, string) error { return nil }

func (s stubCore) CheckoutCommit(context.Context, core.WorktreeID, string) error { return nil }

func (s stubCore) Reset(context.Context, core.WorktreeID, string, core.ResetMode) (core.ResetResult, error) {
	return core.ResetResult{}, nil
}

func (s stubCore) UncommittedCount(context.Context, core.WorktreeID) (int, error) { return 0, nil }

func (s stubCore) CherryPick(context.Context, core.WorktreeID, []string) error { return nil }

func (s stubCore) Revert(context.Context, core.WorktreeID, []string) error { return nil }

func (s stubCore) SetEditorProfile(context.Context, core.EditorProfile) error { return nil }

func (s stubCore) OpenFolder(context.Context, core.WorktreeID) error { return nil }

func (s stubCore) OpenFolderAt(context.Context, core.RepoPath) error { return nil }

func (s stubCore) OpenFile(context.Context, core.WorktreeID, string, int) error { return nil }

func (s stubCore) ListWorktrees(context.Context, core.RepoPath) ([]core.Worktree, error) {
	return nil, nil
}

func (s stubCore) ListBranches(context.Context, core.RepoPath) ([]core.BranchSummary, error) {
	return nil, nil
}

func (s stubCore) AddWorktree(context.Context, core.RepoPath, string, string, string, bool) (core.Worktree, error) {
	return core.Worktree{}, nil
}

func (s stubCore) Jobs(context.Context, core.WorktreeID) ([]core.JobInfo, error) { return nil, nil }

func (s stubCore) CancelJob(context.Context, core.WorktreeID, string) error { return nil }

func (s stubCore) Fetch(context.Context, core.WorktreeID, string) (string, error) { return "", nil }

func (s stubCore) Pull(context.Context, core.WorktreeID, core.PullMode) (string, error) {
	return "", nil
}

func (s stubCore) Push(context.Context, core.WorktreeID, string) (string, error) { return "", nil }

func (s stubCore) ForcePush(context.Context, core.WorktreeID) (string, error) { return "", nil }

func (s stubCore) UpdateBranch(context.Context, core.WorktreeID, string) (string, error) {
	return "", nil
}
func (s stubCore) ContinueOperation(context.Context, core.WorktreeID) error { return nil }

func (s stubCore) AbortOperation(context.Context, core.WorktreeID) error { return nil }

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

package remote_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/remote/remotetest"
)

// fake is a core whose Version behaviour the test picks per call.
type fake struct {
	version func() (core.Version, error)
	sink    core.EventSink

	// paths records the string a RepoPath-taking method last received,
	// keyed by method name; AddWorktree and RemoveWorktree additionally
	// record their target under "<method>:target".
	paths map[string]string
}

func (f *fake) recordPath(method, path string) {
	if f.paths == nil {
		f.paths = map[string]string{}
	}
	f.paths[method] = path
}

func (f *fake) Version(context.Context) (core.Version, error) { return f.version() }

func (f *fake) OpenWorktree(_ context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
	f.recordPath("OpenWorktree", string(path))
	return core.WorktreeInfo{}, nil
}

func (f *fake) Summary(context.Context, core.WorktreeID) (core.Summary, error) {
	return core.Summary{}, nil
}

func (f *fake) Refs(context.Context, core.WorktreeID) (core.Refs, error) {
	return core.Refs{}, nil
}

func (f *fake) CommandLog(context.Context, core.WorktreeID) ([]core.CommandLogEntry, error) {
	return nil, nil
}

func (f *fake) LoadLog(context.Context, core.WorktreeID) error { return nil }

func (f *fake) LogRows(context.Context, core.WorktreeID, int, int) (core.LogRowsResult, error) {
	return core.LogRowsResult{}, nil
}

func (f *fake) CommitDetails(context.Context, core.WorktreeID, string) (core.CommitDetails, error) {
	return core.CommitDetails{}, nil
}

func (f *fake) DiffFile(context.Context, core.WorktreeID, string, string, string, bool) (core.DiffFile, error) {
	return core.DiffFile{}, nil
}

func (f *fake) Status(context.Context, core.WorktreeID) (core.ChangedFiles, error) {
	return core.ChangedFiles{}, nil
}

func (f *fake) StageFile(context.Context, core.WorktreeID, string) error { return nil }

func (f *fake) UnstageFile(context.Context, core.WorktreeID, string) error { return nil }

func (f *fake) StageAll(context.Context, core.WorktreeID) error { return nil }

func (f *fake) UnstageAll(context.Context, core.WorktreeID) error { return nil }

func (f *fake) StageLines(context.Context, core.WorktreeID, string, core.LineSelection, bool) error {
	return nil
}

func (f *fake) DiscardFile(context.Context, core.WorktreeID, string, bool) error { return nil }

func (f *fake) DiscardLines(context.Context, core.WorktreeID, string, core.LineSelection) error {
	return nil
}

func (f *fake) Commit(context.Context, core.WorktreeID, string, bool) error { return nil }

func (f *fake) SetMessageDraft(context.Context, core.WorktreeID, string) error { return nil }

func (f *fake) DiffWorking(context.Context, core.WorktreeID, string, string, bool, bool) (core.DiffFile, error) {
	return core.DiffFile{}, nil
}

func (f *fake) Refresh(context.Context, core.WorktreeID) error { return nil }

func (f *fake) SetFocused(context.Context, core.WorktreeID, bool) error { return nil }

func (f *fake) CloseWorktree(context.Context, core.WorktreeID) error { return nil }

func (f *fake) ReloadLog(context.Context, core.WorktreeID) error { return nil }

func (f *fake) LogIndexOf(context.Context, core.WorktreeID, string) (core.LogIndexResult, error) {
	return core.LogIndexResult{}, nil
}

func (f *fake) CompareCommits(context.Context, core.WorktreeID, string, string) ([]core.ChangedFile, error) {
	return nil, nil
}

func (f *fake) DiffRange(context.Context, core.WorktreeID, string, string, string, string, bool) (core.DiffFile, error) {
	return core.DiffFile{}, nil
}

func (f *fake) DiffAgainstWorking(context.Context, core.WorktreeID, string, string, string, bool) (core.DiffFile, error) {
	return core.DiffFile{}, nil
}

func (f *fake) CompareWithWorking(context.Context, core.WorktreeID, string) ([]core.ChangedFile, error) {
	return nil, nil
}

func (f *fake) SetLogFilter(context.Context, core.WorktreeID, core.LogFilter) error { return nil }

func (f *fake) CreateBranch(context.Context, core.WorktreeID, string, string, bool) error {
	return nil
}

func (f *fake) Checkout(context.Context, core.WorktreeID, string) error { return nil }

func (f *fake) CheckoutCommit(context.Context, core.WorktreeID, string) error { return nil }

func (f *fake) Reset(context.Context, core.WorktreeID, string, core.ResetMode) (core.ResetResult, error) {
	return core.ResetResult{}, nil
}

func (f *fake) UncommittedCount(context.Context, core.WorktreeID) (int, error) { return 0, nil }

func (f *fake) CherryPick(context.Context, core.WorktreeID, []string) error { return nil }

func (f *fake) CherryPickWith(context.Context, core.WorktreeID, []string, core.PickOptions) error {
	return nil
}

func (f *fake) Revert(context.Context, core.WorktreeID, []string) error { return nil }

func (f *fake) RevertWith(context.Context, core.WorktreeID, []string, core.PickOptions) error {
	return nil
}

func (f *fake) SetEditorProfile(context.Context, core.EditorProfile) error { return nil }

func (f *fake) TestEditorProfile(context.Context, core.EditorProfile) error { return nil }

func (f *fake) OpenFolder(context.Context, core.WorktreeID) error { return nil }

func (f *fake) OpenFolderAt(_ context.Context, path core.RepoPath) error {
	f.recordPath("OpenFolderAt", string(path))
	return nil
}

func (f *fake) OpenFile(context.Context, core.WorktreeID, string, int) error { return nil }

func (f *fake) OpenFolderFallback(context.Context, core.WorktreeID, core.EditorFallbackKind) error {
	return nil
}

func (f *fake) OpenFileFallback(context.Context, core.WorktreeID, string, int, core.EditorFallbackKind) error {
	return nil
}

func (f *fake) ListWorktrees(_ context.Context, path core.RepoPath) ([]core.Worktree, error) {
	f.recordPath("ListWorktrees", string(path))
	return nil, nil
}

func (f *fake) ListBranches(_ context.Context, path core.RepoPath) ([]core.BranchSummary, error) {
	f.recordPath("ListBranches", string(path))
	return nil, nil
}

func (f *fake) AddWorktree(_ context.Context, path core.RepoPath, target, _, _ string, _ bool) (core.Worktree, error) {
	f.recordPath("AddWorktree", string(path))
	f.recordPath("AddWorktree:target", target)
	return core.Worktree{}, nil
}

func (f *fake) Jobs(context.Context, core.WorktreeID) ([]core.JobInfo, error) { return nil, nil }

func (f *fake) CancelJob(context.Context, core.WorktreeID, string) error { return nil }

func (f *fake) Fetch(context.Context, core.WorktreeID, string) (string, error) { return "", nil }

func (f *fake) Pull(context.Context, core.WorktreeID, core.PullMode) (string, error) {
	return "", nil
}

func (f *fake) PullWithStash(context.Context, core.WorktreeID, core.PullMode) (string, error) {
	return "", nil
}

func (f *fake) PopStash(context.Context, core.WorktreeID, string) error { return nil }

func (f *fake) DropStash(context.Context, core.WorktreeID, string) error { return nil }

func (f *fake) Push(context.Context, core.WorktreeID, string) (string, error) { return "", nil }

func (f *fake) ForcePush(context.Context, core.WorktreeID) (string, error) { return "", nil }

func (f *fake) UpdateBranch(context.Context, core.WorktreeID, string) (string, error) {
	return "", nil
}
func (f *fake) ContinueOperation(context.Context, core.WorktreeID) error { return nil }

func (f *fake) AbortOperation(context.Context, core.WorktreeID) error { return nil }

func (f *fake) SkipOperation(context.Context, core.WorktreeID) error { return nil }

func (f *fake) RenameBranch(context.Context, core.WorktreeID, string, string) error { return nil }

func (f *fake) DeleteBranch(context.Context, core.WorktreeID, string, bool) error { return nil }

func (f *fake) RemoveWorktree(_ context.Context, path core.RepoPath, target string, _ bool) error {
	f.recordPath("RemoveWorktree", string(path))
	f.recordPath("RemoveWorktree:target", target)
	return nil
}

func (f *fake) PruneWorktrees(_ context.Context, path core.RepoPath) error {
	f.recordPath("PruneWorktrees", string(path))
	return nil
}

// pipeClient serves f over an in-memory pipe and returns a client on it.
func pipeClient(t *testing.T, f *fake, sink core.EventSink) *remote.Client {
	t.Helper()
	return remotetest.NewPipeClient(t, func(sink core.EventSink) core.API {
		f.sink = sink
		return f
	}, sink)
}

func TestADR_3_VersionCallCrossesThePipe(t *testing.T) {
	want := core.Version{App: "1.2.3", Protocol: 7}
	c := pipeClient(t, &fake{version: func() (core.Version, error) { return want, nil }}, nil)

	got, err := c.Version(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("Version = %+v, want %+v", got, want)
	}
}

func TestADR_3_CoreErrorCrossesThePipeWithAllFields(t *testing.T) {
	want := &core.Error{Code: core.NotARepo, Message: "no repo", GitStderr: "x", Command: "git rev-parse"}
	c := pipeClient(t, &fake{version: func() (core.Version, error) { return core.Version{}, want }}, nil)

	_, err := c.Version(context.Background())
	var got *core.Error
	if !errors.As(err, &got) {
		t.Fatalf("err = %#v, want *core.Error", err)
	}
	if *got != *want {
		t.Errorf("error = %+v, want %+v", *got, *want)
	}
}

func TestADR_3_PanicInCoreBecomesInternalErrorAndServerSurvives(t *testing.T) {
	calls := 0
	c := pipeClient(t, &fake{version: func() (core.Version, error) {
		calls++
		if calls == 1 {
			panic("boom")
		}
		return core.Version{App: "ok"}, nil
	}}, nil)

	_, err := c.Version(context.Background())
	var got *core.Error
	if !errors.As(err, &got) || got.Code != core.Internal {
		t.Fatalf("err = %v, want INTERNAL", err)
	}
	v, err := c.Version(context.Background())
	if err != nil || v.App != "ok" {
		t.Errorf("second call = %+v, %v; server did not survive the panic", v, err)
	}
}

func TestADR_3_EventsReachTheClientSink(t *testing.T) {
	got := make(chan core.Event, 1)
	f := &fake{version: func() (core.Version, error) { return core.Version{}, nil }}
	c := pipeClient(t, f, func(e core.Event) { got <- e })
	if _, err := c.Version(context.Background()); err != nil { // ensures the server is up
		t.Fatal(err)
	}

	want := core.Event{Name: "repo:changed", Repo: "r", Worktree: "w1", Payload: json.RawMessage(`{"scope":"refs"}`)}
	f.sink(want)

	select {
	case e := <-got:
		if e.Name != want.Name || e.Repo != want.Repo || e.Worktree != want.Worktree || string(e.Payload) != string(want.Payload) {
			t.Errorf("event = %+v, want %+v", e, want)
		}
	case <-time.After(time.Second):
		t.Fatal("no event within 1s")
	}
}

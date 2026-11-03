// Requirements: FR-CONF-11, QA-PORT-02

package remote_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/remote/remotetest"
)

// fake is a core whose Version behaviour the test picks per call. Of the
// other methods it serves only those taking a RepoPath; the rest panic on
// the nil embedded interface.
type fake struct {
	core.API
	version func() (core.Version, error)
	sink    core.EventSink

	// paths records the string a RepoPath-taking method last received,
	// keyed by method name; AddWorktree and RemoveWorktree additionally
	// record their target under "<method>:target".
	paths map[string]string

	draft *core.MergeDraft
}

func (f *fake) recordPath(method, path string) {
	if f.paths == nil {
		f.paths = map[string]string{}
	}
	f.paths[method] = path
}

func (f *fake) Version(context.Context) (core.Version, error) { return f.version() }

func (f *fake) SetMergeDraft(_ context.Context, _ core.WorktreeID, _ string, draft *core.MergeDraft) error {
	f.draft = draft
	return nil
}

func (f *fake) MergeDraft(context.Context, core.WorktreeID, string) (*core.MergeDraft, error) {
	return f.draft, nil
}

func (f *fake) OpenWorktree(_ context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
	f.recordPath("OpenWorktree", string(path))
	return core.WorktreeInfo{}, nil
}

func (f *fake) OpenFolderAt(_ context.Context, path core.RepoPath) error {
	f.recordPath("OpenFolderAt", string(path))
	return nil
}

func (f *fake) ListWorktrees(_ context.Context, path core.RepoPath) ([]core.Worktree, error) {
	f.recordPath("ListWorktrees", string(path))
	return nil, nil
}

func (f *fake) AddWorktree(_ context.Context, path core.RepoPath, target, _, _ string, _ bool) (core.Worktree, error) {
	f.recordPath("AddWorktree", string(path))
	f.recordPath("AddWorktree:target", target)
	return core.Worktree{}, nil
}

func (f *fake) RemoveWorktree(_ context.Context, path core.RepoPath, target string, _ bool) error {
	f.recordPath("RemoveWorktree", string(path))
	f.recordPath("RemoveWorktree:target", target)
	return nil
}

func (f *fake) PruneWorktrees(_ context.Context, path core.RepoPath) error {
	f.recordPath("PruneWorktrees", string(path))
	return nil
}

func (f *fake) MainWorktree(_ context.Context, path core.RepoPath) (core.Worktree, error) {
	f.recordPath("MainWorktree", string(path))
	return core.Worktree{}, nil
}

// pipeClient serves f over an in-memory pipe and returns a client on it.
func pipeClient(t *testing.T, f *fake, sink core.EventSink) *remote.Client {
	t.Helper()
	return remotetest.NewPipeClient(t, func(sink core.EventSink) core.API {
		f.sink = sink
		return f
	}, sink)
}

func TestClosingAClientDoesNotWaitForTheCoresOutputToEnd(t *testing.T) {
	r, w := io.Pipe()
	t.Cleanup(func() { _ = w.Close() })
	client := remote.NewClient(r, io.Discard, nil)

	closed := make(chan struct{})
	go func() { _ = client.Close(); close(closed) }()

	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close still waits 1s later for the core's output to end")
	}
}

func TestVersionCallCrossesThePipe(t *testing.T) {
	want := core.Version{App: "1.2.3", Protocol: 7}
	c := pipeClient(t, &fake{version: func() (core.Version, error) { return want, nil }}, nil)

	got, err := c.Version(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("Version = %+v, want %+v", got, want)
	}
}

func TestMergeDraftRoundTripsThroughTheProtocol(t *testing.T) {
	c := pipeClient(t, &fake{}, nil)
	want := &core.MergeDraft{
		Text: "a\nb\n", Fingerprint: "fp",
		Regions: []core.DraftRegion{{Range: core.LineRange{Start: 2, Lines: 1}, Resolved: true}},
	}

	if err := c.SetMergeDraft(t.Context(), "/wt", "a.txt", want); err != nil {
		t.Fatal(err)
	}
	got, err := c.MergeDraft(t.Context(), "/wt", "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MergeDraft = %+v, want %+v", got, want)
	}
}

func TestCoreErrorCrossesThePipeWithAllFields(t *testing.T) {
	want := &core.Error{Code: core.NotARepo, Message: "no repo", Key: "notARepo", Args: map[string]string{"path": "x"}, GitStderr: "x", Command: "git rev-parse"}
	c := pipeClient(t, &fake{version: func() (core.Version, error) { return core.Version{}, want }}, nil)

	_, err := c.Version(t.Context())
	got, ok := errors.AsType[*core.Error](err)
	if !ok {
		t.Fatalf("err = %#v, want *core.Error", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("error = %+v, want %+v", got, want)
	}
}

func TestPanicInCoreBecomesInternalErrorAndServerSurvives(t *testing.T) {
	calls := 0
	c := pipeClient(t, &fake{version: func() (core.Version, error) {
		calls++
		if calls == 1 {
			panic("boom")
		}
		return core.Version{App: "ok"}, nil
	}}, nil)

	_, err := c.Version(t.Context())
	if got, ok := errors.AsType[*core.Error](err); !ok || got.Code != core.Internal {
		t.Fatalf("err = %v, want INTERNAL", err)
	}
	v, err := c.Version(t.Context())
	if err != nil || v.App != "ok" {
		t.Errorf("second call = %+v, %v; server did not survive the panic", v, err)
	}
}

func TestEventsReachTheClientSink(t *testing.T) {
	got := make(chan core.Event, 1)
	f := &fake{version: func() (core.Version, error) { return core.Version{}, nil }}
	c := pipeClient(t, f, func(e core.Event) { got <- e })
	if _, err := c.Version(t.Context()); err != nil { // ensures the server is up
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

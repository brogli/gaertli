package remote_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/remote"
)

// fake is a core whose Version behaviour the test picks per call.
type fake struct {
	version func() (core.Version, error)
	sink    core.EventSink
}

func (f *fake) Version(context.Context) (core.Version, error) { return f.version() }

func (f *fake) OpenWorktree(context.Context, core.RepoPath) (core.WorktreeInfo, error) {
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

func (f *fake) SetLogFilter(context.Context, core.WorktreeID, core.LogFilter) error { return nil }

// pipeClient serves f over an in-memory pipe and returns a client on it.
func pipeClient(t *testing.T, f *fake, sink core.EventSink) *remote.Client {
	t.Helper()
	clientIn, serverOut := io.Pipe()
	serverIn, clientOut := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = remote.Serve(ctx, serverIn, serverOut, func(sink core.EventSink) core.API {
			f.sink = sink
			return f
		})
	}()
	t.Cleanup(func() {
		cancel()
		_ = clientOut.Close()
		_ = serverOut.Close()
	})
	return remote.NewClient(clientIn, clientOut, sink)
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

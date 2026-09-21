package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestManagerOpenReusesSessionForSameToplevel(t *testing.T) {
	repo := gittest.New(t)
	sub := filepath.Join(repo.Dir, "sub", "dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	m := NewManager(func(core.Event) {})

	first, err := m.Open(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.Open(context.Background(), core.RepoPath(sub))
	if err != nil {
		t.Fatal(err)
	}

	if first != second {
		t.Errorf("Open on a subdirectory minted a new session, want the same *Worktree")
	}
}

func TestManagerOpenSharesRepoAcrossLinkedWorktrees(t *testing.T) {
	repo := gittest.New(t)
	linked := filepath.Join(t.TempDir(), "linked")
	run(t, repo.Dir, "worktree", "add", "-b", "feature", linked)
	m := NewManager(func(core.Event) {})

	main, err := m.Open(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	other, err := m.Open(context.Background(), core.RepoPath(linked))
	if err != nil {
		t.Fatal(err)
	}

	if main.ID == other.ID {
		t.Errorf("linked worktrees got the same ID %q", main.ID)
	}
	if main.Repo != other.Repo {
		t.Errorf("linked worktrees did not share a *Repo")
	}
}

func TestManagerOpenRejectsNonRepoPath(t *testing.T) {
	m := NewManager(func(core.Event) {})

	_, err := m.Open(context.Background(), core.RepoPath(t.TempDir()))

	assertCode(t, err, core.NotARepo)
}

func TestManagerOpenRejectsMissingPath(t *testing.T) {
	m := NewManager(func(core.Event) {})

	_, err := m.Open(context.Background(), core.RepoPath(filepath.Join(t.TempDir(), "missing")))

	assertCode(t, err, core.NotARepo)
}

func TestManagerGetReturnsInternalErrorForUnknownID(t *testing.T) {
	m := NewManager(func(core.Event) {})

	_, err := m.Get("bogus")

	assertCode(t, err, core.Internal)
}

func TestWorktreeRunnerAppendsToCommandLogAndEmitsSink(t *testing.T) {
	repo := gittest.New(t)
	events := make(chan core.Event, 1)
	m := NewManager(func(e core.Event) { events <- e })

	wt, err := m.Open(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Runner.Run(context.Background(), git.Call{Dir: wt.Path, ReadOnly: true, Args: []string{"status"}}); err != nil {
		t.Fatal(err)
	}

	if got := wt.CommandLog(); len(got) != 1 {
		t.Fatalf("CommandLog() = %d entries, want 1", len(got))
	}

	select {
	case e := <-events:
		if e.Name != "command" || e.Worktree != wt.ID {
			t.Errorf("event = %+v, want Name \"command\", Worktree %q", e, wt.ID)
		}
	default:
		t.Error("sink received no event")
	}
}

func TestWorktreeCommandLogCapsAtLogCap(t *testing.T) {
	wt := &Worktree{}
	for i := range logCap + 1 {
		wt.append(core.CommandLogEntry{Command: string(rune('a' + i%26))})
	}

	got := wt.CommandLog()
	if len(got) != logCap {
		t.Fatalf("CommandLog() = %d entries, want %d", len(got), logCap)
	}
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := (git.Runner{}).Run(context.Background(), git.Call{Dir: dir, Args: args}); err != nil {
		t.Fatal(err)
	}
}

func assertCode(t *testing.T, err error, want core.Code) {
	t.Helper()
	var ce *core.Error
	if !errors.As(err, &ce) || ce.Code != want {
		t.Errorf("err = %v, want *core.Error{Code: %s}", err, want)
	}
}

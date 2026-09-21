package services_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
	"github.com/brogli/nebegit/internal/services"
)

func TestFR_REPO_01_OpenWorktreeReusesSessionForSameToplevel(t *testing.T) {
	repo := gittest.New(t)
	sub := filepath.Join(repo.Dir, "sub", "dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	c := services.New(func(core.Event) {})

	first, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.OpenWorktree(context.Background(), core.RepoPath(sub))
	if err != nil {
		t.Fatal(err)
	}

	if first.ID != second.ID {
		t.Errorf("ID = %q, %q, want equal", first.ID, second.ID)
	}
}

func TestFR_REPO_01_OpenWorktreeGivesLinkedWorktreesDistinctIDsSameRepoKey(t *testing.T) {
	repo := gittest.New(t)
	commit(t, repo.Dir, "seed.txt", "x", "seed")
	linked := filepath.Join(t.TempDir(), "linked")
	run(t, repo.Dir, "worktree", "add", "-b", "feature", linked)
	c := services.New(func(core.Event) {})

	main, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	other, err := c.OpenWorktree(context.Background(), core.RepoPath(linked))
	if err != nil {
		t.Fatal(err)
	}

	if main.ID == other.ID {
		t.Errorf("linked worktrees got the same ID %q", main.ID)
	}
	if main.RepoKey != other.RepoKey {
		t.Errorf("RepoKey = %q, %q, want equal", main.RepoKey, other.RepoKey)
	}
}

func TestFR_REPO_05_OpenWorktreeRejectsPlainDirectory(t *testing.T) {
	c := services.New(func(core.Event) {})

	_, err := c.OpenWorktree(context.Background(), core.RepoPath(t.TempDir()))

	assertCode(t, err, core.NotARepo)
}

func TestFR_REPO_05_OpenWorktreeRejectsMissingPath(t *testing.T) {
	c := services.New(func(core.Event) {})

	_, err := c.OpenWorktree(context.Background(), core.RepoPath(filepath.Join(t.TempDir(), "missing")))

	assertCode(t, err, core.NotARepo)
}

func TestFR_REPO_03_SummaryReportsBranch(t *testing.T) {
	repo := gittest.New(t)
	commit(t, repo.Dir, "seed.txt", "x", "seed")
	run(t, repo.Dir, "switch", "-c", "feature")
	c := services.New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	got, err := c.Summary(context.Background(), wt.ID)
	if err != nil {
		t.Fatal(err)
	}

	if got.Branch != "feature" || got.Detached || got.Operation != "" {
		t.Errorf("Summary = %+v, want Branch feature, Detached false, Operation \"\"", got)
	}
}

func TestFR_REPO_03_SummaryReportsDetachedHEAD(t *testing.T) {
	repo := gittest.New(t)
	commit(t, repo.Dir, "seed.txt", "x", "seed")
	sha := headSHA(t, repo.Dir)
	run(t, repo.Dir, "switch", "--detach", sha)
	c := services.New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	got, err := c.Summary(context.Background(), wt.ID)
	if err != nil {
		t.Fatal(err)
	}

	if !got.Detached || got.Branch != "" {
		t.Errorf("Summary = %+v, want Detached true, Branch \"\"", got)
	}
}

func TestFR_REPO_03_SummaryReportsMergeInProgress(t *testing.T) {
	repo := gittest.New(t)
	commit(t, repo.Dir, "file.txt", "base\n", "base")
	run(t, repo.Dir, "branch", "other")
	commit(t, repo.Dir, "file.txt", "main change\n", "on main")

	linked := filepath.Join(t.TempDir(), "linked")
	run(t, repo.Dir, "worktree", "add", linked, "other")
	commit(t, linked, "file.txt", "other change\n", "on other")
	_, _ = (git.Runner{}).Run(context.Background(), git.Call{Dir: linked, Args: []string{"merge", "master"}})
	if _, err := os.Stat(filepath.Join(linked, "file.txt")); err != nil {
		t.Fatal(err)
	}

	c := services.New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(linked))
	if err != nil {
		t.Fatal(err)
	}

	got, err := c.Summary(context.Background(), wt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Operation != "merge" {
		t.Errorf("Operation = %q, want %q", got.Operation, "merge")
	}
}

func TestFR_REPO_03_SummaryReportsRebaseInProgress(t *testing.T) {
	repo := gittest.New(t)
	commit(t, repo.Dir, "seed.txt", "x", "seed")
	if err := os.Mkdir(filepath.Join(repo.Dir, ".git", "rebase-merge"), 0o755); err != nil {
		t.Fatal(err)
	}
	c := services.New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	got, err := c.Summary(context.Background(), wt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Operation != "rebase" {
		t.Errorf("Operation = %q, want %q", got.Operation, "rebase")
	}
}

func TestFR_REPO_04_SetFocusedTrueRunsAnImmediatePollerCheck(t *testing.T) {
	repo := gittest.New(t)
	commit(t, repo.Dir, "seed.txt", "x", "seed")
	c := services.New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetFocused(context.Background(), wt.ID, false); err != nil {
		t.Fatal(err)
	}
	before, err := c.CommandLog(context.Background(), wt.ID)
	if err != nil {
		t.Fatal(err)
	}

	if err := c.SetFocused(context.Background(), wt.ID, true); err != nil {
		t.Fatal(err)
	}

	after, err := c.CommandLog(context.Background(), wt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before)+1 {
		t.Fatalf("CommandLog grew by %d, want 1 (an immediate poll)", len(after)-len(before))
	}
}

func TestFR_REPO_04_RefreshRunsTheWatcherAndPollerChecksPromptly(t *testing.T) {
	repo := gittest.New(t)
	commit(t, repo.Dir, "seed.txt", "x", "seed")
	events := make(chan core.Event, 16)
	c := services.New(func(e core.Event) { events <- e })
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	drainEvents(events)

	// an external commit just before Refresh: the watcher's baseline
	// predates it, so only a fresh CheckNow notices it without waiting for
	// the fallback ticker's own interval.
	commit(t, repo.Dir, "seed.txt", "y", "second")
	before, err := c.CommandLog(context.Background(), wt.ID)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	if err := c.Refresh(context.Background(), wt.ID); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("Refresh took %v, want a prompt return", elapsed)
	}

	after, err := c.CommandLog(context.Background(), wt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) <= len(before) {
		t.Errorf("CommandLog did not grow, want the poller's CheckNow to have run")
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case e := <-events:
			if e.Name == "repo:changed" && string(e.Payload) == `{"scope":"refs"}` {
				return
			}
		case <-deadline:
			t.Fatal("no repo:changed(refs) event after Refresh (the watcher's CheckNow)")
		}
	}
}

func commit(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", "--", name)
	if _, err := (git.Runner{}).Run(context.Background(), git.Call{Dir: dir, Args: []string{"commit", "-F", "-"}, Stdin: []byte(msg)}); err != nil {
		t.Fatal(err)
	}
}

func headSHA(t *testing.T, dir string) string {
	t.Helper()
	result, err := (git.Runner{}).Run(context.Background(), git.Call{Dir: dir, ReadOnly: true, Args: []string{"rev-parse", "HEAD"}})
	if err != nil {
		t.Fatal(err)
	}
	return string(result.Stdout[:len(result.Stdout)-1])
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

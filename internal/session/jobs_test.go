package session

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git/gittest"
	"github.com/brogli/nebegit/internal/jobs"
)

func TestFR_SYNC_07_RepoJobsEmitsJobUpdateEventsScopedToTheRepo(t *testing.T) {
	repo := gittest.New(t)
	events := make(chan core.Event, 16)
	m := NewManager(func(e core.Event) { events <- e })

	wt, err := m.Open(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	drainEvents(events)

	wt.Repo.Jobs().Start("test", func(ctx context.Context, progress jobs.Progress) (any, error) {
		progress(50, "halfway")
		return nil, nil
	})

	sawProgress, sawDone := false, false
	deadline := time.After(2 * time.Second)
	for !sawProgress || !sawDone {
		select {
		case e := <-events:
			if e.Name != "job:update" {
				continue
			}
			if e.Worktree != "" {
				t.Errorf("job:update Worktree = %q, want empty (repo-scoped)", e.Worktree)
			}
			if e.Repo != wt.Repo.Key {
				t.Errorf("job:update Repo = %q, want %q", e.Repo, wt.Repo.Key)
			}
			var info core.JobInfo
			if err := json.Unmarshal(e.Payload, &info); err != nil {
				t.Fatal(err)
			}
			if info.Progress == 50 {
				sawProgress = true
			}
			if info.State == "done" {
				sawDone = true
			}
		case <-deadline:
			t.Fatal("timed out waiting for job:update progress and terminal events")
		}
	}
}

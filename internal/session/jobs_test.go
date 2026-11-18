// Requirements: FR-SYNC-07, QA-RESP-01, QA-SAFE-04

package session

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/core/coretest"
	"github.com/brogli/gaertli/internal/git/gittest"
	"github.com/brogli/gaertli/internal/jobs"
)

func TestRepoJobsEmitsJobUpdateEventsScopedToTheRepo(t *testing.T) {
	repo := gittest.New(t)
	events := make(chan core.Event, 16)
	m := NewManager(func(e core.Event) { events <- e })

	wt := openSession(t, m, repo.Dir)
	coretest.DrainEvents(events)

	wt.Repo.Jobs().Start(t.Context(), "test", func(ctx context.Context, progress jobs.Progress) (any, error) {
		progress(50, "halfway")
		return nil, nil
	})

	sawProgress, sawDone := false, false
	for !sawProgress || !sawDone {
		e := <-events
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
		if info.State == core.JobDone {
			sawDone = true
		}
	}
}

//go:build race

// Requirements: QA-REL-04

package services

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git/gittest"
)

func TestLogStaysConsistentUnderConcurrentLoadsReloadsAndFilters(t *testing.T) {
	const (
		commits    = 12
		goroutines = 12
		iterations = 15
		refMoves   = 6
	)
	repo := gittest.New(t)
	for i := range commits {
		gittest.Commit(t, repo.Dir, "a.txt", fmt.Sprint(i), fmt.Sprintf("c%d", i))
	}
	gittest.Run(t, repo.Dir, "branch", "side")
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", linked, "side")

	c := New(func(core.Event) {})
	ids := []core.WorktreeID{openOn(t, c, repo.Dir).ID, openOn(t, c, linked).ID}

	var wg sync.WaitGroup
	errs := make(chan error, goroutines*iterations*4)
	for g := range goroutines {
		id := ids[g%len(ids)]
		wg.Go(func() {
			for i := range iterations {
				ctx := t.Context()
				_, rowsErr := c.LogRows(ctx, id, 0, commits)
				errs <- errors.Join(
					c.LoadLog(ctx, id),
					c.ReloadLog(ctx, id),
					rowsErr,
				)
				if i%3 == g%3 {
					errs <- c.SetLogFilter(ctx, id, core.LogFilter{Branches: []string{"side"}})
				}
			}
		})
	}
	for i := range refMoves {
		gittest.Run(t, repo.Dir, "branch", fmt.Sprintf("moved-%d", i))
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}

	for _, id := range ids {
		if err := c.SetLogFilter(t.Context(), id, core.LogFilter{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.ReloadLog(t.Context(), ids[0]); err != nil {
		t.Fatal(err)
	}
	res := waitForLogRows(t, c, ids[0])
	if res.Error != nil {
		t.Errorf("Error = %+v, want none", res.Error)
	}
	if len(res.Rows) != commits {
		t.Errorf("Rows = %d, want the repo's %d commits", len(res.Rows), commits)
	}
}

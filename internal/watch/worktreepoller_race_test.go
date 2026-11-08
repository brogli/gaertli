//go:build race

// Requirements: QA-REL-04

package watch

import (
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
)

func TestWorktreePollerStopsCleanlyAfterConcurrentFocusBlurAndCheckNow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const workers = 12
		// synctest cannot run git, so every poll fails: the test covers the
		// poller's own state, and synctest fails on a goroutine left running.
		runner := git.Runner{Git: nonexistentGit}
		p := NewWorktreePoller(runner, t.TempDir(), func(core.ChangeScope) {})

		hammer := func(worker int) {
			for i := range 40 {
				switch (worker + i) % 3 {
				case 0:
					p.Focus()
				case 1:
					p.Blur()
				default:
					p.CheckNow()
				}
				time.Sleep(PollInterval / 7)
			}
		}

		var wg sync.WaitGroup
		for worker := range workers {
			wg.Go(func() { hammer(worker) })
		}
		wg.Wait()
		p.Close()

		var after sync.WaitGroup
		for range workers {
			after.Go(func() {
				p.Blur()
				p.CheckNow()
				p.Close()
			})
		}
		after.Wait()
	})
}

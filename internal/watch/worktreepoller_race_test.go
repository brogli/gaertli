//go:build race

// Requirements: QA-REL-04

package watch

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
	"github.com/brogli/gaertli/internal/git/gittest"
)

func TestWorktreePollerReportsEditsUnderConcurrentFocusBlurCheckNowAndClose(t *testing.T) {
	setInterval(t, &PollInterval, 5*time.Millisecond)
	const workers, iterations = 12, 40

	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	var changes atomic.Int32
	p := NewWorktreePoller(git.Runner{}, repo.Dir, func(core.ChangeScope) { changes.Add(1) })

	var wg sync.WaitGroup
	for worker := range workers {
		wg.Go(func() {
			for i := range iterations {
				switch (worker + i) % 3 {
				case 0:
					p.Focus()
				case 1:
					p.Blur()
				default:
					p.CheckNow()
				}
				time.Sleep(time.Millisecond)
			}
		})
	}
	wg.Go(func() {
		for i := range iterations {
			if err := os.WriteFile(filepath.Join(repo.Dir, "seed.txt"), []byte(fmt.Sprint(i)), 0o644); err != nil {
				t.Error(err)
			}
			time.Sleep(2 * time.Millisecond)
		}
	})
	wg.Wait()

	p.CheckNow()
	if changes.Load() == 0 {
		t.Error("no edit reported")
	}

	var after sync.WaitGroup
	for range workers {
		after.Go(func() {
			p.Close()
			p.Focus()
			p.CheckNow()
		})
	}
	after.Wait()
}

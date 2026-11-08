//go:build race

// Requirements: QA-REL-04

package jobs

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/brogli/gaertli/internal/core"
)

func TestManagerStaysConsistentUnderConcurrentCalls(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const workers, iterations = 12, 100
		kinds := []core.JobKind{"a", "b", "c"}
		m := NewManager(func(Job) {})

		spamThenWait := func(ctx context.Context, progress Progress) (any, error) {
			for p := range 20 {
				progress(p*5, "working")
			}
			<-ctx.Done()
			return nil, ctx.Err()
		}
		spamThenReturn := func(_ context.Context, progress Progress) (any, error) {
			for p := range 20 {
				progress(p*5, "working")
			}
			return nil, nil
		}

		var wg sync.WaitGroup
		for w := range workers {
			wg.Go(func() {
				var last ID
				for i := range iterations {
					kind := kinds[(w+i)%len(kinds)]
					branch := fmt.Sprint("branch", i%2)
					switch i % 4 {
					case 0:
						last = m.Start(t.Context(), kind, spamThenWait)
					case 1:
						last = m.StartForBranch(t.Context(), kind, branch, spamThenReturn)
					case 2:
						m.Cancel(last)
					default:
						assertNoDuplicateActive(t, m.List())
					}
				}
			})
		}
		wg.Wait()

		for running := true; running; {
			running = false
			for _, j := range m.List() {
				if !isTerminal(j.State) {
					running = true
					m.Cancel(j.ID)
				}
			}
			synctest.Wait()
		}
		assertNoDuplicateActive(t, m.List())
	})
}

func assertNoDuplicateActive(t *testing.T, list []Job) {
	t.Helper()
	type scope struct {
		kind   core.JobKind
		branch string
	}
	seen := map[scope]ID{}
	for _, j := range list {
		if isTerminal(j.State) {
			continue
		}
		s := scope{j.Kind, j.Branch}
		if prev, dup := seen[s]; dup {
			t.Errorf("jobs %s and %s are both active for %+v", prev, j.ID, s)
		}
		seen[s] = j.ID
	}
}

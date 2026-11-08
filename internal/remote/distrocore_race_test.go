//go:build race

// Requirements: QA-REL-04

package remote_test

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/brogli/gaertli/internal/remote"
	"github.com/brogli/gaertli/internal/testwait"
)

// ensureConcurrently runs n Ensure calls at once and returns how many
// reported a respawn.
func ensureConcurrently(t *testing.T, dc *remote.DistroCore, n int) (respawns int) {
	t.Helper()
	var wg sync.WaitGroup
	var count atomic.Int32
	for range n {
		wg.Go(func() {
			if _, respawned, err := dc.Ensure(t.Context()); err != nil {
				t.Error(err)
			} else if respawned {
				count.Add(1)
			}
		})
	}
	wg.Wait()
	return int(count.Load())
}

func TestConcurrentEnsureSharesOneSpawnAndOneOnConnectAlsoAfterAKill(t *testing.T) {
	const callers = 8
	dc, wsl := newDistroCore(t, coreBin, slog.Default())
	var connects atomic.Int32
	dc.OnConnect(func(context.Context, *remote.Client) { connects.Add(1) })

	if got := ensureConcurrently(t, dc, callers); got != 1 {
		t.Fatalf("respawn reports from %d concurrent first Ensure = %d, want 1", callers, got)
	}
	if len(wsl.spawns) != 1 || connects.Load() != 1 {
		t.Fatalf("after first round: spawns = %d, onConnect = %d, want 1 and 1", len(wsl.spawns), connects.Load())
	}

	if err := wsl.spawns[0].Process.Kill(); err != nil {
		t.Fatal(err)
	}

	// the kill is observed asynchronously: Ensure rounds return the old
	// client until then
	testwait.Until(func() bool { return ensureConcurrently(t, dc, callers) != 0 })
	if len(wsl.spawns) != 2 || connects.Load() != 2 {
		t.Fatalf("after the kill: spawns = %d, onConnect = %d, want 2 and 2", len(wsl.spawns), connects.Load())
	}
}

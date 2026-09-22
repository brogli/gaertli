package services_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git/gittest"
	"github.com/brogli/nebegit/internal/services"
)

func TestFR_UX_02_CommandLogReturnsBufferedEntryAndSinkReceivesMatchingEvent(t *testing.T) {
	repo := gittest.New(t)
	events := make(chan core.Event, 8)
	c := services.New(func(e core.Event) { events <- e })

	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	drainEvents(events) // the poller's own baseline status check

	if _, err := c.Summary(context.Background(), wt.ID); err != nil {
		t.Fatal(err)
	}

	got, err := c.CommandLog(context.Background(), wt.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := got[len(got)-1]

	// Summary can run more than one git call (ahead/behind);
	// drain every "command" event the sink received and check the last
	// one, not just the first.
	var payload core.CommandLogEntry
	received := 0
	for drained := false; !drained; {
		select {
		case e := <-events:
			if e.Name != "command" || e.Worktree != wt.ID {
				t.Fatalf("event = %+v, want Name \"command\", Worktree %q", e, wt.ID)
			}
			if err := json.Unmarshal(e.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			received++
		default:
			drained = true
		}
	}
	if received == 0 {
		t.Fatal("sink received no event")
	}
	if payload != want {
		t.Errorf("last event payload = %+v, want %+v", payload, want)
	}
}

// drainEvents discards whatever is already buffered, without waiting.
func drainEvents(events chan core.Event) {
	for {
		select {
		case <-events:
		default:
			return
		}
	}
}

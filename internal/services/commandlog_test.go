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
	events := make(chan core.Event, 1)
	c := services.New(func(e core.Event) { events <- e })

	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Summary(context.Background(), wt.ID); err != nil {
		t.Fatal(err)
	}

	got, err := c.CommandLog(context.Background(), wt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("CommandLog() = %d entries, want 1", len(got))
	}

	select {
	case e := <-events:
		if e.Name != "command" || e.Worktree != wt.ID {
			t.Fatalf("event = %+v, want Name \"command\", Worktree %q", e, wt.ID)
		}
		var payload core.CommandLogEntry
		if err := json.Unmarshal(e.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if payload != got[0] {
			t.Errorf("event payload = %+v, want %+v", payload, got[0])
		}
	default:
		t.Error("sink received no event")
	}
}

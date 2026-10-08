package services

import (
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestQA_PERF_02_FR_LOG_09_FirstRowObservableWhileLoadingWellUnderFullStreamDuration(t *testing.T) {
	repo := gittest.New(t)
	c, id := openWorktree(t, repo.Dir)
	streamFakeLog(t, c, id, 5, 200) // 5 records * 200ms = 1s unstreamed

	start := time.Now()
	if err := c.LoadLog(t.Context(), id); err != nil {
		t.Fatal(err)
	}

	var elapsed time.Duration
	var observed core.LogRowsResult
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		res, err := c.LogRows(t.Context(), id, 0, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Rows) > 0 {
			elapsed = time.Since(start)
			observed = res
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if elapsed == 0 {
		t.Fatal("no row observed within 500ms")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("first row observable after %s, want well under 500ms (the full 1s unstreamed load)", elapsed)
	}

	if observed.Done {
		t.Error("Done = true with the first row, want a partial load")
	}
	if observed.Error != nil {
		t.Errorf("Error = %v, want nil", observed.Error)
	}
	if len(observed.Rows) != 1 {
		t.Errorf("Rows = %d, want exactly the 1 asked for", len(observed.Rows))
	}

	waitForLogDone(t, c, id) // drain the background stream so it doesn't leak noise into later tests
}

func waitForLogDone(t *testing.T, c *Core, id core.WorktreeID) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		res, err := c.LogRows(t.Context(), id, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if res.Error != nil {
			t.Fatal(res.Error)
		}
		if res.Done {
			return res.Loaded
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("log did not finish loading in time")
	return 0
}

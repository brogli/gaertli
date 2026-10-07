package services

import (
	"runtime"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestQA_PERF_02_FirstRowObservableWellUnderFullStreamDuration(t *testing.T) {
	repo := gittest.New(t)
	c, id := openWorktree(t, repo.Dir)
	streamFakeLog(t, c, id, 5, 200) // 5 records * 200ms = 1s unstreamed

	start := time.Now()
	if err := c.LoadLog(t.Context(), id); err != nil {
		t.Fatal(err)
	}

	var elapsed time.Duration
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		res, err := c.LogRows(t.Context(), id, 0, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Rows) > 0 {
			elapsed = time.Since(start)
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

	waitForLogDone(t, c, id) // drain the background stream so it doesn't leak noise into later tests
}

// The 500MB-at-100k-commits target is tools/bench's case.
func TestQA_PERF_07_LogMemoryStaysBoundedPerRow(t *testing.T) {
	const rows = 20000
	repo := gittest.New(t)
	c, id := openWorktree(t, repo.Dir)
	streamFakeLog(t, c, id, rows, 0) // fast, this case measures memory not timing

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	if err := c.LoadLog(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	loaded := waitForLogDone(t, c, id)
	if loaded != rows {
		t.Fatalf("loaded = %d, want %d", loaded, rows)
	}

	runtime.GC()
	runtime.ReadMemStats(&after)

	const perRowBudget = 2000                                 // generous multiple of "a few hundred bytes"
	grown := int64(after.HeapAlloc) - int64(before.HeapAlloc) // may go negative: concurrent GC of other tests' garbage
	if grown > int64(rows*perRowBudget) {
		t.Errorf("heap grew %d bytes for %d rows (%d bytes/row), want under %d bytes/row",
			grown, rows, grown/rows, perRowBudget)
	}
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

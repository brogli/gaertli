package services

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git/gittest"
)

// TestQA_PERF_02_FirstRowObservableWellUnderFullStreamDuration proves the
// streaming mechanism QA-PERF-02 depends on: a row is usable long before
// the whole (delayed) stream finishes, not just after it.
func TestQA_PERF_02_FirstRowObservableWellUnderFullStreamDuration(t *testing.T) {
	repo := gittest.New(t)
	c := New(func(core.Event) {})
	info, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	wt, err := c.sessions.Get(info.ID)
	if err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(t.TempDir(), "records")
	if err := os.WriteFile(file, buildLogFixture(5), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKEGIT_STREAM_FILE", file)
	t.Setenv("FAKEGIT_STREAM_DELAY_MS", "500") // 5 records * 500ms = 2.5s unstreamed
	wt.Runner.Git = fakegit

	start := time.Now()
	if err := c.LoadLog(context.Background(), info.ID); err != nil {
		t.Fatal(err)
	}

	var elapsed time.Duration
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		res, err := c.LogRows(context.Background(), info.ID, 0, 1)
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
		t.Fatal("no row observed within 1s")
	}
	if elapsed > time.Second {
		t.Errorf("first row observable after %s, want well under 1s (the full 2.5s unstreamed load)", elapsed)
	}

	waitForLogDone(t, c, info.ID) // drain the background stream so it doesn't leak noise into later tests
}

// TestQA_PERF_07_LogMemoryStaysBoundedPerRow proves the per-row memory
// shape QA-PERF-07 depends on, at a scale a unit test can run fast and
// deterministically. The literal 500MB-at-100k-commits target is spec 13's
// benchmark.
func TestQA_PERF_07_LogMemoryStaysBoundedPerRow(t *testing.T) {
	const rows = 20000
	repo := gittest.New(t)
	c := New(func(core.Event) {})
	info, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	wt, err := c.sessions.Get(info.ID)
	if err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(t.TempDir(), "records")
	if err := os.WriteFile(file, buildLogFixture(rows), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKEGIT_STREAM_FILE", file) // no FAKEGIT_STREAM_DELAY_MS: fast, this case measures memory not timing
	wt.Runner.Git = fakegit

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	if err := c.LoadLog(context.Background(), info.ID); err != nil {
		t.Fatal(err)
	}
	loaded := waitForLogDone(t, c, info.ID)
	if loaded != rows {
		t.Fatalf("loaded = %d, want %d", loaded, rows)
	}

	runtime.GC()
	runtime.ReadMemStats(&after)

	const perRowBudget = 2000                                 // generous multiple of ADR-5's "a few hundred bytes"
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
		res, err := c.LogRows(context.Background(), id, 0, 0)
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

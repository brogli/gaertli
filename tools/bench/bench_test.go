//go:build bench

// Package bench proves the literal QA-PERF-02/06/07 numbers against the
// real reference repo, through the in-process services.Core — the same call
// path the shell uses, no mock.
//
// The build tag keeps a 100k-commit fixture out of the fast, offline
// default run (QA-MNT-03). tools/trace scans source text, not compiled
// output, so these test names still count as coverage (TR-30). Run via
// `task bench` = `go test -tags=bench ...`.
package bench

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/services"
	"github.com/brogli/nebegit/tools/mkrepo"
)

// refDir is relative to this package's own directory (go test's cwd), not
// the repo root Taskfile's "bench" task runs from — "../.." gets back to
// the same .bench/reference-repo either way.
const refDir = "../../.bench/reference-repo"

// openReference builds (or reuses) the reference repo and returns a real
// services.Core with a worktree already open on it.
func openReference(t *testing.T) (*services.Core, core.WorktreeID) {
	t.Helper()
	if err := mkrepo.Build(t.Context(), refDir, mkrepo.Params); err != nil {
		t.Fatal(err)
	}
	c := services.New(func(core.Event) {})
	wt, err := c.OpenWorktree(t.Context(), core.RepoPath(refDir))
	if err != nil {
		t.Fatal(err)
	}
	return c, wt.ID
}

func TestQA_PERF_02_FirstLogRowUnder1s(t *testing.T) {
	c, id := openReference(t)
	start := time.Now()
	if err := c.LoadLog(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	for {
		rows, err := c.LogRows(t.Context(), id, 0, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows.Rows) > 0 {
			elapsed := time.Since(start)
			t.Logf("first row after %s", elapsed)
			if elapsed > time.Second {
				t.Fatalf("QA-PERF-02: first row after %s, want <= 1s", elapsed)
			}
			return
		}
	}
}

func TestQA_PERF_07_MemoryUnder500MB(t *testing.T) {
	c, id := openReference(t)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	if err := c.LoadLog(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	waitLogDone(t, c, id)
	runtime.GC()
	runtime.ReadMemStats(&after)
	grown := after.HeapAlloc - before.HeapAlloc
	t.Logf("heap growth after loading 100k commits: %d MB", grown/(1<<20))
	if grown > 500*(1<<20) {
		t.Fatalf("QA-PERF-07: heap grew %d MB, want <= 500", grown/(1<<20))
	}
}

func TestQA_PERF_06_RefreshUnder1s(t *testing.T) {
	c, id := openReference(t)
	start := time.Now()
	if err := c.Refresh(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("QA-PERF-06: Refresh took %s, want <= 1s", elapsed)
	}
}

// TestTopoOrderCommitGraphCost records (never fails) the app's exact log
// call on the reference repo, with and without a commit-graph file, via
// internal/git.Runner. It measures; it has no budget of its own.
func TestTopoOrderCommitGraphCost(t *testing.T) {
	if err := mkrepo.Build(t.Context(), refDir, mkrepo.Params); err != nil {
		t.Fatal(err)
	}
	r := git.Runner{}
	graphFile := filepath.Join(refDir, ".git", "objects", "info", "commit-graph")
	_ = os.Remove(graphFile) // this test owns the "without" measurement; a prior test may have already written one

	t.Logf("git log --topo-order --all: %s without a commit-graph file", timeTopoOrderLog(t, r))

	if _, err := r.Run(t.Context(), git.Call{Dir: refDir, Args: []string{"commit-graph", "write", "--reachable"}}); err != nil {
		t.Fatal(err)
	}
	t.Logf("git log --topo-order --all: %s with a commit-graph file", timeTopoOrderLog(t, r))
}

func timeTopoOrderLog(t *testing.T, r git.Runner) time.Duration {
	t.Helper()
	start := time.Now()
	_, err := r.Run(t.Context(), git.Call{
		Dir: refDir, ReadOnly: true, Timeout: 2 * time.Minute,
		Args: services.LogArgs(core.LogFilter{}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return time.Since(start)
}

// TestLogRowsWindowLatency records one LogRows(from, from+50) call's
// latency on the fully-loaded reference repo — the Go-side cost a single
// virtual-scroll tick pays. Not QA-PERF-03 itself (real frame cost includes
// JS/DOM/IPC this measures none of) — a bound on one input to it.
func TestLogRowsWindowLatency(t *testing.T) {
	c, id := openReference(t)
	if err := c.LoadLog(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	waitLogDone(t, c, id)

	start := time.Now()
	if _, err := c.LogRows(t.Context(), id, 50_000, 50_050); err != nil {
		t.Fatal(err)
	}
	t.Logf("LogRows(50000, 50050) latency: %s", time.Since(start))
}

// waitLogDone polls LogRows until the load finishes.
func waitLogDone(t *testing.T, c *services.Core, id core.WorktreeID) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		res, err := c.LogRows(t.Context(), id, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if res.Error != nil {
			t.Fatal(res.Error)
		}
		if res.Done {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("log did not finish loading in time")
}

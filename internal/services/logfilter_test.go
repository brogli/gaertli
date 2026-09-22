package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestFR_LOG_05_BuildFilterArgsQuery(t *testing.T) {
	args := LogArgs(core.LogFilter{Query: "bug"})

	for _, want := range []string{"--grep=bug", "--regexp-ignore-case", "--fixed-strings", "--exclude=refs/stash", "--all"} {
		if !slices.Contains(args, want) {
			t.Errorf("args = %v, want to contain %q", args, want)
		}
	}
	i := slices.Index(args, "--end-of-options")
	if i == -1 {
		t.Fatalf("args = %v, want --end-of-options", args)
	}
	if i != len(args)-1 {
		t.Errorf("args = %v, want nothing after --end-of-options", args)
	}
}

func TestFR_LOG_06_BuildFilterArgsBranches(t *testing.T) {
	args := LogArgs(core.LogFilter{Branches: []string{"main", "feature"}})

	if slices.Contains(args, "--exclude=refs/stash") || slices.Contains(args, "--all") {
		t.Errorf("args = %v, want no --exclude=refs/stash/--all when branches are given", args)
	}
	i := slices.Index(args, "--end-of-options")
	if i == -1 || len(args) < i+3 || args[i+1] != "main" || args[i+2] != "feature" {
		t.Errorf("args = %v, want main, feature immediately after --end-of-options, in order", args)
	}
}

func TestFR_LOG_07_BuildFilterArgsPath(t *testing.T) {
	args := LogArgs(core.LogFilter{Path: "sub/dir"})

	if len(args) < 2 || args[len(args)-2] != "--" || args[len(args)-1] != "sub/dir" {
		t.Errorf("args = %v, want to end with --, sub/dir", args)
	}
}

func TestFR_LOG_08_BuildFilterArgsCombined(t *testing.T) {
	args := LogArgs(core.LogFilter{Query: "q", Author: "a", Branches: []string{"main"}, Path: "p"})

	for _, want := range []string{"--grep=q", "--author=a", "--regexp-ignore-case", "--fixed-strings"} {
		if !slices.Contains(args, want) {
			t.Errorf("args = %v, want to contain %q", args, want)
		}
	}
	if slices.Contains(args, "--all") {
		t.Errorf("args = %v, want no --all when branches are given", args)
	}

	order := []string{"--end-of-options", "main", "--", "p"}
	last := -1
	for _, tok := range order {
		i := slices.Index(args, tok)
		if i == -1 || i < last {
			t.Fatalf("args = %v, want %v in that relative order", args, order)
		}
		last = i
	}
}

func TestFR_LOG_05_HashPatternMatchesHexOnly(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"abcd", true},
		{"a", false},
		{"ABCD", false},
		{"g123", false},
		{"0123456789abcdef0123456789abcdef012345678", false}, // 41 chars, too long
		{"0123456789abcdef0123456789abcdef01234567", true},   // 40 chars, a full SHA
	}
	for _, tt := range tests {
		if got := hashPattern.MatchString(tt.s); got != tt.want {
			t.Errorf("hashPattern.MatchString(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func TestFR_LOG_05_SetLogFilterByTextNarrowsAndIsFlat(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first commit")
	gittest.Commit(t, repo.Dir, "b.txt", "b", "urgent fix")
	gittest.Commit(t, repo.Dir, "c.txt", "c", "third commit")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetLogFilter(context.Background(), wt.ID, core.LogFilter{Query: "urgent"}); err != nil {
		t.Fatal(err)
	}
	res := waitForFilterRows(t, c, wt.ID)

	if len(res.Rows) != 1 || res.Rows[0].Subject != "urgent fix" {
		t.Fatalf("Rows = %+v, want exactly the matching commit", res.Rows)
	}
	if !res.Flat {
		t.Error("Flat = false, want true for a text query")
	}
}

func TestFR_LOG_05_SetLogFilterCombinesQueryAndAuthorWithAND(t *testing.T) {
	repo := gittest.New(t)
	commitAs(t, repo.Dir, "a.txt", "a", "alpha urgent", "Alice", "alice@example.com")
	commitAs(t, repo.Dir, "b.txt", "b", "beta calm", "Bob", "bob@example.com")
	commitAs(t, repo.Dir, "c.txt", "c", "gamma urgent", "Bob", "bob@example.com") // matches Query, wrong Author

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	if err := c.SetLogFilter(context.Background(), wt.ID, core.LogFilter{Query: "urgent", Author: "Alice"}); err != nil {
		t.Fatal(err)
	}
	res := waitForFilterRows(t, c, wt.ID)
	if len(res.Rows) != 1 || res.Rows[0].Subject != "alpha urgent" {
		t.Fatalf("Rows = %+v, want exactly Alice's matching commit (AND, not OR)", res.Rows)
	}

	// Control: Author alone returns every one of Bob's commits regardless
	// of message, proving the narrowing above came from the AND, not from
	// Author alone happening to match one commit.
	if err := c.SetLogFilter(context.Background(), wt.ID, core.LogFilter{Author: "Bob"}); err != nil {
		t.Fatal(err)
	}
	res = waitForFilterRows(t, c, wt.ID)
	if len(res.Rows) != 2 {
		t.Fatalf("Author-only Rows = %+v, want Bob's 2 commits regardless of message", res.Rows)
	}
}

func TestFR_LOG_05_SetLogFilterByHashResolvesAndPopulatesCheckedOutIn(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "base")
	gittest.Commit(t, repo.Dir, "seed.txt", "y", "second")
	sha := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "branch", "feature")
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", linked, "feature")
	gittest.Run(t, repo.Dir, "switch", "--detach", "HEAD~1") // move the main worktree off "second": linked is now the only one there

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	if err := c.SetLogFilter(context.Background(), wt.ID, core.LogFilter{Query: sha[:4]}); err != nil {
		t.Fatal(err)
	}
	res := waitForFilterRows(t, c, wt.ID)
	if len(res.Rows) != 1 || res.Rows[0].SHA != sha {
		t.Fatalf("Rows = %+v, want exactly sha %s", res.Rows, sha)
	}
	if got := res.Rows[0].CheckedOutIn; len(got) != 1 || got[0] != linked {
		t.Errorf("CheckedOutIn = %v, want [%s]", got, linked)
	}

	// A hex-shaped but non-existent query falls through to an ordinary
	// (zero-result) --grep search, not an error.
	if err := c.SetLogFilter(context.Background(), wt.ID, core.LogFilter{Query: "deadbeef"}); err != nil {
		t.Fatal(err)
	}
	res = waitForFilterRows(t, c, wt.ID)
	if res.Error != nil {
		t.Fatalf("Error = %v, want nil (zero-result search, not an error)", res.Error)
	}
	if len(res.Rows) != 0 {
		t.Errorf("Rows = %+v, want zero results", res.Rows)
	}
}

func TestFR_LOG_06_SetLogFilterByBranchScopesToItsOwnHistory(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "root.txt", "r", "root")
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Commit(t, repo.Dir, "main.txt", "m", "on main")
	gittest.Run(t, repo.Dir, "switch", "feature")
	gittest.Commit(t, repo.Dir, "feature.txt", "f", "on feature")
	gittest.Run(t, repo.Dir, "switch", "master")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	if err := c.SetLogFilter(context.Background(), wt.ID, core.LogFilter{Branches: []string{"feature"}}); err != nil {
		t.Fatal(err)
	}
	res := waitForFilterRows(t, c, wt.ID)

	if res.Flat {
		t.Error("Flat = true, want false for a branch-only filter")
	}
	subjects := map[string]bool{}
	for _, r := range res.Rows {
		subjects[r.Subject] = true
		if len(r.Graph.Edges) == 0 {
			t.Errorf("row %+v has no Graph edges, want a real lane", r)
		}
	}
	if len(res.Rows) != 2 || !subjects["root"] || !subjects["on feature"] {
		t.Fatalf("Rows = %+v, want exactly feature's own history (root, on feature)", res.Rows)
	}
}

func TestFR_LOG_07_SetLogFilterByPathRewritesParentsAcrossSkippedCommits(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "path.txt", "1", "A touches path")
	gittest.Commit(t, repo.Dir, "other.txt", "1", "B touches other")
	gittest.Commit(t, repo.Dir, "path.txt", "2", "C touches path")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	if err := c.SetLogFilter(context.Background(), wt.ID, core.LogFilter{Path: "path.txt"}); err != nil {
		t.Fatal(err)
	}
	res := waitForFilterRows(t, c, wt.ID)

	if len(res.Rows) != 2 {
		t.Fatalf("Rows = %+v, want exactly A and C (not B)", res.Rows)
	}
	cRow, aRow := res.Rows[0], res.Rows[1] // newest first
	if cRow.Subject != "C touches path" || aRow.Subject != "A touches path" {
		t.Fatalf("Rows = %+v, want C then A, B excluded entirely", res.Rows)
	}
	if len(cRow.Graph.Edges) == 0 || len(aRow.Graph.Edges) == 0 {
		t.Fatalf("Graph not populated: C=%+v A=%+v", cRow.Graph, aRow.Graph)
	}
	if cRow.Graph.Lane != aRow.Graph.Lane {
		t.Errorf("C.Graph.Lane=%d A.Graph.Lane=%d, want the same lane continuing straight through (no broken/orphan lane)", cRow.Graph.Lane, aRow.Graph.Lane)
	}
}

func TestFR_LOG_08_SetLogFilterCombinesAuthorBranchAndPathWithAND(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "root.txt", "r", "root") // shared ancestor
	gittest.Run(t, repo.Dir, "branch", "feature")
	commitAs(t, repo.Dir, "target.txt", "1", "on main by alice", "Alice", "alice@example.com") // wrong branch
	gittest.Run(t, repo.Dir, "switch", "feature")
	commitAs(t, repo.Dir, "other.txt", "1", "on feature by alice wrong path", "Alice", "alice@example.com")
	commitAs(t, repo.Dir, "target.txt", "2", "on feature by bob wrong author", "Bob", "bob@example.com")
	commitAs(t, repo.Dir, "target.txt", "3", "on feature by alice matches all", "Alice", "alice@example.com")
	gittest.Run(t, repo.Dir, "switch", "master")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	filter := core.LogFilter{Author: "Alice", Branches: []string{"feature"}, Path: "target.txt"}
	if err := c.SetLogFilter(context.Background(), wt.ID, filter); err != nil {
		t.Fatal(err)
	}
	res := waitForFilterRows(t, c, wt.ID)

	if len(res.Rows) != 1 || res.Rows[0].Subject != "on feature by alice matches all" {
		t.Fatalf("Rows = %+v, want exactly the one commit matching Author, Branches, and Path together", res.Rows)
	}
}

func TestFR_LOG_08_SetLogFilterEmptyClearsInstantlyWithNoNewGitCall(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first")
	gittest.Commit(t, repo.Dir, "b.txt", "b", "second")

	c := New(func(core.Event) {})
	info, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadLog(context.Background(), info.ID); err != nil {
		t.Fatal(err)
	}
	waitForLog(t, c, info.ID)

	if err := c.SetLogFilter(context.Background(), info.ID, core.LogFilter{Query: "second"}); err != nil {
		t.Fatal(err)
	}
	waitForFilterRows(t, c, info.ID)

	beforeLog := countCommands(t, c, info.ID, "git log ")
	beforeRevParse := countCommands(t, c, info.ID, "git rev-parse")

	if err := c.SetLogFilter(context.Background(), info.ID, core.LogFilter{}); err != nil {
		t.Fatal(err)
	}

	res, err := c.LogRows(context.Background(), info.ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("Rows = %+v, want the unfiltered cache's 2 rows immediately", res.Rows)
	}
	if got := countCommands(t, c, info.ID, "git log "); got != beforeLog {
		t.Errorf("git log calls = %d, want unchanged from %d (clearing is instant, no new process)", got, beforeLog)
	}
	if got := countCommands(t, c, info.ID, "git rev-parse"); got != beforeRevParse {
		t.Errorf("git rev-parse calls = %d, want unchanged from %d", got, beforeRevParse)
	}
}

func TestFR_LOG_08_SetLogFilterSupersedesARunningFilteredLoad(t *testing.T) {
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

	file1 := filepath.Join(t.TempDir(), "records1")
	if err := os.WriteFile(file1, buildLogFixture(5), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKEGIT_STREAM_FILE", file1)
	t.Setenv("FAKEGIT_STREAM_DELAY_MS", "300")
	wt.Runner.Git = fakegit

	if err := c.SetLogFilter(context.Background(), info.ID, core.LogFilter{Branches: []string{"whatever"}}); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		res, err := c.LogRows(context.Background(), info.ID, 0, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Rows) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Swap in a different, faster fixture before the first load can finish,
	// and start a second filter.
	file2 := filepath.Join(t.TempDir(), "records2")
	if err := os.WriteFile(file2, buildLogFixture(2), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKEGIT_STREAM_FILE", file2)
	t.Setenv("FAKEGIT_STREAM_DELAY_MS", "0")

	if err := c.SetLogFilter(context.Background(), info.ID, core.LogFilter{Branches: []string{"other"}}); err != nil {
		t.Fatal(err)
	}
	res := waitForFilterRows(t, c, info.ID)

	if len(res.Rows) != 2 {
		t.Fatalf("Rows = %d, want exactly the 2nd load's 2 records; none of the 1st load's records leaked in", len(res.Rows))
	}
}

// TestQA_PERF_04_FirstFilteredRowObservableWellUnderFullStreamDuration
// proves the streaming mechanism QA-PERF-04 depends on: a filtered row is
// usable long before the whole (delayed) stream finishes, here for
// Core.LogRows serving a window's own active filtered view. The literal
// ≤2s-at-100k-commits number is tools/bench's own case.
func TestQA_PERF_04_FirstFilteredRowObservableWellUnderFullStreamDuration(t *testing.T) {
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
	if err := c.SetLogFilter(context.Background(), info.ID, core.LogFilter{Branches: []string{"whatever"}}); err != nil {
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
		t.Fatal("no filtered row observed within 1s")
	}
	if elapsed > time.Second {
		t.Errorf("first filtered row observable after %s, want well under 1s (the full 2.5s unstreamed load)", elapsed)
	}

	waitForFilterRows(t, c, info.ID) // drain the background stream so it doesn't leak noise into later tests
}

// waitForFilterRows polls LogRows until a filtered load finishes.
func waitForFilterRows(t *testing.T, c *Core, id core.WorktreeID) core.LogRowsResult {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		res, err := c.LogRows(context.Background(), id, 0, 1000)
		if err != nil {
			t.Fatal(err)
		}
		if res.Done {
			return res
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("filtered log did not finish loading in time")
	return core.LogRowsResult{}
}

// commitAs commits content to file with an explicit author (unlike commit,
// which always uses gittest.New's fixed default identity).
func commitAs(t *testing.T, dir, file, content, msg, authorName, authorEmail string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, dir, "add", "--", file)
	author := fmt.Sprintf("%s <%s>", authorName, authorEmail)
	if _, err := (git.Runner{}).Run(context.Background(), git.Call{
		Dir: dir, Args: []string{"commit", "--author=" + author, "-F", "-"}, Stdin: []byte(msg),
	}); err != nil {
		t.Fatal(err)
	}
}

package services

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestFR_LOG_05_LogArgsHasNoGrepFlatOnlyWithAuthor(t *testing.T) {
	t.Parallel()
	args := LogArgs(core.LogFilter{Author: "a", Branches: []string{"main"}, Path: "p"})
	if slices.ContainsFunc(args, func(a string) bool { return strings.HasPrefix(a, "--grep") }) {
		t.Errorf("args = %v, want no --grep: LogArgs does not search", args)
	}

	if !(core.LogFilter{Author: "a"}).Flat() {
		t.Error("Flat() = false for an Author filter, want true")
	}
	if (core.LogFilter{Branches: []string{"main"}}).Flat() || (core.LogFilter{Path: "p"}).Flat() {
		t.Error("Flat() = true for Branches or Path alone, want false")
	}
}

func TestFR_LOG_05_HashPatternMatchesHexOnly(t *testing.T) {
	t.Parallel()
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

func TestFR_LOG_05_SearchLogFindsMatchingMessagesCaseInsensitiveInLogOrder(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first commit")
	gittest.Commit(t, repo.Dir, "b.txt", "b", "urgent fix")
	gittest.Commit(t, repo.Dir, "c.txt", "c", "another Fix here")
	gittest.Commit(t, repo.Dir, "d.txt", "d", "unrelated")
	gittest.Commit(t, repo.Dir, "e.txt", "e", "mentions deadbeef in passing")

	c, wt := openWorktree(t, repo.Dir)
	if err := c.LoadLog(t.Context(), wt); err != nil {
		t.Fatal(err)
	}
	loaded := waitForLogRows(t, c, wt)

	res, err := c.SearchLog(t.Context(), wt, "fix")
	if err != nil {
		t.Fatal(err)
	}
	if res.Generation != loaded.Generation {
		t.Errorf("Generation = %d, want the loaded view's %d", res.Generation, loaded.Generation)
	}
	if res.Filtered {
		t.Error("Filtered = true, want false: no filter active, this is the base log")
	}

	var subjects []string
	for _, i := range res.Indices {
		subjects = append(subjects, loaded.Rows[i].Subject)
	}
	if !slices.Equal(subjects, []string{"another Fix here", "urgent fix"}) { // newest first, log order
		t.Errorf("matched subjects = %v, want the two case-insensitive matches in log order", subjects)
	}

	hex, err := c.SearchLog(t.Context(), wt, "deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	if len(hex.Indices) != 1 || loaded.Rows[hex.Indices[0]].Subject != "mentions deadbeef in passing" {
		t.Fatalf("Indices = %v, want the one matching commit (a hex-shaped but nonexistent hash is not an error)", hex.Indices)
	}
}

func TestFR_LOG_05_SearchLogCombinesWithTheActiveAuthorFilter(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	commitAs(t, repo.Dir, "a.txt", "a", "ann's change", "Ann", "ann@example.com")
	commitAs(t, repo.Dir, "b.txt", "b", "bob's change", "Bob", "bob@example.com")
	commitAs(t, repo.Dir, "c.txt", "c", "ann again, unrelated", "Ann", "ann@example.com")

	c, wt := openWorktree(t, repo.Dir)
	if err := c.SetLogFilter(t.Context(), wt, core.LogFilter{Author: "Ann"}); err != nil {
		t.Fatal(err)
	}
	res := waitForLogRows(t, c, wt)
	if len(res.Rows) != 2 {
		t.Fatalf("filtered Rows = %+v, want exactly Ann's 2 commits", res.Rows)
	}

	search, err := c.SearchLog(t.Context(), wt, "change")
	if err != nil {
		t.Fatal(err)
	}
	if len(search.Indices) != 1 || res.Rows[search.Indices[0]].Subject != "ann's change" {
		t.Fatalf("Indices = %v (rows %+v), want exactly Ann's matching commit, Bob's excluded by the active filter", search.Indices, res.Rows)
	}
	if !search.Filtered {
		t.Error("Filtered = false, want true: the Author filter is active")
	}
}

func TestFR_LOG_05_SearchLogByHashInAFilteredViewFindsOnlyCommitsInsideIt(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	_, tagSHA := branchAndTagNamedV1(t, repo.Dir)
	outsideSHA := gittest.HeadSHA(t, repo.Dir) // "third", past v1's history

	c, wt := openWorktree(t, repo.Dir)
	if err := c.SetLogFilter(t.Context(), wt, core.LogFilter{Branches: []string{"refs/tags/v1"}}); err != nil {
		t.Fatal(err)
	}
	res := waitForLogRows(t, c, wt)

	outside, err := c.SearchLog(t.Context(), wt, outsideSHA)
	if err != nil {
		t.Fatal(err)
	}
	if len(outside.Indices) != 0 {
		t.Errorf("Indices = %v, want none: %s is outside v1's history", outside.Indices, outsideSHA)
	}
	if !outside.Filtered {
		t.Error("Filtered = false, want true: the tag filter is active")
	}

	inside, err := c.SearchLog(t.Context(), wt, tagSHA)
	if err != nil {
		t.Fatal(err)
	}
	if len(inside.Indices) != 1 || res.Rows[inside.Indices[0]].SHA != tagSHA {
		t.Fatalf("Indices = %v, want exactly tagSHA %s's row in the filtered view", inside.Indices, tagSHA)
	}
	if !inside.Filtered {
		t.Error("Filtered = false, want true: the tag filter is active")
	}
}

func TestFR_LOG_06_SetLogFilterByBranchScopesToItsOwnHistory(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "root.txt", "r", "root")
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Commit(t, repo.Dir, "main.txt", "m", "on main")
	gittest.Run(t, repo.Dir, "tag", "feature") // named like the branch, on another commit
	gittest.Run(t, repo.Dir, "switch", "feature")
	gittest.Commit(t, repo.Dir, "feature.txt", "f", "on feature")
	gittest.Run(t, repo.Dir, "switch", "master")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.SetLogFilter(t.Context(), wt, core.LogFilter{Branches: []string{"refs/heads/feature"}}); err != nil {
		t.Fatal(err)
	}
	res := waitForLogRows(t, c, wt)

	if len(res.Rows) == 0 || len(res.Rows[0].Graph.Edges) == 0 {
		t.Error("no graph row, want one for a branch-only filter")
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

func TestFR_LOG_06_SetLogFilterByTagShowsTheHistoryLeadingToIt(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	branchAndTagNamedV1(t, repo.Dir) // v1: a branch at "first", an annotated tag at "second"

	c, wt := openWorktree(t, repo.Dir)

	if err := c.SetLogFilter(t.Context(), wt, core.LogFilter{Branches: []string{"refs/tags/v1"}}); err != nil {
		t.Fatal(err)
	}
	res := waitForLogRows(t, c, wt)

	var subjects []string
	for _, r := range res.Rows {
		subjects = append(subjects, r.Subject)
	}
	if !slices.Equal(subjects, []string{"second", "first"}) {
		t.Errorf("subjects = %q, want the tag's history [second first]", subjects)
	}
}

func TestFR_LOG_07_SetLogFilterByPathRewritesParentsAcrossSkippedCommits(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "path.txt", "1", "A touches path")
	gittest.Commit(t, repo.Dir, "other.txt", "1", "B touches other")
	gittest.Commit(t, repo.Dir, "path.txt", "2", "C touches path")

	c, wt := openWorktree(t, repo.Dir)

	if err := c.SetLogFilter(t.Context(), wt, core.LogFilter{Path: "path.txt"}); err != nil {
		t.Fatal(err)
	}
	res := waitForLogRows(t, c, wt)

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
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "root.txt", "r", "root") // shared ancestor
	gittest.Run(t, repo.Dir, "branch", "feature")
	commitAs(t, repo.Dir, "target.txt", "1", "on main by alice", "Alice", "alice@example.com") // wrong branch
	gittest.Run(t, repo.Dir, "switch", "feature")
	commitAs(t, repo.Dir, "other.txt", "1", "on feature by alice wrong path", "Alice", "alice@example.com")
	commitAs(t, repo.Dir, "target.txt", "2", "on feature by bob wrong author", "Bob", "bob@example.com")
	commitAs(t, repo.Dir, "target.txt", "3", "on feature by alice matches all", "Alice", "alice@example.com")
	gittest.Run(t, repo.Dir, "switch", "master")

	c, wt := openWorktree(t, repo.Dir)

	filter := core.LogFilter{Author: "Alice", Branches: []string{"refs/heads/feature"}, Path: "target.txt"}
	if err := c.SetLogFilter(t.Context(), wt, filter); err != nil {
		t.Fatal(err)
	}
	res := waitForLogRows(t, c, wt)

	if len(res.Rows) != 1 || res.Rows[0].Subject != "on feature by alice matches all" {
		t.Fatalf("Rows = %+v, want exactly the one commit matching Author, Branches, and Path together", res.Rows)
	}
}

func TestFR_LOG_08_SetLogFilterEmptyClearsInstantlyWithNoNewGitCall(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first")
	gittest.Commit(t, repo.Dir, "b.txt", "b", "second")

	c, id := openWorktree(t, repo.Dir)
	rec := recordCommands(t, c, id)
	loadLog(t, c, id)

	if err := c.SetLogFilter(t.Context(), id, core.LogFilter{Path: "b.txt"}); err != nil {
		t.Fatal(err)
	}
	waitForLogRows(t, c, id)

	beforeLog := countCommands(rec, "git log ")
	beforeRevParse := countCommands(rec, "git rev-parse")

	if err := c.SetLogFilter(t.Context(), id, core.LogFilter{}); err != nil {
		t.Fatal(err)
	}

	res, err := c.LogRows(t.Context(), id, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("Rows = %+v, want the unfiltered cache's 2 rows immediately", res.Rows)
	}
	if got := countCommands(rec, "git log "); got != beforeLog {
		t.Errorf("git log calls = %d, want unchanged from %d (clearing is instant, no new process)", got, beforeLog)
	}
	if got := countCommands(rec, "git rev-parse"); got != beforeRevParse {
		t.Errorf("git rev-parse calls = %d, want unchanged from %d", got, beforeRevParse)
	}
}

func TestFR_LOG_08_SetLogFilterSupersedesARunningFilteredLoad(t *testing.T) {
	repo := gittest.New(t)
	c, id := openWorktree(t, repo.Dir)
	streamFakeLog(t, c, id, 5, 100)

	if err := c.SetLogFilter(t.Context(), id, core.LogFilter{Branches: []string{"whatever"}}); err != nil {
		t.Fatal(err)
	}

	waitFor(t, func() bool {
		res, err := c.LogRows(t.Context(), id, 0, 1)
		if err != nil {
			t.Fatal(err)
		}
		return len(res.Rows) > 0
	})

	// Swap in a different, faster fixture before the first load can finish,
	// and start a second filter.
	streamFakeLog(t, c, id, 2, 0)

	if err := c.SetLogFilter(t.Context(), id, core.LogFilter{Branches: []string{"other"}}); err != nil {
		t.Fatal(err)
	}
	res := waitForLogRows(t, c, id)

	if len(res.Rows) != 2 {
		t.Fatalf("Rows = %d, want exactly the 2nd load's 2 records; none of the 1st load's records leaked in", len(res.Rows))
	}
}

// The ≤2s-at-100k-commits target is tools/bench's case.
func TestQA_PERF_04_FirstFilteredRowObservableWellUnderFullStreamDuration(t *testing.T) {
	repo := gittest.New(t)
	c, id := openWorktree(t, repo.Dir)
	streamFakeLog(t, c, id, 5, 200) // 5 records * 200ms = 1s unstreamed

	start := time.Now()
	if err := c.SetLogFilter(t.Context(), id, core.LogFilter{Branches: []string{"whatever"}}); err != nil {
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
		t.Fatal("no filtered row observed within 500ms")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("first filtered row observable after %s, want well under 500ms (the full 1s unstreamed load)", elapsed)
	}

	waitForLogRows(t, c, id) // drain the background stream so it doesn't leak noise into later tests
}

// commitAs commits content to file with an explicit author.
func commitAs(t *testing.T, dir, file, content, msg, authorName, authorEmail string) {
	t.Helper()
	writeFile(t, dir, file, content)
	gittest.Run(t, dir, "add", "--", file)
	gittest.Run(t, dir, "commit", fmt.Sprintf("--author=%s <%s>", authorName, authorEmail), "-m", msg)
}

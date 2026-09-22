package services

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
	"github.com/brogli/nebegit/internal/testbin"
)

// fakegit is internal/git/testdata/fakegit, for the tests that inject a
// delayed stream.
var fakegit string

// argvWriterBin is internal/editor/testdata/argvwriter, used by
// editor_test.go: a stand-in editor that writes its received argv
// to a file next to itself and exits. Built here, the package's one
// TestMain, alongside fakegit.
var argvWriterBin string

func TestMain(m *testing.M) {
	testbin.Main(m,
		testbin.Bin{Path: &fakegit, Pkg: "../git/testdata/fakegit"},
		testbin.Bin{Path: &argvWriterBin, Pkg: "../editor/testdata/argvwriter"},
	)
}

func TestFR_LOG_01_ParseLogRecordParsesFieldsAndRefs(t *testing.T) {
	tests := []struct {
		name   string
		record string
		want   []core.RefLabel
	}{
		{
			name:   "HEAD and a tag",
			record: "1111111111111111111111111111111111111111\x1f1111111\x1f\x1fAlice\x1f1700000000\x1fHEAD -> refs/heads/main, tag: refs/tags/v1.0\x1fInitial commit",
			want:   []core.RefLabel{{Name: "HEAD", Kind: "head"}, {Name: "main", Kind: "branch"}, {Name: "v1.0", Kind: "tag"}},
		},
		{
			name:   "a remote-tracking branch",
			record: "2222222222222222222222222222222222222222\x1f2222222\x1f1111111111111111111111111111111111111111\x1fBob\x1f1700000100\x1frefs/remotes/origin/feature\x1fFeature work",
			want:   []core.RefLabel{{Name: "origin/feature", Kind: "remote"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commit, _, err := parseLogRecord([]byte(tt.record))
			if err != nil {
				t.Fatal(err)
			}
			if len(commit.Refs) != len(tt.want) {
				t.Fatalf("Refs = %+v, want %+v", commit.Refs, tt.want)
			}
			for i, ref := range commit.Refs {
				if ref != tt.want[i] {
					t.Errorf("Refs[%d] = %+v, want %+v", i, ref, tt.want[i])
				}
			}
		})
	}

	commit, parents, err := parseLogRecord([]byte(tests[1].record))
	if err != nil {
		t.Fatal(err)
	}
	if commit.SHA != "2222222222222222222222222222222222222222" || commit.ShortSHA != "2222222" {
		t.Errorf("SHA/ShortSHA = %q/%q, want the full/short hash", commit.SHA, commit.ShortSHA)
	}
	if len(parents) != 1 || parents[0] != "1111111111111111111111111111111111111111" {
		t.Errorf("parents = %v, want one parent", parents)
	}
	if commit.Author != "Bob" || commit.AuthorAt != 1700000100 || commit.Subject != "Feature work" {
		t.Errorf("commit = %+v, want Author Bob, AuthorAt 1700000100, Subject %q", commit, "Feature work")
	}
}

func TestFR_LOG_01_LoadLogIncludesAllBranchesExcludesStash(t *testing.T) {
	repo := gittest.New(t)
	commitOrphan(t, repo.Dir, "main", "a.txt", "a")
	commitOrphan(t, repo.Dir, "feature", "b.txt", "b")
	commitOrphan(t, repo.Dir, "other", "c.txt", "c")
	if err := os.WriteFile(filepath.Join(repo.Dir, "c.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, repo.Dir, "stash")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadLog(context.Background(), wt.ID); err != nil {
		t.Fatal(err)
	}
	rows := waitForLog(t, c, wt.ID)

	subjects := map[string]bool{}
	for _, r := range rows {
		subjects[r.Subject] = true
	}
	for _, want := range []string{"a", "b", "c"} {
		if !subjects[want] {
			t.Errorf("missing commit %q; got %+v", want, rows)
		}
	}
	if len(rows) != 3 {
		t.Errorf("rows = %+v, want exactly the 3 branch commits, no stash entries", rows)
	}
}

func TestFR_LOG_09_LogRowsWindowAheadOfLoadReturnsFewerRowsNotError(t *testing.T) {
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
	t.Setenv("FAKEGIT_STREAM_DELAY_MS", "500")
	wt.Runner.Git = fakegit

	if err := c.LoadLog(context.Background(), info.ID); err != nil {
		t.Fatal(err)
	}

	var res core.LogRowsResult
	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		res, err = c.LogRows(context.Background(), info.ID, 0, 1)
		if err != nil {
			t.Fatal(err)
		}
		if res.Loaded > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if res.Done {
		t.Fatal("Done became true before a partial-load observation; the test's timing assumption is off")
	}
	if res.Error != nil {
		t.Fatalf("Error = %v, want nil", res.Error)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("Rows = %d, want exactly 1", len(res.Rows))
	}

	waitForLog(t, c, info.ID) // drain the background stream so it doesn't leak noise into later tests
}

func TestADR_5_LogRowsGraphIsPopulatedFromLoad(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "base")
	gittest.Run(t, repo.Dir, "branch", "side")
	gittest.Commit(t, repo.Dir, "a.txt", "b", "on main")
	gittest.Run(t, repo.Dir, "switch", "side")
	gittest.Commit(t, repo.Dir, "b.txt", "c", "on side")
	gittest.Run(t, repo.Dir, "switch", "master")
	_, _ = (git.Runner{}).Run(context.Background(), git.Call{Dir: repo.Dir, Args: []string{"merge", "--no-ff", "-m", "merge side", "side"}})

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadLog(context.Background(), wt.ID); err != nil {
		t.Fatal(err)
	}
	rows := waitForLog(t, c, wt.ID)

	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4 (base, on main, on side, merge side)", len(rows))
	}
	merge := rows[0]
	if merge.Subject != "merge side" {
		t.Fatalf("rows[0] = %+v, want the merge commit first (newest)", merge)
	}
	starts := 0
	for _, e := range merge.Graph.Edges {
		if e.Kind == "start" {
			starts++
		}
	}
	if starts == 0 {
		t.Errorf("merge row Graph = %+v, want at least one start edge for its 2nd parent", merge.Graph)
	}
}

func TestFR_LOG_10_LoadLogMarksCheckedOutWorktrees(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "base")
	older := gittest.HeadSHA(t, repo.Dir)
	gittest.Commit(t, repo.Dir, "seed.txt", "y", "second")
	gittest.Run(t, repo.Dir, "branch", "feature")
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", linked, "feature")
	gittest.Run(t, repo.Dir, "switch", "--detach", older)

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadLog(context.Background(), wt.ID); err != nil {
		t.Fatal(err)
	}
	rows := waitForLog(t, c, wt.ID)

	byMsg := map[string]core.LogCommit{}
	for _, r := range rows {
		byMsg[r.Subject] = r
	}
	if got := byMsg["second"].CheckedOutIn; len(got) != 1 || got[0] != linked {
		t.Errorf("second commit CheckedOutIn = %v, want [%s]", got, linked)
	}
	if got := byMsg["base"].CheckedOutIn; len(got) != 1 || got[0] != repo.Dir {
		t.Errorf("base commit CheckedOutIn = %v, want [%s]", got, repo.Dir)
	}
}

func TestFR_REPO_04_ReloadLogNoOpsWhenTipsUnchanged(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")

	c := New(func(core.Event) {})
	info, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadLog(context.Background(), info.ID); err != nil {
		t.Fatal(err)
	}
	waitForLog(t, c, info.ID)
	beforeCalls := countCommands(t, c, info.ID, "git log ")
	beforeGen := logGeneration(t, c, info.ID)

	if err := c.ReloadLog(context.Background(), info.ID); err != nil {
		t.Fatal(err)
	}

	if got := countCommands(t, c, info.ID, "git log "); got != beforeCalls {
		t.Errorf("git log calls = %d, want unchanged from %d", got, beforeCalls)
	}
	if got := logGeneration(t, c, info.ID); got != beforeGen {
		t.Errorf("Generation = %d, want unchanged (%d)", got, beforeGen)
	}
}

func TestFR_REPO_04_ReloadLogRestartsWhenARefMoved(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")

	c := New(func(core.Event) {})
	info, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadLog(context.Background(), info.ID); err != nil {
		t.Fatal(err)
	}
	waitForLog(t, c, info.ID)
	beforeGen := logGeneration(t, c, info.ID)

	gittest.Run(t, repo.Dir, "branch", "feature")

	if err := c.ReloadLog(context.Background(), info.ID); err != nil {
		t.Fatal(err)
	}
	rows := waitForLog(t, c, info.ID)

	if got := logGeneration(t, c, info.ID); got == beforeGen {
		t.Errorf("Generation = %d, want incremented from %d", got, beforeGen)
	}
	if !hasRefLabel(rows, "feature") {
		t.Errorf("rows after reload = %+v, want the new branch label", rows)
	}
}

func TestFR_REPO_04_ReloadLogRestartsWhenOnlyHEADsTargetChanged(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	gittest.Run(t, repo.Dir, "branch", "feature")

	c := New(func(core.Event) {})
	info, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadLog(context.Background(), info.ID); err != nil {
		t.Fatal(err)
	}
	waitForLog(t, c, info.ID)
	beforeGen := logGeneration(t, c, info.ID)

	gittest.Run(t, repo.Dir, "switch", "feature") // no new commit: only HEAD's target ref changes

	if err := c.ReloadLog(context.Background(), info.ID); err != nil {
		t.Fatal(err)
	}
	waitForLog(t, c, info.ID)

	if got := logGeneration(t, c, info.ID); got == beforeGen {
		t.Errorf("Generation = %d, want incremented from %d", got, beforeGen)
	}
}

func TestFR_LOG_01_LogIndexOfFindsRowBySHA(t *testing.T) {
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "a.txt", "a", "first")
	gittest.Commit(t, repo.Dir, "b.txt", "b", "second")
	gittest.Commit(t, repo.Dir, "c.txt", "c", "third")

	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadLog(context.Background(), wt.ID); err != nil {
		t.Fatal(err)
	}
	rows := waitForLog(t, c, wt.ID)

	byMsg := map[string]core.LogCommit{}
	for _, r := range rows {
		byMsg[r.Subject] = r
	}

	got, err := c.LogIndexOf(context.Background(), wt.ID, byMsg["second"].SHA)
	if err != nil {
		t.Fatal(err)
	}
	if got.Index != 1 || !got.Found {
		t.Errorf("LogIndexOf(second) = %+v, want {Index:1, Found:true}", got)
	}

	missing, err := c.LogIndexOf(context.Background(), wt.ID, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if err != nil {
		t.Fatal(err)
	}
	if missing.Found {
		t.Errorf("LogIndexOf(missing) = %+v, want Found false", missing)
	}
}

func TestQA_PERF_02_LargeLoadWritesCommitGraphOnce(t *testing.T) {
	prev := largeRepoRows
	largeRepoRows = 3
	t.Cleanup(func() { largeRepoRows = prev })

	large := gittest.New(t)
	gittest.Commit(t, large.Dir, "a.txt", "1", "one")
	gittest.Commit(t, large.Dir, "a.txt", "2", "two")
	gittest.Commit(t, large.Dir, "a.txt", "3", "three")
	gittest.Commit(t, large.Dir, "a.txt", "4", "four")

	small := gittest.New(t)
	gittest.Commit(t, small.Dir, "a.txt", "1", "one")
	gittest.Commit(t, small.Dir, "a.txt", "2", "two")

	c := New(func(core.Event) {})

	largeInfo, err := c.OpenWorktree(context.Background(), core.RepoPath(large.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadLog(context.Background(), largeInfo.ID); err != nil {
		t.Fatal(err)
	}
	waitForLog(t, c, largeInfo.ID)

	smallInfo, err := c.OpenWorktree(context.Background(), core.RepoPath(small.Dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadLog(context.Background(), smallInfo.ID); err != nil {
		t.Fatal(err)
	}
	waitForLog(t, c, smallInfo.ID)

	// The write runs after the done signal, so wait for its command-log entry.
	deadline := time.Now().Add(10 * time.Second)
	for countCommands(t, c, largeInfo.ID, "git commit-graph write") == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	largeGraph := filepath.Join(large.Dir, ".git", "objects", "info", "commit-graph")
	if _, err := os.Stat(largeGraph); err != nil {
		t.Errorf("large repo commit-graph missing: %v", err)
	}
	if got := countCommands(t, c, largeInfo.ID, "git commit-graph write"); got != 1 {
		t.Errorf("large repo command log has %d commit-graph writes, want 1", got)
	}

	smallGraph := filepath.Join(small.Dir, ".git", "objects", "info", "commit-graph")
	if _, err := os.Stat(smallGraph); err == nil {
		t.Error("small repo should not get a commit-graph file")
	}
	if got := countCommands(t, c, smallInfo.ID, "git commit-graph write"); got != 0 {
		t.Errorf("small repo command log has %d commit-graph writes, want 0", got)
	}

	if err := c.LoadLog(context.Background(), largeInfo.ID); err != nil {
		t.Fatal(err)
	}
	waitForLog(t, c, largeInfo.ID)
	if got := countCommands(t, c, largeInfo.ID, "git commit-graph write"); got != 1 {
		t.Errorf("command log has %d commit-graph writes after a third LoadLog, want still 1", got)
	}
}

// countCommands counts the command-log entries whose command starts with prefix.
func countCommands(t *testing.T, c *Core, id core.WorktreeID, prefix string) int {
	t.Helper()
	entries, err := c.CommandLog(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Command, prefix) {
			n++
		}
	}
	return n
}

func logGeneration(t *testing.T, c *Core, id core.WorktreeID) int {
	t.Helper()
	res, err := c.LogRows(context.Background(), id, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	return res.Generation
}

func hasRefLabel(rows []core.LogCommit, name string) bool {
	for _, r := range rows {
		for _, ref := range r.Refs {
			if ref.Name == name {
				return true
			}
		}
	}
	return false
}

func buildLogFixture(n int) []byte {
	var buf bytes.Buffer
	for i := 0; i < n; i++ {
		sha := fmt.Sprintf("%040d", i+1)
		fmt.Fprintf(&buf, "%s\x1f%s\x1f\x1fAuthor\x1f%d\x1f\x1fCommit %d\x00\n", sha, sha[:7], 1700000000+i, i)
	}
	return buf.Bytes()
}

func waitForLog(t *testing.T, c *Core, id core.WorktreeID) []core.LogCommit {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		res, err := c.LogRows(context.Background(), id, 0, 1000)
		if err != nil {
			t.Fatal(err)
		}
		if res.Error != nil {
			t.Fatal(res.Error)
		}
		if res.Done {
			return res.Rows
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("log did not finish loading in time")
	return nil
}

func commitOrphan(t *testing.T, dir, branch, file, msg string) {
	t.Helper()
	gittest.Run(t, dir, "checkout", "--orphan", branch)
	_, _ = (git.Runner{}).Run(context.Background(), git.Call{Dir: dir, Args: []string{"rm", "-rf", "--cached", "."}})
	if err := os.WriteFile(filepath.Join(dir, file), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, dir, "add", "--", file)
	if _, err := (git.Runner{}).Run(context.Background(), git.Call{Dir: dir, Args: []string{"commit", "-F", "-"}, Stdin: []byte(msg)}); err != nil {
		t.Fatal(err)
	}
}

func TestFR_UX_02_AppDrivenReadsAreBackgroundCommands(t *testing.T) {
	repo := gittest.New(t)
	c := New(func(core.Event) {})
	wt, err := c.OpenWorktree(context.Background(), core.RepoPath(repo.Dir))
	if err != nil {
		t.Fatal(err)
	}

	if err := c.LoadLog(context.Background(), wt.ID); err != nil {
		t.Fatal(err)
	}
	waitForLog(t, c, wt.ID)
	if _, err := c.Summary(context.Background(), wt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Refs(context.Background(), wt.ID); err != nil {
		t.Fatal(err)
	}

	entries, err := c.CommandLog(context.Background(), wt.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.Background {
			t.Errorf("%q logged as a user command, want background (FR-UX-02: automatic reads are hidden by default)", e.Command)
		}
	}
}
